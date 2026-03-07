package ast

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/base"
)

type ScopeID uint64

func (id ScopeID) String() string {
	return fmt.Sprintf("s%d", id)
}

type BindingID uint64

func (id BindingID) String() string {
	return fmt.Sprintf("b%d", id)
}

type Scope struct {
	ID        ScopeID
	Parent    *Scope
	Node      NodeID
	Namespace string
	bindings  map[string]*Binding
}

type Binding struct {
	ID   BindingID
	Name string
	Decl NodeID
}

func NewScope(root NodeID, id ScopeID, parent *Scope, namespace string) *Scope {
	return &Scope{id, parent, root, namespace, map[string]*Binding{}}
}

func (s *Scope) Bind(name string, decl NodeID) (*Binding, bool) {
	if b, ok := s.bindings[name]; ok {
		return b, false
	}
	b := &Binding{BindingID(decl), name, decl}
	s.bindings[name] = b
	return b, true
}

func (s *Scope) Lookup(name string) (*Binding, *Scope, bool) {
	if b, ok := s.bindings[name]; ok {
		return b, s, true
	}
	if s.Parent != nil {
		return s.Parent.Lookup(name)
	}
	return nil, nil, false
}

func (s *Scope) NamespacedName(parts ...string) string {
	if s.Namespace == "" {
		return strings.Join(parts, ".")
	}
	return s.Namespace + "." + strings.Join(parts, ".")
}

type ScopeGraph struct {
	scopes        map[ScopeID]*Scope
	scopeByNodeID map[NodeID]*Scope
}

func BuildScopeGraph(ast *AST, roots []NodeID) *ScopeGraph {
	g := &ScopeGraph{map[ScopeID]*Scope{}, map[NodeID]*Scope{}}
	var nextScopeID uint64 = 1
	rootScope := NewScope(NodeID(0), ScopeID(0), nil, "")
	g.scopes[rootScope.ID] = rootScope
	scope := rootScope
	enterScope := func(nodeID NodeID, name string) func() {
		namespace := scope.Namespace
		if name != "" {
			if namespace == "" {
				namespace = name
			} else {
				namespace += "." + name
			}
		}
		scope = NewScope(nodeID, ScopeID(nextScopeID), scope, namespace)
		nextScopeID++
		g.scopes[scope.ID] = scope
		return func() { scope = scope.Parent }
	}
	var visit func(nodeID NodeID)
	visit = func(nodeID NodeID) {
		g.setNodeScope(nodeID, scope)
		switch kind := ast.Node(nodeID).Kind.(type) {
		case Block:
			if kind.CreateScope {
				defer enterScope(nodeID, "")()
			}
		case For:
			defer enterScope(nodeID, "")()
		case Module:
			// TODO: The prelude module binds into the root scope so all user code can
			// see its types. This needs a proper module/import system.
			if nodeID < PreludeFirstID {
				defer enterScope(nodeID, kind.Name)()
			}
		case Struct:
			defer enterScope(nodeID, kind.Name.Name)()
		case Shape:
			defer enterScope(nodeID, kind.Name.Name)()
		case FunDecl:
			defer enterScope(nodeID, kind.Name.Name)()
		case Fun:
			defer enterScope(nodeID, kind.Name.Name)()
		}
		ast.Walk(nodeID, visit)
	}
	for _, root := range roots {
		visit(root)
	}
	return g
}

func (g *ScopeGraph) NodeScope(nodeID NodeID) *Scope {
	scope, ok := g.scopeByNodeID[nodeID]
	if !ok {
		panic(base.Errorf("no scope for node %d", nodeID))
	}
	return scope
}

func (g *ScopeGraph) setNodeScope(nodeID NodeID, scope *Scope) {
	if existing, ok := g.scopeByNodeID[nodeID]; ok {
		if scope.ID != existing.ID {
			panic(
				base.Errorf("cannot set scope %s for node %s: scope already set to %s", scope.ID, nodeID, existing.ID),
			)
		}
		return
	}
	g.scopeByNodeID[nodeID] = scope
}
