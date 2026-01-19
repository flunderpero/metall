package check

import (
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type (
	TypeKind int
	TypeID   int
)

const (
	TypeFun TypeKind = iota + 1
	TypeInt
	TypeRef
	TypeStr
	TypeVoid
)

type typBase struct {
	ID   TypeID
	Span base.Span
}

type Type struct {
	Kind TypeKind

	BuiltIn *BuiltInType
	Fun     *FunType
	Ref     *RefType
}

func NewBuiltInType(kind TypeKind, typ *BuiltInType) Type {
	return Type{kind, typ, nil, nil}
}

func NewFunType(typ *FunType) Type {
	return Type{TypeFun, nil, typ, nil}
}

func NewRefType(typ *RefType) Type {
	return Type{TypeRef, nil, nil, typ}
}

func (t Type) ID() TypeID {
	switch t.Kind {
	case TypeInt, TypeStr, TypeVoid:
		return t.BuiltIn.ID
	case TypeFun:
		return t.Fun.ID
	case TypeRef:
		return t.Ref.ID
	default:
		panic(base.Errorf("unknown type kind: %d", t.Kind))
	}
}

func (t Type) Span() base.Span {
	switch t.Kind {
	case TypeInt, TypeStr, TypeVoid:
		return t.BuiltIn.Span
	case TypeFun:
		return t.Fun.Span
	case TypeRef:
		return t.Ref.Span
	default:
		panic(base.Errorf("unknown type kind: %v", t))
	}
}

func (t Type) String() string {
	switch t.Kind {
	case TypeInt:
		return "Int"
	case TypeStr:
		return "Str"
	case TypeVoid:
		return "void"
	case TypeFun:
		return t.Fun.String()
	case TypeRef:
		return t.Ref.String()
	default:
		panic(base.Errorf("unknown type kind: %d", t.Kind))
	}
}

func (t Type) IsAssignableTo(other Type) bool {
	return t.ID() == other.ID()
}

type BuiltInType struct {
	typBase
}

type RefType struct {
	typBase
	Type Type
	Mut  bool
}

func (t *RefType) String() string {
	if t.Mut {
		return "&mut " + t.Type.String()
	}
	return "&" + t.Type.String()
}

type FunType struct {
	typBase
	Params []Type
	Return Type
}

func (t *FunType) String() string {
	var sb strings.Builder
	sb.WriteString("fun(")
	for i, param := range t.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(param.String())
	}
	sb.WriteString(") ")
	sb.WriteString(t.Return.String())
	return sb.String()
}

type RefTypeCacheKey struct {
	TypeID
	Mut bool
}

type TypeEnv struct {
	Types    map[ast.NodeID]Type
	Bindings map[ast.NodeID]Binding
	RefTypes map[RefTypeCacheKey]Type
}

func NewTypeEnv() TypeEnv {
	return TypeEnv{map[ast.NodeID]Type{}, map[ast.NodeID]Binding{}, map[RefTypeCacheKey]Type{}}
}

func (e *TypeEnv) LookupType(id ast.NodeID, span base.Span) (Type, *base.Diagnostic) {
	if t, ok := e.Types[id]; ok {
		return t, nil
	}
	return Type{}, base.NewDiagnostic(span, "no type set for AST node %d", id)
}

func (e *TypeEnv) SetType(id ast.NodeID, t Type, span base.Span) *base.Diagnostic {
	if _, ok := e.Types[id]; ok {
		return base.NewDiagnostic(span, "type already set for AST node %d", id)
	}
	e.Types[id] = t
	return nil
}

func (e *TypeEnv) LookupBinding(id ast.NodeID, span base.Span) (Binding, *base.Diagnostic) {
	if b, ok := e.Bindings[id]; ok {
		return b, nil
	}
	return Binding{}, base.NewDiagnostic(span, "no binding set for AST node %d", id)
}

func (e *TypeEnv) SetBinding(id ast.NodeID, b Binding, span base.Span) *base.Diagnostic {
	if _, ok := e.Bindings[id]; ok {
		return base.NewDiagnostic(span, "binding already set for AST node %d", id)
	}
	e.Bindings[id] = b
	return nil
}

