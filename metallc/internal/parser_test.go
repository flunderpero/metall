//nolint:exhaustruct
package internal

import (
	"testing"
)

func TestParsOK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind string
		src  string
		want any
	}{
		{
			"happy path", "file", `fun foo() Str { "hello" 123 } `,
			file(
				fundecl("foo", fun_params(), typ("Str"), funblock(string_expr("hello"), int_expr(123))),
			),
		},
		{"assign expr", "expr", `foo = 123`, assign(ident("foo"), int_expr(123))},
		{"var expr", "expr", `let foo = 123`, var_("foo", int_expr(123))},
		{"mut expr", "expr", `mut foo = 123`, mut_var("foo", int_expr(123))},
		{"block expr", "expr", `{ 0 "hello" }`, block(int_expr(0), string_expr("hello"))},
		{"empty block", "expr", `{ }`, block()},
		{
			"fun with (mut) params", "expr", `fun foo(a Int, mut b Str) Int { 123 }`,
			fun(
				"foo",
				fun_params(fun_param("a", typ("Int")), mut_fun_param("b", typ("Str"))),
				typ("Int"),
				funblock(int_expr(123)),
			),
		},
		{
			"fun inside block", "expr", `{ fun foo() Str { "hello" 123 } }`,
			block(
				fun("foo", fun_params(), typ("Str"), funblock(string_expr("hello"), int_expr(123)))),
		},
		{"void fun", "expr", `fun foo() void {}`, fun("foo", fun_params(), typ("void"), funblock())},
		{"call", "expr", `foo(123, "hello")`, call(idente("foo"), int_expr(123), string_expr("hello"))},
		{"call w/o args", "expr", `foo()`, call(idente("foo"))},
	}

	assert := NewAssert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := NewSource("test.met", []rune(tt.src))
			tokens := Lex(source)
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
			assert.Equal(true, ok, "ParseFile returned false")
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
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"unexpected token", `=`, []string{
			"test.met:1:1: unexpected token: =\n" +
				`    =` + "\n" +
				"    ^",
		}},
		{"assign to type", `{ Str = "hello" }`, []string{
			"test.met:1:3: unexpected token: <type identifier>\n" +
				`    { Str = "hello" }` + "\n" +
				"      ^^^",
		}},
	}

	assert := NewAssert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := NewSource("test.met", []rune(tt.src))
			tokens := Lex(source)
			parser := NewParser(tokens)
			expr, parseOK := parser.ParseExpr()
			if parseOK {
				zero := &zeroBaseVisitor{}
				zero.VisitExpr(&expr)
			}
			assert.Equal(false, parseOK, "ParseExpr should have failed: %s", Stringify(expr))
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
	typ.astBase = astBase{}
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

func idente(name string) Expr {
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

func fundecl(name_ string, params []FunParam, return_type ASTType, block Block) Decl {
	name := Name{Name: name_}
	return Decl{Kind: DeclFun, Fun: &Fun{Name: name, Params: params, ReturnType: return_type, Block: block}}
}

func fun(name_ string, params []FunParam, return_type ASTType, block Block) Expr {
	name := Name{Name: name_}
	return Expr{Kind: ExprFun, Fun: &Fun{Name: name, Params: params, ReturnType: return_type, Block: block}}
}

func string_expr(value string) Expr { //nolint:unparam
	return Expr{Kind: ExprString, String: &StringExpr{Value: value}}
}

func int_expr(value int64) Expr {
	return Expr{Kind: ExprInt, Int: &IntExpr{Value: value}}
}

func assign(ident Ident, value Expr) Expr {
	return Expr{Kind: ExprAssign, Assign: &Assign{Ident: ident, Value: value}}
}

func typ(name string) ASTType {
	return ASTType{TypeIdent{Name: name}}
}

func funblock(exprs ...Expr) Block {
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

func block(exprs ...Expr) Expr {
	if exprs == nil {
		exprs = []Expr{}
	}
	return Expr{Kind: ExprBlock, Block: &Block{Exprs: exprs}}
}
