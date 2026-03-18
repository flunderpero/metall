package types

import (
	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type EngineCore struct {
	ast         *ast.AST
	debug       base.Debug
	diagnostics base.Diagnostics
	scopeGraph  *ast.ScopeGraph
	env         *TypeEnv
	funs        map[string]FunWork
	structs     map[string]StructWork
	unions      map[string]UnionWork
}

func NewEngineCore(a *ast.AST, g *ast.ScopeGraph) *EngineCore {
	return &EngineCore{ //nolint:exhaustruct
		ast:        a,
		debug:      base.NilDebug{},
		scopeGraph: g,
		env:        NewRootEnv(a, g),
		funs:       map[string]FunWork{},
		structs:    map[string]StructWork{},
		unions:     map[string]UnionWork{},
	}
}

func (c *EngineCore) diag(span base.Span, msg string, msgArgs ...any) {
	c.diagnostics = append(c.diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}

func (c *EngineCore) bind(nodeID ast.NodeID, name string, mut bool, typeID TypeID, span base.Span) bool {
	if !c.env.bind(nodeID, name, mut, typeID) && c.env.IsRoot() {
		c.diag(span, "symbol already defined: %s", name)
		return false
	}
	return true
}

func (c *EngineCore) lookup(nodeID ast.NodeID, name string) (*Binding, bool) {
	return c.env.Lookup(nodeID, name)
}

func (c *EngineCore) enterChildEnv() func() {
	prev := c.env
	c.env = c.env.NewChildEnv()
	return func() { c.env = prev }
}

func (c *EngineCore) registerFun(nodeID ast.NodeID) {
	funNode, ok := c.ast.Node(nodeID).Kind.(ast.Fun)
	if !ok {
		return
	}
	if funNode.Extern {
		return
	}
	name, ok := c.env.NamedFunRef(nodeID)
	if !ok {
		panic(base.Errorf("no namespaced name for function node %s", nodeID))
	}
	if _, ok := c.funs[name]; !ok {
		c.debug.Print(1, "registerFun %s (node=%s)", name, nodeID)
		c.funs[name] = FunWork{NodeID: nodeID, TypeID: c.env.TypeOfNode(nodeID).ID, Name: name, Env: c.env}
	}
}

func (c *EngineCore) registerStruct(structType StructType, nodeID ast.NodeID, typeID TypeID) {
	structNode, ok := c.ast.Node(nodeID).Kind.(ast.Struct)
	if !ok || structNode.Extern {
		return
	}
	if _, ok := c.structs[structType.Name]; !ok {
		c.structs[structType.Name] = StructWork{NodeID: nodeID, TypeID: typeID, Env: c.env}
	}
}

func (c *EngineCore) registerUnion(unionType UnionType, nodeID ast.NodeID, typeID TypeID) {
	if _, ok := c.unions[unionType.Name]; !ok {
		c.unions[unionType.Name] = UnionWork{NodeID: nodeID, TypeID: typeID, Env: c.env}
	}
}

func (c *EngineCore) namespacedName(nodeID ast.NodeID, name string) string {
	return c.scopeGraph.NodeScope(nodeID).NamespacedName(name)
}

func (c *EngineCore) moduleOf(nodeID ast.NodeID) (*ast.Node, ast.Module) {
	scope := c.scopeGraph.NodeScope(nodeID)
	for {
		scopeNode := c.ast.Node(scope.Node)
		if mod, ok := scopeNode.Kind.(ast.Module); ok {
			return scopeNode, mod
		}
		scope = scope.Parent
		if scope == nil {
			panic(base.Errorf("no module found for node %s", nodeID))
		}
	}
}

func (c *EngineCore) updateCachedType(
	node *ast.Node, typeID TypeID, status TypeStatus,
) (TypeID, TypeStatus) {
	if typeID == InvalidTypeID {
		if !status.Failed() {
			panic(
				base.Errorf(
					"InvalidTypeID requires a failed status but got %s at %s",
					status,
					c.ast.Debug(node.ID, false, 0),
				),
			)
		}
		c.env.setNodeType(node.ID, &cachedType{Type: nil, Status: status})
		return InvalidTypeID, status
	}
	cached, ok := c.env.cachedTypeInfo(typeID)
	if !ok {
		panic(base.Errorf("type %s not found for %s", typeID, c.ast.Debug(node.ID, false, 0)))
	}
	if cached.Status != status && cached.Status != TypeInProgress {
		panic(
			base.Errorf(
				"invalid status transition for type %s of %s: %s -> %s",
				typeID,
				c.ast.Debug(node.ID, false, 0),
				cached.Status,
				status,
			),
		)
	}
	cached.Status = status
	c.env.setNodeType(node.ID, cached)
	if status.Failed() {
		return InvalidTypeID, status
	}
	return typeID, status
}
