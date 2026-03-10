package types

import (
	"slices"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type QueryFunc func(nodeID ast.NodeID) (TypeID, TypeStatus)

type EngineCore struct {
	ast         *ast.AST
	debug       base.Debug
	diagnostics base.Diagnostics
	scopeGraph  *ast.ScopeGraph
	env         *TypeEnv
	funs        map[string]FunWork
	structs     map[string]StructWork
}

func NewEngineCore(a *ast.AST, g *ast.ScopeGraph) *EngineCore {
	return &EngineCore{ //nolint:exhaustruct
		ast:        a,
		debug:      base.NilDebug{},
		scopeGraph: g,
		env:        NewRootEnv(a, g),
		funs:       map[string]FunWork{},
		structs:    map[string]StructWork{},
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
	name, ok := c.env.NamedFunRef(nodeID)
	if !ok {
		panic(base.Errorf("no namespaced name for function node %s", nodeID))
	}
	// todo: Once we don't use the print_xxx functions anymore we can remove this.
	if slices.Contains([]string{
		"print_int", "print_uint", "print_str", "print_bool",
		"I8.to_i16", "I8.to_i32", "I8.to_int",
		"I16.to_i32", "I16.to_int",
		"I32.to_int",
		"U8.to_u16", "U8.to_u32", "U8.to_u64",
		"U16.to_u32", "U16.to_u64",
		"U32.to_u64",
		"U8.to_i16", "U8.to_i32", "U8.to_int",
		"U16.to_i32", "U16.to_int",
		"U32.to_int",
		"I16.to_i8_wrapping", "I16.to_i8_clamped",
		"I32.to_i8_wrapping", "I32.to_i8_clamped",
		"Int.to_i8_wrapping", "Int.to_i8_clamped",
		"I32.to_i16_wrapping", "I32.to_i16_clamped",
		"Int.to_i16_wrapping", "Int.to_i16_clamped",
		"Int.to_i32_wrapping", "Int.to_i32_clamped",
		"U16.to_u8_wrapping", "U16.to_u8_clamped",
		"U32.to_u8_wrapping", "U32.to_u8_clamped",
		"U64.to_u8_wrapping", "U64.to_u8_clamped",
		"U32.to_u16_wrapping", "U32.to_u16_clamped",
		"U64.to_u16_wrapping", "U64.to_u16_clamped",
		"U64.to_u32_wrapping", "U64.to_u32_clamped",
		"U8.to_i8_wrapping", "U8.to_i8_clamped",
		"U16.to_i16_wrapping", "U16.to_i16_clamped",
		"U16.to_i8_wrapping", "U16.to_i8_clamped",
		"U32.to_i32_wrapping", "U32.to_i32_clamped",
		"U32.to_i16_wrapping", "U32.to_i16_clamped",
		"U32.to_i8_wrapping", "U32.to_i8_clamped",
		"U64.to_int_wrapping", "U64.to_int_clamped",
		"U64.to_i32_wrapping", "U64.to_i32_clamped",
		"I8.to_u8_wrapping", "I8.to_u8_clamped",
		"I8.to_u16_wrapping", "I8.to_u16_clamped",
		"I8.to_u32_wrapping", "I8.to_u32_clamped",
		"I8.to_u64_wrapping", "I8.to_u64_clamped",
		"I16.to_u8_wrapping", "I16.to_u8_clamped",
		"I16.to_u16_wrapping", "I16.to_u16_clamped",
		"I16.to_u32_wrapping", "I16.to_u32_clamped",
		"I16.to_u64_wrapping", "I16.to_u64_clamped",
		"I32.to_u8_wrapping", "I32.to_u8_clamped",
		"I32.to_u16_wrapping", "I32.to_u16_clamped",
		"I32.to_u32_wrapping", "I32.to_u32_clamped",
		"I32.to_u64_wrapping", "I32.to_u64_clamped",
		"Int.to_u8_wrapping", "Int.to_u8_clamped",
		"Int.to_u16_wrapping", "Int.to_u16_clamped",
		"Int.to_u32_wrapping", "Int.to_u32_clamped",
		"Int.to_u64_wrapping", "Int.to_u64_clamped",
	}, name) {
		return
	}
	if _, ok := c.funs[name]; !ok {
		c.funs[name] = FunWork{NodeID: nodeID, TypeID: c.env.TypeOfNode(nodeID).ID, Name: name, Env: c.env}
	}
}

func (c *EngineCore) registerStruct(structType StructType, nodeID ast.NodeID, typeID TypeID) {
	if _, ok := c.structs[structType.Name]; !ok {
		c.structs[structType.Name] = StructWork{NodeID: nodeID, TypeID: typeID, Env: c.env}
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