func (e *TypeEnv) SetRefType(id TypeID, t Type, mut bool) {
	key := RefTypeCacheKey{id, mut}
	if _, ok := e.RefTypes[key]; ok {
		panic(base.Errorf("ref type already set for type %d and mut %t", id, mut))
	}
	e.RefTypes[key] = t
}

func (e *TypeEnv) LookupRefType(id TypeID, mut bool) (Type, bool) {
	key := RefTypeCacheKey{id, mut}
	typ, ok := e.RefTypes[key]
	return typ, ok
}

type Binding struct {
	Name string
	Type Type
	Mut  bool
}

type Scope struct {
	Parent   *Scope
	Bindings map[string]Binding
}

func NewScope(parent *Scope) *Scope {
	return &Scope{parent, map[string]Binding{}}
}

func NewRootScope() *Scope {
	span := base.NewSpan(base.NewSource("builtin", []rune{}), 0, 0)
	scope := NewScope(nil)
	var id TypeID = 1
	intType := NewBuiltInType(TypeInt, &BuiltInType{typBase{id, span}})
	id++
	strType := NewBuiltInType(TypeStr, &BuiltInType{typBase{id, span}})
	id++
	voidType := NewBuiltInType(TypeVoid, &BuiltInType{typBase{id, span}})
	id++
	printStrFun := NewFunType(&FunType{typBase{id, span}, []Type{strType}, voidType})
	id++
	printIntFun := NewFunType(&FunType{typBase{id, span}, []Type{intType}, voidType})
	if err := scope.Bind("Int", intType, false, span); err != nil {
		panic(err)
	}
	if err := scope.Bind("Str", strType, false, span); err != nil {
		panic(err)
	}
	if err := scope.Bind("void", voidType, false, span); err != nil {
		panic(err)
	}
	if err := scope.Bind("print_str", printStrFun, false, span); err != nil {
		panic(err)
	}
	if err := scope.Bind("print_int", printIntFun, false, span); err != nil {
		panic(err)
	}
	return scope
}

func (s *Scope) Lookup(name string, span base.Span) (Binding, *base.Diagnostic) {
	if t, ok := s.Bindings[name]; ok {
		return t, nil
	}
	if s.Parent != nil {
		return s.Parent.Lookup(name, span)
	}
	return Binding{}, base.NewDiagnostic(span, "symbol not defined: %s", name)
}

func (s *Scope) Bind(name string, t Type, mut bool, span base.Span) *base.Diagnostic {
	if _, ok := s.Bindings[name]; ok {
		return base.NewDiagnostic(span, "symbol already defined: %s", name)
	}
	s.Bindings[name] = Binding{name, t, mut}
	return nil
}

type TypeChecker struct {
	AST         *ast.AST
	Diagnostics base.Diagnostics
	Env         TypeEnv
	Scope       *Scope
	id_         TypeID
	voidType    Type
	intType     Type
	strType     Type
}

func NewTypeChecker(a *ast.AST) *TypeChecker {
	span := base.NewSpan(base.NewSource("builtin", []rune{}), 0, 0)
	scope := NewRootScope()
	voidType, err := scope.Lookup("void", span)
	if err != nil {
		panic(err)
	}
	intType, err := scope.Lookup("Int", span)
	if err != nil {
		panic(err)
	}
	strType, err := scope.Lookup("Str", span)
	if err != nil {
		panic(err)
	}
	return &TypeChecker{
		a,
		base.Diagnostics{},
		NewTypeEnv(),
		scope,
		TypeID(100),
		voidType.Type,
		intType.Type,
		strType.Type,
	}
}

func (t *TypeChecker) Check(id ast.NodeID) {
	node := t.AST.Node(id)
	switch kind := node.Kind.(type) {
	case ast.Assign:
		t.checkAssign(id, kind, node.Span)
	case ast.Block:
		t.checkBlock(id, kind, node.Span)
	case ast.Call:
		t.checkCall(id, kind, node.Span)
	case ast.Deref:
		t.checkDeref(id, kind, node.Span)
	case ast.File:
		t.checkFile(id, kind, node.Span)
	case ast.Fun:
		t.checkFun(id, kind, node.Span)
	case ast.FunParam:
		t.checkFunParam(id, kind, node.Span)
	case ast.Ident:
		t.checkIdent(id, kind, node.Span)
	case ast.Int:
		t.checkInt(id, node.Span)
	case ast.String:
		t.checkString(id, node.Span)
	case ast.Var:
		t.checkVar(id, kind, node.Span)
	case ast.SimpleType:
		t.checkSimpleType(id, kind, node.Span)
	case ast.Ref:
		t.checkRef(id, kind, node.Span)
	case ast.RefType:
		t.checkRefType(id, kind, node.Span)
	default:
		panic(base.Errorf("unknown node kind: %T", kind))
	}
}

