package types

import (
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/macros"
	"github.com/flunderpero/metall/metallc/internal/modules"
)

type FunWork struct {
	NodeID ast.NodeID
	TypeID TypeID
	Name   string
	Env    *TypeEnv
}

type TypeWork struct {
	NodeID ast.NodeID
	TypeID TypeID
	Env    *TypeEnv
}

type ConstWork struct {
	NodeID ast.NodeID
	TypeID TypeID
	Name   string
	Env    *TypeEnv
}

type MacroExpander func(macroSource string, funName string, args []macros.MacroArg) (expandedSource string, err error)

type Engine struct {
	*TypeContext
	resolveGenerics func(nodeID ast.NodeID, typeHint *TypeID) (TypeID, TypeStatus, bool)
	macroExpander   MacroExpander
	consts          []ConstWork
	exports         []ExportWork
	voidTyp         TypeID
	boolTyp         TypeID
	strTyp          TypeID
	runeTyp         TypeID
	arenaTyp        TypeID
	intTyp          TypeID
	u8Typ           TypeID
	floatTyp        TypeID
	f32Typ          TypeID
	rangeTyp        TypeID
}

func NewEngine(
	a *ast.AST,
	preludeAST *ast.AST,
	moduleResolution *modules.ModuleResolution,
	macroExpander MacroExpander,
) *Engine {
	merged, err := preludeAST.Merge(a)
	if err != nil {
		panic(base.WrapErrorf(err, "failed to merge prelude AST"))
	}
	g := ast.BuildScopeGraph(merged, merged.Roots)
	c := NewTypeContext(merged, g, moduleResolution)
	e := &Engine{ //nolint:exhaustruct
		TypeContext:   c,
		macroExpander: macroExpander,
	}
	generics := newGenerics(c, e.Query, e.queryWithHint)
	e.resolveGenerics = func(nodeID ast.NodeID, typeHint *TypeID) (TypeID, TypeStatus, bool) {
		outcome := generics.Resolve(nodeID, typeHint)
		if !outcome.Handled {
			return InvalidTypeID, TypeFailed, false
		}
		e.runResolveTasks(outcome.Tasks)
		return outcome.TypeID, outcome.Status, true
	}
	for _, root := range preludeAST.Roots {
		e.Query(root)
	}
	if len(e.diagnostics) > 0 {
		panic(base.Errorf("prelude type-check failed: %s", e.diagnostics))
	}
	return e
}

func (e *Engine) AST() *ast.AST {
	return e.ast
}

func (e *Engine) ScopeGraph() *ast.ScopeGraph {
	return e.scopeGraph
}

func (e *Engine) Diagnostics() base.Diagnostics {
	return e.diagnostics
}

func (e *Engine) Env() *TypeEnv {
	return e.env
}

func (e *Engine) SetDebug(d base.Debug) {
	e.debug = d
}

func (e *Engine) Funs() []FunWork {
	names := make([]string, 0, len(e.funs))
	for name := range e.funs {
		names = append(names, name)
	}
	slices.Sort(names)
	var result []FunWork
	for _, name := range names {
		fw := e.funs[name]
		funTyp := base.Cast[FunType](e.env.Type(fw.TypeID).Kind)
		if !e.env.hasTypeParam(funTyp.Params) && !e.env.hasTypeParam([]TypeID{funTyp.Return}) {
			result = append(result, fw)
		}
	}
	return result
}

func (e *Engine) Structs() []TypeWork {
	names := make([]string, 0, len(e.structs))
	for name := range e.structs {
		names = append(names, name)
	}
	slices.Sort(names)
	var result []TypeWork
	for _, name := range names {
		sw := e.structs[name]
		structTyp := base.Cast[StructType](e.env.Type(sw.TypeID).Kind)
		if !e.env.hasTypeParam(structTyp.TypeArgs) {
			result = append(result, sw)
		}
	}
	return result
}

func (e *Engine) Unions() []TypeWork {
	names := make([]string, 0, len(e.unions))
	for name := range e.unions {
		names = append(names, name)
	}
	slices.Sort(names)
	var result []TypeWork
	for _, name := range names {
		uw := e.unions[name]
		unionTyp := base.Cast[UnionType](e.env.Type(uw.TypeID).Kind)
		if !e.env.hasTypeParam(unionTyp.TypeArgs) {
			result = append(result, uw)
		}
	}
	return result
}

func (e *Engine) Consts() []ConstWork {
	return e.consts
}

// Enums returns the enums that own an associated-data table: standalone closed
// enums and open roots. Every variant has a generated debug_name, so all of
// them get a table. Subsets contribute their variants to the root's table.
func (e *Engine) Enums() []TypeWork {
	var result []TypeWork
	for typeID, cached := range e.env.reg.types {
		et, ok := cached.Type.Kind.(EnumType)
		if !ok || et.Root != InvalidTypeID {
			continue
		}
		result = append(result, TypeWork{NodeID: cached.Type.NodeID, TypeID: typeID, Env: e.env})
	}
	slices.SortFunc(result, func(a, b TypeWork) int {
		return strings.Compare(
			base.Cast[EnumType](e.env.Type(a.TypeID).Kind).Name,
			base.Cast[EnumType](e.env.Type(b.TypeID).Kind).Name,
		)
	})
	return result
}

func (e *Engine) Query(nodeID ast.NodeID) (TypeID, TypeStatus) { //nolint:funlen
	if cached, ok := e.env.cachedNodeType(nodeID); ok {
		if cached.Status.Failed() {
			return InvalidTypeID, cached.Status
		}
		return cached.Type.ID, cached.Status
	}
	typeHint := e.typeHint
	e.typeHint = nil
	nodeDebug := ""
	debugDedent := func() {}
	if e.debug.Enabled() {
		nodeDebug = e.ast.Debug(nodeID, false, 0)
		debugDedent = e.debug.Print(1, "query start %s", nodeDebug).Indent()
	}
	defer debugDedent()
	node := e.ast.Node(nodeID)
	var typeID TypeID
	var status TypeStatus
	switch nodeKind := node.Kind.(type) {
	case ast.Assign:
		typeID, status = e.checkAssign(nodeKind)
	case ast.Binary:
		typeID, status = e.checkBinary(nodeKind, typeHint)
	case ast.Unary:
		typeID, status = e.checkUnary(nodeKind, typeHint)
	case ast.Block:
		typeID, status = e.checkBlock(nodeID, nodeKind, typeHint)
	case ast.Call:
		typeID, status = e.checkCall(nodeKind, nodeID, node.Span)
	case ast.Deref:
		typeID, status = e.checkDeref(nodeKind)
	case ast.Module:
		typeID, status = e.checkModule(nodeID, nodeKind, node.Span)
	case ast.If:
		typeID, status = e.checkIf(nodeKind, typeHint)
	case ast.When:
		typeID, status = e.checkWhen(nodeKind, typeHint)
	case ast.For:
		typeID, status = e.checkFor(nodeID, nodeKind)
	case ast.Break:
		typeID, status = e.checkBreak(node.Span)
	case ast.Continue:
		typeID, status = e.checkContinue(node.Span)
	case ast.Defer:
		typeID, status = e.checkDefer(nodeKind, node.Span)
	case ast.Fun, ast.Struct, ast.Shape, ast.Union, ast.Enum:
		cachedType, ok := e.env.cachedNodeType(nodeID)
		if !ok {
			nodes := []ast.NodeID{nodeID}
			e.forwardDeclareTypes(nodes)
			e.forwardDeclareFuns(nodes)
			cachedType, ok = e.env.cachedNodeType(nodeID)
			if !ok {
				return InvalidTypeID, TypeFailed
			}
		}
		if cachedType.Type == nil {
			return InvalidTypeID, cachedType.Status
		}
		return cachedType.Type.ID, cachedType.Status
	case ast.FunParam:
		typeID, status = e.checkFunParam(nodeKind)
	case ast.Return:
		typeID, status = e.checkReturn(nodeKind, node.Span)
	case ast.StructField:
		typeID, status = e.checkStructField(nodeKind)
	case ast.Match:
		typeID, status = e.checkMatch(nodeKind, node.Span, typeHint)
	case ast.TypeConstruction:
		typeID, status = e.checkTypeConstruction(nodeID, nodeKind, node.Span, typeHint)
	case ast.ArrayConstruction:
		typeID, status = e.checkArrayConstruction(nodeID, nodeKind, node.Span, typeHint)
	case ast.ArrayType:
		typeID, status = e.checkArrayType(nodeID, nodeKind, node.Span)
	case ast.SliceType:
		typeID, status = e.checkSliceType(nodeID, nodeKind, node.Span)
	case ast.FunType:
		typeID, status = e.checkFunType(nodeID, nodeKind, node.Span)
	case ast.ArrayLiteral:
		typeID, status = e.checkArrayLiteral(nodeID, nodeKind, node.Span, typeHint)
	case ast.EmptySlice:
		typeID, status = e.checkEmptySlice(node.Span, typeHint)
	case ast.Index:
		typeID, status = e.checkIndex(nodeKind)
	case ast.SubSlice:
		typeID, status = e.checkSubSlice(nodeID, nodeKind)
	case ast.Range:
		typeID, status = e.checkRange(nodeKind)
	case ast.AllocatorVar:
		typeID, status = e.checkAllocatorVar(nodeID, nodeKind, node.Span)
	case ast.FieldAccess:
		typeID, status = e.checkFieldAccess(nodeID, nodeKind)
	case ast.Ident:
		typeID, status = e.checkIdent(nodeID, nodeKind, node.Span)
	case ast.Bool:
		typeID, status = e.checkBool()
	case ast.Int:
		typeID, status = e.checkInt(nodeKind, node.Span, typeHint)
	case ast.Float:
		typeID, status = e.checkFloat(nodeKind, node.Span, typeHint)
	case ast.Ref:
		typeID, status = e.checkRef(nodeID, nodeKind, node.Span)
	case ast.RefType:
		typeID, status = e.checkRefType(nodeID, nodeKind, node.Span)
	case ast.SimpleType:
		typeID, status = e.checkSimpleType(nodeID, nodeKind, node.Span)
	case ast.String:
		typeID, status = e.checkString(nodeID, nodeKind, node.Span)
	case ast.RuneLiteral:
		typeID, status = e.checkRuneLiteral(nodeKind, node.Span, typeHint)
	case ast.Var:
		typeID, status = e.checkVar(nodeID, nodeKind, node.Span)
	case ast.Export:
		typeID, status = e.checkExport(nodeID, nodeKind, node.Span)
	default:
		panic(base.Errorf("unknown node kind: %T", nodeKind))
	}
	typeID, status = e.updateCachedType(node, typeID, status)
	if typeHint != nil && !status.Failed() && typeID != *typeHint {
		typeID = e.tryUnionAutoWrap(nodeID, typeID, *typeHint)
	}
	debugDedent()
	if e.debug.Enabled() {
		e.debug.Print(0, "query end   %s -> %s", nodeDebug, e.env.TypeDisplay(typeID))
	}
	return typeID, status
}

func (e *Engine) queryWithHint(nodeID ast.NodeID, typeHint *TypeID) (TypeID, TypeStatus) {
	if typeHint == nil {
		return e.Query(nodeID)
	}
	hintedType, ok := e.env.cachedTypeInfo(*typeHint)
	if ok {
		node := e.ast.Node(nodeID)
		// Integer and rune literals are context-sensitive, so they may need to be retyped even if
		// this node was already cached earlier under a more generic inference path.
		if intNode, ok := node.Kind.(ast.Int); ok {
			if _, ok := hintedType.Type.Kind.(IntType); ok {
				typeID, status := e.checkInt(intNode, node.Span, typeHint)
				if status.Failed() {
					return InvalidTypeID, status
				}
				cached, ok := e.env.cachedTypeInfo(typeID)
				if !ok {
					panic(base.Errorf("type %s not found", typeID))
				}
				e.env.setNodeType(nodeID, cached)
				return typeID, TypeOK
			}
		}
		if runeLit, ok := node.Kind.(ast.RuneLiteral); ok {
			if _, ok := hintedType.Type.Kind.(IntType); ok {
				typeID, status := e.checkRuneLiteral(runeLit, node.Span, typeHint)
				if status.Failed() {
					return InvalidTypeID, status
				}
				cached, ok := e.env.cachedTypeInfo(typeID)
				if !ok {
					panic(base.Errorf("type %s not found", typeID))
				}
				e.env.setNodeType(nodeID, cached)
				return typeID, TypeOK
			}
		}
		if floatNode, ok := node.Kind.(ast.Float); ok {
			if _, ok := hintedType.Type.Kind.(FloatType); ok {
				typeID, status := e.checkFloat(floatNode, node.Span, typeHint)
				if status.Failed() {
					return InvalidTypeID, status
				}
				cached, ok := e.env.cachedTypeInfo(typeID)
				if !ok {
					panic(base.Errorf("type %s not found", typeID))
				}
				e.env.setNodeType(nodeID, cached)
				return typeID, TypeOK
			}
		}
	}
	restore := e.withTypeHint(typeHint)
	typeID, status := e.Query(nodeID)
	restore()
	if !status.Failed() && typeID != *typeHint {
		typeID = e.tryUnionAutoWrap(nodeID, typeID, *typeHint)
	}
	return typeID, status
}

func (e *Engine) runResolveTasks(tasks []Task) {
	for _, t := range tasks {
		switch {
		case t.BodyCheck != nil:
			e.runMaterializedBody(t.BodyCheck.Mat, t.BodyCheck.FunTypeID)
		case t.RegisterStruct != nil:
			e.registerStruct(t.RegisterStruct.Type, t.RegisterStruct.DeclNodeID, t.RegisterStruct.TypeID)
		case t.RegisterUnion != nil:
			e.registerUnion(t.RegisterUnion.Type, t.RegisterUnion.DeclNodeID, t.RegisterUnion.TypeID)
		case t.RegisterFun != 0:
			e.registerFun(t.RegisterFun)
		}
	}
}

