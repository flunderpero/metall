package ast

import (
	"fmt"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestScopes(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		scopes string // "scopeID:parentID" pairs, one per line
		nodes  string // "nodeDebug:scopeID" pairs, one per line
	}{
		{
			name: "simple var",
			src:  `let x = 1`,
			scopes: `
				a:-
			`,
			nodes: `
				n1:Int(value=1):a
				n2:Var(name="x",mut=false,expr=n1:Int):a
			`,
		},
		{
			name: "block creates scope",
			src:  `{ let x = 1 }`,
			scopes: `
				a:-
				b:a
			`,
			nodes: `
				n1:Int(value=1):b
				n2:Var(name="x",mut=false,expr=n1:Int):b
				n3:Block(createScope=true,exprs=[n2:Var]):a
			`,
		},
		{
			name: "nested blocks",
			src:  `{ let x = 1 { let y = 2 } }`,
			scopes: `
				a:-
				b:a
				c:b
			`,
			nodes: `
				n1:Int(value=1):b
				n2:Var(name="x",mut=false,expr=n1:Int):b
				n3:Int(value=2):c
				n4:Var(name="y",mut=false,expr=n3:Int):c
				n5:Block(createScope=true,exprs=[n4:Var]):b
				n6:Block(createScope=true,exprs=[n2:Var,n5:Block]):a
			`,
		},
		{
			name: "function",
			src:  `fun foo(a Int) Int { a }`,
			scopes: `
				a:-
				b:a
			`,
			nodes: `
				n1:SimpleType(name="Int"):b
				n2:FunParam(name="a",type=n1:SimpleType):b
				n3:SimpleType(name="Int"):b
				n4:Ident(name="a"):b
				n5:Block(createScope=false,exprs=[n4:Ident]):b
				n6:Fun(name="foo",params=[n2:FunParam],returnType=n3:SimpleType,block=n5:Block):a
			`,
		},
		{
			name: "function with nested block",
			src:  `fun foo() void { { 1 } }`,
			scopes: `
				a:-
				b:a
				c:b
			`,
			nodes: `
				n1:SimpleType(name="void"):b
				n2:Int(value=1):c
				n3:Block(createScope=true,exprs=[n2:Int]):b
				n4:Block(createScope=false,exprs=[n3:Block]):b
				n5:Fun(name="foo",params=[],returnType=n1:SimpleType,block=n4:Block):a
			`,
		},
		{
			name: "struct creates scope",
			src:  `struct Foo { one Int }`,
			scopes: `
				a:-
				b:a
			`,
			nodes: `
				n1:SimpleType(name="Int"):b
				n2:StructField(name="one",mut=false,type=n1:SimpleType):b
				n3:Struct(name="Foo",fields=[n2:StructField]):a
			`,
		},
		{
			name: "generic struct scope",
			src:  `struct Foo<T> { value T }`,
			scopes: `
				a:-
				b:a
			`,
			nodes: `
				n1:SimpleType(name="T"):b
				n2:SimpleType(name="T"):b
				n3:StructField(name="value",mut=false,type=n2:SimpleType):b
				n4:Struct(name="Foo",typeParams=[n1:SimpleType],fields=[n3:StructField]):a
			`,
		},
	}

	assert := base.NewAssert(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", "test", true, []rune(tt.src))
			tokens := token.Lex(source)
			parser := NewParser(tokens, 1)
			nodeID, parseOK := parser.ParseExpr(0)
			assert.Equal(0, len(parser.Diagnostics), "parse errors: %s", parser.Diagnostics)
			assert.Equal(true, parseOK, "parser returned false")

			g := BuildScopeGraph(parser.AST, []NodeID{nodeID})

			// Verify scopes: collect all scopes and check parent relationships.
			gotScopes := collectScopes(parser.AST, g)
			wantScopes := parseSnapshot(tt.scopes)
			assert.Equal(wantScopes, gotScopes, "scopes mismatch")

			// Verify nodes: check each node has the expected scope.
			gotNodes := collectNodes(parser.AST, g)
			wantNodes := parseSnapshot(tt.nodes)
			assert.Equal(wantNodes, gotNodes, "nodes mismatch")
		})
	}
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

func parseSnapshot(s string) string {
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
