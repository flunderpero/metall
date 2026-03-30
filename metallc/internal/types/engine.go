package types

import (
	"math/big"
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
	*EngineCore
	moduleResolution *modules.ModuleResolution
	macroExpander    MacroExpander

	loopStack          []ast.NodeID
	funStack           []TypeID
	typeHint           *TypeID
	instantiationScope *ast.NodeID
	voidTyp            TypeID
	boolTyp            TypeID
	strTyp             TypeID
	runeTyp            TypeID
	arenaTyp           TypeID
	neverTyp           TypeID
	intTyp             TypeID
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
	c := NewEngineCore(merged, g)
	e := &Engine{ //nolint:exhaustruct
		EngineCore:       c,
		moduleResolution: moduleResolution,
		macroExpander:    macroExpander,
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
	var result []FunWork
	for _, fw := range e.funs {
		funTyp := base.Cast[FunType](e.env.Type(fw.TypeID).Kind)
		if !e.env.hasTypeParam(funTyp.Params) && !e.env.hasTypeParam([]TypeID{funTyp.Return}) {
			result = append(result, fw)
		}
	}
	return result
}

func (e *Engine) Structs() []TypeWork {
	var result []TypeWork
	for _, sw := range e.structs {
		structTyp := base.Cast[StructType](e.env.Type(sw.TypeID).Kind)
		if !e.env.hasTypeParam(structTyp.TypeArgs) {
			result = append(result, sw)
		}
	}
	return result
}

func (e *Engine) Consts() []ConstWork {
	return e.consts
}

func (e *Engine) Unions() []TypeWork {
	var result []TypeWork
	for _, uw := range e.unions {
		unionTyp := base.Cast[UnionType](e.env.Type(uw.TypeID).Kind)
		if !e.env.hasTypeParam(unionTyp.TypeArgs) {
			result = append(result, uw)
		}
	}
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
	nodeDebug := e.ast.Debug(nodeID, false, 0)
	debugDedent := e.debug.Print(1, "query start %s", nodeDebug).Indent()
	defer debugDedent()
	node := e.ast.Node(nodeID)
	var typeID TypeID
	var status TypeStatus
	switch nodeKind := node.Kind.(type) {
	case ast.Assign:
		typeID, status = e.checkAssign(nodeKind)
	case ast.Binary:
		typeID, status = e.checkBinary(nodeKind)
	case ast.Unary:
		typeID, status = e.checkUnary(nodeKind)
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
	case ast.Fun, ast.Struct, ast.Shape, ast.Union:
		cachedType, ok := e.env.cachedNodeType(nodeID)
		if !ok {
			e.forwardDeclare([]ast.NodeID{nodeID})
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
	case ast.ArrayType:
		typeID, status = e.checkArrayType(nodeID, nodeKind, node.Span)
	case ast.SliceType:
		typeID, status = e.checkSliceType(nodeID, nodeKind, node.Span)
	case ast.FunType:
		typeID, status = e.checkFunType(nodeID, nodeKind, node.Span)
	case ast.ArrayLiteral:
		typeID, status = e.checkArrayLiteral(nodeID, nodeKind, node.Span)
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
	case ast.Path:
		typeID, status = e.checkPath(nodeID, nodeKind, node.Span)
	case ast.Ident:
		typeID, status = e.checkIdent(nodeID, nodeKind, node.Span)
	case ast.Bool:
		typeID, status = e.checkBool()
	case ast.Int:
		typeID, status = e.checkInt(nodeKind, node.Span, typeHint)
	case ast.Ref:
		typeID, status = e.checkRef(nodeID, nodeKind, node.Span)
	case ast.RefType:
		typeID, status = e.checkRefType(nodeID, nodeKind, node.Span)
	case ast.SimpleType:
		typeID, status = e.checkSimpleType(nodeID, nodeKind, node.Span)
	case ast.String:
		typeID, status = e.checkString()
	case ast.RuneLiteral:
		typeID, status = e.checkRuneLiteral(nodeKind, node.Span, typeHint)
	case ast.Var:
		typeID, status = e.checkVar(nodeID, nodeKind, node.Span)
	default:
		panic(base.Errorf("unknown node kind: %T", nodeKind))
	}
	typeID, status = e.updateCachedType(node, typeID, status)
	if typeHint != nil && !status.Failed() && typeID != *typeHint {
		typeID = e.tryUnionAutoWrap(nodeID, typeID, *typeHint)
	}
	debugDedent()
	e.debug.Print(0, "query end   %s -> %s", nodeDebug, e.env.TypeDisplay(typeID))
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
	}
	saved := e.typeHint
	e.typeHint = typeHint
	typeID, status := e.Query(nodeID)
	e.typeHint = saved
	if !status.Failed() && typeID != *typeHint {
		typeID = e.tryUnionAutoWrap(nodeID, typeID, *typeHint)
	}
	return typeID, status
}

func (e *Engine) checkAssign(assign ast.Assign) (TypeID, TypeStatus) {
	// `_ = expr` discards the result of expr.
	if ident, ok := e.ast.Node(assign.LHS).Kind.(ast.Ident); ok && ident.Name == "_" {
		rhsTypeID, status := e.Query(assign.RHS)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.isInstantiable(rhsTypeID) {
			e.diag(e.ast.Node(assign.RHS).Span, "cannot discard expression of type %s", e.env.TypeDisplay(rhsTypeID))
			return InvalidTypeID, TypeFailed
		}
		return e.voidTyp, TypeOK
	}
	lhsTypeID, status := e.typeOfPlace(assign.LHS)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
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
	return e.voidTyp, TypeOK
}

func (e *Engine) checkUnary(unary ast.Unary) (TypeID, TypeStatus) {
	exprTypeID, status := e.Query(unary.Expr)
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
	default:
		panic(base.Errorf("unknown unary operator: %s", unary.Op))
	}
}

