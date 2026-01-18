package ast

import "github.com/flunderpero/metall/metallc/internal/base"

type NodeID int

type astBase struct {
	ID   NodeID
	Span base.Span
}

type Ident struct {
	astBase
	Name string
}

type TypeIdent struct {
	astBase
	Name string
}

type Name struct {
	astBase
	Name string
}

type File struct {
	astBase
	Decls []Decl
}

type DeclKind int

const (
	DeclFun DeclKind = iota + 1
)

type Decl struct {
	Kind DeclKind
	Fun  *Fun
}

type TypeKind int

const (
	TypeSimple TypeKind = iota + 1
	TypeRef
)

type SimpleType struct {
	Name
}

type RefType struct {
	astBase
	Type Type
}

type Type struct {
	Kind TypeKind

	SimpleType *SimpleType
	RefType    *RefType
}

func NewSimpleType(typ *SimpleType) Type {
	return Type{Kind: TypeSimple, SimpleType: typ} //nolint:exhaustruct
}

func NewRefType(typ *RefType) Type {
	return Type{Kind: TypeRef, RefType: typ} //nolint:exhaustruct
}

func (t *Type) ID() NodeID {
	switch t.Kind {
	case TypeSimple:
		return t.SimpleType.ID
	case TypeRef:
		return t.RefType.ID
	default:
		panic(base.Errorf("unknown type kind: %d", t.Kind))
	}
}

func (t *Type) Span() base.Span {
	switch t.Kind {
	case TypeSimple:
		return t.SimpleType.Span
	case TypeRef:
		return t.RefType.Span
	default:
		panic(base.Errorf("unknown type kind: %d", t.Kind))
	}
}

type If struct {
	astBase
	Cond Expr
	Then Block
	Else *Block
}

type FunParam struct {
	astBase
	Name Name
	Type Type
	Mut  bool
}

type Fun struct {
	astBase
	Name       Name
	Params     []FunParam
	ReturnType Type
	Block      Block
}

type ExprKind int

const (
	ExprAssign ExprKind = iota + 1
	ExprBlock
	ExprCall
	ExprDeref
	ExprFun
	ExprIdent
	ExprInt
	ExprRef
	ExprString
	ExprVar
)

var exprKindNames = map[ExprKind]string{ //nolint:gochecknoglobals
	ExprAssign: "assign",
	ExprBlock:  "block",
	ExprCall:   "call",
	ExprDeref:  "deref",
	ExprFun:    "fun",
	ExprIdent:  "ident",
	ExprInt:    "int",
	ExprRef:    "ref",
	ExprString: "string",
	ExprVar:    "var",
}

func (k ExprKind) String() string {
	s, ok := exprKindNames[k]
	if !ok {
		panic(base.Errorf("unknown expr kind: %d", k))
	}
	return s
}

type Int struct {
	astBase
	Value int64
}

type String struct {
	astBase
	Value string
}

type Assign struct {
	astBase
	LHS   Expr
	Value Expr
}

type Var struct {
	astBase
	Name Name
	Init Expr
	Mut  bool
}

type Block struct {
	astBase
	Exprs []Expr
}

type Call struct {
	astBase
	Callee Expr
	Args   []Expr
}

type Ref struct {
	astBase
	Ident Ident
}

type Deref struct {
	astBase
	Expr Expr
}

type Expr struct {
	Kind ExprKind

	Assign *Assign
	Block  *Block
	Call   *Call
	Deref  *Deref
	Fun    *Fun
	Ident  *Ident
	Int    *Int
	Ref    *Ref
	String *String
	Var    *Var
}

func NewAssign(assign *Assign) Expr {
	return Expr{Kind: ExprAssign, Assign: assign} //nolint:exhaustruct
}

func NewBlock(block *Block) Expr {
	return Expr{Kind: ExprBlock, Block: block} //nolint:exhaustruct
}

func NewFun(fun *Fun) Expr {
	return Expr{Kind: ExprFun, Fun: fun} //nolint:exhaustruct
}

func NewIdent(ident *Ident) Expr {
	return Expr{Kind: ExprIdent, Ident: ident} //nolint:exhaustruct
}

func NewInt(intExpr *Int) Expr {
	return Expr{Kind: ExprInt, Int: intExpr} //nolint:exhaustruct
}

func NewString(stringExpr *String) Expr {
	return Expr{Kind: ExprString, String: stringExpr} //nolint:exhaustruct
}

func NewVar(varExpr *Var) Expr {
	return Expr{Kind: ExprVar, Var: varExpr} //nolint:exhaustruct
}

func NewCall(call *Call) Expr {
	return Expr{Kind: ExprCall, Call: call} //nolint:exhaustruct
}

func NewRef(ref *Ref) Expr {
	return Expr{Kind: ExprRef, Ref: ref} //nolint:exhaustruct
}

func NewDeref(deref *Deref) Expr {
	return Expr{Kind: ExprDeref, Deref: deref} //nolint:exhaustruct
}

