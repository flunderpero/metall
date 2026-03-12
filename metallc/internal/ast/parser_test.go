//nolint:unparam
package ast

import (
	"math/big"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestParseOK(t *testing.T) {
	tests := []struct {
		name string
		kind string
		src  string
		want func(*TestAST) NodeID
	}{
		{
			"file with fun", "file", `fun foo() Str { "hello" 123 } `,
			func(a *TestAST) NodeID {
				return a.file(
					a.fun("foo", a.str_typ(), a.block(a.string_("hello"), a.int_(123))),
				)
			},
		},
		{
			"assign", "expr", `x = 123`,
			func(a *TestAST) NodeID {
				return a.assign(a.ident("x"), a.int_(123))
			},
		},
		{
			"let binding", "expr", `let x = 123`,
			func(a *TestAST) NodeID {
				return a.var_("x", a.int_(123))
			},
		},
		{
			"mut binding", "expr", `mut x = 123`,
			func(a *TestAST) NodeID {
				return a.mut_var("x", a.int_(123))
			},
		},
		{
			"block", "expr", `{ 0 "hello" }`,
			func(a *TestAST) NodeID {
				return a.block(a.int_(0), a.string_("hello"))
			},
		},
		{
			"empty block", "expr", `{ }`,
			func(a *TestAST) NodeID {
				return a.block()
			},
		},
		{"line comment", "expr", "-- comment\n 123", func(a *TestAST) NodeID { return a.int_(123) }},
		{"multi-line comment", "expr", `
			--- multi
			    line
				comment
			---
			123
			`, func(a *TestAST) NodeID { return a.int_(123) }},
		{
			"fun with &mut param", "expr", `fun foo(a Int, b &mut Str) Int { 123 }`,
			func(a *TestAST) NodeID {
				return a.fun("foo",
					a.fun_param("a", a.int_typ()), a.fun_param("b", a.mut_ref_typ(a.str_typ())),
					a.int_typ(),
					a.block(a.int_(123)),
				)
			},
		},
		{
			"fun in block", "expr", `{ fun foo() Str { "hello" 123 } }`,
			func(a *TestAST) NodeID {
				return a.block(
					a.fun("foo", a.str_typ(), a.block(a.string_("hello"), a.int_(123))),
				)
			},
		},
		{
			"void fun", "expr", `fun foo() void {}`,
			func(a *TestAST) NodeID {
				return a.fun("foo", a.void_typ(), a.block())
			},
		},
		{
			"fun call", "expr", `foo(123, "hello")`,
			func(a *TestAST) NodeID {
				return a.call(a.ident("foo"), a.int_(123), a.string_("hello"))
			},
		},
		{
			"call no args", "expr", `foo()`,
			func(a *TestAST) NodeID {
				return a.call(a.ident("foo"))
			},
		},
		{
			"chained call", "expr", `foo()()`,
			func(a *TestAST) NodeID {
				return a.call(a.call(a.ident("foo")))
			},
		},

		{"fun type", "expr", `fun foo(bar fun(Str, Int) Bool) void {}`, func(a *TestAST) NodeID {
			return a.fun(
				"foo",
				a.fun_param("bar", a.fun_typ(a.str_typ(), a.int_typ(), a.bool_typ())),
				a.void_typ(),
				a.block(),
			)
		}},
		{
			"fun type with void return", "expr", `fun foo(bar fun(Int) void) void {}`, func(a *TestAST) NodeID {
				return a.fun(
					"foo",
					a.fun_param("bar", a.fun_typ(a.int_typ(), a.void_typ())),
					a.void_typ(),
					a.block(),
				)
			},
		},

		{"void ident expr", "expr", "void", func(a *TestAST) NodeID { return a.ident("void") }},
		{"return", "expr", `fun foo() Int { return 123 }`, func(a *TestAST) NodeID {
			return a.fun("foo", a.int_typ(), a.block(a.return_(a.int_(123))))
		}},
		{"return void", "expr", `fun foo() void { return void }`, func(a *TestAST) NodeID {
			return a.fun("foo", a.void_typ(), a.block(a.return_(a.ident("void"))))
		}},

		{"bool true", "expr", "true", func(a *TestAST) NodeID { return a.bool_(true) }},
		{"bool false", "expr", "false", func(a *TestAST) NodeID { return a.bool_(false) }},
		{"rune literal", "expr", `'a'`, func(a *TestAST) NodeID { return a.rune_('a') }},
		{"rune literal unicode", "expr", `'é'`, func(a *TestAST) NodeID { return a.rune_('é') }},
		{
			"if then else", "expr", `if a { 42 } else { 123 }`,
			func(a *TestAST) NodeID {
				cond := a.ident("a")
				then := a.block(a.int_(42))
				else_ := a.block(a.int_(123))
				return a.if_(cond, then, &else_)
			},
		},

		{"struct declaration", "expr", "struct Foo { one Str mut two Int }", func(a *TestAST) NodeID {
			return a.struct_("Foo", a.struct_field("one", a.str_typ()), a.mut_struct_field("two", a.int_typ()))
		}},
		{"struct with allocator field", "expr", "struct Foo { @myalloc Arena }", func(a *TestAST) NodeID {
			return a.struct_("Foo", a.struct_field("@myalloc", a.typ("Arena")))
		}},
		{"struct literal", "expr", "Foo(\"hello\", 123)", func(a *TestAST) NodeID {
			return a.struct_lit(a.ident("Foo"), a.string_("hello"), a.int_(123))
		}},
		// Type constructors parse as struct literals — the type checker distinguishes them.
		{"type constructor U8", "expr", "U8(42)", func(a *TestAST) NodeID {
			return a.struct_lit(a.ident("U8"), a.int_(42))
		}},
		{"type constructor Int", "expr", "Int(123)", func(a *TestAST) NodeID {
			return a.struct_lit(a.ident("Int"), a.int_(123))
		}},
		{"field read", "expr", "x.one", func(a *TestAST) NodeID {
			return a.field_access(a.ident("x"), "one")
		}},
		{"field write", "expr", "x.one = \"hello\"", func(a *TestAST) NodeID {
			return a.assign(a.field_access(a.ident("x"), "one"), a.string_("hello"))
		}},
		{"chained field access", "expr", "x.one.two", func(a *TestAST) NodeID {
			return a.field_access(a.field_access(a.ident("x"), "one"), "two")
		}},
		{"call through field access", "expr", "x.one.two()", func(a *TestAST) NodeID {
			return a.call(a.field_access(a.field_access(a.ident("x"), "one"), "two"))
		}},

		{"&ref", "expr", `&x`, func(a *TestAST) NodeID { return a.ref(a.ident("x")) }},
		{"& has lower precedence than field access", "expr", `&x.one.two`, func(a *TestAST) NodeID {
			return a.ref(a.field_access(a.field_access(a.ident("x"), "one"), "two"))
		}},
		{"&mut field access", "expr", `&mut x.one`, func(a *TestAST) NodeID {
			return a.mut_ref(a.field_access(a.ident("x"), "one"))
		}},
		{"& of index", "expr", `&x[1]`, func(a *TestAST) NodeID {
			return a.ref(a.index(a.ident("x"), a.int_(1)))
		}},
		{"& of deref", "expr", `&x.*`, func(a *TestAST) NodeID {
			return a.ref(a.deref(a.ident("x")))
		}},
		{"& of chained field and index", "expr", `&x.one[2].three`, func(a *TestAST) NodeID {
			return a.ref(a.field_access(a.index(a.field_access(a.ident("x"), "one"), a.int_(2)), "three"))
		}},
		{"&mut ref", "expr", `&mut x`, func(a *TestAST) NodeID { return a.mut_ref(a.ident("x")) }},
		{"deref", "expr", `x.*`, func(a *TestAST) NodeID { return a.deref(a.ident("x")) }},
		{"nested deref", "expr", `x.*.*`, func(a *TestAST) NodeID { return a.deref(a.deref(a.ident("x"))) }},
		{
			"ref type",
			"expr",
			`fun foo() &Int {}`,
			func(a *TestAST) NodeID { return a.fun("foo", a.ref_typ(a.int_typ()), a.block()) },
		},
		{
			"nested ref type", "expr", `fun foo() &&Int {}`,
			func(a *TestAST) NodeID {
				return a.fun("foo", a.ref_typ(a.ref_typ(a.int_typ())), a.block())
			},
		},
		{
			"deref assign", "expr", `x.* = y`,
			func(a *TestAST) NodeID {
				return a.assign(a.deref(a.ident("x")), a.ident("y"))
			},
		},
		{
			"nested deref assign", "expr", `x.*.*.* = y`,
			func(a *TestAST) NodeID {
				return a.assign(a.deref(a.deref(a.deref(a.ident("x")))), a.ident("y"))
			},
		},
		{
			"call with &ref arg", "expr", `{ fun foo(a &Int) void {} let x = 123 foo(&x) }`,
			func(a *TestAST) NodeID {
				return a.block(
					a.fun("foo", a.fun_param("a", a.ref_typ(a.int_typ())), a.void_typ(), a.block()),
					a.var_("x", a.int_(123)),
					a.call(a.ident("foo"), a.ref(a.ident("x"))),
				)
			},
		},

		{"allocator var", "expr", "let @myalloc = Arena(123)", func(a *TestAST) NodeID {
			return a.allocator_var("@myalloc", "Arena", a.int_(123))
		}},
		{"heap alloc", "expr", `@myalloc.new<Foo>(Foo())`, func(a *TestAST) NodeID {
			return a.call(
				a.field_access_t(a.ident("@myalloc"), "new", a.typ("Foo")),
				a.struct_lit(a.ident("Foo")),
			)
		}},
		{
			"alloc fun param", "expr", "fun foo(@myalloc Arena, x Str, @youralloc Arena) void {}",
			func(a *TestAST) NodeID {
				return a.fun("foo",
					a.fun_param("@myalloc", a.typ("Arena")),
					a.fun_param("x", a.str_typ()),
					a.fun_param("@youralloc", a.typ("Arena")),
					a.void_typ(), a.block())
			},
		},
		{
			"pass alloc in call", "expr", "foo(@myalloc)",
			func(a *TestAST) NodeID {
				return a.call(a.ident("foo"), a.ident("@myalloc"))
			},
		},

		{"array type", "expr", `fun foo(a [5]Int) void {}}`, func(a *TestAST) NodeID {
			return a.fun("foo", a.fun_param("a", a.arr_typ(a.int_typ(), 5)), a.void_typ(), a.block())
		}},
		{"multidimensional array type", "expr", `fun foo(a [3][4]Int) void {}}`, func(a *TestAST) NodeID {
			return a.fun(
				"foo",
				a.fun_param("a", a.arr_typ(a.arr_typ(a.int_typ(), 4), 3)),
				a.void_typ(),
				a.block(),
			)
		}},
		{"multidimensional slice type", "expr", `fun foo(a [][]Str) void {}}`, func(a *TestAST) NodeID {
			return a.fun(
				"foo",
				a.fun_param("a", a.slice_typ(a.slice_typ(a.str_typ()))),
				a.void_typ(),
				a.block(),
			)
		}},
		{"mixed array slice type", "expr", `fun foo(a [3][]Int) void {}}`, func(a *TestAST) NodeID {
			return a.fun(
				"foo",
				a.fun_param("a", a.arr_typ(a.slice_typ(a.int_typ()), 3)),
				a.void_typ(),
				a.block(),
			)
		}},
		{"mut slice type", "expr", `fun foo(a []mut Int) void {}}`, func(a *TestAST) NodeID {
			return a.fun(
				"foo",
				a.fun_param("a", a.mut_slice_typ(a.int_typ())),
				a.void_typ(),
				a.block(),
			)
		}},
		{"array literal", "expr", `[1, 2, 3]`, func(a *TestAST) NodeID {
			return a.arr_lit(a.int_(1), a.int_(2), a.int_(3))
		}},
		{"empty slice", "expr", `[]`, func(a *TestAST) NodeID {
			return a.empty_slice()
		}},
		{"index read", "expr", `x[1]`, func(a *TestAST) NodeID {
			return a.index(a.ident("x"), a.int_(1))
		}},
		{"index write", "expr", `x[1] = 2`, func(a *TestAST) NodeID {
			return a.assign(a.index(a.ident("x"), a.int_(1)), a.int_(2))
		}},
		{"subslice lo..hi", "expr", `x[1..3]`, func(a *TestAST) NodeID {
			target := a.ident("x")
			lo, hi := a.int_(1), a.int_(3)
			return a.sub_slice(target, &lo, &hi, false)
		}},
		{"subslice lo..=hi", "expr", `x[1..=3]`, func(a *TestAST) NodeID {
			target := a.ident("x")
			lo, hi := a.int_(1), a.int_(3)
			return a.sub_slice(target, &lo, &hi, true)
		}},
		{"subslice ..hi", "expr", `x[..3]`, func(a *TestAST) NodeID {
			target := a.ident("x")
			hi := a.int_(3)
			return a.sub_slice(target, nil, &hi, false)
		}},
		{"subslice lo..", "expr", `x[1..]`, func(a *TestAST) NodeID {
			target := a.ident("x")
			lo := a.int_(1)
			return a.sub_slice(target, &lo, nil, false)
		}},
		{"subslice ..=hi", "expr", `x[..=3]`, func(a *TestAST) NodeID {
			target := a.ident("x")
			hi := a.int_(3)
			return a.sub_slice(target, nil, &hi, true)
		}},
		{"heap alloc from field", "expr", `x.@myalloc.new<Foo>(Foo("hello"))`, func(a *TestAST) NodeID {
			return a.call(
				a.field_access_t(a.field_access(a.ident("x"), "@myalloc"), "new", a.typ("Foo")),
				a.struct_lit(a.ident("Foo"), a.string_("hello")),
			)
		}},
		{"heap alloc mut struct", "expr", `@myalloc.new_mut<Foo>(Foo())`, func(a *TestAST) NodeID {
			return a.call(
				a.field_access_t(a.ident("@myalloc"), "new_mut", a.typ("Foo")),
				a.struct_lit(a.ident("Foo")),
			)
		}},
		{"make slice", "expr", `@myalloc.make<[]Int>(n, 42)`, func(a *TestAST) NodeID {
			return a.call(
				a.field_access_t(a.ident("@myalloc"), "make", a.slice_typ(a.int_typ())),
				a.ident("n"), a.int_(42),
			)
		}},
		{"make uninit slice", "expr", `@myalloc.make_uninit<[]Int>(n)`, func(a *TestAST) NodeID {
			return a.call(
				a.field_access_t(a.ident("@myalloc"), "make_uninit", a.slice_typ(a.int_typ())),
				a.ident("n"),
			)
		}},

		{"int +", "expr", "1 + 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpAdd, a.int_(1), a.int_(2))
		}},
		{"int -", "expr", "1 - 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpSub, a.int_(1), a.int_(2))
		}},
		{"int *", "expr", "1 * 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpMul, a.int_(1), a.int_(2))
		}},
		{"int /", "expr", "1 / 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpDiv, a.int_(1), a.int_(2))
		}},
		{"int %", "expr", "1 % 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpMod, a.int_(1), a.int_(2))
		}},
		{"int <", "expr", "1 < 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpLt, a.int_(1), a.int_(2))
		}},
		{"int <=", "expr", "1 <= 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpLte, a.int_(1), a.int_(2))
		}},
		{"int >", "expr", "1 > 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpGt, a.int_(1), a.int_(2))
		}},
		{"int >=", "expr", "1 >= 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpGte, a.int_(1), a.int_(2))
		}},
		{"arithmetic operator precedence", "expr", "1 + 2 * 3 + 4", func(a *TestAST) NodeID {
			one := a.int_(1)
			mul := a.binary(BinaryOpMul, a.int_(2), a.int_(3))
			add1 := a.binary(BinaryOpAdd, one, mul)
			return a.binary(BinaryOpAdd, add1, a.int_(4))
		}},

		{"==", "expr", "1 == 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpEq, a.int_(1), a.int_(2))
		}},
		{"!=", "expr", "1 != 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpNeq, a.int_(1), a.int_(2))
		}},
		{"logical and, or, not", "expr", "true and false or not true", func(a *TestAST) NodeID {
			and := a.binary(BinaryOpAnd, a.bool_(true), a.bool_(false))
			not := a.unary(UnaryOpNot, a.bool_(true))
			return a.binary(BinaryOpOr, and, not)
		}},
		{"and binds tighter than or", "expr", "true or false and true", func(a *TestAST) NodeID {
			t := a.bool_(true)
			and := a.binary(BinaryOpAnd, a.bool_(false), a.bool_(true))
			return a.binary(BinaryOpOr, t, and)
		}},

		{"bitwise and", "expr", "1 & 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpBitAnd, a.int_(1), a.int_(2))
		}},
		{"bitwise or", "expr", "1 | 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpBitOr, a.int_(1), a.int_(2))
		}},
		{"bitwise xor", "expr", "1 ^ 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpBitXor, a.int_(1), a.int_(2))
		}},
		{"shift left", "expr", "1 << 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpShl, a.int_(1), a.int_(2))
		}},
		{"shift right", "expr", "1 >> 2", func(a *TestAST) NodeID {
			return a.binary(BinaryOpShr, a.int_(1), a.int_(2))
		}},
		{"bitwise not", "expr", "~1", func(a *TestAST) NodeID {
			return a.unary(UnaryOpBitNot, a.int_(1))
		}},
		{"bitwise precedence", "expr", "1 | 2 ^ 3 & 4", func(a *TestAST) NodeID {
			one := a.int_(1)
			two := a.int_(2)
			band := a.binary(BinaryOpBitAnd, a.int_(3), a.int_(4))
			bxor := a.binary(BinaryOpBitXor, two, band)
			return a.binary(BinaryOpBitOr, one, bxor)
		}},
		{"shift precedence vs add", "expr", "1 + 2 << 3 + 4", func(a *TestAST) NodeID {
			add1 := a.binary(BinaryOpAdd, a.int_(1), a.int_(2))
			add2 := a.binary(BinaryOpAdd, a.int_(3), a.int_(4))
			return a.binary(BinaryOpShl, add1, add2)
		}},

		{"grouped expressions", "expr", "(1 + 2) * 3 + 4", func(a *TestAST) NodeID {
			add := a.binary(BinaryOpAdd, a.int_(1), a.int_(2))
			mul := a.binary(BinaryOpMul, add, a.int_(3))
			return a.binary(BinaryOpAdd, mul, a.int_(4))
		}},

		{"conditional for loop", "expr", "for true { 1 } ", func(a *TestAST) NodeID {
			return a.for_cond(a.bool_(true), a.block(a.int_(1)))
		}},
		{"unconditional for loop", "expr", "for { 1 } ", func(a *TestAST) NodeID {
			return a.for_(a.block(a.int_(1)))
		}},
		{"break", "expr", "break", func(a *TestAST) NodeID {
			return a.break_()
		}},
		{"continue", "expr", "continue", func(a *TestAST) NodeID {
			return a.continue_()
		}},
		{"for in range", "expr", "for x in 0..10 { 1 }", func(a *TestAST) NodeID {
			return a.for_in("x", a.range_(a.int_(0), a.int_(10)), a.block(a.int_(1)))
		}},
		{"for in range inclusive", "expr", "for x in 0..=10 { 1 }", func(a *TestAST) NodeID {
			return a.for_in("x", a.range_inclusive(a.int_(0), a.int_(10)), a.block(a.int_(1)))
		}},

		{"generic struct", "expr", `struct Foo<T> { value T }`, func(a *TestAST) NodeID {
			return a.generic_struct("Foo", []NodeID{a.type_param("T")}, a.struct_field("value", a.typ("T")))
		}},
		{"generic struct two params", "expr", `struct Foo<A, B> { a A b B }`, func(a *TestAST) NodeID {
			return a.generic_struct("Foo",
				[]NodeID{a.type_param("A"), a.type_param("B")},
				a.struct_field("a", a.typ("A")), a.struct_field("b", a.typ("B")),
			)
		}},
		{"generic fun", "expr", `fun foo<T>(x T) T { x }`, func(a *TestAST) NodeID {
			return a.generic_fun("foo",
				[]NodeID{a.type_param("T")},
				a.fun_param("x", a.typ("T")),
				a.typ("T"),
				a.block(a.ident("x")),
			)
		}},
		{"type arg in type", "expr", `fun foo(x Foo<Int>) void {}`, func(a *TestAST) NodeID {
			return a.fun("foo",
				a.fun_param("x", a.typ_args("Foo", a.int_typ())),
				a.void_typ(),
				a.block(),
			)
		}},
		{"nested type args", "expr", `fun foo(x Foo<Bar<Int>, Baz<Str>>) void {}`, func(a *TestAST) NodeID {
			return a.fun("foo",
				a.fun_param("x", a.typ_args("Foo", a.typ_args("Bar", a.int_typ()), a.typ_args("Baz", a.str_typ()))),
				a.void_typ(),
				a.block(),
			)
		}},
		{"struct literal with type args", "expr", `Foo<Int>(42)`, func(a *TestAST) NodeID {
			return a.struct_lit(a.ident_type_args("Foo", a.int_typ()), a.int_(42))
		}},
		{"struct literal nested type args", "expr", `Foo<Bar<Int>>(42)`, func(a *TestAST) NodeID {
			return a.struct_lit(a.ident_type_args("Foo", a.typ_args("Bar", a.int_typ())), a.int_(42))
		}},
		{"call with type args", "expr", `foo<Int>(42)`, func(a *TestAST) NodeID {
			return a.call(a.ident_type_args("foo", a.int_typ()), a.int_(42))
		}},
		{"call nested type args", "expr", `foo<Bar<Int>>(42)`, func(a *TestAST) NodeID {
			return a.call(a.ident_type_args("foo", a.typ_args("Bar", a.int_typ())), a.int_(42))
		}},

		{"constrained type param", "expr", `fun foo<T Showable>(t T) void { }`, func(a *TestAST) NodeID {
			return a.generic_fun("foo",
				[]NodeID{a.constrained_type_param("T", a.typ("Showable"))},
				a.fun_param("t", a.typ("T")),
				a.void_typ(),
				a.block(),
			)
		}},
		{"shape", "expr", `shape Foo { name Str }`, func(a *TestAST) NodeID {
			return a.shape("Foo", []NodeID{a.struct_field("name", a.str_typ())})
		}},
		{"shape with fun", "expr", `shape Foo { fun Foo.bar(f Foo) Str }`, func(a *TestAST) NodeID {
			return a.shape("Foo", nil,
				a.fun_decl("Foo.bar", a.fun_param("f", a.typ("Foo")), a.str_typ()),
			)
		}},
		{
			"shape with field and fun", "expr",
			`shape Foo { name Str fun Foo.bar(f Foo) Str }`,
			func(a *TestAST) NodeID {
				return a.shape("Foo",
					[]NodeID{a.struct_field("name", a.str_typ())},
					a.fun_decl("Foo.bar", a.fun_param("f", a.typ("Foo")), a.str_typ()),
				)
			},
		},
		{"union", "expr", `union Foo = Str | Int`, func(a *TestAST) NodeID {
			return a.union_("Foo", a.str_typ(), a.int_typ())
		}},
		{"union three variants", "expr", `union Foo = Str | Int | Bool`, func(a *TestAST) NodeID {
			return a.union_("Foo", a.str_typ(), a.int_typ(), a.typ("Bool"))
		}},
		{"generic union", "expr", `union Maybe<T> = T | None`, func(a *TestAST) NodeID {
			return a.generic_union("Maybe", []NodeID{a.type_param("T")}, a.typ("T"), a.typ("None"))
		}},
		{"generic union with type args", "expr", `union Foo<T> = Str | Bar<T> | Int`, func(a *TestAST) NodeID {
			return a.generic_union("Foo",
				[]NodeID{a.type_param("T")},
				a.str_typ(), a.typ_args("Bar", a.typ("T")), a.int_typ(),
			)
		}},
		{"union with ref variant", "expr", `union Foo = &Str | Int`, func(a *TestAST) NodeID {
			return a.union_("Foo", a.ref_typ(a.str_typ()), a.int_typ())
		}},
		{"union with slice variant", "expr", `union Foo = []Int | Str`, func(a *TestAST) NodeID {
			return a.union_("Foo", a.slice_typ(a.int_typ()), a.str_typ())
		}},

		{
			"namespaced fun", "expr", `fun Foo.bar(f Foo) Int { 123 }`,
			func(a *TestAST) NodeID {
				return a.fun("Foo.bar",
					a.fun_param("f", a.typ("Foo")),
					a.int_typ(),
					a.block(a.int_(123)),
				)
			},
		},
		{
			"use simple path", "file", `use foo::bar`,
			func(a *TestAST) NodeID {
				return a.file_with_imports([]NodeID{a.import_("foo", "bar")})
			},
		},
		{
			"use deep path", "file", `use foo::bar::baz`,
			func(a *TestAST) NodeID {
				return a.file_with_imports([]NodeID{a.import_("foo", "bar", "baz")})
			},
		},
		{
			"use with alias", "file", `use b = foo::bar`,
			func(a *TestAST) NodeID {
				return a.file_with_imports([]NodeID{a.import_alias("b", "foo", "bar")})
			},
		},
		{
			"use local import", "file", `use local::foo::bar`,
			func(a *TestAST) NodeID {
				return a.file_with_imports([]NodeID{a.import_("local", "foo", "bar")})
			},
		},
		{
			"use local import with alias", "file", `use b = local::foo::bar`,
			func(a *TestAST) NodeID {
				return a.file_with_imports([]NodeID{a.import_alias("b", "local", "foo", "bar")})
			},
		},

		{
			"path expression", "expr", `math::pow`,
			func(a *TestAST) NodeID {
				return a.path_("math", "pow")
			},
		},
		{
			"path expression call", "expr", `math::pow(2, 5)`,
			func(a *TestAST) NodeID {
				return a.call(a.path_("math", "pow"), a.int_(2), a.int_(5))
			},
		},
		{
			"path expression with type ident", "expr", `lib::Point(1, 2)`,
			func(a *TestAST) NodeID {
				return a.struct_lit(a.path_("lib", "Point"), a.int_(1), a.int_(2))
			},
		},
		{
			"path struct literal with type args", "expr", `lib::Foo<Int>(42)`,
			func(a *TestAST) NodeID {
				return a.struct_lit(a.path_type_args([]string{"lib", "Foo"}, a.int_typ()), a.int_(42))
			},
		},
		{
			"path struct literal nested type args", "expr", `lib::Foo<Bar<Int>>(42)`,
			func(a *TestAST) NodeID {
				return a.struct_lit(
					a.path_type_args([]string{"lib", "Foo"}, a.typ_args("Bar", a.int_typ())),
					a.int_(42),
				)
			},
		},
		{
			"path call with type args", "expr", `lib::foo<Int>(42)`,
			func(a *TestAST) NodeID {
				return a.call(a.path_type_args([]string{"lib", "foo"}, a.int_typ()), a.int_(42))
			},
		},
		{
			"path in let binding", "expr", `let x = math::pow`,
			func(a *TestAST) NodeID {
				return a.var_("x", a.path_("math", "pow"))
			},
		},
		{
			"namespaced fun in file", "file", `fun Foo.bar(f Foo) Int { 123 }`,
			func(a *TestAST) NodeID {
				return a.file(
					a.fun("Foo.bar",
						a.fun_param("f", a.typ("Foo")),
						a.int_typ(),
						a.block(a.int_(123)),
					),
				)
			},
		},
	}

	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	assert := base.NewAssert(t)
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", "test", true, []rune(tt.src))
			tokens := token.Lex(source)
			parser := NewParser(tokens, NewAST(1))
			var gotRoot NodeID
			var ok bool
			switch tt.kind {
			case "expr":
				gotRoot, ok = parser.ParseExpr(0)
			case "file":
				gotRoot, ok = parser.ParseModule()
			default:
				t.Fatalf("unknown kind: %s", tt.kind)
			}
			assert.Equal(0, len(parser.Diagnostics), "diagnostics: %s", parser.Diagnostics)
			assert.Equal(true, ok, "parse function returned false")
			wantAST := NewTestAST()
			wantRoot := tt.want(wantAST)
			want := ast_to_list(wantAST.AST, wantRoot)
			got := ast_to_list(parser.AST, gotRoot)
			assert.Equal(want, got)
		})
	}
}

