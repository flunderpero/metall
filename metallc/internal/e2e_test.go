package internal

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

		{"generic method", `
			struct Foo { value Int }
			fun Foo.get<T>(f Foo, x T) T { x }

			fun main() void {
				let f = Foo(42)
				print_int(f.get<Int>(1))
				print_str(f.get<Str>("hello"))
			}
			`, "1\nhello\n"},

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

			fun foo(@myalloc Arena) &Foo {
				new(@myalloc, Foo("hello"))
			}

			fun main() void {
				let @myalloc = Arena()
				let x = new(@myalloc, Foo("x"))
				let y = new(@myalloc, Foo("y"))
				{
					let @youralloc = Arena()
					let z = new(@youralloc, Foo("z"))
					print_str(z.one)
				}
				print_str(y.one)
				print_str(x.one)
				let w = foo(@myalloc)
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
		{"heap alloc array", `
			fun main() void {
				let @myalloc = Arena()
				mut x = new_mut(@myalloc, [5]Int(0))
				x[1] = 1
				x[2] = 2

				print_int(x[0])
				print_int(x[1])
				print_int(x[2])
			}
			`, "0\n1\n2\n"},
		// `new` returns a reference. Assigning `let y = x` where `x = new_mut(@a, Foo(...))` copies
		// the reference, not the underlying data — both variables alias the same heap memory.
		{"heap alloc struct is ref aliased", `
			struct Foo {
				mut one Str
			}

			fun main() void {
				let @a = Arena()
				mut x = new_mut(@a, Foo("hello"))
				mut y = x
				y.one = "world"
				print_str(x.one)
				print_str(y.one)
			}
			`, "world\nworld\n"},

		// Heap-allocated fixed-size array via `new` returns a reference.
		// Copying only copies the pointer — both variables alias the same heap data.
		{"heap alloc array is ref aliased", `
			fun main() void {
				let @a = Arena()
				mut x = new_mut(@a, [3]Int())
				x[0] = 42
				mut y = x
				y[0] = 99
				print_int(x[0])
				print_int(y[0])
			}
			`, "99\n99\n"},

		// `new` without `mut` returns an immutable reference — field reads work via auto-deref.
		{"heap alloc immutable struct read", `
			struct Foo {
				one Str
			}

			fun main() void {
				let @a = Arena()
				let x = new(@a, Foo("hello"))
				print_str(x.one)
			}
			`, "hello\n"},

		// `new_mut(@a, ...)` returns a mutable reference — can pass to fun taking &mut.
		{"heap alloc mut struct as param", `
			struct Foo {
				mut one Str
			}

			fun set(a &mut Foo, b Str) void {
				a.one = b
			}

			fun main() void {
				let @a = Arena()
				let x = new_mut(@a, Foo("hello"))
				set(x, "world")
				print_str(x.one)
			}
			`, "world\n"},

		// Heap-allocated immutable array: index reads work via auto-deref.
		{"heap alloc immutable array read", `
			fun main() void {
				let @a = Arena()
				let x = new_mut(@a, [3]Int())
				x[0] = 42
				let y = new(@a, [3]Int())
				print_int(x[0])
			}
			`, "42\n"},

		// Slice is a fat pointer {ptr, len}. Copying a slice only copies the fat pointer,
		// so both slices point to the same underlying data.
		{"slice copy aliases underlying data", `
			fun main() void {
				let @a = Arena()
				mut x = make(@a, []Int(3))
				x[0] = 42
				mut y = x
				y[0] = 99
				print_int(x[0])
				print_int(y[0])
			}
			`, "99\n99\n"},

		{"make slice", `
			fun main() void {
				let @myalloc = Arena()
				let size = 3
				mut x = make(@myalloc, []Int(size))
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
				mut x = make(@a, []Int(3))
				x[0] = 10
				x[1] = 20
				x[2] = 30
				let i = 2
				print_int(x[i])
				x[i] = 99
				print_int(x[i])
			}
			`, "30\n99\n"},
		// Default value fills all elements of a heap-allocated array.
		{"new array with default value", `
			fun main() void {
				let @a = Arena()
				mut x = new_mut(@a, [100]Int(77))
				print_int(x[0])
				print_int(x[50])
				print_int(x[99])
			}
			`, "77\n77\n77\n"},

		// Default value fills all elements of a heap-allocated slice.
		{"make slice with default value", `
			fun main() void {
				let @a = Arena()
				mut x = make(@a, []Int(100, 77))
				print_int(x[0])
				print_int(x[50])
				print_int(x[99])
			}
			`, "77\n77\n77\n"},

		// Without a default value, memory is uninitialized. Writing then reading works.
		{"new array no default then write", `
			fun main() void {
				let @a = Arena()
				mut x = new_mut(@a, [100]Int())
				x[99] = 42
				print_int(x[99])
			}
			`, "42\n"},

		// Without a default value on slice, writing then reading works.
		{"make slice no default then write", `
			fun main() void {
				let @a = Arena()
				mut x = make(@a, []Int(100))
				x[99] = 42
				print_int(x[99])
			}
			`, "42\n"},

		// Struct array with default value fills all 100 elements.
		{"new struct array with default value", `
			struct Foo {
				one Int
				two Str
			}

			fun main() void {
				let @a = Arena()
				mut x = new_mut(@a, [100]Foo(Foo(42, "hello")))
				print_int(x[0].one)
				print_str(x[0].two)
				print_int(x[50].one)
				print_str(x[50].two)
				print_int(x[99].one)
				print_str(x[99].two)
			}
			`, "42\nhello\n42\nhello\n42\nhello\n"},

		// Struct slice with default value fills all 100 elements.
		{"make struct slice with default value", `
			struct Foo {
				one Int
				two Str
			}

			fun main() void {
				let @a = Arena()
				mut x = make(@a, []Foo(100, Foo(42, "hello")))
				print_int(x[0].one)
				print_str(x[0].two)
				print_int(x[50].one)
				print_str(x[50].two)
				print_int(x[99].one)
				print_str(x[99].two)
			}
			`, "42\nhello\n42\nhello\n42\nhello\n"},

		// Array of mutable references with default value. All elements alias the
		// same heap struct, so mutating through one is visible through the others.
		{"new ref array with default value", `
			struct Foo {
				mut one Int
				two Str
			}

			fun main() void {
				let @a = Arena()
				let def = new_mut(@a, Foo(42, "hello"))
				mut x = new_mut(@a, [3]&mut Foo(def))
				print_int(x[0].one)
				print_int(x[2].one)
				x[0].one = 99
				print_int(x[1].one)
				print_int(x[2].one)
			}
			`, "42\n42\n99\n99\n"},

		// Slice of mutable references with default value. All elements alias the
		// same heap struct, so mutating through one is visible through the others.
		{"make ref slice with default value", `
			struct Foo {
				mut one Int
				two Str
			}

			fun main() void {
				let @a = Arena()
				let def = new_mut(@a, Foo(42, "hello"))
				mut x = make(@a, []&mut Foo(3, def))
				print_int(x[0].one)
				print_int(x[2].one)
				x[0].one = 99
				print_int(x[1].one)
				print_int(x[2].one)
			}
			`, "42\n42\n99\n99\n"},

		{"allocate multidimensional array", `
			fun main() void {
				let @a = Arena()
				mut m = new_mut(@a, [2][3]Int())
				m[0] = [10, 20, 30]
				m[1] = [40, 50, 60]
				print_int(m[0][1])
				print_int(m[1][2])
			}
			`, "20\n60\n"},

		{"make multidimensional slice", `
			fun main() void {
				let @a = Arena()
				mut m = make(@a, [][]Int(2, []))
				m[0] = make(@a, []Int(3, 20))
				m[1] = make(@a, []Int(3, 60))
				print_int(m[0][1])
				print_int(m[1][2])
			}
			`, "20\n60\n"},

		{"empty slice resets slice", `
			fun main() void {
				let @a = Arena()
				mut x = make(@a, []Int(3, 42))
				print_int(x[1])
				x = []
				print_int(x.len)
			}
			`, "42\n0\n"},

		{"int arithmetic", `
			fun main() void {
				print_int(120 + 3)
				print_int(44 - 2)
				print_int(3 * 3)
				print_int(9 / 3)
				print_int(10 % 3)
				print_int(Int(U8(10) % 3))
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

		// Integer types: I8, I16, I32, U8, U16, U32, U64.
		{"I8", `
			fun main() void {
				let x = I8(127)
				let y = I8(0) - I8(1)
				print_int(Int(x))
				print_int(Int(y))
			}
			`, "127\n-1\n"},
		{"I16", `
			fun main() void {
				let x = I16(32767)
				let y = I16(0) - I16(1)
				print_int(Int(x))
				print_int(Int(y))
			}
			`, "32767\n-1\n"},
		{"I32", `
			fun main() void {
				let x = I32(2147483647)
				let y = I32(0) - I32(1)
				print_int(Int(x))
				print_int(Int(y))
			}
			`, "2147483647\n-1\n"},
		{"U8", `
			fun main() void {
				let x = U8(255)
				let y = U8(0)
				print_int(Int(x))
				print_int(Int(y))
			}
			`, "255\n0\n"},
		{"U16", `
			fun main() void {
				let x = U16(65535)
				let y = U16(0)
				print_int(Int(x))
				print_int(Int(y))
			}
			`, "65535\n0\n"},
		{"U32", `
			fun main() void {
				let x = U32(4294967295)
				let y = U32(0)
				print_int(Int(x))
				print_int(Int(y))
			}
			`, "4294967295\n0\n"},
		{"U64", `
			fun main() void {
				let x = U64(18446744073709551615)
				let y = U64(0)
				print_uint(x)
				print_uint(y)
			}
			`, "18446744073709551615\n0\n"},
		// Integer type conversions.
		{"widening U8 to I32", `
			fun main() void {
				let x = U8(200)
				let y = I32(x)
				print_int(Int(y))
			}
			`, "200\n"},
		{"widening I8 to I32", `
			fun main() void {
				let x = I8(42)
				let y = I32(x)
				print_int(Int(y))
			}
			`, "42\n"},
		{"sign-extend I8 to I32", `
			fun main() void {
				let x = I8(0) - I8(1)
				let y = I32(x)
				print_int(Int(y))
			}
			`, "-1\n"},

		// Arithmetic on integer types.
		{"I32 arithmetic", `
			fun main() void {
				print_int(Int(I32(10) + I32(20)))
				print_int(Int(I32(50) - I32(8)))
				print_int(Int(I32(6) * I32(7)))
				print_int(Int(I32(100) / I32(3)))
			}
			`, "30\n42\n42\n33\n"},
		{"U8 division is unsigned", `
			fun main() void {
				print_int(Int(U8(255) / U8(2)))
			}
			`, "127\n"},

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
		{"Str from []U8 slice", `
			fun Str.byte_len(self Str) Int { self.data.len }
			fun main() void {
				let @a = Arena()
				let data = make(@a, []U8(3, U8(65)))
				let s = Str(data)
				print_str(s)
				print_int(s.byte_len())
			}
			`, "AAA\n3\n"},
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
			opts := CompileOpts{
				Listener:         timing,
				Output:           outputPath,
				KeepIR:           true,
				LLVMPasses:       "verify," + DefaultLLVMPasses,
				AddressSanitizer: true,
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
	}{
		{"int divide by zero", `
			fun main() void {
				1 / 0
			}
		`},
		{"int modulo by zero", `
			fun main() void {
				1 % 0
			}
		`},
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
			opts := CompileOpts{
				Listener:         timing,
				Output:           outputPath,
				KeepIR:           true,
				LLVMPasses:       "verify," + DefaultLLVMPasses,
				AddressSanitizer: true,
			}
			exitCode, _, err := CompileAndRun(t.Context(), source, opts)
			timing.Log(t)
			assert.NoError(err)
			assert.NotEqual(0, exitCode, "expected non-zero exit code (trap)")
		})
	}
}
