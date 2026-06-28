// Thin wrappers over lld's in-process drivers.
#include "bridge.h"

#include <csignal>

#include "lld/Common/Driver.h"
#include "llvm/Support/raw_ostream.h"

LLD_HAS_DRIVER(macho)
LLD_HAS_DRIVER(wasm)
LLD_HAS_DRIVER(elf)

namespace {

// lld installs its own crash-recovery signal handlers and an alternate signal
// stack. Inside the Go runtime that corrupts Go's signal handling, so snapshot
// the signal state on entry and restore it on exit.
struct SignalGuard {
    stack_t ss;
    bool ssOK;
    struct sigaction sa[32];
    bool saOK[32];
    SignalGuard() {
        ssOK = sigaltstack(nullptr, &ss) == 0;
        for (int i = 1; i < 32; i++) saOK[i] = sigaction(i, nullptr, &sa[i]) == 0;
    }
    ~SignalGuard() {
        for (int i = 1; i < 32; i++)
            if (saOK[i]) sigaction(i, &sa[i], nullptr);
        if (ssOK) sigaltstack(&ss, nullptr);
    }
};

int run(lld::Flavor flavor, lld::Driver driver, int argc, const char **argv,
        int *can_run_again) {
    SignalGuard guard;
    std::vector<const char *> args(argv, argv + argc);
    // Diagnostics go straight to fd 1/2; we deliberately do NOT capture them.
    // lld's fatal() path _exit()s the process (its CrashRecoveryContext cannot
    // unwind, since LLVM/lld are built without exceptions), so a captured buffer
    // would be lost and the failure would look silent. Writing to the real fd 2
    // lets lld flush the message before it exits, so the error is always visible.
    lld::Result r = lld::lldMain(args, llvm::outs(), llvm::errs(), {{flavor, driver}});
    if (can_run_again) *can_run_again = r.canRunAgain ? 1 : 0;
    return r.retCode;
}

}  // namespace

extern "C" int metall_lld_macho(int argc, const char **argv, int *can_run_again) {
    return run(lld::Darwin, &lld::macho::link, argc, argv, can_run_again);
}

extern "C" int metall_lld_wasm(int argc, const char **argv, int *can_run_again) {
    return run(lld::Wasm, &lld::wasm::link, argc, argv, can_run_again);
}

extern "C" int metall_lld_elf(int argc, const char **argv, int *can_run_again) {
    return run(lld::Gnu, &lld::elf::link, argc, argv, can_run_again);
}