func TestParseErr(t *testing.T) {
	tests := []struct {
		name string
		kind string
		src  string
		want []string
	}{
		{"unexpected token", "expr", `=`, []string{
			"test.met:1:1: unexpected token: expected start of an expression, got =\n" +
				`    =` + "\n" +
				"    ^",
		}},
		// Type names can't appear on the left side of an assignment.
		{"assign to type name", "expr", `{ Str = "hello" }`, []string{
			"test.met:1:7: unexpected token: expected (, got =\n" +
				`    { Str = "hello" }` + "\n" +
				"          ^",
		}},
		{"nested &ref", "expr", `{ &&x }`, []string{
			"test.met:1:4: expected a place expression (variable, field, index, or deref)\n" +
				`    { &&x }` + "\n" +
				"       ^^",
		}},
		{"&ref of literal", "expr", `{ &123 }`, []string{
			"test.met:1:4: expected a place expression (variable, field, index, or deref)\n" +
				`    { &123 }` + "\n" +
				"       ^^^",
		}},

		{"return expects expr", "expr", `{ return }`, []string{
			"test.met:1:10: unexpected token: expected start of an expression, got }\n" +
				`    { return }` + "\n" +
				"             ^",
		}},

		{"reserved word Arena", "expr", `struct Arena{one Str}`, []string{
			"test.met:1:8: reserved word: Arena\n" +
				`    struct Arena{one Str}` + "\n" +
				"           ^^^^^",
		}},
		{"reserved word panic (fun)", "expr", `fun panic() void {}`, []string{
			"test.met:1:5: reserved word: panic\n" +
				`    fun panic() void {}` + "\n" +
				"        ^^^^^",
		}},
		{"reserved word panic (var)", "expr", `let panic = 123`, []string{
			"test.met:1:5: reserved word: panic\n" +
				`    let panic = 123` + "\n" +
				"        ^^^^^",
		}},
		{"mut allocator var", "expr", `mut @a = Arena()`, []string{
			"test.met:1:5: allocator variables cannot be mutable\n" +
				`    mut @a = Arena()` + "\n" +
				"        ^^",
		}},
		{"method fun missing method name", "expr", `fun Foo.() void {}`, []string{
			"test.met:1:9: unexpected token: expected <identifier>, got (\n" +
				"    fun Foo.() void {}\n" +
				"            ^",
		}},
		{"nested type param in struct", "expr", `struct Foo<T<A>> {}`, []string{
			"test.met:1:13: unexpected token: expected ,, got <<immediate>\n" +
				"    struct Foo<T<A>> {}\n" +
				"                ^",
		}},
		{"nested type param in fun", "expr", `fun foo<T<A>>() void {}`, []string{
			"test.met:1:10: unexpected token: expected ,, got <<immediate>\n" +
				"    fun foo<T<A>>() void {}\n" +
				"             ^",
		}},
		{"empty type params in struct", "expr", `struct Foo<> {}`, []string{
			"test.met:1:11: empty type parameter list\n" +
				"    struct Foo<> {}\n" +
				"              ^^",
		}},
		{"empty type params in fun", "expr", `fun foo<>() void {}`, []string{
			"test.met:1:8: empty type parameter list\n" +
				"    fun foo<>() void {}\n" +
				"           ^^",
		}},
		{"empty type args in struct literal", "expr", `{ struct Foo<T> { value T } Foo<>(42) }`, []string{
			"test.met:1:32: empty type argument list\n" +
				"    { struct Foo<T> { value T } Foo<>(42) }\n" +
				"                                   ^^",
		}},
		{"empty type args in type position", "expr", `struct Foo<T> { value Foo<> }`, []string{
			"test.met:1:26: empty type argument list\n" +
				"    struct Foo<T> { value Foo<> }\n" +
				"                             ^^",
		}},

		{"subslice ..= without hi", "expr", `x[..=]`, []string{
			"test.met:1:3: inclusive range (..=) requires an upper bound\n" +
				"    x[..=]\n" +
				"      ^^",
		}},
		{"subslice lo..= without hi", "expr", `x[1..=]`, []string{
			"test.met:1:4: inclusive range (..=) requires an upper bound\n" +
				"    x[1..=]\n" +
				"       ^^",
		}},
		{"subslice missing ]", "expr", `x[1..2`, []string{
			"test.met:1:6: unexpected end of file\n" +
				"    x[1..2\n" +
				"         ^",
			"test.met:1:6: unexpected end of file\n" +
				"    x[1..2\n" +
				"         ^",
		}},

		{"for in missing dotdot", "expr", `for x in 0 { 1 }`, []string{
			"test.met:1:10: expected range expression (e.g. 0..10)\n" +
				"    for x in 0 { 1 }\n" +
				"             ^",
		}},
		{"for in inclusive range without hi", "expr", `for x in 0..= { 1 }`, []string{
			"test.met:1:11: inclusive range (..=) requires an upper bound\n" +
				"    for x in 0..= { 1 }\n" +
				"              ^^",
		}},

		{"union single variant", "expr", `union Foo = Str`, []string{
			"test.met:1:13: union requires at least 2 variants\n" +
				"    union Foo = Str\n" +
				"                ^^^",
		}},
		{"union reserved word", "expr", `union Arena = Str | Int`, []string{
			"test.met:1:7: reserved word: Arena\n" +
				"    union Arena = Str | Int\n" +
				"          ^^^^^",
		}},

		{"use in expression", "expr", `use foo::bar`, []string{
			"test.met:1:1: unexpected token: expected start of an expression, got <use>\n" +
				"    use foo::bar\n" +
				"    ^^^",
		}},
		{"use after decl", "file", `fun main() void {} use foo::bar`, []string{
			"test.met:1:20: unexpected token: <use>\n" +
				"    fun main() void {} use foo::bar\n" +
				"                       ^^^",
		}},
	}

	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}

	assert := base.NewAssert(t)
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", "test", true, []rune(tt.src))
			tokens := token.Lex(source)
			parser := NewParser(tokens, NewAST(1))
			var parseOK bool
			kind := tt.kind
			if kind == "" {
				kind = "expr"
			}
			switch kind {
			case "expr":
				_, parseOK = parser.ParseExpr(0)
			case "file":
				_, parseOK = parser.ParseModule()
			default:
				t.Fatalf("unknown kind: %s", kind)
			}
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
			assert.Equal(false, parseOK, "parse should have failed")
		})
	}
}

