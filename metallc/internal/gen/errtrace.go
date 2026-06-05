package gen

import (
	"fmt"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/types"
)

// errTraceLevel is one step of the tag-test chain that decides, at a return,
// whether the returned value is actually an Err. A plain `!T` needs one step
// (is the union tag the Err variant?); a nested carrier like `?!T`
// (Option<Result<T>>) needs one step per layer. tag is the index to match
// here: the Err variant at the leaf, otherwise the variant holding the next
// layer down.
type errTraceLevel struct {
	unionIR string
	tag     int
	leaf    bool
}

// isErrEnum reports whether typeID is an enum in the `Err` family: the base
// open `Err` enum or any subset of it.
func isErrEnum(env *types.TypeEnv, typeID types.TypeID) bool {
	et, ok := env.Type(typeID).Kind.(types.EnumType)
	if !ok {
		return false
	}
	if et.Name == "Err" {
		return true
	}
	if et.Root != types.InvalidTypeID {
		if rt, ok := env.Type(et.Root).Kind.(types.EnumType); ok && rt.Name == "Err" {
			return true
		}
	}
	return false
}

// errTraceShape builds the tag-test chain (one errTraceLevel per union layer)
// from a return type down to its Err variant, or nil if the type can't be an
// error. It follows only union variants, never struct fields or array elements:
// an Err stored in a field is data, not a propagating error.
func (g *IRFunGen) errTraceShape(typeID types.TypeID) []errTraceLevel {
	union, ok := g.env.Type(typeID).Kind.(types.UnionType)
	if !ok {
		return nil
	}
	for i, v := range union.Variants {
		if isErrEnum(g.env, v) {
			return []errTraceLevel{{unionIR: g.irType(typeID), tag: i, leaf: true}}
		}
	}
	for i, v := range union.Variants {
		if sub := g.errTraceShape(v); sub != nil {
			return append([]errTraceLevel{{unionIR: g.irType(typeID), tag: i, leaf: false}}, sub...)
		}
	}
	return nil
}

// isTryPropagation reports whether exprID is the parser-synthesized `__try_e`
// binding that an else-less `try` returns, i.e. an error being propagated up
// rather than freshly originated at this return.
func (g *IRFunGen) isTryPropagation(exprID ast.NodeID) bool {
	ident, ok := g.ast.Node(exprID).Kind.(ast.Ident)
	return ok && ident.Name == "__try_e"
}

// recordErrTraceReturn emits the tag-test chain at a return and, only when the
// returned value is an Err, calls std::errors to record this frame. isOrigin
// picks restart_trace (error raised here: reset, then record) over record_frame
// (error passing through: append). No-op when tracing is off or this function
// cannot return an error. Reads the value back from funRetReg, so it works
// however the error reached the return.
func (g *IRFunGen) recordErrTraceReturn(id ast.NodeID, span base.Span, isOrigin bool) {
	if !g.opts.ErrorTracing || len(g.errtraceShape) == 0 {
		return
	}
	callee := "@std$errors.record_frame"
	if isOrigin {
		callee = "@std$errors.restart_trace"
	}
	cont := g.label("errtrace_cont", id)
	ptr := g.funRetReg
	for idx, lvl := range g.errtraceShape {
		tagPtr := g.reg()
		g.write("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagPtr, lvl.unionIR, ptr)
		tag := g.reg()
		g.write("%s = load i64, ptr %s", tag, tagPtr)
		match := g.reg()
		g.write("%s = icmp eq i64 %s, %d", match, tag, lvl.tag)
		if lvl.leaf {
			rec := g.label(fmt.Sprintf("errtrace_rec%d", idx), id)
			g.write("br i1 %s, label %%%s, label %%%s", match, rec, cont)
			g.writeLabel(rec)
			loc := g.addStrConst(span.String())
			fn := g.addStrConst(g.errtraceFuncName)
			g.write("call void %s(ptr byval(%%Str) %s, ptr byval(%%Str) %s)", callee, loc, fn)
			g.write("br label %%%s", cont)
		} else {
			next := g.label(fmt.Sprintf("errtrace_next%d", idx), id)
			g.write("br i1 %s, label %%%s, label %%%s", match, next, cont)
			g.writeLabel(next)
			payload := g.reg()
			g.write("%s = getelementptr %s, ptr %s, i32 0, i32 1", payload, lvl.unionIR, ptr)
			ptr = payload
		}
	}
	g.writeLabel(cont)
}

// tailExprIsCall reports whether a block's last expression is a direct call.
// A tail call forwards a callee's error (propagation); any other tail
// expression that yields an error originated it here.
func (g *IRFunGen) tailExprIsCall(blockID ast.NodeID) bool {
	blk, ok := g.ast.Node(blockID).Kind.(ast.Block)
	if !ok || len(blk.Exprs) == 0 {
		return false
	}
	_, isCall := g.ast.Node(blk.Exprs[len(blk.Exprs)-1]).Kind.(ast.Call)
	return isCall
}

// tailExprSpan returns the span of a block's last expression, the location a
// tail-expression error return propagates from. Falls back to the block span.
func (g *IRFunGen) tailExprSpan(blockID ast.NodeID) base.Span {
	if blk, ok := g.ast.Node(blockID).Kind.(ast.Block); ok && len(blk.Exprs) > 0 {
		return g.ast.Node(blk.Exprs[len(blk.Exprs)-1]).Span
	}
	return g.ast.Node(blockID).Span
}

// emitErrTraceMainDump dumps the trace to stderr, called on main's error branch
// after "failed: <name>".
func (g *IRFunGen) emitErrTraceMainDump() {
	if !g.opts.ErrorTracing {
		return
	}
	g.write("call void @std$errors.dump()")
}
