package types

import (
	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

// diagFunc emits a diagnostic. The generics pass passes a no-op so the
// authoritative engine pass reports the error once.
type diagFunc func(span base.Span, msg string, msgArgs ...any)

func noDiag(base.Span, string, ...any) {}

// hasNamedArgs reports whether any argument carries an explicit `name=` label.
func hasNamedArgs(argNames []*ast.Name) bool {
	for _, n := range argNames {
		if n != nil {
			return true
		}
	}
	return false
}

// matchArgs maps source-order arguments onto one slot per parameter. Positional
// arguments fill parameters left to right; a named argument fills the parameter
// it names. slots[i] is the source arg node for parameter i, or 0 if no source
// argument targets it. noun is "parameter" or "field" for error messages.
func matchArgs(
	a *ast.AST,
	paramNames []string,
	noun string,
	args []ast.NodeID,
	argNames []*ast.Name,
	span base.Span,
	diag diagFunc,
) ([]ast.NodeID, bool) {
	slots := make([]ast.NodeID, len(paramNames))
	index := func(name string) int {
		for i, p := range paramNames {
			if p == name {
				return i
			}
		}
		return -1
	}
	pos := 0
	seenNamed := false
	for i, arg := range args {
		var name *ast.Name
		if i < len(argNames) {
			name = argNames[i]
		}
		if name == nil {
			if seenNamed {
				diag(a.Node(arg).Span, "positional argument after named argument")
				return nil, false
			}
			if pos >= len(paramNames) {
				diag(span, "too many arguments: expected at most %d, got %d", len(paramNames), len(args))
				return nil, false
			}
			slots[pos] = arg
			pos++
			continue
		}
		seenNamed = true
		idx := index(name.Name)
		if idx < 0 {
			diag(name.Span, "unknown %s: %s", noun, name.Name)
			return nil, false
		}
		if slots[idx] != 0 {
			diag(name.Span, "%s %s specified more than once", noun, name.Name)
			return nil, false
		}
		slots[idx] = arg
	}
	return slots, true
}

// orderCallArgs maps source-order call arguments onto the user-facing function
// parameters (FunParam nodes, receiver excluded) and fills missing slots from
// each parameter's default. Returns the per-parameter node list (source value or
// default expression), the subset that came from defaults, and ok.
func orderCallArgs(
	a *ast.AST,
	userParams []ast.NodeID,
	args []ast.NodeID,
	argNames []*ast.Name,
	span base.Span,
	diag diagFunc,
) (order []ast.NodeID, defaults []ast.NodeID, ok bool) {
	paramNames := make([]string, len(userParams))
	for i, p := range userParams {
		paramNames[i] = base.Cast[ast.FunParam](a.Node(p).Kind).Name.Name
	}
	slots, ok := matchArgs(a, paramNames, "parameter", args, argNames, span, diag)
	if !ok {
		return nil, nil, false
	}
	order = make([]ast.NodeID, len(slots))
	for i, slot := range slots {
		if slot != 0 {
			order[i] = slot
			continue
		}
		param := base.Cast[ast.FunParam](a.Node(userParams[i]).Kind)
		if param.Default == nil {
			diag(span, "missing argument for parameter: %s", param.Name.Name)
			return nil, nil, false
		}
		order[i] = *param.Default
		defaults = append(defaults, *param.Default)
	}
	return order, defaults, true
}
