//nolint:exhaustruct
package parse

import (
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/lex"
)

func TestParsOK(t *testing.T) {
	Str := typ("Str")
	Int := typ("Int")
	void := typ("void")

	tests := []struct {
		name string
		kind string
		src  string
		want any
	}{
		{
			"happy path", "file", `fun foo() Str { "hello" 123 } `,
			file(
				fun_decl("foo", fun_params(), Str, fun_block(string_("hello"), int_(123))),
			),
		},
		{"assign expr", "expr", `foo = 123`, assign(ident_expr("foo"), int_(123))},
		{"var expr", "expr", `let foo = 123`, var_("foo", int_(123))},
		{"mut expr", "expr", `mut foo = 123`, mut_var("foo", int_(123))},
		{"block expr", "expr", `{ 0 "hello" }`, block(int_(0), string_("hello"))},
		{"empty block", "expr", `{ }`, block()},
		{
			"fun with (mut) params", "expr", `fun foo(a Int, mut b Str) Int { 123 }`,
			fun(
				"foo",
				fun_params(fun_param("a", Int), mut_fun_param("b", Str)),
				Int,
				fun_block(int_(123)),
			),
		},
		{
			"fun inside block", "expr", `{ fun foo() Str { "hello" 123 } }`,
			block(
				fun("foo", fun_params(), Str, fun_block(string_("hello"), int_(123)))),
		},
		{"void fun", "expr", `fun foo() void {}`, fun("foo", fun_params(), void, fun_block())},
		{"call", "expr", `foo(123, "hello")`, call(ident_expr("foo"), int_(123), string_("hello"))},
		{"call w/o args", "expr", `foo()`, call(ident_expr("foo"))},
		{"chained calls", "expr", `foo()()`, call(call(ident_expr("foo")))},

		{"ref ident expr", "expr", `&foo`, ref(ident("foo"))},
		{"deref expr", "expr", `*foo`, deref(ident_expr("foo"))},
		{"nested deref expr", "expr", `**foo`, deref(deref(ident_expr("foo")))},
		{"ref type", "expr", `fun foo() &Int {}`, fun("foo", fun_params(), ref_typ(Int), fun_block())},
		{"nested ref type", "expr", `fun foo() &&Int {}`, fun("foo", fun_params(), ref_typ(ref_typ(Int)), fun_block())},
		{"deref assign", "expr", `*foo = bar`, assign(deref(ident_expr("foo")), ident_expr("bar"))},
		{
			"nested deref assign",
			"expr",
			`***foo = bar`,
			assign(deref(deref(deref(ident_expr("foo")))), ident_expr("bar")),
		},
		{
			"ref param", "expr", `{ fun foo(a &Int) void {} let b = 123 foo(&b) }`,
			block(
				fun("foo", fun_params(fun_param("a", ref_typ(Int))), void, fun_block()),
				var_("b", int_(123)),
				call(ident_expr("foo"), ref(ident("b"))),
			),
		},
	}

	assert := base.NewAssert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := lex.Lex(source)
			parser := NewParser(tokens)
			var got any
			var ok bool
			switch tt.kind {
			case "expr":
				got, ok = parser.ParseExpr()
			case "file":
				got, ok = parser.ParseFile()
			default:
				t.Fatalf("unknown kind: %s", tt.kind)
			}
			assert.Equal(0, len(parser.Diagnostics), "diagnostics: %s", parser.Diagnostics)
			assert.Equal(true, ok, "parse function returned false")
			zero := &zeroBaseVisitor{}
			switch g := got.(type) {
			case Expr:
				zero.VisitExpr(&g)
				got = g
			case File:
				zero.VisitFile(&g)
				got = g
			default:
				t.Fatalf("unknown type: %T", got)
			}
			assert.Equal(tt.want, got)
		})
	}
}

func TestParsErr(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"unexpected token", `=`, []string{
			"test.met:1:1: unexpected token: expected one of '&', '{', <fun>, <identifier>, <number>, <string>, <let>, <mut>, got =\n" +
				`    =` + "\n" +
				"    ^",
		}},
		{"assign to type", `{ Str = "hello" }`, []string{
			"test.met:1:3: unexpected token: expected one of '&', '{', <fun>, <identifier>, <number>, <string>, <let>, <mut>, got <type identifier>\n" +
				`    { Str = "hello" }` + "\n" +
				"      ^^^",
		}},
		{"nested ref expr", `{ &&foo }`, []string{
			"test.met:1:4: unexpected token: expected <identifier>, got &\n" +
				`    { &&foo }` + "\n" +
				"       ^",
		}},
		{"ref to literal", `{ &123 }`, []string{
			"test.met:1:4: unexpected token: expected <identifier>, got <number>\n" +
				`    { &123 }` + "\n" +
				"       ^^^",
		}},
	}

	assert := base.NewAssert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := lex.Lex(source)
			parser := NewParser(tokens)
			expr, parseOK := parser.ParseExpr()
			if parseOK {
				zero := &zeroBaseVisitor{}
				zero.VisitExpr(&expr)
			}
			assert.Equal(false, parseOK, "ParseExpr should have failed: %s", base.Stringify(expr))
			diagnostics := parser.Diagnostics
			for i, want := range tt.want {
				if i >= len(diagnostics) {
					t.Fatalf("no more diagnostics, but wanted: %s", want)
				}
				assert.Equal(want, diagnostics[i].Display())
			}
			if len(diagnostics) > len(tt.want) {
				t.Fatalf("there are more diagnostics than expected: %s", diagnostics[len(tt.want):])
			}
		})
	}
}

type zeroBaseVisitor struct{}

