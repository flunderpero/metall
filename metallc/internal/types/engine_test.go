//nolint:exhaustruct
package types

import (
	"fmt"
	"math/big"
	"slices"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestTypeCheckAndLifetimeOK(t *testing.T) {
	// TypeIDs for builtin types are stable, so we can do this.
	// Registration order: void, Str, Bool, Arena, I8, I16, I32, Int, U8, U16, U32, U64.
	span := base.NewSpan(base.NewSource("builtin", []rune{}), 0, 0)
	void := &Type{1, 0, span, BuiltInType{"void"}}
	Str := &Type{2, 0, span, BuiltInType{"Str"}}
	Bool := &Type{3, 0, span, BuiltInType{"Bool"}}
	// Arena = 4
	// I8=5, I16=6, I32=7
	Int := &Type{8, 0, span, BuiltInType{"Int"}}
	U8 := &Type{9, 0, span, BuiltInType{"U8"}}
	// U16=10, U32=11, U64=12

	tests := []struct {
		name  string
		src   string
		want  *Type
		check func(*Engine, ast.NodeID, base.Assert)
	}{
		{"int literal", `123`, Int, nil},
		{"str literal", `"hello"`, Str, nil},
		{"block", `{ 123 "hello" }`, Str, nil},
		{"empty block is void", `{ }`, void, nil},
		{"let binding", `let x = 123`, void, func(e *Engine, _ ast.NodeID, assert base.Assert) {
			// Make sure the binding type is set correctly.
			b, _, ok := e.Scope().Lookup("x")
			assert.Equal(true, ok)
			bindingType := e.Type(b.TypeID)
			assert.Equal(Int, bindingType)
			assert.Equal(false, b.Mut)
		}},
		{"mut binding", `mut x = 123`, void, func(e *Engine, _ ast.NodeID, assert base.Assert) {
			// Make sure the binding type is set correctly.
			b, _, ok := e.Scope().Lookup("x")
			assert.Equal(true, ok)
			bindingType := e.Type(b.TypeID)
			assert.Equal(Int, bindingType)
			assert.Equal(true, b.Mut)
		}},
		{
			"assign is void",
			`{ mut x = 321 x = 123 }`,
			void,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block, ok := e.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				assignID := block.Exprs[1]
				typ := e.TypeOfNode(assignID)
				assert.Equal(void, typ)
			},
		},
		{"fun declaration", `fun foo(a Int, b Str) Int { 123 }`, fun_t("foo", Int, Str, Int), nil},
		{"fun void return coerces body to void", `fun foo() void { 123 }`, fun_t("foo", void), nil},
		{"fun params", `fun foo(a Int) Int { a }`, fun_t("foo", Int, Int), nil},
		{
			"fun params are scoped to the fun",
			`{ fun foo(a Int) void {} fun bar(a Int) void {} }`,
			fun_t("bar", Int, void),
			nil,
		},
		{"fun call", `{ fun foo(a Int) Int { 123 } foo(321) }`, Int, nil},
		{"call void fun", `{ fun foo() void { } foo() }`, void, nil},
		{"builtin print_str", `print_str("hello")`, void, nil},
		{"builtin print_int", `print_int(123)`, void, nil},
		{"builtin print_bool", `print_bool(true)`, void, nil},
		{"shadowing", `{ let x = { let x = "hello" print_str(x) 123 } print_int(x) }`, void, nil},

		{"struct declaration", `struct Foo { one Str two Int }`, struct_t("Foo", "one", Str, "two", Int), nil},
		{
			"forward declare struct type", `{ fun foo(a Foo) void {} struct Foo { one Str } }`,
			struct_t("Foo", "one", Str),
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block, ok := e.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				funID := block.Exprs[0]
				typ, ok := e.TypeOfNode(funID).Kind.(FunType)
				assert.Equal(true, ok, e.TypeOfNode(funID).ID)
				assert.Equal("struct Foo(one Str)", e.TypeDisplay(typ.Params[0]))
			},
		},
		{
			"struct literal", `{ struct Foo { one Str two Int } let x = Foo("hello", 123) x }`,
			struct_t("Foo", "one", Str, "two", Int), nil,
		},
		{
			"struct ref",
			`{ struct Foo { one Str } let x = Foo("hello") &x }`,
			// Our test strategy does not work for nested types (we zero out all type ids).
			// That's why we verify in the check function.
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				got := e.TypeOfNode(id)
				ref, ok := got.Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal("struct Foo(one Str)", e.TypeDisplay(ref.Type))
			},
		},
		{"field read access", `{ struct Foo { one Str } let x = Foo("hello") x.one }`, Str, nil},
		{
			"field write access",
			`{ struct Foo { mut one Str } mut x = Foo("hello") x.one = "bye" }`,
			void,
			nil,
		},
		{
			"field write through mut ref param",
			`{ struct Foo { mut one Str } fun foo(a &mut Foo) void { a.one = "X" } mut x = Foo("hello") foo(&mut x) }`,
			void,
			nil,
		},
		{
			"nested field write on mut struct",
			`{ struct Foo { mut one Int } struct Bar { mut one Foo } mut x = Bar(Foo(1)) x.one.one = 2 }`,
			void,
			nil,
		},
		{
			"field write through let binding of mut ref",
			`{ struct Foo { mut one Str } mut x = Foo("hello") let y = &mut x y.one = "X" }`,
			void,
			nil,
		},

		{"bool true", "{ true }", Bool, nil},
		{"bool false", "{ false }", Bool, nil},
		{"if then else", `{ let x = true if x { 42 } else { 123 }}`, Int, nil},
		{"if without else", `{ let x = true if x { 42 } }`, void, nil},

		{"ref type", `{ let x = 5 let y = &x y }`, ref_t(Int), nil},
		{"mut ref type", `{ mut x = 5 let y = &mut x y }`, ref_mut_t(Int), nil},
		// &x on a let binding produces &Int, even if y is mut.
		{"mut binding of immutable ref", `{ let x = 5 mut y = &x y }`, ref_t(Int), nil},
		// &x on a mut binding still produces &Int (not &mut), since we didn't write &mut.
		{"immutable ref to mut", `{ mut x = 5 mut y = &x y }`, ref_t(Int), nil},
		{"deref", `{ let x = 5 let y = &x y.* }`, Int, nil},
		{
			"deref field access",
			`{ struct Foo{ one Str } let x = Foo("hello") let y = &x x.one }`,
			Str,
			nil,
		},
		{"deref assign", `{ mut x = 1 mut y = &mut x y.* = 321 }`, void, nil},
		{"nested deref assign", `{ mut x = 1 mut y = &mut x mut z = &mut y y.* = 123 z.*.* = 321 }`, void, nil},
		{"mut ref parameter", `{ fun foo(a &mut Int) void { a.* = 321 } mut x = 123 foo(&mut x) }`, void, nil},
		// &mut coerces to & when passed to a & param.
		{"&mut coerces to &ref in call", `{ fun foo(a &Int) void {} mut x = 123 foo(&x) }`, void, nil},
		// Same coercion but in a struct literal constructor.
		{"&mut coerces to &ref in struct literal", `{ struct Foo { one &Int } mut x = 1 let y = Foo(&x) }`, void, nil},
		{"fun returns ref", `{ fun foo(a &Int) &Int { a } let x = 123 foo(&x) }`, ref_t(Int), nil},
		{
			"deref assign through &mut struct field",
			`{ struct Foo { one &mut Int } mut x = 1 let y = Foo(&mut x) y.one.* = 42 }`,
			void,
			nil,
		},
		{
			"reassign mut field of mut ref type",
			`{ struct Foo { mut one &mut Int } mut x = 1 mut y = 2 mut z = Foo(&mut x) z.one = &mut y z.one.* = 99 }`,
			void,
			nil,
		},

		{"forward declaration call", `{ foo() fun foo() void { } }`, fun_t("foo", void), nil},
		{"self recursion", `{ fun foo(a Int) Int { foo(a) } foo(1) }`, Int, nil},
		{"mutual recursion", `{ fun foo(a Int) Int { bar(a) } fun bar(a Int) Int { foo(a) } foo(10) }`, Int, nil},

		{"allocator var", `let @myalloc = Arena()`, void, func(e *Engine, id ast.NodeID, assert base.Assert) {
			scope := e.ScopeGraph.NodeScope(id)
			b, _, ok := scope.Lookup("@myalloc")
			assert.Equal(true, ok)
			typ, ok := e.Type(b.TypeID).Kind.(AllocatorType)
			assert.Equal(true, ok)
			assert.Equal(AllocatorArena, typ.Impl)
		}},
		{
			"heap alloc struct", `{ let @myalloc = Arena() struct Foo{one Str} let x = new(@myalloc, Foo("hello")) x }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block, ok := e.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal(false, ref.Mut)
				_, ok = e.Type(ref.Type).Kind.(StructType)
				assert.Equal(true, ok)
				assert.Equal("&struct Foo(one Str)", e.TypeDisplay(e.TypeOfNode(lastExpr).ID))
			},
		},
		{"pass alloc to fun", `{ fun foo(@myalloc Arena) void {} let @myalloc = Arena() foo(@myalloc) }`, void, nil},
		{
			"heap alloc mut struct",
			`{ let @a = Arena() struct Bar{one Str} new_mut(@a, Bar("hello")) }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.Node(id).Kind)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal(true, ref.Mut)
				_, ok = e.Type(ref.Type).Kind.(StructType)
				assert.Equal(true, ok)
				assert.Equal("&mut struct Bar(one Str)", e.TypeDisplay(e.TypeOfNode(lastExpr).ID))
			},
		},
		{
			"heap alloc mut array",
			`{ let @a = Arena() new_mut(@a, [5]Int()) }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.Node(id).Kind)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal(true, ref.Mut)
				arr, ok := e.Type(ref.Type).Kind.(ArrayType)
				assert.Equal(true, ok)
				assert.Equal(int64(5), arr.Len)
			},
		},

		{"array type", `fun foo(a [5]Int) void {}`, nil, func(e *Engine, id ast.NodeID, assert base.Assert) {
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
		{
			"struct with allocator field",
			`{ struct Foo { @myalloc Arena } let @myalloc = Arena() let x = Foo(@myalloc) }`, void,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.Node(id).Kind)
				// The `let x = Foo(@myalloc)` is the last expr; its type is void.
				// Inspect the Foo struct literal inside the var.
				varNode := base.Cast[ast.Var](e.Node(block.Exprs[len(block.Exprs)-1]).Kind)
				st, ok := e.TypeOfNode(varNode.Expr).Kind.(StructType)
				assert.Equal(true, ok)
				assert.Equal(1, len(st.Fields))
				assert.Equal("@myalloc", st.Fields[0].Name)
				_, ok = e.Type(st.Fields[0].Type).Kind.(AllocatorType)
				assert.Equal(true, ok)
			},
		},
		{
			"heap alloc from struct field",
			`{ struct Foo{one Str} struct Bar { @myalloc Arena } let @myalloc = Arena() let x = Bar(@myalloc) let y = new(x.@myalloc, Foo("hello")) }`,
			void, nil,
		},
		{
			"heap alloc array",
			`{ let @myalloc = Arena() new(@myalloc, [5]Int()) }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.Node(id).Kind)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal(false, ref.Mut)
				arr, ok := e.Type(ref.Type).Kind.(ArrayType)
				assert.Equal(true, ok)
				assert.Equal(int64(5), arr.Len)
			},
		},
		{
			"new array default", `{ let @myalloc = Arena() new(@myalloc, [5]Int(42)) }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.Node(id).Kind)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				arr, ok := e.Type(ref.Type).Kind.(ArrayType)
				assert.Equal(true, ok)
				assert.Equal(int64(5), arr.Len)
			},
		},
		{"make slice", `{ let @myalloc = Arena() make(@myalloc, []Int(5)) }`, slice_t(Int), nil},
		{"make slice default", `{ let @myalloc = Arena() make(@myalloc, []Int(5, 42)) }`, slice_t(Int), nil},
		// Int is safe uninitialized — no default required.
		{"new array Int no default", `{ let @a = Arena() let x = new(@a, [5]Int()) }`, void, nil},
		{"make slice Int no default", `{ let @a = Arena() let x = make(@a, []Int(5)) }`, void, nil},
		// Struct with only Int fields is safe uninitialized.
		{
			"new array safe struct no default",
			`{ struct Foo{one Int two Int} let @a = Arena() let x = new(@a, [3]Foo()) }`,
			void,
			nil,
		},
		// Bool is unsafe, but providing a default makes it OK.
		{"new array Bool with default", `{ let @a = Arena() let x = new(@a, [5]Bool(false)) }`, void, nil},
		{"make slice Bool with default", `{ let @a = Arena() let x = make(@a, []Bool(5, false)) }`, void, nil},
		{"slice index read", `{ let @myalloc = Arena() let x = make(@myalloc, []Int(3)) x[1] }`, Int, nil},
		{"slice index write", `{ let @myalloc = Arena() mut x = make(@myalloc, []Int(3)) x[1] = 5 }`, void, nil},
		{"slice len", `{ let @myalloc = Arena() let x = make(@myalloc, []Int(3)) x.len }`, Int, nil},
		{
			"slice as fun param",
			`{ let @a = Arena() fun foo(s []Int) Int { s[0] } let x = make(@a, []Int(3)) foo(x) }`,
			Int, nil,
		},
		{
			"slice as fun param and return",
			`{ let @a = Arena() fun foo(s []Int) []Int { s } let x = make(@a, []Int(3)) foo(x) }`,
			slice_t(Int), nil,
		},
		{
			"struct with slice field",
			`{ let @a = Arena() struct Foo { one []Int } let s = make(@a, []Int(3)) let x = Foo(s) x.one[0] }`,
			Int, nil,
		},
		{
			"ref to slice",
			`{ let @a = Arena() let x = make(@a, []Int(3)) &x }`,
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				got := e.TypeOfNode(id)
				ref, ok := got.Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal("[]Int", e.TypeDisplay(ref.Type))
			},
		},
		{
			"slice index through ref",
			`{ let @a = Arena() let x = make(@a, []Int(3)) let y = &x y[0] }`,
			Int, nil,
		},
		{
			"slice len through ref",
			`{ let @a = Arena() let x = make(@a, []Int(3)) let y = &x y.len }`,
			Int, nil,
		},
		{
			"mut ref slice index write",
			`{ let @a = Arena() mut x = make(@a, []Int(3)) let y = &mut x y[0] = 42 }`,
			void, nil,
		},
		{"array literal", `[1, 2, 3]`, arr_t(Int, 3), nil},
		{"index read", `{ let x = [1, 2, 3] x[1] }`, Int, nil},
		{"index write", `{ mut x = [1, 2, 3] x[1] = 5 }`, void, nil},

		{"int +", `1 + 2`, Int, nil},
		{"int -", `1 - 2`, Int, nil},
		{"int *", `1 * 2`, Int, nil},
		{"int /", `1 / 2`, Int, nil},

		{"== on int", `1 == 2`, Bool, nil},
		{"!= on int", `1 != 2`, Bool, nil},
		{"== on bool", `true == true`, Bool, nil},
		{"!= on bool", `true != true`, Bool, nil},

		{"and, not, or", `true and false or not true`, Bool, nil},

		{"type constructor", `U8(42)`, U8, nil},
		// Int literal materializes as the target type via type hint in various contexts.
		{"int materialization binary", `U8(1) + 2`, U8, nil},
		{"int materialization call arg", `{ fun foo(a U8) U8 { a } foo(42) }`, U8, nil},
		{"int materialization array literal", `[U8(1), 2, 3]`, arr_t(U8, 3), nil},
		{"int materialization struct literal", `{ struct Foo { one U8 two U8 } let x = Foo(1, 2) }`, void, nil},

		{"conditional for loop", `for true { 1 }`, void, nil},
		{"unconditional for loop", `for { 1 }`, void, nil},
		{"for body must be scoped", `{ let a = 1 for { let a = "hello" }}`, void, nil},

		// Method syntax.
		{
			"method call basic",
			`{ struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) x.get() }`,
			Int, nil,
		},
		{
			"method call with args",
			`{ struct Foo { mut one Int } fun Foo.add(f Foo, n Int) Int { f.one + n } let x = Foo(10) x.add(5) }`,
			Int, nil,
		},
		{
			"method call on &ref receiver",
			`{ struct Foo { one Int } fun Foo.get(f &Foo) Int { f.one } let x = Foo(42) let y = &x y.get() }`,
			Int, nil,
		},
		{
			"method fun declaration type",
			`{ struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } }`,
			// The result of the block is the method declaration, which has FunType.
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.Node(id).Kind)
				funID := block.Exprs[1]
				typ, ok := e.TypeOfNode(funID).Kind.(FunType)
				assert.Equal(true, ok)
				// Method has one param (Foo) and returns Int.
				assert.Equal(1, len(typ.Params))
			},
		},
		// Direct qualified call: Foo.get(x) instead of x.get().
		{
			"direct qualified call",
			`{ struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) Foo.get(x) }`,
			Int, nil,
		},
		{
			"direct qualified call with extra args",
			`{ struct Foo { one Int } fun Foo.add(f Foo, n Int) Int { f.one + n } let x = Foo(10) Foo.add(x, 5) }`,
			Int, nil,
		},
		// Method syntax on built-in types.
		{
			"method call on builtin type",
			`{ fun Int.double(self Int) Int { self + self } let x = 21 x.double() }`,
			Int, nil,
		},
		{
			"direct qualified call on builtin type",
			`{ fun Int.double(self Int) Int { self + self } Int.double(21) }`,
			Int, nil,
		},
	}

	// We need a little hack here, because the "ref" and "mut ref" tests
	// violate the lifetime rules, but we still wan to test them in isolation.
	skipLifetimeCheck := []string{
		"ref type",
		"mut binding of immutable ref",
		"mut ref type",
		"immutable ref to mut",
		"struct ref",
		"fun returns ref",
		"heap alloc struct",
		"heap alloc mut struct",
		"heap alloc array",
		"new array default",
		"heap alloc mut array",
		"make slice",
		"make slice default",
		"new array Int no default",
		"make slice Int no default",
		"new array safe struct no default",
		"new array Bool with default",
		"make slice Bool with default",
		"slice index read",
		"slice index write",
		"slice len",
		"slice as fun param",
		"slice as fun param and return",
		"struct with slice field",
		"ref to slice",
		"slice index through ref",
		"slice len through ref",
		"mut ref slice index write",
		"new array U8 no default",
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
			exprID, parseOK := parser.ParseExpr(0)
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
		{"undefined symbol", `let y = x`, []string{
			"test.met:1:9: symbol not defined: x\n" +
				"    let y = x\n" +
				"            ^",
		}},
		// Symbol defined later in the same scope is not visible before its declaration.
		{"undefined symbol forward ref", `{ print_int(x) let x = 123 }`, []string{
			"test.met:1:13: symbol not defined: x\n" +
				`    { print_int(x) let x = 123 }` + "\n" +
				"                ^",
		}},
		{"duplicate var", `{ let x = 123 let x = 321 }`, []string{
			"test.met:1:19: symbol already defined: x\n" +
				`    { let x = 123 let x = 321 }` + "\n" +
				"                      ^",
		}},
		{"duplicate fun", `{ fun foo() void {} fun foo() void {} }`, []string{
			"test.met:1:25: symbol already defined: foo\n" +
				`    { fun foo() void {} fun foo() void {} }` + "\n" +
				"                            ^^^",
		}},
		// Body redeclares a name that's already a parameter.
		{"redeclare param in body", `fun foo(a Int) void { let a = 123 }`, []string{
			"test.met:1:27: symbol already defined: a\n" +
				`    fun foo(a Int) void { let a = 123 }` + "\n" +
				"                              ^",
		}},
		{"fun return mismatch", `fun foo() Str { 123 }`, []string{
			"test.met:1:17: return type mismatch: expected Str, got Int\n" +
				"    fun foo() Str { 123 }\n" +
				"                    ^^^",
		}},
		{"assign type mismatch", `{ mut x = 123 x = "hello" }`, []string{
			"test.met:1:19: type mismatch: expected Int, got Str\n" +
				`    { mut x = 123 x = "hello" }` + "\n" +
				"                      ^^^^^^^",
		}},
		{"assign to let binding", `{ let x = 123 x = 321 }`, []string{
			"test.met:1:15: cannot assign to immutable variable: x\n" +
				`    { let x = 123 x = 321 }` + "\n" +
				"                  ^",
		}},
		{"call wrong arg count", `{ fun foo(a Int) Int { 123 } foo(1, 2, "hello") }`, []string{
			"test.met:1:30: argument count mismatch: expected 1, got 3\n" +
				`    { fun foo(a Int) Int { 123 } foo(1, 2, "hello") }` + "\n" +
				"                                 ^^^^^^^^^^^^^^^^^^",
		}},
		{"call wrong arg type", `{ fun foo(a Int, b Int) Int { 123 } foo("hello", 2) }`, []string{
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
		{"main must not have params", `fun main(a Int, b Str) void { }`, []string{
			"test.met:1:10: main function cannot take arguments\n" +
				`    fun main(a Int, b Str) void { }` + "\n" +
				`             ^^^^^^^^^^^^`,
		}},

		{"field access on non-struct", `123.one`, []string{
			"test.met:1:1: cannot access field on non-struct type: Int\n" +
				`    123.one` + "\n" +
				`    ^^^`,
		}},
		{
			"field access unknown field",
			`{ struct Foo{one Str} let x = Foo("hello") x.three }`,
			[]string{
				"test.met:1:46: unknown field: Foo.three\n" +
					`    { struct Foo{one Str} let x = Foo("hello") x.three }` + "\n" +
					"                                                 ^^^^^",
			},
		},

		{"if condition must be bool", `{ if 123 { } }`, []string{
			"test.met:1:6: if condition must evaluate to a boolean value, got Int\n" +
				`    { if 123 { } }` + "\n" +
				`         ^^^`,
		}},
		{"if branches must match", `{ if true { 123 } else { "hello" } }`, []string{
			"test.met:1:24: if branch type mismatch: expected Int, got Str\n" +
				`    { if true { 123 } else { "hello" } }` + "\n" +
				"                           ^^^^^^^^^^^",
		}},

		{"deref non-reference", `{ let x = 5 x.* }`, []string{
			"test.met:1:13: dereference: expected reference, got Int\n" +
				`    { let x = 5 x.* }` + "\n" +
				`                ^`,
		}},
		{"deref assign through immutable ref", `{ let x = 5 let y = &x y.* = 321 }`, []string{
			"test.met:1:24: cannot assign through dereference: expected mutable reference, got &Int\n" +
				`    { let x = 5 let y = &x y.* = 321 }` + "\n" +
				`                           ^^^`,
		}},
		{"pass value to &ref param", `{ fun foo(a &Int) void {} let x = 123 foo(x) }`, []string{
			"test.met:1:43: type mismatch at argument 1: expected &Int, got Int\n" +
				`    { fun foo(a &Int) void {} let x = 123 foo(x) }` + "\n" +
				`                                              ^`,
		}},
		{"deref assign through immutable ref param", `{ fun foo(a &Int) void { a.* = 123 }}`, []string{
			"test.met:1:26: cannot assign through dereference: expected mutable reference, got &Int\n" +
				`    { fun foo(a &Int) void { a.* = 123 }}` + "\n" +
				`                             ^^^`,
		}},

		{"&mut of let binding", `{ let x = 123 let y = &mut x }`, []string{
			"test.met:1:23: cannot take mutable reference to immutable value\n" +
				`    { let x = 123 let y = &mut x }` + "\n" +
				`                          ^^^^^^`,
		}},
		{
			"field write on let binding",
			`{ struct Foo{mut one Str} let x = Foo("hello") x.one = "bye" }`,
			[]string{
				"test.met:1:48: cannot assign to field of immutable value\n" +
					`    { struct Foo{mut one Str} let x = Foo("hello") x.one = "bye" }` + "\n" +
					"                                                   ^^^^^",
			},
		},
		{
			"nested field write on let binding",
			`{ struct Foo{mut one Int} struct Bar{mut one Foo} let x = Bar(Foo(1)) x.one.one = 2 }`,
			[]string{
				"test.met:1:71: cannot assign to field of immutable value\n" +
					`    { struct Foo{mut one Int} struct Bar{mut one Foo} let x = Bar(Foo(1)) x.one.one = 2 }` + "\n" +
					"                                                                          ^^^^^^^^^",
			},
		},
		{
			// Bar.one is not mut, so x.one.one is not writable even though x is mut.
			"nested field write through non-mut field",
			`{ struct Foo{mut one Int} struct Bar{one Foo} mut x = Bar(Foo(1)) x.one.one = 2 }`,
			[]string{
				"test.met:1:67: cannot assign to field of immutable value\n" +
					`    { struct Foo{mut one Int} struct Bar{one Foo} mut x = Bar(Foo(1)) x.one.one = 2 }` + "\n" +
					"                                                                      ^^^^^^^^^",
			},
		},
		{
			"field write through immutable ref",
			`{ struct Foo{mut one Str} let x = Foo("hello") let y = &x y.one = "X" }`,
			[]string{
				"test.met:1:59: cannot assign to field of immutable value\n" +
					`    { struct Foo{mut one Str} let x = Foo("hello") let y = &x y.one = "X" }` + "\n" +
					"                                                              ^^^^^",
			},
		},
		{
			"field write through immutable ref param", `{ struct Foo{mut one Str} fun foo(a &Foo) void { a.one = "X" } }`, []string{
				"test.met:1:50: cannot assign to field of immutable value\n" +
					`    { struct Foo{mut one Str} fun foo(a &Foo) void { a.one = "X" } }` + "\n" +
					"                                                     ^^^^^",
			},
		},
		{
			// Field is not declared mut, so it can't be written even on a mut binding.
			"field write on non-mut field",
			`{ struct Foo { one Str } mut x = Foo("hi") x.one = "bye" }`,
			[]string{
				"test.met:1:44: cannot assign to immutable field: one\n" +
					`    { struct Foo { one Str } mut x = Foo("hi") x.one = "bye" }` + "\n" +
					"                                               ^^^^^",
			},
		},
		{
			// Field expects &mut Int but we pass &Int.
			"pass &ref where &mut field expected",
			`{ struct Foo { one &mut Int } let x = 123 let y = Foo(&x) }`,
			[]string{
				"test.met:1:55: type mismatch at argument 1: expected &mut Int, got &Int\n" +
					`    { struct Foo { one &mut Int } let x = 123 let y = Foo(&x) }` + "\n" +
					"                                                          ^^",
			},
		},
		{
			"deref assign through &ref field",
			`{ struct Foo { one &Int } let x = 123 let y = Foo(&x) y.one.* = 42 }`,
			[]string{
				"test.met:1:55: cannot assign through dereference: expected mutable reference, got &Int\n" +
					`    { struct Foo { one &Int } let x = 123 let y = Foo(&x) y.one.* = 42 }` + "\n" +
					"                                                          ^^^^^^^",
			},
		},
		{
			// Field type is &mut Int but field is not declared mut, so it can't be reassigned.
			"reassign non-mut &mut field",
			`{ struct Foo { one &mut Int } mut x = 1 mut y = 2 mut z = Foo(&mut x) z.one = &y }`,
			[]string{
				"test.met:1:71: cannot assign to immutable field: one\n" +
					`    { struct Foo { one &mut Int } mut x = 1 mut y = 2 mut z = Foo(&mut x) z.one = &y }` + "\n" +
					"                                                                          ^^^^^",
			},
		},
		{
			// Params are always immutable bindings, so a &mut param can't be reassigned.
			"reassign &mut param",
			`{ fun foo(a &mut Int) void { mut x = 1 a = &x } }`,
			[]string{
				"test.met:1:40: cannot assign to immutable variable: a\n" +
					`    { fun foo(a &mut Int) void { mut x = 1 a = &x } }` + "\n" +
					"                                           ^",
			},
		},
		{"non-existing allocator", `{ struct Foo{one Str} let x = new(@myalloc, Foo("hello")) }`, []string{
			"test.met:1:35: symbol not defined: @myalloc\n" +
				`    { struct Foo{one Str} let x = new(@myalloc, Foo("hello")) }` + "\n" +
				`                                      ^^^^^^^^`,
		}},
		{"index on non-array", `{ let x = 123 x[0] }`, []string{
			"test.met:1:15: not an array or slice: Int\n" +
				"    { let x = 123 x[0] }\n" +
				"                  ^",
		}},
		{"index with non-int", `{ let x = [1, 2, 3] x["hello"] }`, []string{
			"test.met:1:23: index type mismatch: expected Int, got Str\n" +
				`    { let x = [1, 2, 3] x["hello"] }` + "\n" +
				`                          ^^^^^^^`,
		}},

		{"add with non-int", `1 + "hello"`, []string{
			"test.met:1:5: type mismatch: expected type of LHS: Int, got Str\n" +
				`    1 + "hello"` + "\n" +
				"        ^^^^^^^",
		}},
		{"== with invalid type", `"hello" == "world"`, []string{
			"test.met:1:1: type mismatch: binary operation '==' expects an integer or Bool, got Str\n" +
				`    "hello" == "world"` + "\n" +
				"    ^^^^^^^",
		}},
		{"`and` with invalid type", `true and 123`, []string{
			"test.met:1:10: type mismatch: expected type of LHS: Bool, got Int\n" +
				`    true and 123` + "\n" +
				"             ^^^",
		}},
		{"`not` on invalid type", `not 123`, []string{
			"test.met:1:5: type mismatch: expected Bool, got Int\n" +
				`    not 123` + "\n" +
				"        ^^^",
		}},

		{"non-boolean condition in for loop", `for 123 {}`, []string{
			"test.met:1:5: type mismatch: expected Bool, got Int\n" +
				`    for 123 {}` + "\n" +
				"        ^^^",
		}},
		{"break outside loop", `{ break }`, []string{
			"test.met:1:3: break statement outside of loop\n" +
				`    { break }` + "\n" +
				"      ^^^^^",
		}},
		{"continue outside loop", `{ continue }`, []string{
			"test.met:1:3: continue statement outside of loop\n" +
				`    { continue }` + "\n" +
				"      ^^^^^^^^",
		}},
		{"unknown field on slice", `{ let @a = Arena() let x = make(@a, []Int(3)) x.foo }`, []string{
			"test.met:1:49: unknown field on slice: foo\n" +
				`    { let @a = Arena() let x = make(@a, []Int(3)) x.foo }` + "\n" +
				"                                                    ^^^",
		}},
		{"make slice non-int length", `{ let @a = Arena() make(@a, []Int("hello")) }`, []string{
			"test.met:1:35: type mismatch: expected Int, got Str\n" +
				`    { let @a = Arena() make(@a, []Int("hello")) }` + "\n" +
				`                                      ^^^^^^^`,
		}},
		{"new array wrong default type", `{ let @a = Arena() new(@a, [5]Int("hello")) }`, []string{
			"test.met:1:35: type mismatch: expected Int, got Str\n" +
				`    { let @a = Arena() new(@a, [5]Int("hello")) }` + "\n" +
				`                                      ^^^^^^^`,
		}},
		{"make slice wrong default type", `{ let @a = Arena() make(@a, []Int(3, "hello")) }`, []string{
			"test.met:1:38: type mismatch: expected Int, got Str\n" +
				`    { let @a = Arena() make(@a, []Int(3, "hello")) }` + "\n" +
				`                                         ^^^^^^^`,
		}},
		{"new array Bool uninitialized", `{ let @a = Arena() new(@a, [5]Bool()) }`, []string{
			"test.met:1:28: Bool is not safe to leave uninitialized, provide a default value\n" +
				`    { let @a = Arena() new(@a, [5]Bool()) }` + "\n" +
				`                               ^^^^^^^`,
		}},
		{"new array Str uninitialized", `{ let @a = Arena() new(@a, [5]Str()) }`, []string{
			"test.met:1:28: Str is not safe to leave uninitialized, provide a default value\n" +
				`    { let @a = Arena() new(@a, [5]Str()) }` + "\n" +
				`                               ^^^^^^`,
		}},
		{"new array ref uninitialized", `{ struct Foo{one Int} let @a = Arena() new(@a, [5]&Foo()) }`, []string{
			"test.met:1:48: &struct Foo(one Int) is not safe to leave uninitialized, provide a default value\n" +
				`    { struct Foo{one Int} let @a = Arena() new(@a, [5]&Foo()) }` + "\n" +
				`                                                   ^^^^^^^`,
		}},
		{
			"new array struct with ref field uninitialized",
			`{ struct Foo{one &Int} let @a = Arena() new(@a, [5]Foo()) }`,
			[]string{
				"test.met:1:49: struct Foo(one &Int) is not safe to leave uninitialized, provide a default value\n" +
					`    { struct Foo{one &Int} let @a = Arena() new(@a, [5]Foo()) }` + "\n" +
					`                                                    ^^^^^^`,
			},
		},
		{"make slice Bool uninitialized", `{ let @a = Arena() make(@a, []Bool(3)) }`, []string{
			"test.met:1:29: Bool is not safe to leave uninitialized, provide a default value\n" +
				`    { let @a = Arena() make(@a, []Bool(3)) }` + "\n" +
				`                                ^^^^^^`,
		}},
		{"make slice ref uninitialized", `{ struct Foo{one Int} let @a = Arena() make(@a, []&Foo(3)) }`, []string{
			"test.met:1:49: &struct Foo(one Int) is not safe to leave uninitialized, provide a default value\n" +
				`    { struct Foo{one Int} let @a = Arena() make(@a, []&Foo(3)) }` + "\n" +
				`                                                    ^^^^^^`,
		}},
		{"cannot return allocator from fun", `fun foo() Arena { }`, []string{
			"test.met:1:11: cannot return an allocator from a function\n" +
				`    fun foo() Arena { }` + "\n" +
				"              ^^^^^",
		}},

		// Type constructor errors — see TestIntegerTypes for comprehensive coverage.
		{"U8 out of range positive", `U8(256)`, []string{
			"test.met:1:4: value 256 out of range for U8 (0..255)\n" +
				"    U8(256)\n" +
				"       ^^^",
		}},
		{"Bool is not a type constructor", `Bool(true)`, []string{
			"test.met:1:1: not a struct: Bool\n" +
				"    Bool(true)\n" +
				"    ^^^^",
		}},
		{"U8 + Int type mismatch", `{ let x = 123 U8(1) + x }`, []string{
			"test.met:1:23: type mismatch: expected type of LHS: U8, got Int\n" +
				"    { let x = 123 U8(1) + x }\n" +
				"                          ^",
		}},

		// Method syntax errors.
		{
			"method call wrong arg count",
			`{ struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) x.get(1, 2) }`,
			[]string{
				"test.met:1:75: argument count mismatch: expected 0, got 2\n" +
					"    { struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) x.get(1, 2) }\n" +
					"                                                                              ^^^^^^^^^^^",
			},
		},
		{
			"method call wrong arg type",
			`{ struct Foo { one Int } fun Foo.add(f Foo, n Int) Int { f.one + n } let x = Foo(42) x.add("hello") }`,
			[]string{
				"test.met:1:92: type mismatch at argument 1: expected Int, got Str\n" +
					`    { struct Foo { one Int } fun Foo.add(f Foo, n Int) Int { f.one + n } let x = Foo(42) x.add("hello") }` + "\n" +
					`                                                                                               ^^^^^^^`,
			},
		},
		{
			"method on unknown field",
			`{ struct Foo { one Int } let x = Foo(42) x.nope() }`,
			[]string{
				"test.met:1:44: unknown field: Foo.nope\n" +
					"    { struct Foo { one Int } let x = Foo(42) x.nope() }\n" +
					"                                               ^^^^",
			},
		},
		{
			"direct qualified call undefined",
			`{ struct Foo { one Int } Foo.nope(Foo(1)) }`,
			[]string{
				"test.met:1:26: symbol not defined: Foo.nope\n" +
					"    { struct Foo { one Int } Foo.nope(Foo(1)) }\n" +
					"                             ^^^^^^^^",
			},
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
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens)
			exprID, parseOK := parser.ParseExpr(0)
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
			src:  `let x = 1`,
			scopes: `
				a:-
			`,
			nodes: `
				n1:Int(value=1):a
				n2:Var(name="x",mut=false,expr=n1:Int):a
			`,
		},
		{
			name: "block creates scope",
			src:  `{ let x = 1 }`,
			scopes: `
				a:-
				b:a
			`,
			nodes: `
				n1:Int(value=1):b
				n2:Var(name="x",mut=false,expr=n1:Int):b
				n3:Block(createScope=true,exprs=[n2:Var]):a
			`,
		},
		{
			name: "nested blocks",
			src:  `{ let x = 1 { let y = 2 } }`,
			scopes: `
				a:-
				b:a
				c:b
			`,
			nodes: `
				n1:Int(value=1):b
				n2:Var(name="x",mut=false,expr=n1:Int):b
				n3:Int(value=2):c
				n4:Var(name="y",mut=false,expr=n3:Int):c
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
			exprID, parseOK := parser.ParseExpr(0)
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
	typ.NodeID = ast.NodeID(0)
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

