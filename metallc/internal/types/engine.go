package types

import (
	"fmt"
	"math/big"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

const InvalidTypeID = TypeID(0)

type TypeID uint64

func (id TypeID) String() string {
	return fmt.Sprintf("t%d", id)
}

type Type struct {
	ID     TypeID
	NodeID ast.NodeID
	Span   base.Span
	Kind   TypeKind
}

type TypeKind interface {
	isTypeKind()
}

type BuiltInType struct {
	Name string
}

func (BuiltInType) isTypeKind() {}

// IntTypeInfo describes a built-in integer type.
type IntTypeInfo struct {
	Name   string
	Signed bool
	Bits   int
	Min    *big.Int // inclusive lower bound
	Max    *big.Int // inclusive upper bound
}

//nolint:gochecknoglobals
var intTypeInfos = map[string]IntTypeInfo{
	"I8":  {"I8", true, 8, big.NewInt(-128), big.NewInt(127)},
	"I16": {"I16", true, 16, big.NewInt(-32768), big.NewInt(32767)},
	"I32": {"I32", true, 32, big.NewInt(-2147483648), big.NewInt(2147483647)},
	"Int": {"Int", true, 64, big.NewInt(-9223372036854775808), big.NewInt(9223372036854775807)},
	"U8":  {"U8", false, 8, big.NewInt(0), big.NewInt(255)},
	"U16": {"U16", false, 16, big.NewInt(0), big.NewInt(65535)},
	"U32": {"U32", false, 32, big.NewInt(0), big.NewInt(4294967295)},
	"U64": {"U64", false, 64, big.NewInt(0), new(big.Int).SetUint64(18446744073709551615)},
}

type RefType struct {
	Type TypeID
	Mut  bool
}

func (RefType) isTypeKind() {}

type FunType struct {
	Params []TypeID
	Return TypeID
}

func (FunType) isTypeKind() {}

type StructField struct {
	Name string
	Type TypeID
	Mut  bool
}

type StructType struct {
	Name   string
	Fields []StructField
}

func (StructType) isTypeKind() {}

type ArrayType struct {
	Elem TypeID
	Len  int64
}

func (ArrayType) isTypeKind() {}

type SliceType struct {
	Elem TypeID
}

func (SliceType) isTypeKind() {}

type AllocatorImpl int

const (
	AllocatorArena AllocatorImpl = iota + 1
)

func (a AllocatorImpl) String() string {
	switch a {
	case AllocatorArena:
		return "Arena"
	default:
		panic(base.Errorf("unknown allocator impl: %d", a))
	}
}

type AllocatorType struct {
	Impl AllocatorImpl
}

func (AllocatorType) isTypeKind() {}

const mutableRefFlag = 1 << 62

type refTypeCacheKey struct {
	TypeID
	Mut bool
}

type arrayTypeCacheKey struct {
	Elem TypeID
	Len  int64
}

type TypeStatus int

const (
	TypeOK TypeStatus = iota + 1
	TypeInProgress
	TypeFailed
	TypeDepFailed
)

func (s TypeStatus) Failed() bool {
	return s == TypeFailed || s == TypeDepFailed
}

func (s TypeStatus) String() string {
	switch s {
	case TypeOK:
		return "<ok>"
	case TypeInProgress:
		return "<in progress>"
	case TypeFailed:
		return "<failed>"
	case TypeDepFailed:
		return "<failed dependency>"
	default:
		panic(base.Errorf("unknown type status: %d", s))
	}
}

type cachedType struct {
	Type   *Type
	Status TypeStatus
}

type Engine struct {
	*ast.AST
	Debug       base.Debug
	Diagnostics base.Diagnostics
	env         *TypeEnv
	nextScopeID ScopeID
	scope       *Scope
	namespace   string
	loopStack   []ast.NodeID
	funStack    []TypeID
	typeHint    *TypeID
}

func NewEngine(a *ast.AST) *Engine {
	rootScope := NewScope(0, 0, nil)
	return &Engine{ //nolint:exhaustruct
		AST:         a,
		Debug:       base.NilDebug{},
		env:         NewRootEnv(a),
		nextScopeID: 1,
		scope:       rootScope,
	}
}

func (e *Engine) Env() *TypeEnv {
	return e.env
}

func (e *Engine) Query(nodeID ast.NodeID) (TypeID, TypeStatus) { //nolint:funlen
	if cached, ok := e.env.nodes[nodeID]; ok {
		if cached.Status.Failed() {
			return InvalidTypeID, cached.Status
		}
		return cached.Type.ID, cached.Status
	}
	// Consume the type hint so it doesn't leak into recursive queries.
	typeHint := e.typeHint
	e.typeHint = nil
	nodeDebug := e.AST.Debug(nodeID, false, 0)
	debugDedent := e.Debug.Print(1, "query start %s", nodeDebug).Indent()
	defer debugDedent()
	// Associate this node with the current scope.
	e.env.ScopeGraph.SetNodeScope(nodeID, e.scope)
	node := e.Node(nodeID)
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
		typeID, status = e.checkBlock(nodeID, nodeKind)
	case ast.Call:
		typeID, status = e.checkCall(nodeKind, nodeID, node.Span)
	case ast.Deref:
		typeID, status = e.checkDeref(nodeKind)
	case ast.Module:
		typeID, status = e.checkModule(nodeID, nodeKind)
	case ast.If:
		typeID, status = e.checkIf(nodeKind)
	case ast.For:
		typeID, status = e.checkFor(nodeID, nodeKind)
	case ast.Break:
		typeID, status = e.checkBreak(node.Span)
	case ast.Continue:
		typeID, status = e.checkContinue(node.Span)
	case ast.Fun, ast.Struct:
		cachedType, ok := e.env.nodes[nodeID]
		if !ok {
			e.forwardDeclare([]ast.NodeID{nodeID})
			cachedType, ok = e.env.nodes[nodeID]
			if !ok {
				return InvalidTypeID, TypeFailed
			}
		}
		if cachedType.Type == nil {
			return InvalidTypeID, cachedType.Status
		}
		return cachedType.Type.ID, cachedType.Status
	case ast.FunParam:
		typeID, status = e.checkFunParam(nodeID, nodeKind, node.Span)
	case ast.Return:
		typeID, status = e.checkReturn(nodeID, nodeKind, node.Span)
	case ast.StructField:
		typeID, status = e.checkStructField(nodeID, nodeKind, node.Span)
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
	case ast.Ref:
		typeID, status = e.checkRef(nodeID, nodeKind, node.Span)
	case ast.RefType:
		typeID, status = e.checkRefType(nodeID, nodeKind, node.Span)
	case ast.SimpleType:
		typeID, status = e.checkSimpleType(nodeKind, node.Span)
	case ast.String:
		typeID, status = e.checkString()
	case ast.Var:
		typeID, status = e.checkVar(nodeID, nodeKind, node.Span)
	default:
		panic(base.Errorf("unknown node kind: %T", nodeKind))
	}
	typeID, status = e.updateCachedType(node, typeID, status)
	debugDedent()
	e.Debug.Print(0, "query end   %s -> %s", nodeDebug, e.env.TypeDisplay(typeID))
	return typeID, status
}

