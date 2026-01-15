package internal

import (
	"strings"
	"testing"
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
	}

	assert := NewAssert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := NewSource("test.met", []rune(tt.src))
			outputPath := "/tmp/test_" + strings.ReplaceAll(tt.name, " ", "_")
			opts := CompileOpts{Listener: nil, Output: outputPath, KeepIR: true}
			exitCode, output, err := CompileAndRun(t.Context(), source, opts)
			assert.NoError(err)
			assert.Equal(0, exitCode)
			assert.Equal(tt.wantOutput, output)
		})
	}
}
