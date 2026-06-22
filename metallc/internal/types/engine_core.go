package types

import (
	"fmt"
	"slices"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/modules"
)

// QueryContext bundles the transient state that flows through a Query
// recursion: stacks pushed/popped as we descend, a type hint set by callers,
// the current block-position cursor, and the current instantiation scope used
// by generic materialization. Lives on TypeContext so both Engine and
// Generics see it; the enter*/with* helpers are the only intended writers.
//
// Use with defer to keep stack invariants:
//
//	defer ctx.enterLoop(nodeID)()
//	defer ctx.withTypeHint(hint)()
type QueryContext struct {
	loopStack          []ast.NodeID
	funStack           []TypeID
	typeHint           *TypeID
	blockExprsIndex    int
	instantiationScope *ast.NodeID
}

func newQueryContext() QueryContext {
	return QueryContext{blockExprsIndex: -1} //nolint:exhaustruct
}

func (c *QueryContext) enterLoop(nodeID ast.NodeID) func() {
	return pushPop(&c.loopStack, nodeID)
}

func (c *QueryContext) enterFun(funTypeID TypeID) func() {
	return pushPop(&c.funStack, funTypeID)
}

func (c *QueryContext) withTypeHint(hint *TypeID) func() {
	return setRestore(&c.typeHint, hint)
}

func (c *QueryContext) withBlockIndex(index int) func() {
	return setRestore(&c.blockExprsIndex, index)
}

func (c *QueryContext) withInstantiationScope(scope *ast.NodeID) func() {
	return setRestore(&c.instantiationScope, scope)
}

func pushPop[T any](stack *[]T, value T) func() {
	*stack = append(*stack, value)
	return func() {
		*stack = (*stack)[:len(*stack)-1]
	}
}

func setRestore[T any](field *T, value T) func() {
	saved := *field
	*field = value
	return func() { *field = saved }
}

type TypeContext struct {
	QueryContext
	ast              *ast.AST
	debug            base.Debug
	diagnostics      base.Diagnostics
	scopeGraph       *ast.ScopeGraph
	env              *TypeEnv
	funs             map[string]FunWork
	structs          map[string]TypeWork
	unions           map[string]TypeWork
	shapes           map[string]TypeWork
	skipRegisterWork bool
	moduleResolution *modules.ModuleResolution
	neverTyp         TypeID
	recursionAborted bool
}

func NewTypeContext(
	a *ast.AST, g *ast.ScopeGraph, moduleResolution *modules.ModuleResolution,
) *TypeContext {
	return &TypeContext{ //nolint:exhaustruct
		QueryContext:     newQueryContext(),
		ast:              a,
		debug:            base.NilDebug{},
		scopeGraph:       g,
		env:              NewRootEnv(a, g),
		funs:             map[string]FunWork{},
		structs:          map[string]TypeWork{},
		unions:           map[string]TypeWork{},
		shapes:           map[string]TypeWork{},
		moduleResolution: moduleResolution,
	}
}

