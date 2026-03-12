package compiler

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
	"github.com/flunderpero/metall/metallc/internal/types"
)

// timingListener records the wall-clock duration of each compiler phase.
// After CompileAndRun returns, call [timingListener.Log] to print every
// step that took longer than 10 ms.
type timingListener struct {
	last  time.Time
	steps []step
}

type step struct {
	name     string
	duration time.Duration
}

func newTimingListener() *timingListener {
	return &timingListener{last: time.Now(), steps: nil}
}

func (l *timingListener) OnLex(_ []token.Token) bool {
	l.record("lex")
	return true
}

func (l *timingListener) OnParse(_ *ast.AST, _ ast.NodeID, _ base.Diagnostics) bool {
	l.record("parse")
	return true
}

func (l *timingListener) OnTypeCheck(_ *types.Engine, _ base.Diagnostics) bool {
	l.record("typecheck")
	return true
}

func (l *timingListener) OnLifetimeCheck(_ *types.LifetimeCheck, _ base.Diagnostics) bool {
	l.record("lifetime")
	return true
}

func (l *timingListener) OnIRGen(_ string) bool {
	l.record("irgen")
	return true
}

func (l *timingListener) OnOptimizeIR() bool {
	l.record("optimize")
	return true
}

func (l *timingListener) OnLink() bool {
	l.record("link")
	return true
}

func (l *timingListener) OnRun(_ int, _ string) bool {
	l.record("run")
	return true
}

// Log prints every step that took longer than 10ms.
func (l *timingListener) Log(t *testing.T) {
	t.Helper()
	for _, s := range l.steps {
		if s.duration >= 10*time.Millisecond {
			t.Logf("%-12s %s", s.name, s.duration.Round(time.Millisecond))
		}
	}
}

// Total returns a display string of all step durations (for debugging).
func (l *timingListener) Total() string {
	var total time.Duration
	parts := make([]string, 0, len(l.steps))
	for _, s := range l.steps {
		total += s.duration
		parts = append(parts, fmt.Sprintf("%s=%s", s.name, s.duration.Round(time.Millisecond)))
	}
	return fmt.Sprintf("total=%s (%s)", total.Round(time.Millisecond), strings.Join(parts, ", "))
}

func (l *timingListener) record(name string) {
	now := time.Now()
	l.steps = append(l.steps, step{name, now.Sub(l.last)})
	l.last = now
}

