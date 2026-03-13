package ast

import (
	"slices"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestParserMD(t *testing.T) {
	mdtest.RunFile(t, mdtest.File("parser_test.md"), mdtest.RunFunc(runParserMDTest))
}

func runParserMDTest(_ *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	results := map[string]string{}

	_, hasAST := tc.Want["ast"]
	_, hasError := tc.Want["error"]
	isModule := slices.Contains(tc.Tags, "module")

	source := base.NewSource("test.met", "test", true, []rune(tc.Input))
	tokens := token.Lex(source)
	parser := NewParser(tokens, NewAST(1))

	var root NodeID
	var ok bool
	if isModule {
		root, ok = parser.ParseModule()
	} else {
		root, ok = parser.ParseExpr(0)
	}

	if hasError {
		results["error"] = parser.Diagnostics.String()
	} else {
		assert.Equal(0, len(parser.Diagnostics), "diagnostics: %s", parser.Diagnostics)
		assert.Equal(true, ok, "parse failed")
	}

	if hasAST {
		assert.Equal(true, ok, "parse failed, cannot produce AST debug output")
		results["ast"] = parser.Debug(root, true, 0, true)
	}

	return results
}