func (e *Engine) isLiteral(nodeID ast.NodeID) bool {
	switch e.ast.Node(nodeID).Kind.(type) {
	case ast.Int, ast.RuneLiteral:
		return true
	}
	return false
}

func (e *Engine) checkBinary(binary ast.Binary) (TypeID, TypeStatus) { //nolint:funlen
	// When the LHS is a literal but the RHS is not, resolve the RHS first so its
	// concrete type can serve as a type hint for the literal (e.g. `10 == byte`).
	lhsIsLiteral := e.isLiteral(binary.LHS) && !e.isLiteral(binary.RHS)
	var lhsTypeID, rhsTypeID TypeID
	var status TypeStatus
	if lhsIsLiteral {
		rhsTypeID, status = e.Query(binary.RHS)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		lhsTypeID, status = e.queryWithHint(binary.LHS, &rhsTypeID)
	} else {
		lhsTypeID, status = e.Query(binary.LHS)
	}
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	var valid bool
	var expected string
	switch binary.Op {
	case ast.BinaryOpEq, ast.BinaryOpNeq:
		valid = e.env.isIntType(lhsTypeID) || lhsTypeID == e.boolTyp
		expected = "an integer or Bool"
	case ast.BinaryOpLt, ast.BinaryOpLte, ast.BinaryOpGt, ast.BinaryOpGte:
		valid = e.env.isIntType(lhsTypeID)
		expected = "an integer"
	case ast.BinaryOpOr, ast.BinaryOpAnd:
		valid = lhsTypeID == e.boolTyp
		expected = "Bool"
	case ast.BinaryOpAdd, ast.BinaryOpSub, ast.BinaryOpMul, ast.BinaryOpDiv, ast.BinaryOpMod,
		ast.BinaryOpWrapAdd, ast.BinaryOpWrapSub, ast.BinaryOpWrapMul:
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
	if rhsTypeID != lhsTypeID {
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
		if i == lastIdx {
			lastExprTypeID, status = e.queryWithHint(exprNodeID, typeHint)
		} else {
			lastExprTypeID, status = e.Query(exprNodeID)
			if !status.Failed() && e.isInstantiable(lastExprTypeID) {
				switch e.ast.Node(exprNodeID).Kind.(type) {
				case ast.Fun, ast.Struct, ast.Shape, ast.Union:
					// Declarations are not expressions — skip the check.
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
		if lastExprTypeID == e.neverTyp {
			wouldBeDeadCode = true
		}
	}
	if depFailed {
		return InvalidTypeID, TypeDepFailed
	}
	return lastExprTypeID, TypeOK
}

func (e *Engine) checkContinue(span base.Span) (TypeID, TypeStatus) {
	if len(e.loopStack) == 0 {
		e.diag(span, "continue statement outside of loop")
		return InvalidTypeID, TypeFailed
	}
	return e.neverTyp, TypeOK
}

func (e *Engine) checkDefer(defer_ ast.Defer, span base.Span) (TypeID, TypeStatus) {
	blockTypeID, status := e.Query(defer_.Block)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if e.isInstantiable(blockTypeID) {
		e.diag(span, "defer block must not yield a value, got %s", e.env.TypeDisplay(blockTypeID))
		return InvalidTypeID, TypeFailed
	}
	return e.voidTyp, TypeOK
}

func (e *Engine) checkBreak(span base.Span) (TypeID, TypeStatus) {
	if len(e.loopStack) == 0 {
		e.diag(span, "break statement outside of loop")
		return InvalidTypeID, TypeFailed
	}
	return e.neverTyp, TypeOK
}

func (e *Engine) checkFor(nodeID ast.NodeID, for_ ast.For) (TypeID, TypeStatus) {
	if for_.Binding != nil {
		range_ := base.Cast[ast.Range](e.ast.Node(*for_.Cond).Kind)
		if range_.Lo == nil || range_.Hi == nil {
			e.diag(e.ast.Node(*for_.Cond).Span, "for-in range requires both lower and upper bound")
			return InvalidTypeID, TypeFailed
		}
		if _, status := e.Query(*for_.Cond); status.Failed() {
			return InvalidTypeID, status
		}
		e.bind(for_.Body, for_.Binding.Name, false, e.intTyp, for_.Binding.Span)
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
	e.loopStack = append(e.loopStack, nodeID)
	defer func() { e.loopStack = e.loopStack[:len(e.loopStack)-1] }()
	bodyTypeID, status := e.Query(for_.Body)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if e.isInstantiable(bodyTypeID) {
		bodySpan := e.ast.Node(for_.Body).Span
		e.diag(bodySpan, "for loop body must not yield a value, got %s", e.env.TypeDisplay(bodyTypeID))
		return InvalidTypeID, TypeFailed
	}
	return e.voidTyp, TypeOK
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
	return e.voidTyp, TypeOK
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
	if elseType == e.neverTyp || thenType == e.neverTyp {
		return e.voidTyp, TypeOK
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

func (e *Engine) checkAllocatorVar(nodeID ast.NodeID, alloc ast.AllocatorVar, span base.Span) (TypeID, TypeStatus) {
	if alloc.Allocator.Name != "Arena" {
		e.diag(alloc.Allocator.Span, "unknown allocator type: %s", alloc.Allocator.Name)
		return InvalidTypeID, TypeFailed
	}
	if len(alloc.Args) != 0 {
		e.diag(span, "argument count mismatch: expected %d, got %d", 0, len(alloc.Args))
		return InvalidTypeID, TypeFailed
	}
	e.bind(nodeID, alloc.Name.Name, false, e.arenaTyp, span)
	return e.voidTyp, TypeOK
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

func (e *Engine) checkArrayLiteral(nodeID ast.NodeID, array ast.ArrayLiteral, span base.Span) (TypeID, TypeStatus) {
	if len(array.Elems) == 0 {
		e.diag(span, "array literal cannot be empty")
		return InvalidTypeID, TypeFailed
	}
	elemTyp, status := e.Query(array.Elems[0])
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	for _, elemNodeID := range array.Elems[1:] {
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
	}
	return e.env.buildArrayType(elemTyp, int64(len(array.Elems)), nodeID, span), TypeOK
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
	if typeID, status, ok := e.InferTypeConstruction(lit, span, typeHint); ok {
		if status.Failed() {
			return InvalidTypeID, status
		}
		return e.dispatchTypeConstruction(nodeID, typeID, lit, span)
	}
	targetTypeID, status := e.Query(lit.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	return e.dispatchTypeConstruction(nodeID, targetTypeID, lit, span)
}

func (e *Engine) dispatchTypeConstruction(
	nodeID ast.NodeID, targetTypeID TypeID, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus) {
	targetTyp := e.env.Type(targetTypeID)
	switch kind := targetTyp.Kind.(type) {
	case IntType:
		if kind.Name == "Rune" && !ast.IsPreludeNode(nodeID) {
			e.diag(span, "Rune cannot be constructed directly; use Rune.from_u32_lossy() instead")
			return InvalidTypeID, TypeFailed
		}
		return e.checkIntConstruction(nodeID, kind, targetTypeID, lit, span)
	case StructType:
		if kind.Name == "Str" && !ast.IsPreludeNode(nodeID) {
			e.diag(span, "Str cannot be constructed directly; use Str.from_utf8_lossy() instead")
			return InvalidTypeID, TypeFailed
		}
		return e.checkStructConstruction(kind, targetTypeID, lit, span)
	case UnionType:
		return e.checkUnionConstruction(kind, targetTypeID, lit, span)
	default:
		calleeSpan := e.ast.Node(lit.Target).Span
		e.diag(calleeSpan, "not a struct or union: %s", e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
}

func (e *Engine) checkStructConstruction(
	struct_ StructType, structTypeID TypeID, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus) {
	if len(lit.Args) != len(struct_.Fields) {
		e.diag(span, "argument count mismatch: expected %d, got %d", len(struct_.Fields), len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	for i, argNodeID := range lit.Args {
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

func (e *Engine) checkFieldAccess(nodeID ast.NodeID, fieldAccess ast.FieldAccess) (TypeID, TypeStatus) {
	targetTypeID, status := e.Query(fieldAccess.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	targetTyp := e.env.Type(targetTypeID)
	if refTyp, ok := targetTyp.Kind.(RefType); ok {
		targetTyp = e.env.Type(refTyp.Type)
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
				return field.Type, TypeOK
			}
		}
	}
	if typeID, status, ok := e.CheckShapeFieldAccess(targetTyp, fieldAccess.Field.Name); ok {
		return typeID, status
	}
	if typeID, status, ok := e.resolveMethod(nodeID, fieldAccess, targetTyp); ok {
		return typeID, status
	}
	switch targetTyp.Kind.(type) {
	case StructType, UnionType, IntType, BoolType, AllocatorType, SliceType:
		e.diag(fieldAccess.Field.Span, "unknown field: %s.%s", typeName, fieldAccess.Field.Name)
	case TypeParamType:
		e.diag(
			fieldAccess.Field.Span,
			"unconstrained type parameter has no fields or methods: %s", typeName,
		)
	default:
		targetSpan := e.ast.Node(fieldAccess.Target).Span
		e.diag(targetSpan, "cannot access field on non-struct type: %s", typeName)
	}
	return InvalidTypeID, TypeFailed
}

func (e *Engine) resolveMethod( //nolint:funlen
	nodeID ast.NodeID,
	fieldAccess ast.FieldAccess,
	targetTyp *Type,
) (TypeID, TypeStatus, bool) {
	e.debug.Print(
		1,
		"resolveMethod .%s on %s (node=%s)",
		fieldAccess.Field.Name,
		e.env.TypeDisplay(targetTyp.ID),
		nodeID,
	)
	methodName := fieldAccess.Field.Name
	lookupName, ok := e.env.methodFQN(targetTyp, methodName)
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	binding, ok := e.lookup(nodeID, lookupName)
	if !ok && e.instantiationScope != nil {
		binding, ok = e.lookup(*e.instantiationScope, lookupName)
	}
	if !ok {
		binding, ok = e.lookupInTypeModule(targetTyp, lookupName)
	}
	if !ok {
		structType, isStruct := targetTyp.Kind.(StructType)
		if isStruct && len(structType.TypeArgs) > 0 {
			structNodeID := e.env.DeclNode(targetTyp.ID)
			structNode := base.Cast[ast.Struct](e.ast.Node(structNodeID).Kind)
			structName := e.scopeGraph.NodeScope(structNodeID).NamespacedName(structNode.Name.Name)
			lookupName = structName + "." + methodName
			binding, ok = e.lookup(nodeID, lookupName)
			if !ok {
				binding, ok = e.lookupInTypeModule(targetTyp, lookupName)
			}
			if !ok ||
				len(base.Cast[ast.Fun](e.ast.Node(binding.Decl).Kind).TypeParams) < len(structType.TypeArgs) {
				return InvalidTypeID, TypeFailed, false
			}
		}
	}
	if !ok {
		unionType, isUnion := targetTyp.Kind.(UnionType)
		if isUnion && len(unionType.TypeArgs) > 0 {
			unionNodeID := e.env.DeclNode(targetTyp.ID)
			unionNode := base.Cast[ast.Union](e.ast.Node(unionNodeID).Kind)
			unionName := e.scopeGraph.NodeScope(unionNodeID).NamespacedName(unionNode.Name.Name)
			lookupName = unionName + "." + methodName
			binding, ok = e.lookup(nodeID, lookupName)
			if !ok {
				binding, ok = e.lookupInTypeModule(targetTyp, lookupName)
			}
			if !ok ||
				len(base.Cast[ast.Fun](e.ast.Node(binding.Decl).Kind).TypeParams) < len(unionType.TypeArgs) {
				return InvalidTypeID, TypeFailed, false
			}
		}
	}
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	return e.ResolveMethodBinding(nodeID, fieldAccess, targetTyp, binding)
}

func (e *Engine) lookupInTypeModule(typ *Type, name string) (*Binding, bool) {
	if len(e.moduleResolution.Imports) == 0 {
		return nil, false
	}
	if _, ok := typ.Kind.(ModuleType); ok {
		return nil, false
	}
	declNodeID := e.env.DeclNode(typ.ID)
	if declNodeID == 0 || ast.IsPreludeNode(declNodeID) {
		return nil, false
	}
	_, typModule := e.moduleOf(declNodeID)
	if len(typModule.Decls) == 0 {
		return nil, false
	}
	return e.env.Lookup(typModule.Decls[0], name)
}

func (e *Engine) checkCall(call ast.Call, callNodeID ast.NodeID, span base.Span) (TypeID, TypeStatus) { //nolint:funlen
	diagCount := len(e.diagnostics)
	if _, status, ok := e.InferFunCall(call, span); ok && status.Failed() {
		if len(e.diagnostics) == diagCount {
			e.diag(span, "could not infer type arguments for this call")
		}
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
	var argNodes []ast.NodeID
	fieldAccess, isFieldAccess := e.ast.Node(call.Callee).Kind.(ast.FieldAccess)
	isMethod := e.env.isNamedFun(call.Callee) && isFieldAccess
	if isMethod {
		argNodes = append(argNodes, fieldAccess.Target)
		e.env.setMethodCallReceiver(callNodeID, fieldAccess.Target)
	}
	argNodes = append(argNodes, call.Args...)
	argNodes = e.fillCallDefaults(call, callNodeID, argNodes, fun)
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
	calleeIsUnsafe := false
	if binding, ok := e.resolveCallBinding(call); ok {
		switch kind := e.ast.Node(binding.Decl).Kind.(type) {
		case ast.Fun:
			calleeIsUnsafe = kind.Unsafe
		case ast.FunDecl:
			calleeIsUnsafe = kind.Unsafe
		}
	}
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

func (e *Engine) fillCallDefaults(
	call ast.Call, callNodeID ast.NodeID, argNodes []ast.NodeID, fun FunType,
) []ast.NodeID {
	if len(argNodes) >= len(fun.Params) {
		return argNodes
	}
	defaults := e.funDeclDefaults(call)
	if defaults == nil {
		return argNodes
	}
	missing := len(fun.Params) - len(argNodes)
	if missing > len(defaults) {
		return argNodes
	}
	fill := defaults[len(defaults)-missing:]
	e.env.setCallDefaults(callNodeID, fill)
	return append(argNodes, fill...)
}

// funDeclDefaults returns the default expression NodeIDs for the trailing
// parameters of the function being called, or nil if not applicable.
func (e *Engine) funDeclDefaults(call ast.Call) []ast.NodeID {
	binding, ok := e.resolveCallBinding(call)
	if !ok {
		return nil
	}
	funNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Fun)
	if !ok {
		return nil
	}
	var defaults []ast.NodeID
	for _, paramNodeID := range funNode.Params {
		param := base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind)
		if param.Default != nil {
			defaults = append(defaults, *param.Default)
		}
	}
	return defaults
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
		for _, id := range module.Imports {
			imp := base.Cast[ast.Import](e.ast.Node(id).Kind)
			isAlias := imp.Alias != nil && imp.Alias.Name == name
			if isAlias || imp.Segments[len(imp.Segments)-1] == name {
				importNodeID = id
				break
			}
		}
		scope := e.scopeGraph.NodeScope(importNodeID)
		e.env.bindInScope(scope, importNodeID, name, typeID)
	}
	return TypeOK
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
	// Process module-level constants before functions so function bodies can reference them.
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
		if callNodeID, ok := ast.FindNode[ast.Call](e.ast, varNode.Expr); ok {
			e.diag(e.ast.Node(callNodeID).Span, "function calls are not allowed in module-level constants")
			depFailed = true
			continue
		}
		typeID := e.env.TypeOfNode(varNode.Expr).ID
		if e.env.containsMutablePart(typeID) {
			exprSpan := e.ast.Node(varNode.Expr).Span
			e.diag(exprSpan, "module-level constants cannot contain mutable fields or references")
			depFailed = true
			continue
		}
		name := e.namespacedName(declNodeID, varNode.Name.Name)
		e.registerConst(declNodeID, name, typeID)
	}
	MarkBuiltins(e.ast, module)
	e.forwardDeclareFuns(module.Decls)
	for _, declNodeID := range module.Decls {
		if fun, ok := e.ast.Node(declNodeID).Kind.(ast.Fun); ok && fun.Name.Name == "main" {
			e.registerFun(declNodeID)
		}
	}
	for _, declNodeID := range module.Decls {
		if _, ok := e.ast.Node(declNodeID).Kind.(ast.Var); ok {
			continue // Already processed above.
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
		// This is from the prelude which lives in the global namespace.
		return e.voidTyp, TypeOK
	}
	typeID := e.env.newType(ModuleType{Name: module.Name, Macro: false}, nodeID, span, TypeOK)
	return typeID, TypeOK
}

type forwardDecl struct {
	node       *ast.Node
	typeID     TypeID
	status     TypeStatus
	cachedType *cachedType
}

func (e *Engine) forwardDeclare(nodeIDs []ast.NodeID) {
	e.forwardDeclareTypes(nodeIDs)
	e.forwardDeclareFuns(nodeIDs)
}

func (e *Engine) forwardDeclareTypes(nodeIDs []ast.NodeID) { //nolint:funlen
	var decls []*forwardDecl
	for _, nodeID := range nodeIDs {
		node := e.ast.Node(nodeID)
		switch node.Kind.(type) {
		case ast.Struct, ast.Shape, ast.Union:
			decls = append(decls, &forwardDecl{node, InvalidTypeID, TypeFailed, nil})
		}
	}

	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Shape); !ok {
			continue
		}
		nodeKind := base.Cast[ast.Shape](decl.node.Kind)
		typeID, status := e.CheckShapeCreateAndBind(decl.node, nodeKind)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.cachedTypeInfo(typeID)
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}

	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Struct); !ok {
			continue
		}
		nodeKind := base.Cast[ast.Struct](decl.node.Kind)
		typeID, status := e.checkStructCreateAndBind(decl.node, nodeKind)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.cachedTypeInfo(typeID)
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}

	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Union); !ok {
			continue
		}
		nodeKind := base.Cast[ast.Union](decl.node.Kind)
		typeID, status := e.checkUnionCreateAndBind(decl.node, nodeKind)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.cachedTypeInfo(typeID)
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}

	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Shape); !ok {
			continue
		}
		if decl.status.Failed() {
			continue
		}
		shapeNode := base.Cast[ast.Shape](decl.node.Kind)
		shapeType := base.Cast[ShapeType](decl.cachedType.Type.Kind)
		decl.status, decl.cachedType.Type.Kind = e.CheckShapeCompleteType(
			decl.node, shapeNode, shapeType,
		)
		decl.typeID, decl.status = e.updateCachedType(decl.node, decl.typeID, decl.status)
	}

	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Struct); !ok {
			continue
		}
		if decl.status.Failed() {
			continue
		}
		structType := base.Cast[StructType](decl.cachedType.Type.Kind)
		structNode := base.Cast[ast.Struct](decl.node.Kind)
		decl.status, decl.cachedType.Type.Kind = e.checkStructCompleteType(structNode, structType)
		decl.typeID, decl.status = e.updateCachedType(decl.node, decl.typeID, decl.status)
		if ast.IsPreludeNode(decl.node.ID) {
			e.fixPreludeType(decl.node, decl.cachedType)
		}
	}

	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Union); !ok {
			continue
		}
		if decl.status.Failed() {
			continue
		}
		unionType := base.Cast[UnionType](decl.cachedType.Type.Kind)
		unionNode := base.Cast[ast.Union](decl.node.Kind)
		decl.status, decl.cachedType.Type.Kind = e.checkUnionCompleteType(unionNode, unionType)
		decl.typeID, decl.status = e.updateCachedType(decl.node, decl.typeID, decl.status)
	}
}

func (e *Engine) forwardDeclareFuns(nodeIDs []ast.NodeID) {
	var decls []*forwardDecl
	for _, nodeID := range nodeIDs {
		node := e.ast.Node(nodeID)
		switch node.Kind.(type) {
		case ast.Fun, ast.FunDecl:
			decls = append(decls, &forwardDecl{node, InvalidTypeID, TypeFailed, nil})
		}
	}
	for _, decl := range decls {
		var funDecl ast.FunDecl
		switch kind := decl.node.Kind.(type) {
		case ast.Fun:
			funDecl = kind.FunDecl
		case ast.FunDecl:
			funDecl = kind
		}
		typeID, status := e.checkFunCreateAndBind(decl.node, funDecl)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.cachedTypeInfo(typeID)
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}
	for _, decl := range decls {
		if decl.status.Failed() {
			continue
		}
		funKind, isFun := decl.node.Kind.(ast.Fun)
		if isFun && !funKind.Builtin && !funKind.Extern {
			funType := base.Cast[FunType](decl.cachedType.Type.Kind)
			e.debug.Print(
				0,
				"forwardDeclare checkFunBody: %s (node=%s, type=%s)",
				funKind.Name.Name,
				decl.node.ID,
				decl.cachedType.Type.ID,
			)
			e.checkFunBody(decl.node.ID, funKind, decl.cachedType.Type.ID, funType)
		}
	}
}

func (e *Engine) checkFunCreateAndBind(node *ast.Node, fun ast.FunDecl) (TypeID, TypeStatus) {
	if status := e.bindTypeParams(fun.TypeParams); status.Failed() {
		return InvalidTypeID, status
	}
	retTypeID, status := e.Query(fun.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if _, ok := e.env.Type(retTypeID).Kind.(AllocatorType); ok {
		e.diag(e.ast.Node(fun.ReturnType).Span, "cannot return an allocator from a function")
		return InvalidTypeID, TypeFailed
	}
	paramTypeIDs := make([]TypeID, len(fun.Params))
	for i, paramNodeID := range fun.Params {
		paramTypeID, status := e.Query(paramNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		paramTypeIDs[i] = paramTypeID
	}
	funTyp := FunType{Params: paramTypeIDs, Return: retTypeID, Macro: false}
	cacheKey := funTypeCacheKey(funTyp)
	cached, ok := e.env.cachedFunType(cacheKey)
	var funTypeID TypeID
	if ok {
		funTypeID = cached.Type.ID
		e.env.setNodeType(node.ID, cached)
	} else {
		funTypeID = e.env.newType(funTyp, node.ID, node.Span, TypeOK)
		e.env.cacheFunType(cacheKey, funTypeID)
	}
	bindName := fun.Name.Name
	if structName, methodName, ok := strings.Cut(fun.Name.Name, "."); ok {
		resolved, ok := e.resolveMethodBindName(node.ID, structName, methodName, fun.Name.Span)
		if !ok {
			return InvalidTypeID, TypeFailed
		}
		bindName = resolved
	}
	if !e.bind(node.ID, bindName, false, funTypeID, fun.Name.Span) {
		return InvalidTypeID, TypeFailed
	}
	if fun.Extern {
		e.env.setNamedFunRef(node.ID, fun.ExternName)
	} else {
		e.env.setNamedFunRef(node.ID, e.namespacedName(node.ID, fun.Name.Name))
	}
	return funTypeID, TypeOK
}

func (e *Engine) resolveMethodBindName(
	nodeID ast.NodeID, structName, methodName string, span base.Span,
) (string, bool) {
	binding, ok := e.lookup(nodeID, structName)
	if !ok {
		e.diag(span, "method receiver type not found: %s", structName)
		return "", false
	}
	fqn, ok := e.env.methodFQN(e.env.Type(binding.TypeID), methodName)
	if !ok {
		e.diag(span, "type %s cannot have methods", structName)
		return "", false
	}
	return fqn, true
}

func (e *Engine) checkStructCreateAndBind(node *ast.Node, structNode ast.Struct) (TypeID, TypeStatus) {
	name := e.namespacedName(node.ID, structNode.Name.Name)
	typeID := e.env.newType(StructType{name, []StructField{}, nil}, node.ID, node.Span, TypeInProgress)
	if !e.bind(node.ID, structNode.Name.Name, false, typeID, structNode.Name.Span) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

func (e *Engine) checkUnionCreateAndBind(node *ast.Node, unionNode ast.Union) (TypeID, TypeStatus) {
	name := e.namespacedName(node.ID, unionNode.Name.Name)
	typeID := e.env.newType(UnionType{name, nil, nil}, node.ID, node.Span, TypeInProgress)
	if !e.bind(node.ID, unionNode.Name.Name, false, typeID, unionNode.Name.Span) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
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
			}
		}
	}
}

func (e *Engine) checkStructCompleteType(structNode ast.Struct, structType StructType) (TypeStatus, StructType) {
	defer e.enterChildEnv()()
	if status := e.bindTypeParams(structNode.TypeParams); status.Failed() {
		return status, structType
	}
	fields := make([]StructField, len(structNode.Fields))
	for i, fieldNodeID := range structNode.Fields {
		fieldTypeID, status := e.Query(fieldNodeID)
		if status.Failed() {
			return TypeDepFailed, structType
		}
		fieldNode := base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind)
		fields[i] = StructField{fieldNode.Name.Name, fieldTypeID, fieldNode.Mut}
	}
	structType.Fields = fields
	return TypeOK, structType
}

