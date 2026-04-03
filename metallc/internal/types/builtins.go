package types

import (
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
)

const ffiModuleName = "std::ffi"

var ffiBuiltinFuns = map[string]bool{ //nolint:gochecknoglobals
	"sizeof":          true,
	"ref_ptr":         true,
	"ref_ptr_mut":     true,
	"slice_ptr":       true,
	"slice_ptr_mut":   true,
	"fun_ptr":         true,
	"fun_ptr_alloc":   true,
	"Ptr.as_u64":      true,
	"Ptr.cast":        true,
	"Ptr.is_null":     true,
	"Ptr.null":        true,
	"Ptr.read":        true,
	"Ptr.offset":      true,
	"Ptr.as_slice":    true,
	"PtrMut.as_u64":   true,
	"PtrMut.null":     true,
	"PtrMut.cast":     true,
	"PtrMut.is_null":  true,
	"PtrMut.write":    true,
	"PtrMut.read":     true,
	"PtrMut.offset":   true,
	"PtrMut.as_ptr":   true,
	"PtrMut.as_slice": true,
	"FunPtr.call":     true,
}

var ffiBuiltinStructs = map[string]bool{ //nolint:gochecknoglobals
	"Ptr":    true,
	"PtrMut": true,
}

// MarkBuiltins marks functions and structs in known builtin modules (e.g. std::ffi)
// so they are handled by the compiler instead of being compiled normally.
func MarkBuiltins(a *ast.AST, module ast.Module) {
	if module.Name != ffiModuleName {
		return
	}
	for _, declNodeID := range module.Decls {
		node := a.Node(declNodeID)
		switch kind := node.Kind.(type) {
		case ast.Fun:
			if ffiBuiltinFuns[kind.Name.Name] {
				kind.Builtin = true
				node.Kind = kind
			}
		case ast.Struct:
			if ffiBuiltinStructs[kind.Name.Name] {
				kind.Builtin = true
				node.Kind = kind
			}
		}
	}
}

// BuiltinName returns the canonical builtin name (e.g. "ffi::sizeof") for a
// named function reference, or "" if it is not a builtin. The namedFunRef is
// the mangled name from the type environment (e.g. "std::ffi.sizeof.t42.t8").
func BuiltinName(namedFunRef string) string {
	if !strings.HasPrefix(namedFunRef, "std::ffi.") {
		return ""
	}
	rest := namedFunRef[len("std::ffi."):]
	for name := range ffiBuiltinFuns {
		if rest == name || strings.HasPrefix(rest, name+".") {
			return "ffi::" + name
		}
	}
	return ""
}

// BuiltinFunEffects returns the lifetime effects for an FFI builtin function,
// or nil if the function has no special lifetime effects.
// The effects describe how argument lifetimes flow to the return value.
func BuiltinFunEffects(name string) *FunEffects {
	switch name {
	case "ffi::ref_ptr", "ffi::ref_ptr_mut", "ffi::slice_ptr", "ffi::slice_ptr_mut":
		// The argument's lifetime flows to the return value.
		return &FunEffects{
			ReturnTaints:  []int{0},
			ReturnAliases: []int{0},
			SideEffects:   nil,
		}
	case "ffi::Ptr.offset", "ffi::PtrMut.offset", "ffi::PtrMut.as_ptr", "ffi::Ptr.cast", "ffi::PtrMut.cast":
		// The receiver's lifetime flows to the return value (param 0 = receiver).
		return &FunEffects{
			ReturnTaints:  []int{0},
			ReturnAliases: []int{0},
			SideEffects:   nil,
		}
	case "ffi::Ptr.read", "ffi::PtrMut.read", "ffi::Ptr.as_slice", "ffi::PtrMut.as_slice":
		// The receiver's taints flow to the return value.
		return &FunEffects{
			ReturnTaints:  []int{0},
			ReturnAliases: []int{0},
			SideEffects:   nil,
		}
	}
	return nil
}

// IsBuiltinPtrStruct returns true if the struct type is an ffi::Ptr<T> type.
// These are opaque pointer types that map to `ptr` in IR.
func IsBuiltinPtrStruct(s StructType) bool {
	return len(s.Fields) == 0 && len(s.TypeArgs) == 1 && strings.HasPrefix(s.Name, ffiModuleName+".")
}