func slice_t(typ *Type) *Type {
	return &Type{Kind: SliceType{typ.ID}}
}

func fun_t(name string, types ...*Type) *Type {
	if len(types) == 0 {
		panic("fun_t requires at least a return type")
	}
	ret := types[len(types)-1]
	params := types[:len(types)-1]
	paramIDs := make([]TypeID, len(params))
	for i, p := range params {
		paramIDs[i] = p.ID
	}
	return &Type{Kind: FunType{name, paramIDs, ret.ID}}
}

func TestIntegerTypes(t *testing.T) {
	assert := base.NewAssert(t)
	allIntTypes := []string{"I8", "I16", "I32", "Int", "U8", "U16", "U32", "U64"}

	typeCheck := func(t *testing.T, src string) *Engine {
		t.Helper()
		source := base.NewSource("test.met", []rune(src))
		tokens := token.Lex(source)
		parser := ast.NewParser(tokens)
		exprID, parseOK := parser.ParseExpr(0)
		if !parseOK || len(parser.Diagnostics) > 0 {
			t.Fatalf("parse failed: %s", parser.Diagnostics)
		}
		e := NewEngine(parser.AST)
		e.Query(exprID)
		return e
	}

	t.Run("literal range", func(t *testing.T) {
		// Each type constructor accepts 0 and its max literal value.
		for _, name := range allIntTypes {
			info := intTypeInfos[name]
			for _, val := range []string{"0", info.Max.String()} {
				src := fmt.Sprintf("%s(%s)", name, val)
				e := typeCheck(t, src)
				assert.Equal(0, len(e.Diagnostics), "%s(%s) should be valid: %s", name, val, e.Diagnostics)
			}
		}
		// NOTE: Signed min values (e.g. I8(-128)) can't be expressed as
		// literals because the language has no negative literal syntax and
		// `0 - 128` produces an Int which can't be narrowed to I8.
	})

	t.Run("literal out of range", func(t *testing.T) {
		for _, name := range allIntTypes {
			info := intTypeInfos[name]
			aboveMax := new(big.Int).Add(info.Max, big.NewInt(1))
			src := fmt.Sprintf("%s(%s)", name, aboveMax)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.Diagnostics), "%s(%s) diagnostics: %s", name, aboveMax, e.Diagnostics)
			assert.Contains(e.Diagnostics[0].Display(), "out of range", "%s(%s)", name, aboveMax)
		}
	})

	t.Run("arithmetic", func(t *testing.T) {
		for _, op := range []string{"+", "-", "*", "/"} {
			for _, name := range allIntTypes {
				src := fmt.Sprintf("%[1]s(1) %[2]s %[1]s(1)", name, op)
				e := typeCheck(t, src)
				assert.Equal(0, len(e.Diagnostics), "%s: %s", src, e.Diagnostics)
			}
		}
	})

	t.Run("comparison", func(t *testing.T) {
		for _, op := range []string{"==", "!="} {
			for _, name := range allIntTypes {
				src := fmt.Sprintf("%[1]s(1) %[2]s %[1]s(1)", name, op)
				e := typeCheck(t, src)
				assert.Equal(0, len(e.Diagnostics), "%s: %s", src, e.Diagnostics)
			}
		}
	})

	t.Run("mixed types rejected in binary ops", func(t *testing.T) {
		e := typeCheck(t, `{ let x = I32(1) let y = U8(1) x + y }`)
		assert.Equal(1, len(e.Diagnostics), "diagnostics: %s", e.Diagnostics)
		assert.Contains(e.Diagnostics[0].Display(), "type mismatch")
	})

	t.Run("non-integer rejected", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf(`%s("hello")`, name)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.Diagnostics), "%s(Str) diagnostics: %s", name, e.Diagnostics)
			assert.Contains(e.Diagnostics[0].Display(), "cannot convert", name)
		}
	})

	t.Run("wrong arg count", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf("%s(1, 2)", name)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.Diagnostics), "%s(1,2) diagnostics: %s", name, e.Diagnostics)
			assert.Contains(e.Diagnostics[0].Display(), "takes exactly 1 argument", name)
		}
	})

	t.Run("type conversions", func(t *testing.T) {
		type convTest struct {
			from, to string
			ok       bool
		}
		tests := []convTest{
			// Identity — always ok.
			{"I8", "I8", true},
			{"U8", "U8", true},
			{"Int", "Int", true},
			{"U64", "U64", true},

			// Same signedness, widening — ok.
			{"I8", "I16", true},
			{"I8", "I32", true},
			{"I8", "Int", true},
			{"I16", "I32", true},
			{"I16", "Int", true},
			{"I32", "Int", true},
			{"U8", "U16", true},
			{"U8", "U32", true},
			{"U8", "U64", true},
			{"U16", "U32", true},
			{"U16", "U64", true},
			{"U32", "U64", true},

			// Same signedness, narrowing — rejected.
			{"I16", "I8", false},
			{"I32", "I8", false},
			{"Int", "I8", false},
			{"I32", "I16", false},
			{"Int", "I16", false},
			{"Int", "I32", false},
			{"U16", "U8", false},
			{"U32", "U8", false},
			{"U64", "U8", false},
			{"U32", "U16", false},
			{"U64", "U16", false},
			{"U64", "U32", false},

			// Unsigned → signed, strictly more bits — ok.
			{"U8", "I16", true},
			{"U8", "I32", true},
			{"U8", "Int", true},
			{"U16", "I32", true},
			{"U16", "Int", true},
			{"U32", "Int", true},

			// Unsigned → signed, same or fewer bits — rejected.
			{"U8", "I8", false},
			{"U16", "I16", false},
			{"U16", "I8", false},
			{"U32", "I32", false},
			{"U32", "I16", false},
			{"U32", "I8", false},
			{"U64", "Int", false},
			{"U64", "I32", false},

			// Signed → unsigned — always rejected.
			{"I8", "U8", false},
			{"I8", "U16", false},
			{"I8", "U32", false},
			{"I8", "U64", false},
			{"I16", "U8", false},
			{"I16", "U16", false},
			{"I16", "U32", false},
			{"I16", "U64", false},
			{"I32", "U8", false},
			{"I32", "U16", false},
			{"I32", "U32", false},
			{"I32", "U64", false},
			{"Int", "U8", false},
			{"Int", "U16", false},
			{"Int", "U32", false},
			{"Int", "U64", false},
		}

		for _, tt := range tests {
			name := fmt.Sprintf("%s_to_%s", tt.from, tt.to)
			t.Run(name, func(t *testing.T) {
				src := fmt.Sprintf("{ let x = %s(1) %s(x) }", tt.from, tt.to)
				e := typeCheck(t, src)
				if tt.ok {
					assert.Equal(0, len(e.Diagnostics), "%s(%s) should be allowed: %s", tt.to, tt.from, e.Diagnostics)
				} else {
					assert.NotEqual(0, len(e.Diagnostics), "%s(%s) should be rejected", tt.to, tt.from)
					if len(e.Diagnostics) > 0 {
						assert.Contains(e.Diagnostics[0].Display(), "cannot convert", "%s → %s", tt.from, tt.to)
					}
				}
			})
		}
	})

	t.Run("safe uninitialized", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf("{ let @a = Arena() let x = new(@a, [5]%s()) }", name)
			e := typeCheck(t, src)
			assert.Equal(0, len(e.Diagnostics), "%s should be safe uninitialized: %s", name, e.Diagnostics)
		}
	})
}
