// CPU-name validation. The codegen path (LLVMCreateTargetMachine) silently
// ignores an unrecognized -cpu and emits generic-baseline code, so we reject a
// bad name up front. isCPUStringValid checks the target's processor table
// without that fallback, and there is no LLVM-C API for it.
#include "bridge.h"

#include <memory>
#include <string>

#include "llvm/MC/MCSubtargetInfo.h"
#include "llvm/MC/TargetRegistry.h"
#include "llvm/TargetParser/Triple.h"

extern "C" int metall_cpu_valid(const char *triple, const char *cpu) {
    metall_init_targets();
    llvm::Triple tt(triple);
    std::string err;
    const llvm::Target *target = llvm::TargetRegistry::lookupTarget(tt, err);
    if (!target) return -1;
    // An empty CPU always constructs cleanly (no warning); validate the
    // candidate against the resulting subtarget's table.
    std::unique_ptr<llvm::MCSubtargetInfo> sti(
        target->createMCSubtargetInfo(tt, "", ""));
    if (!sti) return -1;
    return sti->isCPUStringValid(cpu) ? 1 : 0;
}
