package types

import (
	"fmt"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/comptime"
	"github.com/flunderpero/metall/metallc/internal/modules"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
	"github.com/flunderpero/metall/metallc/internal/token"
)

// moduleFilePaths maps module tags to their file system paths.
var moduleFilePaths = map[string]string{ //nolint:gochecknoglobals
	"lib":      "lib/lib.met",
	"lib_test": "lib/lib_test.met",
	"generic":  "lib/generic.met",
	"local":    "local/hello.met",
	"shapes":   "lib/shapes.met",
}

// moduleContents is populated by the setup test case from the MD file.
var moduleContents = map[string]string{} //nolint:gochecknoglobals

func TestModuleMD(t *testing.T) {
	mdtest.RunFile(t, mdtest.File("module_test.md"), mdtest.RunFunc(runModuleTest))
}

func runModuleTest(_ *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	results := map[string]string{}

	// Setup test: extract module contents from `module.*` Want blocks.
	for lang, content := range tc.Want {
		if tag, ok := strings.CutPrefix(lang, "module."); ok {
			moduleContents[tag] = content
			results[lang] = content
		}
	}
	if tc.Input == "" {
		return results
	}

	// Provide all external modules to every test.
	files := map[string]string{}
	for tag, path := range moduleFilePaths {
		if content, ok := moduleContents[tag]; ok {
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
	moduleResolution, diags := modules.ResolveModules(a, "local", []string{"lib"}, comptime.Env{}, readFile)
	assert.Equal(0, len(diags), "module resolution failed:\n%s", diags)

	preludeAST, _ := ast.PreludeAST(true)
	e := NewEngine(a, preludeAST, moduleResolution, nil)
	e.Query(moduleID)

	_, wantError := tc.Want["error"]
	_, wantBindings := tc.Want["bindings"]

	if wantError {
		results["error"] = e.diagnostics.String()
		return results
	}

	assert.Equal(0, len(e.diagnostics), "unexpected errors:\n%s", e.diagnostics)

	if wantBindings {
		results["bindings"] = e.Env().DebugBindings(moduleID)
	}

	return results
}