func (e *Engine) checkUnionCompleteType(unionNode ast.Union, unionType UnionType) (TypeStatus, UnionType) {
	defer e.enterChildEnv()()
	if status := e.bindTypeParams(unionNode.TypeParams); status.Failed() {
		return status, unionType
	}
	variants := make([]TypeID, len(unionNode.Variants))
	for i, variantNodeID := range unionNode.Variants {
		variantTypeID, status := e.Query(variantNodeID)
		if status.Failed() {
			return TypeDepFailed, unionType
		}
		variants[i] = variantTypeID
	}
	unionType.Variants = variants
	return TypeOK, unionType
}

func (e *Engine) checkFunBody(funNodeID ast.NodeID, funNode ast.Fun, funTypeID TypeID, funType FunType) {
	debugDedent := e.debug.Print(0, "checkFunBody %s (type=%s)", funNode.Name.Name, funTypeID).Indent()
	defer debugDedent()
	e.funStack = append(e.funStack, funTypeID)
	defer func() { e.funStack = e.funStack[:len(e.funStack)-1] }()
	// Bind captures. Each capture references a binding in the outer scope
	// (looked up via the Fun node, which lives in the outer scope) and creates
	// a new binding inside the closure scope.
	for _, capNodeID := range funNode.Captures {
		capture := base.Cast[ast.Capture](e.ast.Node(capNodeID).Kind)
		e.bindCapture(funNodeID, capNodeID, capture)
	}
	for i, paramNodeID := range funNode.Params {
		paramNode := base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind)
		paramTypeID := funType.Params[i]
		if !e.bind(paramNodeID, paramNode.Name.Name, false, paramTypeID, paramNode.Name.Span) {
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
	for _, paramNodeID := range funType.Params {
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
	funTyp := FunType{Params: params, Return: returnType, Macro: false}
	cacheKey := funTypeCacheKey(funTyp)
	cached, ok := e.env.cachedFunType(cacheKey)
	if ok {
		return cached.Type.ID, cached.Status
	}
	typeID := e.env.newType(funTyp, nodeID, span, TypeOK)
	e.env.cacheFunType(cacheKey, typeID)
	return typeID, TypeOK
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
	if exprTypeID != funType.Return {
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

func (e *Engine) checkStructField(structField ast.StructField) (TypeID, TypeStatus) {
	typeID, status := e.Query(structField.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	return typeID, TypeOK
}

func (e *Engine) checkPath(nodeID ast.NodeID, path ast.Path, span base.Span) (TypeID, TypeStatus) {
	if len(path.Segments) < 2 {
		panic(base.Errorf("path must have at least 2 segments"))
	}
	if len(path.Segments) > 2 {
		e.diag(span, "invalid module path")
		return InvalidTypeID, TypeFailed
	}
	moduleName := path.Segments[0]
	modBinding, ok := e.lookup(nodeID, moduleName)
	if !ok {
		e.diag(span, "symbol not defined: %s", moduleName)
		return InvalidTypeID, TypeFailed
	}
	if _, ok := e.env.Type(modBinding.TypeID).Kind.(ModuleType); !ok {
		e.diag(span, "%s is not a module", moduleName)
		return InvalidTypeID, TypeFailed
	}
	// Look up the member in the imported module's scope.
	memberName := path.Segments[1]
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
	// Method references like `lib::Point.sum` need the same bind-name resolution
	// as local `Point.sum`, i.e. the struct name is resolved to its namespaced type name.
	lookupName := memberName
	if structName, methodName, ok := strings.Cut(memberName, "."); ok {
		resolved, ok := e.resolveMethodBindName(mod.Decls[0], structName, methodName, span)
		if !ok {
			return InvalidTypeID, TypeFailed
		}
		lookupName = resolved
	}
	binding, ok := e.env.Lookup(mod.Decls[0], lookupName)
	if !ok {
		e.diag(span, "symbol not defined in %s: %s", moduleName, memberName)
		return InvalidTypeID, TypeFailed
	}
	// Store the binding for codegen (needed for cross-module constant references).
	if _, isVar := e.ast.Node(binding.Decl).Kind.(ast.Var); isVar {
		e.env.pathBindings[nodeID] = binding
	}
	return e.resolveBinding(nodeID, binding, path.TypeArgs)
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
	binding, ok := e.lookup(nodeID, lookupName)
	if !ok {
		e.diag(span, "symbol not defined: %s", ident.Name)
		return InvalidTypeID, TypeFailed
	}
	e.debug.Print(1, "checkIdent %q -> lookup=%q binding.Decl=%s binding.TypeID=%s type=%s",
		ident.Name, lookupName, binding.Decl, binding.TypeID, e.env.TypeDisplay(binding.TypeID))
	if binding.Decl != 0 && e.unreachableBindingInOuterScope(nodeID, binding) {
		e.diag(span, "cannot reference %q from outer scope", ident.Name)
		return InvalidTypeID, TypeFailed
	}
	return e.resolveBinding(nodeID, binding, ident.TypeArgs)
}

// Check whether the binding a variable, allocator, or function parameter declared
// in an outer scope.
func (e *Engine) bindCapture(funNodeID, capNodeID ast.NodeID, capture ast.Capture) {
	// Look up the captured name from the outer scope via the Fun node (which
	// lives in the enclosing scope, before the function's own scope).
	outerBinding, ok := e.lookup(funNodeID, capture.Name.Name)
	if !ok {
		e.diag(capture.Name.Span, "capture: symbol not defined: %s", capture.Name.Name)
		return
	}
	outerTypeID := outerBinding.TypeID
	var captureTypeID TypeID
	switch capture.Mode {
	case ast.CaptureByValue:
		captureTypeID = outerTypeID
	case ast.CaptureByRef:
		captureTypeID = e.env.buildRefType(capNodeID, outerTypeID, false, capture.Name.Span)
	case ast.CaptureByMutRef:
		if !outerBinding.Mut {
			e.diag(capture.Name.Span, "cannot take mutable reference to immutable value")
			return
		}
		captureTypeID = e.env.buildRefType(capNodeID, outerTypeID, true, capture.Name.Span)
	}
	e.bind(capNodeID, capture.Name.Name, false, captureTypeID, capture.Name.Span)
}

func (e *Engine) unreachableBindingInOuterScope(nodeID ast.NodeID, binding *Binding) bool {
	switch e.ast.Node(binding.Decl).Kind.(type) {
	case ast.Var:
		// Module-level constants can be referenced from any function.
		bindingScope := e.scopeGraph.NodeScope(binding.Decl)
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
			// Allow crossing a function boundary if the variable is explicitly captured.
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

func (e *Engine) resolveBinding(nodeID ast.NodeID, binding *Binding, typeArgs []ast.NodeID) (TypeID, TypeStatus) {
	if cached, ok := e.env.cachedTypeInfo(binding.TypeID); ok && cached.Status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	span := e.ast.Node(nodeID).Span
	if binding.Decl != 0 {
		if funNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Fun); ok {
			if len(typeArgs) > 0 || len(funNode.TypeParams) > 0 {
				argTypeIDs, status := e.QueryTypeArgs(typeArgs)
				if status.Failed() {
					return InvalidTypeID, status
				}
				decl, _ := e.NormalizeGenericDecl(binding.Decl, binding.TypeID, "")
				spec, status := e.BuildGenericSpec(decl.originTypeID, decl.typeParams)
				if status.Failed() {
					return InvalidTypeID, status
				}
				argTypeIDs, status = e.SolveGenericArgs(spec, argTypeIDs, nodeID, span)
				if status.Failed() {
					return InvalidTypeID, status
				}
				typeID, mangledName, status := e.MaterializeFun(
					binding.Decl,
					binding.TypeID,
					nodeID,
					span,
					argTypeIDs,
				)
				if status.Failed() {
					return InvalidTypeID, status
				}
				e.env.setNamedFunRef(nodeID, mangledName)
				return typeID, TypeOK
			}
			e.env.copyNamedFunRef(nodeID, binding.Decl)
			e.registerFun(binding.Decl)
		}
		if _, ok := e.ast.Node(binding.Decl).Kind.(ast.FunDecl); ok {
			e.env.copyNamedFunRef(nodeID, binding.Decl)
		}
		if structType, ok := e.env.Type(binding.TypeID).Kind.(StructType); ok {
			if structNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Struct); ok {
				if len(typeArgs) > 0 || len(structNode.TypeParams) > 0 {
					_ = structType
					return e.MaterializeNamedTypeRef(binding.TypeID, typeArgs, span)
				}
				e.registerStruct(structType, binding.Decl, binding.TypeID)
			}
		}
		if unionType, ok := e.env.Type(binding.TypeID).Kind.(UnionType); ok {
			if unionNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Union); ok {
				if len(typeArgs) > 0 || len(unionNode.TypeParams) > 0 {
					_ = unionType
					return e.MaterializeNamedTypeRef(binding.TypeID, typeArgs, span)
				}
				e.registerUnion(unionType, binding.Decl, binding.TypeID)
			}
		}
	}
	return binding.TypeID, TypeOK
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

func (e *Engine) checkRef(nodeID ast.NodeID, ref ast.Ref, span base.Span) (TypeID, TypeStatus) {
	targetTypeID, status := e.Query(ref.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if ref.Mut {
		_, mut := e.isPlaceMutable(ref.Target)
		if !mut {
			e.diag(span, "cannot take mutable reference to immutable value")
			return InvalidTypeID, TypeFailed
		}
	}
	refTypeID := e.env.buildRefType(nodeID, targetTypeID, ref.Mut, span)
	return refTypeID, TypeOK
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

func (e *Engine) checkSimpleType(nodeID ast.NodeID, simpleType ast.SimpleType, span base.Span) (TypeID, TypeStatus) {
	binding, ok := e.lookup(nodeID, simpleType.Name.Name)
	if !ok {
		e.diag(span, "symbol not defined: %s", simpleType.Name.Name)
		return InvalidTypeID, TypeFailed
	}
	if len(simpleType.TypeArgs) > 0 {
		switch kind := e.env.Type(binding.TypeID).Kind.(type) {
		case StructType:
			_ = kind
			return e.MaterializeNamedTypeRef(binding.TypeID, simpleType.TypeArgs, span)
		case UnionType:
			_ = kind
			return e.MaterializeNamedTypeRef(binding.TypeID, simpleType.TypeArgs, span)
		case ShapeType:
			_ = kind
			return e.MaterializeNamedTypeRef(binding.TypeID, simpleType.TypeArgs, span)
		default:
			e.diag(span, "type arguments on non-generic type: %s", simpleType.Name.Name)
			return InvalidTypeID, TypeFailed
		}
	}
	if structType, isStruct := e.env.Type(binding.TypeID).Kind.(StructType); isStruct {
		if structNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Struct); ok {
			if len(structNode.TypeParams) > 0 {
				_ = structType
				return e.MaterializeNamedTypeRef(binding.TypeID, nil, span)
			}
			e.registerStruct(structType, binding.Decl, binding.TypeID)
		}
	}
	if unionType, isUnion := e.env.Type(binding.TypeID).Kind.(UnionType); isUnion {
		if unionNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Union); ok {
			if len(unionNode.TypeParams) > 0 {
				_ = unionType
				return e.MaterializeNamedTypeRef(binding.TypeID, nil, span)
			}
			e.registerUnion(unionType, binding.Decl, binding.TypeID)
		}
	}
	// We need to return the correct type status (the type might have failed).
	cachedType, ok := e.env.cachedTypeInfo(binding.TypeID)
	if ok {
		return binding.TypeID, cachedType.Status
	}
	return binding.TypeID, TypeOK
}

func (e *Engine) checkString() (TypeID, TypeStatus) {
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
	bindTypeID := exprTypeID
	if varNode.Type != nil {
		if !e.isAssignableTo(exprTypeID, declTypeID) {
			exprSpan := e.ast.Node(varNode.Expr).Span
			e.diag(exprSpan, "type mismatch: expected %s, got %s",
				e.env.TypeDisplay(declTypeID), e.env.TypeDisplay(exprTypeID))
			return InvalidTypeID, TypeFailed
		}
		bindTypeID = declTypeID
	}
	if !e.bind(nodeID, varNode.Name.Name, varNode.Mut, bindTypeID, varNode.Name.Span) {
		return InvalidTypeID, TypeFailed
	}
	return e.voidTyp, TypeOK
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
		if strings.HasPrefix(union.Name, "Result.") && union.Variants[0] == e.voidTyp {
			return
		}
	}
	e.diag(e.ast.Node(fun.ReturnType).Span, "main function must return void or !void")
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
			e.diag(node.Span, "cannot assign to immutable field: %s", kind.Field.Name)
		} else {
			e.diag(node.Span, "cannot assign to field of immutable value")
		}
	case ast.Index:
		e.diag(node.Span, "cannot assign to element of immutable array or slice")
	default:
		e.diag(node.Span, "cannot assign to left-hand-side expression of type: %T", kind)
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
		binding, ok := e.lookup(nodeID, kind.Name)
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
			// Slice element mutability comes from the slice type's Mut flag,
			// not from the binding's mut. When accessed through a ref, both
			// the ref and the slice must be mutable.
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
				return typeID, field.Mut
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
	for _, variantID := range unionType.Variants {
		if e.isAssignableTo(typeID, variantID) {
			e.env.recordUnionWrap(nodeID, hintTypeID)
			return hintTypeID
		}
	}
	return typeID
}

func (e *Engine) isInstantiable(t TypeID) bool {
	return t != e.voidTyp && t != e.neverTyp
}

func (e *Engine) isAssignableTo(got TypeID, expected TypeID) bool {
	if got == expected || got == e.neverTyp {
		return true
	}
	// A &mut T is assignable to &T (coerce by masking off the mutable flag).
	if got&mutableRefFlag != 0 && got&^mutableRefFlag == expected {
		return true
	}
	// A []mut T is assignable to []T (coerce by masking off the mutable slice flag).
	return got&mutableSliceFlag != 0 && got&^mutableSliceFlag == expected
}
