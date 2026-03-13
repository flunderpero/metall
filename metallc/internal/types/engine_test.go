package types

import (
	"fmt"
	"math/big"
	"slices"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/modules"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestEngineCheckMD(t *testing.T) {
	mdtest.RunFile(t, mdtest.File("engine_test.md"), mdtest.RunFunc(runEngineTest))
}

func runEngineTest(_ *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	results := map[string]string{}
	isModule := slices.Contains(tc.Tags, "module")

	source := base.NewSource("test.met", "test", true, []rune(tc.Input))
	tokens := token.Lex(source)
	parser := ast.NewParser(tokens, ast.NewAST(1))

	var nodeID ast.NodeID
	if isModule {
		nodeID, _ = parser.ParseModule()
	} else {
		nodeID, _ = parser.ParseExpr(0)
		parser.Roots = append(parser.Roots, nodeID)
	}
	assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)

	preludeAST, _ := ast.PreludeAST(true)
	e := NewEngine(parser.AST, preludeAST, &modules.ModuleResolution{})
	e.Query(nodeID)

	_, wantTypes := tc.Want["types"]
	_, wantBindings := tc.Want["bindings"]
	_, wantError := tc.Want["error"]

	if wantError {
		results["error"] = e.diagnostics.String()
	}

	if wantTypes || wantBindings {
		if len(e.diagnostics) > 0 {
			assert.Equal(0, len(e.diagnostics), "unexpected errors:\n%s", e.diagnostics)
		}
		if wantTypes {
			results["types"] = e.Env().DebugTypes(nodeID)
		}
		if wantBindings {
			results["bindings"] = e.Env().DebugBindings(nodeID)
		}
	}

	// Always verify that work results contain no unresolved type parameters.
	for _, f := range e.Funs() {
		ft := base.Cast[FunType](e.env.Type(f.TypeID).Kind)
		for _, p := range ft.Params {
			assert.Equal(false, e.env.containsTypeParam(p),
				"Funs() should not contain type params in params: %s", f.Name)
		}
		assert.Equal(false, e.env.containsTypeParam(ft.Return),
			"Funs() should not contain type params in return: %s", f.Name)
	}
	for _, s := range e.Structs() {
		st := base.Cast[StructType](e.env.Type(s.TypeID).Kind)
		assert.Equal(false, e.env.hasTypeParam(st.TypeArgs),
			"Structs() should not contain type params: %s", st.Name)
	}
	for _, u := range e.Unions() {
		ut := base.Cast[UnionType](e.env.Type(u.TypeID).Kind)
		assert.Equal(false, e.env.hasTypeParam(ut.TypeArgs),
			"Unions() should not contain type params: %s", ut.Name)
	}

	return results
}