func (v *zeroBaseVisitor) VisitFile(file *File) {
	file.astBase = astBase{}
	WalkFile(file, v)
}

func (v *zeroBaseVisitor) VisitDecl(decl *Decl) {
	WalkDecl(decl, v)
}

func (v *zeroBaseVisitor) VisitType(typ *ASTType) {
	switch typ.Kind {
	case TySimpleType:
		typ.SimpleType.astBase = astBase{}
	case TyRefType:
		typ.RefType.astBase = astBase{}
	default:
		panic(base.Errorf("unknown type kind: %d", typ.Kind))
	}
	WalkASTType(typ, v)
}

func (v *zeroBaseVisitor) VisitFun(fun *Fun) {
	fun.astBase = astBase{}
	WalkFun(fun, v)
}

func (v *zeroBaseVisitor) VisitFunParam(funParam *FunParam) {
	funParam.astBase = astBase{}
	WalkFunParam(funParam, v)
}

func (v *zeroBaseVisitor) VisitIdent(ident *Ident) {
	ident.astBase = astBase{}
	WalkIdent(ident, v)
}

func (v *zeroBaseVisitor) VisitName(name *Name) {
	name.astBase = astBase{}
	WalkName(name, v)
}

func (v *zeroBaseVisitor) VisitExpr(expr *Expr) {
	WalkExpr(expr, v)
}

func (v *zeroBaseVisitor) VisitRefExpr(expr *RefExpr) {
	expr.astBase = astBase{}
	WalkRefExpr(expr, v)
}

func (v *zeroBaseVisitor) VisitDerefExpr(expr *DerefExpr) {
	expr.astBase = astBase{}
	WalkDerefExpr(expr, v)
}

func (v *zeroBaseVisitor) VisitAssign(assign *Assign) {
	assign.astBase = astBase{}
	WalkAssign(assign, v)
}

func (v *zeroBaseVisitor) VisitCall(call *Call) {
	call.astBase = astBase{}
	WalkCall(call, v)
}

func (v *zeroBaseVisitor) VisitBlock(block *Block) {
	block.astBase = astBase{}
	WalkBlock(block, v)
}

func (v *zeroBaseVisitor) VisitIntExpr(expr *IntExpr) {
	expr.astBase = astBase{}
	WalkIntExpr(expr, v)
}

func (v *zeroBaseVisitor) VisitStringExpr(expr *StringExpr) {
	expr.astBase = astBase{}
	WalkStringExpr(expr, v)
}

func (v *zeroBaseVisitor) VisitVar(varExpr *Var) {
	varExpr.astBase = astBase{}
	WalkVar(varExpr, v)
}

func ident(name string) Ident {
	return Ident{Name: name}
}

func ident_expr(name string) Expr {
	return Expr{Kind: ExprIdent, Ident: &Ident{Name: name}}
}

func call(callee Expr, args ...Expr) Expr {
	if args == nil {
		args = []Expr{}
	}
	return Expr{Kind: ExprCall, Call: &Call{Callee: callee, Args: args}}
}

func file(decls ...Decl) File {
	if len(decls) == 0 {
		decls = []Decl{}
	}
	return File{Decls: decls}
}

func mut_fun_param(name string, typ ASTType) FunParam {
	return FunParam{Name: Name{Name: name}, Type: typ, Mut: true}
}

func fun_param(name string, typ ASTType) FunParam {
	return FunParam{Name: Name{Name: name}, Type: typ}
}

func fun_params(params ...FunParam) []FunParam {
	if len(params) == 0 {
		return []FunParam{}
	}
	return params
}

func fun_decl(name_ string, params []FunParam, return_type ASTType, block Block) Decl {
	name := Name{Name: name_}
	return Decl{Kind: DeclFun, Fun: &Fun{Name: name, Params: params, ReturnType: return_type, Block: block}}
}

func fun(name_ string, params []FunParam, return_type ASTType, block Block) Expr { //nolint:unparam
	name := Name{Name: name_}
	return Expr{Kind: ExprFun, Fun: &Fun{Name: name, Params: params, ReturnType: return_type, Block: block}}
}

func string_(value string) Expr { //nolint:unparam
	return Expr{Kind: ExprString, String: &StringExpr{Value: value}}
}

func int_(value int64) Expr {
	return Expr{Kind: ExprInt, Int: &IntExpr{Value: value}}
}

func assign(lhs Expr, value Expr) Expr {
	return Expr{Kind: ExprAssign, Assign: &Assign{LHS: lhs, Value: value}}
}

func typ(name string) ASTType {
	return NewASTSimpleType(&ASTSimpleType{Name{Name: name}})
}

func fun_block(exprs ...Expr) Block {
	if exprs == nil {
		exprs = []Expr{}
	}
	return Block{Exprs: exprs}
}

func var_(name_ string, init Expr) Expr {
	name := Name{Name: name_}
	return Expr{Kind: ExprVar, Var: &Var{Name: name, Init: init}}
}

func mut_var(name_ string, init Expr) Expr {
	name := Name{Name: name_}
	return Expr{Kind: ExprVar, Var: &Var{Name: name, Init: init, Mut: true}}
}

func ref(ident Ident) Expr {
	return Expr{Kind: ExprRef, Ref: &RefExpr{Ident: ident}}
}

func deref(expr Expr) Expr {
	return Expr{Kind: ExprDeref, Deref: &DerefExpr{Expr: expr}}
}

func ref_typ(typ ASTType) ASTType {
	return NewASTRefType(&ASTTypeRef{Type: typ})
}

func block(exprs ...Expr) Expr {
	if exprs == nil {
		exprs = []Expr{}
	}
	return Expr{Kind: ExprBlock, Block: &Block{Exprs: exprs}}
}
