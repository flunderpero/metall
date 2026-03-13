//nolint:exhaustruct
package types

import (
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/modules"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestMatchOK(t *testing.T) {
	void := &Type{1, 0, base.Span{}, VoidType{}}
	Int := &Type{7, 0, base.Span{}, lookupIntType("Int")}
	Str := &Type{13, 0, base.Span{}, StructType{
		Name:   "Str",
		Fields: []StructField{{Name: "data", Type: TypeID(91), Mut: false}},
	}}

	tests := []struct {
		name  string
		src   string
		want  *Type
		check func(*Engine, ast.NodeID, base.Assert)
	}{
		{
			"match union returns variant type",
			`{
				union Foo = Str | Int
				let x = Foo("hello")
				match x {
					case Str: "str"
					case Int: "int"
				}
			}`,
			Str, nil,
		},
		{
			"match union with binding",
			`{
				union Foo = Str | Int
				let x = Foo(42)
				match x {
					case Int n: print_int(n)
					case Str s: print_str(s)
				}
			}`,
			void, nil,
		},
		{
			"match union with else",
			`{
				union Foo = Str | Int | Bool
				let x = Foo("hello")
				match x {
					case Str: "found str"
					else: "other"
				}
			}`,
			Str, nil,
		},
		{
			"match union all arms diverge",
			`fun foo() Int {
				union Foo = Str | Int
				let x = Foo(42)
				match x {
					case Str: return 1
					case Int: return 2
				}
			}`,
			fun_t(Int), nil,
		},
		{
			"match diverging arm excluded from result type",
			`fun foo() Int {
				union Tri = Int | Bool | Str
				let x = Tri(42)
				match x {
					case Str: return 0
					case Int n: n
					case Bool: 99
				}
			}`,
			fun_t(Int), nil,
		},
		{
			"match union with generic type",
			`{
				union Maybe<T> = T | Bool
				let x = Maybe<Int>(42)
				match x {
					case Int n: n
					case Bool: 0
				}
			}`,
			Int, nil,
		},
	}

	assert := base.NewAssert(t)
	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		name := strings.TrimSpace(strings.ReplaceAll(tt.name, "!"+"only", ""))
		t.Run(name, func(t *testing.T) {
			source := base.NewSource("test.met", "test", true, []rune(tt.src))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens, ast.NewAST(1))
			exprID, parseOK := parser.ParseExpr(0)
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			assert.Equal(true, parseOK, "ParseExpr returned false")
			preludeAST, _ := ast.PreludeAST(true)
			parser.Roots = append(parser.Roots, exprID)
			e := NewEngine(parser.AST, preludeAST, &modules.ModuleResolution{})
			e.Query(exprID)
			assert.Equal(0, len(e.diagnostics), "diagnostics:\n%s", e.diagnostics)
			got := e.env.TypeOfNode(exprID)
			if tt.want != nil {
				assert.NotEqual(InvalidTypeID, got.ID, "result type is invalid")
				e.env.IterTypes(zeroIDAndSpan)
				assert.Equal(tt.want, got)
			} else {
				e.env.IterTypes(zeroIDAndSpan)
				assert.NotNil(tt.check, "`tt.check` cannot be nil if `tt.want` is already nil")
			}
			if tt.check != nil {
				tt.check(e, exprID, assert)
			}
			a := NewLifetimeAnalyzer(e.ast, e.scopeGraph, e.Env())
			a.Check(exprID)
			assert.Equal(0, len(a.Diagnostics), "lifetime check failed: %s", a.Diagnostics)
		})
	}
}

func TestMatchErr(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"match on non-union", `
			{
				let x = 42
				match x { case Int: 1 }
			}
			`, []string{
			"test.met:4:23: match expression must be a union type, got Int\n" +
				strings.Trim(`
					let x = 42
					match x { case Int: 1 }
					      ^
				}
				`, "\n"),
		}},
		{"match non-exhaustive", `
			{
				union Foo = Str | Int
				let x = Foo("hi")
				match x { case Str: "s" }
			}
			`, []string{
			"test.met:5:17: non-exhaustive match: missing variant Int\n" +
				strings.Trim(`
					let x = Foo("hi")
					match x { case Str: "s" }
					^^^^^^^^^^^^^^^^^^^^^^^^^
				}
				`, "\n"),
		}},
		{"match duplicate arm", `
			{
				union Foo = Str | Int
				let x = Foo(1)
				match x {
					case Str: "s"
					case Str: "s2"
					case Int: "i"
				}
			}
			`, []string{
			"test.met:7:26: duplicate match arm for variant Str\n" +
				strings.Trim(`
						case Str: "s"
						case Str: "s2"
						     ^^^
						case Int: "i"
				`, "\n"),
		}},
		{"match arm type mismatch", `
			{
				union Foo = Str | Int
				let x = Foo("hi")
				match x {
					case Str: "s"
					case Int: 42
				}
			}
			`, []string{
			"test.met:7:31: match arm type mismatch: expected Str, got Int\n" +
				strings.Trim(`
						case Str: "s"
						case Int: 42
						          ^^
					}
				`, "\n"),
		}},
		{"match not a variant", `
			{
				union Foo = Str | Int
				let x = Foo(1)
				match x {
					case Bool: true
					case Str: "s"
					case Int: "i"
				}
			}
			`, []string{
			"test.met:6:26: type Bool is not a variant of Foo\n" +
				strings.Trim(`
					match x {
						case Bool: true
						     ^^^^
						case Str: "s"
				`, "\n"),
		}},
		{"match all arms diverge cannot assign", `
			fun foo() Int {
				union Foo = Str | Int
				let x = Foo(42)
				let y = match x {
					case Str: return 1
					case Int: return 2
				}
				y
			}
			`, []string{
			"test.met:5:17: cannot assign void to a variable\n" +
				strings.Trim(`
					let x = Foo(42)
					let y = match x {
					^
						case Str: return 1
						case Int: return 2
					}
					^
					y
				`, "\n"),
			"test.met:9:17: symbol not defined: y\n" +
				strings.Trim(`
					}
					y
					^
				}
				`, "\n"),
		}},
	}

	assert := base.NewAssert(t)
	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		name := strings.TrimSpace(strings.ReplaceAll(tt.name, "!"+"only", ""))
		t.Run(name, func(t *testing.T) {
			source := base.NewSource("test.met", "test", true, []rune(strings.ReplaceAll(tt.src, "\t", "    ")))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens, ast.NewAST(1))
			exprID, _ := parser.ParseExpr(0)
			parser.Roots = append(parser.Roots, exprID)
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			preludeAST, _ := ast.PreludeAST(true)
			e := NewEngine(parser.AST, preludeAST, &modules.ModuleResolution{})
			e.Query(exprID)
			for i, want := range tt.want {
				if i >= len(e.diagnostics) {
					t.Fatalf("no more diagnostics, but wanted: %s", want)
				}
				want = strings.Trim(strings.ReplaceAll(want, "\t", "    "), "\n")
				want = strings.TrimRight(want, " ")
				assert.Equal(want, e.diagnostics[i].Display())
			}
			if len(e.diagnostics) > len(tt.want) {
				t.Fatalf(
					"there are more diagnostics than expected. \n>>> want:\n%s\n>>> got:\n%s",
					tt.want,
					e.diagnostics,
				)
			}
		})
	}
}
