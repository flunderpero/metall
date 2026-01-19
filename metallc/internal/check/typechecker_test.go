//nolint:exhaustruct
package check

import (
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestTypeCheckOK(t *testing.T) {
	Int := builtint("Int")
	Str := builtint("Str")
	void := builtint("void")
	span := base.NewSpan(base.NewSource("test.met", []rune{}), 0, 0)

	tests := []struct {
		name  string
		src   string
		want  Type
		check func(*TypeChecker, *ast.AST, ast.NodeID, base.Assert)
	}{
		{"Int", `123`, Int, nil},
		{"Str", `"hello"`, Str, nil},
		{"block", `{ 123 "hello" }`, Str, nil},
		{"empty block is void", `{ }`, void, nil},
		{"let", `let foo = 123`, void, func(tc *TypeChecker, a *ast.AST, id ast.NodeID, assert base.Assert) {
			typ, err := tc.Env.LookupType(id, span)
			assert.NoError(err)
			assert.Equal(TypeVoid, typ.Kind)
			// Make sure the binding is set correctly.
			b, err := tc.Env.LookupBinding(id, span)
			assert.NoError(err)
			assert.Equal("foo", b.Name)
			assert.Equal(TypeInt, b.Type.Kind)
			assert.Equal(false, b.Mut)
		}},
		{"mut", `mut foo = 123`, void, func(tc *TypeChecker, a *ast.AST, id ast.NodeID, assert base.Assert) {
			b, err := tc.Env.LookupBinding(id, span)
			assert.NoError(err)
			assert.Equal("foo", b.Name)
			assert.Equal(TypeInt, b.Type.Kind)
			assert.Equal(true, b.Mut)
		}},
		{
			"assign is void",
			`{ mut foo = 321 foo = 123 }`,
			void,
			func(tc *TypeChecker, a *ast.AST, id ast.NodeID, assert base.Assert) {
				block, ok := a.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				assignID := block.Exprs[1]
				typ, err := tc.Env.LookupType(assignID, span)
				assert.NoError(err)
				assert.Equal(TypeVoid, typ.Kind)
			},
		},
		{
			"fun",
			`fun foo(a Int, b Str) Int { 123 }`,
			funt(Int, Str, Int),
			func(tc *TypeChecker, a *ast.AST, id ast.NodeID, assert base.Assert) {
				typ, err := tc.Env.LookupType(id, span)
				assert.NoError(err)
				assert.Equal(TypeFun, typ.Kind)
				assert.Equal(funt(Int, Str, Int).Fun, typ.Fun)
				// Make sure the binding is set correctly.
				b, err := tc.Env.LookupBinding(id, span)
				assert.NoError(err)
				assert.Equal("foo", b.Name)
				assert.Equal(funt(Int, Str, Int).Fun, b.Type.Fun)
			},
		},
		{"fun void return coerces body to void", `fun foo() void { 123 }`, funt(void), nil},
		{"fun params", `fun foo(a Int) Int { a }`, funt(Int, Int), nil},
		{
			"call",
			`{ fun foo(a Int) Int { 123 } foo(321) }`,
			Int,
			func(tc *TypeChecker, a *ast.AST, id ast.NodeID, assert base.Assert) {
				block, ok := a.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				callID := block.Exprs[1]
				typ, err := tc.Env.LookupType(callID, span)
				assert.NoError(err)
				assert.Equal(TypeInt, typ.Kind)
			},
		},
		{"call void fun", `{ fun foo() void { } foo() }`, void, nil},
		{"builtin print_str", `print_str("hello")`, void, nil},
		{"builtin print_int", `print_int(123)`, void, nil},
		{"shadowing", `{ let foo = { let foo = "hello" print_str(foo) 123 } print_int(foo) }`, void, nil},

		{"ref", `{ let a = 5 let b = &a b }`, ref_t(Int), nil},
		{"mut ref", `{ mut a = 5 mut b = &a b }`, ref_mut_t(Int), nil},
		{"deref", `{ let a = 5 let b = &a *b }`, Int, nil},
		{"deref assign", `{ mut a = 1 mut b = &a *b = 321 }`, void, nil},
		{"nested deref assign", `{ mut a = 1 mut b = &a mut c = &b *b = 123 **c = 321 }`, void, nil},
		{"mut ref parameter", `{ fun foo(mut a &Int) void { *a = 321 } let b = 123 foo(&b) }`, void, nil},
		{"ref return", `{ fun foo(a &Int) &Int { a } let b = 123 foo(&b) }`, ref_t(Int), nil},
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
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens)
			exprID, parseOK := parser.ParseExpr()
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			assert.Equal(true, parseOK, "ParseExpr returned false")
			tc := NewTypeChecker(parser.AST)
			tc.Check(exprID)
			got, ok := tc.Env.Types[exprID]
			assert.Equal(0, len(tc.Diagnostics), "diagnostics:\n%s", tc.Diagnostics)
			assert.Equal(true, ok, "Type not found after typechecking")
			zeroTypeBase(&got)
			zeroTypeBase(&tt.want)
			assert.Equal(tt.want, got)
			if tt.check != nil {
				tt.check(tc, parser.AST, exprID, assert)
			}
		})
	}
}