func (e *Expr) ID() NodeID {
	switch e.Kind {
	case ExprAssign:
		return e.Assign.ID
	case ExprBlock:
		return e.Block.ID
	case ExprCall:
		return e.Call.ID
	case ExprDeref:
		return e.Deref.ID
	case ExprFun:
		return e.Fun.ID
	case ExprIdent:
		return e.Ident.ID
	case ExprInt:
		return e.Int.ID
	case ExprRef:
		return e.Ref.ID
	case ExprString:
		return e.String.ID
	case ExprVar:
		return e.Var.ID
	default:
		panic(base.Errorf("unknown expr kind: %d", e.Kind))
	}
}

func (e *Expr) Span() base.Span {
	switch e.Kind {
	case ExprAssign:
		return e.Assign.Span
	case ExprBlock:
		return e.Block.Span
	case ExprCall:
		return e.Call.Span
	case ExprDeref:
		return e.Deref.Span
	case ExprFun:
		return e.Fun.Span
	case ExprIdent:
		return e.Ident.Span
	case ExprInt:
		return e.Int.Span
	case ExprRef:
		return e.Ref.Span
	case ExprString:
		return e.String.Span
	case ExprVar:
		return e.Var.Span
	default:
		panic(base.Errorf("unknown expr kind: %d", e.Kind))
	}
}

type Visitor interface {
	VisitAssign(*Assign)
	VisitBlock(*Block)
	VisitCall(*Call)
	VisitDecl(*Decl)
	VisitDeref(*Deref)
	VisitExpr(*Expr)
	VisitFile(*File)
	VisitFun(*Fun)
	VisitFunParam(*FunParam)
	VisitIdent(*Ident)
	VisitInt(*Int)
	VisitName(*Name)
	VisitRef(*Ref)
	VisitString(*String)
	VisitType(*Type)
	VisitVar(*Var)
}

func WalkFile(file *File, v Visitor) {
	for i := range len(file.Decls) {
		v.VisitDecl(&file.Decls[i])
	}
}

func WalkDecl(decl *Decl, v Visitor) {
	switch decl.Kind {
	case DeclFun:
		v.VisitFun(decl.Fun)
	default:
		panic(base.Errorf("unknown decl kind: %d", decl.Kind))
	}
}

func WalkType(typ *Type, v Visitor) {
	switch typ.Kind {
	case TypeSimple:
		v.VisitName(&typ.SimpleType.Name)
	case TypeRef:
		v.VisitType(&typ.RefType.Type)
	default:
		panic(base.Errorf("unknown type kind: %d", typ.Kind))
	}
}

func WalkFun(fun *Fun, v Visitor) {
	v.VisitName(&fun.Name)
	for i := range len(fun.Params) {
		v.VisitFunParam(&fun.Params[i])
	}
	v.VisitType(&fun.ReturnType)
	v.VisitBlock(&fun.Block)
}

func WalkFunParam(funParam *FunParam, v Visitor) {
	v.VisitName(&funParam.Name)
	v.VisitType(&funParam.Type)
}

func WalkExpr(expr *Expr, v Visitor) {
	switch expr.Kind {
	case ExprAssign:
		v.VisitAssign(expr.Assign)
	case ExprBlock:
		v.VisitBlock(expr.Block)
	case ExprCall:
		v.VisitCall(expr.Call)
	case ExprDeref:
		v.VisitDeref(expr.Deref)
	case ExprFun:
		v.VisitFun(expr.Fun)
	case ExprIdent:
		v.VisitIdent(expr.Ident)
	case ExprInt:
		v.VisitInt(expr.Int)
	case ExprRef:
		v.VisitRef(expr.Ref)
	case ExprString:
		v.VisitString(expr.String)
	case ExprVar:
		v.VisitVar(expr.Var)
	default:
		panic(base.Errorf("unknown expr kind: %d", expr.Kind))
	}
}

func WalkRef(expr *Ref, v Visitor) {
	v.VisitIdent(&expr.Ident)
}

func WalkDeref(expr *Deref, v Visitor) {
	v.VisitExpr(&expr.Expr)
}

func WalkAssign(assign *Assign, v Visitor) {
	v.VisitExpr(&assign.LHS)
	v.VisitExpr(&assign.Value)
}

func WalkBlock(block *Block, v Visitor) {
	for i := range len(block.Exprs) {
		v.VisitExpr(&block.Exprs[i])
	}
}

func WalkCall(call *Call, v Visitor) {
	v.VisitExpr(&call.Callee)
	for i := range len(call.Args) {
		v.VisitExpr(&call.Args[i])
	}
}

func WalkIdent(ident *Ident, v Visitor) {
}

func WalkName(name *Name, v Visitor) {
}

func WalkInt(expr *Int, v Visitor) {
}

func WalkString(expr *String, v Visitor) {
}

func WalkVar(varExpr *Var, v Visitor) {
	v.VisitName(&varExpr.Name)
	WalkExpr(&varExpr.Init, v)
}
