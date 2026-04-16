package comptime

import (
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func parse(t *testing.T, src string) (*ast.AST, ast.NodeID) {
	t.Helper()
	source := base.NewSource("test.met", "test", true, []rune(src))
	tokens := token.Lex(source)
	parser := ast.NewParser(tokens, ast.NewAST(1))
	root, ok := parser.ParseModule()
	if !ok || len(parser.Diagnostics) > 0 {
		t.Fatalf("parse failed: %s", parser.Diagnostics)
	}
	return parser.AST, root
}

func moduleDecls(a *ast.AST, root ast.NodeID) []ast.NodeID {
	return base.Cast[ast.Module](a.Node(root).Kind).Decls
}

func TestCompTime(t *testing.T) {
	t.Parallel()
	testEnv := Env{
		"os":     {"darwin": true, "linux": false},
		"arch":   {"aarch64": true, "x86_64": false},
		"endian": {"little": true, "big": false},
		"tag":    {"debug": true},
	}

	t.Run("active condition inlines body", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a, root := parse(t, "#if os.darwin\nfun f() void {}\n#end")
		diags := ResolveModule(a, root, testEnv)
		assert.Equal(0, len(diags), "diagnostics: %s", diags)
		decls := moduleDecls(a, root)
		assert.Equal(1, len(decls), "expected 1 decl (inlined fun)")
		_, isFun := a.Node(decls[0]).Kind.(ast.Fun)
		assert.Equal(true, isFun, "expected Fun node")
	})

	t.Run("inactive condition removes body", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a, root := parse(t, "#if os.linux\nfun f() void {}\n#end")
		diags := ResolveModule(a, root, testEnv)
		assert.Equal(0, len(diags), "diagnostics: %s", diags)
		assert.Equal(0, len(moduleDecls(a, root)), "expected no decls")
	})

	t.Run("and with not", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a, root := parse(t, "#if os.darwin and not arch.x86_64\nfun f() void {}\n#end")
		diags := ResolveModule(a, root, testEnv)
		assert.Equal(0, len(diags), "diagnostics: %s", diags)
		assert.Equal(1, len(moduleDecls(a, root)), "expected 1 decl")
	})

	t.Run("or", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a, root := parse(t, "#if os.linux or endian.little\nfun f() void {}\n#end")
		diags := ResolveModule(a, root, testEnv)
		assert.Equal(0, len(diags), "diagnostics: %s", diags)
		assert.Equal(1, len(moduleDecls(a, root)), "expected 1 decl")
	})

	t.Run("unknown category", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a, root := parse(t, "#if foo.bar\nfun f() void {}\n#end")
		diags := ResolveModule(a, root, testEnv)
		assert.NotEqual(0, len(diags), "expected diagnostic for unknown category")
	})

	t.Run("unknown key", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a, root := parse(t, "#if os.darwim\nfun f() void {}\n#end")
		diags := ResolveModule(a, root, testEnv)
		assert.NotEqual(0, len(diags), "expected diagnostic for typo in key")
	})

	t.Run("unknown tag is inactive", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a, root := parse(t, "#if tag.something\nfun f() void {}\n#end")
		diags := ResolveModule(a, root, testEnv)
		assert.Equal(0, len(diags), "diagnostics: %s", diags)
		assert.Equal(0, len(moduleDecls(a, root)), "expected no decls")
	})

	t.Run("provided tag is active", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a, root := parse(t, "#if tag.debug\nfun f() void {}\n#end")
		diags := ResolveModule(a, root, testEnv)
		assert.Equal(0, len(diags), "diagnostics: %s", diags)
		assert.Equal(1, len(moduleDecls(a, root)), "expected 1 decl")
	})
}