func TestTypeCheckErr(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"not defined", `let bar = foo`, []string{
			"test.met:1:11: symbol not defined: foo\n" +
				"    let bar = foo\n" +
				"              ^^^",
		}},
		{"fun return type mismatch", `fun foo() Str { 123 }`, []string{
			"test.met:1:17: return type mismatch: expected Str, got Int\n" +
				"    fun foo() Str { 123 }\n" +
				"                    ^^^",
		}},
		{"assign mismatch", `{ mut foo = 123 foo = "hello" }`, []string{
			"test.met:1:23: type mismatch: expected Int, got Str\n" +
				`    { mut foo = 123 foo = "hello" }` + "\n" +
				"                          ^^^^^^^",
		}},
		{"assign to immutable var", `{ let foo = 123 foo = 321 }`, []string{
			"test.met:1:17: cannot assign to immutable variable: foo\n" +
				`    { let foo = 123 foo = 321 }` + "\n" +
				"                    ^^^",
		}},
		{"call argument count mismatch", `{ fun foo(a Int) Int { 123 } foo(1, 2, "hello") }`, []string{
			"test.met:1:33: argument count mismatch: expected 1, got 3\n" +
				`    { fun foo(a Int) Int { 123 } foo(1, 2, "hello") }` + "\n" +
				"                                    ^^^^^^^^^^^^^^^",
		}},
		{"call argument type mismatch", `{ fun foo(a Int, b Int) Int { 123 } foo("hello", 2) }`, []string{
			"test.met:1:41: type mismatch at argument 1: expected Int, got Str\n" +
				`    { fun foo(a Int, b Int) Int { 123 } foo("hello", 2) }` + "\n" +
				"                                            ^^^^^^^",
		}},
		{"main must return void", `fun main() Int { 123 }`, []string{
			"test.met:1:12: main function cannot return a value\n" +
				`    fun main() Int { 123 }` + "\n" +
				`               ^^^`,
		}},
		{"main must not have parameters", `fun main(a Int, b Str) void { }`, []string{
			"test.met:1:10: main function cannot take arguments\n" +
				`    fun main(a Int, b Str) void { }` + "\n" +
				`             ^^^^^^^^^^^^`,
		}},

		{"deref a non-ref", `{ let foo = 5 *foo }`, []string{
			"test.met:1:16: dereference: expected reference, got Int\n" +
				`    { let foo = 5 *foo }` + "\n" +
				`                   ^^^`,
		}},
		{"assign through immutable deref", `{ let a = 5 let b = &a *b = 321 }`, []string{
			"test.met:1:24: cannot assign through dereference: expected mutable reference, got &Int\n" +
				`    { let a = 5 let b = &a *b = 321 }` + "\n" +
				`                           ^^`,
		}},
		{"calling ref param with value", `{ fun foo(a &Int) void {} let bar = 123 foo(bar) }`, []string{
			"test.met:1:45: type mismatch at argument 1: expected &Int, got Int\n" +
				`    { fun foo(a &Int) void {} let bar = 123 foo(bar) }` + "\n" +
				`                                                ^^^`,
		}},
		{"assign through immutable fun param", `{ fun foo(a &Int) void { *a = 123 }}`, []string{
			"test.met:1:26: cannot assign through dereference: expected mutable reference, got &Int\n" +
				`    { fun foo(a &Int) void { *a = 123 }}` + "\n" +
				`                             ^^`,
		}},
		{"take mutable ref to immutable in var", `{ let a = 123 mut b = &a }`, []string{
			"test.met:1:23: cannot take a mutable reference to an immutable value\n" +
				`    { let a = 123 mut b = &a }` + "\n" +
				`                          ^^`,
		}},
		{"take mutable ref to immutable in assign", `{ mut a = 123 let b = 123 mut c = &a c = &b }`, []string{
			"test.met:1:42: type mismatch: expected &mut Int, got &Int\n" +
				`    { mut a = 123 let b = 123 mut c = &a c = &b }` + "\n" +
				`                                             ^^`,
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
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens)
			exprID, parseOK := parser.ParseExpr()
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			assert.Equal(true, parseOK)
			tc := NewTypeChecker(parser.AST)
			tc.Check(exprID)
			diagnostics := base.Diagnostics{}
			for _, diag := range tc.Diagnostics {
				if !strings.Contains(diag.Message, "no type set for AST node") {
					diagnostics = append(diagnostics, diag)
				}
			}
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

func zeroTypeBase(typ *Type) {
	switch {
	case typ.BuiltIn != nil:
		typ.BuiltIn.typBase = typBase{}
	case typ.Fun != nil:
		typ.Fun.typBase = typBase{}
	case typ.Ref != nil:
		typ.Ref.typBase = typBase{}
	default:
		panic(base.Errorf("unknown type kind: %d", typ.Kind))
	}
	WalkType(typ, zeroTypeBase)
}

func funt(paramsAndReturn ...Type) Type {
	l := len(paramsAndReturn)
	return Type{Kind: TypeFun, Fun: &FunType{Params: paramsAndReturn[0 : l-1], Return: paramsAndReturn[l-1]}}
}

func ref_t(typ Type) Type {
	return NewRefType(&RefType{typBase{}, typ, false})
}

func ref_mut_t(typ Type) Type {
	return NewRefType(&RefType{typBase{}, typ, true})
}

func builtint(name string) Type {
	b, err := NewRootScope().Lookup(name, base.NewSpan(base.NewSource("builtin", []rune{}), 0, 0))
	if err != nil {
		panic(err)
	}
	return b.Type
}
