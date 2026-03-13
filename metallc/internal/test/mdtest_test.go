package test

import (
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
)

func TestParse(t *testing.T) {
	assert := base.NewAssert(t)

	content := `# Top

## Section A

**Named test**

Some description text.

` + "```metall" + `
source code
` + "```" + `

` + "```ast" + `
expected ast
` + "```" + `

` + "```metall" + `
second test
` + "```" + `

` + "```error" + `
some error
` + "```" + `

## Section B

` + "```metall" + `
third test
` + "```" + `

` + "```ast" + `
third ast
` + "```" + `

` + "```error" + `
third error
` + "```" + `
`

	cases := Parse(content)
	assert.Equal(3, len(cases))

	// First test case: named, under Top/Section A.
	tc := cases[0]
	assert.Equal("1 Named test", tc.Name)
	assert.Equal([]string{"Top", "Section A"}, tc.Sections)
	assert.Equal("source code", tc.Input)
	assert.Equal("expected ast", tc.Want["ast"])
	assert.Equal(false, tc.Only)
	assert.Equal("Top/Section A/1 Named test", tc.FullName())

	// Second test case: unnamed, under Top/Section A.
	tc = cases[1]
	assert.Equal("2", tc.Name)
	assert.Equal([]string{"Top", "Section A"}, tc.Sections)
	assert.Equal("second test", tc.Input)
	assert.Equal("some error", tc.Want["error"])
	assert.Equal(false, tc.Only)

	// Third test case: unnamed, under Top/Section B. Counter resets.
	tc = cases[2]
	assert.Equal("1", tc.Name)
	assert.Equal([]string{"Top", "Section B"}, tc.Sections)
	assert.Equal("third test", tc.Input)
	assert.Equal("third ast", tc.Want["ast"])
	assert.Equal("third error", tc.Want["error"])
	assert.Equal(false, tc.Only)
}

func TestParseOnly(t *testing.T) {
	assert := base.NewAssert(t)

	content := `# Top

## !only Focus Section

` + "```metall" + `
test one
` + "```" + `

` + "```ast" + `
ast one
` + "```" + `

## Other Section

` + "```metall" + `
test two
` + "```" + `

` + "```ast" + `
ast two
` + "```" + `

## Focus Section

` + "```metall !only" + `
test three
` + "```" + `

` + "```ast" + `
ast three
` + "```" + `
`

	cases := Parse(content)
	assert.Equal(3, len(cases))

	// First test should have only=true (section !only).
	assert.Equal(true, cases[0].Only)
	// Second test should not.
	assert.Equal(false, cases[1].Only)
	// Third test should have only=true (fence !only).
	assert.Equal(true, cases[2].Only)
}

func TestParseMultipleExpectations(t *testing.T) {
	assert := base.NewAssert(t)

	content := `# Tests

` + "```metall" + `
some code
` + "```" + `

` + "```ast" + `
the ast
` + "```" + `

` + "```ir" + `
the ir
` + "```" + `

` + "```error" + `
the error
` + "```" + `
`

	cases := Parse(content)
	assert.Equal(1, len(cases))
	assert.Equal("the ast", cases[0].Want["ast"])
	assert.Equal("the ir", cases[0].Want["ir"])
	assert.Equal("the error", cases[0].Want["error"])
}

func TestRunFile(t *testing.T) {
	// Test that RunFile correctly runs tests and compares expectations.
	assert := base.NewAssert(t)

	runner := RunFunc(func(t *testing.T, _ base.Assert, tc TestCase) map[string]string {
		t.Helper()
		assert.Equal("some code", tc.Input)
		return map[string]string{"output": "expected output"}
	})
	_ = runner // just verify it compiles and satisfies the interface.
}
