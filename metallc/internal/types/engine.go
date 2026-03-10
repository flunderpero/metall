package types

import (
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/modules"
)

type FunWork struct {
	NodeID ast.NodeID
	TypeID TypeID
	Name   string
	Env    *TypeEnv
}

type StructWork struct {
	NodeID ast.NodeID
	TypeID TypeID
	Env    *TypeEnv
}

type Engine struct {
	*EngineCore
	generics         *GenericsEngine
	moduleResolution *modules.ModuleResolution

	loopStack []ast.NodeID
	funStack  []TypeID
	typeHint  *TypeID
	voidTyp   TypeID
	boolTyp   TypeID
	strTyp    TypeID
	arenaTyp  TypeID
	intTyp    TypeID
}

func NewEngine(a *ast.AST, preludeAST *ast.AST, moduleResolution *modules.ModuleResolution) *Engine {
	merged, err := preludeAST.Merge(a)
	if err != nil {
		panic(base.WrapErrorf(err, "failed to merge prelude AST"))
	}
	g := ast.BuildScopeGraph(merged, merged.Roots)
	c := NewEngineCore(merged, g)
	e := &Engine{ //nolint:exhaustruct
		EngineCore:       c,
		moduleResolution: moduleResolution,
	}
	e.generics = NewGenericsEngine(c, e.Query, e.checkFunBody)
	for _, root := range preludeAST.Roots {
		e.Query(root)
	}
	e.funs = map[string]FunWork{}
	e.structs = map[string]StructWork{}
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

func (e *Engine) Structs() []StructWork {
	var result []StructWork
	for _, sw := range e.structs {
		structTyp := base.Cast[StructType](e.env.Type(sw.TypeID).Kind)
		if !e.env.hasTypeParam(structTyp.TypeArgs) {
			result = append(result, sw)
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
		typeID, status = e.checkBlock(nodeKind)
	case ast.Call:
		typeID, status = e.checkCall(nodeKind, nodeID, node.Span)
	case ast.Deref:
		typeID, status = e.checkDeref(nodeKind)
	case ast.Module:
		typeID, status = e.checkModule(nodeID, nodeKind, node.Span)
	case ast.If:
		typeID, status = e.checkIf(nodeKind)
	case ast.For:
		typeID, status = e.checkFor(nodeID, nodeKind)
	case ast.Break:
		typeID, status = e.checkBreak(node.Span)
	case ast.Continue:
		typeID, status = e.checkContinue(node.Span)
	case ast.Fun, ast.Struct, ast.Shape:
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
	case ast.StructLiteral:
		typeID, status = e.checkStructLiteral(nodeKind, node.Span)
	case ast.New:
		typeID, status = e.checkNew(nodeID, nodeKind, node.Span)
	case ast.ArrayType:
		typeID, status = e.checkArrayType(nodeID, nodeKind, node.Span)
	case ast.SliceType:
		typeID, status = e.checkSliceType(nodeID, nodeKind, node.Span)
	case ast.FunType:
		typeID, status = e.checkFunType(nodeID, nodeKind, node.Span)
	case ast.NewArray:
		typeID, status = e.checkNewArray(nodeKind)
	case ast.MakeSlice:
		typeID, status = e.checkMakeSlice(nodeKind)
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
	case ast.Var:
		typeID, status = e.checkVar(nodeID, nodeKind, node.Span)
	default:
		panic(base.Errorf("unknown node kind: %T", nodeKind))
	}
	typeID, status = e.updateCachedType(node, typeID, status)
	debugDedent()
	e.debug.Print(0, "query end   %s -> %s", nodeDebug, e.env.TypeDisplay(typeID))
	return typeID, status
}

func (e *Engine) queryWithHint(nodeID ast.NodeID, typeHint TypeID) (TypeID, TypeStatus) {
	saved := e.typeHint
	e.typeHint = &typeHint
	typeID, status := e.Query(nodeID)
	e.typeHint = saved
	return typeID, status
}

func (e *Engine) checkAssign(assign ast.Assign) (TypeID, TypeStatus) {
	lhsTypeID, status := e.typeOfPlace(assign.LHS)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	rhsTypeID, status := e.queryWithHint(assign.RHS, lhsTypeID)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if !e.env.isAssignableTo(rhsTypeID, lhsTypeID) {
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
	default:
		panic(base.Errorf("unknown unary operator: %s", unary.Op))
	}
}

func (e *Engine) checkBinary(binary ast.Binary) (TypeID, TypeStatus) {
	lhsTypeID, status := e.Query(binary.LHS)
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
	case ast.BinaryOpAdd, ast.BinaryOpSub, ast.BinaryOpMul, ast.BinaryOpDiv, ast.BinaryOpMod:
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
	rhsTypeID, status := e.queryWithHint(binary.RHS, lhsTypeID)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
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

func (e *Engine) checkBlock(block ast.Block) (TypeID, TypeStatus) {
	if len(block.Exprs) == 0 {
		return e.voidTyp, TypeOK
	}
	e.forwardDeclare(block.Exprs)
	depFailed := false
	var lastExprTypeID TypeID
	var status TypeStatus
	wouldBeDeadCode := false
	for _, exprNodeID := range block.Exprs {
		if wouldBeDeadCode {
			e.diag(e.ast.Node(exprNodeID).Span, "unreachable code")
			return InvalidTypeID, TypeDepFailed
		}
		lastExprTypeID, status = e.Query(exprNodeID)
		if status.Failed() {
			depFailed = true
		}
		switch e.ast.Node(exprNodeID).Kind.(type) {
		case ast.Continue, ast.Break, ast.Return:
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
	return e.voidTyp, TypeOK
}

func (e *Engine) checkBreak(span base.Span) (TypeID, TypeStatus) {
	if len(e.loopStack) == 0 {
		e.diag(span, "break statement outside of loop")
		return InvalidTypeID, TypeFailed
	}
	return e.voidTyp, TypeOK
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
	if bodyTypeID != e.voidTyp {
		bodySpan := e.ast.Node(for_.Body).Span
		e.diag(bodySpan, "for loop body must be void, got %s", e.env.TypeDisplay(bodyTypeID))
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

func (e *Engine) checkIf(if_ ast.If) (TypeID, TypeStatus) {
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
	thenType, status := e.Query(if_.Then)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if if_.Else == nil {
		return e.voidTyp, TypeOK
	}
	elseType, status := e.Query(*if_.Else)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if e.ast.BlockBreaksControlFlow(if_.Then, false) ||
		e.ast.BlockBreaksControlFlow(*if_.Else, false) {
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

func (e *Engine) checkNewArray(alloc ast.NewArray) (TypeID, TypeStatus) {
	arrTypeID, status := e.Query(alloc.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	arrType := base.Cast[ArrayType](e.env.Type(arrTypeID).Kind)
	if alloc.DefaultValue != nil {
		defTypeID, defStatus := e.queryWithHint(*alloc.DefaultValue, arrType.Elem)
		if defStatus.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.env.isAssignableTo(defTypeID, arrType.Elem) {
			defSpan := e.ast.Node(*alloc.DefaultValue).Span
			e.diag(defSpan, "type mismatch: expected %s, got %s",
				e.env.TypeDisplay(arrType.Elem), e.env.TypeDisplay(defTypeID))
			return InvalidTypeID, TypeFailed
		}
	} else if !e.env.isSafeUninitialized(arrType.Elem) {
		typeSpan := e.ast.Node(alloc.Type).Span
		e.diag(typeSpan, "%s is not safe to leave uninitialized, provide a default value",
			e.env.TypeDisplay(arrType.Elem))
		return InvalidTypeID, TypeFailed
	}
	return arrTypeID, TypeOK
}

func (e *Engine) checkMakeSlice(makeSlice ast.MakeSlice) (TypeID, TypeStatus) {
	allocTypeID, allocStatus := e.Query(makeSlice.Allocator)
	if allocStatus.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	allocType := e.env.Type(allocTypeID)
	if _, ok := allocType.Kind.(AllocatorType); !ok {
		allocSpan := e.ast.Node(makeSlice.Allocator).Span
		e.diag(allocSpan, "expected allocator, got %s", e.env.TypeDisplay(allocTypeID))
		return InvalidTypeID, TypeFailed
	}
	sliceTypeID, status := e.Query(makeSlice.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	lenTypeID, lenStatus := e.Query(makeSlice.Len)
	if lenStatus.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if lenTypeID != e.intTyp {
		lenSpan := e.ast.Node(makeSlice.Len).Span
		e.diag(lenSpan, "type mismatch: expected Int, got %s", e.env.TypeDisplay(lenTypeID))
		return InvalidTypeID, TypeFailed
	}
	sliceType := base.Cast[SliceType](e.env.Type(sliceTypeID).Kind)
	if makeSlice.DefaultValue != nil {
		defTypeID, defStatus := e.queryWithHint(*makeSlice.DefaultValue, sliceType.Elem)
		if defStatus.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.env.isAssignableTo(defTypeID, sliceType.Elem) {
			defSpan := e.ast.Node(*makeSlice.DefaultValue).Span
			e.diag(defSpan, "type mismatch: expected %s, got %s",
				e.env.TypeDisplay(sliceType.Elem), e.env.TypeDisplay(defTypeID))
			return InvalidTypeID, TypeFailed
		}
	} else if !e.env.isSafeUninitialized(sliceType.Elem) {
		typeSpan := e.ast.Node(makeSlice.Type).Span
		e.diag(typeSpan, "%s is not safe to leave uninitialized, provide a default value",
			e.env.TypeDisplay(sliceType.Elem))
		return InvalidTypeID, TypeFailed
	}
	return sliceTypeID, TypeOK
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
		elemTyp2, status := e.queryWithHint(elemNodeID, elemTyp)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.env.isAssignableTo(elemTyp2, elemTyp) {
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

func (e *Engine) checkNew(nodeID ast.NodeID, alloc ast.New, span base.Span) (TypeID, TypeStatus) {
	allocTypeID, allocStatus := e.Query(alloc.Allocator)
	if allocStatus.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	allocType := e.env.Type(allocTypeID)
	if _, ok := allocType.Kind.(AllocatorType); !ok {
		allocSpan := e.ast.Node(alloc.Allocator).Span
		e.diag(allocSpan, "expected allocator, got %s", e.env.TypeDisplay(allocTypeID))
		return InvalidTypeID, TypeFailed
	}
	typeID, status := e.Query(alloc.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	typ := e.env.Type(typeID)
	switch typ.Kind.(type) {
	case StructType, ArrayType:
	default:
		targetSpan := e.ast.Node(alloc.Target).Span
		e.diag(targetSpan, "only structs and arrays can be allocated, got %s", e.env.TypeDisplay(typeID))
		return InvalidTypeID, TypeFailed
	}
	return e.env.buildRefType(nodeID, typeID, alloc.Mut, span), TypeOK
}

func (e *Engine) checkStructLiteral(lit ast.StructLiteral, span base.Span) (TypeID, TypeStatus) {
	targetTypeID, status := e.Query(lit.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	targetTyp := e.env.Type(targetTypeID)
	switch kind := targetTyp.Kind.(type) {
	case IntType:
		return e.checkTypeConstructor(kind, targetTypeID, lit, span)
	case StructType:
		return e.checkStructLiteralFields(kind, targetTypeID, lit, span)
	default:
		calleeSpan := e.ast.Node(lit.Target).Span
		e.diag(calleeSpan, "not a struct: %s", e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
}

func (e *Engine) checkStructLiteralFields(
	struct_ StructType, structTypeID TypeID, lit ast.StructLiteral, span base.Span,
) (TypeID, TypeStatus) {
	if len(lit.Args) != len(struct_.Fields) {
		e.diag(span, "argument count mismatch: expected %d, got %d", len(struct_.Fields), len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	for i, argNodeID := range lit.Args {
		argNode := e.ast.Node(argNodeID)
		argTypeID, status := e.queryWithHint(argNodeID, struct_.Fields[i].Type)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.env.isAssignableTo(argTypeID, struct_.Fields[i].Type) {
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

func (e *Engine) checkTypeConstructor(
	targetTyp IntType, targetTypeID TypeID, lit ast.StructLiteral, span base.Span,
) (TypeID, TypeStatus) {
	if len(lit.Args) != 1 {
		e.diag(span, "%s() takes exactly 1 argument, got %d", targetTyp.Name, len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	argNodeID := lit.Args[0]
	argTypeID, status := e.queryWithHint(argNodeID, targetTypeID)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if argTypeID == targetTypeID {
		return targetTypeID, TypeOK
	}
	argInfo, ok := e.env.Type(argTypeID).Kind.(IntType)
	if !ok {
		argSpan := e.ast.Node(argNodeID).Span
		e.diag(argSpan, "cannot convert %s to %s", e.env.TypeDisplay(argTypeID), e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
	allowed := false
	switch {
	case argInfo.Signed == targetTyp.Signed:
		allowed = targetTyp.Bits >= argInfo.Bits
	case !argInfo.Signed && targetTyp.Signed:
		allowed = targetTyp.Bits > argInfo.Bits
	case argInfo.Signed && !targetTyp.Signed:
		allowed = false
	}
	if !allowed {
		argSpan := e.ast.Node(argNodeID).Span
		e.diag(argSpan, "cannot convert %s to %s", e.env.TypeDisplay(argTypeID), e.env.TypeDisplay(targetTypeID))
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
	if _, ok := targetTyp.Kind.(SliceType); ok {
		if fieldAccess.Field.Name == "len" {
			return e.intTyp, TypeOK
		}
		e.diag(fieldAccess.Field.Span, "unknown field on slice: %s", fieldAccess.Field.Name)
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
	if typeID, status, ok := e.generics.checkShapeFieldAccess(targetTyp, fieldAccess.Field.Name); ok {
		return typeID, status
	}
	if typeID, status, ok := e.resolveMethod(nodeID, fieldAccess, targetTyp); ok {
		return typeID, status
	}
	if _, ok := targetTyp.Kind.(StructType); ok {
		e.diag(fieldAccess.Field.Span, "unknown field: %s.%s", typeName, fieldAccess.Field.Name)
	} else {
		targetSpan := e.ast.Node(fieldAccess.Target).Span
		e.diag(targetSpan, "cannot access field on non-struct type: %s", typeName)
	}
	return InvalidTypeID, TypeFailed
}

func (e *Engine) resolveMethod(
	nodeID ast.NodeID,
	fieldAccess ast.FieldAccess,
	targetTyp *Type,
) (TypeID, TypeStatus, bool) {
	methodName := fieldAccess.Field.Name
	lookupName := e.env.typeName(targetTyp) + "." + methodName
	binding, ok := e.lookup(nodeID, lookupName)
	if !ok {
		binding, ok = e.lookupInTypeModule(targetTyp, lookupName)
	}
	if !ok {
		structType, isStruct := targetTyp.Kind.(StructType)
		if !isStruct || len(structType.TypeArgs) == 0 {
			return InvalidTypeID, TypeFailed, false
		}
		structNodeID := e.env.DeclNode(targetTyp.ID)
		structNode := base.Cast[ast.Struct](e.ast.Node(structNodeID).Kind)
		structName := e.scopeGraph.NodeScope(structNodeID).NamespacedName(structNode.Name.Name)
		lookupName = structName + "." + methodName
		binding, ok = e.lookup(nodeID, lookupName)
		if !ok {
			binding, ok = e.lookupInTypeModule(targetTyp, lookupName)
		}
		if !ok || len(base.Cast[ast.Fun](e.ast.Node(binding.Decl).Kind).TypeParams) < len(structType.TypeArgs) {
			return InvalidTypeID, TypeFailed, false
		}
	}
	if typeID, status, ok := e.generics.resolveShapeMethod(nodeID, binding, targetTyp); ok {
		return typeID, status, true
	}
	funNode, isFun := e.ast.Node(binding.Decl).Kind.(ast.Fun)
	if !isFun || (len(funNode.TypeParams) == 0 && len(fieldAccess.TypeArgs) == 0) {
		e.env.copyNamedFunRef(nodeID, binding.Decl)
		e.registerFun(binding.Decl)
		return binding.TypeID, TypeOK, true
	}
	return e.generics.resolveGenericMethod(nodeID, fieldAccess, targetTyp, binding)
}

func (e *Engine) lookupInTypeModule(typ *Type, name string) (*Binding, bool) {
	if len(e.moduleResolution.Imports) == 0 {
		return nil, false
	}
	declNodeID := e.env.DeclNode(typ.ID)
	if declNodeID == 0 || declNodeID >= ast.PreludeFirstID {
		return nil, false
	}
	_, typModule := e.moduleOf(declNodeID)
	if len(typModule.Decls) == 0 {
		return nil, false
	}
	return e.env.Lookup(typModule.Decls[0], name)
}

func (e *Engine) checkCall(call ast.Call, callNodeID ast.NodeID, span base.Span) (TypeID, TypeStatus) {
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
		argTypeID, status := e.queryWithHint(argNodeID, paramTypeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.env.isAssignableTo(argTypeID, paramTypeID) {
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
	return fun.Return, TypeOK
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

func (e *Engine) checkModule(nodeID ast.NodeID, module ast.Module, span base.Span) (TypeID, TypeStatus) {
	// Bind imports into this module's scope.
	if importMap, ok := e.moduleResolution.Imports[nodeID]; ok {
		for name, importedModuleNodeID := range importMap {
			typeID, status := e.Query(importedModuleNodeID)
			if status.Failed() {
				return InvalidTypeID, TypeDepFailed
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
	}
	e.forwardDeclare(module.Decls)
	for _, declNodeID := range module.Decls {
		if fun, ok := e.ast.Node(declNodeID).Kind.(ast.Fun); ok && fun.Name.Name == "main" {
			e.registerFun(declNodeID)
		}
	}
	depFailed := false
	for _, declNodeID := range module.Decls {
		_, status := e.Query(declNodeID)
		if status.Failed() {
			depFailed = true
		}
	}
	if depFailed {
		return InvalidTypeID, TypeDepFailed
	}
	if nodeID >= ast.PreludeFirstID {
		return e.voidTyp, TypeOK
	}
	typeID := e.env.newType(ModuleType{Name: module.Name}, nodeID, span, TypeOK)
	return typeID, TypeOK
}

func (e *Engine) forwardDeclare(nodeIDs []ast.NodeID) { //nolint:funlen
	type decl struct {
		node       *ast.Node
		typeID     TypeID
		status     TypeStatus
		cachedType *cachedType
	}
	decls := []*decl{}

	for _, nodeID := range nodeIDs {
		node := e.ast.Node(nodeID)
		switch node.Kind.(type) {
		case ast.Fun, ast.Struct, ast.Shape:
			decls = append(decls, &decl{node, InvalidTypeID, TypeFailed, nil})
		}
	}

	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Shape); !ok {
			continue
		}
		nodeKind := base.Cast[ast.Shape](decl.node.Kind)
		typeID, status := e.generics.checkShapeCreateAndBind(decl.node, nodeKind)
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
		if _, ok := decl.node.Kind.(ast.Shape); !ok {
			continue
		}
		if decl.status.Failed() {
			continue
		}
		shapeNode := base.Cast[ast.Shape](decl.node.Kind)
		shapeType := base.Cast[ShapeType](decl.cachedType.Type.Kind)
		decl.status, decl.cachedType.Type.Kind = e.generics.checkShapeCompleteType(
			decl.node, shapeNode, shapeType,
		)
		decl.typeID, decl.status = e.updateCachedType(decl.node, decl.typeID, decl.status)
	}

	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Fun); !ok {
			continue
		}
		nodeKind := base.Cast[ast.Fun](decl.node.Kind)
		typeID, status := e.checkFunCreateAndBind(decl.node, nodeKind)
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
		if decl.status.Failed() {
			continue
		}
		structType := base.Cast[StructType](decl.cachedType.Type.Kind)
		structNode := base.Cast[ast.Struct](decl.node.Kind)
		decl.status, decl.cachedType.Type.Kind = e.checkStructCompleteType(structNode, structType)
		decl.typeID, decl.status = e.updateCachedType(decl.node, decl.typeID, decl.status)
		if decl.node.ID >= ast.PreludeFirstID {
			e.fixPreludeType(decl.node, decl.cachedType)
		}
	}

	for _, decl := range decls {
		if decl.status.Failed() {
			continue
		}
		typeKind := decl.cachedType.Type.Kind
		switch nodeKind := decl.node.Kind.(type) {
		case ast.Fun:
			funType := base.Cast[FunType](typeKind)
			e.checkFunBody(nodeKind, decl.cachedType.Type.ID, funType)
		case ast.Struct, ast.Shape:
		default:
			panic(base.Errorf("node kind not supported: %T", nodeKind))
		}
	}
}

func (e *Engine) checkFunCreateAndBind(node *ast.Node, fun ast.Fun) (TypeID, TypeStatus) {
	if status := e.generics.bindTypeParams(fun.TypeParams); status.Failed() {
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
	funTyp := FunType{paramTypeIDs, retTypeID}
	cacheKey := funTypeCacheKey(funTyp)
	cached, ok := e.env.cachedFunType(cacheKey)
	var funTypeID TypeID
	var funStatus TypeStatus
	if ok {
		funTypeID = cached.Type.ID
		funStatus = cached.Status
		e.env.setNodeType(node.ID, cached)
	} else {
		funTypeID = e.env.newType(funTyp, node.ID, node.Span, TypeOK)
		funStatus = TypeOK
		e.env.cacheFunType(cacheKey, funTypeID)
	}
	bindName := fun.Name.Name
	if structName, methodName, ok := strings.Cut(fun.Name.Name, "."); ok {
		bindName = e.resolveMethodBindName(node.ID, structName, methodName)
	}
	if !e.bind(node.ID, bindName, false, funTypeID, fun.Name.Span) {
		return InvalidTypeID, TypeFailed
	}
	e.env.setNamedFunRef(node.ID, e.namespacedName(node.ID, fun.Name.Name))
	return funTypeID, funStatus
}

func (e *Engine) resolveMethodBindName(nodeID ast.NodeID, structName, methodName string) string {
	binding, ok := e.lookup(nodeID, structName)
	if !ok {
		panic(base.Errorf("method receiver type not found: %s", structName))
	}
	return e.env.typeName(e.env.Type(binding.TypeID)) + "." + methodName
}

func (e *Engine) checkStructCreateAndBind(node *ast.Node, structNode ast.Struct) (TypeID, TypeStatus) {
	name := e.namespacedName(node.ID, structNode.Name.Name)
	typeID := e.env.newType(StructType{name, []StructField{}, nil}, node.ID, node.Span, TypeInProgress)
	if !e.bind(node.ID, structNode.Name.Name, false, typeID, structNode.Name.Span) {
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
			}
		}
	}
}

func (e *Engine) checkStructCompleteType(structNode ast.Struct, structType StructType) (TypeStatus, StructType) {
	defer e.enterChildEnv()()
	if status := e.generics.bindTypeParams(structNode.TypeParams); status.Failed() {
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

func (e *Engine) checkFunBody(funNode ast.Fun, funTypeID TypeID, funType FunType) {
	e.funStack = append(e.funStack, funTypeID)
	defer func() { e.funStack = e.funStack[:len(e.funStack)-1] }()
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
	blockTypeID, status := e.Query(funNode.Block)
	if status.Failed() {
		return
	}
	blockNode := e.ast.Node(funNode.Block)
	block := base.Cast[ast.Block](blockNode.Kind)
	if e.ast.BlockReturns(funNode.Block) {
		return
	}
	if funType.Return != e.voidTyp && !e.env.isAssignableTo(blockTypeID, funType.Return) {
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
	funTyp := FunType{params, returnType}
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
	exprTypeID, status := e.Query(return_.Expr)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	funType := base.Cast[FunType](e.env.Type(e.funStack[len(e.funStack)-1]).Kind)
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
	return e.voidTyp, TypeOK
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
		lookupName = e.resolveMethodBindName(mod.Decls[0], structName, methodName)
	}
	binding, ok := e.env.Lookup(mod.Decls[0], lookupName)
	if !ok {
		e.diag(span, "symbol not defined in %s: %s", moduleName, memberName)
		return InvalidTypeID, TypeFailed
	}
	return e.resolveBinding(nodeID, binding, path.TypeArgs)
}

func (e *Engine) checkIdent(nodeID ast.NodeID, ident ast.Ident, span base.Span) (TypeID, TypeStatus) {
	lookupName := ident.Name
	if structName, methodName, ok := strings.Cut(ident.Name, "."); ok {
		lookupName = e.resolveMethodBindName(nodeID, structName, methodName)
	}
	binding, ok := e.lookup(nodeID, lookupName)
	if !ok {
		e.diag(span, "symbol not defined: %s", ident.Name)
		return InvalidTypeID, TypeFailed
	}
	return e.resolveBinding(nodeID, binding, ident.TypeArgs)
}

func (e *Engine) resolveBinding(nodeID ast.NodeID, binding *Binding, typeArgs []ast.NodeID) (TypeID, TypeStatus) {
	if cached, ok := e.env.cachedTypeInfo(binding.TypeID); ok && cached.Status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	span := e.ast.Node(nodeID).Span
	if binding.Decl != 0 {
		if funNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Fun); ok {
			if len(typeArgs) > 0 || len(funNode.TypeParams) > 0 {
				argTypeIDs, status := e.generics.resolveTypeArgs(typeArgs)
				if status.Failed() {
					return InvalidTypeID, status
				}
				typeID, mangledName, status := e.generics.instantiateFun(binding.TypeID, argTypeIDs, span)
				if status.Failed() {
					return InvalidTypeID, status
				}
				e.env.setNamedFunRef(nodeID, mangledName)
				return typeID, TypeOK
			}
			e.env.copyNamedFunRef(nodeID, binding.Decl)
			e.registerFun(binding.Decl)
		}
		if structType, ok := e.env.Type(binding.TypeID).Kind.(StructType); ok {
			if structNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Struct); ok {
				if len(typeArgs) > 0 || len(structNode.TypeParams) > 0 {
					return e.generics.instantiateStruct(structType, binding.TypeID, typeArgs, span)
				}
				e.registerStruct(structType, binding.Decl, binding.TypeID)
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
		structType, isStruct := e.env.Type(binding.TypeID).Kind.(StructType)
		if !isStruct {
			e.diag(span, "type arguments on non-struct type: %s", simpleType.Name.Name)
			return InvalidTypeID, TypeFailed
		}
		return e.generics.instantiateStruct(structType, binding.TypeID, simpleType.TypeArgs, span)
	}
	if structType, isStruct := e.env.Type(binding.TypeID).Kind.(StructType); isStruct {
		if structNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Struct); ok {
			if len(structNode.TypeParams) > 0 {
				return e.generics.instantiateStruct(structType, binding.TypeID, nil, span)
			}
			e.registerStruct(structType, binding.Decl, binding.TypeID)
		}
	}
	return binding.TypeID, TypeOK
}

func (e *Engine) checkString() (TypeID, TypeStatus) {
	return e.strTyp, TypeOK
}

func (e *Engine) checkVar(
	nodeID ast.NodeID, varNode ast.Var, span base.Span,
) (TypeID, TypeStatus) {
	exprTypeID, status := e.Query(varNode.Expr)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if exprTypeID == e.voidTyp {
		e.diag(span, "cannot assign void to a variable")
		return InvalidTypeID, TypeFailed
	}
	if !e.bind(nodeID, varNode.Name.Name, varNode.Mut, exprTypeID, varNode.Name.Span) {
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
	retNode := e.ast.Node(fun.ReturnType)
	if simpleType, ok := retNode.Kind.(ast.SimpleType); ok && simpleType.Name.Name != "void" {
		e.diag(retNode.Span, "main function cannot return a value")
	}
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
