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

		{"int arithmetic", `
			fun main() void {
				print_int(120 + 3)
				print_int(44 - 2)
				print_int(3 * 3)
				print_int(9 / 3)
			}

			`, "123\n42\n9\n3\n"},

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
			fun Foo.get_x(self Foo) Int { self.x }
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
			source := base.NewSource("test.met", []rune(tt.src))
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
			assert.Equal(0, exitCode)
			assert.Equal(tt.wantOutput, output)
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
			source := base.NewSource("test.met", []rune(tt.src))
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