func TestCompile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		src        string
		wantOutput string
	}{
		{"print str", `fun main() void { print_str("hello") }`, "hello\n"},
		{"int literal", `fun main() void { print_int(123) }`, "123\n"},

		{"str var", `fun main() void { let x = "hello" print_str(x) }`, "hello\n"},
		{"int var", `fun main() void { let x = 123 print_int(x) }`, "123\n"},
		{"bool var", `fun main() void { let x = true print_bool(x) }`, "true\n"},
		{"mut var reassign", `fun main() void { mut x = 123 print_int(x) x = 456 print_int(x) }`, "123\n456\n"},

		{"fun returns int", `fun foo() Int { 123 } fun main() void { print_int(foo()) }`, "123\n"},
		{"fun returns str", `fun foo() Str { "hello" } fun main() void { print_str(foo()) }`, "hello\n"},
		{"fun returns bool", `fun foo() Bool { true } fun main() void { print_bool(foo()) }`, "true\n"},
		{"fun with int param", `fun foo(a Int) Int { a } fun main() void { print_int(foo(123)) }`, "123\n"},
		{
			"fun with str param",
			`fun foo(a Str) Str { a } fun main() void { let x = foo("hello") print_str(x) }`,
			"hello\n",
		},
		{"fun with bool param", `fun foo(a Bool) Bool { a } fun main() void { print_bool(foo(true)) }`, "true\n"},
		{"fun with return", `fun foo() Int { return 123 } fun main() void { print_int(foo()) }`, "123\n"},
		{
			"fun with return struct", `
			struct Foo { one Str }
			fun foo() Foo { return Foo("hello") } 
			fun main() void { print_str(foo().one) }`,
			"hello\n",
		},
		{
			"fun with multiple return", `
			fun foo(a Int) Int { 
				if a != 2 {
					if a == 0 {
						return 100
					} else {
						"just some expr"
					}
					return 101
				}
				return a + 200
			} 

			fun main() void { 
				print_int(foo(0)) 
				print_int(foo(1)) 
				print_int(foo(2)) 
			}`,
			"100\n101\n202\n",
		},

		{
			"return void in if", `
			fun foo(x Int) void {
				if x > 0 {
					return void
				}
				print_int(x)
			}
			fun main() void { foo(1) foo(0) }`,
			"0\n",
		},

		{"free flowing fun", `
			fun double(a Int) Int { a + a }

			fun math(a Int, f fun(Int) Int) Int { f(a) }

			fun main() void {
				print_int(math(7, double))
				let local_math = math
				let local_double = double
				print_int(local_math(9, double))
			}
			`, "14\n18\n"},
		{"free flowing fun reassign", `
			fun double(a Int) Int { a + a }
			fun triple(a Int) Int { a + a + a }

			fun main() void {
				mut f = double
				print_int(f(5))
				f = triple
				print_int(f(5))
			}
			`, "10\n15\n"},
		{"free flowing fun qualified call", `
			struct Foo { x Int }
			fun Foo.get(self Foo) Int { self.x }

			fun main() void {
				let f = Foo(42)
				print_int(Foo.get(f))
				let g = Foo.get
				print_int(g(f))
			}
			`, "42\n42\n"},
		{"free flowing fun return", `
			fun double(a Int) Int { a + a }

			fun get_double() fun(Int) Int { double }

			fun main() void {
				let f = get_double()
				print_int(f(5))
			}
			`, "10\n"},

		{"free flowing fun in struct field", `
			fun double(a Int) Int { a + a }

			struct Foo { one fun(Int) Int }

			fun main() void {
				let x = Foo(double)
				print_int(x.one(5))
				let y = x.one
				print_int(y(5))
			}
			`, "10\n10\n"},
		{"free flowing fun higher order", `
			fun double(a Int) Int { a + a }
			fun apply_twice(f fun(Int) Int, x Int) Int { f(f(x)) }
			fun main() void {
				print_int(apply_twice(double, 3))
			}
			`, "12\n"},
		{"free flowing fun recursive as value", `
			fun factorial(n Int) Int {
				if n == 0 { 1 } else { n * factorial(n - 1) }
			}
			fun apply(f fun(Int) Int, x Int) Int { f(x) }
			fun main() void {
				print_int(apply(factorial, 5))
			}
			`, "120\n"},
		{"free flowing fun builtin as value", `
			fun apply(f fun(Int) void, x Int) void { f(x) }
			fun main() void {
				let f = print_int
				f(42)
				apply(print_int, 99)
			}
			`, "42\n99\n"},
		{"free flowing fun if branches", `
			fun double(a Int) Int { a + a }
			fun triple(a Int) Int { a + a + a }
			fun pick(use_double Bool) fun(Int) Int {
				if use_double { double } else { triple }
			}
			fun main() void {
				let f = pick(true)
				print_int(f(5))
				let g = pick(false)
				print_int(g(5))
			}
			`, "10\n15\n"},

		{"nested fun", `
			fun foo() Int { 321 }
			fun main() void {
				fun foo() Int { 
					fun foo() Int { 123 }
					foo()
				}
				print_int(foo())
			}
			`, "123\n"},

		{"nested fun mutual recursion", `
			fun main() void {
				fun is_even(n Int) Bool {
					if n == 0 { true } else { is_odd(n - 1) }
				}
				fun is_odd(n Int) Bool {
					if n == 0 { false } else { is_even(n - 1) }
				}
				print_bool(is_even(4))
				print_bool(is_odd(4))
			}
			`, "true\nfalse\n"},

		{"nested fun as value", `
			fun apply(f fun(Int) Int, x Int) Int { f(x) }
			fun main() void {
				fun double(a Int) Int { a + a }
				print_int(apply(double, 21))
			}
			`, "42\n"},

		{"nested struct", `
			struct Foo { x Int }
			fun Foo.foo(self Foo) Int { self.x }

			fun main() void {
				struct Foo { x Int y Str }
				fun Foo.bar(f Foo) Int { f.x + 1 }
				let f = Foo(42, "hello")
				print_int(f.bar())
				print_str(f.y)
			}
			`, "43\nhello\n"},

		{"nested fun same name different scopes", `
			fun main() void {
				fun foo() Int {
					fun helper() Int { 10 }
					helper()
				}
				fun bar() Int {
					fun helper() Int { 20 }
					helper()
				}
				print_int(foo() + bar())
			}
			`, "30\n"},

		{"block expression", `fun main() void { let x = { "hello" } print_str(x) }`, "hello\n"},
		{"var expr is void", `fun main() void { print_str("hello") let x = 123 }`, "hello\n"},
		{"assign expr is void", `fun main() void { print_str("hello") mut x = 123 x = 321 }`, "hello\n"},

		{"if true branch", `fun main() void { let x = if true { 123 } else { 321 } print_int(x) }`, "123\n"},
		{"if false branch", `fun main() void { let x = if false { 123 } else { 321 } print_int(x) }`, "321\n"},
		{
			"if assigns to mut var",
			`fun main() void { mut x = 1 if true { x = 123 } else { x = 321 } print_int(x) }`,
			"123\n",
		},
		{"nested if", `
			fun main() void {
				let x = if true {
					if false { 1 } else { 123 }
				} else {
					2
				}
				print_int(x)
			}
			`, "123\n"},

		{
			"ref deref",
			`fun main() void { mut x = 123 mut y = &mut x print_int(y.*) y.* = 321 print_int(x) }`,
			"123\n321\n",
		},
		{"nested ref deref", `
			fun main() void { 
				mut x = 123 
				mut y = &mut x
				mut z = &mut y
				print_int(y.*)
				y.* = 321 
				print_int(x)
				z.*.* = 111
				print_int(x)
			}`, "123\n321\n111\n"},
		{"deref assign through &mut param", `
			fun foo(a &mut Int) void { 
				print_int(a.*)
				a.* = 321 
			}
			fun main() void { 
				mut x = 123 
				foo(&mut x)
				print_int(x)
			}
			`, "123\n321\n"},

		{"struct field read and write", `
			struct Foo {
				mut one Str
				mut two Int
			}

			fun main() void {
				mut x = Foo("hello", 123)
				print_str(x.one)
				print_int(x.two)

				x.one = "bye"
				x.two = 456
				print_str(x.one)
				print_int(x.two)
			}
			`, "hello\n123\nbye\n456\n"},

		{"struct as value param", `
			struct Foo {
				one Str
			}

			fun foo(a Foo) void {
				print_str(a.one)
			}

			fun main() void {
				let x = Foo("hello")
				foo(x)
			}
			`, "hello\n"},

		{"struct &ref and &mut ref params", `
			struct Foo {
				mut one Str
			}

			fun foo(a &Foo) void {
				print_str(a.one)
			}

			fun bar(a &mut Foo, b Str) void {
				a.one = b
			}

			fun main() void {
				mut x = Foo("hello")
				foo(&x)

				bar(&mut x, "bye")
				foo(&x)
			}
			`, "hello\nbye\n"},

		{"fun returns struct", `
			struct Foo {
				one Str
			}

			fun foo() Foo {
				Foo("hello")
			}

			fun main() void {
				let x = foo()
				print_str(x.one)
			}
			`, "hello\n"},

		{"nested struct field access", `
			struct Foo {
				mut one Str
			}

			struct Bar {
				one Foo
				mut two Foo
			}

			fun main() void {
				mut x = Bar(Foo("hello"), Foo("world"))
				print_str(x.one.one)
				print_str(x.two.one)
				x.two.one = "bye"
				print_str(x.two.one)
			}
			`, "hello\nworld\nbye\n"},

		// Assigning a struct to another variable copies by value.
		{"struct value copy", `
			struct Foo {
				mut one Str
			}

			fun main() void {
				mut x = Foo("hello")
				mut y = x
				y.one = "world"
				print_str(x.one)
				print_str(y.one)
			}
			`, "hello\nworld\n"},

		// Assigning a struct to a field copies by value.
		{"nested struct value copy", `
			struct Foo {
				mut one Str
			}

			struct Bar {
				mut one Foo
			}

			fun main() void {
				mut x = Bar(Foo("hello"))
				mut y = Foo("world")
				x.one = y
				y.one = "bye"
				print_str(x.one.one)
				print_str(y.one)
			}
			`, "world\nbye\n"},

		{"struct with &ref field", `
			struct Wrapper {
				one Int
				two &Int
			}

			fun main() void {
				mut x = 42
				let y = Wrapper(1, &x)
				print_int(y.one)
				print_int(y.two.*)
				x = 99
				print_int(y.two.*)
			}
			`, "1\n42\n99\n"},

		// Ref alias sees mutations through the original binding.
		{"struct ref alias sees mutation", `
			struct Foo {
				mut one Str
			}

			fun main() void {
				mut x = Foo("hello")
				let y = &x
				let z = y
				x.one = "world"
				print_str(z.one)
			}
			`, "world\n"},

		{"struct in if else", `
			struct Foo {
				mut one Str
			}

			fun main() void {
				let x = if true { Foo("hello") } else { Foo("world") }
				print_str(x.one)
				mut y = if false { Foo("hello") } else { Foo("world") }
				print_str(y.one)
				y.one = "bye"
				print_str(y.one)
			}
			`, "hello\nworld\nbye\n"},

		{"struct reassign from if else", `
			struct Foo {
				one Str
			}

			fun main() void {
				mut x = Foo("hello")
				print_str(x.one)
				x = if true { Foo("world") } else { Foo("bye") }
				print_str(x.one)
			}
			`, "hello\nworld\n"},

		{"struct from block as arg", `
			struct Foo {
				one Str
			}

			fun foo(a Foo) void {
				print_str(a.one)
			}

			fun main() void {
				foo({ Foo("hello") })
			}
			`, "hello\n"},

		{"generic struct", `
			struct Pair<T> {
				first T
				second T
			}

			fun main() void {
				let p = Pair<Int>(10, 20)
				print_int(p.first)
				print_int(p.second)
			}
			`, "10\n20\n"},

		{"generic fun", `
			struct Box<T> { value T }
			fun id<T>(x T) T { x }

			fun main() void {
				print_int(id<Int>(42))
				print_str(id<Str>("hello"))
				let b = id<Box<Int>>(Box<Int>(99))
				print_int(b.value)
			}
			`, "42\nhello\n99\n"},

		{"generic fun as value", `
			fun id<T>(x T) T { x }

			fun main() void {
				let f = id<Int>
				print_int(f(42))
				let g = id<Str>
				print_str(g("hello"))
			}
			`, "42\nhello\n"},

		{"method on generic struct", `
			struct Foo<T> { one T }
			fun Foo.bar<T>(f Foo<T>, a T, b Bool) T { if b { return f.one } a }

			fun main() void {
				let x = Foo<Int>(42)
				print_int(x.bar(99, true))
				print_int(x.bar(99, false))
			}
			`, "42\n99\n"},

		{"generic method", `
			struct Foo { value Int }
			fun Foo.get<T>(f Foo, x T) T { x }

			fun main() void {
				let f = Foo(42)
				print_int(f.get<Int>(1))
				print_str(f.get<Str>("hello"))
			}
			`, "1\nhello\n"},

		{"generic method with extra type param on generic struct", `
			struct Foo<T> { one T }
			fun Foo.bar<T, U>(f Foo<T>, a U) U { a }

			fun main() void {
				let x = Foo<Int>(42)
				print_str(x.bar<Str>("hello"))
				print_int(x.bar<Int>(99))
			}
			`, "hello\n99\n"},

		{"method on generic struct accesses field", `
			struct Pair<A, B> { first A second B }
			fun Pair.get_first<A, B>(p Pair<A, B>) A { p.first }
			fun Pair.get_second<A, B>(p Pair<A, B>) B { p.second }

			fun main() void {
				let p = Pair<Int, Str>(42, "hello")
				print_int(p.get_first())
				print_str(p.get_second())
			}
			`, "42\nhello\n"},

		{"method on multi-param generic struct with extra type param", `
			struct Pair<A, B> { first A second B }
			fun Pair.swap<A, B>(p Pair<A, B>) Pair<B, A> { Pair<B, A>(p.second, p.first) }
			fun Pair.map_first<A, B, C>(p Pair<A, B>, f fun(A) C) Pair<C, B> { Pair<C, B>(f(p.first), p.second) }

			fun to_str(x Int) Str { "mapped" }

			fun main() void {
				let p = Pair<Int, Str>(42, "hello")
				let s = p.swap()
				print_str(s.first)
				print_int(s.second)
				let m = p.map_first<Str>(to_str)
				print_str(m.first)
				print_str(m.second)
			}
			`, "hello\n42\nmapped\nhello\n"},

		{"generic method chain", `
			struct Wrap<T> { inner T }
			fun Wrap.unwrap<T>(w Wrap<T>) T { w.inner }

			fun main() void {
				let w = Wrap<Wrap<Int>>(Wrap<Int>(99))
				let inner = w.unwrap()
				print_int(inner.unwrap())
			}
			`, "99\n"},

		{"generic struct method calls generic fun", `
			struct Box<T> { value T }
			fun id<T>(x T) T { x }
			fun Box.get_id<T>(b Box<T>) T { id<T>(b.value) }

			fun main() void {
				let b = Box<Int>(7)
				print_int(b.get_id())
			}
			`, "7\n"},

		{"generic shadowing", `
			struct Box<T> { value T }
			fun id<T>(x T) T { x }

			fun main() void {
				print_int(id<Int>(1))
				print_int(Box<Int>(2).value)
				{
					struct Box<T> { value T value2 T }
					fun id<T>(x T) T { x }
					print_int(id<Int>(3))
					print_int(Box<Int>(4, 5).value2)
				}
			}
			`, "1\n2\n3\n5\n"},

		{"generic struct method called from generic fun", `
			struct Box<V> {
				mut items []V
			}

			fun Box.len<V>(b &Box<V>) Int {
				b.items.len
			}

			fun wrap<V>(@a Arena, v V) Int {
				let items = @a.slice<V>(2, v)
				let b = @a.new_mut<Box<V>>(Box<V>(items))
				b.len()
			}

			fun main() void {
				let @a = Arena()
				print_int(wrap<Str>(@a, "x"))
			}
			`, "2\n"},

		{"forward declared fun", `
			fun main() void {
				print_int(foo())
			}

			fun foo() Int {
				123
			}

			`, "123\n"},

		{"heap alloc with arena", `
			struct Foo {
				one Str
			}

			fun foo(@a Arena) &Foo {
				@a.new<Foo>(Foo("hello"))
			}

			fun main() void {
				let @a = Arena()
				let x = @a.new<Foo>(Foo("x"))
				let y = @a.new<Foo>(Foo("y"))
				{
					let @b = Arena()
					let z = @b.new<Foo>(Foo("z"))
					print_str(z.one)
				}
				print_str(y.one)
				print_str(x.one)
				let w = foo(@a)
				print_str(w.one)
			}
			`, "z\ny\nx\nhello\n"},

		{"int array", `
			fun main() void {
				let x = [1, 2, 3]
				print_int(x[2])
				print_int(x[1])
				print_int(x[0])
			}
			`, "3\n2\n1\n"},
		{"array index with variable", `
			fun main() void {
				mut x = [10, 20, 30]
				let i = 1
				print_int(x[i])
				x[i] = 99
				print_int(x[i])
			}
			`, "20\n99\n"},

		{"struct array", `
			struct Foo {
				one Str
			}

			fun main() void {
				let x = [
					Foo("x"),
					Foo("y"),
					Foo("z"),
				]
				print_str(x[2].one)
				print_str(x[1].one)
				print_str(x[0].one)
			}
			`, "z\ny\nx\n"},
		{"nested array", `
			fun main() void {
				let x = [
					[1, 2],
					[3, 4],
					[5, 6],
				]
				let y = x[0]
				print_int(y[1])
				let z = x[1]
				print_int(z[0])
				let w = x[2]
				print_int(w[1])
			}
			`, "2\n3\n6\n"},
		{"array in struct", `
			struct Foo {
				one [3]Int
			}

			fun main() void {
				let x = Foo([1, 2, 3])
				print_int(x.one[1])
			}
			`, "2\n"},
		{"array with refs", `
			struct Foo {
			 	one Str
			}

			fun main() void {
				let x = Foo("x")
				let y = Foo("y")
				let z = [x, y]
				print_str(z[1].one)
				print_str(z[0].one)

				let w = 1
				let v = 2
				let u = [&w, &v]
				print_int(u[1].*)
				print_int(u[0].*)
			}
			`, "y\nx\n2\n1\n"},
		{"array index write", `
			fun main() void {
				mut x = [1, 2, 3]
				print_int(x[1])
				x[1] = 4
				print_int(x[1])
			}
			`, "2\n4\n"},
		{"array struct index write", `
			struct Foo { one Str }

			fun main() void {
				mut x = [Foo("x"), Foo("y")]
				print_str(x[0].one)
				x[0] = Foo("z")
				print_str(x[0].one)
			}
			`, "x\nz\n"},
		{"array of refs index write", `
			struct Foo { one Str }

			fun main() void {
				let x = Foo("x")
				let y = Foo("y")
				let z = Foo("z")
				mut w = [&x, &y]
				print_str(w[0].one)
				w[0] = &z
				print_str(w[0].one)
			}
			`, "x\nz\n"},
		{"heap alloc slice", `
			fun main() void {
				let @a = Arena()
				let x = @a.slice_mut<Int>(5, 0)
				x[1] = 1
				x[2] = 2

				print_int(x[0])
				print_int(x[1])
				print_int(x[2])
			}
			`, "0\n1\n2\n"},
		// `new_mut` returns a mutable reference. Assigning `let y = x` copies the reference,
		// not the underlying data — both variables alias the same heap memory.
		{"heap alloc struct is ref aliased", `
			struct Foo {
				mut one Str
			}

			fun main() void {
				let @a = Arena()
				mut x = @a.new_mut<Foo>(Foo("hello"))
				mut y = x
				y.one = "world"
				print_str(x.one)
				print_str(y.one)
			}
			`, "world\nworld\n"},

		// Slice is a fat pointer — copying only copies the fat pointer,
		// so both variables alias the same underlying data.
		{"heap alloc slice is aliased", `
			fun main() void {
				let @a = Arena()
				let x = @a.slice_uninit_mut<Int>(3)
				x[0] = 42
				let y = x
				y[0] = 99
				print_int(x[0])
				print_int(y[0])
			}
			`, "99\n99\n"},

		// `new` returns an immutable reference — field reads work via auto-deref.
		{"heap alloc immutable struct read", `
			struct Foo {
				one Str
			}

			fun main() void {
				let @a = Arena()
				let x = @a.new<Foo>(Foo("hello"))
				print_str(x.one)
			}
			`, "hello\n"},

		// `new_mut` returns a mutable reference — can pass to fun taking &mut.
		{"heap alloc mut struct as param", `
			struct Foo {
				mut one Str
			}

			fun set(a &mut Foo, b Str) void {
				a.one = b
			}

			fun main() void {
				let @a = Arena()
				let x = @a.new_mut<Foo>(Foo("hello"))
				set(x, "world")
				print_str(x.one)
			}
			`, "world\n"},

		// Heap-allocated slices: mut and immutable variants.
		{"heap alloc slice read", `
			fun main() void {
				let @a = Arena()
				let x = @a.slice_uninit_mut<Int>(3)
				x[0] = 42
				let y = @a.slice_uninit<Int>(3)
				print_int(x[0])
			}
			`, "42\n"},

		// Slice is a fat pointer {ptr, len}. Copying a slice only copies the fat pointer,
		// so both slices point to the same underlying data.
		{"slice copy aliases underlying data", `
			fun main() void {
				let @a = Arena()
				let x = @a.slice_uninit_mut<Int>(3)
				x[0] = 42
				let y = x
				y[0] = 99
				print_int(x[0])
				print_int(y[0])
			}
			`, "99\n99\n"},

		{"make slice", `
			fun main() void {
				let @a = Arena()
				let size = 3
				let x = @a.slice_uninit_mut<Int>(size)
				x[0] = 10
				x[1] = 20
				x[2] = 30

				print_int(x[0])
				print_int(x[1])
				print_int(x[2])
				print_int(x.len)
			}
			`, "10\n20\n30\n3\n"},

		{"slice index with variable", `
			fun main() void {
				let @a = Arena()
				let x = @a.slice_uninit_mut<Int>(3)
				x[0] = 10
				x[1] = 20
				x[2] = 30
				let i = 2
				print_int(x[i])
				x[i] = 99
				print_int(x[i])
			}
			`, "30\n99\n"},
		// Default value fills all elements of a heap-allocated slice.
		{"make slice with default value", `
			fun main() void {
				let @a = Arena()
				let x = @a.slice<Int>(100, 77)
				print_int(x[0])
				print_int(x[50])
				print_int(x[99])
			}
			`, "77\n77\n77\n"},

		// Default value fills all elements via make.
		{"make slice with default value 2", `
			fun main() void {
				let @a = Arena()
				let x = @a.slice<Int>(100, 77)
				print_int(x[0])
				print_int(x[50])
				print_int(x[99])
			}
			`, "77\n77\n77\n"},

		// Without a default value, memory is uninitialized. Writing then reading works.
		{"make uninit then write", `
			fun main() void {
				let @a = Arena()
				let x = @a.slice_uninit_mut<Int>(100)
				x[99] = 42
				print_int(x[99])
			}
			`, "42\n"},

		// Without a default value on slice, writing then reading works.
		{"make uninit slice then write", `
			fun main() void {
				let @a = Arena()
				let x = @a.slice_uninit_mut<Int>(100)
				x[99] = 42
				print_int(x[99])
			}
			`, "42\n"},

		// Struct slice with default value fills all 100 elements.
		{"make struct slice with default value 1", `
			struct Foo {
				one Int
				two Str
			}

			fun main() void {
				let @a = Arena()
				let x = @a.slice<Foo>(100, Foo(42, "hello"))
				print_int(x[0].one)
				print_str(x[0].two)
				print_int(x[50].one)
				print_str(x[50].two)
				print_int(x[99].one)
				print_str(x[99].two)
			}
			`, "42\nhello\n42\nhello\n42\nhello\n"},

		// Struct slice with default value fills all 100 elements.
		{"make struct slice with default value 2", `
			struct Foo {
				one Int
				two Str
			}

			fun main() void {
				let @a = Arena()
				let x = @a.slice<Foo>(100, Foo(42, "hello"))
				print_int(x[0].one)
				print_str(x[0].two)
				print_int(x[50].one)
				print_str(x[50].two)
				print_int(x[99].one)
				print_str(x[99].two)
			}
			`, "42\nhello\n42\nhello\n42\nhello\n"},

		// Slice of mutable references with default value. All elements alias the
		// same heap struct, so mutating through one is visible through the others.
		{"make ref slice with default value 1", `
			struct Foo {
				mut one Int
				two Str
			}

			fun main() void {
				let @a = Arena()
				let def = @a.new_mut<Foo>(Foo(42, "hello"))
				let x = @a.slice_mut<&mut Foo>(3, def)
				print_int(x[0].one)
				print_int(x[2].one)
				x[0].one = 99
				print_int(x[1].one)
				print_int(x[2].one)
			}
			`, "42\n42\n99\n99\n"},

		// Slice of mutable references with default value (variant 2).
		{"make ref slice with default value 2", `
			struct Foo {
				mut one Int
				two Str
			}

			fun main() void {
				let @a = Arena()
				let def = @a.new_mut<Foo>(Foo(42, "hello"))
				let x = @a.slice_mut<&mut Foo>(3, def)
				print_int(x[0].one)
				print_int(x[2].one)
				x[0].one = 99
				print_int(x[1].one)
				print_int(x[2].one)
			}
			`, "42\n42\n99\n99\n"},

		{"allocate multidimensional slice", `
			fun main() void {
				let @a = Arena()
				let m = @a.slice_mut<[]Int>(2, [])
				m[0] = @a.slice<Int>(3, 10)
				m[1] = @a.slice<Int>(3, 40)
				print_int(m[0][1])
				print_int(m[1][2])
			}
			`, "10\n40\n"},

		{"make multidimensional slice", `
			fun main() void {
				let @a = Arena()
				let m = @a.slice_mut<[]Int>(2, [])
				m[0] = @a.slice<Int>(3, 20)
				m[1] = @a.slice<Int>(3, 60)
				print_int(m[0][1])
				print_int(m[1][2])
			}
			`, "20\n60\n"},

		{"empty slice resets slice", `
			fun main() void {
				let @a = Arena()
				mut x = @a.slice<Int>(3, 42)
				print_int(x[1])
				x = []
				print_int(x.len)
			}
			`, "42\n0\n"},

		{"update array element in place", `
			fun main() void {
				struct Foo { mut one Int }
				mut a = [Foo(1)]
				a[0].one = 42
				print_int(a[0].one)
			}
			`, "42\n"},

		{"update slice element in place", `
			fun main() void {
				let @a = Arena()
				struct Foo { mut one Int }
				let a = @a.slice_uninit_mut<Foo>(1)
				a[0] = Foo(1)
				a[0].one = 42
				print_int(a[0].one)
			}
			`, "42\n"},

		{"ref to array element", `
			fun main() void {
				struct Foo { mut one Int }
				mut a = [Foo(1)]
				mut b = &mut a[0]
				b.one = 42
				print_int(a[0].one)
			}
			`, "42\n"},

		{"ref to slice element", `
			fun main() void {
				let @a = Arena()
				struct Foo { mut one Int }
				let a = @a.slice_uninit_mut<Foo>(1)
				a[0] = Foo(1)
				let b = &mut a[0]
				b.one = 42
				print_int(a[0].one)
			}
			`, "42\n"},

		{"subslice exclusive range", `
			fun main() void {
				let arr = [10, 20, 30, 40, 50]
				let s = arr[1..3]
				print_int(s.len)
				print_int(s[0])
				print_int(s[1])
			}
			`, "2\n20\n30\n"},
		{"subslice inclusive range", `
			fun main() void {
				let arr = [10, 20, 30, 40, 50]
				let s = arr[1..=3]
				print_int(s.len)
				print_int(s[0])
				print_int(s[2])
			}
			`, "3\n20\n40\n"},
		{"subslice open lo", `
			fun main() void {
				let arr = [10, 20, 30, 40, 50]
				let s = arr[..2]
				print_int(s.len)
				print_int(s[0])
				print_int(s[1])
			}
			`, "2\n10\n20\n"},
		{"subslice open hi", `
			fun main() void {
				let arr = [10, 20, 30, 40, 50]
				let s = arr[3..]
				print_int(s.len)
				print_int(s[0])
				print_int(s[1])
			}
			`, "2\n40\n50\n"},
		{"subslice of slice", `
			fun main() void {
				let @a = Arena()
				let sl = @a.slice_uninit_mut<Int>(5)
				sl[0] = 100
				sl[1] = 200
				sl[2] = 300
				sl[3] = 400
				sl[4] = 500
				let s = sl[2..4]
				print_int(s.len)
				print_int(s[0])
				print_int(s[1])
			}
			`, "2\n300\n400\n"},
		{"mutate array through subslice", `
			fun main() void {
				mut arr = [10, 20, 30, 40, 50]
				mut s = arr[1..4]
				s[0] = 99
				s[2] = 88
				print_int(arr[1])
				print_int(arr[3])
			}
			`, "99\n88\n"},
		{"mutate slice through subslice", `
			fun main() void {
				let @a = Arena()
				let sl = @a.slice_uninit_mut<Int>(4)
				sl[0] = 1
				sl[1] = 2
				sl[2] = 3
				sl[3] = 4
				let sub = sl[1..3]
				sub[0] = 77
				sub[1] = 88
				print_int(sl[1])
				print_int(sl[2])
			}
			`, "77\n88\n"},

		{"int arithmetic", `
			fun main() void {
				print_int(120 + 3)
				print_int(44 - 2)
				print_int(3 * 3)
				print_int(9 / 3)
				print_int(10 % 3)
				print_int((U8(10) % 3).to_int())
			}
			`, "123\n42\n9\n3\n1\n1\n"},

		{"bool operators", `
			fun main() void {
				print_bool(1 == 2)
				print_bool(1 != 2)
				print_bool(true == false)
				print_bool(true != false)

				print_bool(1 == 2 and 3 == 3)
				print_bool(1 == 2 or 3 == 3)

				print_bool(not true)
			}
			`, "false\ntrue\nfalse\ntrue\nfalse\ntrue\nfalse\n"},

		{"int comparison operators", `
			fun main() void {
				print_bool(1 < 2)
				print_bool(2 < 1)
				print_bool(1 <= 1)
				print_bool(1 <= 0)
				print_bool(2 > 1)
				print_bool(1 > 2)
				print_bool(1 >= 1)
				print_bool(0 >= 1)
				print_bool(U8(1) < 2)
			}
			`, "true\nfalse\ntrue\nfalse\ntrue\nfalse\ntrue\nfalse\ntrue\n"},

		{"bitwise I8", `
			fun main() void {
				let a = I8(90)
				let b = I8(60)
				print_int((a & b).to_int())
				print_int((a | b).to_int())
				print_int((a ^ b).to_int())
				print_int((a << I8(1)).to_int())
				print_int((a >> I8(2)).to_int())
				print_int((~a).to_int())
				let c = I8(0) - I8(100)
				print_int((c >> I8(2)).to_int())
				print_int((~c).to_int())
			}
			`, "24\n126\n102\n-76\n22\n-91\n-25\n99\n"},
		{"bitwise I16", `
			fun main() void {
				let a = I16(90)
				let b = I16(60)
				print_int((a & b).to_int())
				print_int((a | b).to_int())
				print_int((a ^ b).to_int())
				print_int((a << I16(4)).to_int())
				print_int((a >> I16(2)).to_int())
				print_int((~a).to_int())
				let c = I16(0) - I16(1000)
				print_int((c >> I16(3)).to_int())
				print_int((~c).to_int())
			}
			`, "24\n126\n102\n1440\n22\n-91\n-125\n999\n"},
		{"bitwise I32", `
			fun main() void {
				let a = I32(65280)
				let b = I32(4080)
				print_int((a & b).to_int())
				print_int((a | b).to_int())
				print_int((a ^ b).to_int())
				print_int((a << I32(4)).to_int())
				print_int((a >> I32(8)).to_int())
				print_int((~a).to_int())
				let c = I32(0) - I32(1)
				print_int((c >> I32(16)).to_int())
				print_int((~c).to_int())
			}
			`, "3840\n65520\n61680\n1044480\n255\n-65281\n-1\n0\n"},
		{"bitwise Int", `
			fun main() void {
				let a = 3735928559
				let b = 4294901760
				print_int(a & b)
				print_int(a | b)
				print_int(a ^ b)
				print_int(1 << 32)
				print_int(a >> 16)
				print_int(~a)
				let c = 0 - 1
				print_int(c >> 32)
				print_int(~c)
			}
			`, "3735879680\n4294950639\n559070959\n4294967296\n57005\n-3735928560\n-1\n0\n"},
		{"bitwise U8", `
			fun main() void {
				let a = U8(172)
				let b = U8(58)
				print_uint((a & b).to_u64())
				print_uint((a | b).to_u64())
				print_uint((a ^ b).to_u64())
				print_uint((a << U8(1)).to_u64())
				print_uint((a >> U8(3)).to_u64())
				print_uint((~a).to_u64())
			}
			`, "40\n190\n150\n88\n21\n83\n"},
		{"bitwise U16", `
			fun main() void {
				let a = U16(43981)
				let b = U16(255)
				print_uint((a & b).to_u64())
				print_uint((a | b).to_u64())
				print_uint((a ^ b).to_u64())
				print_uint((a << U16(4)).to_u64())
				print_uint((a >> U16(8)).to_u64())
				print_uint((~a).to_u64())
			}
			`, "205\n44031\n43826\n48336\n171\n21554\n"},
		{"bitwise U32", `
			fun main() void {
				let a = U32(3735928559)
				let b = U32(4294901760)
				print_uint((a & b).to_u64())
				print_uint((a | b).to_u64())
				print_uint((a ^ b).to_u64())
				print_uint((U32(1) << U32(16)).to_u64())
				print_uint((a >> U32(16)).to_u64())
				print_uint((~a).to_u64())
			}
			`, "3735879680\n4294950639\n559070959\n65536\n57005\n559038736\n"},
		{"bitwise U64", `
			fun main() void {
				let a = U64(16045690984503098046)
				let b = U64(18446744069414584320)
				print_uint(a & b)
				print_uint(a | b)
				print_uint(a ^ b)
				print_uint(U64(1) << U64(48))
				print_uint(a >> U64(32))
				print_uint(~a)
			}
			`, "16045690981097406464\n18446744072820275902\n2401053091722869438\n281474976710656\n3735928559\n2401053089206453569\n"},

		{"conditional for loop", `
			fun main() void {
				mut x = 0
				for x != 3 {
					print_int(x)
					x = x + 1
				}
			}
			`, "0\n1\n2\n"},
		{"unconditional for loop", `
			fun main() void {
				mut x = 0
				for {
					x = x + 1
					if x == 4 {
						break
					}
					if x == 2 {
						continue
					}
					print_int(x)
				}
			}
			`, "1\n3\n"},
		{"for-in range", `
			fun main() void {
				for i in 0..5 {
					print_int(i)
				}
			}
			`, "0\n1\n2\n3\n4\n"},
		{"for-in range inclusive", `
			fun main() void {
				for i in 0..=3 {
					print_int(i)
				}
			}
			`, "0\n1\n2\n3\n"},
		{"for-in range with expressions", `
			fun main() void {
				let start = 2
				let end = 5
				for i in start..end {
					print_int(i)
				}
			}
			`, "2\n3\n4\n"},
		{"for-in range with break", `
			fun main() void {
				for i in 0..10 {
					if i == 3 {
						break
					}
					print_int(i)
				}
			}
			`, "0\n1\n2\n"},
		{"for-in range with continue", `
			fun main() void {
				for i in 0..5 {
					if i == 2 {
						continue
					}
					print_int(i)
				}
			}
			`, "0\n1\n3\n4\n"},
		{"for-in range zero iterations", `
			fun main() void {
				for i in 5..5 {
					print_int(i)
				}
				print_int(99)
			}
			`, "99\n"},

		// Integer types and conversions.
		{"integer types", `
			fun main() void {
				print_int(I8(127).to_int())
				print_int((I8(0) - I8(1)).to_int())
				print_int(I16(32767).to_int())
				print_int(I32(2147483647).to_int())
				print_int(U8(255).to_int())
				print_int(U16(65535).to_int())
				print_int(U32(4294967295).to_int())
				print_uint(U64(18446744073709551615))
			}
			`, "127\n-1\n32767\n2147483647\n255\n65535\n4294967295\n18446744073709551615\n"},
		{"widening conversions", `
			fun main() void {
				let x = I8(42)
				print_int(x.to_i16().to_int())
				print_int(x.to_i32().to_int())
				print_int(x.to_int())
				print_uint(U8(200).to_u16().to_u32().to_u64())
				print_int(U8(200).to_i32().to_int())
				print_int((I8(0) - I8(1)).to_i32().to_int())
			}
			`, "42\n42\n42\n200\n200\n-1\n"},
		{"wrapping conversions", `
			fun main() void {
				print_int(I32(200).to_i8_wrapping().to_int())
				print_int(2147483648.to_i32_wrapping().to_int())
				print_int(U16(300).to_u8_wrapping().to_int())
				print_int(U64(4294967296).to_u32_wrapping().to_int())
				print_int((I8(0) - I8(1)).to_u8_wrapping().to_int())
				print_uint((0 - 1).to_u64_wrapping())
			}
			`, "-56\n-2147483648\n44\n0\n255\n18446744073709551615\n"},
		{"clamped conversions", `
			fun main() void {
				print_int(I32(200).to_i8_clamped().to_int())
				print_int((I32(0) - I32(200)).to_i8_clamped().to_int())
				print_int(U16(300).to_u8_clamped().to_int())
				print_int(U16(100).to_u8_clamped().to_int())
				print_uint((0 - 1).to_u64_clamped())
				print_uint(42.to_u64_clamped())
				print_int(U8(200).to_i8_clamped().to_int())
				print_int(U8(50).to_i8_clamped().to_int())
			}
			`, "127\n-128\n255\n100\n0\n42\n127\n50\n"},

		// Arithmetic on integer types.
		{"typed int arithmetic", `
			fun main() void {
				print_int((I32(10) + I32(20)).to_int())
				print_int((I32(50) - I32(8)).to_int())
				print_int((I32(6) * I32(7)).to_int())
				print_int((I32(100) / I32(3)).to_int())
				print_int((U8(255) / U8(2)).to_int())
			}
			`, "30\n42\n42\n33\n127\n"},

		// Rune type.
		{"rune literal and to_u32", `
			fun main() void {
				print_uint('a'.to_u32().to_u64())
				print_uint('z'.to_u32().to_u64())
				print_uint('é'.to_u32().to_u64())
			}
			`, "97\n122\n233\n"},
		{"rune comparison", `
			fun main() void {
				print_bool('a' == 'a')
				print_bool('a' != 'b')
				print_bool('a' == 'b')
			}
			`, "true\ntrue\nfalse\n"}, //nolint:dupword
		{"rune arithmetic", `
			fun main() void {
				let next = 'a' + 1
				print_uint(next.to_u32().to_u64())
				let diff = 'z' - 'a'
				print_uint(diff.to_u32().to_u64())
			}
			`, "98\n25\n"},
		{"rune let binding", `
			fun main() void {
				let r = 'x'
				print_uint(r.to_u32().to_u64())
			}
			`, "120\n"},

		// Method syntax.
		{"method call on struct", `
			struct Foo { x Int }
			fun Foo.get_x(self Foo) Int { self.x }
			fun main() void {
				let f = Foo(42)
				print_int(f.get_x())
			}
			`, "42\n"},
		{"method call with args", `
			struct Foo { x Int }
			fun Foo.add(self Foo, y Int) Int { self.x + y }
			fun main() void {
				let f = Foo(10)
				print_int(f.add(32))
			}
			`, "42\n"},
		{"method call on &ref", `
			struct Foo { x Int }
			fun Foo.get_x(self &Foo) Int { self.x }
			fun main() void {
				let f = Foo(42)
				let r = &f
				print_int(r.get_x())
			}
			`, "42\n"},
		{"direct qualified call", `
			struct Foo { x Int }
			fun Foo.add(self Foo, y Int) Int { self.x + y }
			fun main() void {
				let f = Foo(10)
				print_int(Foo.add(f, 32))
			}
			`, "42\n"},

		{"method call on Int", `
			fun Int.double(self Int) Int { self + self }
			fun main() void {
				let x = 21
				print_int(x.double())
			}
			`, "42\n"},
		{"Str.byte_len method", `
			fun Str.byte_len(self Str) Int { self.data.len }
			fun main() void {
				let s = "hello"
				print_int(s.byte_len())
				print_int("".byte_len())
				print_int("abc".byte_len())
			}
			`, "5\n0\n3\n"},
		{"shape field access", `
			shape HasPair { one Str two Int }
			struct Pair { one Str two Int }
			fun first<T HasPair>(t T) Str { t.one }
			fun main() void {
				let p = Pair("hello", 42)
				print_str(first<Pair>(p))
			}
			`, "hello\n"},
		{"shape method call", `
			shape Showable {
				fun Showable.show(self Showable) Str
			}
			struct Guitar {
				name Str
			}
			fun Guitar.show(g Guitar) Str { g.name }
			fun display<T Showable>(t T) Str { t.show() }
			fun main() void {
				print_str(display<Guitar>(Guitar("Telecaster")))
			}
			`, "Telecaster\n"},

		{
			"import local module", `
			use local::e2e

			fun main() void {
				e2e::say_hello()

				mut f = e2e::Foo(123)
				f.print()

				f.one = 321
				e2e::Foo.print(f)
			}
			`, "hello\n123\n321\n",
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
	if err := os.MkdirAll(".build", 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		name := strings.TrimSpace(strings.ReplaceAll(tt.name, "!"+"only", ""))
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := base.NewSource("test.met", "test", true, []rune(tt.src))
			reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
			outputPath := "./.build/" + reg.ReplaceAllString(name, "_")
			timing := newTimingListener()
			opts := CompileOpts{ //nolint:exhaustruct
				ProjectRoot:      ".",
				Listener:         timing,
				Output:           outputPath,
				KeepIR:           true,
				LLVMPasses:       "verify," + DefaultLLVMPasses,
				AddressSanitizer: true,
				MinimalPrelude:   true,
			}
			exitCode, output, err := CompileAndRun(t.Context(), source, opts)
			timing.Log(t)
			assert.NoError(err)
			assert.Equal(0, exitCode, "exit code")
			assert.Equal(tt.wantOutput, output, "output")
		})
	}
}

