package types

import (
	"fmt"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/macros"
	"github.com/flunderpero/metall/metallc/internal/modules"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
	"github.com/flunderpero/metall/metallc/internal/token"
)

var macroModuleContents = map[string]string{} //nolint:gochecknoglobals

var macroModuleFilePaths = map[string]string{ //nolint:gochecknoglobals
	"hello_macro":         "local/hello_macro.met",
	"no_macro_funs_macro": "local/no_macro_funs_macro.met",
	"param_macro":         "local/param_macro.met",
}

func TestMacroMD(t *testing.T) {
	mdtest.RunFile(t, mdtest.File("macro_test.md"), mdtest.RunFunc(runMacroTest))
}

func runMacroTest(_ *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	results := map[string]string{}

	for lang, content := range tc.Want {
		if tag, ok := strings.CutPrefix(lang, "module."); ok {
			macroModuleContents[tag] = content
			results[lang] = content
		}
	}
	if tc.Input == "" {
		return results
	}

	files := map[string]string{}
	for tag, path := range macroModuleFilePaths {
		if content, ok := macroModuleContents[tag]; ok {
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
		if !ok {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return []byte(content), nil
	}
	moduleResolution, diags := modules.ResolveModules(a, "local", []string{}, readFile)
	assert.Equal(0, len(diags), "module resolution failed:\n%s", diags)

	var expander MacroExpander
	if expanderOutput, ok := tc.Want["expander"]; ok {
		results["expander"] = expanderOutput
		expander = func(_ string, _ string, _ []macros.MacroArg) (string, error) {
			return expanderOutput, nil
		}
	} else if _, ok := tc.Want["expander_error"]; ok {
		results["expander_error"] = tc.Want["expander_error"]
		expander = func(_ string, _ string, _ []macros.MacroArg) (string, error) {
			return "", fmt.Errorf("%s", tc.Want["expander_error"])
		}
	}

	preludeAST, _ := ast.PreludeAST(true)
	e := NewEngine(a, preludeAST, moduleResolution, expander)
	e.Query(moduleID)

	if _, ok := tc.Want["error"]; ok {
		results["error"] = e.diagnostics.String()
	} else {
		assert.Equal(0, len(e.diagnostics), "unexpected errors:\n%s", e.diagnostics)
	}

	return results
}