type TestAST struct {
	*AST
	span base.Span
}

func NewTestAST() *TestAST {
	return &TestAST{AST: NewAST(1), span: base.Span{}}
}

func (a *TestAST) file(decls ...NodeID) NodeID {
	return a.NewModule("test.met", "test", true, nil, decls, a.span)
}

func (a *TestAST) file_with_imports(imports []NodeID, decls ...NodeID) NodeID {
	if imports == nil {
		imports = []NodeID{}
	}
	if decls == nil {
		decls = []NodeID{}
	}
	return a.NewModule("test.met", "test", true, imports, decls, a.span)
}

func (a *TestAST) path_(segments ...string) NodeID {
	return a.NewPath(segments, nil, a.span)
}

func (a *TestAST) path_type_args(segments []string, typeArgs ...NodeID) NodeID {
	return a.NewPath(segments, typeArgs, a.span)
}

func (a *TestAST) import_(segments ...string) NodeID {
	return a.NewImport(nil, segments, a.span)
}

func (a *TestAST) import_alias(alias string, segments ...string) NodeID {
	n := Name{alias, a.span}
	return a.NewImport(&n, segments, a.span)
}

func (a *TestAST) fun_param(name string, typ NodeID) NodeID {
	return a.NewFunParam(Name{name, a.span}, typ, a.span)
}

