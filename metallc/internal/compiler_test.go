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
		{"happy path", `fun main() void { print_str("hello") }`, "hello\n"},
		{"int constant", `fun main() void { print_int(123) }`, "123\n"},

		{"str variable", `fun main() void { let a = "hello" print_str(a) }`, "hello\n"},
		{"int variable", `fun main() void { let a = 123 print_int(a) }`, "123\n"},
		{"bool variable", `fun main() void { let a = true print_bool(a) }`, "true\n"},
		{"mut variable same scope", `fun main() void { mut a = 123 print_int(a) a = 456 print_int(a) }`, "123\n456\n"},

		{"int function", `fun get() Int { 123 } fun main() void { print_int(get()) }`, "123\n"},
		{"str function", `fun get() Str { "hello" } fun main() void { print_str(get()) }`, "hello\n"},
		{"bool function", `fun get() Bool { true } fun main() void { print_bool(get()) }`, "true\n"},
		{"fun int param", `fun foo(a Int) Int { a } fun main() void { print_int(foo(123)) }`, "123\n"},
		{"fun str param", `fun foo(a Str) Str { a } fun main() void { let s = foo("hello") print_str(s) }`, "hello\n"},
		{"fun bool param", `fun foo(a Bool) Bool { a } fun main() void { print_bool(foo(true)) }`, "true\n"},

		{"block expr", `fun main() void { let s = { "hello" } print_str(s) }`, "hello\n"},
		{"var block expr is void", `fun main() void { print_str("hello") let a = 123 }`, "hello\n"},
		{"assign block expr is void", `fun main() void { print_str("hello") mut a = 123 a = 321 }`, "hello\n"},

		{"if expr", `fun main() void { let a = if true { 123 } else { 321 } print_int(a) }`, "123\n"},
		{"if expr else", `fun main() void { let a = if false { 123 } else { 321 } print_int(a) }`, "321\n"},
		{
			"if expr var",
			`fun main() void { mut a = 1 if true { a = 123 } else { a = 321 } print_int(a) }`,
			"123\n",
		},
		{"nested if exper", `
			fun main() void {
				let a = if true {
					if false { 1 } else { 123 }
				} else {
					2
				}
				print_int(a)
			}
			`, "123\n"},

		{"ref/deref", `fun main() void { mut a = 123 mut b = &a print_int(*b) *b = 321 print_int(a) }`, "123\n321\n"},
		{"nested ref/deref", `
			fun main() void { 
				mut a = 123 
				mut b = &a
				mut c = &b
				print_int(*b)
				*b = 321 
				print_int(a)
				**c = 111
				print_int(a)
			}`, "123\n321\n111\n"},
		{"assign through mut ref parameter", `
			fun foo(mut a &Int) void { 
				print_int(*a)
				*a = 321 
			}
			fun main() void { 
				mut a = 123 
				foo(&a)
				print_int(a)
			}
			`, "123\n321\n"},

		{"struct", `
			struct Planet {
				name Str
				diameter Int
			}

			fun main() void {
				let earth = Planet("Earth", 12500)
				print_str(earth.name)
				print_int(earth.diameter)

				earth.name = "Mother"
				earth.diameter = 12742
				print_str(earth.name)
				print_int(earth.diameter)
			}
			`, "Earth\n12500\nMother\n12742\n"},

		{"forward declare", `
			fun main() void {
				print_int(foo())
			}

			fun foo() Int {
				123
			}

			`, "123\n"},
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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := base.NewSource("test.met", []rune(tt.src))
			reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
			outputPath := "./.build/" + reg.ReplaceAllString(tt.name, "_")
			opts := CompileOpts{Listener: nil, Output: outputPath, KeepIR: true}
			exitCode, output, err := CompileAndRun(t.Context(), source, opts)
			assert.NoError(err)
			assert.Equal(0, exitCode)
			assert.Equal(tt.wantOutput, output)
		})
	}
}
