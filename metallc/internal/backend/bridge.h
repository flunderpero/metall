#ifndef METALL_BACKEND_BRIDGE_H
#define METALL_BACKEND_BRIDGE_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

// All int-returning functions return 0 on success. On failure they return
// non-zero and, where an `err` out-param is given, set *err to a malloc'd
// message the caller must free.

// Host default target triple (e.g. "arm64-apple-macosx26.0.0").
char *metall_default_triple(void);

// Major version of the linked LLVM (e.g. 22), which names the clang resource
// dir (lib/clang/<major>).
unsigned metall_llvm_major(void);

// Data-layout string for a triple, via a throwaway TargetMachine.
int metall_data_layout(const char *triple, char **out, char **err);

// Parse LLVM IR text, run the middle-end pass pipeline, and emit a native or
// wasm object to out_path. codegen_level is an LLVMCodeGenOptLevel (0..3).
// passes is a new-PM pipeline string ("" runs none).
int metall_emit_object(const char *ir, size_t ir_len, const char *triple,
                       const char *cpu, int codegen_level, const char *passes,
                       const char *out_path, char **err);

// In-process lld drivers (the only part that must be C++: lld has no C API).
// argv is a full linker command line with argv[0] = the driver name. lld writes
// its diagnostics straight to stderr (fd 2); we do not capture them, so an error
// stays visible even when lld terminates the process itself. Returns the linker
// exit code. *can_run_again is lldMain's re-entry flag: 0 means lld's
// process-global state was left unsafe to reuse and no further link must run.
int metall_lld_macho(int argc, const char **argv, int *can_run_again);
int metall_lld_wasm(int argc, const char **argv, int *can_run_again);
int metall_lld_elf(int argc, const char **argv, int *can_run_again);

#ifdef __cplusplus
}
#endif

#endif
