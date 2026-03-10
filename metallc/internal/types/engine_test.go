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
	"github.com/flunderpero/metall/metallc/internal/modules"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestTypeCheckAndLifetimeOK(t *testing.T) {
	void := &Type{1, 0, base.Span{}, VoidType{}}
	Bool := &Type{3, 0, base.Span{}, BoolType{}}
	Int := &Type{7, 0, base.Span{}, lookupIntType("Int")}
	U8 := &Type{8, 0, base.Span{}, lookupIntType("U8")}
	Str := &Type{12, 0, base.Span{}, StructType{
		Name:   "Str",
		Fields: []StructField{{Name: "data", Type: TypeID(71), Mut: false}},
	}}

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
		{"let binding", `let x = 123`, void, func(e *Engine, id ast.NodeID, assert base.Assert) {
			// Make sure the binding type is set correctly.
			b, ok := e.lookup(id, "x")
			assert.Equal(true, ok)
			bindingType := e.env.Type(b.TypeID)
			assert.Equal(Int, bindingType)
			assert.Equal(false, b.Mut)
		}},
		{"mut binding", `mut x = 123`, void, func(e *Engine, id ast.NodeID, assert base.Assert) {
			// Make sure the binding type is set correctly.
			b, ok := e.lookup(id, "x")
			assert.Equal(true, ok)
			bindingType := e.env.Type(b.TypeID)
			assert.Equal(Int, bindingType)
			assert.Equal(true, b.Mut)
		}},
		{
			"assign is void",
			`{ mut x = 321 x = 123 }`,
			void,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block, ok := e.ast.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				assignID := block.Exprs[1]
				typ := e.env.TypeOfNode(assignID)
				assert.Equal(void, typ)
			},
		},
		{"fun declaration", `fun foo(a Int, b Str) Int { 123 }`, fun_t(Int, Str, Int), nil},
		{"fun void return coerces body to void", `fun foo() void { 123 }`, fun_t(void), nil},
		{"fun params", `fun foo(a Int) Int { a }`, fun_t(Int, Int), nil},
		{
			"fun params are scoped to the fun",
			`{ fun foo(a Int) void {} fun bar(a Int) void {} }`,
			fun_t(Int, void),
			nil,
		},
		{"fun call", `{ fun foo(a Int) Int { 123 } foo(321) }`, Int, nil},
		{"call void fun", `{ fun foo() void { } foo() }`, void, nil},
		{"builtin print_str", `print_str("hello")`, void, nil},
		{"builtin print_int", `print_int(123)`, void, nil},
		{"builtin print_bool", `print_bool(true)`, void, nil},
		{"shadowing", `{ let x = { let x = "hello" print_str(x) 123 } print_int(x) }`, void, nil},

		{"return", `fun foo() Int { return 1 }`, fun_t(Int), nil},
		{"return void", `fun foo() void { return void }`, fun_t(void), nil},
		{
			"return expr type is void",
			`fun foo() Int { return 123 }`,
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				fun, ok := e.ast.Node(id).Kind.(ast.Fun)
				assert.Equal(true, ok)
				block, ok := e.ast.Node(fun.Block).Kind.(ast.Block)
				assert.Equal(true, ok)
				_, ok = e.ast.Node(block.Exprs[0]).Kind.(ast.Return)
				assert.Equal(true, ok)
				retTyp := e.env.TypeOfNode(block.Exprs[0])
				assert.Equal(void, retTyp)
			},
		},

		{"fun type", `fun foo(bar fun(Int) Str) void {}`, nil, func(e *Engine, id ast.NodeID, assert base.Assert) {
			outer := base.Cast[FunType](e.env.TypeOfNode(id).Kind)
			inner := base.Cast[FunType](e.env.Type(outer.Params[0]).Kind)
			assert.Equal(Int.ID, inner.Params[0])
			assert.Equal(Str.ID, inner.Return)
		}},
		{
			"fun type identity",
			`{
				fun foo(a Int) Str { "x" }
				fun bar(a Int) Str { "y" }
				fun apply(f fun(Int) Str, g fun(Int) Str, h fun(fun(Int) Str) Bool) void {}
			}`,
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.ast.Node(id).Kind)
				fooTyp := e.env.TypeOfNode(block.Exprs[0])
				barTyp := e.env.TypeOfNode(block.Exprs[1])
				applyFT := base.Cast[FunType](e.env.TypeOfNode(block.Exprs[2]).Kind)
				fTyp := e.env.Type(applyFT.Params[0])
				gTyp := e.env.Type(applyFT.Params[1])
				assert.Equal(fTyp, gTyp)
				h := base.Cast[FunType](e.env.Type(applyFT.Params[2]).Kind)
				hParamTyp := e.env.Type(h.Params[0])
				assert.Equal(fTyp, hParamTyp)
				assert.Equal(fooTyp, barTyp)
				assert.Equal(fooTyp, fTyp)
			},
		},
		{"named fun assignable to fun-type", `
			{
				fun foo(a Int) Int { a }
				fun bar(f fun(Int) Int) Int { f(0) }
				
				bar(foo)
			}`, Int, nil},
		{
			"if branches with different named funs", `
				{
					fun double(a Int) Int { a + a }
					fun triple(a Int) Int { a + a + a }
					if true { double } else { triple }
				}
			`, fun_t(Int, Int), nil,
		},

		{"struct declaration", `struct Foo { one Str two Int }`, struct_t("Foo", "one", Str, "two", Int), nil},
		{
			"forward declare struct type", `{ fun foo(a Foo) void {} struct Foo { one Str } }`,
			struct_t("Foo", "one", Str),
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block, ok := e.ast.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				funID := block.Exprs[0]
				typ, ok := e.env.TypeOfNode(funID).Kind.(FunType)
				assert.Equal(true, ok, e.env.TypeOfNode(funID).ID)
				paramTyp := e.env.Type(typ.Params[0])
				foo := base.Cast[StructType](paramTyp.Kind)
				assert.Equal("Foo", foo.Name)
				assert.Equal(1, len(foo.Fields))
				assert.Equal("one", foo.Fields[0].Name)
				assert.Equal(Str.ID, foo.Fields[0].Type)
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
				got := e.env.TypeOfNode(id)
				ref, ok := got.Kind.(RefType)
				assert.Equal(true, ok)
				inner := e.env.Type(ref.Type)
				foo := base.Cast[StructType](inner.Kind)
				assert.Equal("Foo", foo.Name)
				assert.Equal(1, len(foo.Fields))
				assert.Equal("one", foo.Fields[0].Name)
				assert.Equal(Str.ID, foo.Fields[0].Type)
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
		{
			"if with one branch return",
			`fun foo() Int { if true { return 123 } else { "hello" } 321 }`,
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				fun := base.Cast[ast.Fun](e.ast.Node(id).Kind)
				block := base.Cast[ast.Block](e.ast.Node(fun.Block).Kind)
				if_ := base.Cast[ast.If](e.ast.Node(block.Exprs[0]).Kind)
				assert.Equal(void, e.env.TypeOfNode(if_.Then))
				assert.Equal(Str, e.env.TypeOfNode(*if_.Else))
				assert.Equal(void, e.env.TypeOfNode(block.Exprs[0]), "if expr should be void")
			},
		},
		{
			"if with one branch break",
			`fun foo() void { for { if true { break } else { "hello" } } }`,
			fun_t(void),
			nil,
		},
		{
			"if with one branch continue",
			`fun foo() void { for { if true { continue } else { "hello" } } }`,
			fun_t(void),
			nil,
		},
		{
			"if with both branches return",
			`fun foo() Int { if true { return 1 } else { return 2 } }`,
			fun_t(Int),
			nil,
		},
		{
			"nested if with all branches return",
			`fun foo() Int { if true { if false { return 1 } else { return 2 } } else { return 3 } }`,
			fun_t(Int),
			nil,
		},
		// Detect that a nested if/else expr is detected as "breaking control flow" and this making
		// the whole if expression 'void'.
		{
			"nested return breaks outer 'if control flow'",
			`fun foo(a Int) Int { if true { if a == 0 { return 1 } else { return 2 } } else { "hello" } 321 }`,
			fun_t(Int, Int),
			nil,
		},

		{"ref type", `{ let x = 5 let y = &x y }`, ref_t(Int), nil},
		{"mut ref type", `{ mut x = 5 let y = &mut x y }`, ref_mut_t(Int), nil},
		// &x on a let binding produces &Int, even if y is mut.
		{"mut binding of immutable ref", `{ let x = 5 mut y = &x y }`, ref_t(Int), nil},
		// &x on a mut binding still produces &Int (not &mut), since we didn't write &mut.
		{"immutable ref to mut", `{ mut x = 5 mut y = &x y }`, ref_t(Int), nil},
		{"ref of field", `{ struct Foo { one Int } let x = Foo(42) let y = &x.one y }`, ref_t(Int), nil},
		{
			"ref of nested field",
			`{ struct Bar { one Int } struct Foo { bar Bar } let x = Foo(Bar(1)) let y = &x.bar.one y }`,
			ref_t(Int), nil,
		},
		{"ref of deref", `{ mut x = 5 let y = &mut x let z = &y.* z }`, ref_t(Int), nil},
		{
			"mut ref of mut field",
			`{ struct Foo { mut one Int } mut x = Foo(42) let y = &mut x.one y }`,
			ref_mut_t(Int), nil,
		},
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

		{"forward declaration call", `{ foo() fun foo() void { } }`, fun_t(void), nil},
		{"self recursion", `{ fun foo(a Int) Int { foo(a) } foo(1) }`, Int, nil},
		{"mutual recursion", `{ fun foo(a Int) Int { bar(a) } fun bar(a Int) Int { foo(a) } foo(10) }`, Int, nil},

		{"allocator var", `let @myalloc = Arena()`, void, func(e *Engine, id ast.NodeID, assert base.Assert) {
			b, ok := e.lookup(id, "@myalloc")
			assert.Equal(true, ok)
			typ, ok := e.env.Type(b.TypeID).Kind.(AllocatorType)
			assert.Equal(true, ok)
			assert.Equal(AllocatorArena, typ.Impl)
		}},
		{
			"heap alloc struct", `{ let @myalloc = Arena() struct Foo{one Str} let x = new(@myalloc, Foo("hello")) x }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block, ok := e.ast.Node(id).Kind.(ast.Block)
				assert.Equal(true, ok)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.env.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal(false, ref.Mut)
				foo := base.Cast[StructType](e.env.Type(ref.Type).Kind)
				assert.Equal("Foo", foo.Name)
				assert.Equal(1, len(foo.Fields))
				assert.Equal("one", foo.Fields[0].Name)
				assert.Equal(Str.ID, foo.Fields[0].Type)
			},
		},
		{"pass alloc to fun", `{ fun foo(@myalloc Arena) void {} let @myalloc = Arena() foo(@myalloc) }`, void, nil},
		{
			"heap alloc mut struct",
			`{ let @a = Arena() struct Bar{one Str} new_mut(@a, Bar("hello")) }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.ast.Node(id).Kind)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.env.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal(true, ref.Mut)
				bar := base.Cast[StructType](e.env.Type(ref.Type).Kind)
				assert.Equal("Bar", bar.Name)
				assert.Equal(1, len(bar.Fields))
				assert.Equal("one", bar.Fields[0].Name)
				assert.Equal(Str.ID, bar.Fields[0].Type)
			},
		},
		{
			"heap alloc mut array",
			`{ let @a = Arena() new_mut(@a, [5]Int()) }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.ast.Node(id).Kind)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.env.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal(true, ref.Mut)
				arr, ok := e.env.Type(ref.Type).Kind.(ArrayType)
				assert.Equal(true, ok)
				assert.Equal(int64(5), arr.Len)
			},
		},

		{"array type", `fun foo(a [5]Int) void {}`, nil, func(e *Engine, id ast.NodeID, assert base.Assert) {
			fun, ok := e.env.TypeOfNode(id).Kind.(FunType)
			assert.Equal(true, ok)
			assert.Equal(1, len(fun.Params))
			arr, ok := e.env.Type(fun.Params[0]).Kind.(ArrayType)
			assert.Equal(true, ok)
			assert.Equal(int64(5), arr.Len)
			assert.Equal(Int.ID, arr.Elem)
		}},
		{
			"array type ids are stable",
			`fun foo(a [5]Int, b [5]Int, c [6]Int) void { [1, 2, 3, 4, 5]}`,
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				funNode, ok := e.ast.Node(id).Kind.(ast.Fun)
				assert.Equal(ok, true)
				fun, ok := e.env.TypeOfNode(id).Kind.(FunType)
				assert.Equal(true, ok)
				assert.Equal(3, len(fun.Params))
				for _, elem := range fun.Params {
					_, ok := e.env.Type(elem).Kind.(ArrayType)
					assert.Equal(true, ok)
				}
				assert.Equal(fun.Params[0], fun.Params[1])
				assert.NotEqual(fun.Params[0], fun.Params[2])
				// The array literal in the body should have the same type as param 0.
				block, ok := e.ast.Node(funNode.Block).Kind.(ast.Block)
				assert.Equal(true, ok)
				literalTyp := e.env.TypeOfNode(block.Exprs[0])
				paramTyp := e.env.Type(fun.Params[0])
				assert.Equal(paramTyp, literalTyp)
			},
		},
		{
			"struct with allocator field",
			`{ struct Foo { @myalloc Arena } let @myalloc = Arena() let x = Foo(@myalloc) }`, void,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.ast.Node(id).Kind)
				// The `let x = Foo(@myalloc)` is the last expr; its type is void.
				// Inspect the Foo struct literal inside the var.
				varNode := base.Cast[ast.Var](e.ast.Node(block.Exprs[len(block.Exprs)-1]).Kind)
				st, ok := e.env.TypeOfNode(varNode.Expr).Kind.(StructType)
				assert.Equal(true, ok)
				assert.Equal(1, len(st.Fields))
				assert.Equal("@myalloc", st.Fields[0].Name)
				_, ok = e.env.Type(st.Fields[0].Type).Kind.(AllocatorType)
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
				block := base.Cast[ast.Block](e.ast.Node(id).Kind)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.env.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal(false, ref.Mut)
				arr, ok := e.env.Type(ref.Type).Kind.(ArrayType)
				assert.Equal(true, ok)
				assert.Equal(int64(5), arr.Len)
			},
		},
		{
			"new array default", `{ let @myalloc = Arena() new(@myalloc, [5]Int(42)) }`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.ast.Node(id).Kind)
				lastExpr := block.Exprs[len(block.Exprs)-1]
				ref, ok := e.env.TypeOfNode(lastExpr).Kind.(RefType)
				assert.Equal(true, ok)
				arr, ok := e.env.Type(ref.Type).Kind.(ArrayType)
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
		{"slice index write", `{ let @myalloc = Arena() let x = make(@myalloc, []mut Int(3)) x[1] = 5 }`, void, nil},
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
				got := e.env.TypeOfNode(id)
				ref, ok := got.Kind.(RefType)
				assert.Equal(true, ok)
				assert.Equal("[]Int", e.env.TypeDisplay(ref.Type))
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
			`{ let @a = Arena() mut x = make(@a, []mut Int(3)) let y = &mut x y[0] = 42 }`,
			void, nil,
		},
		{"make mut slice", `{ let @a = Arena() make(@a, []mut Int(5)) }`, mut_slice_t(Int), nil},
		{
			"mut slice assignable to immutable",
			`{ let @a = Arena() fun foo(s []Int) Int { s[0] } let x = make(@a, []mut Int(3)) foo(x) }`,
			Int, nil,
		},
		{
			"mut slice index write no mut binding",
			`{ let @a = Arena() let x = make(@a, []mut Int(3)) x[0] = 5 }`,
			void, nil,
		},
		{
			"subslice mut array",
			`{ mut x = [1, 2, 3] x[0..2] }`,
			mut_slice_t(Int), nil,
		},
		{
			"subslice mut slice",
			`{ let @a = Arena() let x = make(@a, []mut Int(5)) x[1..3] }`,
			mut_slice_t(Int), nil,
		},
		{
			"subslice mut slice through mut ref",
			`{ let @a = Arena() mut x = make(@a, []mut Int(5)) let y = &mut x y[1..3] }`,
			mut_slice_t(Int), nil,
		},
		{
			"subslice mut slice through immutable ref",
			`{ let @a = Arena() let x = make(@a, []mut Int(5)) let y = &x y[1..3] }`,
			slice_t(Int), nil,
		},
		{"array literal", `[1, 2, 3]`, arr_t(Int, 3), nil},
		{"index read", `{ let x = [1, 2, 3] x[1] }`, Int, nil},
		{"index write", `{ mut x = [1, 2, 3] x[1] = 5 }`, void, nil},
		{"subslice array lo..hi", `{ let x = [1, 2, 3] x[0..2] }`, slice_t(Int), nil},
		{"subslice array lo..=hi", `{ let x = [1, 2, 3] x[0..=2] }`, slice_t(Int), nil},
		{"subslice array ..hi", `{ let x = [1, 2, 3] x[..2] }`, slice_t(Int), nil},
		{"subslice array lo..", `{ let x = [1, 2, 3] x[1..] }`, slice_t(Int), nil},
		{
			"subslice slice",
			`{ let @a = Arena() let x = make(@a, []Int(5)) x[1..3] }`,
			slice_t(Int), nil,
		},
		{
			"subslice through ref",
			`{ let x = [1, 2, 3] let y = &x y[0..2] }`,
			slice_t(Int), nil,
		},
		{
			"empty slice in make",
			`{ let @a = Arena() let x = make(@a, [][]Int(2, [])) }`,
			void, nil,
		},
		{
			"empty slice in assignment",
			`{ let @a = Arena() mut x = make(@a, []Int(3)) x = [] }`,
			void, nil,
		},
		{
			"empty slice as fun arg",
			`{ fun foo(s []Int) void {} foo([]) }`,
			void, nil,
		},
		{
			"empty slice in struct literal",
			`{ struct Foo { items []Int } Foo([]) }`,
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				got := e.env.TypeOfNode(id)
				_, ok := got.Kind.(StructType)
				assert.Equal(true, ok)
			},
		},
		{
			"empty slice in new array default",
			`{ let @a = Arena() let x = new(@a, [3][]Int([])) }`,
			void, nil,
		},
		{
			"multidimensional array type", `fun foo(a [3][4]Int) void {}`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				fun, ok := e.env.TypeOfNode(id).Kind.(FunType)
				assert.Equal(true, ok)
				outer, ok := e.env.Type(fun.Params[0]).Kind.(ArrayType)
				assert.Equal(true, ok)
				assert.Equal(int64(3), outer.Len)
				inner, ok := e.env.Type(outer.Elem).Kind.(ArrayType)
				assert.Equal(true, ok)
				assert.Equal(int64(4), inner.Len)
				assert.Equal(Int.ID, inner.Elem)
			},
		},
		{
			"multidimensional slice type", `fun foo(a [][]Int) void {}`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				fun, ok := e.env.TypeOfNode(id).Kind.(FunType)
				assert.Equal(true, ok)
				outer, ok := e.env.Type(fun.Params[0]).Kind.(SliceType)
				assert.Equal(true, ok)
				inner, ok := e.env.Type(outer.Elem).Kind.(SliceType)
				assert.Equal(true, ok)
				assert.Equal(Int.ID, inner.Elem)
			},
		},
		{
			"mixed array slice type", `fun foo(a [3][]Int) void {}`, nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				fun, ok := e.env.TypeOfNode(id).Kind.(FunType)
				assert.Equal(true, ok)
				outer, ok := e.env.Type(fun.Params[0]).Kind.(ArrayType)
				assert.Equal(true, ok)
				assert.Equal(int64(3), outer.Len)
				inner, ok := e.env.Type(outer.Elem).Kind.(SliceType)
				assert.Equal(true, ok)
				assert.Equal(Int.ID, inner.Elem)
			},
		},

		{"int +", `1 + 2`, Int, nil},
		{"int -", `1 - 2`, Int, nil},
		{"int *", `1 * 2`, Int, nil},
		{"int /", `1 / 2`, Int, nil},
		{"int %", `1 % 2`, Int, nil},

		{"< on int", `1 < 2`, Bool, nil},
		{"<= on int", `1 <= 2`, Bool, nil},
		{"> on int", `1 > 2`, Bool, nil},
		{">= on int", `1 >= 2`, Bool, nil},
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

		{"conditional for loop", `for true { }`, void, nil},
		{"unconditional for loop", `for { }`, void, nil},
		{"for body must be scoped", `{ let a = 1 for { let a = "hello" }}`, void, nil},
		{"for in range", `{ for x in 0..10 { } }`, void, nil},
		{"for in range inclusive", `{ for x in 0..=9 { } }`, void, nil},
		{
			"for in range with break",
			`{ for x in 0..10 { if x == 5 { break } } }`,
			void, nil,
		},
		{
			"for in range binding is Int",
			`{ for x in 0..10 { let y = x + 1 } }`,
			void, nil,
		},
		{
			"for in range binding shadows outer",
			`{ let i = 0 for i in 0..1 { } }`,
			void, nil,
		},

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
				block := base.Cast[ast.Block](e.ast.Node(id).Kind)
				funID := block.Exprs[1]
				typ, ok := e.env.TypeOfNode(funID).Kind.(FunType)
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
		{
			"method call in namespace",
			`fun ns() Int { struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) x.get() }`,
			nil,
			func(e *Engine, _ ast.NodeID, assert base.Assert) {
				assert.Equal(0, len(e.diagnostics))
			},
		},
		{
			"direct qualified call in namespace",
			`fun ns() Int { struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) Foo.get(x) }`,
			nil,
			func(e *Engine, _ ast.NodeID, assert base.Assert) {
				assert.Equal(0, len(e.diagnostics))
			},
		},
		{
			"generic struct two params nested",
			`{
				struct Pair<A, B> { first A second B }
				struct Box<T> { value T }
				let x = Pair<Box<Int>, Str>(Box<Int>(42), "hello")
				x.first.value
			}`,
			Int, nil,
		},

		// Generic functions.
		{
			"generic fun",
			`{
				struct Box<T> { value T }
				fun id<T>(x T) T { x }
				id<Int>(42)
				id<Str>("hello")
				let b = id<Box<Int>>(Box<Int>(99))
				b.value
			}`,
			Int, nil,
		},
		{
			"generic fun two type params",
			`{
				fun first<A, B>(a A, b B) A { a }
				first<Int, Str>(1, "x")
			}`,
			Int, nil,
		},
		{
			"generic fun dedup same type arg",
			`{
				fun id<T>(x T) T { x }
				id<Int>(1)
				id<Int>(2)
			}`,
			Int, nil,
		},
		{
			"method on generic struct",
			`{
				struct Foo<T> { one T }
				fun Foo.bar<T>(f Foo<T>, a T, b Bool) T { if b { return f.one } a }
				let x = Foo<Int>(42)
				x.bar(123, true)
			}`,
			Int,
			func(e *Engine, _ ast.NodeID, assert base.Assert) {
				structs := e.Structs()
				assert.Equal(1, len(structs))
				st := base.Cast[StructType](e.env.Type(structs[0].TypeID).Kind)
				base.Cast[IntType](e.env.Type(st.Fields[0].Type).Kind)
				funs := e.Funs()
				assert.Equal(1, len(funs))
				ft := base.Cast[FunType](e.env.Type(funs[0].TypeID).Kind)
				assert.Equal(3, len(ft.Params))
				base.Cast[StructType](e.env.Type(ft.Params[0]).Kind)
				base.Cast[IntType](e.env.Type(ft.Params[1]).Kind)
				base.Cast[BoolType](e.env.Type(ft.Params[2]).Kind)
				base.Cast[IntType](e.env.Type(ft.Return).Kind)
			},
		},
		{
			"generic method on non-generic struct",
			`{
				struct Foo { value Int }
				fun Foo.get<T>(f Foo, x T) T { x }
				let f = Foo(42)
				f.get<Str>("hello")
			}`,
			Str, nil,
		},
		{
			"generic method with extra type param on generic struct",
			`{
				struct Foo<T> { one T }
				fun Foo.bar<T, U>(f Foo<T>, a U) U { a }
				let x = Foo<Int>(42)
				x.bar<Str>("hello")
			}`,
			nil,
			func(e *Engine, _ ast.NodeID, assert base.Assert) {
				structs := e.Structs()
				assert.Equal(2, len(structs), "Foo<Int> and Str")
				funs := e.Funs()
				assert.Equal(1, len(funs))
				ft := base.Cast[FunType](e.env.Type(funs[0].TypeID).Kind)
				assert.Equal(2, len(ft.Params))
				paramStruct := base.Cast[StructType](e.env.Type(ft.Params[0]).Kind)
				base.Cast[IntType](e.env.Type(paramStruct.Fields[0].Type).Kind)
				base.Cast[StructType](e.env.Type(ft.Params[1]).Kind)
				base.Cast[StructType](e.env.Type(ft.Return).Kind)
			},
		},
		{
			"generic fun calls generic fun",
			`{
				fun id<T>(x T) T { x }
				fun wrap<T>(x T) T { id<T>(x) }
				wrap<Int>(42)
			}`,
			Int, nil,
		},
		{
			"generic fun shadowing",
			`{
				fun foo<T>(x T) T { x }
				let a = foo<Int>(1)
				{
					fun foo<T>(x T) Int { 99 }
					let b = foo<Str>("hi")
					b
				}
			}`,
			Int, nil,
		},
		{
			"generic fun creates struct from type param",
			`{
				struct Box<T> { value T }
				fun box<T>(x T) Box<T> { Box<T>(x) }
				let b = box<Int>(42)
				b.value
			}`,
			Int, nil,
		},
		{
			"generic fun with ref param",
			`{
				fun deref<T>(x &T) T { x.* }
				let x = 42
				deref<Int>(&x)
			}`,
			Int, nil,
		},
		{
			"generic fun as value",
			`{
				fun id<T>(x T) T { x }
				let f = id<Int>
				f(42)
			}`,
			Int, nil,
		},
		{
			"generic method as value",
			`{
				struct Foo { value Int }
				fun Foo.get<T>(f Foo, x T) T { x }
				let g = Foo.get<Str>
				g(Foo(1), "hello")
				let h = Foo.get<Int>
				h(Foo(1), 42)
			}`,
			Int, nil,
		},
		{
			"generic fun with mut ref param",
			`{
				fun set<T>(x &mut T, v T) void { x.* = v }
				mut x = 1
				set<Int>(&mut x, 42)
			}`,
			void, nil,
		},

		{
			"generic struct",
			`{ struct Foo<T> { value T } let x = Foo<Int>(42) x.value }`,
			Int, nil,
		},
		{
			"generic struct identity and distinctness",
			`{ struct Foo<T> { value T } let a = Foo<Int>(1) let b = Foo<Int>(2) let c = Foo<Str>("x") }`,
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				block := base.Cast[ast.Block](e.ast.Node(id).Kind)
				aVar := base.Cast[ast.Var](e.ast.Node(block.Exprs[1]).Kind)
				bVar := base.Cast[ast.Var](e.ast.Node(block.Exprs[2]).Kind)
				cVar := base.Cast[ast.Var](e.ast.Node(block.Exprs[3]).Kind)
				aName := base.Cast[StructType](e.env.TypeOfNode(aVar.Expr).Kind).Name
				bName := base.Cast[StructType](e.env.TypeOfNode(bVar.Expr).Kind).Name
				cName := base.Cast[StructType](e.env.TypeOfNode(cVar.Expr).Kind).Name
				assert.Equal(aName, bName, "Foo<Int> should be the same type")
				assert.NotEqual(aName, cName, "Foo<Int> and Foo<Str> should be distinct")
			},
		},
		{
			"generic struct recursive",
			`{
				struct Node<T> { value T next &Node<T> }
				fun foo(n &Node<Int>) Int { n.next.value }
			}`,
			nil,
			func(e *Engine, id ast.NodeID, assert base.Assert) {
				funTyp := base.Cast[FunType](e.env.TypeOfNode(id).Kind)
				assert.Equal(1, len(funTyp.Params))
				ref := base.Cast[RefType](e.env.Type(funTyp.Params[0]).Kind)
				node := base.Cast[StructType](e.env.Type(ref.Type).Kind)
				assert.Equal(2, len(node.Fields))
				assert.Equal("value", node.Fields[0].Name)
				assert.Equal("next", node.Fields[1].Name)
				nextRef := base.Cast[RefType](e.env.Type(node.Fields[1].Type).Kind)
				assert.Equal(ref.Type, nextRef.Type, "recursive: next field should be same type")
			},
		},
		{
			"generic struct shadowed",
			`{
				struct Foo<T> { one T }
				let a = Foo<Int>(1)
				{
					struct Foo<T> { one T two T }
					let b = Foo<Int>(2, 3)
					b.two
				}
				a.one
			}`,
			Int, nil,
		},
		{
			"generic struct nested type arg",
			`{
				struct Foo<T> { value T }
				struct Bar<T> { inner Foo<T> }
				let x = Bar<Int>(Foo<Int>(42))
				x.inner.value
			}`,
			Int, nil,
		},
		{
			"generic method with ref to generic struct",
			`{
				struct Box<V> { value V }
				fun Box.get<V>(b &Box<V>) V { b.value }
				fun wrap<V>(b &Box<V>) V { b.get() }
				let b = Box<Int>(42)
				wrap<Int>(&b)
			}`,
			Int,
			nil,
		},
		{
			"generic fun with slice of type param",
			`{
				struct Bag<V> { items []V }
				fun Bag.len<V>(b &Bag<V>) Int { b.items.len }
				fun count<V>(b &Bag<V>) Int { b.len() }
				let @a = Arena()
				let b = Bag<Str>(make(@a, []Str(2, "")))
				count<Str>(&b)
			}`,
			Int,
			nil,
		},
		{
			"generic fun with fun-typed param",
			`{
				fun apply<T>(x T, f fun(T) Int) Int { f(x) }
				fun to_len(s Str) Int { s.data.len }
				apply<Str>("hi", to_len)
			}`,
			Int,
			nil,
		},

		// Shapes.
		{
			"shape field access",
			`{
				shape HasPair { one Str two Int }
				struct Pair { one Str two Int }
				fun first<T HasPair>(t T) Str { t.one }
				first<Pair>(Pair("hello", 42))
			}`,
			Str, nil,
		},
		{
			"shape satisfied with extra fields and different order",
			`{
				shape HasPair { one Str two Int }
				struct Big { extra Bool two Int name Str one Str }
				fun first<T HasPair>(t T) Str { t.one }
				first<Big>(Big(true, 42, "world", "hello"))
			}`,
			Str, nil,
		},
		{
			"shape forward declared after struct",
			`{
				struct Pair { one Str two Int }
				shape HasPair { one Str two Int }
				fun first<T HasPair>(t T) Str { t.one }
				first<Pair>(Pair("hello", 42))
			}`,
			Str, nil,
		},
		{
			"shape method call",
			`{
				shape Showable {
					fun Showable.show(self Showable) Str
				}
				struct Guitar {
					name Str
				}
				fun Guitar.show(g Guitar) Str { g.name }
				fun display<T Showable>(t T) Str { t.show() }
				display<Guitar>(Guitar("Telecaster"))
			}`,
			Str, nil,
		},
		{
			"shape method call with ref receiver",
			`{
				shape HasValue {
					fun HasValue.val(self &HasValue) Int
				}
				struct Foo { one Int }
				fun Foo.val(self &Foo) Int { self.one }
				fun peek<T HasValue>(t &T) Int { t.val() }
				let f = Foo(42)
				peek<Foo>(&f)
			}`,
			Int, nil,
		},
		{
			"shape method call with mut ref to immutable ref coercion",
			`{
				shape HasValue {
					fun HasValue.val(self &HasValue) Int
				}
				struct Foo { one Int }
				fun Foo.val(self &Foo) Int { self.one }
				fun peek<T HasValue>(t &mut T) Int { t.val() }
				mut f = Foo(42)
				peek<Foo>(&mut f)
			}`,
			Int, nil,
		},
		{
			"shape two constrained type params",
			`{
				shape HasName { fun HasName.name(self HasName) Str }
				shape HasAge { fun HasAge.age(self HasAge) Int }
				struct Foo { n Str }
				struct Bar { a Int }
				fun Foo.name(f Foo) Str { f.n }
				fun Bar.age(b Bar) Int { b.a }
				fun combine<A HasName, B HasAge>(a A, b B) Str { a.name() }
				combine<Foo, Bar>(Foo("x"), Bar(1))
			}`,
			Str, nil,
		},
		{
			"shape method returns self type",
			`{
				shape Clonable {
					fun Clonable.clone(self Clonable) Clonable
				}
				struct Foo { x Int }
				fun Foo.clone(f Foo) Foo { Foo(f.x) }
				fun dup<T Clonable>(t T) T { t.clone() }
				dup<Foo>(Foo(1))
			}`,
			nil,
			func(e *Engine, _ ast.NodeID, assert base.Assert) {
				assert.Equal(0, len(e.diagnostics))
			},
		},
		{
			"shape method with two shape params",
			`{
				shape Eq {
					fun Eq.eq(self Eq, other Eq) Bool
				}
				struct Num { x Int }
				fun Num.eq(a Num, b Num) Bool { a.x == b.x }
				fun same<T Eq>(a T, b T) Bool { a.eq(b) }
				same<Num>(Num(1), Num(1))
			}`,
			Bool, nil,
		},
		{
			"shape two structs same shape",
			`{
				shape Showable {
					fun Showable.show(self Showable) Str
				}
				struct A { }
				struct B { }
				fun A.show(a A) Str { "a" }
				fun B.show(b B) Str { "b" }
				fun display<T Showable>(t T) Str { t.show() }
				let x = display<A>(A())
				display<B>(B())
			}`,
			Str, nil,
		},
		{
			"shape multiple methods",
			`{
				shape S {
					fun S.foo(self S) Int
					fun S.bar(self S) Str
				}
				struct X { }
				fun X.foo(x X) Int { 1 }
				fun X.bar(x X) Str { "x" }
				fun test<T S>(t T) Str { let n = t.foo() t.bar() }
				test<X>(X())
			}`,
			Str, nil,
		},
		{
			"shape field and method combined",
			`{
				shape Named {
					name Str
					fun Named.greet(self Named) Str
				}
				struct Person { name Str age Int }
				fun Person.greet(p Person) Str { p.name }
				fun intro<T Named>(t T) Str {
					let n = t.name
					t.greet()
				}
				intro<Person>(Person("Alice", 30))
			}`,
			Str, nil,
		},
	}

	// We need a little hack here, because the "ref" and "mut ref" tests
	// violate the lifetime rules, but we still wan to test them in isolation.
	skipLifetimeCheck := []string{
		"ref type",
		"mut binding of immutable ref",
		"mut ref type",
		"immutable ref to mut",
		"ref of field",
		"ref of nested field",
		"ref of deref",
		"mut ref of mut field",
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
		"empty slice in make",
		"empty slice in assignment",
		"empty slice as fun arg",
		"empty slice in struct literal",
		"empty slice in new array default",
		"subslice array lo..hi",
		"subslice array lo..=hi",
		"subslice array ..hi",
		"subslice array lo..",
		"subslice slice",
		"subslice through ref",
		"make mut slice",
		"mut slice assignable to immutable",
		"mut slice index write no mut binding",
		"subslice mut array",
		"subslice mut slice",
		"subslice mut slice through mut ref",
		"subslice mut slice through immutable ref",
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
			// Check that fun- and struct-work does not contain type parameters.
			for _, f := range e.Funs() {
				ft := base.Cast[FunType](e.env.Type(f.TypeID).Kind)
				for _, p := range ft.Params {
					assert.Equal(false, e.env.containsTypeParam(p),
						"Funs() should not contain type params in params: %s", f.Name)
				}
				assert.Equal(false, e.env.containsTypeParam(ft.Return),
					"Funs() should not contain type params in return: %s", f.Name)
			}
			for _, s := range e.Structs() {
				st := base.Cast[StructType](e.env.Type(s.TypeID).Kind)
				assert.Equal(false, e.env.hasTypeParam(st.TypeArgs),
					"Structs() should not contain type params: %s", st.Name)
			}
			if !slices.Contains(skipLifetimeCheck, name) {
				a := NewLifetimeAnalyzer(e.ast, e.scopeGraph, e.Env())
				// a.Debug = base.NewStdoutDebug("lifetime")
				a.Check(exprID)
				assert.Equal(0, len(a.Diagnostics), "lifetime check failed: %s", a.Diagnostics)
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
		{
			"duplicate generic method called (module)",
			"struct Foo<T> { one T }\n" +
				"fun Foo.bar<T>(f &Foo<T>) T { f.one }\n" +
				"fun Foo.bar<T>(f &Foo<T>) T { f.one }\n" +
				"fun main() void { let f = Foo<Int>(42) let r = &f r.bar() }",
			[]string{
				"test.met:3:5: symbol already defined: test.Foo.bar\n" +
					"    fun Foo.bar<T>(f &Foo<T>) T { f.one }\n" +
					"    fun Foo.bar<T>(f &Foo<T>) T { f.one }\n" +
					"        ^^^^^^^\n" +
					"    fun main() void { let f = Foo<Int>(42) let r = &f r.bar() }\n",
			},
		},
		{"fun return mismatch", `fun foo() Str { 123 }`, []string{
			"test.met:1:17: return type mismatch: expected Str, got Int\n" +
				"    fun foo() Str { 123 }\n" +
				"                    ^^^",
		}},
		{"return mismatch", `fun foo() Str { return 123 }`, []string{
			"test.met:1:24: return type mismatch: expected Str, got Int\n" +
				"    fun foo() Str { return 123 }\n" +
				"                           ^^^",
		}},
		{"unreachable code after return", `fun foo() Int { return 123 456 }`, []string{
			"test.met:1:28: unreachable code\n" +
				"    fun foo() Int { return 123 456 }\n" +
				"                               ^^^",
		}},
		{"unreachable code after break", `fun foo() Int { for { break 456 } }`, []string{
			"test.met:1:29: unreachable code\n" +
				"    fun foo() Int { for { break 456 } }\n" +
				"                                ^^^",
		}},
		{"unreachable code after continue", `fun foo() Int { for { continue 456 } }`, []string{
			"test.met:1:32: unreachable code\n" +
				"    fun foo() Int { for { continue 456 } }\n" +
				"                                   ^^^",
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
		// These two tests are parsed as modules (not expressions) because
		// main function validation only applies inside a main module.
		{"main must return void (module)", `fun main() Int { 123 }`, []string{
			"test.met:1:12: main function cannot return a value\n" +
				`    fun main() Int { 123 }` + "\n" +
				`               ^^^`,
		}},
		{"main must not have params (module)", `fun main(a Int, b Str) void { }`, []string{
			"test.met:1:10: main function cannot take arguments\n" +
				`    fun main(a Int, b Str) void { }` + "\n" +
				`             ^^^^^^^^^^^^`,
		}},

		{"field access on non-struct", `123.one`, []string{
			"test.met:1:5: unknown field: Int.one\n" +
				`    123.one` + "\n" +
				`        ^^^`,
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
		// Checking the body should not modify the function type.
		{
			"type error in fun body does not poison fun type",
			`{ fun foo(s Str) Int { s.nope } foo("hello") }`,
			[]string{
				"test.met:1:26: unknown field: Str.nope\n" +
					`    { fun foo(s Str) Int { s.nope } foo("hello") }` + "\n" +
					"                             ^^^^",
			},
		},

		{"Str cannot be constructed directly", `{ let @a = Arena() let d = make(@a, []U8(1)) Str(d) }`, []string{
			"test.met:1:46: Str cannot be constructed directly; use Str.from_utf8_lossy() instead\n" +
				`    { let @a = Arena() let d = make(@a, []U8(1)) Str(d) }` + "\n" +
				"                                                 ^^^^^^",
		}},

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
			"&mut of immutable field",
			`{ struct Foo { one Int } mut x = Foo(42) let y = &mut x.one }`,
			[]string{
				"test.met:1:50: cannot take mutable reference to immutable value\n" +
					`    { struct Foo { one Int } mut x = Foo(42) let y = &mut x.one }` + "\n" +
					"                                                     ^^^^^^^^^^",
			},
		},
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
		{"subslice on non-array", `{ let x = 123 x[0..1] }`, []string{
			"test.met:1:15: not an array or slice: Int\n" +
				"    { let x = 123 x[0..1] }\n" +
				"                  ^",
		}},
		{"subslice with non-int lo", `{ let x = [1, 2, 3] x["a"..2] }`, []string{
			"test.met:1:23: range bound must be Int, got Str\n" +
				`    { let x = [1, 2, 3] x["a"..2] }` + "\n" +
				`                          ^^^`,
		}},
		{"subslice with non-int hi", `{ let x = [1, 2, 3] x[0.."b"] }`, []string{
			"test.met:1:26: range bound must be Int, got Str\n" +
				`    { let x = [1, 2, 3] x[0.."b"] }` + "\n" +
				`                             ^^^`,
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
		{"for in range bound not Int", `{ for x in "a".."z" {} }`, []string{
			"test.met:1:12: range bound must be Int, got Str\n" +
				`    { for x in "a".."z" {} }` + "\n" +
				`               ^^^`,
		}},
		{"for in binding is immutable", `{ for x in 0..10 { x = 5 } }`, []string{
			"test.met:1:20: cannot assign to immutable variable: x\n" +
				`    { for x in 0..10 { x = 5 } }` + "\n" +
				"                       ^",
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
			"test.met:1:48: &Foo is not safe to leave uninitialized, provide a default value\n" +
				`    { struct Foo{one Int} let @a = Arena() new(@a, [5]&Foo()) }` + "\n" +
				`                                                   ^^^^^^^`,
		}},
		{
			"new array struct with ref field uninitialized",
			`{ struct Foo{one &Int} let @a = Arena() new(@a, [5]Foo()) }`,
			[]string{
				"test.met:1:49: Foo is not safe to leave uninitialized, provide a default value\n" +
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
			"test.met:1:49: &Foo is not safe to leave uninitialized, provide a default value\n" +
				`    { struct Foo{one Int} let @a = Arena() make(@a, []&Foo(3)) }` + "\n" +
				`                                                    ^^^^^^`,
		}},
		{"empty slice without context", `[]`, []string{
			"test.met:1:1: cannot infer type of empty slice []\n" +
				"    []\n" +
				"    ^^",
		}},
		{"empty slice in let binding", `let x = []`, []string{
			"test.met:1:9: cannot infer type of empty slice []\n" +
				"    let x = []\n" +
				"            ^^",
		}},
		{
			"write to immutable slice element",
			`{ let @a = Arena() let x = make(@a, []Int(3)) x[0] = 5 }`,
			[]string{
				"test.met:1:47: cannot assign to element of immutable array or slice\n" +
					`    { let @a = Arena() let x = make(@a, []Int(3)) x[0] = 5 }` + "\n" +
					"                                                  ^^^^",
			},
		},
		{
			"write through mut ref to immutable slice",
			`{ let @a = Arena() mut x = make(@a, []Int(3)) let y = &mut x y[0] = 5 }`,
			[]string{
				"test.met:1:62: cannot assign to element of immutable array or slice\n" +
					`    { let @a = Arena() mut x = make(@a, []Int(3)) let y = &mut x y[0] = 5 }` + "\n" +
					"                                                                 ^^^^",
			},
		},
		{
			"immutable slice not assignable to mut slice param",
			`{ let @a = Arena() fun foo(s []mut Int) void {} foo(make(@a, []Int(3))) }`,
			[]string{
				"test.met:1:53: type mismatch at argument 1: expected []mut Int, got []Int\n" +
					`    { let @a = Arena() fun foo(s []mut Int) void {} foo(make(@a, []Int(3))) }` + "\n" +
					"                                                        ^^^^^^^^^^^^^^^^^^",
			},
		},
		{"cannot return allocator from fun", `fun foo() Arena { }`, []string{
			"test.met:1:11: cannot return an allocator from a function\n" +
				`    fun foo() Arena { }` + "\n" +
				"              ^^^^^",
		}},

		// Type constructor errors — see TestIntTypes for comprehensive coverage.
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
		{
			"method call receiver ref mismatch", `
            {
                struct Foo { one Int }
                fun Foo.get(f Foo) Int { f.one }
                let x = Foo(42)
                let y = &x
                y.get()
            }`,
			[]string{
				"test.met:7:17: type mismatch at receiver: expected Foo, got &Foo\n" +
					strings.Trim(`
                    let y = &x
                    y.get()
                    ^
                }`, "\n") + "\n",
			},
		},

		// Generic struct errors.
		{
			"generic struct type arg count too few",
			`{ struct Foo<T, U> { one T two U } let x = Foo<Int>(42, 42) }`,
			[]string{
				"test.met:1:44: type argument count mismatch: expected 2, got 1\n" +
					"    { struct Foo<T, U> { one T two U } let x = Foo<Int>(42, 42) }\n" +
					"                                               ^^^^^^^^",
			},
		},
		{
			"generic struct type arg count too many",
			`{ struct Foo<T> { value T } let x = Foo<Int, Str>(42) }`,
			[]string{
				"test.met:1:37: type argument count mismatch: expected 1, got 2\n" +
					"    { struct Foo<T> { value T } let x = Foo<Int, Str>(42) }\n" +
					"                                        ^^^^^^^^^^^^^",
			},
		},
		{
			"generic struct duplicate type param",
			`{ struct Foo<T, T> { one T two T } }`,
			[]string{
				"test.met:1:17: duplicate type parameter: T\n" +
					"    { struct Foo<T, T> { one T two T } }\n" +
					"                    ^",
			},
		},
		{
			"generic struct type args on non-struct",
			`{ fun foo(x Int<Str>) void {} }`,
			[]string{
				"test.met:1:13: type arguments on non-struct type: Int\n" +
					"    { fun foo(x Int<Str>) void {} }\n" +
					"                ^^^^^^^^",
			},
		},
		{
			"generic struct missing type args",
			`{ struct Foo<T> { value T } fun bar(f Foo) void {} }`,
			[]string{
				"test.met:1:39: type argument count mismatch: expected 1, got 0\n" +
					"    { struct Foo<T> { value T } fun bar(f Foo) void {} }\n" +
					"                                          ^^^",
			},
		},

		// Generic function errors.
		{
			"generic fun type arg count too few",
			`{ fun foo<A, B>(a A, b B) A { a } foo<Int>(1, 2) }`,
			[]string{
				"test.met:1:35: type argument count mismatch: expected 2, got 1\n" +
					"    { fun foo<A, B>(a A, b B) A { a } foo<Int>(1, 2) }\n" +
					"                                      ^^^^^^^^",
			},
		},
		{
			"generic fun type arg count too many",
			`{ fun foo<T>(x T) T { x } foo<Int, Str>(1) }`,
			[]string{
				"test.met:1:27: type argument count mismatch: expected 1, got 2\n" +
					"    { fun foo<T>(x T) T { x } foo<Int, Str>(1) }\n" +
					"                              ^^^^^^^^^^^^^",
			},
		},
		{
			"generic fun duplicate type param",
			`{ fun foo<T, T>(x T) T { x } }`,
			[]string{
				"test.met:1:14: duplicate type parameter: T\n" +
					"    { fun foo<T, T>(x T) T { x } }\n" +
					"                 ^",
			},
		},
		{
			"generic fun missing type args",
			`{ fun foo<T>(x T) T { x } foo(1) }`,
			[]string{
				"test.met:1:27: type argument count mismatch: expected 1, got 0\n" +
					"    { fun foo<T>(x T) T { x } foo(1) }\n" +
					"                              ^^^",
			},
		},
		{
			"method on generic struct too few type args",
			`{ struct Foo<T> { one T } fun Foo.bar<T, U>(f Foo<T>, a U) U { a } let x = Foo<Int>(1) x.bar() }`,
			[]string{
				"test.met:1:90: type argument count mismatch: expected 2, got 1\n" +
					"    { struct Foo<T> { one T } fun Foo.bar<T, U>(f Foo<T>, a U) U { a } let x = Foo<Int>(1) x.bar() }\n" +
					"                                                                                             ^^^",
			},
		},
		{
			"method on generic struct too many type args",
			`{ struct Foo<T> { one T } fun Foo.bar<T>(f Foo<T>) T { f.one } let x = Foo<Int>(1) x.bar<Int, Str>() }`,
			[]string{
				"test.met:1:86: type argument count mismatch: expected 1, got 3\n" +
					"    { struct Foo<T> { one T } fun Foo.bar<T>(f Foo<T>) T { f.one } let x = Foo<Int>(1) x.bar<Int, Str>() }\n" +
					"                                                                                         ^^^",
			},
		},
		{
			"method on generic struct wrong first param type",
			`{ struct Foo<T> { one T } fun Foo.bar<T>(f Int) T { f } let x = Foo<Int>(1) x.bar() }`,
			[]string{
				"test.met:1:53: return type mismatch: expected T, got Int\n" +
					"    { struct Foo<T> { one T } fun Foo.bar<T>(f Int) T { f } let x = Foo<Int>(1) x.bar() }\n" +
					"                                                        ^",
				"test.met:1:77: type mismatch at receiver: expected Int, got Foo<Int>\n" +
					"    { struct Foo<T> { one T } fun Foo.bar<T>(f Int) T { f } let x = Foo<Int>(1) x.bar() }\n" +
					"                                                                                ^",
			},
		},
		{
			"method on generic struct type param must be in first position",
			`{ struct Foo<T> { one T } fun Foo.bar<U, V>(f Foo<V>, a U) U { a } let x = Foo<Int>(1) x.bar<Str>("hi") }`,
			[]string{
				"test.met:1:88: type mismatch at receiver: expected Foo<Str>, got Foo<Int>\n" +
					"    { struct Foo<T> { one T } fun Foo.bar<U, V>(f Foo<V>, a U) U { a } let x = Foo<Int>(1) x.bar<Str>(\"hi\") }\n" +
					"                                                                                           ^",
			},
		},

		// Shape errors.
		{
			"shape not satisfied missing field",
			`{ shape HasPair { one Str two Int } struct Foo { one Str } fun first<T HasPair>(t T) Str { t.one } first<Foo>(Foo("hello")) }`,
			[]string{
				"test.met:1:100: type Foo does not satisfy shape HasPair: missing field two\n" +
					`    { shape HasPair { one Str two Int } struct Foo { one Str } fun first<T HasPair>(t T) Str { t.one } first<Foo>(Foo("hello")) }` + "\n" +
					"                                                                                                       ^^^^^^^^^^",
			},
		},
		{
			"shape not satisfied wrong field type",
			`{ shape HasPair { one Str two Int } struct Foo { one Str two Str } fun first<T HasPair>(t T) Str { t.one } first<Foo>(Foo("hello", "world")) }`,
			[]string{
				"test.met:1:108: type Foo does not satisfy shape HasPair: field two has type Str, expected Int\n" +
					`    { shape HasPair { one Str two Int } struct Foo { one Str two Str } fun first<T HasPair>(t T) Str { t.one } first<Foo>(Foo("hello", "world")) }` + "\n" +
					"                                                                                                               ^^^^^^^^^^",
			},
		},
		{
			"shape not satisfied field not mut",
			`{ shape S { mut one Int } struct Foo { one Int } fun foo<T S>(t T) Int { t.one } foo<Foo>(Foo(1)) }`,
			[]string{
				"test.met:1:82: type Foo does not satisfy shape S: field one must be mut\n" +
					"    { shape S { mut one Int } struct Foo { one Int } fun foo<T S>(t T) Int { t.one } foo<Foo>(Foo(1)) }\n" +
					"                                                                                     ^^^^^^^^",
			},
		},
		{
			"shape not satisfied ref vs mut ref",
			`{ shape S { one &mut Int } struct Foo { one &Int } fun foo<T S>(t T) Int { 1 } let x = 1 foo<Foo>(Foo(&x)) }`,
			[]string{
				"test.met:1:90: type Foo does not satisfy shape S: field one has type &Int, expected &mut Int\n" +
					"    { shape S { one &mut Int } struct Foo { one &Int } fun foo<T S>(t T) Int { 1 } let x = 1 foo<Foo>(Foo(&x)) }\n" +
					"                                                                                             ^^^^^^^^",
			},
		},
		{
			"shape not satisfied missing method",
			`{ shape S { fun S.foo(s S) Int } struct Foo { } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }`,
			[]string{
				"test.met:1:83: type Foo does not satisfy shape S: missing method foo\n" +
					"    { shape S { fun S.foo(s S) Int } struct Foo { } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }\n" +
					"                                                                                      ^^^^^^^^",
			},
		},
		{
			"shape not satisfied wrong method return type",
			`{ shape S { fun S.foo(s S) Int } struct Foo { } fun Foo.foo(f Foo) Str { "x" } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }`,
			[]string{
				"test.met:1:114: type Foo does not satisfy shape S: method foo has signature fun(Foo) Str, expected fun(S) Int\n" +
					`    { shape S { fun S.foo(s S) Int } struct Foo { } fun Foo.foo(f Foo) Str { "x" } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }` + "\n" +
					"                                                                                                                     ^^^^^^^^",
			},
		},
		{
			"shape not satisfied wrong method param type",
			`{ shape S { fun S.foo(s S, n Int) Int } struct Foo { } fun Foo.foo(f Foo, n Str) Int { 1 } fun bar<T S>(t T) Int { t.foo(1) } bar<Foo>(Foo()) }`,
			[]string{
				"test.met:1:127: type Foo does not satisfy shape S: method foo has signature fun(Foo, Str) Int, expected fun(S, Int) Int\n" +
					"    { shape S { fun S.foo(s S, n Int) Int } struct Foo { } fun Foo.foo(f Foo, n Str) Int { 1 } fun bar<T S>(t T) Int { t.foo(1) } bar<Foo>(Foo()) }\n" +
					"                                                                                                                                  ^^^^^^^^",
			},
		},
		{
			"shape not satisfied wrong method param count",
			`{ shape S { fun S.foo(s S) Int } struct Foo { } fun Foo.foo(f Foo, n Int) Int { n } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }`,
			[]string{
				"test.met:1:119: type Foo does not satisfy shape S: method foo has signature fun(Foo, Int) Int, expected fun(S) Int\n" +
					"    { shape S { fun S.foo(s S) Int } struct Foo { } fun Foo.foo(f Foo, n Int) Int { n } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }\n" +
					"                                                                                                                          ^^^^^^^^",
			},
		},
		{
			"method on unconstrained type param (module)",
			"shape X { fun X.to_str(x X) Str }\n" +
				"struct Value<T X> { value T }\n" +
				"fun Value.to_str<T>(v Value<T>) Str { v.value.to_str() }",
			[]string{
				"test.met:3:47: unconstrained type parameter has no fields or methods: T\n" +
					"    struct Value<T X> { value T }\n" +
					"    fun Value.to_str<T>(v Value<T>) Str { v.value.to_str() }\n" +
					"                                                  ^^^^^^",
			},
		},
		{
			"shape duplicate method",
			`{ shape S { fun S.foo(s S) Int fun S.foo(s S) Str } }`,
			[]string{
				"test.met:1:36: symbol already defined: S.foo\n" +
					"    { shape S { fun S.foo(s S) Int fun S.foo(s S) Str } }\n" +
					"                                       ^^^^^",
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
			source := base.NewSource("test.met", "test", true, []rune(tt.src))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens, ast.NewAST(1))
			var nodeID ast.NodeID
			if strings.HasSuffix(name, "(module)") {
				nodeID, _ = parser.ParseModule()
			} else {
				nodeID, _ = parser.ParseExpr(0)
				parser.Roots = append(parser.Roots, nodeID)
			}
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			preludeAST, _ := ast.PreludeAST(true)
			e := NewEngine(parser.AST, preludeAST, &modules.ModuleResolution{})
			e.Query(nodeID)
			for i, want := range tt.want {
				if i >= len(e.diagnostics) {
					t.Fatalf("no more diagnostics, but wanted: %s", want)
				}
				assert.Equal(want, e.diagnostics[i].Display())
			}
			if len(e.diagnostics) > len(tt.want) {
				t.Fatalf("there are more diagnostics than expected: %s", e.diagnostics[len(tt.want):])
			}
		})
	}
}

func zeroIDAndSpan(typ *Type, _ TypeStatus) bool {
	isPrelude := typ.NodeID >= ast.PreludeFirstID
	typ.NodeID = ast.NodeID(0)
	typ.Span = base.Span{}
	switch typ.Kind.(type) {
	case IntType, BoolType, VoidType, AllocatorType:
		return true
	case StructType, SliceType:
		if isPrelude {
			return true
		}
	}
	typ.ID = TypeID(0)
	return true
}

func lookupIntType(name string) IntType {
	for _, t := range intTypes {
		if t.Name == name {
			return t
		}
	}
	panic("unknown integer type: " + name)
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
	return &Type{Kind: StructType{name, structFields, nil}}
}

func arr_t(typ *Type, size int) *Type {
	return &Type{Kind: ArrayType{typ.ID, int64(size)}}
}

func slice_t(typ *Type) *Type {
	return &Type{Kind: SliceType{Elem: typ.ID, Mut: false}}
}

func mut_slice_t(typ *Type) *Type {
	return &Type{Kind: SliceType{Elem: typ.ID, Mut: true}}
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

func TestIntTypes(t *testing.T) {
	assert := base.NewAssert(t)
	allIntTypes := []string{"I8", "I16", "I32", "Int", "U8", "U16", "U32", "U64"}

	typeCheck := func(t *testing.T, src string) *Engine {
		t.Helper()
		source := base.NewSource("test.met", "test", true, []rune(src))
		tokens := token.Lex(source)
		parser := ast.NewParser(tokens, ast.NewAST(1))
		exprID, parseOK := parser.ParseExpr(0)
		if !parseOK || len(parser.Diagnostics) > 0 {
			t.Fatalf("parse failed: %s", parser.Diagnostics)
		}
		preludeAST, _ := ast.PreludeAST(true)
		parser.Roots = append(parser.Roots, exprID)
		e := NewEngine(parser.AST, preludeAST, &modules.ModuleResolution{})
		e.Query(exprID)
		return e
	}

	t.Run("literal range", func(t *testing.T) {
		// Each type constructor accepts 0 and its max literal value.
		for _, info := range intTypes {
			for _, val := range []string{"0", info.Max.String()} {
				src := fmt.Sprintf("%s(%s)", info.Name, val)
				e := typeCheck(t, src)
				assert.Equal(0, len(e.diagnostics), "%s(%s) should be valid: %s", info.Name, val, e.diagnostics)
			}
		}
		// NOTE: Signed min values (e.g. I8(-128)) can't be expressed as
		// literals because the language has no negative literal syntax and
		// `0 - 128` produces an Int which can't be narrowed to I8.
	})

	t.Run("literal out of range", func(t *testing.T) {
		for _, typ := range intTypes {
			aboveMax := new(big.Int).Add(typ.Max, big.NewInt(1))
			src := fmt.Sprintf("%s(%s)", typ.Name, aboveMax)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.diagnostics), "%s(%s) diagnostics: %s", typ.Name, aboveMax, e.diagnostics)
			assert.Contains(e.diagnostics[0].Display(), "out of range", "%s(%s)", typ.Name, aboveMax)
		}
	})

	t.Run("arithmetic", func(t *testing.T) {
		for _, op := range []string{"+", "-", "*", "/"} {
			for _, name := range allIntTypes {
				src := fmt.Sprintf("%[1]s(1) %[2]s %[1]s(1)", name, op)
				e := typeCheck(t, src)
				assert.Equal(0, len(e.diagnostics), "%s: %s", src, e.diagnostics)
			}
		}
	})

	t.Run("comparison", func(t *testing.T) {
		for _, op := range []string{"==", "!="} {
			for _, name := range allIntTypes {
				src := fmt.Sprintf("%[1]s(1) %[2]s %[1]s(1)", name, op)
				e := typeCheck(t, src)
				assert.Equal(0, len(e.diagnostics), "%s: %s", src, e.diagnostics)
			}
		}
	})

	t.Run("mixed types rejected in binary ops", func(t *testing.T) {
		e := typeCheck(t, `{ let x = I32(1) let y = U8(1) x + y }`)
		assert.Equal(1, len(e.diagnostics), "diagnostics: %s", e.diagnostics)
		assert.Contains(e.diagnostics[0].Display(), "type mismatch")
	})

	t.Run("non-integer rejected", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf(`%s("hello")`, name)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.diagnostics), "%s(Str) diagnostics: %s", name, e.diagnostics)
			assert.Contains(e.diagnostics[0].Display(), "use conversion methods instead", name)
		}
	})

	t.Run("wrong arg count", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf("%s(1, 2)", name)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.diagnostics), "%s(1,2) diagnostics: %s", name, e.diagnostics)
			assert.Contains(e.diagnostics[0].Display(), "takes exactly 1 argument", name)
		}
	})

	t.Run("type constructor rejects cross-type conversions", func(t *testing.T) {
		crossTypeTests := []struct{ from, to string }{
			{"I8", "I16"},
			{"I8", "I32"},
			{"I8", "Int"},
			{"I16", "I32"},
			{"I16", "Int"},
			{"I32", "Int"},
			{"U8", "U16"},
			{"U8", "U32"},
			{"U8", "U64"},
			{"U16", "U32"},
			{"U16", "U64"},
			{"U32", "U64"},
			{"U8", "I16"},
			{"U8", "I32"},
			{"U8", "Int"},
			{"U16", "I32"},
			{"U16", "Int"},
			{"U32", "Int"},
			{"I16", "I8"},
			{"I32", "I8"},
			{"Int", "I8"},
			{"I8", "U8"},
			{"I8", "U64"},
			{"U64", "Int"},
			{"U64", "I32"},
		}
		for _, tt := range crossTypeTests {
			name := fmt.Sprintf("%s_to_%s", tt.from, tt.to)
			t.Run(name, func(t *testing.T) {
				src := fmt.Sprintf("{ let x = %s(1) %s(x) }", tt.from, tt.to)
				e := typeCheck(t, src)
				assert.NotEqual(0, len(e.diagnostics), "%s(%s) should be rejected", tt.to, tt.from)
				if len(e.diagnostics) > 0 {
					assert.Contains(
						e.diagnostics[0].Display(), "use conversion methods instead", "%s → %s", tt.from, tt.to,
					)
				}
			})
		}
	})

	t.Run("type constructor allows identity", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf("{ let x = %s(1) %s(x) }", name, name)
			e := typeCheck(t, src)
			assert.Equal(0, len(e.diagnostics), "%s(%s) identity should be allowed: %s", name, name, e.diagnostics)
		}
	})

	t.Run("safe uninitialized", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf("{ let @a = Arena() let x = new(@a, [5]%s()) }", name)
			e := typeCheck(t, src)
			assert.Equal(0, len(e.diagnostics), "%s should be safe uninitialized: %s", name, e.diagnostics)
		}
	})
}
