package internal

import (
	"strings"
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
	Span Span
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
		panic(Errorf("unknown type kind: %d", t.Kind))
	}
}

func (t Type) Span() Span {
	switch t.Kind {
	case TypeInt, TypeStr, TypeVoid:
		return t.BuiltIn.Span
	case TypeFun:
		return t.Fun.Span
	case TypeRef:
		return t.Ref.Span
	default:
		panic(Errorf("unknown type kind: %v", t))
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
		panic(Errorf("unknown type kind: %d", t.Kind))
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
	Types    map[ASTID]Type
	Bindings map[ASTID]Binding
	RefTypes map[RefTypeCacheKey]Type
}

func NewTypeEnv() TypeEnv {
	return TypeEnv{map[ASTID]Type{}, map[ASTID]Binding{}, map[RefTypeCacheKey]Type{}}
}

func (e *TypeEnv) LookupType(id ASTID, span Span) (Type, *Diagnostic) {
	if t, ok := e.Types[id]; ok {
		return t, nil
	}
	return Type{}, NewDiagnostic(span, "no type set for AST node %d", id)
}

func (e *TypeEnv) SetType(id ASTID, t Type, span Span) *Diagnostic {
	if _, ok := e.Types[id]; ok {
		return NewDiagnostic(span, "type already set for AST node %d", id)
	}
	e.Types[id] = t
	return nil
}

func (e *TypeEnv) LookupBinding(id ASTID, span Span) (Binding, *Diagnostic) {
	if b, ok := e.Bindings[id]; ok {
		return b, nil
	}
	return Binding{}, NewDiagnostic(span, "no binding set for AST node %d", id)
}

func (e *TypeEnv) SetBinding(id ASTID, b Binding, span Span) *Diagnostic {
	if _, ok := e.Bindings[id]; ok {
		return NewDiagnostic(span, "binding already set for AST node %d", id)
	}
	e.Bindings[id] = b
	return nil
}

func (e *TypeEnv) SetRefType(id TypeID, t Type, mut bool) {
	key := RefTypeCacheKey{id, mut}
	if _, ok := e.RefTypes[key]; ok {
		panic(Errorf("ref type already set for type %d and mut %t", id, mut))
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
	span := NewSpan(NewSource("builtin", []rune{}), 0, 0)
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

func (s *Scope) Lookup(name string, span Span) (Binding, *Diagnostic) {
	if t, ok := s.Bindings[name]; ok {
		return t, nil
	}
	if s.Parent != nil {
		return s.Parent.Lookup(name, span)
	}
	return Binding{}, NewDiagnostic(span, "symbol not defined: %s", name)
}

func (s *Scope) Bind(name string, t Type, mut bool, span Span) *Diagnostic {
	if _, ok := s.Bindings[name]; ok {
		return NewDiagnostic(span, "symbol already defined: %s", name)
	}
	s.Bindings[name] = Binding{name, t, mut}
	return nil
}

type TypeChecker struct {
	Diagnostics Diagnostics
	Env         TypeEnv
	Scope       *Scope
	id_         TypeID
	voidType    Type
	intType     Type
	strType     Type
}

func NewTypeChecker() *TypeChecker {
	span := NewSpan(NewSource("builtin", []rune{}), 0, 0)
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
	return &TypeChecker{Diagnostics{}, NewTypeEnv(), scope, TypeID(100), voidType.Type, intType.Type, strType.Type}
}

func (t *TypeChecker) VisitExpr(expr *Expr) {
	WalkExpr(expr, t)
}

func (t *TypeChecker) VisitRefExpr(expr *RefExpr) {
	WalkRefExpr(expr, t)
	typ, ok := t.getType(expr.Ident.ID, expr.Ident.Span)
	if !ok {
		return
	}
	b, ok := t.lookupInScope(expr.Ident.Name, expr.Span)
	if !ok {
		return
	}
	refTyp := t.buildRefType(typ, b.Mut, expr.Span)
	t.setType(expr.ID, refTyp, expr.Span)
}

func (t *TypeChecker) VisitDerefExpr(expr *DerefExpr) {
	WalkDerefExpr(expr, t)
	typ, ok := t.getType(expr.Expr.ID(), expr.Expr.Span())
	if !ok {
		return
	}
	if typ.Kind != TypeRef {
		t.diag(expr.Expr.Span(), "dereference: expected reference, got %s", typ)
		return
	}
	t.setType(expr.ID, typ.Ref.Type, expr.Span)
}

func (t *TypeChecker) VisitVar(var_ *Var) {
	WalkExpr(&var_.Init, t)
	initTyp, ok := t.getType(var_.Init.ID(), var_.Name.Span)
	if !ok {
		return
	}
	if var_.Mut && initTyp.Kind == TypeRef && !initTyp.Ref.Mut {
		t.diag(var_.Init.Span(), "cannot take a mutable reference to an immutable value")
		return
	}
	t.bindInScope(var_.ID, var_.Name.Name, initTyp, var_.Mut, var_.Name.Span)
	t.setType(var_.ID, t.voidType, var_.Span)
}

func (t *TypeChecker) VisitAssign(assign *Assign) {
	WalkAssign(assign, t)
	valueTyp, ok := t.getType(assign.Value.ID(), assign.Value.Span())
	if !ok {
		return
	}
	placeType, ok := t.typeOfPlace(assign.LHS)
	if !ok {
		return
	}
	if !valueTyp.IsAssignableTo(placeType) {
		t.diag(assign.Value.Span(), "type mismatch: expected %s, got %s", placeType, valueTyp)
		return
	}
	t.setType(assign.ID, t.voidType, assign.Span)
}

func (t *TypeChecker) VisitBlock(block *Block) {
	defer t.enterScope()()
	WalkBlock(block, t)
	if len(block.Exprs) == 0 {
		t.setType(block.ID, t.voidType, block.Span)
		return
	}
	last := block.Exprs[len(block.Exprs)-1]
	typ, ok := t.getType(last.ID(), last.Span())
	if !ok {
		return
	}
	t.setType(block.ID, typ, block.Span)
}

func (t *TypeChecker) VisitCall(call *Call) {
	WalkCall(call, t)
	typ, ok := t.getType(call.Callee.ID(), call.Callee.Span())
	if !ok {
		return
	}
	if typ.Kind != TypeFun {
		t.diag(call.Callee.Span(), "callee is not a function")
		return
	}
	fun := typ.Fun
	if len(call.Args) != len(fun.Params) {
		t.diag(call.Span, "argument count mismatch: expected %d, got %d", len(fun.Params), len(call.Args))
		return
	}
	for i, arg := range call.Args {
		argType, ok := t.getType(arg.ID(), arg.Span())
		if !ok {
			return
		}
		if !argType.IsAssignableTo(fun.Params[i]) {
			t.diag(arg.Span(), "type mismatch at argument %d: expected %s, got %s", i+1, fun.Params[i], argType)
			return
		}
	}
	t.setType(call.ID, fun.Return, call.Span)
}

func (t *TypeChecker) VisitFunParam(funParam *FunParam) {
	WalkFunParam(funParam, t)
	typ, ok := t.getType(funParam.Type.ID(), funParam.Name.Span)
	if !ok {
		return
	}
	if funParam.Mut && typ.Kind != TypeRef {
		t.diag(funParam.Span, "only reference types can be mutable parameters")
		return
	}
	if funParam.Mut && typ.Kind == TypeRef {
		typ = t.buildRefType(typ.Ref.Type, true, funParam.Span)
	}
	if !t.setType(funParam.ID, typ, funParam.Span) {
		return
	}
	t.bindInScope(funParam.ID, funParam.Name.Name, typ, funParam.Mut, funParam.Name.Span)
}

func (t *TypeChecker) VisitFun(fun *Fun) {
	// We need to bind the function before we enter the scope, because
	// the function may refer to itself and we would bind it in the wrong scope.
	t.VisitType(&fun.ReturnType)
	ret, ok := t.getType(fun.ReturnType.ID(), fun.ReturnType.Span())
	if !ok {
		return
	}
	params := make([]Type, len(fun.Params))
	funTyp := NewFunType(&FunType{t.base(fun.Span), params, ret})
	if !t.bindInScope(fun.ID, fun.Name.Name, funTyp, false, fun.Name.Span) {
		return
	}
	defer t.enterScope()()
	for i, param := range fun.Params {
		t.VisitFunParam(&param)
		typ, ok := t.getType(param.Type.ID(), param.Type.Span())
		if !ok {
			return
		}
		params[i] = typ
	}
	t.VisitBlock(&fun.Block)
	if ret.Kind != TypeVoid {
		blockTyp, ok := t.getType(fun.Block.ID, fun.Block.Span)
		if !ok {
			return
		}
		if !blockTyp.IsAssignableTo(ret) {
			span := blockTyp.Span()
			if len(fun.Block.Exprs) > 0 {
				span = fun.Block.Exprs[len(fun.Block.Exprs)-1].Span()
			}
			t.diag(span, "return type mismatch: expected %s, got %s", ret, blockTyp)
		}
	}
	t.setType(fun.ID, funTyp, fun.Span)
	if fun.Name.Name == "main" {
		t.verifyMain(fun)
	}
}

func (t *TypeChecker) VisitIdent(ident *Ident) {
	WalkIdent(ident, t)
	b, ok := t.lookupInScope(ident.Name, ident.Span)
	if !ok {
		return
	}
	t.setType(ident.ID, b.Type, ident.Span)
}

func (t *TypeChecker) VisitName(name *Name) {
}

func (t *TypeChecker) VisitIntExpr(expr *IntExpr) {
	WalkIntExpr(expr, t)
	t.setType(expr.ID, t.intType, expr.Span)
}

func (t *TypeChecker) VisitStringExpr(expr *StringExpr) {
	WalkStringExpr(expr, t)
	t.setType(expr.ID, t.strType, expr.Span)
}

func (t *TypeChecker) VisitFile(file *File) {
	WalkFile(file, t)
	t.setType(file.ID, t.voidType, file.Span)
}

func (t *TypeChecker) VisitDecl(decl *Decl) {
	WalkDecl(decl, t)
}

func (t *TypeChecker) VisitType(typ *ASTType) {
	WalkASTType(typ, t)
	var b Binding
	var ok bool
	switch typ.Kind {
	case TySimpleType:
		b, ok = t.lookupInScope(typ.SimpleType.Name.Name, typ.Span())
		if !ok {
			return
		}
		t.setType(typ.ID(), b.Type, typ.Span())
	case TyRefType:
		inner, err := t.Env.LookupType(typ.RefType.Type.ID(), typ.Span())
		if err != nil {
			t.Diagnostics = append(t.Diagnostics, *err)
			return
		}
		refType := t.buildRefType(inner, false, typ.Span())
		t.setType(typ.ID(), refType, typ.Span())
	default:
		panic(Errorf("unknown type kind: %d", typ.Kind))
	}
}

func (t *TypeChecker) buildRefType(typ Type, mut bool, span Span) Type {
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

func (t *TypeChecker) lookupInScope(name string, span Span) (Binding, bool) {
	b, err := t.Scope.Lookup(name, span)
	if err != nil {
		t.Diagnostics = append(t.Diagnostics, *err)
		return Binding{}, false
	}
	return b, true
}

func (t *TypeChecker) bindInScope(id ASTID, name string, typ Type, mut bool, span Span) bool {
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

func (t *TypeChecker) setType(id ASTID, typ Type, span Span) bool {
	if err := t.Env.SetType(id, typ, span); err != nil {
		t.Diagnostics = append(t.Diagnostics, *err)
		return false
	}
	return true
}

func (t *TypeChecker) getType(id ASTID, span Span) (Type, bool) {
	typ, ok := t.Env.Types[id]
	if !ok {
		t.diag(span, "no type set for AST node %d", id)
		return Type{}, false
	}
	return typ, true
}

func (t *TypeChecker) typeOfPlace(e Expr) (Type, bool) {
	switch e.Kind { //nolint:exhaustive
	case ExprIdent:
		b, ok := t.lookupInScope(e.Ident.Name, e.Span())
		if !ok {
			return Type{}, false
		}
		if !b.Mut {
			t.diag(e.Span(), "cannot assign to immutable variable: %s", b.Name)
			return Type{}, false
		}
		return b.Type, true
	case ExprDeref:
		innerTyp, ok := t.getType(e.Deref.Expr.ID(), e.Deref.Expr.Span())
		if !ok {
			return Type{}, false
		}
		if innerTyp.Kind != TypeRef {
			t.diag(e.Span(), "cannot assign through dereference: expected reference, got %s", innerTyp)
			return Type{}, false
		}
		if !innerTyp.Ref.Mut {
			t.diag(e.Span(), "cannot assign through dereference: expected mutable reference, got %s", innerTyp)
			return Type{}, false
		}
		return innerTyp.Ref.Type, true
	default:
		t.diag(e.Span(), "left-hand side is not assignable: %s", e.Kind)
		return Type{}, false
	}
}

func (t *TypeChecker) diag(span Span, msg string, msgArgs ...any) {
	t.Diagnostics = append(t.Diagnostics, *NewDiagnostic(span, msg, msgArgs...))
}

func (t *TypeChecker) verifyMain(fun *Fun) {
	if len(fun.Params) != 0 {
		span := fun.Params[0].Span.Combine(fun.Params[len(fun.Params)-1].Span)
		t.diag(span, "main function cannot take arguments")
	}
	// todo: this check should not be so cumbersome.
	if fun.ReturnType.Kind == TySimpleType && fun.ReturnType.SimpleType.Name.Name != "void" {
		t.diag(fun.ReturnType.Span(), "main function cannot return a value")
	}
}

func (t *TypeChecker) base(span Span) typBase {
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
		panic(Errorf("unknown type kind: %d", typ.Kind))
	}
}
