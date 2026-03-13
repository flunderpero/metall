package ast

import (
	"fmt"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestScopesMD(t *testing.T) {
	mdtest.RunFile(t, mdtest.File("scope_test.md"), mdtest.RunFunc(runScopeMDTest))
}

func runScopeMDTest(_ *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	results := map[string]string{}

	source := base.NewSource("test.met", "test", true, []rune(tc.Input))
	tokens := token.Lex(source)
	parser := NewParser(tokens, NewAST(1))
	nodeID, parseOK := parser.ParseExpr(0)
	assert.Equal(0, len(parser.Diagnostics), "parse errors: %s", parser.Diagnostics)
	assert.Equal(true, parseOK, "parser returned false")

	g := BuildScopeGraph(parser.AST, []NodeID{nodeID})

	if _, ok := tc.Want["scopes"]; ok {
		results["scopes"] = collectScopes(parser.AST, g)
	}
	if _, ok := tc.Want["nodes"]; ok {
		results["nodes"] = collectNodes(parser.AST, g)
	}

	return results
}

func collectScopes(a *AST, g *ScopeGraph) string {
	seen := map[ScopeID]bool{}
	var scopes []*Scope
	a.Iter(func(nodeID NodeID) bool {
		scope := g.NodeScope(nodeID)
		if !seen[scope.ID] {
			seen[scope.ID] = true
			scopes = append(scopes, scope)
		}
		return true
	})
	// Sort by ID for stable output.
	for i := range scopes {
		for j := i + 1; j < len(scopes); j++ {
			if scopes[i].ID > scopes[j].ID {
				scopes[i], scopes[j] = scopes[j], scopes[i]
			}
		}
	}
	// Map real scope IDs to sequential letters for stable test output.
	letterMap := map[ScopeID]string{}
	for i, scope := range scopes {
		letterMap[scope.ID] = scopeLetter(ScopeID(i)) //nolint:gosec
	}
	var lines []string
	for _, scope := range scopes {
		if scope.Parent != nil {
			parentLetter, ok := letterMap[scope.Parent.ID]
			if !ok {
				parentLetter = scopeLetter(scope.Parent.ID)
			}
			lines = append(lines, fmt.Sprintf("%s:%s", letterMap[scope.ID], parentLetter))
		} else {
			lines = append(lines, fmt.Sprintf("%s:-", letterMap[scope.ID]))
		}
	}
	return strings.Join(lines, "\n")
}

func collectNodes(a *AST, g *ScopeGraph) string {
	// Build scope letter map: collect all user scopes and assign sequential letters.
	seenScopes := map[ScopeID]bool{}
	var sortedScopes []ScopeID
	var nodeIDs []NodeID
	a.Iter(func(nodeID NodeID) bool {
		nodeIDs = append(nodeIDs, nodeID)
		scope := g.NodeScope(nodeID)
		if !seenScopes[scope.ID] {
			seenScopes[scope.ID] = true
			sortedScopes = append(sortedScopes, scope.ID)
		}
		return true
	})
	for i := range sortedScopes {
		for j := i + 1; j < len(sortedScopes); j++ {
			if sortedScopes[i] > sortedScopes[j] {
				sortedScopes[i], sortedScopes[j] = sortedScopes[j], sortedScopes[i]
			}
		}
	}
	letterMap := map[ScopeID]string{}
	for i, id := range sortedScopes {
		letterMap[id] = scopeLetter(ScopeID(i)) //nolint:gosec
	}
	// Sort nodes by ID for stable output.
	for i := range nodeIDs {
		for j := i + 1; j < len(nodeIDs); j++ {
			if nodeIDs[i] > nodeIDs[j] {
				nodeIDs[i], nodeIDs[j] = nodeIDs[j], nodeIDs[i]
			}
		}
	}
	lines := make([]string, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		scope := g.NodeScope(nodeID)
		lines = append(lines, fmt.Sprintf("%s:%s", a.Debug(nodeID, false, 0), letterMap[scope.ID]))
	}
	return strings.Join(lines, "\n")
}

func scopeLetter(id ScopeID) string {
	return string('a' + rune(id))
}