func (t *TypeChecker) checkRef(id ast.NodeID, ref ast.Ref, span base.Span) {
	b, ok := t.lookupInScope(ref.Name.Name, ref.Name.Span)
	if !ok {
		return
	}
	refTyp := t.buildRefType(b.Type, b.Mut, span)
	t.setType(id, refTyp, span)
}

func (t *TypeChecker) checkDeref(id ast.NodeID, deref ast.Deref, span base.Span) {
	t.Check(deref.Expr)
	exprNode := t.AST.Node(deref.Expr)
	typ, ok := t.getType(deref.Expr, exprNode.Span)
	if !ok {
		return
	}
	if typ.Kind != TypeRef {
		t.diag(exprNode.Span, "dereference: expected reference, got %s", typ)
		return
	}
	t.setType(id, typ.Ref.Type, span)
}

func (t *TypeChecker) checkVar(id ast.NodeID, v ast.Var, span base.Span) {
	t.Check(v.Expr)
	exprNode := t.AST.Node(v.Expr)
	initTyp, ok := t.getType(v.Expr, v.Name.Span)
	if !ok {
		return
	}
	if v.Mut && initTyp.Kind == TypeRef && !initTyp.Ref.Mut {
		t.diag(exprNode.Span, "cannot take a mutable reference to an immutable value")
		return
	}
	t.bindInScope(id, v.Name.Name, initTyp, v.Mut, v.Name.Span)
	t.setType(id, t.voidType, span)
}

func (t *TypeChecker) checkAssign(id ast.NodeID, assign ast.Assign, span base.Span) {
	t.Check(assign.LHS)
	t.Check(assign.RHS)
	rhsNode := t.AST.Node(assign.RHS)
	valueTyp, ok := t.getType(assign.RHS, rhsNode.Span)
	if !ok {
		return
	}
	placeType, ok := t.typeOfPlace(assign.LHS)
	if !ok {
		return
	}
	if !valueTyp.IsAssignableTo(placeType) {
		t.diag(rhsNode.Span, "type mismatch: expected %s, got %s", placeType, valueTyp)
		return
	}
	t.setType(id, t.voidType, span)
}

func (t *TypeChecker) checkBlock(id ast.NodeID, block ast.Block, span base.Span) {
	defer t.enterScope()()
	for _, expr := range block.Exprs {
		t.Check(expr)
	}
	if len(block.Exprs) == 0 {
		t.setType(id, t.voidType, span)
		return
	}
	last := block.Exprs[len(block.Exprs)-1]
	lastNode := t.AST.Node(last)
	typ, ok := t.getType(last, lastNode.Span)
	if !ok {
		return
	}
	t.setType(id, typ, span)
}

func (t *TypeChecker) checkCall(id ast.NodeID, call ast.Call, span base.Span) {
	t.Check(call.Callee)
	for _, arg := range call.Args {
		t.Check(arg)
	}
	calleeNode := t.AST.Node(call.Callee)
	typ, ok := t.getType(call.Callee, calleeNode.Span)
	if !ok {
		return
	}
	if typ.Kind != TypeFun {
		t.diag(calleeNode.Span, "callee is not a function")
		return
	}
	fun := typ.Fun
	if len(call.Args) != len(fun.Params) {
		t.diag(span, "argument count mismatch: expected %d, got %d", len(fun.Params), len(call.Args))
		return
	}
	for i, arg := range call.Args {
		argNode := t.AST.Node(arg)
		argType, ok := t.getType(arg, argNode.Span)
		if !ok {
			return
		}
		if !argType.IsAssignableTo(fun.Params[i]) {
			t.diag(argNode.Span, "type mismatch at argument %d: expected %s, got %s", i+1, fun.Params[i], argType)
			return
		}
	}
	t.setType(id, fun.Return, span)
}