func (c *TypeContext) diag(span base.Span, msg string, msgArgs ...any) {
	// Unbounded generic recursion is fatal and spews a cascade of secondary
	// errors over the half-built deep types; report the root cause and go quiet.
	if c.recursionAborted {
		return
	}
	c.diagnostics = append(c.diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}

func (c *TypeContext) bind(
	nodeID ast.NodeID,
	name string,
	mut bool,
	typeID TypeID,
	span base.Span,
	blockExprsIndex int,
) bool {
	// A prelude top-level symbol is reserved: redefining or shadowing it in any scope
	// is an error.
	if existing, ok := c.lookup(nodeID, name, blockExprsIndex); ok &&
		ast.IsMinPreludeNode(c.ast, existing.Decl) && !ast.IsPreludeNode(nodeID) {
		switch c.ast.Node(existing.Decl).Kind.(type) {
		case ast.Struct, ast.Union, ast.Enum, ast.Shape, ast.Fun:
			c.diag(span, "reserved symbol: %s (defined in prelude)", name)
			return false
		}
	}
	if !c.env.bind(nodeID, name, mut, typeID, blockExprsIndex) && c.env.IsRoot() {
		c.diag(span, "symbol already defined: %s", name)
		return false
	}
	return true
}

func (c *TypeContext) lookup(nodeID ast.NodeID, name string, blockExprsIndex int) (*Binding, bool) {
	return c.env.Lookup(nodeID, name, blockExprsIndex)
}

// enterChildEnv pushes a fresh, anonymous child env. Each call creates a new
// env; nothing is cached.
func (c *TypeContext) enterChildEnv() func() {
	prev := c.env
	c.env = prev.NewChildEnv(0)
	return func() { c.env = prev }
}

// enterChildEnvFor pushes the child env keyed by `node`, creating it on first
// use. A subsequent call from the same parent returns the same env so bindings
// recorded by one caller are visible to a later one. Re-entering an env already
// keyed to the same node panics.
func (c *TypeContext) enterChildEnvFor(node ast.NodeID) func() {
	if c.env.node == node {
		panic(base.Errorf("enterChildEnvFor: re-entering env already keyed to node %s", node))
	}
	prev := c.env
	env := prev.childEnvs[node]
	if env == nil {
		env = prev.NewChildEnv(node)
		prev.childEnvs[node] = env
	}
	c.env = env
	return func() { c.env = prev }
}

func (c *TypeContext) registerFun(nodeID ast.NodeID) {
	funNode, ok := c.ast.Node(nodeID).Kind.(ast.Fun)
	if !ok {
		return
	}
	if funNode.Builtin || funNode.Extern {
		return
	}
	if c.skipRegisterWork {
		return
	}
	name, ok := c.env.NamedFunRef(nodeID)
	if !ok {
		panic(base.Errorf("no namespaced name for function node %s", nodeID))
	}
	if _, ok := c.funs[name]; !ok {
		if c.debug.Enabled() {
			c.debug.Print(1, "registerFun %s (node=%s)", name, nodeID)
		}
		c.recordFunWork(name, FunWork{NodeID: nodeID, TypeID: c.env.TypeOfNode(nodeID).ID, Name: name, Env: c.env})
	}
}

func (c *TypeContext) recordFunWork(name string, work FunWork) {
	c.funs[name] = work
}

func (c *TypeContext) loadFunWork(name string) (FunWork, bool) {
	work, ok := c.funs[name]
	return work, ok
}

func (c *TypeContext) recordStructWork(name string, work TypeWork) {
	c.structs[name] = work
}

func (c *TypeContext) loadStructWork(name string) (TypeWork, bool) {
	work, ok := c.structs[name]
	return work, ok
}

func (c *TypeContext) recordUnionWork(name string, work TypeWork) {
	c.unions[name] = work
}

func (c *TypeContext) loadUnionWork(name string) (TypeWork, bool) {
	work, ok := c.unions[name]
	return work, ok
}

func (c *TypeContext) recordShapeWork(name string, work TypeWork) {
	c.shapes[name] = work
}

func (c *TypeContext) loadShapeWork(name string) (TypeWork, bool) {
	work, ok := c.shapes[name]
	return work, ok
}

// Synthetic module for prelude nodes, which have no Module AST parent.
var preludeModule = ast.Module{ //nolint:gochecknoglobals
	FileName: "prelude", Name: "prelude", Main: false, Decls: nil,
}

var preludeModuleNode = &ast.Node{ //nolint:gochecknoglobals
	ID: ast.PreludeFirstID, Span: base.Span{}, Kind: preludeModule,
}

func (c *TypeContext) moduleOf(nodeID ast.NodeID) (*ast.Node, ast.Module) {
	scope := c.scopeGraph.NodeScope(nodeID)
	for scope != nil && scope.Node != 0 {
		scopeNode := c.ast.Node(scope.Node)
		if mod, ok := scopeNode.Kind.(ast.Module); ok {
			return scopeNode, mod
		}
		scope = scope.Parent
	}
	if !ast.IsPreludeNode(nodeID) {
		panic(base.Errorf("no module found for node %s", nodeID))
	}
	return preludeModuleNode, preludeModule
}

func (c *TypeContext) updateCachedType(
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
	// Only the node that declares a type owns its lifecycle status. A node that
	// merely references a still-forward-declared type (e.g. a struct field whose
	// type is a union not yet completed) shares the same cachedType pointer; if it
	// wrote its own <ok> here, the type's later genuine completion would trip the
	// transition guard as <ok> -> <failed dependency>.
	if cached.Type.NodeID == node.ID {
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
	}
	c.env.setNodeType(node.ID, cached)
	if status.Failed() {
		return InvalidTypeID, status
	}
	return typeID, status
}

func (c *TypeContext) declMangledName(nodeID ast.NodeID, name string) string {
	n := c.scopeGraph.NodeScope(nodeID).NamespacedName(name)
	if len(c.funStack) > 0 {
		parentTypeID := c.funStack[len(c.funStack)-1]
		if _, ok := c.env.GenericOrigin(parentTypeID); ok {
			n = fmt.Sprintf("%s.%s", n, parentTypeID)
		}
	}
	return n
}

func (c *TypeContext) isSync(typeID TypeID) bool {
	typ := c.env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case VoidType, NeverType, BoolType, IntType, FloatType, EnumType:
		return true
	case StructType:
		declNode := c.env.DeclNode(typeID)
		if declNode != 0 {
			if s, ok := c.ast.Node(declNode).Kind.(ast.Struct); ok && s.Sync != ast.SyncNone {
				return s.Sync == ast.SyncSync
			}
		}
		for _, field := range kind.Fields {
			if !c.isSync(field.Type) {
				return false
			}
		}
		return true
	case UnionType:
		declNode := c.env.DeclNode(typeID)
		if declNode != 0 {
			if u, ok := c.ast.Node(declNode).Kind.(ast.Union); ok && u.Sync != ast.SyncNone {
				return u.Sync == ast.SyncSync
			}
		}
		for _, variant := range kind.Variants {
			if !c.isSync(variant) {
				return false
			}
		}
		return true
	case ArrayType:
		return c.isSync(kind.Elem)
	case TypeParamType:
		declNode := c.env.DeclNode(typeID)
		if declNode != 0 {
			if tp, ok := c.ast.Node(declNode).Kind.(ast.TypeParam); ok {
				return tp.Sync == ast.SyncSync
			}
		}
		return false
	case FunType:
		return typeID&syncFunFlag != 0
	case RefType, SliceType, AllocatorType, ShapeType:
		return false
	default:
		panic(fmt.Sprintf("unknown type: %T", kind))
	}
}

