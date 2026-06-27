// Codegen half of the backend, in pure C via the LLVM-C API: parse IR, run the
// pass pipeline, emit a native/wasm object. The linker half lives in lld.cpp
// (LLD has no C API). All LLVM/LLD calls are serialized by the Go caller.
#include "bridge.h"

#include <pthread.h>
#include <stdlib.h>
#include <string.h>

#include <llvm-c/Core.h>
#include <llvm-c/Error.h>
#include <llvm-c/IRReader.h>
#include <llvm-c/Target.h>
#include <llvm-c/TargetMachine.h>
#include <llvm-c/Transforms/PassBuilder.h>

static char *dup_msg(const char *s) {
    if (!s) s = "";
    size_t n = strlen(s) + 1;
    char *r = malloc(n);
    memcpy(r, s, n);
    return r;
}

static void init_targets(void) {
    LLVMInitializeAllTargetInfos();
    LLVMInitializeAllTargets();
    LLVMInitializeAllTargetMCs();
    LLVMInitializeAllAsmPrinters();
    LLVMInitializeAllAsmParsers();
}

// Initialize every target backend, so metallc can emit code for any of them
// (cross-compilation). The static LLVM is built with all targets. pthread_once
// keeps this safe on its own, so it does not depend on the Go-side mutex that
// serializes the rest of the bridge (which could be relaxed for the read-only
// triple/data-layout queries).
static void init_once(void) {
    static pthread_once_t once = PTHREAD_ONCE_INIT;
    pthread_once(&once, init_targets);
}

char *metall_default_triple(void) {
    init_once();
    char *t = LLVMGetDefaultTargetTriple();
    char *r = dup_msg(t);
    LLVMDisposeMessage(t);
    return r;
}

int metall_data_layout(const char *triple, char **out, char **err) {
    init_once();
    char *msg = NULL;
    LLVMTargetRef target = NULL;
    if (LLVMGetTargetFromTriple(triple, &target, &msg)) {
        if (err) *err = dup_msg(msg ? msg : "no target for triple");
        LLVMDisposeMessage(msg);
        return 1;
    }
    LLVMTargetMachineRef tm =
        LLVMCreateTargetMachine(target, triple, "", "",
                                LLVMCodeGenLevelDefault, LLVMRelocPIC, LLVMCodeModelDefault);
    LLVMTargetDataRef td = LLVMCreateTargetDataLayout(tm);
    char *rep = LLVMCopyStringRepOfTargetData(td);
    *out = dup_msg(rep);
    LLVMDisposeMessage(rep);
    LLVMDisposeTargetData(td);
    LLVMDisposeTargetMachine(tm);
    return 0;
}

int metall_emit_object(const char *ir, size_t ir_len, const char *triple,
                       const char *cpu, int codegen_level, const char *passes,
                       const char *out_path, char **err) {
    init_once();
    LLVMContextRef ctx = LLVMContextCreate();
    LLVMMemoryBufferRef buf =
        LLVMCreateMemoryBufferWithMemoryRangeCopy(ir, ir_len, "metall-ir");

    char *msg = NULL;
    LLVMModuleRef mod = NULL;
    if (LLVMParseIRInContext(ctx, buf, &mod, &msg)) {
        if (err) *err = dup_msg(msg ? msg : "cannot parse IR");
        LLVMDisposeMessage(msg);
        LLVMContextDispose(ctx);
        return 2;
    }

    LLVMTargetRef target = NULL;
    if (LLVMGetTargetFromTriple(triple, &target, &msg)) {
        if (err) *err = dup_msg(msg ? msg : "no target for triple");
        LLVMDisposeMessage(msg);
        LLVMDisposeModule(mod);
        LLVMContextDispose(ctx);
        return 3;
    }

    // wasm ignores the relocation model; PIC is right for native PIE.
    LLVMTargetMachineRef tm = LLVMCreateTargetMachine(
        target, triple, cpu ? cpu : "", "", (LLVMCodeGenOptLevel)codegen_level,
        LLVMRelocPIC, LLVMCodeModelDefault);
    LLVMSetTarget(mod, triple);

    if (passes && passes[0]) {
        LLVMPassBuilderOptionsRef opts = LLVMCreatePassBuilderOptions();
        LLVMErrorRef perr = LLVMRunPasses(mod, passes, tm, opts);
        LLVMDisposePassBuilderOptions(opts);
        if (perr) {
            char *em = LLVMGetErrorMessage(perr);
            if (err) *err = dup_msg(em);
            LLVMDisposeErrorMessage(em);
            LLVMDisposeTargetMachine(tm);
            LLVMDisposeModule(mod);
            LLVMContextDispose(ctx);
            return 4;
        }
    }

    char *emit_err = NULL;
    if (LLVMTargetMachineEmitToFile(tm, mod, out_path, LLVMObjectFile, &emit_err)) {
        if (err) *err = dup_msg(emit_err ? emit_err : "object emission failed");
        LLVMDisposeMessage(emit_err);
        LLVMDisposeTargetMachine(tm);
        LLVMDisposeModule(mod);
        LLVMContextDispose(ctx);
        return 5;
    }

    LLVMDisposeTargetMachine(tm);
    LLVMDisposeModule(mod);
    LLVMContextDispose(ctx);
    return 0;
}
