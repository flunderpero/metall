package comptime

import (
	"slices"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

// Env maps category names (e.g. "os", "arch", "endian") to their fully
// specified key sets. Every key must be present with a true/false value.
// The special category "tag" is user-provided and not validated for unknown keys.
type Env map[string]map[string]bool

type resolver struct {
	ast    *ast.AST
	env    Env
	diags  base.Diagnostics
	active map[ast.NodeID]bool
}

// ResolveModule deletes every `#if` inside a module that is not active.
func ResolveModule(a *ast.AST, moduleID ast.NodeID, env Env) base.Diagnostics {
	r := &resolver{a, env, nil, make(map[ast.NodeID]bool)}
	r.resolveNode(moduleID)
	if len(r.diags) > 0 {
		return r.diags
	}
	r.eliminate(moduleID)
	return r.diags
}

// resolveNode evaluates CompIf conditions recursively (including nested ones).
func (r *resolver) resolveNode(id ast.NodeID) {
	node := r.ast.Node(id)
	if compIf, ok := node.Kind.(ast.CompIf); ok {
		value, valid := r.evalCond(compIf.Cond)
		if valid {
			r.active[id] = value
		}
	}
	r.ast.Walk(id, func(childID ast.NodeID) {
		r.resolveNode(childID)
	})
}

// eliminate removes CompIf nodes from the AST. For each Module/Block it
// rewrites the child list, then deletes the now-orphaned CompIf nodes.
func (r *resolver) eliminate(id ast.NodeID) {
	r.ast.Walk(id, r.eliminate)
	node := r.ast.Node(id)
	switch kind := node.Kind.(type) {
	case ast.Module:
		newDecls, toDelete := r.rewriteList(kind.Decls)
		kind.Decls = newDecls
		node.Kind = kind
		for _, did := range toDelete {
			r.ast.DeleteNode(did)
		}
	case ast.Block:
		newExprs, toDelete := r.rewriteList(kind.Exprs)
		kind.Exprs = newExprs
		node.Kind = kind
		for _, did := range toDelete {
			r.ast.DeleteNode(did)
		}
	}
}

// rewriteList returns (newList, toDelete). Active CompIfs have their body
// inlined into newList; the shell is detached (Body set to nil) so that
// DeleteNode only removes the shell + condition, not the relocated body.
// Inactive CompIfs go into toDelete as-is so DeleteNode removes the whole
// subtree.
func (r *resolver) rewriteList(ids []ast.NodeID) (result, toDelete []ast.NodeID) {
	for _, id := range ids {
		ci, ok := r.ast.Node(id).Kind.(ast.CompIf)
		if !ok {
			result = append(result, id)
			continue
		}
		if r.active[id] {
			inlined, moreToDelete := r.rewriteList(ci.Body)
			result = append(result, inlined...)
			toDelete = append(toDelete, moreToDelete...)
			ci.Body = nil
			r.ast.Node(id).Kind = ci
			toDelete = append(toDelete, id)
		} else {
			toDelete = append(toDelete, id)
		}
	}
	return result, toDelete
}

func (r *resolver) evalCond(id ast.NodeID) (value bool, valid bool) {
	node := r.ast.Node(id)
	switch kind := node.Kind.(type) {
	case ast.FieldAccess:
		return r.evalFieldAccess(kind, node.Span)
	case ast.Binary:
		return r.evalBinary(kind)
	case ast.Unary:
		return r.evalUnary(kind)
	default:
		r.diag(node.Span, "invalid compile-time condition: expected <category>.<key>, got %T", kind)
		return false, false
	}
}

func (r *resolver) evalFieldAccess(fa ast.FieldAccess, span base.Span) (value bool, valid bool) {
	targetNode := r.ast.Node(fa.Target)
	ident, ok := targetNode.Kind.(ast.Ident)
	if !ok {
		r.diag(targetNode.Span, "invalid compile-time condition: expected identifier, got %T", targetNode.Kind)
		return false, false
	}
	category := ident.Name
	key := fa.Field.Name
	catMap, catOK := r.env[category]
	if !catOK {
		r.diag(span, "unknown compile-time category: %q (available: %s)", category, availableCategories(r.env))
		return false, false
	}
	val, keyOK := catMap[key]
	if !keyOK {
		if category == "tag" {
			return false, true
		}
		r.diag(fa.Field.Span, "unknown key %q in category %q (available: %s)", key, category, availableKeys(catMap))
		return false, false
	}
	return val, true
}

func (r *resolver) evalBinary(bin ast.Binary) (value bool, valid bool) {
	if bin.Op != ast.BinaryOpAnd && bin.Op != ast.BinaryOpOr {
		node := r.ast.Node(bin.LHS)
		r.diag(node.Span, "invalid operator in compile-time condition: %q (only 'and' and 'or' are allowed)", bin.Op)
		return false, false
	}
	lhs, ok := r.evalCond(bin.LHS)
	if !ok {
		return false, false
	}
	rhs, ok := r.evalCond(bin.RHS)
	if !ok {
		return false, false
	}
	if bin.Op == ast.BinaryOpAnd {
		return lhs && rhs, true
	}
	return lhs || rhs, true
}

func (r *resolver) evalUnary(un ast.Unary) (value bool, valid bool) {
	if un.Op != ast.UnaryOpNot {
		node := r.ast.Node(un.Expr)
		r.diag(node.Span, "invalid operator in compile-time condition: %q (only 'not' is allowed)", un.Op)
		return false, false
	}
	val, ok := r.evalCond(un.Expr)
	if !ok {
		return false, false
	}
	return !val, true
}

func (r *resolver) diag(span base.Span, msg string, args ...any) {
	r.diags = append(r.diags, *base.NewDiagnostic(span, msg, args...))
}

func availableCategories(env Env) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return strings.Join(keys, ", ")
}

func availableKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return strings.Join(keys, ", ")
}
