package types

import (
	"slices"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
)

var builtinFuns = []string{ //nolint:gochecknoglobals
	"std::ffi.sizeof",
	"std::ffi.ref_ptr",
	"std::ffi.ref_ptr_mut",
	"std::ffi.slice_ptr",
	"std::ffi.slice_ptr_mut",
	"std::ffi.fun_ptr",
	"std::ffi.fun_ptr_alloc",
	"std::ffi.Ptr.as_u64",
	"std::ffi.Ptr.cast",
	"std::ffi.Ptr.is_null",
	"std::ffi.Ptr.null",
	"std::ffi.Ptr.read",
	"std::ffi.Ptr.offset",
	"std::ffi.Ptr.as_slice",
	"std::ffi.PtrMut.as_u64",
	"std::ffi.PtrMut.null",
	"std::ffi.PtrMut.cast",
	"std::ffi.PtrMut.is_null",
	"std::ffi.PtrMut.write",
	"std::ffi.PtrMut.read",
	"std::ffi.PtrMut.offset",
	"std::ffi.PtrMut.as_ptr",
	"std::ffi.PtrMut.as_slice",
	"std::ffi.FunPtr.call",
	"std::os.args",
}

// builtinCanonicalName converts "std::ffi.sizeof" to "ffi::sizeof".
func builtinCanonicalName(qualifiedName string) string {
	s := strings.TrimPrefix(qualifiedName, "std::")
	return strings.Replace(s, ".", "::", 1)
}

var ffiBuiltinStructs = map[string]bool{ //nolint:gochecknoglobals
	"Ptr":    true,
	"PtrMut": true,
}

// MarkBuiltins marks functions and structs in known builtin modules (e.g. std::ffi)
// so they are handled by the compiler instead of being compiled normally.
func MarkBuiltins(a *ast.AST, module ast.Module) {
	for _, declNodeID := range module.Decls {
		node := a.Node(declNodeID)
		switch kind := node.Kind.(type) {
		case ast.Fun:
			if isBuiltinFun(module.Name + "." + kind.Name.Name) {
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

func isBuiltinFun(qualifiedName string) bool {
	return slices.Contains(builtinFuns, qualifiedName)
}

// BuiltinName returns the canonical builtin name (e.g. "ffi::sizeof") for a
// named function reference, or "" if it is not a builtin. The namedFunRef is
// the mangled name from the type environment (e.g. "std::ffi.sizeof.t42.t8").
func BuiltinName(namedFunRef string) string {
	for _, b := range builtinFuns {
		if namedFunRef == b || strings.HasPrefix(namedFunRef, b+".") {
			return builtinCanonicalName(b)
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

// IsBuiltinPtrStruct returns true if the struct type lives in std::ffi.
func IsBuiltinPtrStruct(s StructType) bool {
	return len(s.Fields) == 0 && len(s.TypeArgs) == 1 && strings.HasPrefix(s.Name, "std::ffi.")
}
