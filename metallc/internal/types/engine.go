package types

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

const InvalidTypeID = TypeID(0)

type TypeID uint64

func (id TypeID) String() string {
	return fmt.Sprintf("t%d", id)
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

const mutableRefFlag = 1 << 62

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
	Debug       base.Debug
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
	boolType    TypeID
}

func NewEngine(a *ast.AST) *Engine {
	rootScope := NewScope(0, nil)
	e := &Engine{ //nolint:exhaustruct
		AST:         a,
		Debug:       base.NilDebug{},
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
	boolType := e.newType(BuiltInType{"Bool"}, 0, span, TypeOK)
	printStrFun := e.newType(FunType{[]TypeID{strType}, voidType}, 0, span, TypeOK)
	printIntFun := e.newType(FunType{[]TypeID{intType}, voidType}, 0, span, TypeOK)
	printBoolFun := e.newType(FunType{[]TypeID{boolType}, voidType}, 0, span, TypeOK)

	e.voidType = voidType
	e.boolType = boolType
	e.builtins = map[string]TypeID{
		"Int":        intType,
		"Str":        strType,
		"Bool":       boolType,
		"void":       e.voidType,
		"print_str":  printStrFun,
		"print_int":  printIntFun,
		"print_bool": printBoolFun,
	}
	return e
}

func (e *Engine) TypeDisplay(typeID TypeID) string {
	if typeID == InvalidTypeID {
		return "<invalid>"
	}
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
	case StructType:
		var sb strings.Builder
		fmt.Fprintf(&sb, "struct %s(", kind.Name)
		for i, field := range kind.Fields {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%s %s", field.Name, e.TypeDisplay(field.Type))
		}
		sb.WriteString(")")
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
	nodeDebug := e.AST.Debug(nodeID, false, 0)
	debugDedent := e.Debug.Print(1, "query start %s", nodeDebug).Indent()
	defer debugDedent()
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
	case ast.If:
		typeID, status = e.checkIf(nodeKind)
	case ast.Fun, ast.Struct:
		cachedType, ok := e.nodes[nodeID]
		if !ok {
			e.forwardDeclare([]ast.NodeID{nodeID})
			cachedType, ok = e.nodes[nodeID]
			if !ok {
				return InvalidTypeID, TypeFailed
			}
		}
		return cachedType.Type.ID, cachedType.Status
	case ast.FunParam:
		typeID, status = e.checkFunParam(nodeID, nodeKind, node.Span)
	case ast.StructField:
		typeID, status = e.checkStructField(nodeID, nodeKind, node.Span)
	case ast.StructLiteral:
		typeID, status = e.checkStructLiteral(nodeKind, node.Span)
	case ast.FieldAccess:
		typeID, status = e.checkFieldAccess(nodeKind)
	case ast.Ident:
		typeID, status = e.checkIdent(nodeKind, node.Span)
	case ast.Bool:
		typeID, status = e.checkBool()
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
	typeID, status = e.updateCachedType(node, typeID, status)
	debugDedent()
	e.Debug.Print(0, "query end   %s -> %s", nodeDebug, e.TypeDisplay(typeID))
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
		panic(base.Errorf("type not found for %s", e.AST.Debug(nodeID, false, 0)))
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
	case StructType:
		for _, field := range kind.Fields {
			fieldTyp := e.Type(field.Type)
			f(fieldTyp, e)
		}
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
		e.nodes[node.ID] = &cachedType{Type: nil, Status: status}
		return InvalidTypeID, status
	}
	cached, ok := e.types[typeID]
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
	e.nodes[node.ID] = cached
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
	rhsTypeID, status := e.Query(assign.RHS)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if !e.isAssignableTo(rhsTypeID, lhsTypeID) {
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
	e.forwardDeclare(block.Exprs)
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

func (e *Engine) checkIf(if_ ast.If) (TypeID, TypeStatus) {
	condType, status := e.Query(if_.Cond)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if condType != e.boolType {
		condSpan := e.Node(if_.Cond).Span
		e.diag(condSpan, "if condition must evaluate to a boolean value, got %s", e.TypeDisplay(condType))
		return InvalidTypeID, TypeFailed
	}
	thenType, status := e.Query(if_.Then)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if if_.Else == nil {
		// Without an "else" branch, "if" always evaluates to "void".
		return e.voidType, TypeOK
	}
	elseType, status := e.Query(*if_.Else)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if thenType != elseType {
		e.diag(
			e.Node(*if_.Else).Span,
			"if branch type mismatch: expected %s, got %s",
			e.TypeDisplay(thenType),
			e.TypeDisplay(elseType),
		)
		return InvalidTypeID, TypeFailed
	}
	return thenType, TypeOK
}

func (e *Engine) checkStructLiteral(lit ast.StructLiteral, span base.Span) (TypeID, TypeStatus) {
	structTypeID, status := e.Query(lit.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	structTyp := e.Type(structTypeID)
	struct_, ok := structTyp.Kind.(StructType)
	if !ok {
		calleeSpan := e.Node(lit.Target).Span
		e.diag(calleeSpan, "cannot call non-function: %s", e.TypeDisplay(structTypeID))
		return InvalidTypeID, TypeFailed
	}
	if len(lit.Args) != len(struct_.Fields) {
		e.diag(span, "argument count mismatch: expected %d, got %d", len(struct_.Fields), len(lit.Args))
		return InvalidTypeID, TypeFailed
	}
	for i, argNodeID := range lit.Args {
		argNode := e.Node(argNodeID)
		argTypeID, status := e.Query(argNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.isAssignableTo(argTypeID, struct_.Fields[i].Type) {
			e.diag(
				argNode.Span,
				"type mismatch at argument %d: expected %s, got %s",
				i+1,
				e.TypeDisplay(struct_.Fields[i].Type),
				e.TypeDisplay(argTypeID),
			)
			return InvalidTypeID, TypeFailed
		}
	}
	return structTypeID, TypeOK
}

func (e *Engine) checkFieldAccess(fieldAccess ast.FieldAccess) (TypeID, TypeStatus) {
	targetTypeID, status := e.Query(fieldAccess.Target)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	targetTyp := e.Type(targetTypeID)
	if refTyp, ok := targetTyp.Kind.(RefType); ok {
		// Auto de-reference one level deep.
		targetTyp = e.Type(refTyp.Type)
	}
	struct_, ok := targetTyp.Kind.(StructType)
	if !ok {
		targetSpan := e.Node(fieldAccess.Target).Span
		e.diag(targetSpan, "cannot access field on a non-struct type: %s", e.TypeDisplay(targetTypeID))
		return InvalidTypeID, TypeFailed
	}
	var targetFieldTypeID TypeID
	for _, field := range struct_.Fields {
		if field.Name == fieldAccess.Field.Name {
			targetFieldTypeID = field.Type
			break
		}
	}
	if targetFieldTypeID == 0 {
		e.diag(fieldAccess.Field.Span, "unknown field: %s.%s", struct_.Name, fieldAccess.Field.Name)
		return InvalidTypeID, TypeFailed
	}
	return targetFieldTypeID, TypeOK
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
		if !e.isAssignableTo(argTypeID, fun.Params[i]) {
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
	e.forwardDeclare(file.Decls)
	// Everything should have been forward declared, but for good measure we query again.
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
			e.ScopeGraph.SetNodeScope(nodeID, e.scope)
			// Functions create their own scope for their parameters and their body.
			e.enterScope()
			scope := e.scope
			e.leaveScope()
			decls = append(decls, &decl{node, InvalidTypeID, TypeFailed, nil, scope})
		case ast.Struct:
			e.ScopeGraph.SetNodeScope(nodeID, e.scope)
			decls = append(decls, &decl{node, InvalidTypeID, TypeFailed, nil, e.scope})
		}
	}

	// Create the types and bind names.
	for _, decl := range decls {
		var typeID TypeID
		var status TypeStatus
		switch nodeKind := decl.node.Kind.(type) {
		case ast.Fun:
			typeID, status = e.checkFunCreateAndBind(decl.node, nodeKind)
		case ast.Struct:
			typeID, status = e.checkStructCreateAndBind(decl.node, nodeKind)
		default:
			panic(base.Errorf("node kind not supported: %T", nodeKind))
		}
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.types[typeID]
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}

	// Complete the types.
	for _, decl := range decls {
		if decl.status.Failed() {
			continue
		}
		prevScope := e.scope
		e.scope = decl.scope
		defer func() { e.scope = prevScope }()
		typeKind := decl.cachedType.Type.Kind
		switch nodeKind := decl.node.Kind.(type) {
		case ast.Fun:
			funType := base.Cast[FunType](typeKind)
			decl.status, decl.cachedType.Type.Kind = e.checkFunCompleteType(nodeKind, funType)
		case ast.Struct:
			structType := base.Cast[StructType](typeKind)
			decl.status, decl.cachedType.Type.Kind = e.checkStructCompleteType(nodeKind, structType)
		default:
			panic(base.Errorf("node kind not supported: %T", nodeKind))
		}
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
			funType := base.Cast[FunType](typeKind)
			decl.status = e.checkFunBody(nodeKind, funType)
		case ast.Struct:
		// Structs don't have a body.
		default:
			panic(base.Errorf("node kind not supported: %T", nodeKind))
		}
		decl.typeID, decl.status = e.updateCachedType(decl.node, decl.typeID, decl.status)
	}
}

func (e *Engine) checkFunCreateAndBind(node *ast.Node, fun ast.Fun) (TypeID, TypeStatus) {
	retTypeID, status := e.Query(fun.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	funTypeID := e.newType(FunType{[]TypeID{}, retTypeID}, node.ID, node.Span, TypeInProgress)
	if !e.bind(fun.Name.Name, false, node.ID, funTypeID, fun.Name.Span) {
		return funTypeID, TypeFailed
	}
	return funTypeID, TypeInProgress
}

func (e *Engine) checkStructCreateAndBind(node *ast.Node, structNode ast.Struct) (TypeID, TypeStatus) {
	structTypeID := e.newType(StructType{structNode.Name.Name, []StructField{}}, node.ID, node.Span, TypeInProgress)
	if !e.bind(structNode.Name.Name, false, node.ID, structTypeID, structNode.Name.Span) {
		return structTypeID, TypeFailed
	}
	return structTypeID, TypeInProgress
}

func (e *Engine) checkFunCompleteType(funNode ast.Fun, funType FunType) (TypeStatus, FunType) {
	paramTypeIDs := make([]TypeID, len(funNode.Params))
	for i, paramNodeID := range funNode.Params {
		paramTypeID, status := e.Query(paramNodeID)
		if status.Failed() {
			return TypeDepFailed, funType
		}
		paramTypeIDs[i] = paramTypeID
	}
	funType.Params = paramTypeIDs
	return TypeInProgress, funType
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

func (e *Engine) checkFunBody(funNode ast.Fun, funType FunType) TypeStatus {
	// Bind parameters.
	for i, paramNodeID := range funNode.Params {
		paramNode := base.Cast[ast.FunParam](e.Node(paramNodeID).Kind)
		paramTypeID := funType.Params[i]
		if !e.bind(paramNode.Name.Name, paramNode.Mut, paramNodeID, paramTypeID, paramNode.Name.Span) {
			return TypeFailed
		}
	}
	blockTypeID, status := e.Query(funNode.Block)
	if status.Failed() {
		return TypeDepFailed
	}
	blockNode := e.Node(funNode.Block)
	block, ok := blockNode.Kind.(ast.Block)
	if !ok {
		panic(base.Errorf("expected block, got %T", blockNode.Kind))
	}
	// If the function returns void, we coerce the body to void.
	if funType.Return != e.voidType && !e.isAssignableTo(blockTypeID, funType.Return) {
		// We want the span of the last expression for better diagnostics.
		diagSpan := blockNode.Span
		if len(block.Exprs) > 0 {
			lastNode := e.Node(block.Exprs[len(block.Exprs)-1])
			diagSpan = lastNode.Span
		}
		e.diag(
			diagSpan,
			"return type mismatch: expected %s, got %s",
			e.TypeDisplay(funType.Return),
			e.TypeDisplay(blockTypeID),
		)
		return TypeFailed
	}
	if funNode.Name.Name == "main" {
		e.verifyMain(funNode)
	}
	return TypeOK
}

func (e *Engine) checkFunParam(nodeID ast.NodeID, funParam ast.FunParam, span base.Span) (TypeID, TypeStatus) {
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
	return typeID, TypeOK
}

func (e *Engine) checkStructField(nodeID ast.NodeID, structField ast.StructField, span base.Span) (TypeID, TypeStatus) {
	typeID, status := e.Query(structField.Type)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if ref, ok := e.Type(typeID).Kind.(RefType); ok && structField.Mut {
		typeID = e.buildRefType(nodeID, ref.Type, true, span)
	}
	return typeID, TypeOK
}

func (e *Engine) checkIdent(ident ast.Ident, span base.Span) (TypeID, TypeStatus) {
	if typeID, ok := e.builtins[ident.Name]; ok {
		return typeID, TypeOK
	}
	binding, _, ok := e.scope.Lookup(ident.Name)
	if !ok {
		e.diag(span, "symbol not defined: %s", ident.Name)
		return InvalidTypeID, TypeFailed
	}
	if cached, ok := e.types[binding.TypeID]; ok && cached.Status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	return binding.TypeID, TypeOK
}

func (e *Engine) checkBool() (TypeID, TypeStatus) {
	typeID, ok := e.builtins["Bool"]
	if !ok {
		panic(base.Errorf("builtin type Bool not found"))
	}
	return typeID, TypeOK
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
	binding, _, ok := e.scope.Lookup(ref.Name.Name)
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
	binding, _, ok := e.scope.Lookup(simpleType.Name.Name)
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

func (e *Engine) buildRefType(nodeID ast.NodeID, innerTypeID TypeID, mut bool, span base.Span) TypeID {
	cacheKey := refTypeCacheKey{innerTypeID, mut}
	if cached, ok := e.refTypes[cacheKey]; ok {
		return cached.Type.ID
	}
	if !mut {
		refTypeID := e.newType(RefType{innerTypeID, mut}, nodeID, span, TypeOK)
		e.refTypes[cacheKey] = e.types[refTypeID]
		return refTypeID
	}
	immutableRefTypID := e.buildRefType(0, innerTypeID, false, span)
	refTypeID := immutableRefTypID | mutableRefFlag
	e.newTypeWithID(refTypeID, RefType{innerTypeID, true}, nodeID, span, TypeOK)
	e.refTypes[cacheKey] = e.types[refTypeID]
	return refTypeID
}

func (e *Engine) isAssignableTo(got TypeID, expected TypeID) bool {
	if got == expected {
		return true
	}
	// A &mut T is assignable to &T (coerce by masking off the mutable flag).
	return got&mutableRefFlag != 0 && got&^mutableRefFlag == expected
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
			e.TypeDisplay(exprTypeID))
	case ast.FieldAccess:
		targetTypeID, _ := e.Query(kind.Target)
		var containerMut bool
		if ref, ok := e.Type(targetTypeID).Kind.(RefType); ok {
			containerMut = ref.Mut
		} else {
			_, containerMut = e.isPlaceMutable(kind.Target)
		}
		if containerMut {
			e.diag(node.Span, "cannot assign to immutable field: %s", kind.Field.Name)
		} else {
			e.diag(node.Span, "cannot assign to field of immutable value")
		}
	default:
		e.diag(node.Span, "cannot assign to left-hand-side expression of type: %T", kind)
	}
	return InvalidTypeID, TypeFailed
}

// Check whether the given node is a valid mutable assignment target.
// Return the node's type and whether it is mutable.
func (e *Engine) isPlaceMutable(nodeID ast.NodeID) (TypeID, bool) {
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
		ref := base.Cast[RefType](e.Type(exprTypeID).Kind)
		return typeID, ref.Mut
	case ast.FieldAccess:
		targetTypeID, status := e.Query(kind.Target)
		if status.Failed() {
			return InvalidTypeID, false
		}
		// Check if the container is mutable (ref mutability or root binding).
		var containerMut bool
		var structTypeID TypeID
		if ref, ok := e.Type(targetTypeID).Kind.(RefType); ok {
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
		structType := base.Cast[StructType](e.Type(structTypeID).Kind)
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

func (e *Engine) enterScope() {
	e.scope = NewScope(e.nextScopeID, e.scope)
	e.nextScopeID++
}

func (e *Engine) leaveScope() {
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

func (e *Engine) newType(kind TypeKind, nodeID ast.NodeID, span base.Span, status TypeStatus) TypeID {
	newTypeID := e.nextID
	e.nextID++
	e.newTypeWithID(newTypeID, kind, nodeID, span, status)
	return newTypeID
}

func (e *Engine) newTypeWithID(
	typeID TypeID, kind TypeKind, nodeID ast.NodeID, span base.Span, status TypeStatus,
) {
	// todo: `nodeID != 0` is a workaround for the current special nature of builtin types and we
	// 		 abuse it elsewhere (buildRefType).
	if cached, ok := e.nodes[nodeID]; nodeID != 0 && ok {
		panic(base.Errorf("type already set for %s: %s", e.AST.Debug(nodeID, false, 0), cached.Type.ID))
	}
	typ := &Type{ID: typeID, Span: span, Kind: kind}
	cached := &cachedType{Type: typ, Status: status}
	e.types[typeID] = cached
	e.nodes[nodeID] = cached
}
