// Thin wrappers over lld's in-process drivers.
#include "bridge.h"

#include <csignal>
#include <cstdlib>
#include <cstring>
#include <string>

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
        char **err, int *can_run_again) {
    SignalGuard guard;
    std::vector<const char *> args(argv, argv + argc);
    // Capture lld's diagnostics instead of letting them hit metallc's own fd 2,
    // so the Go caller can attach them to a link error (or drop them on success).
    std::string diag;
    llvm::raw_string_ostream diagOS(diag);
    lld::Result r = lld::lldMain(args, llvm::outs(), diagOS, {{flavor, driver}});
    diagOS.flush();
    if (can_run_again) *can_run_again = r.canRunAgain ? 1 : 0;
    if (err) {
        *err = diag.empty() ? nullptr : strdup(diag.c_str());
    }
    return r.retCode;
}

}  // namespace

extern "C" int metall_lld_macho(int argc, const char **argv, char **err, int *can_run_again) {
    return run(lld::Darwin, &lld::macho::link, argc, argv, err, can_run_again);
}

extern "C" int metall_lld_wasm(int argc, const char **argv, char **err, int *can_run_again) {
    return run(lld::Wasm, &lld::wasm::link, argc, argv, err, can_run_again);
}

extern "C" int metall_lld_elf(int argc, const char **argv, char **err, int *can_run_again) {
    return run(lld::Gnu, &lld::elf::link, argc, argv, err, can_run_again);
}