func (e *Engine) BuildWorkList(module ast.Module) ([]FunWork, []StructWork) {
	funs := make([]FunWork, 0, len(e.env.funs))
	for _, id := range e.env.funs {
		name, ok := e.env.NamedFunRef(id)
		if !ok {
			panic(base.Errorf("no namespaced name for function node %s", id))
		}
		isMain := module.Main && name == module.Name+".main"
		if isMain {
			name = "main"
		}
		funs = append(funs, FunWork{NodeID: id, Name: name, IsMain: isMain, Env: e.env})
	}
	structs := make([]StructWork, 0, len(e.env.structs))
	for _, id := range e.env.structs {
		structs = append(structs, StructWork{NodeID: id, Env: e.env})
	}
	return funs, structs
}

func (e *Engine) isIntType(typeID TypeID) bool {
	_, ok := e.env.IntTypeInfo(typeID)
	return ok
}

// queryWithHint sets a type hint before querying a node. Int literals use the
// hint to materialize as the expected integer type (e.g. U8 instead of Int).
func (e *Engine) queryWithHint(nodeID ast.NodeID, typeHint TypeID) (TypeID, TypeStatus) {
	saved := e.typeHint
	e.typeHint = &typeHint
	typeID, status := e.Query(nodeID)
	e.typeHint = saved
	return typeID, status
}

func (e *Engine) updateCachedType(node *ast.Node, typeID TypeID, status TypeStatus) (TypeID, TypeStatus) {
	if typeID == InvalidTypeID {
		if !status.Failed() {
			panic(
				base.Errorf(
					"InvalidTypeID requires a failed status but got %s at %s",
					status,
					e.AST.Debug(node.ID, false, 0),
				),
			)
		}
		e.env.nodes[node.ID] = &cachedType{Type: nil, Status: status}
		return InvalidTypeID, status
	}
	cached, ok := e.env.reg.types[typeID]
	if !ok {
		panic(base.Errorf("type %s not found for %s", typeID, e.AST.Debug(node.ID, false, 0)))
	}
	if cached.Status != status && cached.Status != TypeInProgress {
		panic(
			base.Errorf(
				"invalid status transition for type %s of %s: %s -> %s",
				typeID,
				e.AST.Debug(node.ID, false, 0),
				cached.Status,
				status,
			),
		)
	}
	cached.Status = status
	e.env.nodes[node.ID] = cached
	if status.Failed() {
		return InvalidTypeID, status
	}
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
	if !e.isAssignableTo(rhsTypeID, lhsTypeID) {
		span := e.Node(assign.RHS).Span
		e.diag(span, "type mismatch: expected %s, got %s", e.env.TypeDisplay(lhsTypeID), e.env.TypeDisplay(rhsTypeID))
		return InvalidTypeID, TypeDepFailed
	}
	return e.env.builtins.VoidType, TypeOK
}

