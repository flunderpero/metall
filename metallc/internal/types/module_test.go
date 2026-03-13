package types

import (
	"fmt"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/modules"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestTypeCheckModuleOK(t *testing.T) {
	fileModules := map[string]string{
		"local/hello.met": `fun get_hello() Str { "hello" }`,
		"lib/lib.met": `
			struct Point { x Int y Int }
			fun get_lib() Int { 42 }
			fun Point.sum(p Point) Int { p.x + p.y }
		`,
		"lib/generic.met": `
			struct Pair<A, B> { first A second B }
			fun identity<T>(x T) T { x }
		`,
	}

	tests := []struct {
		name  string
		src   string
		check func(*Engine, base.Assert)
	}{
		{
			"import unused",
			`use lib fun main() void {}`,
			nil,
		},
		{
			"local import unused",
			`use local::hello fun main() void {}`,
			nil,
		},
		{
			"call imported function",
			`use lib fun main() void { print_int(lib::get_lib()) }`,
			nil,
		},
		{
			"call imported function return type",
			`use lib fun main() void { let x = lib::get_lib() print_int(x) }`,
			func(e *Engine, assert base.Assert) {
				assertMainBinding(e, assert, "x", "Int")
			},
		},
		{
			"use imported struct",
			`use lib fun main() void { let p = lib::Point(1, 2) print_int(p.x) }`,
			func(e *Engine, assert base.Assert) {
				assertMainBinding(e, assert, "p", "lib.Point")
			},
		},
		{
			"call method on imported struct",
			`use lib fun main() void { let p = lib::Point(1, 2) print_int(p.sum()) }`,
			nil,
		},
		{
			"call method on imported struct via path",
			`use lib fun main() void { let p = lib::Point(1, 2) print_int(lib::Point.sum(p)) }`,
			nil,
		},
		{
			"assign imported function to variable",
			`use lib fun main() void { let f = lib::get_lib print_int(f()) }`,
			nil,
		},
		{
			"local import call",
			`use local::hello fun main() void { let s = hello::get_hello() print_str(s) }`,
			func(e *Engine, assert base.Assert) {
				assertMainBinding(e, assert, "s", "Str")
			},
		},
		{
			"aliased import",
			`use l = lib fun main() void { print_int(l::get_lib()) }`,
			nil,
		},
		{
			"generic function from import",
			`use generic fun main() void { let x = generic::identity<Int>(42) print_int(x) }`,
			func(e *Engine, assert base.Assert) {
				assertMainBinding(e, assert, "x", "Int")
			},
		},
		{
			"generic struct from import",
			`use generic fun main() void {
				let p = generic::Pair<Int, Str>(1, "hi")
				print_int(p.first)
				print_str(p.second)
			}`,
			func(e *Engine, assert base.Assert) {
				typ := assertMainBindingType(e, assert, "p")
				st := base.Cast[StructType](typ.Kind)
				assert.Contains(st.Name, "generic.Pair")
				assert.Equal(2, len(st.Fields))
				assert.Equal("first", st.Fields[0].Name)
				assert.Equal("second", st.Fields[1].Name)
				assert.Equal("Int", e.env.TypeDisplay(st.Fields[0].Type))
				assert.Equal("Str", e.env.TypeDisplay(st.Fields[1].Type))
			},
		},
	}

	assert := base.NewAssert(t)
	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		name := strings.TrimSpace(strings.ReplaceAll(tt.name, "!"+"only", ""))
		t.Run(name, func(t *testing.T) {
			e := typeCheckModule(t, assert, tt.src, fileModules)
			assert.Equal(0, len(e.diagnostics), "type-check failed:\n%s", e.diagnostics)
			if tt.check != nil {
				tt.check(e, assert)
			}
		})
	}
}