func (a *TestAST) fun(name string, paramsReturnAndBlock ...NodeID) NodeID {
	block := paramsReturnAndBlock[len(paramsReturnAndBlock)-1]
	returnTyp := paramsReturnAndBlock[len(paramsReturnAndBlock)-2]
	params := paramsReturnAndBlock[:len(paramsReturnAndBlock)-2]
	return a.NewFun(Name{name, a.span}, nil, params, returnTyp, block, a.span)
}

func (a *TestAST) struct_field(name string, typ NodeID) NodeID {
	return a.NewStructField(Name{name, a.span}, typ, false, a.span)
}

func (a *TestAST) mut_struct_field(name string, typ NodeID) NodeID {
	return a.NewStructField(Name{name, a.span}, typ, true, a.span)
}

func (a *TestAST) struct_(name string, fields ...NodeID) NodeID {
	if fields == nil {
		fields = []NodeID{}
	}
	return a.NewStruct(Name{name, a.span}, nil, fields, a.span)
}

func (a *TestAST) type_param(name string) NodeID {
	return a.NewTypeParam(Name{name, a.span}, nil, a.span)
}

func (a *TestAST) constrained_type_param(name string, constraint NodeID) NodeID {
	return a.NewTypeParam(Name{name, a.span}, &constraint, a.span)
}

