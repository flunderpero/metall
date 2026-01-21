package types

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

const InvalidTypeID = TypeID(0)

type TypeID int

func (id TypeID) String() string {
	return fmt.Sprintf("type_%d", id)
}

type Type struct {
	ID   TypeID
	Span base.Span
	Kind TypeKind
}

type TypeKind interface {
	isTypeKind()
}

type BuiltInType struct {
	Name string
}

func (BuiltInType) isTypeKind() {}

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

type refTypeCacheKey struct {
	TypeID
	Mut bool
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
	Diagnostics base.Diagnostics
	ScopeGraph  *ScopeGraph
	nodes       map[ast.NodeID]*cachedType
	types       map[TypeID]*cachedType
	refTypes    map[refTypeCacheKey]*cachedType
	nextID      TypeID
	nextScopeID ScopeID
	scope       *Scope
	builtins    map[string]TypeID
	voidType    TypeID
}

func NewEngine(a *ast.AST) *Engine {
	rootScope := NewScope(0, nil)
	e := &Engine{ //nolint:exhaustruct
		AST:         a,
		ScopeGraph:  NewScopeGraph(),
		nodes:       map[ast.NodeID]*cachedType{},
		types:       map[TypeID]*cachedType{},
		refTypes:    map[refTypeCacheKey]*cachedType{},
		nextID:      1,
		nextScopeID: 1,
		scope:       rootScope,
	}
	span := base.NewSpan(base.NewSource("builtin", []rune{}), 0, 0)
	voidType := e.newType(BuiltInType{"void"}, 0, span, TypeOK)
	intType := e.newType(BuiltInType{"Int"}, 0, span, TypeOK)
	strType := e.newType(BuiltInType{"Str"}, 0, span, TypeOK)
	printStrFun := e.newType(FunType{[]TypeID{strType}, voidType}, 0, span, TypeOK)
	printIntFun := e.newType(FunType{[]TypeID{intType}, voidType}, 0, span, TypeOK)

	e.voidType = voidType
	e.builtins = map[string]TypeID{
		"Int":       intType,
		"Str":       strType,
		"void":      e.voidType,
		"print_str": printStrFun,
		"print_int": printIntFun,
	}
	return e
}

func (e *Engine) TypeDisplay(typeID TypeID) string {
	cached, ok := e.types[typeID]
	if !ok {
		panic(base.Errorf("type %s not found", typeID))
	}
	if cached.Status != TypeOK {
		return cached.Status.String()
	}
	switch kind := cached.Type.Kind.(type) {
	case BuiltInType:
		return kind.Name
	case RefType:
		if kind.Mut {
			return fmt.Sprintf("&mut %s", e.TypeDisplay(kind.Type))
		}
		return fmt.Sprintf("&%s", e.TypeDisplay(kind.Type))
	case FunType:
		var sb strings.Builder
		sb.WriteString("fun(")
		for i, paramTypeID := range kind.Params {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(e.TypeDisplay(paramTypeID))
		}
		sb.WriteString(") ")
		sb.WriteString(e.TypeDisplay(kind.Return))
		return sb.String()
	default:
		panic(base.Errorf("unknown type kind: %T", kind))
	}
}

