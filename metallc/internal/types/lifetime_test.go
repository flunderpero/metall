package types

import (
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/modules"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestLifetimeMD(t *testing.T) {
	mdtest.RunFile(t, mdtest.File("lifetime_test.md"), mdtest.RunFunc(runLifetimeTest))
}

func runLifetimeTest(_ *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	source := base.NewSource("test.met", "test", true, []rune(tc.Input))
	tokens := token.Lex(source)
	parser := ast.NewParser(tokens, ast.NewAST(1))
	exprID, _ := parser.ParseExpr(0)
	parser.Roots = append(parser.Roots, exprID)
	assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)

	preludeAST, _ := ast.PreludeAST(true)
	e := NewEngine(parser.AST, preludeAST, &modules.ModuleResolution{}, nil)
	e.Query(exprID)
	assert.Equal(0, len(e.diagnostics), "type check failed:\n%s", e.diagnostics)

	a := NewLifetimeAnalyzer(e.ast, e.scopeGraph, e.Env())
	a.Debug = base.NewStdoutDebug("lifetime")
	a.Check(exprID)

	return map[string]string{
		"error": a.Diagnostics.String(),
	}
}
