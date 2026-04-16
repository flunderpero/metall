package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func parseModule(src string, name string, main bool) (*ast.AST, ast.NodeID) {
	source := base.NewSource(name+".met", name, main, []rune(src))
	tokens := token.Lex(source)
	parser := ast.NewParser(tokens, ast.NewAST(1))
	id, ok := parser.ParseModule()
	if !ok || len(parser.Diagnostics) > 0 {
		panic(fmt.Sprintf("failed to parse module %s: %s", name, parser.Diagnostics))
	}
	return parser.AST, id
}

func memFS(files map[string]string) ReadFileFn {
	return func(path string) ([]byte, error) {
		content, ok := files[path]
		if !ok {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return []byte(content), nil
	}
}

func TestResolveModules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		src          string
		files        map[string]string
		projectRoot  string
		includePaths []string
		wantImports  map[string]string
		wantModules  []string
	}{
		{
			name:         "single import from include path",
			src:          `use std.collections.map`,
			files:        map[string]string{"lib/std/collections/map.met": `fun get() void {}`},
			projectRoot:  "/project",
			includePaths: []string{"lib"},
			wantImports:  map[string]string{"map": "std::collections::map"},
			wantModules:  []string{"main", "std::collections::map"},
		},
		{
			name:         "aliased import",
			src:          `use m = std.collections.map`,
			files:        map[string]string{"lib/std/collections/map.met": `fun get() void {}`},
			projectRoot:  "/project",
			includePaths: []string{"lib"},
			wantImports:  map[string]string{"m": "std::collections::map"},
			wantModules:  []string{"main", "std::collections::map"},
		},
		{
			name:         "local import",
			src:          `use local.util`,
			files:        map[string]string{"/project/util.met": `fun helper() void {}`},
			projectRoot:  "/project",
			includePaths: nil,
			wantImports:  map[string]string{"util": "local::util"},
			wantModules:  []string{"main", "local::util"},
		},
		{
			name:         "local import with alias",
			src:          `use u = local.util`,
			files:        map[string]string{"/project/util.met": `fun helper() void {}`},
			projectRoot:  "/project",
			includePaths: nil,
			wantImports:  map[string]string{"u": "local::util"},
			wantModules:  []string{"main", "local::util"},
		},
		{
			name: "two modules import same dependency",
			src:  `use std.a use std.b`,
			files: map[string]string{
				"lib/std/a.met":      `use std.shared`,
				"lib/std/b.met":      `use std.shared`,
				"lib/std/shared.met": `fun common() void {}`,
			},
			projectRoot:  "/project",
			includePaths: []string{"lib"},
			wantImports:  map[string]string{"a": "std::a", "b": "std::b"},
			wantModules:  []string{"main", "std::a", "std::b", "std::shared"},
		},
		{
			name:         "no imports",
			src:          `fun main() void {}`,
			files:        nil,
			projectRoot:  "/project",
			includePaths: nil,
			wantImports:  map[string]string{},
			wantModules:  []string{"main"},
		},
		{
			name: "transitive import",
			src:  `use std.a`,
			files: map[string]string{
				"lib/std/a.met": `use std.b fun foo() void {}`,
				"lib/std/b.met": `fun bar() void {}`,
			},
			projectRoot:  "/project",
			includePaths: []string{"lib"},
			wantImports:  map[string]string{"a": "std::a"},
			wantModules:  []string{"main", "std::a", "std::b"},
		},
	}

	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	assert := base.NewAssert(t)
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, mainID := parseModule(tt.src, "main", true)
			res, diags := ResolveModules(a, tt.projectRoot, tt.includePaths, nil, memFS(tt.files))
			assert.Equal(0, len(diags), "diagnostics: %s", diags)
			mainImports := res.Imports[mainID]
			assert.Equal(len(tt.wantImports), len(mainImports), "import count")
			for wantName, wantPath := range tt.wantImports {
				depID, ok := mainImports[wantName]
				assert.Equal(true, ok, "import %q not found", wantName)
				if ok {
					depMod := base.Cast[ast.Module](res.AST.Node(depID).Kind)
					assert.Equal(wantPath, depMod.Name)
				}
			}
			moduleNames := make([]string, 0, len(res.AST.Roots))
			for _, root := range res.AST.Roots {
				mod := base.Cast[ast.Module](res.AST.Node(root).Kind)
				moduleNames = append(moduleNames, mod.Name)
			}
			for _, want := range tt.wantModules {
				assert.Contains(moduleNames, want, "module %q not in AST roots", want)
			}
			assert.Equal(len(tt.wantModules), len(moduleNames), "module count")
		})
	}
}

func TestResolveModulesErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		src          string
		files        map[string]string
		projectRoot  string
		includePaths []string
		want         []string
	}{
		{
			name:         "module not found",
			src:          `use std.missing`,
			files:        nil,
			projectRoot:  "/project",
			includePaths: []string{"lib"},
			want:         []string{"module not found: std::missing (include paths: lib)"},
		},
		{
			name:         "local module not found",
			src:          `use local.missing`,
			files:        nil,
			projectRoot:  "/project",
			includePaths: nil,
			want:         []string{"module not found: local::missing (project root: /project)"},
		},
		{
			name:         "duplicate import",
			src:          `use std.a use std.a`,
			files:        map[string]string{"lib/std/a.met": `fun foo() void {}`},
			projectRoot:  "/project",
			includePaths: []string{"lib"},
			want:         []string{"duplicate import: std::a"},
		},
		{
			name: "duplicate import name",
			src:  `use std.a use other.a`,
			files: map[string]string{
				"lib/std/a.met":   `fun foo() void {}`,
				"lib/other/a.met": `fun bar() void {}`,
			},
			projectRoot:  "/project",
			includePaths: []string{"lib"},
			want:         []string{"import name `a` already used"},
		},
		{
			name: "circular import",
			src:  `use std.a`,
			files: map[string]string{
				"lib/std/a.met": `use std.b`,
				"lib/std/b.met": `use std.a`,
			},
			projectRoot:  "/project",
			includePaths: []string{"lib"},
			want:         []string{"circular import: std::a"},
		},
	}

	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	assert := base.NewAssert(t)
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, _ := parseModule(tt.src, "main", true)
			_, diags := ResolveModules(a, tt.projectRoot, tt.includePaths, nil, memFS(tt.files))
			assert.Equal(len(tt.want), len(diags), "diagnostic count: %s", diags)
			for i, want := range tt.want {
				if i < len(diags) {
					assert.Contains(diags[i].Message, want)
				}
			}
		})
	}
}
