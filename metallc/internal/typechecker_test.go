//nolint:exhaustruct
package internal

import (
	"strings"
	"testing"
)

func TestTypeCheckOK(t *testing.T) {
	t.Parallel()
	Int := builtint("Int")
	Str := builtint("Str")
	void := builtint("void")
	span := NewSpan(NewSource("test.met", []rune{}), 0, 0)

	tests := []struct {
		name  string
		src   string
		want  Type
		check func(*TypeChecker, *Expr, Assert)
	}{
		{"Int", `123`, Int, nil},
		{"Str", `"hello"`, Str, nil},
		{"block", `{ 123 "hello" }`, Str, nil},
		{"empty block is void", `{ }`, void, nil},
		{"let", `let foo = 123`, void, func(tc *TypeChecker, e *Expr, assert Assert) {
			v := e.Var
			typ, err := tc.Env.LookupType(v.ID, span)
			assert.NoError(err)
			assert.Equal(TypeVoid, typ.Kind)
			// Make sure the binding is set correctly.
			b, err := tc.Env.LookupBinding(v.ID, span)
			assert.NoError(err)
			assert.Equal("foo", b.Name)
			assert.Equal(TypeInt, b.Type.Kind)
			assert.Equal(false, b.Mut)
		}},
		{"mut", `mut foo = 123`, void, func(tc *TypeChecker, e *Expr, assert Assert) {
			v := e.Var
			b, err := tc.Env.LookupBinding(v.ID, span)
			assert.NoError(err)
			assert.Equal("foo", b.Name)
			assert.Equal(TypeInt, b.Type.Kind)
			assert.Equal(true, b.Mut)
		}},
		{"assign is void", `{ mut foo = 321 foo = 123 }`, void, func(tc *TypeChecker, e *Expr, assert Assert) {
			block := e.Block
			assign := block.Exprs[1].Assign
			typ, err := tc.Env.LookupType(assign.ID, span)
			assert.NoError(err)
			assert.Equal(TypeVoid, typ.Kind)
		}},
		{
			"fun",
			`fun foo(a Int, b Str) Int { 123 }`,
			funt(Int, Str, Int),
			func(tc *TypeChecker, e *Expr, assert Assert) {
				f := e.Fun
				typ, err := tc.Env.LookupType(f.ID, span)
				assert.NoError(err)
				assert.Equal(TypeFun, typ.Kind)
				assert.Equal(funt(Int, Str, Int).Fun, typ.Fun)
				// Make sure the binding is set correctly.
				b, err := tc.Env.LookupBinding(f.ID, span)
				assert.NoError(err)
				assert.Equal("foo", b.Name)
				assert.Equal(funt(Int, Str, Int).Fun, b.Type.Fun)
			},
		},
		{"fun void return coerces body to void", `fun foo() void { 123 }`, funt(void), nil},
		{"fun params", `fun foo(a Int) Int { a }`, funt(Int, Int), nil},
		{"call", `{ fun foo(a Int) Int { 123 } foo(321) }`, Int, func(tc *TypeChecker, e *Expr, assert Assert) {
			block := e.Block
			call := block.Exprs[1].Call
			typ, err := tc.Env.LookupType(call.ID, span)
			assert.NoError(err)
			assert.Equal(TypeInt, typ.Kind)
		}},
		{"call void fun", `{ fun foo() void { } foo() }`, void, nil},
		{"builtin print_str", `print_str("hello")`, void, nil},
		{"builtin print_int", `print_int(123)`, void, nil},
		{"shadowing", `{ let foo = { let foo = "hello" print_str(foo) 123 } print_int(foo) }`, void, nil},
	}

	assert := NewAssert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := NewSource("test.met", []rune(tt.src))
			tokens := Lex(source)
			parser := NewParser(tokens)
			tc := NewTypeChecker()
			expr, parseOK := parser.ParseExpr()
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			assert.Equal(true, parseOK, "ParseExpr returned false")
			tc.VisitExpr(&expr)
			got, ok := tc.Env.Types[expr.ID()]
			assert.Equal(0, len(tc.Diagnostics), "diagnostics:\n%s", tc.Diagnostics)
			assert.Equal(true, ok, "Type not found after typechecking")
			zeroTypeBase(&got)
			zeroTypeBase(&tt.want)
			WalkType(&got, zeroTypeBase)
			WalkType(&tt.want, zeroTypeBase)
			assert.Equal(tt.want, got)
			if tt.check != nil {
				tt.check(tc, &expr, assert)
			}
		})
	}
}

func TestTypeCheckErr(t *testing.T) {
	t.Parallel()

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
		{"already defined", `{ let foo = 1 let foo = 1 }`, []string{
			"test.met:1:19: symbol already defined: foo\n" +
				`    { let foo = 1 let foo = 1 }` + "\n" +
				"                      ^^^",
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
			"test.met:1:30: argument count mismatch: expected 1, got 3\n" +
				`    { fun foo(a Int) Int { 123 } foo(1, 2, "hello") }` + "\n" +
				"                                 ^^^^^^^^^^^^^^^^^^",
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
	}

	assert := NewAssert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := NewSource("test.met", []rune(tt.src))
			tokens := Lex(source)
			parser := NewParser(tokens)
			tc := NewTypeChecker()
			expr, parseOK := parser.ParseExpr()
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			assert.Equal(true, parseOK)
			tc.VisitExpr(&expr)
			diagnostics := Diagnostics{}
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
	default:
		panic(Errorf("unknown type kind: %d", typ.Kind))
	}
}

func funt(paramsAndReturn ...Type) Type {
	l := len(paramsAndReturn)
	return Type{Kind: TypeFun, Fun: &FunType{Params: paramsAndReturn[0 : l-1], Return: paramsAndReturn[l-1]}}
}

func builtint(name string) Type {
	b, err := NewRootScope().Lookup(name, NewSpan(NewSource("builtin", []rune{}), 0, 0))
	if err != nil {
		panic(err)
	}
	return b.Type
}
