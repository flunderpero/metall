package types

import (
	"fmt"
	"unicode"

	"github.com/flunderpero/metall/metallc/internal/ast"
)

func isCapitalized(name string) bool {
	return name != "" && unicode.IsUpper(rune(name[0]))
}

// isSymbolImport reports whether `name`, in the module enclosing nodeID, is a
// `use a.b.name` symbol import (as opposed to a local declaration).
func (c *TypeContext) isSymbolImport(nodeID ast.NodeID, name string) bool {
	for scope := c.scopeGraph.NodeScope(nodeID); scope != nil && scope.Node != 0; scope = scope.Parent {
		if _, ok := c.ast.Node(scope.Node).Kind.(ast.Module); ok {
			imp, ok := c.moduleResolution.ImportFor(scope.Node, name)
			return ok && !imp.IsModule()
		}
	}
	return false
}

// symbolImportNameMismatch reports a capitalization mismatch between the local
// name and the imported symbol's kind: a type must bind to a Capitalized name,
// any other symbol to a lowercase one. Returns "" when they match (so the
// non-rename `use a.b.Name` always passes, since the name is the symbol's own).
func (c *TypeContext) symbolImportNameMismatch(name string, declNode ast.NodeID) string {
	isType := false
	switch c.ast.Node(declNode).Kind.(type) {
	case ast.Struct, ast.Union, ast.Enum, ast.Shape:
		isType = true
	}
	if isType == isCapitalized(name) {
		return ""
	}
	if isType {
		return fmt.Sprintf("a type imported as `%s` must be capitalized", name)
	}
	return fmt.Sprintf("a value imported as `%s` must not be capitalized", name)
}