func (a *TestAST) shape(name string, fields []NodeID, funs ...NodeID) NodeID {
	if fields == nil {
		fields = []NodeID{}
	}
	return a.NewShape(Name{name, a.span}, fields, funs, a.span)
}

func (a *TestAST) fun_decl(name string, paramsAndReturn ...NodeID) NodeID {
	returnTyp := paramsAndReturn[len(paramsAndReturn)-1]
	params := paramsAndReturn[:len(paramsAndReturn)-1]
	return a.NewFunDecl(Name{name, a.span}, nil, params, returnTyp, a.span)
}

func (a *TestAST) generic_struct(name string, typeParams []NodeID, fields ...NodeID) NodeID {
	if fields == nil {
		fields = []NodeID{}
	}
	return a.NewStruct(Name{name, a.span}, typeParams, fields, a.span)
}

func (a *TestAST) union_(name string, variants ...NodeID) NodeID {
	return a.NewUnion(Name{name, a.span}, nil, variants, a.span)
}

func (a *TestAST) generic_union(name string, typeParams []NodeID, variants ...NodeID) NodeID {
	return a.NewUnion(Name{name, a.span}, typeParams, variants, a.span)
}

func (a *TestAST) generic_fun(name string, typeParams []NodeID, paramsReturnAndBlock ...NodeID) NodeID {
	block := paramsReturnAndBlock[len(paramsReturnAndBlock)-1]
	returnTyp := paramsReturnAndBlock[len(paramsReturnAndBlock)-2]
	params := paramsReturnAndBlock[:len(paramsReturnAndBlock)-2]
	return a.NewFun(Name{name, a.span}, typeParams, params, returnTyp, block, a.span)
}

