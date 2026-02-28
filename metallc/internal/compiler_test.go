package internal

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
)

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
				let x = new @myalloc Foo("hello")
				&x
			}

			fun main() void {
				alloc @myalloc = Arena()
				let x = new @myalloc Foo("x")
				let y = new @myalloc Foo("y")
				{
					alloc @youralloc = Arena()
					let z = new @youralloc Foo("z")
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
				alloc @myalloc = Arena()
				mut x = new @myalloc [5]Int()
				x[1] = 1
				x[2] = 2

				print_int(x[0])
				print_int(x[1])
				print_int(x[2])
			}
			`, "0\n1\n2\n"},
	}

	assert := base.NewAssert(t)
	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	_ = os.RemoveAll(".build")
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
			opts := CompileOpts{
				Listener:         nil,
				Output:           outputPath,
				KeepIR:           true,
				LLVMPasses:       "verify," + DefaultLLVMPasses,
				AddressSanitizer: true,
			}
			exitCode, output, err := CompileAndRun(t.Context(), source, opts)
			assert.NoError(err)
			assert.Equal(0, exitCode)
			assert.Equal(tt.wantOutput, output)
		})
	}
}
