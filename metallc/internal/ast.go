package internal

type ASTID int

type astBase struct {
	ID   ASTID
	Span Span
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

type ASTType struct {
	TypeIdent
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
	ExprFun
	ExprIdent
	ExprInt
	ExprString
	ExprVar
)

var exprKindNames = map[ExprKind]string{ //nolint:gochecknoglobals
	ExprAssign: "assign",
	ExprBlock:  "block",
	ExprCall:   "call",
	ExprFun:    "fun",
	ExprIdent:  "ident",
	ExprInt:    "int",
	ExprString: "string",
	ExprVar:    "var",
}

func (k ExprKind) String() string {
	s, ok := exprKindNames[k]
	if !ok {
		panic(Errorf("unknown expr kind: %d", k))
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
	Ident Ident
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

type Expr struct {
	Kind ExprKind

	Assign *Assign
	Block  *Block
	Call   *Call
	Fun    *Fun
	Ident  *Ident
	Int    *IntExpr
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

func (e *Expr) ID() ASTID {
	switch e.Kind {
	case ExprAssign:
		return e.Assign.ID
	case ExprBlock:
		return e.Block.ID
	case ExprCall:
		return e.Call.ID
	case ExprFun:
		return e.Fun.ID
	case ExprIdent:
		return e.Ident.ID
	case ExprInt:
		return e.Int.ID
	case ExprString:
		return e.String.ID
	case ExprVar:
		return e.Var.ID
	default:
		panic(Errorf("unknown expr kind: %d", e.Kind))
	}
}

func (e *Expr) Span() Span {
	switch e.Kind {
	case ExprAssign:
		return e.Assign.Span
	case ExprBlock:
		return e.Block.Span
	case ExprCall:
		return e.Call.Span
	case ExprFun:
		return e.Fun.Span
	case ExprIdent:
		return e.Ident.Span
	case ExprInt:
		return e.Int.Span
	case ExprString:
		return e.String.Span
	case ExprVar:
		return e.Var.Span
	default:
		panic(Errorf("unknown expr kind: %d", e.Kind))
	}
}

type ASTVisitor interface {
	VisitFile(*File)
	VisitDecl(*Decl)
	VisitType(*ASTType)
	VisitFunParam(*FunParam)
	VisitExpr(*Expr)
	VisitAssign(*Assign)
	VisitBlock(*Block)
	VisitCall(*Call)
	VisitFun(*Fun)
	VisitIdent(*Ident)
	VisitName(*Name)
	VisitIntExpr(*IntExpr)
	VisitStringExpr(*StringExpr)
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
		panic(Errorf("unknown decl kind: %d", decl.Kind))
	}
}

func WalkASTType(typ *ASTType, v ASTVisitor) {
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
	case ExprFun:
		v.VisitFun(expr.Fun)
	case ExprIdent:
		v.VisitIdent(expr.Ident)
	case ExprInt:
		v.VisitIntExpr(expr.Int)
	case ExprString:
		v.VisitStringExpr(expr.String)
	case ExprVar:
		v.VisitVar(expr.Var)
	default:
		panic(Errorf("unknown expr kind: %d", expr.Kind))
	}
}

func WalkAssign(assign *Assign, v ASTVisitor) {
	v.VisitIdent(&assign.Ident)
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