func (a *TestAST) struct_lit(struct_ NodeID, args ...NodeID) NodeID {
	if args == nil {
		args = []NodeID{}
	}
	return a.NewStructLiteral(struct_, args, a.span)
}

func (a *TestAST) binary(op BinaryOp, lhs NodeID, rhs NodeID) NodeID {
	return a.NewBinary(op, lhs, rhs, a.span)
}

func (a *TestAST) unary(op UnaryOp, expr NodeID) NodeID {
	return a.NewUnary(op, expr, a.span)
}

func (a *TestAST) field_access(base NodeID, field string) NodeID {
	return a.NewFieldAccess(base, Name{field, a.span}, nil, a.span)
}

func (a *TestAST) field_access_t(base NodeID, field string, typeArgs ...NodeID) NodeID {
	return a.NewFieldAccess(base, Name{field, a.span}, typeArgs, a.span)
}

func (a *TestAST) if_(cond NodeID, then NodeID, else_ *NodeID) NodeID {
	return a.NewIf(cond, then, else_, a.span)
}

func (a *TestAST) for_(block NodeID) NodeID {
	return a.NewFor(nil, nil, block, a.span)
}

func (a *TestAST) for_cond(cond NodeID, block NodeID) NodeID {
	return a.NewFor(nil, &cond, block, a.span)
}

