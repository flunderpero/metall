package types

import (
	"os"
	"slices"
	"strings"
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

// lifetimeModuleFilePaths maps module tags to their file system paths.
var lifetimeModuleFilePaths = map[string]string{ //nolint:gochecknoglobals
	"lib": "lib/lib.met",
}

// lifetimeModuleContents is populated by setup test cases from the MD file.
var lifetimeModuleContents = map[string]string{} //nolint:gochecknoglobals

func runLifetimeTest(_ *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	results := map[string]string{}

	// Collect module content from `module.*` Want blocks (setup tests).
	for lang, content := range tc.Want {
		if tag, ok := strings.CutPrefix(lang, "module."); ok {
			lifetimeModuleContents[tag] = content
			results[lang] = content
		}
	}

	if tc.Input == "" {
		return results
	}
	if slices.Contains(tc.Tags, "module") {
		return runLifetimeModuleTest(assert, tc, results)
	}
	return runLifetimeExprTest(assert, tc, results)
}

func runLifetimeExprTest(assert base.Assert, tc mdtest.TestCase, results map[string]string) map[string]string {
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
	// a.Debug = base.NewStdoutDebug("lifetime")
	a.Check(exprID)

	results["error"] = a.Diagnostics.String()
	return results
}

func runLifetimeModuleTest(assert base.Assert, tc mdtest.TestCase, results map[string]string) map[string]string {
	if tc.Input == "" {
		return results
	}

	files := map[string]string{}
	for tag, path := range lifetimeModuleFilePaths {
		if content, ok := lifetimeModuleContents[tag]; ok {
			files[path] = content
		}
	}

	source := base.NewSource("test.met", "main", true, []rune(tc.Input))
	tokens := token.Lex(source)
	a := ast.NewAST(1)
	parser := ast.NewParser(tokens, a)
	moduleID, _ := parser.ParseModule()
	assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)

	readFile := func(path string) ([]byte, error) {
		content, ok := files[path]
		if ok {
			return []byte(content), nil
		}
		return os.ReadFile("../../../" + path)
	}
	moduleResolution, diags := modules.ResolveModules(a, "local", []string{"lib"}, readFile)
	assert.Equal(0, len(diags), "module resolution failed:\n%s", diags)

	preludeAST, _ := ast.PreludeAST(true)
	e := NewEngine(a, preludeAST, moduleResolution, nil)
	e.Query(moduleID)
	assert.Equal(0, len(e.diagnostics), "type check failed:\n%s", e.diagnostics)

	lifetime := NewLifetimeAnalyzer(e.ast, e.scopeGraph, e.Env())
	lifetime.Check(moduleID)

	results["error"] = lifetime.Diagnostics.String()
	return results
}