func (c *TypeContext) isAssignableTo(got TypeID, expected TypeID) bool {
	if got == expected || got == c.neverTyp {
		return true
	}
	if got&mutableRefFlag != 0 && got&^mutableRefFlag == expected {
		return true
	}
	if got&mutableSliceFlag != 0 && got&^mutableSliceFlag == expected {
		return true
	}
	if got&syncFunFlag != 0 && got&^syncFunFlag == expected {
		return true
	}
	gotTyp := c.env.Type(got)
	expTyp := c.env.Type(expected)
	switch gotKind := gotTyp.Kind.(type) {
	case SliceType:
		// Only immutable slices are covariant in their element. `[]mut` is
		// invariant: widening the element (e.g. `[]mut &mut Int` to `[]mut &Int`)
		// lets you store a widened value through one alias and read it back at the
		// narrower element type through another. The top-level `[]mut T -> []T`
		// drop is handled by the mutableSliceFlag check above.
		if expKind, ok := expTyp.Kind.(SliceType); ok && !gotKind.Mut && !expKind.Mut {
			return c.isAssignableTo(gotKind.Elem, expKind.Elem)
		}
	case RefType:
		// Same invariance rule: `&mut` is invariant in its pointee, `&` is covariant.
		if expKind, ok := expTyp.Kind.(RefType); ok && !gotKind.Mut && !expKind.Mut {
			return c.isAssignableTo(gotKind.Type, expKind.Type)
		}
	case FunType:
		// Noescape differences are checked by lifetime analysis, not here.
		if expKind, ok := expTyp.Kind.(FunType); ok && gotKind.Macro == expKind.Macro &&
			gotKind.Return == expKind.Return && gotKind.Sync == expKind.Sync &&
			gotKind.Unsafe == expKind.Unsafe && slices.Equal(gotKind.Params, expKind.Params) {
			return true
		}
	case EnumType:
		// A subset of an open enum widens to its root (a no-op: same backing int
		// and the same whole-program tag).
		return gotKind.Root != InvalidTypeID && gotKind.Root == expected
	}
	return false
}