func (a *TestAST) for_in(name string, iter NodeID, block NodeID) NodeID {
	binding := Name{Name: name, Span: a.span}
	return a.NewFor(&binding, &iter, block, a.span)
}

func (a *TestAST) range_(lo NodeID, hi NodeID) NodeID {
	return a.NewRange(&lo, &hi, false, a.span)
}

func (a *TestAST) range_inclusive(lo NodeID, hi NodeID) NodeID {
	return a.NewRange(&lo, &hi, true, a.span)
}

func (a *TestAST) break_() NodeID {
	return a.NewBreak(a.span)
}

func (a *TestAST) return_(expr NodeID) NodeID {
	return a.NewReturn(expr, a.span)
}

func (a *TestAST) continue_() NodeID {
	return a.NewContinue(a.span)
}

func (a *TestAST) bool_(value bool) NodeID {
	return a.NewBool(value, a.span)
}

func (a *TestAST) string_(value string) NodeID {
	return a.NewString(value, a.span)
}

func (a *TestAST) int_(value int64) NodeID {
	return a.NewInt(big.NewInt(value), a.span)
}

func (a *TestAST) rune_(value rune) NodeID {
	return a.NewRuneLiteral(uint32(value), a.span)
}

func (a *TestAST) block(exprs ...NodeID) NodeID {
	if exprs == nil {
		exprs = []NodeID{}
	}
	return a.NewBlock(exprs, a.span)
}

