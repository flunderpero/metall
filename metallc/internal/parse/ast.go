package parse

import "github.com/flunderpero/metall/metallc/internal/base"

type ASTID int

type astBase struct {
	ID   ASTID
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

type ASTTypeKind int

const (
	TySimpleType ASTTypeKind = iota + 1
	TyRefType
)

type ASTSimpleType struct {
	Name
}

type ASTTypeRef struct {
	astBase
	Type ASTType
}

type ASTType struct {
	Kind ASTTypeKind

	SimpleType *ASTSimpleType
	RefType    *ASTTypeRef
}

func NewASTSimpleType(typ *ASTSimpleType) ASTType {
	return ASTType{Kind: TySimpleType, SimpleType: typ} //nolint:exhaustruct
}

func NewASTRefType(typ *ASTTypeRef) ASTType {
	return ASTType{Kind: TyRefType, RefType: typ} //nolint:exhaustruct
}

func (t *ASTType) ID() ASTID {
	switch t.Kind {
	case TySimpleType:
		return t.SimpleType.ID
	case TyRefType:
		return t.RefType.ID
	default:
		panic(base.Errorf("unknown type kind: %d", t.Kind))
	}
}

func (t *ASTType) Span() base.Span {
	switch t.Kind {
	case TySimpleType:
		return t.SimpleType.Span
	case TyRefType:
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
	Type ASTType
	Mut  bool
}

type Fun struct {
	astBase
	Name       Name
	Params     []FunParam
	ReturnType ASTType
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

type IntExpr struct {
	astBase
	Value int64
}

type StringExpr struct {
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

type RefExpr struct {
	astBase
	Ident Ident
}

type DerefExpr struct {
	astBase
	Expr Expr
}

type Expr struct {
	Kind ExprKind

	Assign *Assign
	Block  *Block
	Call   *Call
	Deref  *DerefExpr
	Fun    *Fun
	Ident  *Ident
	Int    *IntExpr
	Ref    *RefExpr
	String *StringExpr
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

func NewInt(intExpr *IntExpr) Expr {
	return Expr{Kind: ExprInt, Int: intExpr} //nolint:exhaustruct
}

func NewString(stringExpr *StringExpr) Expr {
	return Expr{Kind: ExprString, String: stringExpr} //nolint:exhaustruct
}

func NewVar(varExpr *Var) Expr {
	return Expr{Kind: ExprVar, Var: varExpr} //nolint:exhaustruct
}

func NewCall(call *Call) Expr {
	return Expr{Kind: ExprCall, Call: call} //nolint:exhaustruct
}

func NewRef(ref *RefExpr) Expr {
	return Expr{Kind: ExprRef, Ref: ref} //nolint:exhaustruct
}

func NewDeref(deref *DerefExpr) Expr {
	return Expr{Kind: ExprDeref, Deref: deref} //nolint:exhaustruct
}

func (e *Expr) ID() ASTID {
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

type ASTVisitor interface {
	VisitAssign(*Assign)
	VisitBlock(*Block)
	VisitCall(*Call)
	VisitDecl(*Decl)
	VisitDerefExpr(*DerefExpr)
	VisitExpr(*Expr)
	VisitFile(*File)
	VisitFun(*Fun)
	VisitFunParam(*FunParam)
	VisitIdent(*Ident)
	VisitIntExpr(*IntExpr)
	VisitName(*Name)
	VisitRefExpr(*RefExpr)
	VisitStringExpr(*StringExpr)
	VisitType(*ASTType)
	VisitVar(*Var)
}

func WalkFile(file *File, v ASTVisitor) {
	for i := range len(file.Decls) {
		v.VisitDecl(&file.Decls[i])
	}
}

func WalkDecl(decl *Decl, v ASTVisitor) {
	switch decl.Kind {
	case DeclFun:
		v.VisitFun(decl.Fun)
	default:
		panic(base.Errorf("unknown decl kind: %d", decl.Kind))
	}
}

func WalkASTType(typ *ASTType, v ASTVisitor) {
	switch typ.Kind {
	case TySimpleType:
		v.VisitName(&typ.SimpleType.Name)
	case TyRefType:
		v.VisitType(&typ.RefType.Type)
	default:
		panic(base.Errorf("unknown type kind: %d", typ.Kind))
	}
}

func WalkFun(fun *Fun, v ASTVisitor) {
	v.VisitName(&fun.Name)
	for i := range len(fun.Params) {
		v.VisitFunParam(&fun.Params[i])
	}
	v.VisitType(&fun.ReturnType)
	v.VisitBlock(&fun.Block)
}

func WalkFunParam(funParam *FunParam, v ASTVisitor) {
	v.VisitName(&funParam.Name)
	v.VisitType(&funParam.Type)
}

func WalkExpr(expr *Expr, v ASTVisitor) {
	switch expr.Kind {
	case ExprAssign:
		v.VisitAssign(expr.Assign)
	case ExprBlock:
		v.VisitBlock(expr.Block)
	case ExprCall:
		v.VisitCall(expr.Call)
	case ExprDeref:
		v.VisitDerefExpr(expr.Deref)
	case ExprFun:
		v.VisitFun(expr.Fun)
	case ExprIdent:
		v.VisitIdent(expr.Ident)
	case ExprInt:
		v.VisitIntExpr(expr.Int)
	case ExprRef:
		v.VisitRefExpr(expr.Ref)
	case ExprString:
		v.VisitStringExpr(expr.String)
	case ExprVar:
		v.VisitVar(expr.Var)
	default:
		panic(base.Errorf("unknown expr kind: %d", expr.Kind))
	}
}

func WalkRefExpr(expr *RefExpr, v ASTVisitor) {
	v.VisitIdent(&expr.Ident)
}

func WalkDerefExpr(expr *DerefExpr, v ASTVisitor) {
	v.VisitExpr(&expr.Expr)
}

func WalkAssign(assign *Assign, v ASTVisitor) {
	v.VisitExpr(&assign.LHS)
	v.VisitExpr(&assign.Value)
}

func WalkBlock(block *Block, v ASTVisitor) {
	for i := range len(block.Exprs) {
		v.VisitExpr(&block.Exprs[i])
	}
}

func WalkCall(call *Call, v ASTVisitor) {
	v.VisitExpr(&call.Callee)
	for i := range len(call.Args) {
		v.VisitExpr(&call.Args[i])
	}
}

func WalkIdent(ident *Ident, v ASTVisitor) {
}

func WalkName(name *Name, v ASTVisitor) {
}

func WalkIntExpr(expr *IntExpr, v ASTVisitor) {
}

func WalkStringExpr(expr *StringExpr, v ASTVisitor) {
}

func WalkVar(varExpr *Var, v ASTVisitor) {
	v.VisitName(&varExpr.Name)
	WalkExpr(&varExpr.Init, v)
}