func (t *TypeChecker) checkFunParam(id ast.NodeID, param ast.FunParam, span base.Span) {
	t.Check(param.Type)
	typ, ok := t.getType(param.Type, param.Name.Span)
	if !ok {
		return
	}
	if param.Mut && typ.Kind != TypeRef {
		t.diag(span, "only reference types can be mutable parameters")
		return
	}
	if !t.setType(id, typ, span) {
		return
	}
	if param.Mut && typ.Kind == TypeRef {
		typ = t.buildRefType(typ.Ref.Type, true, span)
	}
	t.bindInScope(id, param.Name.Name, typ, param.Mut, param.Name.Span)
}

func (t *TypeChecker) checkFun(id ast.NodeID, fun ast.Fun, span base.Span) {
	// We need to bind the function before we enter the scope, because
	// the function may refer to itself and we would bind it in the wrong scope.
	t.Check(fun.ReturnType)
	retNode := t.AST.Node(fun.ReturnType)
	ret, ok := t.getType(fun.ReturnType, retNode.Span)
	if !ok {
		return
	}
	params := make([]Type, len(fun.Params))
	funTyp := NewFunType(&FunType{t.base(span), params, ret})
	if !t.bindInScope(id, fun.Name.Name, funTyp, false, fun.Name.Span) {
		return
	}
	defer t.enterScope()()
	for i, paramID := range fun.Params {
		t.Check(paramID)
		param := t.AST.Node(paramID)
		typ, ok := t.getType(paramID, param.Span)
		if !ok {
			return
		}
		params[i] = typ
	}
	t.Check(fun.Block)
	blockNode := t.AST.Node(fun.Block)
	if ret.Kind != TypeVoid {
		blockTyp, ok := t.getType(fun.Block, blockNode.Span)
		if !ok {
			return
		}
		if !blockTyp.IsAssignableTo(ret) {
			diagSpan := blockTyp.Span()
			// todo: We should not need to cast to block here.
			block, ok := blockNode.Kind.(ast.Block)
			if !ok {
				panic(base.Errorf("expected block, got %T", blockNode.Kind))
			}
			if len(block.Exprs) > 0 {
				lastNode := t.AST.Node(block.Exprs[len(block.Exprs)-1])
				diagSpan = lastNode.Span
			}
			t.diag(diagSpan, "return type mismatch: expected %s, got %s", ret, blockTyp)
		}
	}
	t.setType(id, funTyp, span)
	if fun.Name.Name == "main" {
		t.verifyMain(fun)
	}
}

func (t *TypeChecker) checkIdent(id ast.NodeID, ident ast.Ident, span base.Span) {
	b, ok := t.lookupInScope(ident.Name, span)
	if !ok {
		return
	}
	t.setType(id, b.Type, span)
}

func (t *TypeChecker) checkInt(id ast.NodeID, span base.Span) {
	t.setType(id, t.intType, span)
}

func (t *TypeChecker) checkString(id ast.NodeID, span base.Span) {
	t.setType(id, t.strType, span)
}

func (t *TypeChecker) checkFile(id ast.NodeID, file ast.File, span base.Span) {
	for _, decl := range file.Decls {
		t.Check(decl)
	}
	t.setType(id, t.voidType, span)
}

func (t *TypeChecker) checkSimpleType(id ast.NodeID, typ ast.SimpleType, span base.Span) {
	b, ok := t.lookupInScope(typ.Name.Name, span)
	if !ok {
		return
	}
	t.setType(id, b.Type, span)
}

func (t *TypeChecker) checkRefType(id ast.NodeID, typ ast.RefType, span base.Span) {
	t.Check(typ.Type)
	inner, err := t.Env.LookupType(typ.Type, span)
	if err != nil {
		t.Diagnostics = append(t.Diagnostics, *err)
		return
	}
	refType := t.buildRefType(inner, false, span)
	t.setType(id, refType, span)
}

