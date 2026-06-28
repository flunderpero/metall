// Package backend drives LLVM and LLD in-process via cgo, so metallc emits
// objects and links executables without shelling out to clang/opt/lld.
package backend

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// LLD and parts of LLVM's global setup (target registry, command-line option
// globals, lld's error handler) are not safe to call concurrently. The Go
// frontend still runs in parallel; only these in-process backend calls
// serialize.
//
//nolint:gochecknoglobals // serialize non-reentrant LLVM/LLD, both guarded by mu
var (
	mu sync.Mutex
	// poisoned is set once lld reports (canRunAgain == 0) that a link left its
	// process-global state unsafe to reuse; further links fail fast instead of
	// running on corrupted linker state. Guarded by mu.
	poisoned bool
)

var errPoisoned = errors.New(
	"in-process linker is disabled: a previous link left lld in an unrecoverable state")

// LLVMCodeGenOptLevel values: none (-O0) and aggressive (-O3).
const (
	CodeGenNone       = 0
	CodeGenAggressive = 3
)

// DefaultTriple returns the host's default target triple.
func DefaultTriple() string {
	mu.Lock()
	defer mu.Unlock()
	c := C.metall_default_triple()
	defer C.free(unsafe.Pointer(c))
	return C.GoString(c)
}

// LLVMMajor returns the major version of the linked LLVM (e.g. 22), which names
// the clang resource dir (lib/clang/<major>). A stateless version read, so it
// needs no serialization.
func LLVMMajor() int {
	return int(C.metall_llvm_major())
}

// DataLayout returns the LLVM data-layout string for a triple.
func DataLayout(triple string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	ct := C.CString(triple)
	defer C.free(unsafe.Pointer(ct))
	var out, cerr *C.char
	//nolint:gocritic // dupSubExpr false positive on the cgo-expanded call
	if rc := C.metall_data_layout(ct, &out, &cerr); rc != 0 {
		return "", takeErr(cerr, "query data layout")
	}
	defer C.free(unsafe.Pointer(out))
	return C.GoString(out), nil
}

// EmitObject parses LLVM IR text, runs the middle-end pass pipeline, and writes
// a native or wasm object to objPath. passes is a new-PM pipeline string.
func EmitObject(ir []byte, triple, cpu, passes string, codegenLevel int, objPath string) error {
	mu.Lock()
	defer mu.Unlock()
	var irPtr *C.char
	if len(ir) > 0 {
		irPtr = (*C.char)(unsafe.Pointer(&ir[0]))
	}
	ct, cc, cp, co := C.CString(triple), C.CString(cpu), C.CString(passes), C.CString(objPath)
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cc))
	defer C.free(unsafe.Pointer(cp))
	defer C.free(unsafe.Pointer(co))
	var cerr *C.char
	//nolint:gocritic // dupSubExpr false positive on the cgo-expanded call
	rc := C.metall_emit_object(irPtr, C.size_t(len(ir)), ct, cc, C.int(codegenLevel), cp, co, &cerr)
	if rc != 0 {
		return takeErr(cerr, "emit object")
	}
	return nil
}

// LinkMachO/LinkWasm/LinkELF invoke the in-process lld driver. args is a full
// linker command line with args[0] = the driver name.
func LinkMachO(args []string) error {
	mu.Lock()
	defer mu.Unlock()
	if poisoned {
		return errPoisoned
	}
	argv, free := cArgv(args)
	defer free()
	var canRun C.int
	// Hold ForkLock so no concurrent fork inherits lld's still-open output fd;
	// otherwise exec of the just-linked executable races to ETXTBSY. The
	// out-of-process linker never leaked an fd into our forks.
	syscall.ForkLock.RLock()
	rc := C.metall_lld_macho(C.int(len(argv)), &argv[0], &canRun)
	syscall.ForkLock.RUnlock()
	return lldResult("mach-o", rc, canRun)
}

func LinkWasm(args []string) error {
	mu.Lock()
	defer mu.Unlock()
	if poisoned {
		return errPoisoned
	}
	argv, free := cArgv(args)
	defer free()
	var canRun C.int
	rc := C.metall_lld_wasm(C.int(len(argv)), &argv[0], &canRun)
	return lldResult("wasm", rc, canRun)
}

func LinkELF(args []string) error {
	mu.Lock()
	defer mu.Unlock()
	if poisoned {
		return errPoisoned
	}
	argv, free := cArgv(args)
	defer free()
	var canRun C.int
	// Hold ForkLock so no concurrent fork inherits lld's still-open output fd;
	// otherwise exec of the just-linked executable races to ETXTBSY. The
	// out-of-process linker never leaked an fd into our forks.
	syscall.ForkLock.RLock()
	rc := C.metall_lld_elf(C.int(len(argv)), &argv[0], &canRun)
	syscall.ForkLock.RUnlock()
	return lldResult("elf", rc, canRun)
}

// lldResult maps an lld return into a Go error and updates the poison flag. The
// caller must hold mu. lld has already written any diagnostics to stderr, so the
// error is just a short summary pointing there.
func lldResult(kind string, rc, canRun C.int) error {
	if canRun == 0 {
		poisoned = true
	}
	if rc == 0 {
		return nil
	}
	return fmt.Errorf("%s link failed (lld exit %d); see the linker diagnostics above", kind, int(rc))
}

// cArgv builds a NULL-free C string array from args. args must be non-empty.
func cArgv(args []string) ([]*C.char, func()) {
	argv := make([]*C.char, len(args))
	for i, a := range args {
		argv[i] = C.CString(a)
	}
	return argv, func() {
		for _, p := range argv {
			C.free(unsafe.Pointer(p))
		}
	}
}

func takeErr(cerr *C.char, what string) error {
	if cerr == nil {
		return fmt.Errorf("%s failed", what)
	}
	defer C.free(unsafe.Pointer(cerr))
	return fmt.Errorf("%s: %s", what, C.GoString(cerr))
}