func TestTypeCheckModuleErr(t *testing.T) {
	fileModules := map[string]string{
		"lib/lib.met": `struct Point { x Int y Int } fun get_lib() Int { 42 }`,
		"lib/generic.met": `
			struct Pair<A, B> { first A second B }
			fun identity<T>(x T) T { x }
		`,
	}

	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			"unknown symbol in import",
			`use lib fun main() void { lib::unknown() }`,
			"symbol not defined in lib: unknown",
		},
		{
			"wrong arg type to imported function",
			`use lib fun main() void { lib::get_lib("oops") }`,
			"argument count mismatch",
		},
		{
			"generic struct wrong type arg count",
			`use generic fun main() void { generic::Pair<Int>(1) }`,
			"type argument count mismatch: expected 2, got 1",
		},
		{
			"generic function wrong type arg count",
			`use generic fun main() void { generic::identity<Int, Str>(42) }`,
			"type argument count mismatch: expected 1, got 2",
		},
		{
			"nested module access",
			`use lib fun main() void { lib::sub::foo() }`,
			"invalid module path",
		},
		{
			"dot syntax on module",
			`use lib fun main() void { lib.get_lib() }`,
			"cannot access field on non-struct type: lib",
		},
	}

	assert := base.NewAssert(t)
	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		name := strings.TrimSpace(strings.ReplaceAll(tt.name, "!"+"only", ""))
		t.Run(name, func(t *testing.T) {
			e := typeCheckModule(t, assert, tt.src, fileModules)
			assert.NotEqual(0, len(e.diagnostics), "expected diagnostics")
			assert.Contains(e.diagnostics[0].Message, tt.want)
		})
	}
}

func assertMainBindingType(e *Engine, assert base.Assert, name string) *Type {
	for _, root := range e.ast.Roots {
		mod, ok := e.ast.Node(root).Kind.(ast.Module)
		if !ok || !mod.Main {
			continue
		}
		for _, declID := range mod.Decls {
			fun, ok := e.ast.Node(declID).Kind.(ast.Fun)
			if !ok || fun.Name.Name != "main" {
				continue
			}
			block := base.Cast[ast.Block](e.ast.Node(fun.Block).Kind)
			lastExpr := block.Exprs[len(block.Exprs)-1]
			b, ok := e.lookup(lastExpr, name)
			assert.Equal(true, ok, "binding %q not found", name)
			if ok {
				return e.env.reg.types[b.TypeID].Type
			}
			return nil
		}
	}
	assert.Equal(true, false, "main function not found")
	return nil
}

func assertMainBinding(e *Engine, assert base.Assert, name string, wantType string) {
	for _, root := range e.ast.Roots {
		mod, ok := e.ast.Node(root).Kind.(ast.Module)
		if !ok || !mod.Main {
			continue
		}
		for _, declID := range mod.Decls {
			fun, ok := e.ast.Node(declID).Kind.(ast.Fun)
			if !ok || fun.Name.Name != "main" {
				continue
			}
			block := base.Cast[ast.Block](e.ast.Node(fun.Block).Kind)
			lastExpr := block.Exprs[len(block.Exprs)-1]
			b, ok := e.lookup(lastExpr, name)
			assert.Equal(true, ok, "binding %q not found", name)
			if ok {
				assert.Equal(wantType, e.env.TypeDisplay(b.TypeID))
			}
			return
		}
	}
	assert.Equal(true, false, "main function not found")
}

func memFS(files map[string]string) modules.ReadFileFn {
	return func(path string) ([]byte, error) {
		content, ok := files[path]
		if !ok {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return []byte(content), nil
	}
}

func typeCheckModule(
	t *testing.T, assert base.Assert, src string, fileModules map[string]string,
) *Engine {
	t.Helper()
	source := base.NewSource("test.met", "main", true, []rune(src))
	tokens := token.Lex(source)
	a := ast.NewAST(1)
	parser := ast.NewParser(tokens, a)
	moduleID, _ := parser.ParseModule()
	assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
	moduleResolution, diags := modules.ResolveModules(a, "local", []string{"lib"}, memFS(fileModules))
	assert.Equal(0, len(diags), "module resolution failed:\n%s", diags)
	preludeAST, _ := ast.PreludeAST(true)
	e := NewEngine(a, preludeAST, moduleResolution)
	e.Query(moduleID)
	return e
}