func (e *Engine) runMaterializedBody(mat FunMaterialization, funTypeID TypeID) {
	if !mat.NeedsBodyCheck {
		return
	}
	prevEnv := e.env
	prevSkip := e.skipRegisterWork
	e.env = mat.Env
	e.skipRegisterWork = mat.SkipRegister
	defer func() {
		e.env = prevEnv
		e.skipRegisterWork = prevSkip
	}()
	defer e.withInstantiationScope(&mat.CallSiteNodeID)()
	funNode := base.Cast[ast.Fun](e.ast.Node(mat.FunNodeID).Kind)
	e.checkFunBody(mat.FunNodeID, funNode, funTypeID, mat.FunType)
}

func (e *Engine) checkModule( //nolint:funlen
	nodeID ast.NodeID,
	module ast.Module,
	span base.Span,
) (TypeID, TypeStatus) {
	if macros.IsMacroModule(module.Name) {
		return e.checkMacroModule(nodeID, module, span)
	}
	if status := e.bindImports(nodeID, module); status.Failed() {
		return InvalidTypeID, status
	}
	e.forwardDeclareTypes(module.Decls)
	newDeclIDs, ok := e.expandMacrosInModule(nodeID, &module)
	if !ok {
		return InvalidTypeID, TypeFailed
	}
	e.forwardDeclareTypes(newDeclIDs)
	// Module-level constants are checked before functions so function bodies
	// can reference them.
	depFailed := false
	for _, declNodeID := range module.Decls {
		varNode, isVar := e.ast.Node(declNodeID).Kind.(ast.Var)
		if !isVar {
			continue
		}
		_, status := e.Query(declNodeID)
		if status.Failed() {
			depFailed = true
			continue
		}
		if !e.isConstExpr(varNode.Expr) {
			e.diag(e.ast.Node(varNode.Expr).Span, "module-level constant must be a constant expression")
			depFailed = true
			continue
		}
		typeID := e.env.TypeOfNode(varNode.Expr).ID
		name := e.declMangledName(declNodeID, varNode.Name.Name)
		e.consts = append(e.consts, ConstWork{NodeID: declNodeID, TypeID: typeID, Name: name, Env: e.env})
	}
	MarkBuiltins(e.ast, module)
	e.forwardDeclareFuns(module.Decls)
	// Eagerly register every fun in runtime modules: generated IR calls into
	// them by mangled name (e.g. @runtime$arena.arena_alloc) without Metall
	// ever `use`-ing them, so the default lazy registration would miss them.
	for _, declNodeID := range module.Decls {
		if fun, ok := e.ast.Node(declNodeID).Kind.(ast.Fun); ok {
			if fun.Name.Name == "main" ||
				module.Name == "runtime::arena" ||
				module.Name == "runtime::wasmalloc" ||
				module.Name == "std::errors" {
				e.registerFun(declNodeID)
			}
		}
	}
	for _, declNodeID := range module.Decls {
		switch e.ast.Node(declNodeID).Kind.(type) {
		case ast.Var, ast.Import:
			continue
		}
		_, status := e.Query(declNodeID)
		if status.Failed() {
			depFailed = true
		}
	}
	if depFailed {
		return InvalidTypeID, TypeDepFailed
	}
	if module.Name == "" {
		return e.voidTyp, TypeOK
	}
	typeID := e.env.newType(ModuleType{Name: module.Name, Macro: false}, nodeID, span, TypeOK)
	return typeID, TypeOK
}

func (e *Engine) bindImports(nodeID ast.NodeID, module ast.Module) TypeStatus {
	importMap, ok := e.moduleResolution.Imports[nodeID]
	if !ok {
		return TypeOK
	}
	for name, importedModuleNodeID := range importMap {
		typeID, status := e.Query(importedModuleNodeID)
		if status.Failed() {
			return TypeDepFailed
		}
		var importNodeID ast.NodeID
		for _, id := range module.Decls {
			imp, ok := e.ast.Node(id).Kind.(ast.Import)
			if !ok {
				continue
			}
			isAlias := imp.Alias != nil && imp.Alias.Name == name
			if isAlias || imp.Segments[len(imp.Segments)-1] == name {
				importNodeID = id
				break
			}
		}
		if importNodeID == 0 {
			panic(base.Errorf(
				"module resolution exposed %q with no matching import decl in module %q",
				name, module.Name,
			))
		}
		scope := e.scopeGraph.NodeScope(importNodeID)
		e.env.bindInScope(scope, importNodeID, name, typeID)
	}
	return TypeOK
}

func (e *Engine) checkBlock(blockNodeID ast.NodeID, block ast.Block, typeHint *TypeID) (TypeID, TypeStatus) {
	if len(block.Exprs) == 0 {
		return e.voidTyp, TypeOK
	}
	e.forwardDeclareTypes(block.Exprs)
	newDeclIDs, ok := e.expandMacrosInBlock(blockNodeID, &block)
	if !ok {
		return InvalidTypeID, TypeFailed
	}
	e.forwardDeclareTypes(newDeclIDs)
	// Function literals with inferred types are resolved in a single pass before
	// forwardDeclareFuns, which only deals with fully-typed declarations.
	// We need either inferred param types (which always require a hint) or
	// an inferred return type with a usable hint.
	if len(block.Exprs) == 2 {
		if funNode, ok := e.ast.Node(block.Exprs[0]).Kind.(ast.Fun); ok {
			if e.funLitNeedsInference(funNode) {
				return e.checkInferredFunLit(blockNodeID, block, funNode, typeHint)
			}
		}
	}
	e.forwardDeclareFuns(block.Exprs)
	depFailed := false
	var lastExprTypeID TypeID
	var status TypeStatus
	wouldBeDeadCode := false
	lastIdx := len(block.Exprs) - 1
	for i, exprNodeID := range block.Exprs {
		if wouldBeDeadCode {
			e.diag(e.ast.Node(exprNodeID).Span, "unreachable code")
			return InvalidTypeID, TypeDepFailed
		}
		restoreIndex := e.withBlockIndex(i)
		if i == lastIdx {
			lastExprTypeID, status = e.queryWithHint(exprNodeID, typeHint)
		} else {
			lastExprTypeID, status = e.Query(exprNodeID)
			if !status.Failed() && e.isInstantiable(lastExprTypeID) && lastExprTypeID != e.voidTyp {
				switch e.ast.Node(exprNodeID).Kind.(type) {
				case ast.Fun, ast.Struct, ast.Shape, ast.Union, ast.Enum:
				default:
					e.diag(
						e.ast.Node(exprNodeID).Span,
						"expression result of type %s is unused, assign to _ to discard",
						e.env.TypeDisplay(lastExprTypeID),
					)
					depFailed = true
				}
			}
		}
		if status.Failed() {
			depFailed = true
		}
		restoreIndex()
		if lastExprTypeID == e.neverTyp {
			wouldBeDeadCode = true
		}
	}
	if depFailed {
		return InvalidTypeID, TypeDepFailed
	}
	return lastExprTypeID, TypeOK
}

func (e *Engine) checkIf(if_ ast.If, typeHint *TypeID) (TypeID, TypeStatus) {
	condType, status := e.Query(if_.Cond)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if condType != e.boolTyp {
		condSpan := e.ast.Node(if_.Cond).Span
		e.diag(condSpan, "if condition must evaluate to a boolean value, got %s",
			e.env.TypeDisplay(condType))
		return InvalidTypeID, TypeFailed
	}
	thenType, status := e.queryWithHint(if_.Then, typeHint)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if if_.Else == nil {
		return e.voidTyp, TypeOK
	}
	elseType, status := e.queryWithHint(*if_.Else, typeHint)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if elseType == e.neverTyp && thenType == e.neverTyp {
		return e.neverTyp, TypeOK
	}
	// A diverging branch (`return`/`break`/`panic`) yields no value, so the if
	// takes the live branch's type, matching `when`/`match` arm unification.
	if elseType == e.neverTyp {
		return thenType, TypeOK
	}
	if thenType == e.neverTyp {
		return elseType, TypeOK
	}
	if thenType != elseType {
		e.diag(
			e.ast.Node(*if_.Else).Span,
			"if branch type mismatch: expected %s, got %s",
			e.env.TypeDisplay(thenType),
			e.env.TypeDisplay(elseType),
		)
		return InvalidTypeID, TypeFailed
	}
	return thenType, TypeOK
}

func (e *Engine) checkWhen(when ast.When, typeHint *TypeID) (TypeID, TypeStatus) {
	bodyNodeIDs := make([]ast.NodeID, 0, len(when.Cases)+1)
	for _, case_ := range when.Cases {
		bodyNodeIDs = append(bodyNodeIDs, case_.Body)
	}
	if when.Else != nil {
		bodyNodeIDs = append(bodyNodeIDs, *when.Else)
	}
	for _, case_ := range when.Cases {
		condType, status := e.Query(case_.Cond)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if condType != e.boolTyp {
			e.diag(
				e.ast.Node(case_.Cond).Span,
				"when case condition must evaluate to a boolean value, got %s",
				e.env.TypeDisplay(condType),
			)
			return InvalidTypeID, TypeFailed
		}
	}
	var resultType *TypeID
	if when.Else == nil {
		resultType = &e.voidTyp
	}
	mergeBodyType := func(body ast.NodeID) (TypeStatus, bool) {
		bodyType, status := e.queryWithHint(body, typeHint)
		if status.Failed() {
			return TypeDepFailed, false
		}
		if bodyType == e.neverTyp {
			return TypeOK, true
		}
		if resultType == nil {
			resultType = &bodyType
			return TypeOK, true
		}
		if bodyType != *resultType {
			e.diag(
				e.ast.Node(body).Span,
				"when branch type mismatch: expected %s, got %s",
				e.env.TypeDisplay(*resultType),
				e.env.TypeDisplay(bodyType),
			)
			return TypeFailed, false
		}
		return TypeOK, true
	}
	for _, bodyNodeID := range bodyNodeIDs {
		if status, ok := mergeBodyType(bodyNodeID); !ok {
			return InvalidTypeID, status
		}
	}
	if resultType == nil {
		return e.neverTyp, TypeOK
	}
	return *resultType, TypeOK
}

func (e *Engine) checkFor(nodeID ast.NodeID, for_ ast.For) (TypeID, TypeStatus) { //nolint:funlen
	if for_.Binding != nil {
		cond := e.ast.Node(*for_.Cond)
		// A literal `lo..hi` range needs both bounds; it then iterates as a Range
		// value through the same iterator protocol as any other iterable.
		if range_, ok := cond.Kind.(ast.Range); ok && (range_.Lo == nil || range_.Hi == nil) {
			e.diag(cond.Span, "for-in range requires both lower and upper bound")
			return InvalidTypeID, TypeFailed
		}
		{
			iterTypeID, status := e.Query(*for_.Cond)
			if status.Failed() {
				return InvalidTypeID, status
			}
			var elemTypeID TypeID
			var elemMut bool
			isIter := false
			switch k := e.env.Type(iterTypeID).Kind.(type) {
			case ArrayType:
				elemTypeID = k.Elem
				_, elemMut = e.isPlaceMutable(*for_.Cond)
			case SliceType:
				elemTypeID = k.Elem
				elemMut = k.Mut
			default:
				var status TypeStatus
				elemTypeID, status = e.checkForInIter(nodeID, for_, iterTypeID, cond.Span)
				if status.Failed() {
					return InvalidTypeID, status
				}
				isIter = true
			}
			bindTypeID := elemTypeID
			if for_.Ref {
				if isIter {
					e.diag(for_.Binding.Span,
						"for-in over an iterator cannot bind a reference; have next() yield one")
					return InvalidTypeID, TypeFailed
				}
				if for_.Mut && !elemMut {
					e.diag(for_.Binding.Span,
						"`for &mut` requires a mutable slice ([]mut T) or a mutable array, got %s",
						e.env.TypeDisplay(iterTypeID))
					return InvalidTypeID, TypeFailed
				}
				bindTypeID = e.env.buildRefType(nodeID, elemTypeID, for_.Mut, for_.Binding.Span)
			} else if !e.isCopyable(elemTypeID) {
				e.diag(for_.Binding.Span,
					"cannot copy value of nocopy type %s; iterate by reference with `for &`",
					e.env.TypeDisplay(elemTypeID))
				return InvalidTypeID, TypeFailed
			}
			e.bind(for_.Body, for_.Binding.Name, false, bindTypeID, for_.Binding.Span, -1)
			if for_.Index != nil {
				// Bindings are keyed by their decl node, so the index reuses the
				// iterand node to get a distinct BindingID from the element.
				e.bind(*for_.Cond, for_.Index.Name, false, e.intTyp, for_.Index.Span, -1)
			}
		}
	} else if for_.Cond != nil {
		condType, status := e.Query(*for_.Cond)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if condType != e.boolTyp {
			condSpan := e.ast.Node(*for_.Cond).Span
			e.diag(condSpan, "type mismatch: expected Bool, got %s", e.env.TypeDisplay(condType))
			return InvalidTypeID, TypeFailed
		}
	}
	defer e.enterLoop(nodeID)()
	bodyTypeID, status := e.Query(for_.Body)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if bodyTypeID != e.voidTyp && e.isInstantiable(bodyTypeID) {
		bodySpan := e.ast.Node(for_.Body).Span
		e.diag(bodySpan, "for loop body must not yield a value, got %s", e.env.TypeDisplay(bodyTypeID))
		return InvalidTypeID, TypeFailed
	}
	return e.voidTyp, TypeOK
}