func (t *TypeChecker) buildRefType(typ Type, mut bool, span base.Span) Type {
	refType, ok := t.Env.LookupRefType(typ.ID(), mut)
	if ok {
		return refType
	}
	if typ.Kind == TypeRef {
		mut = typ.Ref.Mut
	}
	res := NewRefType(&RefType{t.base(span), typ, mut})
	t.Env.SetRefType(typ.ID(), res, mut)
	return res
}

func (t *TypeChecker) enterScope() func() {
	scope := NewScope(t.Scope)
	t.Scope = scope
	return func() {
		t.Scope = scope.Parent
	}
}

func (t *TypeChecker) lookupInScope(name string, span base.Span) (Binding, bool) {
	b, err := t.Scope.Lookup(name, span)
	if err != nil {
		t.Diagnostics = append(t.Diagnostics, *err)
		return Binding{}, false
	}
	return b, true
}

func (t *TypeChecker) bindInScope(id ast.NodeID, name string, typ Type, mut bool, span base.Span) bool {
	if err := t.Scope.Bind(name, typ, mut, span); err != nil {
		t.Diagnostics = append(t.Diagnostics, *err)
		return false
	}
	if err := t.Env.SetBinding(id, Binding{name, typ, mut}, span); err != nil {
		t.Diagnostics = append(t.Diagnostics, *err)
		return false
	}
	return true
}

func (t *TypeChecker) setType(id ast.NodeID, typ Type, span base.Span) bool {
	if err := t.Env.SetType(id, typ, span); err != nil {
		t.Diagnostics = append(t.Diagnostics, *err)
		return false
	}
	return true
}

func (t *TypeChecker) getType(id ast.NodeID, span base.Span) (Type, bool) {
	typ, ok := t.Env.Types[id]
	if !ok {
		t.diag(span, "no type set for AST node %d", id)
		return Type{}, false
	}
	return typ, true
}

func (t *TypeChecker) typeOfPlace(id ast.NodeID) (Type, bool) {
	node := t.AST.Node(id)
	switch kind := node.Kind.(type) {
	case ast.Ident:
		b, ok := t.lookupInScope(kind.Name, node.Span)
		if !ok {
			return Type{}, false
		}
		if !b.Mut {
			t.diag(node.Span, "cannot assign to immutable variable: %s", b.Name)
			return Type{}, false
		}
		return b.Type, true
	case ast.Deref:
		exprNode := t.AST.Node(kind.Expr)
		innerTyp, ok := t.getType(kind.Expr, exprNode.Span)
		if !ok {
			return Type{}, false
		}
		if innerTyp.Kind != TypeRef {
			t.diag(node.Span, "cannot assign through dereference: expected reference, got %s", innerTyp)
			return Type{}, false
		}
		if !innerTyp.Ref.Mut {
			t.diag(node.Span, "cannot assign through dereference: expected mutable reference, got %s", innerTyp)
			return Type{}, false
		}
		return innerTyp.Ref.Type, true
	default:
		t.diag(node.Span, "left-hand side is not assignable: %T", kind)
		return Type{}, false
	}
}

func (t *TypeChecker) diag(span base.Span, msg string, msgArgs ...any) {
	t.Diagnostics = append(t.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}

func (t *TypeChecker) verifyMain(fun ast.Fun) {
	if len(fun.Params) != 0 {
		firstParam := t.AST.Node(fun.Params[0])
		lastParam := t.AST.Node(fun.Params[len(fun.Params)-1])
		span := firstParam.Span.Combine(lastParam.Span)
		t.diag(span, "main function cannot take arguments")
	}
	retNode := t.AST.Node(fun.ReturnType)
	if retTyp, ok := retNode.Kind.(ast.SimpleType); ok && retTyp.Name.Name != "void" {
		t.diag(retNode.Span, "main function cannot return a value")
	}
}

func (t *TypeChecker) base(span base.Span) typBase {
	self := t.id_
	t.id_++
	return typBase{self, span}
}

func WalkType(typ *Type, visit func(*Type)) {
	switch typ.Kind {
	case TypeInt, TypeStr, TypeVoid:
		return
	case TypeFun:
		for i := range typ.Fun.Params {
			visit(&typ.Fun.Params[i])
		}
		visit(&typ.Fun.Return)
	case TypeRef:
		visit(&typ.Ref.Type)
	default:
		panic(base.Errorf("unknown type kind: %d", typ.Kind))
	}
}