// lookupInTypeModule resolves a member of a type's home module, e.g.
// `String.len` finds `len` in the module that declared `String`.
func (c *TypeContext) lookupInTypeModule(typ *Type, name string) (*Binding, bool) {
	if len(c.moduleResolution.Imports) == 0 {
		return nil, false
	}
	if _, ok := typ.Kind.(ModuleType); ok {
		return nil, false
	}
	declNodeID := c.env.DeclNode(typ.ID)
	if declNodeID == 0 || ast.IsPreludeNode(declNodeID) {
		return nil, false
	}
	_, typModule := c.moduleOf(declNodeID)
	if len(typModule.Decls) == 0 {
		return nil, false
	}
	return c.env.Lookup(typModule.Decls[0], name, -1)
}

func (c *TypeContext) declIsPub(declNodeID ast.NodeID) bool {
	switch kind := c.ast.Node(declNodeID).Kind.(type) {
	case ast.Fun:
		return kind.Pub
	case ast.FunDecl:
		return kind.Pub
	case ast.Struct:
		return kind.Pub
	case ast.Shape:
		return kind.Pub
	case ast.Union:
		return kind.Pub
	case ast.Enum:
		return kind.Pub
	case ast.Var:
		return kind.Pub
	}
	return false
}

// isVisible reports whether a decl is visible from `from`. A "_test" companion
// module sees everything in its subject module.
func (c *TypeContext) isVisible(declNodeID ast.NodeID, pub bool, from ast.NodeID) bool {
	if pub {
		return true
	}
	if len(c.moduleResolution.Imports) == 0 {
		return true
	}
	if declNodeID == 0 {
		return true
	}
	fromModuleNode, fromModule := c.moduleOf(from)
	declModuleNode, declModule := c.moduleOf(declNodeID)
	if fromModuleNode.ID == declModuleNode.ID {
		return true
	}
	if subject, ok := strings.CutSuffix(fromModule.Name, "_test"); ok && subject == declModule.Name {
		return true
	}
	return false
}

// isMemberVisible reports whether member `name` of module `modNode` is reachable
// from `from`. Module-member visibility is distinct from decl visibility: a
// `use`-imported symbol's binding decl lives in a foreign module (so the real
// symbol resolves for construction, methods, generics), hence isVisible can't
// judge it. A symbol import is visible outside its module only when `pub use`
// re-exports it; any other member uses its own decl's pub.
func (c *TypeContext) isMemberVisible(modNode ast.NodeID, name string, b *Binding, from ast.NodeID) bool {
	if imp, ok := c.moduleResolution.ImportFor(modNode, name); ok && !imp.IsModule() {
		return imp.Pub
	}
	return c.isVisible(b.Decl, c.declIsPub(b.Decl), from)
}

func (c *TypeContext) funLitNeedsInference(fun ast.Fun) bool {
	if fun.ReturnType == ast.InferredType {
		return true
	}
	for _, paramNodeID := range fun.Params {
		if base.Cast[ast.FunParam](c.ast.Node(paramNodeID).Kind).Type == ast.InferredType {
			return true
		}
	}
	return false
}

// isTypeReference reports whether the expression at nodeID names a type
// rather than a value.
func (c *TypeContext) isTypeReference(nodeID ast.NodeID) bool {
	switch kind := c.ast.Node(nodeID).Kind.(type) {
	case ast.SimpleType:
		return true
	case ast.Ident:
		binding, ok := c.lookup(nodeID, kind.Name, -1)
		if !ok {
			return false
		}
		switch c.ast.Node(binding.Decl).Kind.(type) {
		case ast.Struct, ast.Union, ast.Shape, ast.Enum:
			return true
		}
	case ast.FieldAccess:
		if b, ok := c.env.PathBinding(nodeID); ok {
			switch c.ast.Node(b.Decl).Kind.(type) {
			case ast.Struct, ast.Union, ast.Shape, ast.Enum:
				return true
			}
		}
	}
	return false
}
