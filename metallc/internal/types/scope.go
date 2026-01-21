package types

import (
	"fmt"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type ScopeID int

func (id ScopeID) String() string {
	return fmt.Sprintf("scope_%d", id)
}

type Scope struct {
	ID       ScopeID
	Parent   *Scope
	bindings map[string]*Binding
}

type Binding struct {
	Name   string
	Decl   ast.NodeID
	Mut    bool
	TypeID TypeID
}

func NewScope(id ScopeID, parent *Scope) *Scope {
	return &Scope{id, parent, map[string]*Binding{}}
}

func (s *Scope) Bind(name string, mut bool, decl ast.NodeID) bool {
	if _, ok := s.bindings[name]; ok {
		return false
	}
	s.bindings[name] = &Binding{name, decl, mut, InvalidTypeID}
	return true
}

func (s *Scope) Lookup(name string) (*Binding, bool) {
	if b, ok := s.bindings[name]; ok {
		return b, true
	}
	if s.Parent != nil {
		return s.Parent.Lookup(name)
	}
	return nil, false
}

type ScopeGraph struct {
	scopes        map[ScopeID]*Scope
	scopeByNodeID map[ast.NodeID]*Scope
}

func NewScopeGraph() *ScopeGraph {
	return &ScopeGraph{map[ScopeID]*Scope{}, map[ast.NodeID]*Scope{}}
}

func (g *ScopeGraph) NodeScope(nodeID ast.NodeID) *Scope {
	scope, ok := g.scopeByNodeID[nodeID]
	if !ok {
		panic(base.Errorf("no scope for node %d", nodeID))
	}
	return scope
}

func (g *ScopeGraph) SetNodeScope(nodeID ast.NodeID, scope *Scope) {
	if _, ok := g.scopeByNodeID[nodeID]; ok {
		panic(base.Errorf("scope already set for node %d", nodeID))
	}
	g.scopeByNodeID[nodeID] = scope
}
