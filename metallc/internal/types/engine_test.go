//nolint:exhaustruct
package types

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestTypeCheckAndLifetimeOK(t *testing.T) {
	// TypeIDs for builtin types are stable, so we can do this.
	span := base.NewSpan(base.NewSource("builtin", []rune{}), 0, 0)
	void := &Type{1, span, BuiltInType{"void"}}
	Int := &Type{2, span, BuiltInType{"Int"}}
	Str := &Type{3, span, BuiltInType{"Str"}}
	Bool := &Type{4, span, BuiltInType{"Bool"}}

	tests := []struct {
		name  string
		src   string
		want  *Type
		check func(*Engine, ast.NodeID, base.Assert)
	}{
		{"Int", `123`, Int, nil},
		{"Str", `"hello"`, Str, nil},
		{"block", `{ 123 "hello" }`, Str, nil},
		{"empty block is void", `{ }`, void, nil},
		{"let", `let foo = 123`, void, func(e *Engine, _ ast.NodeID, assert base.Assert) {
			// Make sure the binding type is set correctly.
			b, _, ok := e.Scope().Lookup("foo")
			assert.Equal(true, ok)
			bindingType := e.Type(b.TypeID)
			assert.Equal(Int, bindingType)
			assert.Equal(false, b.Mut)
		}},
		{"mut", `mut foo = 123`, void, func(e *Engine, _ ast.NodeID, assert base.Assert) {
			// Make sure the binding type is set correctly.
			b, _, ok := e.Scope().Lookup("foo")
			assert.Equal(true, ok)
			bindingType := e.Type(b.TypeID)
			assert.Equal(Int, bindingType)
			assert.Equal(true, b.Mut)
		}},
		{
			"assign is void",
			`{ mut foo = 321 foo = 123 }`,
			void,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block, ok := e.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				assignID := block.Exprs[1]
				typ := e.TypeOfNode(assignID)
				assert.Equal(void, typ)
			},
		},
		{"fun", `fun foo(a Int, b Str) Int { 123 }`, fun_t(Int, Str, Int), nil},
		{"fun void return coerces body to void", `fun foo() void { 123 }`, fun_t(void), nil},
		{"fun params", `fun foo(a Int) Int { a }`, fun_t(Int, Int), nil},
		{
			"fun params are scoped to the fun",
			`{ fun foo(a Int) void {} fun bar(a Int) void {} }`,
			fun_t(Int, void),
			nil,
		},
		{"call", `{ fun foo(a Int) Int { 123 } foo(321) }`, Int, nil},
		{"call void fun", `{ fun foo() void { } foo() }`, void, nil},
		{"builtin print_str", `print_str("hello")`, void, nil},
		{"builtin print_int", `print_int(123)`, void, nil},
		{"builtin print_bool", `print_bool(true)`, void, nil},
		{"shadowing", `{ let foo = { let foo = "hello" print_str(foo) 123 } print_int(foo) }`, void, nil},

		{"struct", `struct Planet { name Str diameter Int }`, struct_t("Planet", "name", Str, "diameter", Int), nil},
		{
			"forward declare struct type", `{ fun foo(a Planet) void {} struct Planet { name Str } }`,
			struct_t("Planet", "name", Str),
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block, ok := e.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				funID := block.Exprs[0]
				typ, ok := e.TypeOfNode(funID).Kind.(FunType)
				assert.Equal(true, ok, e.TypeOfNode(funID).ID)
				assert.Equal("struct Planet(name Str)", e.TypeDisplay(typ.Params[0]))
			},
		},
		{
			"struct literal", `{ struct Planet { name Str diameter Int } let earth = Planet("Earth", 12500) earth }`,
			struct_t("Planet", "name", Str, "diameter", Int), nil,
		},
		{
			"struct ref",
			`{ struct Planet { name Str } let p = Planet("Earth") &p }`,
			// Our test strategy does not work for nested types (we zero out all type ids).
			// That's why we verify in the check function.
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				got := e.TypeOfNode(id)
				ref, ok := got.Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal("struct Planet(name Str)", e.TypeDisplay(ref.Type))
			},
		},
		{"field read access", `{ struct Planet { name Str } let earth = Planet("Earth") earth.name }`, Str, nil},
		{
			"field write access",
			`{ struct Planet { mut name Str } mut earth = Planet("Earth") earth.name = "Mother" }`,
			void,
			nil,
		},
		{
			"field write through mut ref param",
			`{ struct Planet { mut name Str } fun foo(p &mut Planet) void { p.name = "X" } mut p = Planet("Earth") foo(&p) }`,
			void,
			nil,
		},
		{
			"nested field write on mut struct",
			`{ struct Inner { mut x Int } struct Outer { mut inner Inner } mut o = Outer(Inner(1)) o.inner.x = 2 }`,
			void,
			nil,
		},
		{
			"field write through let binding of mut ref",
			`{ struct Planet { mut name Str } mut earth = Planet("Earth") let p = &earth p.name = "X" }`,
			void,
			nil,
		},

		{"bool true", "{ true }", Bool, nil},
		{"bool false", "{ false }", Bool, nil},
		{"if then else", `{ let a = true if a { 42 } else { 123 }}`, Int, nil},
		{"if w/o else", `{ let a = true if a { 42 } }`, void, nil},

		{"ref", `{ let a = 5 let b = &a b }`, ref_t(Int), nil},
		{"mut binding of immutable ref", `{ let a = 5 mut b = &a b }`, ref_t(Int), nil},
		{"mut ref", `{ mut a = 5 mut b = &a b }`, ref_mut_t(Int), nil},
		{"deref", `{ let a = 5 let b = &a *b }`, Int, nil},
		{
			"deref field access",
			`{ struct Planet{ name Str } let p = Planet("Earth") let r = &p r.name }`,
			Str,
			nil,
		},
		{"deref assign", `{ mut a = 1 mut b = &a *b = 321 }`, void, nil},
		{"nested deref assign", `{ mut a = 1 mut b = &a mut c = &b *b = 123 **c = 321 }`, void, nil},
		{"mut ref parameter", `{ fun foo(a &mut Int) void { *a = 321 } mut b = 123 foo(&b) }`, void, nil},
		{"mut ref coercion", `{ fun foo(a &Int) void {} mut b = 123 foo(&b) }`, void, nil},
		{"mut ref coercion in struct literal", `{ struct Foo { ptr &Int } mut a = 1 let f = Foo(&a) }`, void, nil},
		{"ref return", `{ fun foo(a &Int) &Int { a } let b = 123 foo(&b) }`, ref_t(Int), nil},
		{
			"write through mut ref struct field",
			`{ struct Foo { ptr &mut Int } mut a = 1 let f = Foo(&a) *f.ptr = 42 }`,
			void,
			nil,
		},
		{
			"reassign mut field of mut ref type",
			`{ struct Foo { mut ptr &mut Int } mut a = 1 mut b = 2 mut f = Foo(&a) f.ptr = &b *f.ptr = 99 }`,
			void,
			nil,
		},

		{"forward declaration call", `{ foo() fun foo() void { } }`, fun_t(void), nil},
		{"self recursion", `{ fun foo(a Int) Int { foo(a) } foo(1) }`, Int, nil},
		{"mutual recursion", `{ fun foo(a Int) Int { bar(a) } fun bar(a Int) Int { foo(a) } foo(10) }`, Int, nil},

		{"declare alloc", `alloc @test = Arena()`, void, func(e *Engine, id ast.NodeID, assert base.Assert) {
			scope := e.ScopeGraph.NodeScope(id)
			b, _, ok := scope.Lookup("@test")
			assert.Equal(true, ok)
			typ, ok := e.Type(b.TypeID).Kind.(AllocType)
			assert.Equal(true, ok)
			assert.Equal(AllocArena, typ.Impl)
		}},
		{
			"alloc", `{ alloc @test = Arena() struct Planet{name Str} let p = @test Planet("Earth") p }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block, ok := e.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				_, ok = e.TypeOfNode(lastExpr).Kind.(StructType)
				assert.Equal(true, ok)
				assert.Equal("struct Planet(name Str)", e.TypeDisplay(e.TypeOfNode(lastExpr).ID))
			},
		},
		{"pass alloc to fun", `{ fun foo(@alloc Arena) void {} alloc @a = Arena() foo(@a) }`, void, nil},

		{"array type", `fun foo(arr [5]Int) void {}`, nil, func(e *Engine, id ast.NodeID, assert base.Assert) {
			fun, ok := e.TypeOfNode(id).Kind.(FunType)
			assert.Equal(true, ok)
			assert.Equal(1, len(fun.Params))
			arr, ok := e.Type(fun.Params[0]).Kind.(ArrayType)
			assert.Equal(true, ok)
			assert.Equal(int64(5), arr.Len)
			assert.Equal(Int.ID, arr.Elem)
		}},
		{
			"array type ids are stable",
			`fun foo(a [5]Int, b [5]Int, c [6]Int) void { [1, 2, 3, 4, 5]}`,
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				funNode, ok := e.Node(id).Kind.(ast.Fun)
				assert.Equal(ok, true)
				fun, ok := e.TypeOfNode(id).Kind.(FunType)
				assert.Equal(true, ok)
				assert.Equal(3, len(fun.Params))
				for _, elem := range fun.Params {
					_, ok := e.Type(elem).Kind.(ArrayType)
					assert.Equal(true, ok)
				}
				assert.Equal(fun.Params[0], fun.Params[1])
				assert.NotEqual(fun.Params[0], fun.Params[2])
				// The array literal in the body should have the same type as param 0.
				block, ok := e.Node(funNode.Block).Kind.(ast.Block)
				assert.Equal(true, ok)
				literalTypeID := e.TypeOfNode(block.Exprs[0]).ID
				assert.Equal(fun.Params[0], literalTypeID)
			},
		},
		{"array alloc", `{ alloc @a = Arena() @a [5]Int() }`, arr_t(Int, 5), nil},
		{"array literal", `[1, 2, 3]`, arr_t(Int, 3), nil},
		{"index read", `{ let a = [1, 2, 3] a[1] }`, Int, nil},
		{"index write", `{ mut a = [1, 2, 3] a[1] = 5 }`, void, nil},
	}

	// We need a little hack here, because the "ref" and "mut ref" tests
	// violate the lifetime rules, but we still wan to test them in isolation.
	skipLifetimeCheck := []string{
		"ref",
		"mut binding of immutable ref",
		"mut ref",
		"struct ref",
		"ref return",
		"alloc",
		"array alloc",
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
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens)
			exprID, parseOK := parser.ParseExpr()
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			assert.Equal(true, parseOK, "ParseExpr returned false")
			e := NewEngine(parser.AST)
			// e.Debug = base.NewStdoutDebug("engine")
			e.Query(exprID)
			assert.Equal(0, len(e.Diagnostics), "diagnostics:\n%s", e.Diagnostics)
			got := e.TypeOfNode(exprID)
			if tt.want != nil {
				assert.NotEqual(InvalidTypeID, got.ID, "result type is invalid")
				e.IterTypes(zeroIDAndSpan)
				assert.Equal(tt.want, got)
			} else {
				assert.NotNil(tt.check, "`tt.check` cannot be nil if `tt.want` is already nil")
			}
			if tt.check != nil {
				tt.check(e, exprID, assert)
			}
			if !slices.Contains(skipLifetimeCheck, name) {
				a := NewLifetimeAnalyzer(e)
				// a.Debug = base.NewStdoutDebug("lifetime")
				a.Check(exprID)
				assert.Equal(0, len(a.Diagnostics), "lifetime check failed: %s", a.Diagnostics)
			}
			// Make sure every node has a scope.
			parser.Iter(func(nodeID ast.NodeID) bool {
				_, ok := e.ScopeGraph.scopeByNodeID[nodeID]
				assert.Equal(true, ok, "no scope for %s", e.AST.Debug(nodeID, false, 0))
				return true
			})
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
		{"not defined (var comes later)", `{ print_int(a) let a = 123 }`, []string{
			"test.met:1:13: symbol not defined: a\n" +
				`    { print_int(a) let a = 123 }` + "\n" +
				"                ^",
		}},
		{"rebind var", `{ let foo = 123 let foo = 321 }`, []string{
			"test.met:1:21: symbol already defined: foo\n" +
				`    { let foo = 123 let foo = 321 }` + "\n" +
				"                        ^^^",
		}},
		{"rebind fun", `{ fun foo() void {} fun foo() void {} }`, []string{
			"test.met:1:25: symbol already defined: foo\n" +
				`    { fun foo() void {} fun foo() void {} }` + "\n" +
				"                            ^^^",
		}},
		{"rebind fun param", `fun foo(bar Int) void { let bar = 123 }`, []string{
			"test.met:1:29: symbol already defined: bar\n" +
				`    fun foo(bar Int) void { let bar = 123 }` + "\n" +
				"                                ^^^",
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
		{"call non-function", `{ 123() }`, []string{
			"test.met:1:3: cannot call non-function: Int\n" +
				`    { 123() }` + "\n" +
				`      ^^^`,
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

		{"field access on non-struct type", `123.name`, []string{
			"test.met:1:1: cannot access field on a non-struct type: Int\n" +
				`    123.name` + "\n" +
				`    ^^^`,
		}},
		{
			"field access of non-existing field",
			`{ struct Planet{name Str} let earth = Planet("Earth") earth.age }`,
			[]string{
				"test.met:1:61: unknown field: Planet.age\n" +
					`    { struct Planet{name Str} let earth = Planet("Earth") earth.age }` + "\n" +
					"                                                                ^^^",
			},
		},

		{"if cond must be bool", `{ if 123 { } }`, []string{
			"test.met:1:6: if condition must evaluate to a boolean value, got Int\n" +
				`    { if 123 { } }` + "\n" +
				`         ^^^`,
		}},
		{"if then/else must match", `{ if true { 123 } else { "hello" } }`, []string{
			"test.met:1:24: if branch type mismatch: expected Int, got Str\n" +
				`    { if true { 123 } else { "hello" } }` + "\n" +
				"                           ^^^^^^^^^^^",
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

		{"take mutable ref to immutable in assign", `{ mut a = 123 let b = 123 mut c = &a c = &b }`, []string{
			"test.met:1:42: type mismatch: expected &mut Int, got &Int\n" +
				`    { mut a = 123 let b = 123 mut c = &a c = &b }` + "\n" +
				`                                             ^^`,
		}},
		{
			"assign to field of immutable struct",
			`{ struct Planet{mut name Str} let p = Planet("Earth") p.name = "Mother" }`,
			[]string{
				"test.met:1:55: cannot assign to field of immutable value\n" +
					`    { struct Planet{mut name Str} let p = Planet("Earth") p.name = "Mother" }` + "\n" +
					"                                                          ^^^^^^",
			},
		},
		{
			"assign to nested field of immutable struct",
			`{ struct Inner{mut x Int} struct Outer{mut inner Inner} let o = Outer(Inner(1)) o.inner.x = 2 }`,
			[]string{
				"test.met:1:81: cannot assign to field of immutable value\n" +
					`    { struct Inner{mut x Int} struct Outer{mut inner Inner} let o = Outer(Inner(1)) o.inner.x = 2 }` + "\n" +
					"                                                                                    ^^^^^^^^^",
			},
		},
		{
			"assign to nested field through immutable intermediate field",
			`{ struct Inner{mut x Int} struct Outer{inner Inner} mut o = Outer(Inner(1)) o.inner.x = 2 }`,
			[]string{
				"test.met:1:77: cannot assign to field of immutable value\n" +
					`    { struct Inner{mut x Int} struct Outer{inner Inner} mut o = Outer(Inner(1)) o.inner.x = 2 }` + "\n" +
					"                                                                                ^^^^^^^^^",
			},
		},
		{
			"assign to field through immutable ref",
			`{ struct Planet{mut name Str} let p = Planet("Earth") let r = &p r.name = "X" }`,
			[]string{
				"test.met:1:66: cannot assign to field of immutable value\n" +
					`    { struct Planet{mut name Str} let p = Planet("Earth") let r = &p r.name = "X" }` + "\n" +
					"                                                                     ^^^^^^",
			},
		},
		{
			"assign to field through immutable ref param", `{ struct Planet{mut name Str} fun foo(p &Planet) void { p.name = "X" } }`, []string{
				"test.met:1:57: cannot assign to field of immutable value\n" +
					`    { struct Planet{mut name Str} fun foo(p &Planet) void { p.name = "X" } }` + "\n" +
					"                                                            ^^^^^^",
			},
		},
		{
			"assign to immutable field on mutable struct",
			`{ struct Foo { name Str } mut f = Foo("hi") f.name = "bye" }`,
			[]string{
				"test.met:1:45: cannot assign to immutable field: name\n" +
					`    { struct Foo { name Str } mut f = Foo("hi") f.name = "bye" }` + "\n" +
					"                                                ^^^^^^",
			},
		},
		{
			"pass immutable ref to mut ref struct field",
			`{ struct Foo { ptr &mut Int } let a = 123 let f = Foo(&a) }`,
			[]string{
				"test.met:1:55: type mismatch at argument 1: expected &mut Int, got &Int\n" +
					`    { struct Foo { ptr &mut Int } let a = 123 let f = Foo(&a) }` + "\n" +
					"                                                          ^^",
			},
		},
		{
			"write through immutable ref struct field",
			`{ struct Foo { ptr &Int } let a = 123 let f = Foo(&a) *f.ptr = 42 }`,
			[]string{
				"test.met:1:55: cannot assign through dereference: expected mutable reference, got &Int\n" +
					`    { struct Foo { ptr &Int } let a = 123 let f = Foo(&a) *f.ptr = 42 }` + "\n" +
					"                                                          ^^^^^^",
			},
		},
		{
			"reassign immutable mut-ref field",
			`{ struct Foo { ptr &mut Int } mut a = 1 mut b = 2 mut f = Foo(&a) f.ptr = &b }`,
			[]string{
				"test.met:1:67: cannot assign to immutable field: ptr\n" +
					`    { struct Foo { ptr &mut Int } mut a = 1 mut b = 2 mut f = Foo(&a) f.ptr = &b }` + "\n" +
					"                                                                      ^^^^^",
			},
		},
		{
			"reassign mut ref param",
			`{ fun foo(p &mut Int) void { mut x = 1 p = &x } }`,
			[]string{
				"test.met:1:40: cannot assign to immutable variable: p\n" +
					`    { fun foo(p &mut Int) void { mut x = 1 p = &x } }` + "\n" +
					"                                           ^",
			},
		},
		{"non-existing allocator", `{ struct Planet{name Str} let p = @test Planet("Earth") }`, []string{
			"test.met:1:35: unknown allocator: @test\n" +
				`    { struct Planet{name Str} let p = @test Planet("Earth") }` + "\n" +
				`                                      ^^^^^`,
		}},
		{"index non-array", `{ let a = 123 a[0] }`, []string{
			"test.met:1:15: not an array: Int\n" +
				"    { let a = 123 a[0] }\n" +
				"                  ^",
		}},
		{"index with non-int", `{ let a = [1, 2, 3] a["hello"] }`, []string{
			"test.met:1:23: index type mismatch: expected Int, got Str\n" +
				`    { let a = [1, 2, 3] a["hello"] }` + "\n" +
				`                          ^^^^^^^`,
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
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens)
			exprID, parseOK := parser.ParseExpr()
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			assert.Equal(true, parseOK)
			e := NewEngine(parser.AST)
			e.Query(exprID)
			for i, want := range tt.want {
				if i >= len(e.Diagnostics) {
					t.Fatalf("no more diagnostics, but wanted: %s", want)
				}
				assert.Equal(want, e.Diagnostics[i].Display())
			}
			if len(e.Diagnostics) > len(tt.want) {
				t.Fatalf("there are more diagnostics than expected: %s", e.Diagnostics[len(tt.want):])
			}
		})
	}
}

func TestScopes(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		scopes string // "scopeID:parentID" pairs, one per line
		nodes  string // "nodeDebug:scopeID" pairs, one per line
	}{
		{
			name: "simple var",
			src:  `let a = 1`,
			scopes: `
				a:-
			`,
			nodes: `
				n1:Int(value=1):a
				n2:Var(name="a",mut=false,expr=n1:Int):a
			`,
		},
		{
			name: "block creates scope",
			src:  `{ let a = 1 }`,
			scopes: `
				a:-
				b:a
			`,
			nodes: `
				n1:Int(value=1):b
				n2:Var(name="a",mut=false,expr=n1:Int):b
				n3:Block(createScope=true,exprs=[n2:Var]):a
			`,
		},
		{
			name: "nested blocks",
			src:  `{ let a = 1 { let b = 2 } }`,
			scopes: `
				a:-
				b:a
				c:b
			`,
			nodes: `
				n1:Int(value=1):b
				n2:Var(name="a",mut=false,expr=n1:Int):b
				n3:Int(value=2):c
				n4:Var(name="b",mut=false,expr=n3:Int):c
				n5:Block(createScope=true,exprs=[n4:Var]):b
				n6:Block(createScope=true,exprs=[n2:Var,n5:Block]):a
			`,
		},
		{
			name: "function",
			src:  `fun foo(a Int) Int { a }`,
			scopes: `
				a:-
				b:a
			`,
			nodes: `
				n1:SimpleType(name="Int"):b
				n2:FunParam(name="a",type=n1:SimpleType):b
				n3:SimpleType(name="Int"):a
				n4:Ident(name="a"):b
				n5:Block(createScope=false,exprs=[n4:Ident]):b
				n6:Fun(name="foo",params=[n2:FunParam],returnType=n3:SimpleType,block=n5:Block):a
			`,
		},
		{
			name: "function with nested block",
			src:  `fun foo() void { { 1 } }`,
			scopes: `
				a:-
				b:a
				c:b
			`,
			nodes: `
				n1:SimpleType(name="void"):a
				n2:Int(value=1):c
				n3:Block(createScope=true,exprs=[n2:Int]):b
				n4:Block(createScope=false,exprs=[n3:Block]):b
				n5:Fun(name="foo",params=[],returnType=n1:SimpleType,block=n4:Block):a
			`,
		},
	}

	assert := base.NewAssert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens)
			exprID, parseOK := parser.ParseExpr()
			assert.Equal(true, parseOK, "ParseExpr returned false")
			assert.Equal(0, len(parser.Diagnostics), "parse errors: %s", parser.Diagnostics)

			e := NewEngine(parser.AST)
			e.Query(exprID)
			assert.Equal(0, len(e.Diagnostics), "type errors: %s", e.Diagnostics)

			// Verify scopes: collect all scopes and check parent relationships.
			gotScopes := collectScopes(e, parser.AST)
			wantScopes := parseSnapshot(tt.scopes)
			assert.Equal(wantScopes, gotScopes, "scopes mismatch")

			// Verify nodes: check each node has the expected scope.
			gotNodes := collectNodes(e, parser.AST)
			wantNodes := parseSnapshot(tt.nodes)
			assert.Equal(wantNodes, gotNodes, "nodes mismatch")
		})
	}
}

func collectScopes(e *Engine, a *ast.AST) string {
	seen := map[ScopeID]bool{}
	var scopes []*Scope
	a.Iter(func(nodeID ast.NodeID) bool {
		scope := e.ScopeGraph.NodeScope(nodeID)
		if !seen[scope.ID] {
			seen[scope.ID] = true
			scopes = append(scopes, scope)
		}
		return true
	})
	// Sort by ID for stable output.
	for i := range scopes {
		for j := i + 1; j < len(scopes); j++ {
			if scopes[i].ID > scopes[j].ID {
				scopes[i], scopes[j] = scopes[j], scopes[i]
			}
		}
	}
	var lines []string
	for _, scope := range scopes {
		if scope.Parent != nil {
			lines = append(lines, fmt.Sprintf("%s:%s", scopeLetter(scope.ID), scopeLetter(scope.Parent.ID)))
		} else {
			lines = append(lines, fmt.Sprintf("%s:-", scopeLetter(scope.ID)))
		}
	}
	return strings.Join(lines, "\n")
}

func collectNodes(e *Engine, a *ast.AST) string {
	var nodeIDs []ast.NodeID
	a.Iter(func(nodeID ast.NodeID) bool {
		nodeIDs = append(nodeIDs, nodeID)
		return true
	})
	// Sort by ID for stable output.
	for i := range nodeIDs {
		for j := i + 1; j < len(nodeIDs); j++ {
			if nodeIDs[i] > nodeIDs[j] {
				nodeIDs[i], nodeIDs[j] = nodeIDs[j], nodeIDs[i]
			}
		}
	}
	lines := make([]string, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		scope := e.ScopeGraph.NodeScope(nodeID)
		lines = append(lines, fmt.Sprintf("%s:%s", a.Debug(nodeID, false, 0), scopeLetter(scope.ID)))
	}
	return strings.Join(lines, "\n")
}

func scopeLetter(id ScopeID) string {
	return string('a' + rune(id))
}

func parseSnapshot(s string) string {
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func zeroIDAndSpan(typ *Type, status TypeStatus) bool {
	if _, ok := typ.Kind.(BuiltInType); ok {
		return true
	}
	typ.ID = TypeID(0)
	typ.Span = base.Span{}
	return true
}

func ref_t(typ *Type) *Type {
	return &Type{Kind: RefType{typ.ID, false}}
}

func ref_mut_t(typ *Type) *Type {
	return &Type{Kind: RefType{typ.ID, true}}
}

func struct_t(name string, fields ...any) *Type {
	structFields := []StructField{}
	for i, f := range fields {
		if i%2 == 0 {
			name := base.Cast[string](f)
			structFields = append(structFields, StructField{name, 0, false})
		} else {
			structFields[len(structFields)-1].Type = base.Cast[*Type](f).ID
		}
	}
	return &Type{Kind: StructType{name, structFields}}
}

func arr_t(typ *Type, size int) *Type {
	return &Type{Kind: ArrayType{typ.ID, int64(size)}}
}

func fun_t(types ...*Type) *Type {
	if len(types) == 0 {
		panic("fun_t requires at least a return type")
	}
	ret := types[len(types)-1]
	params := types[:len(types)-1]
	paramIDs := make([]TypeID, len(params))
	for i, p := range params {
		paramIDs[i] = p.ID
	}
	return &Type{Kind: FunType{paramIDs, ret.ID}}
}