// checkForInIter resolves `for x in <iter>` over a type whose next() returns
// ?T, returning the element type T. It records next()'s mangled name, the
// optional return type, and next's noescape flag for codegen and lifetime. The
// loop iterates a private copy of the iterator, so the iterand need not be a
// mutable place.
func (e *Engine) checkForInIter(
	forNodeID ast.NodeID, for_ ast.For, iterTypeID TypeID, span base.Span,
) (TypeID, TypeStatus) {
	// Resolve next() by looking up `<iterable>.next` through the normal method
	// resolver. The synthesized field-access (off-tree, only here to drive
	// resolution) lets one call cover all receiver shapes: a concrete type finds
	// next in its module, a type parameter dispatches through its Iter shape, and
	// a generic instance gets monomorphized and registered for codegen.
	faNode := e.ast.NewFieldAccess(*for_.Cond, ast.Name{Name: "next", Span: span}, nil, span)
	e.scopeGraph.SetNodeScope(faNode, e.scopeGraph.NodeScope(forNodeID))
	// A non-iterable fails here with the resolver's own "unknown field next"
	// diagnostic, which is left to stand.
	nextTypeID, status := e.Query(faNode)
	if status.Failed() {
		return InvalidTypeID, status
	}
	nextFun, ok := e.env.Type(nextTypeID).Kind.(FunType)
	if !ok {
		e.diag(span, "cannot iterate over %s", e.env.TypeDisplay(iterTypeID))
		return InvalidTypeID, TypeFailed
	}
	retUnion, ok := e.env.Type(nextFun.Return).Kind.(UnionType)
	if !ok || len(retUnion.TypeArgs) == 0 {
		e.diag(span, "cannot iterate over %s: its next() must return an optional ?T",
			e.env.TypeDisplay(iterTypeID))
		return InvalidTypeID, TypeFailed
	}
	// Codegen iterates a private copy of the iterator, so it must be copyable.
	if !e.isCopyable(iterTypeID) {
		e.diag(span, "cannot iterate over nocopy iterator %s", e.env.TypeDisplay(iterTypeID))
		return InvalidTypeID, TypeFailed
	}
	// The mangled next() name lands on the field-access; carry it to the For node
	// for codegen. Absent at the generic level (shape dispatch), which is fine:
	// only concrete instances are emitted.
	if name, ok := e.env.NamedFunRef(faNode); ok {
		e.env.setNamedFunRef(forNodeID, name)
	}
	e.env.setForIterRet(forNodeID, nextFun.Return)
	return retUnion.TypeArgs[0], TypeOK
}

func (e *Engine) checkRange(range_ ast.Range) (TypeID, TypeStatus) {
	checkBound := func(bound *ast.NodeID) bool {
		if bound == nil {
			return true
		}
		boundTypeID, s := e.Query(*bound)
		if s.Failed() {
			return false
		}
		if boundTypeID != e.intTyp {
			boundSpan := e.ast.Node(*bound).Span
			e.diag(boundSpan, "range bound must be Int, got %s", e.env.TypeDisplay(boundTypeID))
			return false
		}
		return true
	}
	if !checkBound(range_.Lo) || !checkBound(range_.Hi) {
		return InvalidTypeID, TypeFailed
	}
	return e.rangeTyp, TypeOK
}

func (e *Engine) checkBreak(span base.Span) (TypeID, TypeStatus) {
	if len(e.loopStack) == 0 {
		e.diag(span, "break statement outside of loop")
		return InvalidTypeID, TypeFailed
	}
	return e.neverTyp, TypeOK
}

func (e *Engine) checkContinue(span base.Span) (TypeID, TypeStatus) {
	if len(e.loopStack) == 0 {
		e.diag(span, "continue statement outside of loop")
		return InvalidTypeID, TypeFailed
	}
	return e.neverTyp, TypeOK
}

func (e *Engine) checkDefer(defer_ ast.Defer, span base.Span) (TypeID, TypeStatus) {
	if e.deferTransfersControl(defer_.Block, false) {
		e.diag(span, "defer block cannot transfer control: no return, break, continue, or try")
		return InvalidTypeID, TypeFailed
	}
	blockTypeID, status := e.Query(defer_.Block)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if blockTypeID != e.voidTyp && e.isInstantiable(blockTypeID) {
		e.diag(span, "defer block must not yield a value, got %s", e.env.TypeDisplay(blockTypeID))
		return InvalidTypeID, TypeFailed
	}
	return e.voidTyp, TypeOK
}

// deferTransfersControl reports whether the subtree transfers control out of a
// defer block. `return` always does; `break`/`continue` do unless they target a
// loop inside the defer. Nested functions are their own control scope. A `try`
// is caught via the `return` it desugars to.
func (e *Engine) deferTransfersControl(nodeID ast.NodeID, inLoop bool) bool {
	switch e.ast.Node(nodeID).Kind.(type) {
	case ast.Return:
		return true
	case ast.Break, ast.Continue:
		return !inLoop
	case ast.Fun:
		return false
	case ast.For:
		inLoop = true
	}
	found := false
	e.ast.Walk(nodeID, func(child ast.NodeID) {
		if !found && e.deferTransfersControl(child, inLoop) {
			found = true
		}
	})
	return found
}

func (e *Engine) checkReturn(return_ ast.Return, span base.Span) (TypeID, TypeStatus) {
	if len(e.funStack) == 0 {
		e.diag(span, "return outside of function")
		return InvalidTypeID, TypeFailed
	}
	funType := base.Cast[FunType](e.env.Type(e.funStack[len(e.funStack)-1]).Kind)
	exprTypeID, status := e.queryWithHint(return_.Expr, &funType.Return)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if !e.isAssignableTo(exprTypeID, funType.Return) {
		span := e.ast.Node(return_.Expr).Span
		e.diag(
			span,
			"return type mismatch: expected %s, got %s",
			e.env.TypeDisplay(funType.Return),
			e.env.TypeDisplay(exprTypeID),
		)
		return InvalidTypeID, TypeFailed
	}
	return e.neverTyp, TypeOK
}

// checkInferredFunLit resolves a function literal whose param or return type
// is omitted, using the supplied hint as the expected function type. Fun
// literals don't participate in mutual recursion and the hint is only
// available here, so they bypass the two-phase forwardDeclareFuns approach.
func (e *Engine) checkInferredFunLit( //nolint:funlen
	_ ast.NodeID, block ast.Block, funNode ast.Fun, hintTypeID *TypeID,
) (TypeID, TypeStatus) {
	var hintFun *FunType
	if hintTypeID != nil {
		if cached, ok := e.env.cachedTypeInfo(*hintTypeID); ok {
			if ft, ok := cached.Type.Kind.(FunType); ok {
				hintFun = &ft
			}
		}
	}
	funNodeID := block.Exprs[0]
	node := e.ast.Node(funNodeID)
	paramTypeIDs := make([]TypeID, len(funNode.Params))
	noescapeParams := make([]bool, len(funNode.Params))
	for i, paramNodeID := range funNode.Params {
		paramNode := base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind)
		if paramNode.Type == ast.InferredType {
			if hintFun == nil || i >= len(hintFun.Params) {
				e.diag(paramNode.Name.Span, "cannot infer type of parameter '%s'", paramNode.Name.Name)
				return InvalidTypeID, TypeFailed
			}
			paramTypeIDs[i] = hintFun.Params[i]
			paramCached, paramOK := e.env.cachedTypeInfo(hintFun.Params[i])
			if !paramOK {
				panic(base.Errorf("type %s not found", hintFun.Params[i]))
			}
			e.env.setNodeType(paramNodeID, paramCached)
		} else {
			paramTypeID, status := e.Query(paramNodeID)
			if status.Failed() {
				return InvalidTypeID, TypeDepFailed
			}
			paramTypeIDs[i] = paramTypeID
		}
		if paramNode.Noescape {
			noescapeParams[i] = true
		}
	}
	var retTypeID TypeID
	inferRetFromBody := false
	if funNode.ReturnType == ast.InferredType {
		if hintFun == nil {
			e.diag(node.Span, "cannot infer return type of function literal")
			return InvalidTypeID, TypeFailed
		} else if _, isTypeParam := e.env.Type(hintFun.Return).Kind.(TypeParamType); isTypeParam {
			inferRetFromBody = true
		} else {
			retTypeID = hintFun.Return
		}
	} else {
		var status TypeStatus
		retTypeID, status = e.Query(funNode.ReturnType)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
	}
	// Push a temporary fun type so `return` and `try` in the body resolve
	// against this function, not an outer one. We skip the push when
	// inferring the return from the body (which has no return/try support).
	if !inferRetFromBody {
		tmpFunType := FunType{
			Params: paramTypeIDs, Return: retTypeID,
			Macro: false, Sync: false, Unsafe: funNode.Unsafe, NoescapeParams: noescapeParams,
			NoescapeReturn: funNode.NoescapeReturn,
		}
		tmpFunTypeID := e.env.buildFunType(tmpFunType, 0, node.Span)
		defer e.enterFun(tmpFunTypeID)()
	}
	for _, capNodeID := range funNode.Captures {
		capture := base.Cast[ast.Capture](e.ast.Node(capNodeID).Kind)
		e.bindCapture(funNodeID, capNodeID, capture)
	}
	for i, paramNodeID := range funNode.Params {
		paramNode := base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind)
		if !e.bind(paramNodeID, paramNode.Name.Name, false, paramTypeIDs[i], paramNode.Name.Span, -1) {
			return InvalidTypeID, TypeFailed
		}
	}
	if inferRetFromBody {
		// The hint's return type is an unresolved type parameter, so use the
		// body's result as the return type without supplying a hint.
		blockTypeID, status := e.Query(funNode.Block)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		retTypeID = blockTypeID
	} else {
		blockTypeID, status := e.queryWithHint(funNode.Block, &retTypeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if blockTypeID != e.neverTyp && !e.isAssignableTo(blockTypeID, retTypeID) {
			e.diag(e.ast.Node(funNode.Block).Span,
				"return type mismatch: expected %s, got %s",
				e.env.TypeDisplay(retTypeID), e.env.TypeDisplay(blockTypeID))
			return InvalidTypeID, TypeFailed
		}
	}
	isSync := e.isFunDeclSync(node, paramTypeIDs, retTypeID)
	funType := FunType{
		Params: paramTypeIDs, Return: retTypeID,
		Macro: false, Sync: isSync, Unsafe: funNode.Unsafe, NoescapeParams: noescapeParams,
		NoescapeReturn: funNode.NoescapeReturn,
	}
	funTypeID := e.env.buildFunType(funType, funNodeID, node.Span)
	e.updateCachedType(node, funTypeID, TypeOK)
	e.bind(funNodeID, funNode.Name.Name, false, funTypeID, funNode.Name.Span, -1)
	e.env.setNamedFunRef(funNodeID, e.declMangledName(funNodeID, funNode.Name.Name))
	identTypeID, status := e.queryWithHint(block.Exprs[1], hintTypeID)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	return identTypeID, TypeOK
}

func (e *Engine) checkFunBody(funNodeID ast.NodeID, funNode ast.Fun, funTypeID TypeID, funType FunType) {
	debugDedent := func() {}
	if e.debug.Enabled() {
		debugDedent = e.debug.Print(0, "checkFunBody %s (type=%s)", funNode.Name.Name, funTypeID).Indent()
	}
	defer debugDedent()
	defer e.enterFun(funTypeID)()
	for _, capNodeID := range funNode.Captures {
		capture := base.Cast[ast.Capture](e.ast.Node(capNodeID).Kind)
		e.bindCapture(funNodeID, capNodeID, capture)
	}
	for i, paramNodeID := range funNode.Params {
		paramNode := base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind)
		paramTypeID := funType.Params[i]
		if !e.bind(paramNodeID, paramNode.Name.Name, false, paramTypeID, paramNode.Name.Span, -1) {
			return
		}
	}
	if funNode.Name.Name == "main" {
		e.verifyMain(funNode)
	}
	blockTypeID, status := e.queryWithHint(funNode.Block, &funType.Return)
	if status.Failed() {
		return
	}
	blockNode := e.ast.Node(funNode.Block)
	block := base.Cast[ast.Block](blockNode.Kind)
	if blockTypeID == e.neverTyp {
		return
	}
	if !e.isAssignableTo(blockTypeID, funType.Return) {
		diagSpan := blockNode.Span
		if len(block.Exprs) > 0 {
			lastNode := e.ast.Node(block.Exprs[len(block.Exprs)-1])
			diagSpan = lastNode.Span
		}
		e.diag(
			diagSpan,
			"return type mismatch: expected %s, got %s",
			e.env.TypeDisplay(funType.Return),
			e.env.TypeDisplay(blockTypeID),
		)
		return
	}
}