func TestCompilePanic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want string
	}{
		{"panic", `
			fun main() void {
				panic("hello")
			}
		`, "test.met:3:5: hello\n"},
		{"int divide by zero", `
			fun main() void {
				1 / 0
			}
		`, "test.met:3:5: division by zero\n"},
		{"int modulo by zero", `
			fun main() void {
				1 % 0
			}
		`, "test.met:3:5: division by zero\n"},
		{"rune arithmetic overflow", `
			fun main() void {
				'😀' * 9
			}
		`, "test.met:3:5: illegal rune\n"},
		{"rune arithmetic underflow", `
			fun main() void {
				'a' - 'b'
			}
		`, "test.met:3:5: illegal rune\n"},
		{"rune arithmetic into surrogate", `
			fun main() void {
				'퟿' + 1
			}
		`, "test.met:3:5: illegal rune\n"},
		{"array index out of bounds", `
			fun main() void {
				let arr = [10, 20, 30]
				print_int(arr[3])
			}
		`, "test.met:4:15: index out of bounds\n"},
		{"array index negative", `
			fun main() void {
				let arr = [10, 20, 30]
				let i = 0 - 1
				print_int(arr[i])
			}
		`, "test.met:5:15: index out of bounds\n"},
		{"slice index out of bounds", `
			fun main() void {
				let @a = Arena()
				let s = @a.slice<Int>(3, 0)
				print_int(s[3])
			}
		`, "test.met:5:15: index out of bounds\n"},
		{"array write index out of bounds", `
			fun main() void {
				mut arr = [10, 20, 30]
				arr[3] = 99
			}
		`, "test.met:4:5: index out of bounds\n"},
		{"subslice hi out of bounds", `
			fun main() void {
				let arr = [10, 20, 30]
				let s = arr[0..4]
			}
		`, "test.met:4:13: slice out of bounds\n"},
		{"subslice lo greater than hi", `
			fun main() void {
				let arr = [10, 20, 30]
				let s = arr[2..1]
			}
		`, "test.met:4:13: slice out of bounds\n"},
		{"subslice of slice out of bounds", `
			fun main() void {
				let @a = Arena()
				let s = @a.slice<Int>(3, 0)
				let sub = s[0..4]
			}
		`, "test.met:5:15: slice out of bounds\n"},
	}

	assert := base.NewAssert(t)
	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	if err := os.MkdirAll(".build", 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		name := strings.TrimSpace(strings.ReplaceAll(tt.name, "!"+"only", ""))
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := base.NewSource("test.met", "test", true, []rune(tt.src))
			reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
			outputPath := "./.build/" + reg.ReplaceAllString(name, "_")
			timing := newTimingListener()
			opts := CompileOpts{ //nolint:exhaustruct
				Listener:         timing,
				Output:           outputPath,
				KeepIR:           true,
				LLVMPasses:       "verify," + DefaultLLVMPasses,
				AddressSanitizer: true,
				MinimalPrelude:   true,
			}
			exitCode, stdout, err := CompileAndRun(t.Context(), source, opts)
			timing.Log(t)
			assert.NoError(err)
			assert.NotEqual(0, exitCode, "expected non-zero exit code (trap)")
			assert.Equal(tt.want, stdout)
		})
	}
}
