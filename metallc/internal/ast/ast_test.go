package ast

import (
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
)

// makeSpan returns a span for tests where span content doesn't matter.
func makeSpan() base.Span {
	src := base.NewSource("test.met", "test", true, []rune(""))
	return base.NewSpan(src, 0, 0)
}

func TestASTDelete(t *testing.T) {
	t.Parallel()

	t.Run("delete from Block.Exprs", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a := NewAST(1)
		e1 := a.NewInt(nil, makeSpan())
		e2 := a.NewInt(nil, makeSpan())
		e3 := a.NewInt(nil, makeSpan())
		block := a.NewBlock([]NodeID{e1, e2, e3}, makeSpan())
		a.DeleteNode(e2)
		got := base.Cast[Block](a.Node(block).Kind).Exprs
		assert.Equal(2, len(got))
		assert.Equal(e1, got[0])
		assert.Equal(e3, got[1])
	})

	t.Run("delete from Module.Decls", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a := NewAST(1)
		d1 := a.NewInt(nil, makeSpan())
		d2 := a.NewInt(nil, makeSpan())
		mod := a.NewModule("test.met", "test", true, []NodeID{d1, d2}, makeSpan())
		a.DeleteNode(d1)
		got := base.Cast[Module](a.Node(mod).Kind).Decls
		assert.Equal(1, len(got))
		assert.Equal(d2, got[0])
	})

	t.Run("delete If.Else", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a := NewAST(1)
		cond := a.NewBool(true, makeSpan())
		thenBlk := a.NewBlock(nil, makeSpan())
		elseBlk := a.NewBlock(nil, makeSpan())
		ifID := a.NewIf(cond, thenBlk, &elseBlk, makeSpan())
		a.DeleteNode(elseBlk)
		got := base.Cast[If](a.Node(ifID).Kind)
		assert.Equal((*NodeID)(nil), got.Else)
	})

	t.Run("deleting If.Then panics", func(t *testing.T) {
		t.Parallel()
		a := NewAST(1)
		cond := a.NewBool(true, makeSpan())
		thenBlk := a.NewBlock(nil, makeSpan())
		_ = a.NewIf(cond, thenBlk, nil, makeSpan())
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when deleting required If.Then")
			}
		}()
		a.DeleteNode(thenBlk)
	})

	t.Run("deleting If.Cond", func(t *testing.T) {
		t.Parallel()
		a := NewAST(1)
		cond := a.NewBool(true, makeSpan())
		thenBlk := a.NewBlock(nil, makeSpan())
		_ = a.NewIf(cond, thenBlk, nil, makeSpan())
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when deleting required If.Cond")
			}
		}()
		a.DeleteNode(cond)
	})

	t.Run("deleting Binary.LHS panics", func(t *testing.T) {
		t.Parallel()
		a := NewAST(1)
		lhs := a.NewInt(nil, makeSpan())
		rhs := a.NewInt(nil, makeSpan())
		_ = a.NewBinary(BinaryOpAdd, lhs, rhs, makeSpan())
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when deleting required Binary.LHS")
			}
		}()
		a.DeleteNode(lhs)
	})

	t.Run("delete optional Var.Type", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a := NewAST(1)
		typ := a.NewSimpleType(Name{"Int", makeSpan()}, nil, makeSpan())
		expr := a.NewInt(nil, makeSpan())
		varID := a.NewVar(Name{"x", makeSpan()}, &typ, expr, false, false, makeSpan())
		a.DeleteNode(typ)
		got := base.Cast[Var](a.Node(varID).Kind)
		assert.Equal((*NodeID)(nil), got.Type)
	})

	t.Run("deleting Var.Expr panics", func(t *testing.T) {
		t.Parallel()
		a := NewAST(1)
		expr := a.NewInt(nil, makeSpan())
		_ = a.NewVar(Name{"x", makeSpan()}, nil, expr, false, false, makeSpan())
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when deleting required Var.Expr")
			}
		}()
		a.DeleteNode(expr)
	})

	t.Run("delete FunParam.Default", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a := NewAST(1)
		typ := a.NewSimpleType(Name{"Int", makeSpan()}, nil, makeSpan())
		def := a.NewInt(nil, makeSpan())
		paramID := a.NewFunParam(Name{"x", makeSpan()}, typ, &def, false, makeSpan())
		a.DeleteNode(def)
		got := base.Cast[FunParam](a.Node(paramID).Kind)
		assert.Equal((*NodeID)(nil), got.Default)
	})

	t.Run("delete MatchArm.Guard", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a := NewAST(1)
		expr := a.NewIdent("x", nil, makeSpan())
		pattern := a.NewSimpleType(Name{"Int", makeSpan()}, nil, makeSpan())
		guard := a.NewBool(true, makeSpan())
		body := a.NewBlock(nil, makeSpan())
		arms := []MatchArm{{Pattern: pattern, Binding: nil, Guard: &guard, Body: body}}
		matchID := a.NewMatch(expr, arms, nil, makeSpan())
		a.DeleteNode(guard)
		got := base.Cast[Match](a.Node(matchID).Kind)
		assert.Equal((*NodeID)(nil), got.Arms[0].Guard)
	})

	t.Run("deleting MatchArm.Body panics", func(t *testing.T) {
		t.Parallel()
		a := NewAST(1)
		expr := a.NewIdent("x", nil, makeSpan())
		pattern := a.NewSimpleType(Name{"Int", makeSpan()}, nil, makeSpan())
		body := a.NewBlock(nil, makeSpan())
		arms := []MatchArm{{Pattern: pattern, Binding: nil, Guard: nil, Body: body}}
		_ = a.NewMatch(expr, arms, nil, makeSpan())
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when deleting required MatchArm.Body")
			}
		}()
		a.DeleteNode(body)
	})

	t.Run("deleting MatchArm.Pattern panics", func(t *testing.T) {
		t.Parallel()
		a := NewAST(1)
		expr := a.NewIdent("x", nil, makeSpan())
		pattern := a.NewSimpleType(Name{"Int", makeSpan()}, nil, makeSpan())
		body := a.NewBlock(nil, makeSpan())
		arms := []MatchArm{{Pattern: pattern, Binding: nil, Guard: nil, Body: body}}
		_ = a.NewMatch(expr, arms, nil, makeSpan())
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when deleting required MatchArm.Pattern")
			}
		}()
		a.DeleteNode(pattern)
	})

	t.Run("delete optional Range.Lo", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a := NewAST(1)
		lo := a.NewInt(nil, makeSpan())
		hi := a.NewInt(nil, makeSpan())
		rangeID := a.NewRange(&lo, &hi, false, makeSpan())
		a.DeleteNode(lo)
		got := base.Cast[Range](a.Node(rangeID).Kind)
		assert.Equal((*NodeID)(nil), got.Lo)
		assert.Equal(hi, *got.Hi)
	})

	t.Run("DeleteNode removes descendants too", func(t *testing.T) {
		t.Parallel()
		assert := base.NewAssert(t)
		a := NewAST(1)
		inner1 := a.NewInt(nil, makeSpan())
		inner2 := a.NewInt(nil, makeSpan())
		inner3 := a.NewInt(nil, makeSpan())
		block := a.NewBlock([]NodeID{inner1, inner2, inner3}, makeSpan())
		outer := a.NewBlock([]NodeID{block}, makeSpan())
		a.DeleteNode(block)
		// block and its inner children should all be gone from the AST.
		assertDeleted := func(id NodeID) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("node %d should have been deleted", id)
				}
			}()
			a.Node(id)
		}
		assertDeleted(block)
		assertDeleted(inner1)
		assertDeleted(inner2)
		assertDeleted(inner3)
		// outer is still there, with empty Exprs.
		got := base.Cast[Block](a.Node(outer).Kind)
		assert.Equal(0, len(got.Exprs))
	})
}
