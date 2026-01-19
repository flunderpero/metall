package ast

import (
	"fmt"

	"github.com/flunderpero/metall/metallc/internal/base"
)

type ScopeID int

func (id ScopeID) String() string {
	return fmt.Sprintf("scope_%d", id)
}

type Binding struct {
	Decl NodeID
	Mut  bool
}

type Scope struct {
	id       ScopeID
	root     NodeID
	parent   *Scope
	bindings map[string]Binding
}

func NewScope(id ScopeID, parent *Scope) *Scope {
	return &Scope{id, 0, parent, map[string]Binding{}}
}

func (s *Scope) Bind(name string, mut bool, decl NodeID) bool {
	if _, ok := s.bindings[name]; ok {
		return false
	}
	s.bindings[name] = Binding{decl, mut}
	return true
}

type ScopeGraph struct {
	scopes        map[ScopeID]*Scope
	scopeByNodeID map[NodeID]*Scope
}

func NewScopeGraph() *ScopeGraph {
	return &ScopeGraph{map[ScopeID]*Scope{}, map[NodeID]*Scope{}}
}

func (g *ScopeGraph) NodeScope(node NodeID) *Scope {
	return g.scopeByNodeID[node]
}

func (g *ScopeGraph) SetNodeScope(node NodeID, scope *Scope) {
	if _, ok := g.scopeByNodeID[node]; ok {
		panic(base.Errorf("scope already set for node %d", node))
	}
	g.scopeByNodeID[node] = scope
}