func (a *TestAST) str_typ() NodeID {
	return a.NewSimpleType(Name{"Str", a.span}, nil, a.span)
}

func (a *TestAST) int_typ() NodeID {
	return a.NewSimpleType(Name{"Int", a.span}, nil, a.span)
}

func (a *TestAST) bool_typ() NodeID {
	return a.NewSimpleType(Name{"Bool", a.span}, nil, a.span)
}

func (a *TestAST) arr_typ(typ NodeID, len_ int) NodeID {
	return a.NewArrayType(typ, int64(len_), a.span)
}

func (a *TestAST) fun_typ(paramsAndReturn ...NodeID) NodeID {
	returnTyp := paramsAndReturn[len(paramsAndReturn)-1]
	params := paramsAndReturn[:len(paramsAndReturn)-1]
	return a.NewFunType(params, returnTyp, a.span)
}

func (a *TestAST) slice_typ(typ NodeID) NodeID {
	return a.NewSliceType(typ, false, a.span)
}

func (a *TestAST) mut_slice_typ(typ NodeID) NodeID {
	return a.NewSliceType(typ, true, a.span)
}

func (a *TestAST) arr_lit(elems ...NodeID) NodeID {
	if elems == nil {
		elems = []NodeID{}
	}
	return a.NewArrayLiteral(elems, a.span)
}

func (a *TestAST) empty_slice() NodeID {
	return a.NewEmptySlice(a.span)
}

func (a *TestAST) index(base NodeID, index NodeID) NodeID {
	return a.NewIndex(base, index, a.span)
}

func (a *TestAST) sub_slice(target NodeID, lo *NodeID, hi *NodeID, inclusive bool) NodeID {
	range_ := a.NewRange(lo, hi, inclusive, a.span)
	return a.NewSubSlice(target, range_, a.span)
}

func (a *TestAST) void_typ() NodeID {
	return a.NewSimpleType(Name{"void", a.span}, nil, a.span)
}

func (a *TestAST) typ(name string) NodeID {
	return a.NewSimpleType(Name{name, a.span}, nil, a.span)
}

func (a *TestAST) typ_args(name string, args ...NodeID) NodeID {
	return a.NewSimpleType(Name{name, a.span}, args, a.span)
}

func (a *TestAST) ident_type_args(name string, args ...NodeID) NodeID {
	return a.NewIdent(name, args, a.span)
}

func (a *TestAST) ident(name string) NodeID {
	return a.NewIdent(name, nil, a.span)
}

func (a *TestAST) allocator_var(name string, allocator string, args ...NodeID) NodeID {
	if args == nil {
		args = []NodeID{}
	}
	return a.NewAllocatorVar(Name{name, a.span}, Name{allocator, a.span}, args, a.span)
}

func (a *TestAST) assign(lhs NodeID, rhs NodeID) NodeID {
	return a.NewAssign(lhs, rhs, a.span)
}

func (a *TestAST) var_(name string, expr NodeID) NodeID {
	return a.NewVar(Name{name, a.span}, expr, false, a.span)
}

func (a *TestAST) mut_var(name string, expr NodeID) NodeID {
	return a.NewVar(Name{name, a.span}, expr, true, a.span)
}

func (a *TestAST) call(callee NodeID, args ...NodeID) NodeID {
	if args == nil {
		args = []NodeID{}
	}
	return a.NewCall(callee, args, a.span)
}

func (a *TestAST) ref(target NodeID) NodeID {
	return a.NewRef(target, false, a.span)
}

func (a *TestAST) mut_ref(target NodeID) NodeID {
	return a.NewRef(target, true, a.span)
}

func (a *TestAST) deref(expr NodeID) NodeID {
	return a.NewDeref(expr, a.span)
}

func (a *TestAST) ref_typ(typ NodeID) NodeID {
	return a.NewRefType(typ, false, a.span)
}

func (a *TestAST) mut_ref_typ(typ NodeID) NodeID {
	return a.NewRefType(typ, true, a.span)
}

func ast_to_list(ast *AST, nodeID NodeID) []*Node {
	var nodes []*Node
	var f func(NodeID)
	f = func(nodeID NodeID) {
		node := ast.Node(nodeID)
		node.Span = base.Span{}
		switch kind := node.Kind.(type) {
		case SimpleType:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case FunParam:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case FunDecl:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case Fun:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case TypeParam:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case Shape:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case StructField:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case Struct:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case Union:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case StructLiteral:
			node.Kind = kind
		case AllocatorVar:
			kind.Name.Span = base.Span{}
			kind.Allocator.Span = base.Span{}
			node.Kind = kind
		case FieldAccess:
			kind.Field.Span = base.Span{}
			node.Kind = kind
		case Var:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case For:
			if kind.Binding != nil {
				kind.Binding.Span = base.Span{}
				node.Kind = kind
			}
		case Import:
			if kind.Alias != nil {
				a := *kind.Alias
				a.Span = base.Span{}
				kind.Alias = &a
			}
			node.Kind = kind
		case Path:
		case Ref:
		}
		nodes = append(nodes, node)
		ast.Walk(nodeID, f)
	}
	f(nodeID)
	return nodes
}