func (e *Engine) checkFunParam(funParam ast.FunParam) (TypeID, TypeStatus) {
	typeID, status := e.Query(funParam.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if funParam.Default != nil {
		if _, ok := e.ast.Node(funParam.Type).Kind.(ast.RefType); ok {
			e.diag(e.ast.Node(*funParam.Default).Span, "default parameters cannot be references")
			return InvalidTypeID, TypeFailed
		}
		defaultTypeID, status := e.queryWithHint(*funParam.Default, &typeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.isAssignableTo(defaultTypeID, typeID) {
			e.diag(e.ast.Node(*funParam.Default).Span,
				"default value type mismatch: expected %s, got %s",
				e.env.TypeDisplay(typeID), e.env.TypeDisplay(defaultTypeID))
			return InvalidTypeID, TypeFailed
		}
	}
	return typeID, TypeOK
}

func (e *Engine) checkFunType(nodeID ast.NodeID, funType ast.FunType, span base.Span) (TypeID, TypeStatus) {
	params := []TypeID{}
	for _, paramNodeID := range funType.ParamTypes {
		paramType, status := e.Query(paramNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		params = append(params, paramType)
	}
	returnType, status := e.Query(funType.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	funTyp := FunType{
		Params:         params,
		Return:         returnType,
		Macro:          false,
		Sync:           funType.Sync == ast.SyncSync,
		Unsafe:         false,
		NoescapeParams: funType.NoescapeParams,
		NoescapeReturn: funType.NoescapeReturn,
	}
	return e.env.buildFunType(funTyp, nodeID, span), TypeOK
}

func (e *Engine) verifyMain(fun ast.Fun) {
	if len(fun.Params) != 0 {
		firstNode := e.ast.Node(fun.Params[0])
		lastNode := e.ast.Node(fun.Params[len(fun.Params)-1])
		span := firstNode.Span.Combine(lastNode.Span)
		e.diag(span, "main function cannot take arguments")
	}
	retTypeID, status := e.Query(fun.ReturnType)
	if status.Failed() {
		return
	}
	if retTypeID == e.voidTyp {
		return
	}
	if union, ok := e.env.Type(retTypeID).Kind.(UnionType); ok {
		if strings.HasPrefix(union.Name, "Result.") && len(union.Variants) > 0 && union.Variants[0] == e.voidTyp {
			return
		}
	}
	e.diag(e.ast.Node(fun.ReturnType).Span, "main function must return void or !void")
}

func (e *Engine) checkIdent(nodeID ast.NodeID, ident ast.Ident, span base.Span) (TypeID, TypeStatus) {
	if ident.Name == "_" {
		e.diag(span, "_ can only be used as the left-hand side of an assignment")
		return InvalidTypeID, TypeFailed
	}
	lookupName := ident.Name
	if structName, methodName, ok := strings.Cut(ident.Name, "."); ok {
		resolved, ok := e.resolveMethodBindName(nodeID, structName, methodName, span)
		if !ok {
			return InvalidTypeID, TypeFailed
		}
		lookupName = resolved
	}
	binding, ok := e.lookup(nodeID, lookupName, e.blockExprsIndex)
	if !ok {
		e.diag(span, "symbol not defined: %s", ident.Name)
		return InvalidTypeID, TypeFailed
	}
	if e.debug.Enabled() {
		e.debug.Print(1, "checkIdent %q -> lookup=%q binding.Decl=%s binding.TypeID=%s type=%s",
			ident.Name, lookupName, binding.Decl, binding.TypeID, e.env.TypeDisplay(binding.TypeID))
	}
	if binding.Decl != 0 && e.unreachableBindingInOuterScope(nodeID, binding) {
		e.diag(span, "cannot reference %q from outer scope", ident.Name)
		return InvalidTypeID, TypeFailed
	}
	e.env.SetPathBinding(nodeID, binding)
	if v, ok := e.ast.Node(binding.Decl).Kind.(ast.EnumVariant); ok {
		e.env.recordEnumVariantRef(nodeID, binding.TypeID, v.Name.Name)
	}
	return e.resolveBinding(nodeID, binding)
}

func (e *Engine) checkFieldAccess(nodeID ast.NodeID, fieldAccess ast.FieldAccess) (TypeID, TypeStatus) {
	targetTypeID, status := e.Query(fieldAccess.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	targetTyp := e.env.Type(targetTypeID)
	if refTyp, ok := targetTyp.Kind.(RefType); ok {
		targetTyp = e.env.Type(refTyp.Type)
	}
	if _, ok := targetTyp.Kind.(ModuleType); ok {
		return e.checkModuleMemberAccess(nodeID, fieldAccess)
	}
	if _, ok := targetTyp.Kind.(SliceType); ok && fieldAccess.Field.Name == "len" {
		return e.intTyp, TypeOK
	}
	if _, ok := targetTyp.Kind.(ArrayType); ok {
		if fieldAccess.Field.Name == "len" {
			return e.intTyp, TypeOK
		}
		e.diag(fieldAccess.Field.Span, "unknown field on array: %s", fieldAccess.Field.Name)
		return InvalidTypeID, TypeFailed
	}
	typeName := e.env.TypeDisplay(targetTyp.ID)
	if struct_, ok := targetTyp.Kind.(StructType); ok {
		for _, field := range struct_.Fields {
			if field.Name == fieldAccess.Field.Name {
				if !e.isVisible(e.env.DeclNode(targetTyp.ID), field.Pub, nodeID) {
					e.diag(fieldAccess.Field.Span, "field %s.%s is not public",
						typeName, field.Name)
					return InvalidTypeID, TypeFailed
				}
				return field.Type, TypeOK
			}
		}
	}
	if enum, ok := targetTyp.Kind.(EnumType); ok {
		if typeID, handled := e.checkEnumFieldAccess(nodeID, fieldAccess, enum, targetTyp.ID); handled {
			return typeID, TypeOK
		}
	}
	if typeID, status, handled := e.resolveGenerics(nodeID, nil); handled {
		return typeID, status
	}
	switch kind := targetTyp.Kind.(type) {
	case StructType, UnionType, IntType, FloatType, BoolType, AllocatorType, SliceType, EnumType:
		e.diag(fieldAccess.Field.Span, "unknown field: %s.%s", typeName, fieldAccess.Field.Name)
	case TypeParamType:
		e.diagTypeParamFieldAccess(kind, fieldAccess, typeName)
	default:
		targetSpan := e.ast.Node(fieldAccess.Target).Span
		e.diag(targetSpan, "cannot access field on non-struct type: %s", typeName)
	}
	return InvalidTypeID, TypeFailed
}

// checkEnumFieldAccess resolves a module-qualified variant reference
// (`ord.Color.red`, where the target is the enum type), a value's generated
// `debug_name`, and associated-data fields. Local `Color.red` is a bound name
// (checkIdent), not a field access. Returns handled=false for any other name.
func (e *Engine) checkEnumFieldAccess(
	nodeID ast.NodeID, fieldAccess ast.FieldAccess, enum EnumType, enumTypeID TypeID,
) (TypeID, bool) {
	field := fieldAccess.Field.Name
	if e.isTypeReference(fieldAccess.Target) {
		if enum.VariantIndex(field) >= 0 {
			e.env.recordEnumVariantRef(nodeID, enumTypeID, field)
			return enumTypeID, true
		}
		return InvalidTypeID, false
	}
	assoc := base.Cast[StructType](e.env.Type(enum.AssociatedDataStruct).Kind)
	for _, f := range assoc.Fields {
		if f.Name == field {
			return f.Type, true
		}
	}
	return InvalidTypeID, false
}

func (e *Engine) diagTypeParamFieldAccess(
	tpt TypeParamType, fieldAccess ast.FieldAccess, typeName string,
) {
	if tpt.Shape == nil {
		e.diag(
			fieldAccess.Field.Span,
			"unconstrained type parameter has no fields or methods: %s", typeName,
		)
		return
	}
	shapeType := base.Cast[ShapeType](e.env.Type(*tpt.Shape).Kind)
	names := make([]string, 0)
	for _, f := range shapeType.Fields {
		names = append(names, f.Name)
	}
	if shapeNodeID := e.env.DeclNode(*tpt.Shape); shapeNodeID != 0 {
		if shapeNode, ok := e.ast.Node(shapeNodeID).Kind.(ast.Shape); ok {
			for _, funDeclNodeID := range shapeNode.Funs {
				funDecl := base.Cast[ast.FunDecl](e.ast.Node(funDeclNodeID).Kind)
				if _, methodName, found := strings.Cut(funDecl.Name.Name, "."); found {
					names = append(names, methodName)
				} else {
					names = append(names, funDecl.Name.Name)
				}
			}
		}
	}
	e.diag(
		fieldAccess.Field.Span,
		"shape %s (constraint on %s) has no member %s; available: %v",
		e.env.TypeDisplay(*tpt.Shape), typeName, fieldAccess.Field.Name, names,
	)
}

func (e *Engine) checkModuleMemberAccess(nodeID ast.NodeID, fieldAccess ast.FieldAccess) (TypeID, TypeStatus) {
	span := e.ast.Node(nodeID).Span
	targetIdent := base.Cast[ast.Ident](e.ast.Node(fieldAccess.Target).Kind)
	moduleName := targetIdent.Name
	memberName := fieldAccess.Field.Name
	thisModuleNode, _ := e.moduleOf(nodeID)
	importedModuleNodeID, ok := e.moduleResolution.Imports[thisModuleNode.ID][moduleName]
	if !ok {
		e.diag(span, "module not found: %s", moduleName)
		return InvalidTypeID, TypeFailed
	}
	mod := base.Cast[ast.Module](e.ast.Node(importedModuleNodeID).Kind)
	if len(mod.Decls) == 0 {
		e.diag(span, "symbol not defined in %s: %s", moduleName, memberName)
		return InvalidTypeID, TypeFailed
	}
	binding, ok := e.env.Lookup(mod.Decls[0], memberName, -1)
	if !ok {
		e.diag(span, "symbol not defined in %s: %s", moduleName, memberName)
		return InvalidTypeID, TypeFailed
	}
	if !e.isVisible(binding.Decl, e.declIsPub(binding.Decl), nodeID) {
		e.diag(span, "%s::%s is not public", moduleName, memberName)
		return InvalidTypeID, TypeFailed
	}
	e.env.SetPathBinding(nodeID, binding)
	return e.resolveBinding(nodeID, binding)
}

func (e *Engine) resolveBinding(nodeID ast.NodeID, binding *Binding) (TypeID, TypeStatus) {
	if cached, ok := e.env.cachedTypeInfo(binding.TypeID); ok && cached.Status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if typeID, status, handled := e.resolveGenerics(nodeID, nil); handled {
		return typeID, status
	}
	return binding.TypeID, TypeOK
}

func (e *Engine) checkCall(call ast.Call, callNodeID ast.NodeID, span base.Span) (TypeID, TypeStatus) { //nolint:funlen
	if _, status, handled := e.resolveGenerics(callNodeID, nil); handled && status.Failed() {
		return InvalidTypeID, status
	}
	calleeTypeID, status := e.Query(call.Callee)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	calleeTyp := e.env.Type(calleeTypeID)
	fun, ok := calleeTyp.Kind.(FunType)
	if !ok {
		calleeSpan := e.ast.Node(call.Callee).Span
		e.diag(calleeSpan, "cannot call non-function: %s", e.env.TypeDisplay(calleeTypeID))
		return InvalidTypeID, TypeFailed
	}
	fieldAccess, isFieldAccess := e.ast.Node(call.Callee).Kind.(ast.FieldAccess)
	// Method calls are named functions reached via field access on a value
	// (not on a module or a type name).
	isMethod := false
	if e.env.isNamedFun(call.Callee) && isFieldAccess {
		targetType := e.env.TypeOfNode(fieldAccess.Target)
		switch targetType.Kind.(type) {
		case ModuleType:
		default:
			isMethod = !e.isTypeReference(fieldAccess.Target)
		}
	}
	if isMethod {
		e.env.setMethodCallReceiver(callNodeID, fieldAccess.Target)
	}
	if hasNamedArgs(call.ArgNames) {
		if status := e.resolveNamedCallArgs(call, callNodeID, isMethod, span); status.Failed() {
			return InvalidTypeID, status
		}
	} else {
		e.fillCallDefaults(callNodeID, call, fun, isMethod)
	}
	argNodes := e.env.CallArgNodes(callNodeID)
	if len(argNodes) != len(fun.Params) {
		expected := len(fun.Params)
		if isMethod {
			expected--
		}
		e.diag(span, "argument count mismatch: expected %d, got %d", expected, len(call.Args))
		return InvalidTypeID, TypeFailed
	}
	for i, argNodeID := range argNodes {
		paramTypeID := fun.Params[i]
		argTypeID, status := e.queryWithHint(argNodeID, &paramTypeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.isAssignableTo(argTypeID, paramTypeID) {
			handled := false
			if i == 0 && isMethod {
				var status TypeStatus
				handled, status = e.tryAutoRefReceiver(callNodeID, argNodeID, argTypeID, paramTypeID)
				if handled && status.Failed() {
					return InvalidTypeID, TypeFailed
				}
			}
			if !handled {
				argNode := e.ast.Node(argNodeID)
				if i == 0 && isMethod {
					e.diag(argNode.Span, "type mismatch at receiver: expected %s, got %s",
						e.env.TypeDisplay(paramTypeID), e.env.TypeDisplay(argTypeID))
				} else {
					argIndex := i
					if isMethod {
						argIndex--
					}
					e.diag(argNode.Span, "type mismatch at argument %d: expected %s, got %s",
						argIndex+1, e.env.TypeDisplay(paramTypeID), e.env.TypeDisplay(argTypeID))
				}
				return InvalidTypeID, TypeFailed
			}
		}
		if _, isRef := e.env.Type(paramTypeID).Kind.(RefType); !isRef {
			if !e.checkNocopy(argNodeID, argTypeID, e.ast.Node(argNodeID).Span) {
				return InvalidTypeID, TypeFailed
			}
		}
	}
	calleeIsUnsafe := fun.Unsafe
	if calleeIsUnsafe && !call.Unsafe {
		e.diag(span, "calling unsafe function requires the unsafe keyword")
		return InvalidTypeID, TypeFailed
	}
	if call.Unsafe && !calleeIsUnsafe {
		e.diag(span, "unsafe keyword can only be used on unsafe functions")
		return InvalidTypeID, TypeFailed
	}
	return fun.Return, TypeOK
}

// resolveNamedCallArgs reorders a call's named/positional arguments into
// parameter order and records that order (and any defaults) for CallArgNodes.
func (e *Engine) resolveNamedCallArgs(
	call ast.Call, callNodeID ast.NodeID, isMethod bool, span base.Span,
) TypeStatus {
	params, builtin, ok := e.calleeParams(call.Callee)
	if !ok {
		e.diag(span, "named arguments are not supported for indirect calls")
		return TypeFailed
	}
	if builtin {
		e.diag(span, "named arguments are not supported for builtin functions")
		return TypeFailed
	}
	userParams := params
	if isMethod {
		userParams = params[1:]
	}
	order, defaults, ok := orderCallArgs(e.ast, userParams, call.Args, call.ArgNames, span, e.diag)
	if !ok {
		return TypeFailed
	}
	e.env.setArgOrder(callNodeID, order)
	if len(defaults) > 0 {
		e.env.setCallDefaults(callNodeID, defaults)
	}
	return TypeOK
}

// funParams returns the parameter nodes declared by an ast.Fun or ast.FunDecl
// (receiver first for methods) and whether it is a builtin. ok is false for any
// other node.
func (c *TypeContext) funParams(declNodeID ast.NodeID) (params []ast.NodeID, builtin bool, ok bool) {
	switch k := c.ast.Node(declNodeID).Kind.(type) {
	case ast.Fun:
		return k.Params, k.Builtin, true
	case ast.FunDecl:
		return k.Params, k.Builtin, true
	default:
		return nil, false, false
	}
}

// calleeParams resolves a directly-called function's parameters via its binding.
// ok is false for indirect calls and other callees with no static parameters.
func (c *TypeContext) calleeParams(callee ast.NodeID) (params []ast.NodeID, builtin bool, ok bool) {
	binding, ok := c.env.LocalPathBinding(callee)
	if !ok || binding.Decl == 0 {
		return nil, false, false
	}
	return c.funParams(binding.Decl)
}

// tryAutoRefReceiver implicitly borrows a method receiver passed by value to a
// `&`/`&mut` receiver parameter, so `x.foo()` works where foo takes `&mut Bar`.
// Only the receiver is auto-borrowed, never a regular argument. handled is false
// when no borrow applies (the caller then reports the plain type mismatch); for a
// `&mut` borrow of an immutable place it reports the error itself and returns a
// failed status.
func (e *Engine) tryAutoRefReceiver(
	callNodeID, recvNodeID ast.NodeID, recvTypeID, paramTypeID TypeID,
) (handled bool, status TypeStatus) {
	refParam, ok := e.env.Type(paramTypeID).Kind.(RefType)
	if !ok {
		return false, TypeOK
	}
	if _, recvIsRef := e.env.Type(recvTypeID).Kind.(RefType); recvIsRef {
		return false, TypeOK
	}
	// Auto-ref binds a reference to the place, so a `&mut` receiver (which can write
	// the place back via `self.*`) needs the place type to equal the pointee
	// exactly, while a read-only `&` receiver only needs it readable as the pointee.
	if refParam.Mut {
		if recvTypeID != refParam.Type {
			return false, TypeOK
		}
	} else if !e.isAssignableTo(recvTypeID, refParam.Type) {
		return false, TypeOK
	}
	if refParam.Mut && !isTemporaryExpr(e.ast.Node(recvNodeID).Kind) {
		if _, mut := e.isPlaceMutable(recvNodeID); !mut {
			e.diag(e.ast.Node(recvNodeID).Span,
				"cannot call a method requiring a mutable receiver on an immutable value")
			return true, TypeFailed
		}
	}
	e.env.setMethodReceiverAutoRef(callNodeID, refParam.Mut)
	return true, TypeOK
}

// fillCallDefaults records the trailing default expressions a positional call
// omits, so CallArgNodes can append them in parameter order.
func (e *Engine) fillCallDefaults(callNodeID ast.NodeID, call ast.Call, fun FunType, isMethod bool) {
	provided := len(call.Args)
	if isMethod {
		provided++
	}
	if provided >= len(fun.Params) {
		return
	}
	defaults := e.funDeclDefaults(call)
	missing := len(fun.Params) - provided
	if missing > len(defaults) {
		return
	}
	e.env.setCallDefaults(callNodeID, defaults[len(defaults)-missing:])
}

// funDeclDefaults returns the default-expression NodeIDs for the trailing
// parameters of the callee, or nil if not applicable.
func (e *Engine) funDeclDefaults(call ast.Call) []ast.NodeID {
	params, _, ok := e.calleeParams(call.Callee)
	if !ok {
		return nil
	}
	var defaults []ast.NodeID
	for _, paramNodeID := range params {
		param := base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind)
		if param.Default != nil {
			defaults = append(defaults, *param.Default)
		}
	}
	return defaults
}

func (e *Engine) checkBool() (TypeID, TypeStatus) {
	return e.boolTyp, TypeOK
}

func (e *Engine) checkInt(intNode ast.Int, span base.Span, typeHint *TypeID) (TypeID, TypeStatus) {
	target := e.intTyp
	if typeHint != nil {
		if _, ok := e.env.Type(*typeHint).Kind.(IntType); ok {
			target = *typeHint
		}
	}
	info := base.Cast[IntType](e.env.Type(target).Kind)
	if intNode.Value.Cmp(info.Min) < 0 || intNode.Value.Cmp(info.Max) > 0 {
		e.diag(span, "value %s out of range for %s (%s..%s)",
			intNode.Value, info.Name, info.Min, info.Max)
		return InvalidTypeID, TypeFailed
	}
	return target, TypeOK
}

func (e *Engine) checkFloat(floatNode ast.Float, span base.Span, typeHint *TypeID) (TypeID, TypeStatus) {
	target := e.floatTyp
	if typeHint != nil {
		if _, ok := e.env.Type(*typeHint).Kind.(FloatType); ok {
			target = *typeHint
		}
	}
	info := base.Cast[FloatType](e.env.Type(target).Kind)
	if info.Bits == 32 && floatNode.F32Value == nil {
		e.diag(span, "value %s out of range for F32", strconv.FormatFloat(*floatNode.F64Value, 'g', -1, 64))
		return InvalidTypeID, TypeFailed
	}
	return target, TypeOK
}

func (e *Engine) checkString(
	nodeID ast.NodeID, str ast.String, span base.Span,
) (TypeID, TypeStatus) { //nolint:unparam
	if str.Bytes {
		return e.env.buildSliceType(e.u8Typ, false, nodeID, span), TypeOK
	}
	return e.strTyp, TypeOK
}

func (e *Engine) checkRuneLiteral(lit ast.RuneLiteral, span base.Span, typeHint *TypeID) (TypeID, TypeStatus) {
	const maxUnicodeCodepoint = 0x10FFFF
	const surrogateMin = 0xD800
	const surrogateMax = 0xDFFF
	if lit.Value > maxUnicodeCodepoint {
		e.diag(span, "invalid rune literal: value U+%04X exceeds maximum Unicode codepoint U+10FFFF", lit.Value)
		return InvalidTypeID, TypeFailed
	}
	if lit.Value >= surrogateMin && lit.Value <= surrogateMax {
		e.diag(span, "invalid rune literal: value U+%04X is in the surrogate range (U+D800..U+DFFF)", lit.Value)
		return InvalidTypeID, TypeFailed
	}
	if typeHint != nil {
		if info, ok := e.env.Type(*typeHint).Kind.(IntType); ok && *typeHint != e.runeTyp {
			val := new(big.Int).SetUint64(uint64(lit.Value))
			if val.Cmp(info.Min) >= 0 && val.Cmp(info.Max) <= 0 {
				return *typeHint, TypeOK
			}
		}
	}
	return e.runeTyp, TypeOK
}

func (e *Engine) checkSimpleType(nodeID ast.NodeID, simpleType ast.SimpleType, span base.Span) (TypeID, TypeStatus) {
	if typeID, status, handled := e.resolveGenerics(nodeID, nil); handled {
		return typeID, status
	}
	if _, ok := e.lookup(nodeID, simpleType.Name.Name, -1); !ok {
		e.diag(span, "symbol not defined: %s", simpleType.Name.Name)
	}
	return InvalidTypeID, TypeFailed
}

func (e *Engine) checkRefType(
	nodeID ast.NodeID, refType ast.RefType, span base.Span,
) (TypeID, TypeStatus) {
	innerTypeID, status := e.Query(refType.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	return e.env.buildRefType(nodeID, innerTypeID, refType.Mut, span), TypeOK
}

func (e *Engine) checkRef(nodeID ast.NodeID, ref ast.Ref, span base.Span) (TypeID, TypeStatus) {
	targetTypeID, status := e.Query(ref.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if ref.Mut && !isTemporaryExpr(e.ast.Node(ref.Target).Kind) {
		_, mut := e.isPlaceMutable(ref.Target)
		if !mut {
			e.diag(span, "cannot take mutable reference to immutable value")
			return InvalidTypeID, TypeFailed
		}
	}
	refTypeID := e.env.buildRefType(nodeID, targetTypeID, ref.Mut, span)
	return refTypeID, TypeOK
}

func (e *Engine) checkDeref(deref ast.Deref) (TypeID, TypeStatus) {
	exprTypeID, status := e.Query(deref.Expr)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	exprTyp := e.env.Type(exprTypeID)
	ref, ok := exprTyp.Kind.(RefType)
	if !ok {
		exprSpan := e.ast.Node(deref.Expr).Span
		e.diag(exprSpan, "dereference: expected reference, got %s", e.env.TypeDisplay(exprTypeID))
		return InvalidTypeID, TypeFailed
	}
	return ref.Type, TypeOK
}

// isTemporaryExpr reports whether a ref's target is a temporary, in which
// case `&expr` materializes into a fresh scope-local slot and is implicitly
// mutable.
func isTemporaryExpr(kind ast.Kind) bool {
	switch kind.(type) {
	case ast.Ident, ast.FieldAccess, ast.Index, ast.Deref:
		return false
	}
	return true
}

func (e *Engine) checkAssign(assign ast.Assign) (TypeID, TypeStatus) {
	if assign.Op == nil {
		if ident, ok := e.ast.Node(assign.LHS).Kind.(ast.Ident); ok && ident.Name == "_" {
			rhsTypeID, status := e.Query(assign.RHS)
			if status.Failed() {
				return InvalidTypeID, TypeDepFailed
			}
			if !e.isInstantiable(rhsTypeID) {
				e.diag(
					e.ast.Node(assign.RHS).Span,
					"cannot discard expression of type %s",
					e.env.TypeDisplay(rhsTypeID),
				)
				return InvalidTypeID, TypeFailed
			}
			return e.voidTyp, TypeOK
		}
	}
	lhsTypeID, status := e.typeOfPlace(assign.LHS)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	// A compound assignment `lhs op= rhs` desugars to `lhs = lhs op rhs`, so the
	// place must be a type the operator accepts: any integer, or a float for the
	// arithmetic operators (floats reject `%`, bitwise, and shifts).
	if assign.Op != nil {
		op := *assign.Op
		arith := op == ast.BinaryOpAdd || op == ast.BinaryOpSub || op == ast.BinaryOpMul || op == ast.BinaryOpDiv
		expected := "an integer"
		valid := e.env.isIntType(lhsTypeID)
		if arith {
			expected = "an integer or float"
			valid = e.env.isNumericType(lhsTypeID)
		}
		if !valid {
			e.diag(e.ast.Node(assign.LHS).Span,
				"compound assignment '%s=' expects %s, got %s",
				op, expected, e.env.TypeDisplay(lhsTypeID))
			return InvalidTypeID, TypeDepFailed
		}
	}
	rhsTypeID, status := e.queryWithHint(assign.RHS, &lhsTypeID)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if !e.isAssignableTo(rhsTypeID, lhsTypeID) {
		span := e.ast.Node(assign.RHS).Span
		e.diag(span, "type mismatch: expected %s, got %s",
			e.env.TypeDisplay(lhsTypeID), e.env.TypeDisplay(rhsTypeID))
		return InvalidTypeID, TypeDepFailed
	}
	if !e.checkNocopy(assign.RHS, rhsTypeID, e.ast.Node(assign.RHS).Span) {
		return InvalidTypeID, TypeFailed
	}
	return e.voidTyp, TypeOK
}

func (e *Engine) checkBinary(binary ast.Binary, typeHint *TypeID) (TypeID, TypeStatus) { //nolint:funlen
	// Arithmetic and bitwise ops yield their operand type, so a numeric hint
	// flows into the operands: a literal expression like `1.0 + 2.0` narrows to
	// the expected type (e.g. F32) just as a bare `1.5` does. Comparison and
	// logical ops yield Bool, so their hint must not reach the numeric operands.
	var operandHint *TypeID
	if typeHint != nil && e.env.isNumericType(*typeHint) {
		switch binary.Op { //nolint:exhaustive
		case ast.BinaryOpAdd, ast.BinaryOpSub, ast.BinaryOpMul, ast.BinaryOpDiv,
			ast.BinaryOpMod, ast.BinaryOpWrapAdd, ast.BinaryOpWrapSub, ast.BinaryOpWrapMul,
			ast.BinaryOpBitAnd, ast.BinaryOpBitOr, ast.BinaryOpBitXor,
			ast.BinaryOpShl, ast.BinaryOpShr:
			operandHint = typeHint
		}
	}
	// When the LHS is a literal but the RHS is not, resolve the RHS first so its
	// concrete type can serve as a type hint for the literal (e.g. `10 == byte`).
	lhsIsLiteral := e.isLiteral(binary.LHS) && !e.isLiteral(binary.RHS)
	var lhsTypeID, rhsTypeID TypeID
	var status TypeStatus
	if lhsIsLiteral {
		rhsTypeID, status = e.queryWithHint(binary.RHS, operandHint)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		lhsTypeID, status = e.queryWithHint(binary.LHS, &rhsTypeID)
	} else {
		lhsTypeID, status = e.queryWithHint(binary.LHS, operandHint)
	}
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	var valid bool
	var expected string
	switch binary.Op {
	case ast.BinaryOpEq, ast.BinaryOpNeq:
		valid = e.env.isNumericType(lhsTypeID) || lhsTypeID == e.boolTyp || e.env.isEnumType(lhsTypeID)
		expected = "an integer, float, or Bool"
	case ast.BinaryOpLt, ast.BinaryOpLte, ast.BinaryOpGt, ast.BinaryOpGte:
		valid = e.env.isNumericType(lhsTypeID)
		expected = "an integer or float"
	case ast.BinaryOpOr, ast.BinaryOpAnd:
		valid = lhsTypeID == e.boolTyp
		expected = "Bool"
	case ast.BinaryOpAdd, ast.BinaryOpSub, ast.BinaryOpMul, ast.BinaryOpDiv:
		valid = e.env.isNumericType(lhsTypeID)
		expected = "an integer or float"
	case ast.BinaryOpMod, ast.BinaryOpWrapAdd, ast.BinaryOpWrapSub, ast.BinaryOpWrapMul:
		valid = e.env.isIntType(lhsTypeID)
		expected = "an integer"
	case ast.BinaryOpBitAnd, ast.BinaryOpBitOr, ast.BinaryOpBitXor, ast.BinaryOpShl, ast.BinaryOpShr:
		valid = e.env.isIntType(lhsTypeID)
		expected = "an integer"
	default:
		panic(base.Errorf("unknown binary operator: %s", binary.Op))
	}
	if !valid {
		e.diag(
			e.ast.Node(binary.LHS).Span,
			"type mismatch: binary operation '%s' expects %s, got %s",
			binary.Op,
			expected,
			e.env.TypeDisplay(lhsTypeID),
		)
		return InvalidTypeID, TypeDepFailed
	}
	if !lhsIsLiteral {
		rhsTypeID, status = e.queryWithHint(binary.RHS, &lhsTypeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
	}
	if rhsTypeID != lhsTypeID && !e.env.sameEnumFamily(lhsTypeID, rhsTypeID) {
		span := e.ast.Node(binary.RHS).Span
		e.diag(
			span,
			"type mismatch: expected type of LHS: %s, got %s",
			e.env.TypeDisplay(lhsTypeID),
			e.env.TypeDisplay(rhsTypeID),
		)
		return InvalidTypeID, TypeDepFailed
	}
	switch binary.Op { //nolint:exhaustive
	case ast.BinaryOpEq, ast.BinaryOpNeq, ast.BinaryOpLt, ast.BinaryOpLte, ast.BinaryOpGt, ast.BinaryOpGte:
		return e.boolTyp, TypeOK
	default:
		return lhsTypeID, TypeOK
	}
}

func (e *Engine) checkUnary(unary ast.Unary, typeHint *TypeID) (TypeID, TypeStatus) {
	// Negation and bitwise-not yield their operand type, so a numeric hint flows
	// into the operand (`-1.5` narrows to F32). Logical not yields Bool.
	var operandHint *TypeID
	if typeHint != nil && e.env.isNumericType(*typeHint) {
		switch unary.Op { //nolint:exhaustive
		case ast.UnaryOpNeg, ast.UnaryOpBitNot:
			operandHint = typeHint
		}
	}
	exprTypeID, status := e.queryWithHint(unary.Expr, operandHint)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	switch unary.Op {
	case ast.UnaryOpNot:
		if exprTypeID != e.boolTyp {
			span := e.ast.Node(unary.Expr).Span
			e.diag(
				span,
				"type mismatch: expected %s, got %s",
				e.env.TypeDisplay(e.boolTyp),
				e.env.TypeDisplay(exprTypeID),
			)
			return InvalidTypeID, TypeDepFailed
		}
		return e.boolTyp, TypeOK
	case ast.UnaryOpBitNot:
		if !e.env.isIntType(exprTypeID) {
			span := e.ast.Node(unary.Expr).Span
			e.diag(
				span,
				"type mismatch: bitwise NOT expects an integer, got %s",
				e.env.TypeDisplay(exprTypeID),
			)
			return InvalidTypeID, TypeDepFailed
		}
		return exprTypeID, TypeOK
	case ast.UnaryOpNeg:
		if e.env.isFloatType(exprTypeID) {
			return exprTypeID, TypeOK
		}
		if intTyp, ok := e.env.Type(exprTypeID).Kind.(IntType); !ok || !intTyp.Signed {
			span := e.ast.Node(unary.Expr).Span
			e.diag(
				span,
				"type mismatch: unary minus expects a signed integer or float, got %s",
				e.env.TypeDisplay(exprTypeID),
			)
			return InvalidTypeID, TypeDepFailed
		}
		return exprTypeID, TypeOK
	default:
		panic(base.Errorf("unknown unary operator: %s", unary.Op))
	}
}

func (e *Engine) isLiteral(nodeID ast.NodeID) bool {
	switch e.ast.Node(nodeID).Kind.(type) {
	case ast.Int, ast.Float, ast.RuneLiteral:
		return true
	}
	return false
}

func (e *Engine) checkVar(nodeID ast.NodeID, varNode ast.Var, span base.Span) (TypeID, TypeStatus) {
	var declTypeID TypeID
	if varNode.Type != nil {
		var status TypeStatus
		declTypeID, status = e.Query(*varNode.Type)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
	}
	var exprTypeID TypeID
	if varNode.Type != nil {
		var status TypeStatus
		exprTypeID, status = e.queryWithHint(varNode.Expr, &declTypeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
	} else {
		var status TypeStatus
		exprTypeID, status = e.Query(varNode.Expr)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
	}
	if !e.isInstantiable(exprTypeID) {
		e.diag(span, "cannot assign expression of type '%s' to a variable", e.env.TypeDisplay(exprTypeID))
		return InvalidTypeID, TypeFailed
	}
	if e.holdsAllocator(exprTypeID) {
		e.diag(span, "allocators must be bound to an @-identifier (e.g. `let @%s = ...`)", varNode.Name.Name)
		return InvalidTypeID, TypeFailed
	}
	if !e.checkNocopy(varNode.Expr, exprTypeID, e.ast.Node(varNode.Expr).Span) {
		return InvalidTypeID, TypeFailed
	}
	bindTypeID := exprTypeID
	// A freshly built array (`[a, b][..]` or `[N of v][..]`) is owned by this
	// binding, so mutating through the slice is safe.
	if varNode.Mut {
		if sliceTyp, ok := e.env.Type(exprTypeID).Kind.(SliceType); ok && !sliceTyp.Mut {
			if subSlice, ok := e.ast.Node(varNode.Expr).Kind.(ast.SubSlice); ok {
				switch e.ast.Node(subSlice.Target).Kind.(type) {
				case ast.ArrayLiteral, ast.ArrayConstruction:
					bindTypeID = e.env.buildSliceType(sliceTyp.Elem, true, nodeID, span)
				}
			}
		}
	}
	if varNode.Type != nil {
		if !e.isAssignableTo(exprTypeID, declTypeID) {
			exprSpan := e.ast.Node(varNode.Expr).Span
			e.diag(exprSpan, "type mismatch: expected %s, got %s",
				e.env.TypeDisplay(declTypeID), e.env.TypeDisplay(exprTypeID))
			return InvalidTypeID, TypeFailed
		}
		bindTypeID = declTypeID
	}
	if !e.bind(nodeID, varNode.Name.Name, varNode.Mut, bindTypeID, varNode.Name.Span, e.blockExprsIndex) {
		return InvalidTypeID, TypeFailed
	}
	return e.voidTyp, TypeOK
}

func (e *Engine) checkAllocatorVar(nodeID ast.NodeID, alloc ast.AllocatorVar, span base.Span) (TypeID, TypeStatus) {
	exprTypeID, status := e.Query(alloc.Expr)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if !e.holdsAllocator(exprTypeID) {
		e.diag(e.ast.Node(alloc.Expr).Span,
			"allocator binding '%s' must be initialized with an allocator, got %s",
			alloc.Name.Name, e.env.TypeDisplay(exprTypeID))
		return InvalidTypeID, TypeFailed
	}
	e.bind(nodeID, alloc.Name.Name, false, exprTypeID, span, e.blockExprsIndex)
	return e.voidTyp, TypeOK
}

// holdsAllocator reports whether the type is an allocator, or a union (e.g.
// `?Arena`, `!Arena`) one of whose variants holds an allocator. Allocator
// capabilities stay marked by an @-identifier even through such wrappers.
func (e *Engine) holdsAllocator(typeID TypeID) bool {
	switch kind := e.env.Type(typeID).Kind.(type) {
	case AllocatorType:
		return true
	case UnionType:
		return slices.ContainsFunc(kind.Variants, e.holdsAllocator)
	}
	return false
}

// checkAllocatorNaming enforces that allocator-holding bindings use an
// @-identifier and that @-identifiers only ever name allocator-holding values.
func (e *Engine) checkAllocatorNaming(name ast.Name, typeID TypeID) bool {
	isAlloc := e.holdsAllocator(typeID)
	hasAt := strings.HasPrefix(name.Name, "@")
	if isAlloc && !hasAt {
		e.diag(name.Span, "allocator '%s' must be bound to an @-identifier", name.Name)
		return false
	}
	if hasAt && !isAlloc {
		e.diag(name.Span, "@-identifier '%s' must hold an allocator, got %s",
			name.Name, e.env.TypeDisplay(typeID))
		return false
	}
	return true
}

func (e *Engine) checkArrayType(nodeID ast.NodeID, array ast.ArrayType, span base.Span) (TypeID, TypeStatus) {
	elemTypeID, status := e.Query(array.Elem)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	return e.env.buildArrayType(elemTypeID, array.Len, nodeID, span), TypeOK
}

func (e *Engine) checkSliceType(nodeID ast.NodeID, sliceType ast.SliceType, span base.Span) (TypeID, TypeStatus) {
	elemTypeID, status := e.Query(sliceType.Elem)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	return e.env.buildSliceType(elemTypeID, sliceType.Mut, nodeID, span), TypeOK
}

func (e *Engine) checkArrayLiteral(
	nodeID ast.NodeID, array ast.ArrayLiteral, span base.Span, typeHint *TypeID,
) (TypeID, TypeStatus) {
	if len(array.Elems) == 0 {
		e.diag(span, "array literal cannot be empty")
		return InvalidTypeID, TypeFailed
	}
	// An array-typed hint flows its element type into every element (so a `U8`
	// array literal need not write `U8(..)` on the first element); otherwise the
	// first element seeds the type.
	var elemTyp TypeID
	hinted := false
	if typeHint != nil {
		if arr, ok := e.env.Type(*typeHint).Kind.(ArrayType); ok {
			elemTyp = arr.Elem
			hinted = true
		}
	}
	if !hinted {
		seed, status := e.Query(array.Elems[0])
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		elemTyp = seed
	}
	for _, elemNodeID := range array.Elems {
		elemTyp2, status := e.queryWithHint(elemNodeID, &elemTyp)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.isAssignableTo(elemTyp2, elemTyp) {
			e.diag(
				e.ast.Node(elemNodeID).Span,
				"array literal element type mismatch: expected %s, got %s",
				e.env.TypeDisplay(elemTyp),
				e.env.TypeDisplay(elemTyp2),
			)
			return InvalidTypeID, TypeFailed
		}
		if !e.checkNocopy(elemNodeID, elemTyp2, e.ast.Node(elemNodeID).Span) {
			return InvalidTypeID, TypeFailed
		}
	}
	typeID := e.env.buildArrayType(elemTyp, int64(len(array.Elems)), nodeID, span)
	// Promote to a shared immutable global only when the value is genuinely
	// compile-time constant. A runtime-valued array must not share storage, two
	// evaluations would alias the same global and clobber each other's data.
	if e.isConstExpr(nodeID) && !e.env.containsMutablePart(typeID) {
		e.env.constArrays[nodeID] = true
	}
	return typeID, TypeOK
}

func (e *Engine) checkEmptySlice(span base.Span, typeHint *TypeID) (TypeID, TypeStatus) {
	if typeHint != nil {
		if _, ok := e.env.Type(*typeHint).Kind.(SliceType); ok {
			return *typeHint, TypeOK
		}
	}
	e.diag(span, "cannot infer type of empty slice []")
	return InvalidTypeID, TypeFailed
}

func (e *Engine) checkIndex(index ast.Index) (TypeID, TypeStatus) {
	targetTypeID, status := e.Query(index.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	targetTyp := e.env.Type(targetTypeID)
	if refTyp, ok := targetTyp.Kind.(RefType); ok {
		targetTyp = e.env.Type(refTyp.Type)
	}
	var elemTypeID TypeID
	switch kind := targetTyp.Kind.(type) {
	case ArrayType:
		elemTypeID = kind.Elem
	case SliceType:
		elemTypeID = kind.Elem
	default:
		targetSpan := e.ast.Node(index.Target).Span
		e.diag(targetSpan, "not an array or slice: %s", e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
	indexTypeID, status := e.Query(index.Index)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if indexTypeID != e.intTyp {
		indexSpan := e.ast.Node(index.Index).Span
		e.diag(
			indexSpan,
			"index type mismatch: expected %s, got %s",
			e.env.TypeDisplay(e.intTyp),
			e.env.TypeDisplay(indexTypeID),
		)
		return InvalidTypeID, TypeFailed
	}
	return elemTypeID, TypeOK
}

func (e *Engine) checkSubSlice(nodeID ast.NodeID, subSlice ast.SubSlice) (TypeID, TypeStatus) {
	targetTypeID, status := e.Query(subSlice.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	targetTyp := e.env.Type(targetTypeID)
	var throughRef *RefType
	if refTyp, ok := targetTyp.Kind.(RefType); ok {
		throughRef = &refTyp
		targetTyp = e.env.Type(refTyp.Type)
	}
	var elemTypeID TypeID
	var mut bool
	switch kind := targetTyp.Kind.(type) {
	case ArrayType:
		elemTypeID = kind.Elem
		if throughRef != nil {
			mut = throughRef.Mut
		} else {
			_, mut = e.isPlaceMutable(subSlice.Target)
		}
	case SliceType:
		elemTypeID = kind.Elem
		if throughRef != nil {
			mut = throughRef.Mut && kind.Mut
		} else {
			mut = kind.Mut
		}
	default:
		targetSpan := e.ast.Node(subSlice.Target).Span
		e.diag(targetSpan, "not an array or slice: %s", e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
	if _, status := e.Query(subSlice.Range); status.Failed() {
		return InvalidTypeID, status
	}
	span := e.ast.Node(nodeID).Span
	return e.env.buildSliceType(elemTypeID, mut, nodeID, span), TypeOK
}

func (e *Engine) checkTypeConstruction(
	nodeID ast.NodeID, lit ast.TypeConstruction, span base.Span, typeHint *TypeID,
) (TypeID, TypeStatus) {
	if typeID, status, handled := e.resolveGenerics(nodeID, typeHint); handled {
		if status.Failed() {
			return InvalidTypeID, status
		}
		return e.dispatchTypeConstruction(nodeID, typeID, lit, span)
	}
	targetTypeID, status := e.Query(lit.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	resultID, status := e.dispatchTypeConstruction(nodeID, targetTypeID, lit, span)
	if name, ok := e.underInstantiatedGeneric(targetTypeID); ok && !status.Failed() {
		// Inference resolved fewer type arguments than the generic declares (e.g. a
		// param used by no field), so an abstract generic reaches here. Reject it,
		// or codegen tries to size an uninstantiated type.
		e.diag(span, "cannot infer type arguments for %s", name)
		return InvalidTypeID, TypeFailed
	}
	return resultID, status
}

// underInstantiatedGeneric reports whether typeID is a generic struct or union
// constructed with fewer type arguments than its declaration has type
// parameters, i.e. some parameter was never pinned down. The bool is the
// declaration's name, for the diagnostic.
func (e *Engine) underInstantiatedGeneric(typeID TypeID) (string, bool) {
	declNode := e.env.DeclNode(typeID)
	if declNode == 0 {
		return "", false
	}
	switch decl := e.ast.Node(declNode).Kind.(type) {
	case ast.Struct:
		st, ok := e.env.Type(typeID).Kind.(StructType)
		return decl.Name.Name, ok && len(st.TypeArgs) < len(decl.TypeParams)
	case ast.Union:
		ut, ok := e.env.Type(typeID).Kind.(UnionType)
		return decl.Name.Name, ok && len(ut.TypeArgs) < len(decl.TypeParams)
	}
	return "", false
}

func (e *Engine) dispatchTypeConstruction(
	nodeID ast.NodeID, targetTypeID TypeID, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus) {
	targetTyp := e.env.Type(targetTypeID)
	if _, isStruct := targetTyp.Kind.(StructType); !isStruct && hasNamedArgs(lit.ArgNames) {
		e.diag(span, "named arguments are only supported when constructing a struct")
		return InvalidTypeID, TypeFailed
	}
	switch kind := targetTyp.Kind.(type) {
	case IntType:
		if kind.Name == "Rune" && !ast.IsPreludeNode(nodeID) {
			e.diag(span, "Rune cannot be constructed directly; use Rune.from_u32_lossy() instead")
			return InvalidTypeID, TypeFailed
		}
		return e.checkIntConstruction(nodeID, kind, targetTypeID, lit, span)
	case FloatType:
		return e.checkFloatConstruction(targetTypeID, lit, span)
	case StructType:
		if kind.Name == "Str" && !ast.IsPreludeNode(nodeID) {
			e.diag(span, "Str cannot be constructed directly; use Str.from_utf8_lossy() instead")
			return InvalidTypeID, TypeFailed
		}
		return e.checkStructConstruction(nodeID, kind, targetTypeID, lit, span)
	case UnionType:
		return e.checkUnionConstruction(kind, targetTypeID, lit, span)
	case AllocatorType:
		if len(lit.Args) != 0 {
			e.diag(span, "argument count mismatch: expected %d, got %d", 0, len(lit.Args))
			return InvalidTypeID, TypeFailed
		}
		return targetTypeID, TypeOK
	default:
		calleeSpan := e.ast.Node(lit.Target).Span
		e.diag(calleeSpan, "not a struct or union: %s", e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
}

// orderConstructionArgs returns the construction's arguments in field order.
// For a positional construction it returns lit.Args unchanged; for a named one
// it reorders by field name (every field is required, structs have no defaults)
// and records the order for the backend.
func (e *Engine) orderConstructionArgs(
	nodeID ast.NodeID, lit ast.TypeConstruction, fields []StructField, span base.Span,
) ([]ast.NodeID, bool) {
	if !hasNamedArgs(lit.ArgNames) {
		return lit.Args, true
	}
	fieldNames := make([]string, len(fields))
	for i, f := range fields {
		fieldNames[i] = f.Name
	}
	slots, ok := matchArgs(e.ast, fieldNames, "field", lit.Args, lit.ArgNames, span, e.diag)
	if !ok {
		return nil, false
	}
	for i, slot := range slots {
		if slot == 0 {
			e.diag(span, "missing argument for field: %s", fieldNames[i])
			return nil, false
		}
	}
	e.env.setArgOrder(nodeID, slots)
	return slots, true
}

func (e *Engine) checkStructConstruction(
	nodeID ast.NodeID, struct_ StructType, structTypeID TypeID, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus) {
	structDeclNode := e.env.DeclNode(structTypeID)
	for _, field := range struct_.Fields {
		if !e.isVisible(structDeclNode, field.Pub, lit.Target) {
			e.diag(span, "cannot construct %s from outside its module: field %s is not public",
				e.env.TypeDisplay(structTypeID), field.Name)
			return InvalidTypeID, TypeFailed
		}
	}
	args, ok := e.orderConstructionArgs(nodeID, lit, struct_.Fields, span)
	if !ok {
		return InvalidTypeID, TypeFailed
	}
	if len(args) != len(struct_.Fields) {
		e.diag(span, "argument count mismatch: expected %d, got %d", len(struct_.Fields), len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	for i, argNodeID := range args {
		argNode := e.ast.Node(argNodeID)
		fieldType := struct_.Fields[i].Type
		argTypeID, status := e.queryWithHint(argNodeID, &fieldType)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.isAssignableTo(argTypeID, struct_.Fields[i].Type) {
			e.diag(
				argNode.Span,
				"type mismatch at argument %d: expected %s, got %s",
				i+1,
				e.env.TypeDisplay(struct_.Fields[i].Type),
				e.env.TypeDisplay(argTypeID),
			)
			return InvalidTypeID, TypeFailed
		}
		if _, isRef := e.env.Type(struct_.Fields[i].Type).Kind.(RefType); !isRef {
			if !e.checkNocopy(argNodeID, argTypeID, argNode.Span) {
				return InvalidTypeID, TypeFailed
			}
		}
	}
	return structTypeID, TypeOK
}

func (e *Engine) checkUnionConstruction(
	union UnionType, unionTypeID TypeID, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus) {
	if len(lit.Args) != 1 {
		e.diag(span, "union constructor takes exactly 1 argument, got %d", len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	argNodeID := lit.Args[0]
	argTypeID, status := e.Query(argNodeID)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	for _, variantTypeID := range union.Variants {
		if e.isAssignableTo(argTypeID, variantTypeID) {
			if _, isRef := e.env.Type(variantTypeID).Kind.(RefType); !isRef {
				if !e.checkNocopy(argNodeID, argTypeID, e.ast.Node(argNodeID).Span) {
					return InvalidTypeID, TypeFailed
				}
			}
			return unionTypeID, TypeOK
		}
	}
	e.diag(
		e.ast.Node(argNodeID).Span,
		"type %s is not a variant of %s",
		e.env.TypeDisplay(argTypeID),
		e.env.TypeDisplay(unionTypeID),
	)
	return InvalidTypeID, TypeFailed
}

func (e *Engine) checkArrayConstruction(
	nodeID ast.NodeID, ac ast.ArrayConstruction, span base.Span, typeHint *TypeID,
) (TypeID, TypeStatus) {
	if ac.Elem != nil {
		elemTypeID, status := e.Query(*ac.Elem)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !ac.Unsafe {
			e.diag(span, "uninitialized array requires unsafe: write [N of v] to fill it")
			return InvalidTypeID, TypeFailed
		}
		return e.env.buildArrayType(elemTypeID, ac.Len, nodeID, span), TypeOK
	}
	if ac.Unsafe {
		e.diag(span, "unsafe applies only to an uninitialized array [N uninit T]")
		return InvalidTypeID, TypeFailed
	}
	// An array-typed hint flows its element type into the fill value, so
	// `[32 of 1]` against a `[32]U8` field fills with `U8`, not `Int`.
	var elemHint *TypeID
	if typeHint != nil {
		if arr, ok := e.env.Type(*typeHint).Kind.(ArrayType); ok {
			h := arr.Elem
			elemHint = &h
		}
	}
	valTypeID, status := e.queryWithHint(*ac.Fill, elemHint)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if !e.isCopyable(valTypeID) {
		e.diag(span, "cannot fill an array with nocopy type %s; use unsafe [N uninit T] and set each element",
			e.env.TypeDisplay(valTypeID))
		return InvalidTypeID, TypeFailed
	}
	return e.env.buildArrayType(valTypeID, ac.Len, nodeID, span), TypeOK
}

func (e *Engine) checkIntConstruction(
	nodeID ast.NodeID, targetTyp IntType, targetTypeID TypeID, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus) {
	if len(lit.Args) != 1 {
		e.diag(span, "%s() takes exactly 1 argument, got %d", targetTyp.Name, len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	argNodeID := lit.Args[0]
	argTypeID, status := e.queryWithHint(argNodeID, &targetTypeID)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if argTypeID != targetTypeID {
		if targetTyp.Name == "Rune" && ast.IsPreludeNode(nodeID) {
			argTyp := e.env.Type(argTypeID)
			if argIntTyp, ok := argTyp.Kind.(IntType); ok && argIntTyp.Bits == targetTyp.Bits {
				return targetTypeID, TypeOK
			}
		}
		argSpan := e.ast.Node(argNodeID).Span
		e.diag(argSpan, "cannot use %s as %s; use conversion methods instead",
			e.env.TypeDisplay(argTypeID), e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
	return targetTypeID, TypeOK
}

func (e *Engine) checkFloatConstruction(
	targetTypeID TypeID, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus) {
	targetTyp := base.Cast[FloatType](e.env.Type(targetTypeID).Kind)
	if len(lit.Args) != 1 {
		e.diag(span, "%s() takes exactly 1 argument, got %d", targetTyp.Name, len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	argNodeID := lit.Args[0]
	argTypeID, status := e.queryWithHint(argNodeID, &targetTypeID)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if argTypeID != targetTypeID {
		argSpan := e.ast.Node(argNodeID).Span
		e.diag(argSpan, "cannot use %s as %s; use conversion methods instead",
			e.env.TypeDisplay(argTypeID), e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
	return targetTypeID, TypeOK
}

func (e *Engine) checkStructField(structField ast.StructField) (TypeID, TypeStatus) {
	typeID, status := e.Query(structField.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if !e.checkAllocatorNaming(structField.Name, typeID) {
		return InvalidTypeID, TypeFailed
	}
	return typeID, TypeOK
}

func (e *Engine) typeOfPlace(nodeID ast.NodeID) (TypeID, TypeStatus) {
	typeID, mut := e.isPlaceMutable(nodeID)
	if typeID == InvalidTypeID {
		return InvalidTypeID, TypeFailed
	}
	if mut {
		return typeID, TypeOK
	}
	node := e.ast.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.Ident:
		e.diag(node.Span, "cannot assign to immutable variable: %s", kind.Name)
	case ast.Deref:
		exprTypeID, _ := e.Query(kind.Expr)
		e.diag(node.Span, "cannot assign through dereference: expected mutable reference, got %s",
			e.env.TypeDisplay(exprTypeID))
	case ast.FieldAccess:
		targetTypeID, _ := e.Query(kind.Target)
		var containerMut bool
		if ref, ok := e.env.Type(targetTypeID).Kind.(RefType); ok {
			containerMut = ref.Mut
		} else {
			_, containerMut = e.isPlaceMutable(kind.Target)
		}
		if containerMut {
			e.diag(node.Span, "cannot assign to non-public field: %s", kind.Field.Name)
		} else {
			e.diag(node.Span, "cannot assign to field of immutable value")
		}
	case ast.Index:
		e.diag(node.Span, "cannot assign to element of immutable array or slice")
	default:
		e.diag(node.Span, "cannot assign to this expression, which is not a variable, field, or element")
	}
	return InvalidTypeID, TypeFailed
}

func (e *Engine) isPlaceMutable(nodeID ast.NodeID) (TypeID, bool) { //nolint:funlen
	typeID, status := e.Query(nodeID)
	if status.Failed() {
		return InvalidTypeID, false
	}
	node := e.ast.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.Ident:
		binding, ok := e.lookup(nodeID, kind.Name, -1)
		if !ok {
			return InvalidTypeID, false
		}
		return typeID, binding.Mut
	case ast.Deref:
		exprTypeID, status := e.Query(kind.Expr)
		if status.Failed() {
			return InvalidTypeID, false
		}
		ref := base.Cast[RefType](e.env.Type(exprTypeID).Kind)
		return typeID, ref.Mut
	case ast.Index:
		targetTypeID, status := e.Query(kind.Target)
		if status.Failed() {
			return InvalidTypeID, false
		}
		targetTyp := e.env.Type(targetTypeID)
		var throughRef *RefType
		if ref, ok := targetTyp.Kind.(RefType); ok {
			throughRef = &ref
			targetTyp = e.env.Type(ref.Type)
		}
		switch k := targetTyp.Kind.(type) {
		case ArrayType:
			var mut bool
			if throughRef != nil {
				mut = throughRef.Mut
			} else {
				_, mut = e.isPlaceMutable(kind.Target)
			}
			return k.Elem, mut
		case SliceType:
			// Element mutability comes from the slice's Mut flag, not the
			// binding's mut. Through a ref, both must be mutable.
			if throughRef != nil {
				return k.Elem, throughRef.Mut && k.Mut
			}
			return k.Elem, k.Mut
		default:
			return InvalidTypeID, false
		}
	case ast.FieldAccess:
		targetTypeID, status := e.Query(kind.Target)
		if status.Failed() {
			return InvalidTypeID, false
		}
		var containerMut bool
		var structTypeID TypeID
		if ref, ok := e.env.Type(targetTypeID).Kind.(RefType); ok {
			containerMut = ref.Mut
			structTypeID = ref.Type
		} else {
			_, containerMut = e.isPlaceMutable(kind.Target)
			structTypeID = targetTypeID
		}
		if !containerMut {
			return typeID, false
		}
		var fields []StructField
		switch kind := e.env.Type(structTypeID).Kind.(type) {
		case StructType:
			fields = kind.Fields
		case TypeParamType:
			if kind.Shape == nil {
				panic(base.Errorf("expected type parameter to be constrained by shape"))
			}
			shape := base.Cast[ShapeType](e.env.Type(*kind.Shape).Kind)
			fields = shape.Fields
		default:
			panic(base.Errorf("expected struct or shape, got: %T", kind))
		}
		for _, field := range fields {
			if field.Name == kind.Field.Name {
				return typeID, e.isVisible(e.env.DeclNode(structTypeID), field.Pub, nodeID)
			}
		}
		return typeID, false
	default:
		return typeID, false
	}
}

func (e *Engine) tryUnionAutoWrap(nodeID ast.NodeID, typeID TypeID, hintTypeID TypeID) TypeID {
	if typeID == e.neverTyp {
		return typeID
	}
	unionType, ok := e.env.Type(hintTypeID).Kind.(UnionType)
	if !ok {
		return typeID
	}
	for i, variantID := range unionType.Variants {
		if e.isAssignableTo(typeID, variantID) {
			e.env.recordUnionWrap(nodeID, hintTypeID, i)
			return hintTypeID
		}
	}
	return typeID
}

func (e *Engine) isInstantiable(t TypeID) bool {
	return t != e.neverTyp
}

func (e *Engine) registerStruct(structType StructType, nodeID ast.NodeID, typeID TypeID) {
	structNode, ok := e.ast.Node(nodeID).Kind.(ast.Struct)
	if !ok || structNode.Builtin {
		return
	}
	if e.skipRegisterWork {
		return
	}
	if _, ok := e.loadStructWork(structType.Name); !ok {
		e.recordStructWork(structType.Name, TypeWork{NodeID: nodeID, TypeID: typeID, Env: e.env})
	}
}

func (e *Engine) registerUnion(unionType UnionType, nodeID ast.NodeID, typeID TypeID) {
	if e.skipRegisterWork {
		return
	}
	if _, ok := e.loadUnionWork(unionType.Name); !ok {
		e.recordUnionWork(unionType.Name, TypeWork{NodeID: nodeID, TypeID: typeID, Env: e.env})
	}
}

func (e *Engine) fixPreludeType(node *ast.Node, typ *cachedType) {
	structNode := base.Cast[ast.Struct](node.Kind)
	switch structNode.Name.Name {
	case "Arena":
		typ.Type.Kind = AllocatorType{AllocatorArena}
		e.arenaTyp = typ.Type.ID
	case "never":
		typ.Type.Kind = NeverType{}
		e.neverTyp = typ.Type.ID
	case "void":
		typ.Type.Kind = VoidType{}
		e.voidTyp = typ.Type.ID
	case "Bool":
		typ.Type.Kind = BoolType{}
		e.boolTyp = typ.Type.ID
	case "Str":
		e.strTyp = typ.Type.ID
	case "Range":
		e.rangeTyp = typ.Type.ID
	default:
		for _, intTyp := range intTypes {
			if intTyp.Name == structNode.Name.Name {
				typ.Type.Kind = intTyp
				if intTyp.Name == "Int" {
					e.intTyp = typ.Type.ID
				}
				if intTyp.Name == "Rune" {
					e.runeTyp = typ.Type.ID
				}
				if intTyp.Name == "U8" {
					e.u8Typ = typ.Type.ID
				}
			}
		}
		for _, floatTyp := range floatTypes {
			if floatTyp.Name == structNode.Name.Name {
				typ.Type.Kind = floatTyp
				if floatTyp.Name == "Float" {
					e.floatTyp = typ.Type.ID
				}
				if floatTyp.Name == "F32" {
					e.f32Typ = typ.Type.ID
				}
			}
		}
	}
}

// isCopyable reports whether a value of the given type can be copied.
// A struct/union becomes non-copyable when marked `nocopy` or when any of
// its fields/variants is non-copyable.
func (e *Engine) isCopyable(typeID TypeID) bool {
	// visit guards the recursive walk against a value type of infinite size (a
	// recursive type, diagnosed separately when types are completed): a type
	// already on the path returns true so the walk terminates instead of
	// overflowing. The per-type nocopy check runs first, so a nocopy recursive
	// type is still reported non-copyable.
	var visit func(typeID TypeID, visiting map[TypeID]bool) bool
	visit = func(typeID TypeID, visiting map[TypeID]bool) bool {
		switch kind := e.env.Type(typeID).Kind.(type) {
		case VoidType, NeverType, BoolType, IntType, FloatType:
			return true
		case StructType:
			if declNode := e.env.DeclNode(typeID); declNode != 0 {
				if s, ok := e.ast.Node(declNode).Kind.(ast.Struct); ok && s.Nocopy {
					return false
				}
			}
			if visiting[typeID] {
				return true
			}
			visiting[typeID] = true
			for _, field := range kind.Fields {
				if !visit(field.Type, visiting) {
					return false
				}
			}
			return true
		case UnionType:
			if declNode := e.env.DeclNode(typeID); declNode != 0 {
				if u, ok := e.ast.Node(declNode).Kind.(ast.Union); ok && u.Nocopy {
					return false
				}
			}
			if visiting[typeID] {
				return true
			}
			visiting[typeID] = true
			for _, variant := range kind.Variants {
				if !visit(variant, visiting) {
					return false
				}
			}
			return true
		case ArrayType:
			return visit(kind.Elem, visiting)
		case RefType, SliceType, FunType, AllocatorType, TypeParamType, ShapeType, EnumType:
			return true
		default:
			panic(fmt.Sprintf("isCopyable: unknown type: %T", kind))
		}
	}
	return visit(typeID, map[TypeID]bool{})
}

// isFreshValue reports whether the expression creates a new value rather
// than reading from an existing binding.
func (e *Engine) isFreshValue(nodeID ast.NodeID) bool {
	node := e.ast.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.TypeConstruction:
		return true
	case ast.Call:
		return true
	case ast.AllocatorVar:
		return e.isFreshValue(kind.Expr)
	case ast.Block:
		if len(kind.Exprs) > 0 {
			return e.isFreshValue(kind.Exprs[len(kind.Exprs)-1])
		}
		return true
	case ast.If:
		return e.isFreshValue(kind.Then) && (kind.Else == nil || e.isFreshValue(*kind.Else))
	case ast.ArrayLiteral:
		for _, elem := range kind.Elems {
			if !e.isFreshValue(elem) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (e *Engine) isConstExpr(nodeID ast.NodeID) bool {
	// A bare enum variant (e.g. `Color.red`) is a constant discriminant.
	if _, _, ok := e.env.EnumVariantRef(nodeID); ok {
		return true
	}
	// A reference to a module-level constant, possibly qualified (`mod.x`).
	if e.resolvesToModuleConst(nodeID) {
		return true
	}
	switch kind := e.ast.Node(nodeID).Kind.(type) {
	case ast.Int, ast.Float, ast.Bool, ast.String, ast.RuneLiteral:
		return true
	case ast.Unary:
		return e.isConstExpr(kind.Expr)
	case ast.Binary:
		return e.isConstExpr(kind.LHS) && e.isConstExpr(kind.RHS)
	case ast.Ref:
		// `&` of a const place is a static address; `&mut` is never const.
		return !kind.Mut && e.isConstExpr(kind.Target)
	case ast.FieldAccess:
		// A field read of a const value (`pt.x`, `Level.low.weight`).
		return e.isConstExpr(kind.Target)
	case ast.Index:
		return e.isConstExpr(kind.Target) && e.isConstExpr(kind.Index)
	case ast.SubSlice:
		return e.isConstExpr(kind.Target) && e.isConstExpr(kind.Range)
	case ast.Range:
		return (kind.Lo == nil || e.isConstExpr(*kind.Lo)) &&
			(kind.Hi == nil || e.isConstExpr(*kind.Hi))
	case ast.TypeConstruction:
		for _, arg := range kind.Args {
			if !e.isConstExpr(arg) {
				return false
			}
		}
		return true
	case ast.ArrayLiteral:
		for _, elem := range kind.Elems {
			if !e.isConstExpr(elem) {
				return false
			}
		}
		return true
	case ast.ArrayConstruction:
		// `[N of v]` is const iff v is; `[N uninit T]` is uninitialized.
		return kind.Fill != nil && e.isConstExpr(*kind.Fill)
	default:
		return false
	}
}

// resolvesToModuleConst reports whether nodeID is a reference (an identifier or
// qualified `mod.x`) bound to a module-level `let`.
func (e *Engine) resolvesToModuleConst(nodeID ast.NodeID) bool {
	binding, ok := e.env.PathBinding(nodeID)
	if !ok || binding.Decl == 0 {
		return false
	}
	if _, isVar := e.ast.Node(binding.Decl).Kind.(ast.Var); !isVar {
		return false
	}
	scope := e.scopeGraph.NodeScope(binding.Decl)
	if scope.Node == 0 {
		return false
	}
	_, atModule := e.ast.Node(scope.Node).Kind.(ast.Module)
	return atModule
}

// checkNocopy verifies that a nocopy value is not being copied. Returns true
// when the type is copyable or the expression produces a fresh value.
func (e *Engine) checkNocopy(exprNodeID ast.NodeID, exprTypeID TypeID, span base.Span) bool {
	if e.isCopyable(exprTypeID) {
		return true
	}
	if e.isFreshValue(exprNodeID) {
		return true
	}
	e.diag(span, "cannot copy value of nocopy type %s", e.env.TypeDisplay(exprTypeID))
	return false
}

func (e *Engine) bindCapture(funNodeID, capNodeID ast.NodeID, capture ast.Capture) {
	// The Fun node lives in the enclosing scope, so looking up from there
	// resolves the captured name in the outer scope.
	outerBinding, ok := e.lookup(funNodeID, capture.Name.Name, -1)
	if !ok {
		e.diag(capture.Name.Span, "capture: symbol not defined: %s", capture.Name.Name)
		return
	}
	outerTypeID := outerBinding.TypeID
	if _, ok := e.env.Type(outerTypeID).Kind.(AllocatorType); ok && capture.Mode != ast.CaptureByValue {
		e.diag(capture.Name.Span, "allocator captures must be by value, not by reference")
		return
	}
	var captureTypeID TypeID
	switch capture.Mode {
	case ast.CaptureByValue:
		captureTypeID = outerTypeID
		if !e.checkNocopy(capNodeID, outerTypeID, capture.Name.Span) {
			return
		}
	case ast.CaptureByRef:
		captureTypeID = e.env.buildRefType(capNodeID, outerTypeID, false, capture.Name.Span)
	case ast.CaptureByMutRef:
		if !outerBinding.Mut {
			e.diag(capture.Name.Span, "cannot take mutable reference to immutable value")
			return
		}
		captureTypeID = e.env.buildRefType(capNodeID, outerTypeID, true, capture.Name.Span)
	}
	e.bind(capNodeID, capture.Name.Name, false, captureTypeID, capture.Name.Span, -1)
	e.env.captureOrigins[capNodeID] = outerBinding
}

func (e *Engine) unreachableBindingInOuterScope(nodeID ast.NodeID, binding *Binding) bool {
	switch e.ast.Node(binding.Decl).Kind.(type) {
	case ast.Var:
		// Module-level constants are reachable from any function.
		bindingScope := e.scopeGraph.NodeScope(binding.Decl)
		if bindingScope.Node == 0 {
			return false
		}
		if _, ok := e.ast.Node(bindingScope.Node).Kind.(ast.Module); ok {
			return false
		}
	case ast.FunParam, ast.AllocatorVar:
	default:
		return false
	}
	scope := e.scopeGraph.NodeScope(nodeID)
	bindingScope := e.scopeGraph.NodeScope(binding.Decl)
	for scope != nil && scope.ID != bindingScope.ID {
		if fun, ok := e.ast.Node(scope.Node).Kind.(ast.Fun); ok {
			captured := false
			for _, capNodeID := range fun.Captures {
				capture := base.Cast[ast.Capture](e.ast.Node(capNodeID).Kind)
				if capture.Name.Name == binding.Name {
					captured = true
					break
				}
			}
			if !captured {
				return true
			}
		}
		scope = scope.Parent
	}
	return false
}

// isFunDeclSync determines if a function declaration is sync. The resolved
// types are passed in because function literals may have inferred types not
// present as AST nodes.
func (c *TypeContext) isFunDeclSync(node *ast.Node, paramTypeIDs []TypeID, retTypeID TypeID) bool {
	funNode, ok := node.Kind.(ast.Fun)
	if !ok {
		return false
	}
	if funNode.Sync == ast.SyncUnsync {
		return false
	}
	for _, capNodeID := range funNode.Captures {
		capture := base.Cast[ast.Capture](c.ast.Node(capNodeID).Kind)
		if capture.Mode != ast.CaptureByValue {
			return false
		}
		outerBinding, ok := c.lookup(node.ID, capture.Name.Name, -1)
		if !ok {
			return false
		}
		if !c.isSync(outerBinding.TypeID) {
			return false
		}
	}
	for _, paramTypeID := range paramTypeIDs {
		if !c.isSync(paramTypeID) {
			return false
		}
	}
	return c.isSync(retTypeID)
}

func (c *TypeContext) resolveMethodBindName(
	nodeID ast.NodeID, structName, methodName string, span base.Span,
) (string, bool) {
	binding, ok := c.lookup(nodeID, structName, -1)
	if !ok {
		c.diag(span, "method receiver type not found: %s", structName)
		return "", false
	}
	fqn, ok := c.env.methodFQN(c.env.Type(binding.TypeID), methodName)
	if !ok {
		c.diag(span, "type %s cannot have methods", structName)
		return "", false
	}
	return fqn, true
}