func (e *Engine) checkUnary(unary ast.Unary) (TypeID, TypeStatus) {
	exprTypeID, status := e.Query(unary.Expr)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	switch unary.Op {
	case ast.UnaryOpNot:
		if exprTypeID != e.env.builtins.BoolType {
			span := e.Node(unary.Expr).Span
			e.diag(
				span,
				"type mismatch: expected %s, got %s",
				e.env.TypeDisplay(e.env.builtins.BoolType),
				e.env.TypeDisplay(exprTypeID),
			)
			return InvalidTypeID, TypeDepFailed
		}
		return e.env.builtins.BoolType, TypeOK
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
		valid = e.isIntType(lhsTypeID) || lhsTypeID == e.env.builtins.BoolType
		expected = "an integer or Bool"
	case ast.BinaryOpLt, ast.BinaryOpLte, ast.BinaryOpGt, ast.BinaryOpGte:
		valid = e.isIntType(lhsTypeID)
		expected = "an integer"
	case ast.BinaryOpOr, ast.BinaryOpAnd:
		valid = lhsTypeID == e.env.builtins.BoolType
		expected = "Bool"
	case ast.BinaryOpAdd, ast.BinaryOpSub, ast.BinaryOpMul, ast.BinaryOpDiv, ast.BinaryOpMod:
		valid = e.isIntType(lhsTypeID)
		expected = "an integer"
	default:
		panic(base.Errorf("unknown binary operator: %s", binary.Op))
	}
	if !valid {
		e.diag(
			e.Node(binary.LHS).Span,
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
		span := e.Node(binary.RHS).Span
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
		return e.env.builtins.BoolType, TypeOK
	default:
		return lhsTypeID, TypeOK
	}
}

func (e *Engine) checkBlock(nodeID ast.NodeID, block ast.Block) (TypeID, TypeStatus) {
	if block.CreateScope {
		e.enterScope(nodeID)
		defer e.leaveScope()
	}
	if len(block.Exprs) == 0 {
		return e.env.builtins.VoidType, TypeOK
	}
	e.forwardDeclare(block.Exprs)
	depFailed := false
	var lastExprTypeID TypeID
	var status TypeStatus
	wouldBeDeadCode := false
	for _, exprNodeID := range block.Exprs {
		if wouldBeDeadCode {
			e.diag(e.Node(exprNodeID).Span, "unreachable code")
			return InvalidTypeID, TypeDepFailed
		}
		lastExprTypeID, status = e.Query(exprNodeID)
		if status.Failed() {
			depFailed = true
		}
		switch e.Node(exprNodeID).Kind.(type) {
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
	return e.env.builtins.VoidType, TypeOK
}

func (e *Engine) checkBreak(span base.Span) (TypeID, TypeStatus) {
	if len(e.loopStack) == 0 {
		e.diag(span, "break statement outside of loop")
		return InvalidTypeID, TypeFailed
	}
	return e.env.builtins.VoidType, TypeOK
}

func (e *Engine) checkFor(nodeID ast.NodeID, for_ ast.For) (TypeID, TypeStatus) {
	if for_.Cond != nil {
		condType, status := e.Query(*for_.Cond)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if condType != e.env.builtins.BoolType {
			condSpan := e.Node(*for_.Cond).Span
			e.diag(condSpan, "type mismatch: expected Bool, got %s", e.env.TypeDisplay(condType))
			return InvalidTypeID, TypeFailed
		}
	}
	e.loopStack = append(e.loopStack, nodeID)
	defer func() { e.loopStack = e.loopStack[:len(e.loopStack)-1] }()
	e.enterScope(nodeID)
	defer e.leaveScope()
	_, status := e.Query(for_.Body)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	// We always coerce the body to `void`.
	// todo: we don't want to ever coerce to `void` (we do this in function bodies to)
	return e.env.builtins.VoidType, TypeOK
}

func (e *Engine) checkIf(if_ ast.If) (TypeID, TypeStatus) {
	condType, status := e.Query(if_.Cond)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if condType != e.env.builtins.BoolType {
		condSpan := e.Node(if_.Cond).Span
		e.diag(condSpan, "if condition must evaluate to a boolean value, got %s", e.env.TypeDisplay(condType))
		return InvalidTypeID, TypeFailed
	}
	thenType, status := e.Query(if_.Then)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if if_.Else == nil {
		// Without an "else" branch, "if" always evaluates to "void".
		return e.env.builtins.VoidType, TypeOK
	}
	elseType, status := e.Query(*if_.Else)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	// If either of the branches returns early it is void.
	if e.BlockBreaksControlFlow(if_.Then, false) || e.BlockBreaksControlFlow(*if_.Else, false) {
		return e.env.builtins.VoidType, TypeOK
	}
	if thenType != elseType {
		e.diag(
			e.Node(*if_.Else).Span,
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
	e.bind(alloc.Name.Name, false, nodeID, e.env.builtins.ArenaType, span)
	return e.env.builtins.VoidType, TypeOK
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
	return e.env.buildSliceType(elemTypeID, nodeID, span), TypeOK
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
		if !e.isAssignableTo(defTypeID, arrType.Elem) {
			defSpan := e.Node(*alloc.DefaultValue).Span
			e.diag(defSpan, "type mismatch: expected %s, got %s",
				e.env.TypeDisplay(arrType.Elem), e.env.TypeDisplay(defTypeID))
			return InvalidTypeID, TypeFailed
		}
	} else if !e.isSafeUninitialized(arrType.Elem) {
		typeSpan := e.Node(alloc.Type).Span
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
		allocSpan := e.Node(makeSlice.Allocator).Span
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
	if lenTypeID != e.env.builtins.IntType {
		lenSpan := e.Node(makeSlice.Len).Span
		e.diag(lenSpan, "type mismatch: expected Int, got %s", e.env.TypeDisplay(lenTypeID))
		return InvalidTypeID, TypeFailed
	}
	sliceType := base.Cast[SliceType](e.env.Type(sliceTypeID).Kind)
	if makeSlice.DefaultValue != nil {
		defTypeID, defStatus := e.queryWithHint(*makeSlice.DefaultValue, sliceType.Elem)
		if defStatus.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.isAssignableTo(defTypeID, sliceType.Elem) {
			defSpan := e.Node(*makeSlice.DefaultValue).Span
			e.diag(defSpan, "type mismatch: expected %s, got %s",
				e.env.TypeDisplay(sliceType.Elem), e.env.TypeDisplay(defTypeID))
			return InvalidTypeID, TypeFailed
		}
	} else if !e.isSafeUninitialized(sliceType.Elem) {
		typeSpan := e.Node(makeSlice.Type).Span
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
		if !e.isAssignableTo(elemTyp2, elemTyp) {
			e.diag(
				e.Node(elemNodeID).Span,
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
		// Auto de-reference one level deep.
		targetTyp = e.env.Type(refTyp.Type)
	}
	var elemTypeID TypeID
	switch kind := targetTyp.Kind.(type) {
	case ArrayType:
		elemTypeID = kind.Elem
	case SliceType:
		elemTypeID = kind.Elem
	default:
		targetSpan := e.Node(index.Target).Span
		e.diag(targetSpan, "not an array or slice: %s", e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
	indexTypeID, status := e.Query(index.Index)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if indexTypeID != e.env.builtins.IntType {
		indexSpan := e.Node(index.Index).Span
		e.diag(
			indexSpan,
			"index type mismatch: expected %s, got %s",
			e.env.TypeDisplay(e.env.builtins.IntType),
			e.env.TypeDisplay(indexTypeID),
		)
		return InvalidTypeID, TypeFailed
	}
	return elemTypeID, TypeOK
}

func (e *Engine) checkNew(nodeID ast.NodeID, alloc ast.New, span base.Span) (TypeID, TypeStatus) {
	allocTypeID, allocStatus := e.Query(alloc.Allocator)
	if allocStatus.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	allocType := e.env.Type(allocTypeID)
	if _, ok := allocType.Kind.(AllocatorType); !ok {
		allocSpan := e.Node(alloc.Allocator).Span
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
		targetSpan := e.Node(alloc.Target).Span
		e.diag(targetSpan, "only structs and arrays can be allocated, got %s", e.env.TypeDisplay(typeID))
		return InvalidTypeID, TypeFailed
	}
	return e.env.buildRefType(nodeID, typeID, alloc.Mut, span), TypeOK
}

func (e *Engine) checkStructLiteral(lit ast.StructLiteral, span base.Span) (TypeID, TypeStatus) {
	structTypeID, status := e.Query(lit.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	structTyp := e.env.Type(structTypeID)
	if builtin, ok := structTyp.Kind.(BuiltInType); ok {
		return e.checkTypeConstructor(builtin, structTypeID, lit, span)
	}
	struct_, ok := structTyp.Kind.(StructType)
	if !ok {
		calleeSpan := e.Node(lit.Target).Span
		e.diag(calleeSpan, "not a struct: %s", e.env.TypeDisplay(structTypeID))
		return InvalidTypeID, TypeFailed
	}
	if len(lit.Args) != len(struct_.Fields) {
		e.diag(span, "argument count mismatch: expected %d, got %d", len(struct_.Fields), len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	for i, argNodeID := range lit.Args {
		argNode := e.Node(argNodeID)
		argTypeID, status := e.queryWithHint(argNodeID, struct_.Fields[i].Type)
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

// checkTypeConstructor handles type constructor syntax like I32(x), U8(x), Int(x).
// These look like struct literals in the parser but the target is a builtin integer type.
// The argument must itself be an integer type; non-integer types (Str, Bool, etc.) are rejected.
//
// Conversion rules (for non-literal arguments):
//   - Same signedness: target bits >= source bits (widening, identity).
//   - Unsigned to signed: target bits > source bits (need the extra bit for sign).
//   - Signed to unsigned: always rejected.
func (e *Engine) checkTypeConstructor(
	builtin BuiltInType, targetTypeID TypeID, lit ast.StructLiteral, span base.Span,
) (TypeID, TypeStatus) {
	targetInfo, isIntTarget := e.env.IntTypeInfo(targetTypeID)
	if !isIntTarget {
		calleeSpan := e.Node(lit.Target).Span
		e.diag(calleeSpan, "not a struct: %s", builtin.Name)
		return InvalidTypeID, TypeFailed
	}
	if len(lit.Args) != 1 {
		e.diag(span, "%s() takes exactly 1 argument, got %d", builtin.Name, len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	argNodeID := lit.Args[0]
	// Query with a hint so int literals materialize as the target type directly.
	argTypeID, status := e.queryWithHint(argNodeID, targetTypeID)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	// If the literal materialized as the target type, no conversion needed.
	if argTypeID == targetTypeID {
		return targetTypeID, TypeOK
	}
	argInfo, isIntArg := e.env.IntTypeInfo(argTypeID)
	if !isIntArg {
		argSpan := e.Node(argNodeID).Span
		e.diag(argSpan, "cannot convert %s to %s", e.env.TypeDisplay(argTypeID), e.env.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
	// Check conversion rules.
	allowed := false
	switch {
	case argInfo.Signed == targetInfo.Signed:
		allowed = targetInfo.Bits >= argInfo.Bits
	case !argInfo.Signed && targetInfo.Signed:
		// Unsigned to signed: need strictly more bits for the sign bit.
		allowed = targetInfo.Bits > argInfo.Bits
	case argInfo.Signed && !targetInfo.Signed:
		// Signed to unsigned: always rejected.
		allowed = false
	}
	if !allowed {
		argSpan := e.Node(argNodeID).Span
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
		// Auto de-reference one level deep.
		targetTyp = e.env.Type(refTyp.Type)
	}
	if _, ok := targetTyp.Kind.(SliceType); ok {
		if fieldAccess.Field.Name == "len" {
			return e.env.builtins.IntType, TypeOK
		}
		e.diag(fieldAccess.Field.Span, "unknown field on slice: %s", fieldAccess.Field.Name)
		return InvalidTypeID, TypeFailed
	}
	// Try struct field access, then method lookup.
	struct_, isStruct := targetTyp.Kind.(StructType)
	if isStruct {
		for _, field := range struct_.Fields {
			if field.Name == fieldAccess.Field.Name {
				return field.Type, TypeOK
			}
		}
		// No matching field — try method lookup: `TypeName.fieldName`.
		methodName := struct_.Name + "." + fieldAccess.Field.Name
		if binding, _, ok := e.scope.Lookup(methodName); ok {
			e.env.namedFunRef[nodeID] = e.env.namedFunRef[binding.Decl]
			return binding.TypeID, TypeOK
		}
		e.diag(fieldAccess.Field.Span, "unknown field: %s.%s", struct_.Name, fieldAccess.Field.Name)
		return InvalidTypeID, TypeFailed
	}
	// Method lookup on built-in types (Int, Bool, etc.).
	if builtin, ok := targetTyp.Kind.(BuiltInType); ok {
		methodName := builtin.Name + "." + fieldAccess.Field.Name
		if binding, _, ok := e.scope.Lookup(methodName); ok {
			e.env.namedFunRef[nodeID] = e.env.namedFunRef[binding.Decl]
			return binding.TypeID, TypeOK
		}
	}
	targetSpan := e.Node(fieldAccess.Target).Span
	e.diag(targetSpan, "cannot access field on non-struct type: %s", e.env.TypeDisplay(targetTypeID))
	return InvalidTypeID, TypeFailed
}

func (e *Engine) checkCall(call ast.Call, callNodeID ast.NodeID, span base.Span) (TypeID, TypeStatus) {
	calleeTypeID, status := e.Query(call.Callee)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	calleeTyp := e.env.Type(calleeTypeID)
	fun, ok := calleeTyp.Kind.(FunType)
	if !ok {
		calleeSpan := e.Node(call.Callee).Span
		e.diag(calleeSpan, "cannot call non-function: %s", e.env.TypeDisplay(calleeTypeID))
		return InvalidTypeID, TypeFailed
	}
	// Build the full argument node list. For method calls, prepend the receiver.
	// A field access is a method call only if it resolved to a named function
	// (recorded in namedFunRef), not a struct field that happens to hold a function.
	var argNodes []ast.NodeID
	fieldAccess, isFieldAccess := e.Node(call.Callee).Kind.(ast.FieldAccess)
	_, isMethod := e.env.namedFunRef[call.Callee]
	isMethod = isMethod && isFieldAccess
	if isMethod {
		argNodes = append(argNodes, fieldAccess.Target)
		e.env.methodCallReceiver[callNodeID] = fieldAccess.Target
	}
	argNodes = append(argNodes, call.Args...)
	if len(argNodes) != len(fun.Params) {
		expected := len(fun.Params)
		if isMethod {
			expected-- // report expected count without the implicit receiver
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
		if !e.isAssignableTo(argTypeID, paramTypeID) {
			argNode := e.Node(argNodeID)
			if i == 0 && isMethod {
				e.diag(argNode.Span, "type mismatch at receiver: expected %s, got %s",
					e.env.TypeDisplay(paramTypeID), e.env.TypeDisplay(argTypeID))
			} else {
				argIndex := i
				if isMethod {
					argIndex-- // report 0-based index relative to explicit args
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
		exprSpan := e.Node(deref.Expr).Span
		e.diag(exprSpan, "dereference: expected reference, got %s", e.env.TypeDisplay(exprTypeID))
		return InvalidTypeID, TypeFailed
	}
	return ref.Type, TypeOK
}

func (e *Engine) checkModule(nodeID ast.NodeID, module ast.Module) (TypeID, TypeStatus) {
	e.enterScope(nodeID)
	defer e.leaveScope()
	e.forwardDeclare(module.Decls)
	// Everything should have been forward declared, but for good measure we query again.
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
	return e.env.builtins.VoidType, TypeOK
}

func (e *Engine) forwardDeclare(nodeIDs []ast.NodeID) { //nolint:funlen
	type decl struct {
		node       *ast.Node
		typeID     TypeID
		status     TypeStatus
		cachedType *cachedType
		scope      *Scope
	}
	decls := []*decl{}

	// Find declaration nodes.
	for _, nodeID := range nodeIDs {
		node := e.Node(nodeID)
		switch node.Kind.(type) {
		case ast.Fun:
			e.env.ScopeGraph.SetNodeScope(nodeID, e.scope)
			// Functions create their own scope for their parameters and their body.
			e.enterScope(nodeID)
			scope := e.scope
			e.leaveScope()
			decls = append(decls, &decl{node, InvalidTypeID, TypeFailed, nil, scope})
			e.env.funs = append(e.env.funs, nodeID)
		case ast.Struct:
			e.env.ScopeGraph.SetNodeScope(nodeID, e.scope)
			decls = append(decls, &decl{node, InvalidTypeID, TypeFailed, nil, e.scope})
			e.env.structs = append(e.env.structs, nodeID)
		}
	}

	// Create struct types and bind their names first so that functions can
	// reference them in their parameter types.
	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Struct); !ok {
			continue
		}
		nodeKind := base.Cast[ast.Struct](decl.node.Kind)
		typeID, status := e.checkStructCreateAndBind(decl.node, nodeKind)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.reg.types[typeID]
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}

	// Create function types (with full signatures) and bind their names.
	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Fun); !ok {
			continue
		}
		nodeKind := base.Cast[ast.Fun](decl.node.Kind)
		typeID, status := e.checkFunCreateAndBind(decl.node, nodeKind, decl.scope)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.reg.types[typeID]
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}

	// Complete struct types (resolve fields).
	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Struct); !ok {
			continue
		}
		if decl.status.Failed() {
			continue
		}
		prevScope := e.scope
		e.scope = decl.scope
		defer func() { e.scope = prevScope }()
		structType := base.Cast[StructType](decl.cachedType.Type.Kind)
		structNode := base.Cast[ast.Struct](decl.node.Kind)
		decl.status, decl.cachedType.Type.Kind = e.checkStructCompleteType(structNode, structType)
		decl.typeID, decl.status = e.updateCachedType(decl.node, decl.typeID, decl.status)
	}

	// Check the bodies.
	for _, decl := range decls {
		if decl.status.Failed() {
			continue
		}
		typeKind := decl.cachedType.Type.Kind
		prevScope := e.scope
		e.scope = decl.scope
		defer func() { e.scope = prevScope }()
		switch nodeKind := decl.node.Kind.(type) {
		case ast.Fun:
			e.namespace += nodeKind.Name.Name + "."
			funType := base.Cast[FunType](typeKind)
			e.checkFunBody(nodeKind, decl.cachedType.Type.ID, funType)
			e.namespace = e.namespace[:len(e.namespace)-len(nodeKind.Name.Name)-1]
		case ast.Struct:
		// Structs don't have a body.
		default:
			panic(base.Errorf("node kind not supported: %T", nodeKind))
		}
	}
}

func (e *Engine) checkFunCreateAndBind(node *ast.Node, fun ast.Fun, funScope *Scope) (TypeID, TypeStatus) {
	retTypeID, status := e.Query(fun.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if _, ok := e.env.Type(retTypeID).Kind.(AllocatorType); ok {
		e.diag(e.Node(fun.ReturnType).Span, "cannot return an allocator from a function")
		return InvalidTypeID, TypeFailed
	}
	// Query params inside the function's own scope so that param nodes are
	// associated with the correct scope in the ScopeGraph.
	prevScope := e.scope
	e.scope = funScope
	paramTypeIDs := make([]TypeID, len(fun.Params))
	for i, paramNodeID := range fun.Params {
		paramTypeID, status := e.Query(paramNodeID)
		if status.Failed() {
			e.scope = prevScope
			return InvalidTypeID, TypeDepFailed
		}
		paramTypeIDs[i] = paramTypeID
	}
	e.scope = prevScope
	funTyp := FunType{paramTypeIDs, retTypeID}
	cacheKey := funTypeCacheKey(funTyp)
	cached, ok := e.env.reg.funTypes[cacheKey]
	var funTypeID TypeID
	var funStatus TypeStatus
	if ok {
		funTypeID = cached.Type.ID
		funStatus = cached.Status
		e.env.nodes[node.ID] = cached
	} else {
		funTypeID = e.env.newType(funTyp, node.ID, node.Span, TypeOK)
		funStatus = TypeOK
		e.env.reg.funTypes[cacheKey] = e.env.reg.types[funTypeID]
	}
	if !e.bind(fun.Name.Name, false, node.ID, funTypeID, fun.Name.Span) {
		return InvalidTypeID, TypeFailed
	}
	e.env.namedFunRef[node.ID] = e.namespace + fun.Name.Name
	return funTypeID, funStatus
}

func (e *Engine) checkStructCreateAndBind(node *ast.Node, structNode ast.Struct) (TypeID, TypeStatus) {
	structTypeID := e.env.newType(StructType{structNode.Name.Name, []StructField{}}, node.ID, node.Span, TypeInProgress)
	if !e.bind(structNode.Name.Name, false, node.ID, structTypeID, structNode.Name.Span) {
		return structTypeID, TypeFailed
	}
	return structTypeID, TypeInProgress
}

func (e *Engine) checkStructCompleteType(structNode ast.Struct, structType StructType) (TypeStatus, StructType) {
	fields := make([]StructField, len(structNode.Fields))
	for i, fieldNodeID := range structNode.Fields {
		fieldTypeID, status := e.Query(fieldNodeID)
		if status.Failed() {
			return TypeDepFailed, structType
		}
		fieldNode := base.Cast[ast.StructField](e.Node(fieldNodeID).Kind)
		fields[i] = StructField{fieldNode.Name.Name, fieldTypeID, fieldNode.Mut}
	}
	structType.Fields = fields
	return TypeOK, structType
}

func (e *Engine) checkFunBody(funNode ast.Fun, funTypeID TypeID, funType FunType) {
	e.funStack = append(e.funStack, funTypeID)
	defer func() { e.funStack = e.funStack[:len(e.funStack)-1] }()
	for i, paramNodeID := range funNode.Params {
		paramNode := base.Cast[ast.FunParam](e.Node(paramNodeID).Kind)
		paramTypeID := funType.Params[i]
		// Params are never reassignable - mutability of the *binding* is always false.
		if !e.bind(paramNode.Name.Name, false, paramNodeID, paramTypeID, paramNode.Name.Span) {
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
	blockNode := e.Node(funNode.Block)
	block := base.Cast[ast.Block](blockNode.Kind)
	// If the block ends with an return expr, we don't need to check any further.
	if e.BlockReturns(funNode.Block) {
		return
	}
	// If the function returns void, we coerce the body to void.
	if funType.Return != e.env.builtins.VoidType && !e.isAssignableTo(blockTypeID, funType.Return) {
		// We want the span of the last expression for better diagnostics.
		diagSpan := blockNode.Span
		if len(block.Exprs) > 0 {
			lastNode := e.Node(block.Exprs[len(block.Exprs)-1])
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

func (e *Engine) checkFunParam(_ ast.NodeID, funParam ast.FunParam, _ base.Span) (TypeID, TypeStatus) {
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
	cached, ok := e.env.reg.funTypes[cacheKey]
	if ok {
		return cached.Type.ID, cached.Status
	}
	typeID := e.env.newType(funTyp, nodeID, span, TypeOK)
	e.env.reg.funTypes[cacheKey] = e.env.reg.types[typeID]
	return typeID, TypeOK
}

func (e *Engine) checkReturn(_ ast.NodeID, return_ ast.Return, span base.Span) (TypeID, TypeStatus) {
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
		span := e.Node(return_.Expr).Span
		e.diag(
			span,
			"return type mismatch: expected %s, got %s",
			e.env.TypeDisplay(funType.Return),
			e.env.TypeDisplay(exprTypeID),
		)
		return InvalidTypeID, TypeFailed
	}
	return e.env.builtins.VoidType, TypeOK
}

func (e *Engine) checkStructField(_ ast.NodeID, structField ast.StructField, _ base.Span) (TypeID, TypeStatus) {
	typeID, status := e.Query(structField.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	return typeID, TypeOK
}

func (e *Engine) checkIdent(nodeID ast.NodeID, ident ast.Ident, span base.Span) (TypeID, TypeStatus) {
	if typeID, ok := e.env.builtins.Names[ident.Name]; ok {
		if _, ok := e.env.Type(typeID).Kind.(FunType); ok {
			e.env.namedFunRef[nodeID] = ident.Name
		}
		return typeID, TypeOK
	}
	binding, _, ok := e.scope.Lookup(ident.Name)
	if !ok {
		e.diag(span, "symbol not defined: %s", ident.Name)
		return InvalidTypeID, TypeFailed
	}
	if cached, ok := e.env.reg.types[binding.TypeID]; ok && cached.Status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if _, ok := e.Node(binding.Decl).Kind.(ast.Fun); ok {
		e.env.namedFunRef[nodeID] = e.env.namedFunRef[binding.Decl]
	}
	return binding.TypeID, TypeOK
}

func (e *Engine) checkBool() (TypeID, TypeStatus) {
	typeID, ok := e.env.builtins.Names["Bool"]
	if !ok {
		panic(base.Errorf("builtin type Bool not found"))
	}
	return typeID, TypeOK
}

func (e *Engine) checkInt(intNode ast.Int, span base.Span, typeHint *TypeID) (TypeID, TypeStatus) {
	target := e.env.builtins.IntType
	if typeHint != nil {
		if _, ok := e.env.IntTypeInfo(*typeHint); ok {
			target = *typeHint
		}
	}
	info, _ := e.env.IntTypeInfo(target)
	if intNode.Value.Cmp(info.Min) < 0 || intNode.Value.Cmp(info.Max) > 0 {
		e.diag(span, "value %s out of range for %s (%s..%s)",
			intNode.Value, info.Name, info.Min, info.Max)
		return InvalidTypeID, TypeFailed
	}
	return target, TypeOK
}

func (e *Engine) checkRef(
	nodeID ast.NodeID, ref ast.Ref, span base.Span,
) (TypeID, TypeStatus) {
	binding, _, ok := e.scope.Lookup(ref.Name.Name)
	if !ok {
		e.diag(span, "symbol not defined: %s", ref.Name.Name)
		return InvalidTypeID, TypeFailed
	}
	if ref.Mut && !binding.Mut {
		e.diag(span, "cannot take mutable reference to immutable value")
		return InvalidTypeID, TypeFailed
	}
	refTypeID := e.env.buildRefType(nodeID, binding.TypeID, ref.Mut, span)
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

func (e *Engine) checkSimpleType(
	simpleType ast.SimpleType, span base.Span,
) (TypeID, TypeStatus) {
	builtinTypeID, ok := e.env.builtins.Names[simpleType.Name.Name]
	if ok {
		return builtinTypeID, TypeOK
	}
	binding, _, ok := e.scope.Lookup(simpleType.Name.Name)
	if !ok {
		e.diag(span, "symbol not defined: %s", simpleType.Name.Name)
		return InvalidTypeID, TypeFailed
	}
	return binding.TypeID, TypeOK
}

func (e *Engine) checkString() (TypeID, TypeStatus) {
	typeID, ok := e.env.builtins.Names["Str"]
	if !ok {
		panic(base.Errorf("builtin type Str not found"))
	}
	return typeID, TypeOK
}

func (e *Engine) checkVar(
	nodeID ast.NodeID, varNode ast.Var, span base.Span,
) (TypeID, TypeStatus) {
	exprTypeID, status := e.Query(varNode.Expr)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if exprTypeID == e.env.builtins.VoidType {
		e.diag(span, "cannot assign void to a variable")
		return InvalidTypeID, TypeFailed
	}
	if !e.bind(varNode.Name.Name, varNode.Mut, nodeID, exprTypeID, varNode.Name.Span) {
		return InvalidTypeID, TypeFailed
	}
	return e.env.builtins.VoidType, TypeOK
}

func (e *Engine) isAssignableTo(got TypeID, expected TypeID) bool {
	if got == expected {
		return true
	}
	// A &mut T is assignable to &T (coerce by masking off the mutable flag).
	return got&mutableRefFlag != 0 && got&^mutableRefFlag == expected
}

// isSafeUninitialized reports whether a type is safe to use on uninitialized memory.
// All integer types are safe (any bit pattern is valid), but Bool (must be 0 or 1), Str,
// references, slices, and allocators are not. Structs are safe only if all their fields are safe.
func (e *Engine) isSafeUninitialized(typeID TypeID) bool {
	if e.isIntType(typeID) {
		return true
	}
	typ := e.env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case StructType:
		for _, field := range kind.Fields {
			if !e.isSafeUninitialized(field.Type) {
				return false
			}
		}
		return true
	case ArrayType:
		return e.isSafeUninitialized(kind.Elem)
	default:
		return false
	}
}

func (e *Engine) verifyMain(fun ast.Fun) {
	if len(fun.Params) != 0 {
		firstNode := e.Node(fun.Params[0])
		lastNode := e.Node(fun.Params[len(fun.Params)-1])
		span := firstNode.Span.Combine(lastNode.Span)
		e.diag(span, "main function cannot take arguments")
	}
	retNode := e.Node(fun.ReturnType)
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
	node := e.Node(nodeID)
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
		e.diag(node.Span, "cannot assign to element of immutable array")
	default:
		e.diag(node.Span, "cannot assign to left-hand-side expression of type: %T", kind)
	}
	return InvalidTypeID, TypeFailed
}

// Check whether the given node is a valid mutable assignment target.
// Return the node's type and whether it is mutable.
func (e *Engine) isPlaceMutable(nodeID ast.NodeID) (TypeID, bool) { //nolint:funlen
	typeID, status := e.Query(nodeID)
	if status.Failed() {
		return InvalidTypeID, false
	}
	node := e.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.Ident:
		binding, _, ok := e.scope.Lookup(kind.Name)
		if !ok {
			return InvalidTypeID, false
		}
		return typeID, binding.Mut
	case ast.Deref:
		// Mutability comes from the reference being dereferenced.
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
		var mut bool
		targetTyp := e.env.Type(targetTypeID)
		if ref, ok := targetTyp.Kind.(RefType); ok {
			mut = ref.Mut
			targetTyp = e.env.Type(ref.Type)
		} else {
			_, mut = e.isPlaceMutable(kind.Target)
		}
		switch k := targetTyp.Kind.(type) {
		case ArrayType:
			return k.Elem, mut
		case SliceType:
			return k.Elem, mut
		default:
			return InvalidTypeID, false
		}
	case ast.FieldAccess:
		targetTypeID, status := e.Query(kind.Target)
		if status.Failed() {
			return InvalidTypeID, false
		}
		// Check if the container is mutable (ref mutability or root binding).
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
		// Check if the field itself is declared mut.
		structType := base.Cast[StructType](e.env.Type(structTypeID).Kind)
		for _, field := range structType.Fields {
			if field.Name == kind.Field.Name {
				return typeID, field.Mut
			}
		}
		return typeID, false
	default:
		return typeID, false
	}
}

func (e *Engine) enterScope(nodeID ast.NodeID) {
	node := e.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.Module:
		e.namespace += kind.Name + "."
	case ast.Fun:
		e.namespace += kind.Name.Name + "."
	}
	e.scope = NewScope(nodeID, e.nextScopeID, e.scope)
	e.nextScopeID++
}

func (e *Engine) leaveScope() {
	node := e.Node(e.scope.Node)
	switch kind := node.Kind.(type) {
	case ast.Module:
		e.namespace = e.namespace[:len(e.namespace)-len(kind.Name)-1]
	case ast.Fun:
		e.namespace = e.namespace[:len(e.namespace)-len(kind.Name.Name)-1]
	}
	e.scope = e.scope.Parent
}

func (e *Engine) bind(name string, mut bool, nodeID ast.NodeID, typeID TypeID, span base.Span) bool {
	scope := e.scope
	if !scope.Bind(name, mut, nodeID, typeID) {
		e.diag(span, "symbol already defined: %s", name)
		return false
	}
	return true
}

func (e *Engine) diag(span base.Span, msg string, msgArgs ...any) {
	e.Diagnostics = append(e.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}
