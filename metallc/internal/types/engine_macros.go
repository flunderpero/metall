package types

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/macros"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func (e *Engine) checkMacroModule(
	nodeID ast.NodeID, module ast.Module, span base.Span,
) (TypeID, TypeStatus) {
	if status := e.bindImports(nodeID, module); status.Failed() {
		return InvalidTypeID, status
	}
	if len(module.Decls) != 1 {
		e.diag(span, "macro modules must contain a single `apply` function")
		return InvalidTypeID, TypeFailed
	}
	applyNode := e.ast.Node(module.Decls[0])
	applyFun, ok := applyNode.Kind.(ast.Fun)
	if !ok || applyFun.Name.Name != "apply" {
		e.diag(applyNode.Span, "macro modules must contain a single `apply` function")
		return InvalidTypeID, TypeFailed
	}
	if len(applyFun.Params) < 2 {
		e.diag(applyNode.Span, "macro `apply` must have at least `sb &mut StrBuilder` and `@a Arena` parameters")
		return InvalidTypeID, TypeFailed
	}
	visibleParams := applyFun.Params[:len(applyFun.Params)-2]
	retTypeID, status := e.Query(applyFun.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	paramTypeIDs := make([]TypeID, len(visibleParams))
	for i, paramNodeID := range visibleParams {
		paramTypeID, status := e.Query(paramNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		paramTypeIDs[i] = paramTypeID
	}
	funTyp := FunType{Params: paramTypeIDs, Return: retTypeID, Macro: true}
	funTypeID := e.env.newType(funTyp, applyNode.ID, applyNode.Span, TypeOK)
	scope := e.scopeGraph.NodeScope(applyNode.ID)
	e.env.bindInScope(scope, applyNode.ID, "apply", funTypeID)
	e.env.setNamedFunRef(applyNode.ID, "apply")
	typeID := e.env.newType(ModuleType{Name: module.Name, Macro: true}, nodeID, span, TypeOK)
	return typeID, TypeOK
}

func (e *Engine) expandMacros(nodeID ast.NodeID, module *ast.Module) ([]ast.NodeID, bool) {
	var allDecls []ast.NodeID
	var newDecls []ast.NodeID
	expanded := false
	for _, declNodeID := range module.Decls {
		node := e.ast.Node(declNodeID)
		call, ok := node.Kind.(ast.Call)
		if !ok {
			allDecls = append(allDecls, declNodeID)
			continue
		}
		isMacro, macroModuleNodeID := e.isMacroCall(nodeID, call)
		if !isMacro {
			e.diag(node.Span, "only macro calls are allowed at the top level")
			return nil, false
		}
		macroDecls, ok := e.expandMacroCall(nodeID, call, node.Span, macroModuleNodeID)
		if !ok {
			return nil, false
		}
		allDecls = append(allDecls, macroDecls...)
		newDecls = append(newDecls, macroDecls...)
		expanded = true
	}
	if expanded {
		module.Decls = allDecls
		e.ast.Node(nodeID).Kind = *module
	}
	return newDecls, true
}

func (e *Engine) isMacroCall(moduleNodeID ast.NodeID, call ast.Call) (bool, ast.NodeID) {
	calleeNode := e.ast.Node(call.Callee)
	path, ok := calleeNode.Kind.(ast.Path)
	if !ok || len(path.Segments) != 2 {
		return false, 0
	}
	moduleName := path.Segments[0]
	modBinding, ok := e.lookup(call.Callee, moduleName)
	if !ok {
		return false, 0
	}
	modType, ok := e.env.Type(modBinding.TypeID).Kind.(ModuleType)
	if !ok || !modType.Macro {
		return false, 0
	}
	importedModuleNodeID, ok := e.moduleResolution.Imports[moduleNodeID][moduleName]
	if !ok {
		return false, 0
	}
	return true, importedModuleNodeID
}

func (e *Engine) expandMacroCall(
	contextNodeID ast.NodeID, call ast.Call, span base.Span, macroModuleNodeID ast.NodeID,
) ([]ast.NodeID, bool) {
	if e.macroExpander == nil {
		panic(base.Errorf("macro expander not set"))
	}
	macroModule := base.Cast[ast.Module](e.ast.Node(macroModuleNodeID).Kind)
	macroSource := e.macroModuleSource(macroModuleNodeID)
	if macroSource == "" {
		e.diag(span, "could not read macro module source")
		return nil, false
	}
	args := make([]macros.MacroArg, len(call.Args))
	for i, argNodeID := range call.Args {
		argNode := e.ast.Node(argNodeID)
		arg, ok := e.renderMacroArg(argNode)
		if !ok {
			return nil, false
		}
		args[i] = arg
	}
	expandedSource, err := e.macroExpander(macroSource, args)
	if err != nil {
		e.diag(span, "macro expansion failed: %s", err)
		return nil, false
	}
	source := base.NewSource(
		macroModule.FileName+".expanded",
		"__macro_expansion__",
		false,
		[]rune(expandedSource),
	)
	tokens := token.Lex(source)
	parser := ast.NewParser(tokens, e.ast)
	decls, ok := parser.ParseDecls()
	if len(parser.Diagnostics) > 0 {
		e.diagnostics = append(e.diagnostics, parser.Diagnostics...)
		return nil, false
	}
	if !ok {
		e.diag(span, "failed to parse macro expansion output")
		return nil, false
	}
	e.scopeGraph.WalkNodes(e.ast, decls, contextNodeID)
	return decls, true
}

func (e *Engine) renderMacroArg(argNode *ast.Node) (macros.MacroArg, bool) {
	if call, ok := argNode.Kind.(ast.Call); ok {
		if arg, ok := e.renderCompTypeOf(call, argNode.Span); ok {
			return arg, true
		}
	}
	rendered, diag := macros.RenderArg(argNode.Kind, argNode.Span)
	if diag != nil {
		e.diagnostics = append(e.diagnostics, *diag)
		return macros.MacroArg{}, false
	}
	return macros.MacroArg{Preamble: "", Expr: rendered}, true
}

func (e *Engine) renderCompTypeOf(call ast.Call, span base.Span) (macros.MacroArg, bool) {
	calleeNode := e.ast.Node(call.Callee)
	path, ok := calleeNode.Kind.(ast.Path)
	if !ok || len(path.Segments) != 2 || path.Segments[1] != "type_of" {
		return macros.MacroArg{}, false
	}
	modBinding, ok := e.lookup(call.Callee, path.Segments[0])
	if !ok {
		return macros.MacroArg{}, false
	}
	modType, ok := e.env.Type(modBinding.TypeID).Kind.(ModuleType)
	if !ok || modType.Name != "std::comp" {
		return macros.MacroArg{}, false
	}
	if len(path.TypeArgs) != 1 {
		e.diag(span, "comp::type_of requires exactly one type argument")
		return macros.MacroArg{}, false
	}
	typeID, status := e.Query(path.TypeArgs[0])
	if status.Failed() {
		return macros.MacroArg{}, false
	}
	r := &compTypeRenderer{engine: e, preamble: "", counter: 0}
	expr, ok := r.render(typeID, span)
	if !ok {
		return macros.MacroArg{}, false
	}
	return macros.MacroArg{Preamble: r.preamble, Expr: expr}, true
}

type compTypeRenderer struct {
	engine   *Engine
	preamble string
	counter  int
}

func (r *compTypeRenderer) let(expr string) string {
	name := fmt.Sprintf("__t%d", r.counter)
	r.counter++
	r.preamble += fmt.Sprintf("    let %s = %s\n", name, expr)
	return name
}

func (r *compTypeRenderer) letTyped(typeName string, expr string) string {
	name := fmt.Sprintf("__t%d", r.counter)
	r.counter++
	r.preamble += fmt.Sprintf("    let %s %s = %s\n", name, typeName, expr)
	return name
}

func (r *compTypeRenderer) render(typeID TypeID, span base.Span) (string, bool) {
	if typeID == r.engine.strTyp {
		return "comp::StrType()", true
	}
	if typeID == r.engine.boolTyp {
		return "comp::BoolType()", true
	}
	if typeID == r.engine.voidTyp {
		return "comp::VoidType()", true
	}
	typ := r.engine.env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case IntType:
		return fmt.Sprintf("comp::IntType(%q, %t, %d)", kind.Name, kind.Signed, kind.Bits), true
	case StructType:
		return r.renderStruct(kind, span)
	}
	r.engine.diag(span, "comp::type_of does not support type %s", r.engine.env.TypeDisplay(typeID))
	return "", false
}

func (r *compTypeRenderer) renderStruct(kind StructType, span base.Span) (string, bool) {
	var fields []string
	for _, f := range kind.Fields {
		fExpr, ok := r.render(f.Type, span)
		if !ok {
			return "", false
		}
		val := r.letTyped("comp::Type", fExpr)
		ref := r.let(fmt.Sprintf("@a.new(%s)", val))
		fields = append(fields, fmt.Sprintf("comp::Field(%q, %s, %t)", f.Name, ref, f.Mut))
	}
	return fmt.Sprintf("comp::StructType(%q, [%s][..])", kind.Name, strings.Join(fields, ", ")), true
}

func (e *Engine) macroModuleSource(moduleNodeID ast.NodeID) string {
	mod := base.Cast[ast.Module](e.ast.Node(moduleNodeID).Kind)
	for _, declNodeID := range mod.Decls {
		node := e.ast.Node(declNodeID)
		if node.Span.Source != nil {
			return string(node.Span.Source.Content)
		}
	}
	return ""
}
