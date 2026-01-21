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
		{"mut variable same scope", `fun main() void { mut a = 123 print_int(a) a = 456 print_int(a) }`, "123\n456\n"},

		{"int function", `fun get() Int { 123 } fun main() void { print_int(get()) }`, "123\n"},
		{"str function", `fun get() Str { "hello" } fun main() void { print_str(get()) }`, "hello\n"},
		{"fun int param", `fun foo(a Int) Int { a } fun main() void { print_int(foo(123)) }`, "123\n"},
		{"fun str param", `fun foo(a Str) Str { a } fun main() void { let s = foo("hello") print_str(s) }`, "hello\n"},

		{"block expr", `fun main() void { let s = { "hello" } print_str(s) }`, "hello\n"},
		{"var block expr is void", `fun main() void { print_str("hello") let a = 123 }`, "hello\n"},
		{"assign block expr is void", `fun main() void { print_str("hello") mut a = 123 a = 321 }`, "hello\n"},

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