func TestIntTypes(t *testing.T) {
	assert := base.NewAssert(t)
	allIntTypes := []string{"I8", "I16", "I32", "Int", "U8", "U16", "U32", "U64", "Rune"}

	// lit returns a source expression that produces a value of the given int type.
	// For Rune this is a rune literal ('a'), for all others it's the type constructor (e.g. I32(1)).
	lit := func(name string) string {
		if name == "Rune" {
			return "'a'"
		}
		return name + "(1)"
	}

	typeCheck := func(t *testing.T, src string) *Engine {
		t.Helper()
		source := base.NewSource("test.met", "test", true, []rune(src))
		tokens := token.Lex(source)
		parser := ast.NewParser(tokens, ast.NewAST(1))
		exprID, parseOK := parser.ParseExpr(0)
		if !parseOK || len(parser.Diagnostics) > 0 {
			t.Fatalf("parse failed: %s", parser.Diagnostics)
		}
		preludeAST, _ := ast.PreludeAST(true)
		parser.Roots = append(parser.Roots, exprID)
		e := NewEngine(parser.AST, preludeAST, &modules.ModuleResolution{})
		e.Query(exprID)
		return e
	}

	t.Run("literal range", func(t *testing.T) {
		// Each type constructor accepts 0 and its max literal value.
		// Rune is tested separately because Rune(...) is prelude-only.
		for _, info := range intTypes {
			if info.Name == "Rune" {
				continue
			}
			for _, val := range []string{"0", info.Max.String()} {
				src := fmt.Sprintf("%s(%s)", info.Name, val)
				e := typeCheck(t, src)
				assert.Equal(0, len(e.diagnostics), "%s(%s) should be valid: %s", info.Name, val, e.diagnostics)
			}
		}
		// NOTE: Signed min values (e.g. I8(-128)) can't be expressed as
		// literals because the language has no negative literal syntax and
		// `0 - 128` produces an Int which can't be narrowed to I8.
	})

	t.Run("literal out of range", func(t *testing.T) {
		// Rune is tested separately because Rune(...) is prelude-only.
		for _, typ := range intTypes {
			if typ.Name == "Rune" {
				continue
			}
			aboveMax := new(big.Int).Add(typ.Max, big.NewInt(1))
			src := fmt.Sprintf("%s(%s)", typ.Name, aboveMax)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.diagnostics), "%s(%s) diagnostics: %s", typ.Name, aboveMax, e.diagnostics)
			assert.Contains(e.diagnostics[0].Display(), "out of range", "%s(%s)", typ.Name, aboveMax)
		}
	})

	t.Run("Rune constructor is prelude-only", func(t *testing.T) {
		e := typeCheck(t, `{ let r = 'a' Rune(r) }`)
		assert.Equal(1, len(e.diagnostics), "Rune() diagnostics: %s", e.diagnostics)
		assert.Contains(e.diagnostics[0].Display(), "Rune cannot be constructed directly")
	})

	t.Run("arithmetic", func(t *testing.T) {
		for _, op := range []string{"+", "-", "*", "/"} {
			for _, name := range allIntTypes {
				src := fmt.Sprintf("%s %s %s", lit(name), op, lit(name))
				e := typeCheck(t, src)
				assert.Equal(0, len(e.diagnostics), "%s: %s", src, e.diagnostics)
			}
		}
	})

	t.Run("comparison", func(t *testing.T) {
		for _, op := range []string{"==", "!="} {
			for _, name := range allIntTypes {
				src := fmt.Sprintf("%s %s %s", lit(name), op, lit(name))
				e := typeCheck(t, src)
				assert.Equal(0, len(e.diagnostics), "%s: %s", src, e.diagnostics)
			}
		}
	})

	t.Run("bitwise binary", func(t *testing.T) {
		for _, op := range []string{"|", "^", "<<", ">>"} {
			for _, name := range allIntTypes {
				src := fmt.Sprintf("%s %s %s", lit(name), op, lit(name))
				e := typeCheck(t, src)
				assert.Equal(0, len(e.diagnostics), "%s: %s", src, e.diagnostics)
			}
		}
	})

	t.Run("bitwise and", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf("%s & %s", lit(name), lit(name))
			e := typeCheck(t, src)
			assert.Equal(0, len(e.diagnostics), "%s & %s: %s", lit(name), lit(name), e.diagnostics)
		}
	})

	t.Run("bitwise not", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf("~%s", lit(name))
			e := typeCheck(t, src)
			assert.Equal(0, len(e.diagnostics), "~%s: %s", lit(name), e.diagnostics)
		}
	})

	t.Run("bitwise on Bool rejected", func(t *testing.T) {
		for _, op := range []string{"|", "^", "<<", ">>"} {
			src := fmt.Sprintf("true %s false", op)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.diagnostics), "%s: %s", src, e.diagnostics)
			assert.Contains(e.diagnostics[0].Display(), "an integer")
		}
	})

	t.Run("bitwise not on Bool rejected", func(t *testing.T) {
		e := typeCheck(t, "~true")
		assert.Equal(1, len(e.diagnostics), "diagnostics: %s", e.diagnostics)
		assert.Contains(e.diagnostics[0].Display(), "an integer")
	})

	t.Run("mixed types rejected in binary ops", func(t *testing.T) {
		e := typeCheck(t, `{ let x = I32(1) let y = U8(1) x + y }`)
		assert.Equal(1, len(e.diagnostics), "diagnostics: %s", e.diagnostics)
		assert.Contains(e.diagnostics[0].Display(), "type mismatch")
	})

	t.Run("Rune mixed with U32 rejected", func(t *testing.T) {
		e := typeCheck(t, `{ let r = 'a' let u = U32(1) r + u }`)
		assert.Equal(1, len(e.diagnostics), "diagnostics: %s", e.diagnostics)
		assert.Contains(e.diagnostics[0].Display(), "type mismatch")
	})

	t.Run("non-integer rejected", func(t *testing.T) {
		// Rune is excluded because Rune(...) is prelude-only.
		for _, name := range allIntTypes {
			if name == "Rune" {
				continue
			}
			src := fmt.Sprintf(`%s("hello")`, name)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.diagnostics), "%s(Str) diagnostics: %s", name, e.diagnostics)
			assert.Contains(e.diagnostics[0].Display(), "use conversion methods instead", name)
		}
	})

	t.Run("wrong arg count", func(t *testing.T) {
		// Rune is excluded because Rune(...) is prelude-only.
		for _, name := range allIntTypes {
			if name == "Rune" {
				continue
			}
			src := fmt.Sprintf("%s(1, 2)", name)
			e := typeCheck(t, src)
			assert.Equal(1, len(e.diagnostics), "%s(1,2) diagnostics: %s", name, e.diagnostics)
			assert.Contains(e.diagnostics[0].Display(), "takes exactly 1 argument", name)
		}
	})

	t.Run("type constructor rejects cross-type conversions", func(t *testing.T) {
		crossTypeTests := []struct{ from, to string }{
			{"I8", "I16"},
			{"I8", "I32"},
			{"I8", "Int"},
			{"I16", "I32"},
			{"I16", "Int"},
			{"I32", "Int"},
			{"U8", "U16"},
			{"U8", "U32"},
			{"U8", "U64"},
			{"U16", "U32"},
			{"U16", "U64"},
			{"U32", "U64"},
			{"U8", "I16"},
			{"U8", "I32"},
			{"U8", "Int"},
			{"U16", "I32"},
			{"U16", "Int"},
			{"U32", "Int"},
			{"I16", "I8"},
			{"I32", "I8"},
			{"Int", "I8"},
			{"I8", "U8"},
			{"I8", "U64"},
			{"U64", "Int"},
			{"U64", "I32"},
		}
		for _, tt := range crossTypeTests {
			name := fmt.Sprintf("%s_to_%s", tt.from, tt.to)
			t.Run(name, func(t *testing.T) {
				src := fmt.Sprintf("{ let x = %s(1) %s(x) }", tt.from, tt.to)
				e := typeCheck(t, src)
				assert.NotEqual(0, len(e.diagnostics), "%s(%s) should be rejected", tt.to, tt.from)
				if len(e.diagnostics) > 0 {
					assert.Contains(
						e.diagnostics[0].Display(), "use conversion methods instead", "%s → %s", tt.from, tt.to,
					)
				}
			})
		}
	})

	t.Run("type constructor allows identity", func(t *testing.T) {
		// Rune is excluded because Rune(...) is prelude-only.
		for _, name := range allIntTypes {
			if name == "Rune" {
				continue
			}
			src := fmt.Sprintf("{ let x = %s(1) %s(x) }", name, name)
			e := typeCheck(t, src)
			assert.Equal(0, len(e.diagnostics), "%s(%s) identity should be allowed: %s", name, name, e.diagnostics)
		}
	})

	t.Run("safe uninitialized", func(t *testing.T) {
		for _, name := range allIntTypes {
			src := fmt.Sprintf("{ let @a = Arena() let x = @a.slice_uninit<%s>(5) }", name)
			e := typeCheck(t, src)
			assert.Equal(0, len(e.diagnostics), "%s should be safe uninitialized: %s", name, e.diagnostics)
		}
	})
}