func (e *Engine) Query(nodeID ast.NodeID) (TypeID, TypeStatus) { //nolint:funlen
	if cached, ok := e.nodes[nodeID]; ok {
		if cached.Status.Failed() {
			return InvalidTypeID, cached.Status
		}
		return cached.Type.ID, cached.Status
	}
	// Associate this node with the current scope.
	e.ScopeGraph.SetNodeScope(nodeID, e.scope)
	node := e.Node(nodeID)
	var typeID TypeID
	var status TypeStatus
	switch nodeKind := node.Kind.(type) {
	case ast.Assign:
		typeID, status = e.checkAssign(nodeKind)
	case ast.Block:
		typeID, status = e.checkBlock(nodeKind)
	case ast.Call:
		typeID, status = e.checkCall(nodeKind, node.Span)
	case ast.Deref:
		typeID, status = e.checkDeref(nodeKind)
	case ast.File:
		typeID, status = e.checkFile(nodeKind)
	case ast.Fun:
		typeID, status = e.checkFun(nodeID, nodeKind)
	case ast.FunParam:
		typeID, status = e.checkFunParam(nodeID, nodeKind, node.Span)
	case ast.Ident:
		typeID, status = e.checkIdent(nodeKind, node.Span)
	case ast.Int:
		typeID, status = e.checkInt()
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
	// Update cache.
	if typeID == InvalidTypeID {
		if !status.Failed() {
			panic(base.Errorf("InvalidTypeID requires a failed status at node %s: %#v", nodeID, node))
		}
		e.nodes[nodeID] = &cachedType{Type: nil, Status: status}
		return InvalidTypeID, status
	}
	cached, ok := e.types[typeID]
	if !ok {
		panic(base.Errorf("type %s not found for node %s: %#v", typeID, nodeID, node))
	}
	if cached.Status != status && cached.Status != TypeInProgress {
		panic(base.Errorf("invalid status transition for type %s: %s -> %s", typeID, cached.Status, status))
	}
	cached.Status = status
	e.nodes[nodeID] = cached
	if status.Failed() {
		return InvalidTypeID, status
	}
	return typeID, status
}

func (e *Engine) Type(typeID TypeID) *Type {
	cached, ok := e.types[typeID]
	if !ok {
		panic(base.Errorf("type %s not found", typeID))
	}
	return cached.Type
}

func (e *Engine) TypeOfNode(nodeID ast.NodeID) *Type {
	cached, ok := e.nodes[nodeID]
	if !ok {
		panic(base.Errorf("type not found for AST node %s", nodeID))
	}
	return cached.Type
}

func (e *Engine) WalkType(typeID TypeID, f func(typ *Type, e *Engine)) {
	typ := e.Type(typeID)
	switch kind := typ.Kind.(type) {
	case BuiltInType:
	case RefType:
		innerTyp := e.Type(kind.Type)
		f(innerTyp, e)
	case FunType:
		for _, paramTypeID := range kind.Params {
			paramTyp := e.Type(paramTypeID)
			f(paramTyp, e)
		}
		retTyp := e.Type(kind.Return)
		f(retTyp, e)
	default:
		panic(base.Errorf("unknown type kind: %T", kind))
	}
}

func (e *Engine) IterTypes(f func(*Type, TypeStatus) bool) {
	for _, cached := range e.types {
		if !f(cached.Type, cached.Status) {
			return
		}
	}
}

func (e *Engine) Scope() *Scope {
	return e.scope
}

func (e *Engine) checkAssign(assign ast.Assign) (TypeID, TypeStatus) {
	lhsTypeID, status := e.typeOfPlace(assign.LHS)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	rhsTypeID, status := e.Query(assign.RHS)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if lhsTypeID != rhsTypeID {
		rhsSpan := e.Node(assign.RHS).Span
		e.diag(rhsSpan, "type mismatch: expected %s, got %s", e.TypeDisplay(lhsTypeID), e.TypeDisplay(rhsTypeID))
		return InvalidTypeID, TypeDepFailed
	}
	return e.voidType, TypeOK
}

func (e *Engine) checkBlock(block ast.Block) (TypeID, TypeStatus) {
	if block.CreateScope {
		e.enterScope()
		defer e.leaveScope()
	}
	if len(block.Exprs) == 0 {
		return e.voidType, TypeOK
	}
	depFailed := false
	var lastExprTypeID TypeID
	var status TypeStatus
	for _, exprNodeID := range block.Exprs {
		lastExprTypeID, status = e.Query(exprNodeID)
		if status.Failed() {
			depFailed = true
		}
	}
	if depFailed {
		return InvalidTypeID, TypeDepFailed
	}
	return lastExprTypeID, TypeOK
}

func (e *Engine) checkCall(call ast.Call, span base.Span) (TypeID, TypeStatus) {
	calleeTypeID, status := e.Query(call.Callee)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	calleeTyp := e.Type(calleeTypeID)
	fun, ok := calleeTyp.Kind.(FunType)
	if !ok {
		calleeSpan := e.Node(call.Callee).Span
		e.diag(calleeSpan, "cannot call non-function: %s", e.TypeDisplay(calleeTypeID))
		return InvalidTypeID, TypeFailed
	}
	if len(call.Args) != len(fun.Params) {
		e.diag(span, "argument count mismatch: expected %d, got %d", len(fun.Params), len(call.Args))
		return InvalidTypeID, TypeFailed
	}
	for i, argNodeID := range call.Args {
		argNode := e.Node(argNodeID)
		argTypeID, status := e.Query(argNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if argTypeID != fun.Params[i] {
			e.diag(
				argNode.Span,
				"type mismatch at argument %d: expected %s, got %s",
				i+1,
				e.TypeDisplay(fun.Params[i]),
				e.TypeDisplay(argTypeID),
			)
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
	exprTyp := e.Type(exprTypeID)
	ref, ok := exprTyp.Kind.(RefType)
	if !ok {
		exprSpan := e.Node(deref.Expr).Span
		e.diag(exprSpan, "dereference: expected reference, got %s", e.TypeDisplay(exprTypeID))
		return InvalidTypeID, TypeFailed
	}
	return ref.Type, TypeOK
}

func (e *Engine) checkFile(file ast.File) (TypeID, TypeStatus) {
	e.enterScope()
	defer e.leaveScope()
	depFailed := false
	for _, declNodeID := range file.Decls {
		_, status := e.Query(declNodeID)
		if status.Failed() {
			depFailed = true
		}
	}
	if depFailed {
		return InvalidTypeID, TypeDepFailed
	}
	return e.voidType, TypeOK
}

func (e *Engine) checkFun(nodeID ast.NodeID, fun ast.Fun) (TypeID, TypeStatus) {
	node := e.Node(nodeID)
	retTypeID, status := e.Query(fun.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	// Enter scope for parameters and body.
	e.enterScope()
	defer e.leaveScope()
	paramTypeIDs := make([]TypeID, len(fun.Params))
	for i, paramNodeID := range fun.Params {
		paramTypeID, status := e.Query(paramNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		paramTypeIDs[i] = paramTypeID
	}
	// We need to create the new type before we check the block because
	// the block might refer to it (recursion).
	funTypeID := e.newType(FunType{paramTypeIDs, retTypeID}, nodeID, node.Span, TypeInProgress)
	// Bind the function in the parent scope.
	if !e.bind(fun.Name.Name, false, nodeID, funTypeID, fun.Name.Span) {
		return funTypeID, TypeFailed
	}
	blockTypeID, status := e.Query(fun.Block)
	if status.Failed() {
		return funTypeID, TypeDepFailed
	}
	blockNode := e.Node(fun.Block)
	block, ok := blockNode.Kind.(ast.Block)
	if !ok {
		panic(base.Errorf("expected block, got %T", blockNode.Kind))
	}
	// If the function returns void, we coerce the body to void.
	if retTypeID != e.voidType && blockTypeID != retTypeID {
		// We want the span of the last expression for better diagnostics.
		diagSpan := blockNode.Span
		if len(block.Exprs) > 0 {
			lastNode := e.Node(block.Exprs[len(block.Exprs)-1])
			diagSpan = lastNode.Span
		}
		e.diag(
			diagSpan,
			"return type mismatch: expected %s, got %s",
			e.TypeDisplay(retTypeID),
			e.TypeDisplay(blockTypeID),
		)
		return funTypeID, TypeFailed
	}
	if fun.Name.Name == "main" {
		e.verifyMain(fun)
	}
	return funTypeID, TypeOK
}

func (e *Engine) checkFunParam(
	nodeID ast.NodeID, funParam ast.FunParam, span base.Span,
) (TypeID, TypeStatus) {
	typeID, status := e.Query(funParam.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	typ := e.Type(typeID)
	ref, isRef := typ.Kind.(RefType)
	if funParam.Mut && !isRef {
		e.diag(span, "only reference types can be mutable parameters")
		return InvalidTypeID, TypeFailed
	}
	if isRef {
		typeID = e.buildRefType(nodeID, ref.Type, funParam.Mut, span)
	}
	if !e.bind(funParam.Name.Name, funParam.Mut, nodeID, typeID, funParam.Name.Span) {
		return InvalidTypeID, TypeFailed
	}
	return typeID, TypeOK
}

func (e *Engine) checkIdent(ident ast.Ident, span base.Span) (TypeID, TypeStatus) {
	if typeID, ok := e.builtins[ident.Name]; ok {
		return typeID, TypeOK
	}
	binding, ok := e.scope.Lookup(ident.Name)
	if !ok {
		e.diag(span, "symbol not defined: %s", ident.Name)
		return InvalidTypeID, TypeFailed
	}
	return binding.TypeID, TypeOK
}

func (e *Engine) checkInt() (TypeID, TypeStatus) {
	typeID, ok := e.builtins["Int"]
	if !ok {
		panic(base.Errorf("builtin type Int not found"))
	}
	return typeID, TypeOK
}

func (e *Engine) checkRef(
	nodeID ast.NodeID, ref ast.Ref, span base.Span,
) (TypeID, TypeStatus) {
	binding, ok := e.scope.Lookup(ref.Name.Name)
	if !ok {
		e.diag(span, "symbol not defined: %s", ref.Name.Name)
		return InvalidTypeID, TypeFailed
	}
	refTypeID := e.buildRefType(nodeID, binding.TypeID, binding.Mut, span)
	return refTypeID, TypeOK
}

func (e *Engine) checkRefType(
	nodeID ast.NodeID, refType ast.RefType, span base.Span,
) (TypeID, TypeStatus) {
	innerTypeID, status := e.Query(refType.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	return e.buildRefType(nodeID, innerTypeID, false, span), TypeOK
}

func (e *Engine) checkSimpleType(
	simpleType ast.SimpleType, span base.Span,
) (TypeID, TypeStatus) {
	builtinTypeID, ok := e.builtins[simpleType.Name.Name]
	if ok {
		return builtinTypeID, TypeOK
	}
	binding, ok := e.scope.Lookup(simpleType.Name.Name)
	if !ok {
		e.diag(span, "symbol not defined: %s", simpleType.Name.Name)
		return InvalidTypeID, TypeFailed
	}
	return binding.TypeID, TypeOK
}

func (e *Engine) checkString() (TypeID, TypeStatus) {
	typeID, ok := e.builtins["Str"]
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
	if exprTypeID == e.voidType {
		e.diag(span, "cannot assign void to a variable")
		return InvalidTypeID, TypeFailed
	}
	exprTyp := e.Type(exprTypeID)
	ref, isRef := exprTyp.Kind.(RefType)
	if varNode.Mut && isRef && !ref.Mut {
		exprSpan := e.Node(varNode.Expr).Span
		e.diag(exprSpan, "cannot take a mutable reference to an immutable value")
		return InvalidTypeID, TypeFailed
	}
	if !e.bind(varNode.Name.Name, varNode.Mut, nodeID, exprTypeID, varNode.Name.Span) {
		return InvalidTypeID, TypeFailed
	}
	return e.voidType, TypeOK
}

func (e *Engine) buildRefType(
	nodeID ast.NodeID, innerTypeID TypeID, mut bool, span base.Span,
) TypeID {
	cacheKey := refTypeCacheKey{innerTypeID, mut}
	if cached, ok := e.refTypes[cacheKey]; ok {
		return cached.Type.ID
	}
	refTypeID := e.newType(RefType{innerTypeID, mut}, nodeID, span, TypeOK)
	e.refTypes[cacheKey] = e.types[refTypeID]
	return refTypeID
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
	// Always query the node first to ensure it's type-checked and gets a scope.
	typeID, status := e.Query(nodeID)
	if status.Failed() {
		return InvalidTypeID, status
	}
	node := e.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.Ident:
		binding, ok := e.scope.Lookup(kind.Name)
		if !ok {
			// This shouldn't happen since Query succeeded.
			panic(base.Errorf("binding not found after successful Query: %s", kind.Name))
		}
		if !binding.Mut {
			e.diag(node.Span, "cannot assign to immutable variable: %s", kind.Name)
			return InvalidTypeID, TypeFailed
		}
		return typeID, TypeOK
	case ast.Deref:
		// Query returned the dereferenced type. To check mutability,
		// we need to look at the inner expression's type (the reference).
		exprTypeID, status := e.Query(kind.Expr)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		exprTyp := e.Type(exprTypeID)
		ref, ok := exprTyp.Kind.(RefType)
		if !ok {
			exprSpan := e.Node(kind.Expr).Span
			e.diag(
				exprSpan,
				"cannot assign through dereference: expected reference, got %s",
				e.TypeDisplay(exprTypeID),
			)
			return InvalidTypeID, TypeFailed
		}
		if !ref.Mut {
			exprSpan := e.Node(kind.Expr).Span
			e.diag(
				exprSpan,
				"cannot assign through dereference: expected mutable reference, got %s",
				e.TypeDisplay(exprTypeID),
			)
			return InvalidTypeID, TypeFailed
		}
		return typeID, TypeOK
	default:
		e.diag(node.Span, "left-hand side is not assignable: %T", kind)
		return InvalidTypeID, TypeFailed
	}
}

func (e *Engine) enterScope() {
	e.scope = NewScope(e.nextScopeID, e.scope)
	e.nextScopeID++
}

func (e *Engine) leaveScope() {
	e.scope = e.scope.Parent
}

func (e *Engine) bind(
	name string, mut bool, nodeID ast.NodeID, typeID TypeID, span base.Span,
) bool {
	// For functions, we need to bind in the parent scope.
	scope := e.scope
	node := e.Node(nodeID)
	if _, ok := node.Kind.(ast.Fun); ok {
		scope = e.scope.Parent
	}
	if !scope.Bind(name, mut, nodeID) {
		e.diag(span, "symbol already defined: %s", name)
		return false
	}
	binding, _ := scope.Lookup(name)
	binding.TypeID = typeID
	return true
}

func (e *Engine) diag(span base.Span, msg string, msgArgs ...any) {
	e.Diagnostics = append(e.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}

func (e *Engine) newType(
	kind TypeKind, nodeID ast.NodeID, span base.Span, status TypeStatus,
) TypeID {
	// todo: `nodeID != 0` is a workaround for the current special nature of builtin types.
	if cached, ok := e.nodes[nodeID]; nodeID != 0 && ok {
		panic(base.Errorf("type already set for AST node %s: %s", nodeID, cached.Type.ID))
	}
	newTypeID := e.nextID
	e.nextID++
	typ := &Type{ID: newTypeID, Span: span, Kind: kind}
	cached := &cachedType{Type: typ, Status: status}
	e.types[newTypeID] = cached
	e.nodes[nodeID] = cached
	return newTypeID
}
