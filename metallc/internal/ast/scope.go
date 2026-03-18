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
	Bindings  map[string]*Binding
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
	if b, ok := s.Bindings[name]; ok {
		return b, b.Decl == decl
	}
	b := &Binding{BindingID(decl), name, decl}
	s.Bindings[name] = b
	return b, true
}

func (s *Scope) Lookup(name string) (*Binding, *Scope, bool) {
	if b, ok := s.Bindings[name]; ok {
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
	scopes          map[ScopeID]*Scope
	scopeByNodeID   map[NodeID]*Scope
	scopeByRootNode map[NodeID]*Scope
	nextScopeID     uint64
}

func BuildScopeGraph(ast *AST, roots []NodeID) *ScopeGraph {
	g := &ScopeGraph{map[ScopeID]*Scope{}, map[NodeID]*Scope{}, map[NodeID]*Scope{}, 1}
	rootScope := NewScope(NodeID(0), ScopeID(0), nil, "")
	g.scopes[rootScope.ID] = rootScope
	g.walkNodes(ast, roots, rootScope)
	return g
}

// WalkNodes walks new AST nodes into an existing module scope.
// Used by macro expansion to register expanded declarations in the calling module's scope.
func (g *ScopeGraph) WalkNodes(a *AST, nodeIDs []NodeID, moduleNodeID NodeID) {
	moduleScope := g.IntroducedScope(moduleNodeID)
	g.walkNodes(a, nodeIDs, moduleScope)
}

func (g *ScopeGraph) NodeScope(nodeID NodeID) *Scope {
	scope, ok := g.scopeByNodeID[nodeID]
	if !ok {
		panic(base.Errorf("no scope for node %d", nodeID))
	}
	return scope
}

func (g *ScopeGraph) IntroducedScope(nodeID NodeID) *Scope {
	scope, ok := g.scopeByRootNode[nodeID]
	if !ok {
		panic(base.Errorf("no own scope for node %d", nodeID))
	}
	return scope
}

func (g *ScopeGraph) walkNodes(a *AST, nodeIDs []NodeID, startScope *Scope) {
	scope := startScope
	enterScope := func(nodeID NodeID, name string) func() {
		namespace := scope.Namespace
		if name != "" {
			if namespace == "" {
				namespace = name
			} else {
				namespace += "." + name
			}
		}
		scope = NewScope(nodeID, ScopeID(g.nextScopeID), scope, namespace)
		g.nextScopeID++
		g.scopes[scope.ID] = scope
		g.scopeByRootNode[nodeID] = scope
		return func() { scope = scope.Parent }
	}
	var visit func(nodeID NodeID)
	visit = func(nodeID NodeID) {
		g.setNodeScope(nodeID, scope)
		switch kind := a.Node(nodeID).Kind.(type) {
		case Block:
			defer enterScope(nodeID, "")()
		case For:
			defer enterScope(nodeID, "")()
		case Match:
			visitMatch(a, g, kind, nodeID, visit, enterScope)
			return
		case Module:
			// Modules with an empty name (prelude) bind into the root
			// scope so all user code can see their declarations.
			if kind.Name != "" {
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
		a.Walk(nodeID, visit)
	}
	for _, root := range nodeIDs {
		visit(root)
	}
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

func visitMatch(
	a *AST,
	g *ScopeGraph,
	match Match,
	matchNodeID NodeID,
	visit func(NodeID),
	enterScope func(NodeID, string) func(),
) {
	visit(match.Expr)
	for _, arm := range match.Arms {
		visit(arm.Pattern)
		// Manually set the body block's scope and enter it so that
		// both the guard and the body's children live in the same scope.
		matchScope := g.NodeScope(matchNodeID)
		g.setNodeScope(arm.Body, matchScope)
		leaveScope := enterScope(arm.Body, "")
		if arm.Guard != nil {
			visit(*arm.Guard)
		}
		body := base.Cast[Block](a.Node(arm.Body).Kind)
		for _, expr := range body.Exprs {
			visit(expr)
		}
		leaveScope()
	}
	if match.Else != nil {
		visit(match.Else.Body)
	}
}
