package types

import (
	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

// ExportWork describes an `export <c_name> = <fun>` declaration: a Metall
// function that is exposed to C under an unmangled symbol name.
type ExportWork struct {
	FunNodeID         ast.NodeID // target ast.Fun
	FunTypeID         TypeID
	InternalIR        string // mangled IR symbol of the target function
	CName             string // unmangled C symbol
	NameSpan          base.Span
	Env               *TypeEnv
	duplicateReported bool
}

func (e *Engine) Exports() []ExportWork {
	return e.exports
}

func (e *Engine) checkExport(nodeID ast.NodeID, exportNode ast.Export, span base.Span) (TypeID, TypeStatus) {
	_, mod := e.moduleOf(nodeID)
	if !mod.Main {
		e.diag(span, "export is only allowed in the main module")
		return InvalidTypeID, TypeFailed
	}
	for i, prev := range e.exports {
		if prev.CName == exportNode.Name.Name {
			if !prev.duplicateReported {
				e.diag(prev.NameSpan, "export name already used: %s", exportNode.Name.Name)
				e.exports[i].duplicateReported = true
			}
			e.diag(exportNode.Name.Span, "export name already used: %s", exportNode.Name.Name)
			return InvalidTypeID, TypeFailed
		}
	}
	targetTypeID, status := e.Query(exportNode.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	targetFun, funNodeID, status := e.resolveExportTarget(exportNode, targetTypeID)
	if status.Failed() {
		return InvalidTypeID, status
	}
	if status := e.validateExportSignature(targetTypeID, targetFun); status.Failed() {
		return InvalidTypeID, status
	}
	// FunDeclNode succeeded above, so the ident has a NamedFunRef too
	// (both maps are populated together in copyNamedFunRef).
	internalIR, _ := e.env.NamedFunRef(exportNode.Target)
	e.exports = append(e.exports, ExportWork{
		FunNodeID:         funNodeID,
		FunTypeID:         targetTypeID,
		InternalIR:        internalIR,
		CName:             exportNode.Name.Name,
		NameSpan:          exportNode.Name.Span,
		Env:               e.env,
		duplicateReported: false,
	})
	return e.voidTyp, TypeOK
}

// resolveExportTarget returns the ast.Fun the export refers to, emitting a
// diagnostic when the target is not a concrete Metall function in the current
// main module.
func (e *Engine) resolveExportTarget(exportNode ast.Export, targetTypeID TypeID) (ast.Fun, ast.NodeID, TypeStatus) {
	targetSpan := e.ast.Node(exportNode.Target).Span
	funNodeID, ok := e.env.FunDeclNode(exportNode.Target)
	if !ok {
		e.diag(targetSpan, "export target must be a function")
		return ast.Fun{}, 0, TypeFailed
	}
	targetFun, ok := e.ast.Node(funNodeID).Kind.(ast.Fun)
	if !ok {
		e.diag(targetSpan, "export target must be a Metall function declared in the current module")
		return ast.Fun{}, 0, TypeFailed
	}
	if targetFun.Builtin || targetFun.Extern {
		e.diag(targetSpan, "cannot export a builtin or extern function")
		return ast.Fun{}, 0, TypeFailed
	}
	if _, targetMod := e.moduleOf(funNodeID); !targetMod.Main {
		e.diag(targetSpan, "export target must be declared in the current main module")
		return ast.Fun{}, 0, TypeFailed
	}
	if e.env.containsTypeParam(targetTypeID) {
		e.diag(targetSpan, "cannot export a generic function")
		return ast.Fun{}, 0, TypeFailed
	}
	return targetFun, funNodeID, TypeOK
}

func (e *Engine) validateExportSignature(targetTypeID TypeID, targetFun ast.Fun) TypeStatus {
	funType := base.Cast[FunType](e.env.Type(targetTypeID).Kind)
	for i, paramTypeID := range funType.Params {
		if !isCCompatibleType(e.env, paramTypeID, false) {
			e.diag(e.ast.Node(targetFun.Params[i]).Span,
				"parameter type '%s' is not exportable to C", e.env.TypeDisplay(paramTypeID))
			return TypeFailed
		}
	}
	if !isCCompatibleType(e.env, funType.Return, true) {
		e.diag(e.ast.Node(targetFun.ReturnType).Span,
			"return type '%s' is not exportable to C", e.env.TypeDisplay(funType.Return))
		return TypeFailed
	}
	return TypeOK
}

// isCCompatibleType reports whether typeID maps to a primitive C type. Void
// is only allowed when `asReturn` is true, matching C's rule that `void`
// stands for "no value" in return position but is not a parameter type.
func isCCompatibleType(env *TypeEnv, typeID TypeID, asReturn bool) bool {
	switch env.Type(typeID).Kind.(type) {
	case IntType, FloatType, BoolType:
		return true
	case VoidType:
		return asReturn
	}
	return false
}

// checkExternCABIType rejects an extern parameter or return passed by value as a
// struct or union. Metall has no portable C ABI for aggregates by value, so they
// must be passed by pointer (ffi.Ptr). Scalars and pointer structs pass through.
func (e *Engine) checkExternCABIType(typeID TypeID, span base.Span, role string) bool {
	byValueAggregate := false
	switch kind := e.env.Type(typeID).Kind.(type) {
	case StructType:
		byValueAggregate = !IsBuiltinPtrStruct(kind)
	case UnionType:
		byValueAggregate = true
	}
	if byValueAggregate {
		e.diag(span, "extern function %s '%s' cannot be passed by value across the C ABI: "+
			"pass a struct or union by pointer (ffi.Ptr)", role, e.env.TypeDisplay(typeID))
		return false
	}
	return true
}
