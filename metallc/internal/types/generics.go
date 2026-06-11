package types

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

// Generics owns materialization, type-argument inference, and shape constraint
// machinery. It uses the embedded *TypeContext's callbacks to recursively
// type-check AST nodes but holds no back-reference to *Engine.
type Generics struct {
	*TypeContext

	// implicitOwnerArgs maps a generic owner's origin TypeID to the TypeIDs
	// that stand in for its type parameters in the current method's scope, so
	// a bare reference like `b Box` materializes as `Box<T0, T1, ...>`.
	implicitOwnerArgs map[TypeID][]TypeID
	// concreteAssociated remembers associated-type values inferred during
	// satisfiesShape, so projections like `C.Value` resolve after C has been
	// bound to a concrete type during method materialization.
	concreteAssociated map[TypeID]map[string]TypeID
	pendingTasks       []Task
	query              func(ast.NodeID) (TypeID, TypeStatus)
	queryWithHint      func(ast.NodeID, *TypeID) (TypeID, TypeStatus)
}

type Outcome struct {
	Handled bool
	TypeID  TypeID
	Status  TypeStatus
	Tasks   []Task
}

type Task struct {
	BodyCheck      *BodyCheckTask
	RegisterStruct *RegisterStructTask
	RegisterUnion  *RegisterUnionTask
	RegisterFun    ast.NodeID
}

type BodyCheckTask struct {
	Mat       FunMaterialization
	FunTypeID TypeID
}

type RegisterStructTask struct {
	Type       StructType
	DeclNodeID ast.NodeID
	TypeID     TypeID
}

type RegisterUnionTask struct {
	Type       UnionType
	DeclNodeID ast.NodeID
	TypeID     TypeID
}

func newGenerics(
	core *TypeContext,
	query func(ast.NodeID) (TypeID, TypeStatus),
	queryWithHint func(ast.NodeID, *TypeID) (TypeID, TypeStatus),
) *Generics {
	return &Generics{
		TypeContext:        core,
		implicitOwnerArgs:  map[TypeID][]TypeID{},
		concreteAssociated: map[TypeID]map[string]TypeID{},
		pendingTasks:       nil,
		query:              query,
		queryWithHint:      queryWithHint,
	}
}

func (g *Generics) funTypeParams(funNodeID ast.NodeID) []ast.NodeID {
	return funTypeParamsOf(g.env, g.ast, funNodeID)
}

func funTypeParamsOf(env *TypeEnv, a *ast.AST, funNodeID ast.NodeID) []ast.NodeID {
	if ext, ok := env.FunTypeParams(funNodeID); ok {
		return ext
	}
	switch kind := a.Node(funNodeID).Kind.(type) {
	case ast.Fun:
		return kind.TypeParams
	case ast.FunDecl:
		return kind.TypeParams
	}
	return nil
}

func (g *Generics) Resolve(nodeID ast.NodeID, typeHint *TypeID) Outcome {
	node := g.ast.Node(nodeID)
	var out Outcome
	switch kind := node.Kind.(type) {
	case ast.SimpleType:
		out = g.resolveSimpleType(nodeID, kind, node.Span)
	case ast.Ident:
		out = g.resolveIdent(nodeID, kind, node.Span)
	case ast.FieldAccess:
		out = g.resolveFieldAccess(nodeID, kind, node.Span)
	case ast.Call:
		typeID, status, handled := g.inferFunCall(kind, node.Span)
		if handled {
			out = Outcome{Handled: true, TypeID: typeID, Status: status}
		}
	case ast.TypeConstruction:
		typeID, status, handled := g.inferTypeConstruction(kind, node.Span, typeHint)
		if handled {
			out = Outcome{Handled: true, TypeID: typeID, Status: status}
		}
	case ast.Shape:
		out = g.resolveShapeDecl(node, kind)
	case ast.Fun:
		out = g.resolveFunDecl(node, kind.FunDecl)
	case ast.FunDecl:
		out = g.resolveFunDecl(node, kind)
	case ast.Struct:
		out = g.resolveCompositeDecl(node, kind.TypeParams)
	case ast.Union:
		out = g.resolveCompositeDecl(node, kind.TypeParams)
	}
	out.Tasks = g.drainTasks()
	return out
}

func (g *Generics) resolveSimpleType(nodeID ast.NodeID, st ast.SimpleType, span base.Span) Outcome {
	if base_, projection, hasDot := strings.Cut(st.Name.Name, "."); hasDot {
		if typeID, status, handled := g.resolveAssociatedType(nodeID, base_, projection, st.TypeArgs, span); handled {
			return Outcome{Handled: true, TypeID: typeID, Status: status}
		}
	}
	binding, ok := g.lookup(nodeID, st.Name.Name, -1)
	if !ok {
		return Outcome{}
	}
	if len(st.TypeArgs) > 0 {
		switch g.env.Type(binding.TypeID).Kind.(type) {
		case StructType, UnionType, ShapeType:
			typeID, status := g.materializeNamedTypeRef(binding.TypeID, st.TypeArgs, span)
			return Outcome{Handled: true, TypeID: typeID, Status: status}
		default:
			g.diag(span, "type arguments on non-generic type: %s", st.Name.Name)
			return Outcome{Handled: true, TypeID: InvalidTypeID, Status: TypeFailed}
		}
	}
	if typeID, status, handled := g.resolveImplicitGenericRef(binding, span); handled {
		return Outcome{Handled: true, TypeID: typeID, Status: status}
	}
	if out := g.resolveBinding(nodeID, binding, nil, span); out.Handled {
		return out
	}
	cached, ok := g.env.cachedTypeInfo(binding.TypeID)
	if ok {
		return Outcome{Handled: true, TypeID: binding.TypeID, Status: cached.Status}
	}
	return Outcome{Handled: true, TypeID: binding.TypeID, Status: TypeOK}
}

func (g *Generics) resolveIdent(nodeID ast.NodeID, ident ast.Ident, span base.Span) Outcome {
	binding, ok := g.env.LocalPathBinding(nodeID)
	if !ok {
		binding, ok = g.lookup(nodeID, ident.Name, -1)
	}
	if !ok || binding.Decl == 0 {
		return Outcome{}
	}
	return g.resolveBinding(nodeID, binding, ident.TypeArgs, span)
}

func (g *Generics) resolveBinding(
	nodeID ast.NodeID, binding *Binding, typeArgs []ast.NodeID, span base.Span,
) Outcome {
	declKind := g.ast.Node(binding.Decl).Kind
	if _, isFun := declKind.(ast.Fun); isFun {
		if len(typeArgs) == 0 && len(g.funTypeParams(binding.Decl)) == 0 {
			g.env.copyNamedFunRef(nodeID, binding.Decl)
			g.pendingTasks = append(g.pendingTasks, Task{RegisterFun: binding.Decl})
			return g.bindingOutcome(binding)
		}
		typeID, status := g.materializeFunRef(binding, nodeID, span, typeArgs)
		return Outcome{Handled: true, TypeID: typeID, Status: status}
	}
	if _, isFunDecl := declKind.(ast.FunDecl); isFunDecl {
		g.env.copyNamedFunRef(nodeID, binding.Decl)
		return g.bindingOutcome(binding)
	}
	if structType, ok := g.env.Type(binding.TypeID).Kind.(StructType); ok {
		if structNode, ok := declKind.(ast.Struct); ok {
			if len(typeArgs) > 0 {
				typeID, status := g.materializeNamedTypeRef(binding.TypeID, typeArgs, span)
				return Outcome{Handled: true, TypeID: typeID, Status: status}
			}
			if len(structNode.TypeParams) == 0 {
				g.pendingTasks = append(g.pendingTasks, Task{
					RegisterStruct: &RegisterStructTask{
						Type:       structType,
						DeclNodeID: binding.Decl,
						TypeID:     binding.TypeID,
					},
				})
			}
			return g.bindingOutcome(binding)
		}
	}
	if unionType, ok := g.env.Type(binding.TypeID).Kind.(UnionType); ok {
		if unionNode, ok := declKind.(ast.Union); ok {
			if len(typeArgs) > 0 {
				typeID, status := g.materializeNamedTypeRef(binding.TypeID, typeArgs, span)
				return Outcome{Handled: true, TypeID: typeID, Status: status}
			}
			if len(unionNode.TypeParams) == 0 {
				g.pendingTasks = append(g.pendingTasks, Task{
					RegisterUnion: &RegisterUnionTask{
						Type:       unionType,
						DeclNodeID: binding.Decl,
						TypeID:     binding.TypeID,
					},
				})
			}
			return g.bindingOutcome(binding)
		}
	}
	return Outcome{}
}

func (g *Generics) bindingOutcome(binding *Binding) Outcome {
	if cached, ok := g.env.cachedTypeInfo(binding.TypeID); ok {
		return Outcome{Handled: true, TypeID: binding.TypeID, Status: cached.Status}
	}
	return Outcome{Handled: true, TypeID: binding.TypeID, Status: TypeOK}
}

func (g *Generics) resolveFieldAccess(nodeID ast.NodeID, fa ast.FieldAccess, span base.Span) Outcome {
	if binding, ok := g.env.LocalPathBinding(nodeID); ok && binding.Decl != 0 {
		if _, isFunDecl := g.ast.Node(binding.Decl).Kind.(ast.FunDecl); isFunDecl {
			if out, ok := g.tryResolveShapeMethodFieldAccess(nodeID, fa, binding); ok {
				return out
			}
		}
		if _, isFun := g.ast.Node(binding.Decl).Kind.(ast.Fun); isFun {
			if len(fa.TypeArgs) > 0 || len(g.funTypeParams(binding.Decl)) > 0 {
				return g.resolveMethodCallFieldAccess(nodeID, fa, binding)
			}
		}
		return g.resolveBinding(nodeID, binding, fa.TypeArgs, span)
	}
	if out, ok := g.tryResolveShapeFieldAccess(fa); ok {
		return out
	}
	if out, ok := g.tryResolveMethod(nodeID, fa); ok {
		return out
	}
	return Outcome{}
}

func (g *Generics) tryResolveMethod(nodeID ast.NodeID, fa ast.FieldAccess) (Outcome, bool) {
	targetTyp, status := g.fieldAccessTargetTyp(fa)
	if status.Failed() {
		return Outcome{}, false
	}
	methodName := fa.Field.Name
	binding, ok := g.lookupMethod(nodeID, targetTyp, methodName)
	if !ok {
		return Outcome{}, false
	}
	if !g.isVisible(binding.Decl, g.declIsPub(binding.Decl), nodeID) {
		g.diag(fa.Field.Span, "method %s.%s is not public", g.env.TypeDisplay(targetTyp.ID), methodName)
		return Outcome{Handled: true, TypeID: InvalidTypeID, Status: TypeFailed}, true
	}
	// Set immediately so the self-recursion below sees the binding via
	// LocalPathBinding; this is not a Generics→Engine handoff (no Task).
	g.env.SetPathBinding(nodeID, binding)
	return g.resolveFieldAccess(nodeID, fa, g.ast.Node(nodeID).Span), true
}

func (g *Generics) lookupMethod(nodeID ast.NodeID, targetTyp *Type, methodName string) (*Binding, bool) {
	lookupName, ok := g.env.methodFQN(targetTyp, methodName)
	if !ok {
		return nil, false
	}
	binding, ok := g.lookup(nodeID, lookupName, -1)
	if !ok && g.instantiationScope != nil {
		binding, ok = g.lookup(*g.instantiationScope, lookupName, -1)
	}
	if !ok {
		binding, ok = g.lookupInTypeModule(targetTyp, lookupName)
	}
	if !ok {
		if structType, isStruct := targetTyp.Kind.(StructType); isStruct && len(structType.TypeArgs) > 0 {
			structNodeID := g.env.DeclNode(targetTyp.ID)
			structNode := base.Cast[ast.Struct](g.ast.Node(structNodeID).Kind)
			structName := g.scopeGraph.NodeScope(structNodeID).NamespacedName(structNode.Name.Name)
			lookupName = structName + "." + methodName
			binding, ok = g.lookup(nodeID, lookupName, -1)
			if !ok {
				binding, ok = g.lookupInTypeModule(targetTyp, lookupName)
			}
		}
	}
	if !ok {
		if unionType, isUnion := targetTyp.Kind.(UnionType); isUnion && len(unionType.TypeArgs) > 0 {
			unionNodeID := g.env.DeclNode(targetTyp.ID)
			unionNode := base.Cast[ast.Union](g.ast.Node(unionNodeID).Kind)
			unionName := g.scopeGraph.NodeScope(unionNodeID).NamespacedName(unionNode.Name.Name)
			lookupName = unionName + "." + methodName
			binding, ok = g.lookup(nodeID, lookupName, -1)
			if !ok {
				binding, ok = g.lookupInTypeModule(targetTyp, lookupName)
			}
		}
	}
	if !ok {
		// A subset enum inherits methods defined on its open root.
		if enumKind, isEnum := targetTyp.Kind.(EnumType); isEnum && enumKind.Root != InvalidTypeID {
			return g.lookupMethod(nodeID, g.env.Type(enumKind.Root), methodName)
		}
	}
	return binding, ok
}

func (g *Generics) tryResolveShapeFieldAccess(fa ast.FieldAccess) (Outcome, bool) {
	targetTyp, status := g.fieldAccessTargetTyp(fa)
	if status.Failed() {
		return Outcome{}, false
	}
	typeParamType, ok := targetTyp.Kind.(TypeParamType)
	if !ok || typeParamType.Shape == nil {
		return Outcome{}, false
	}
	shapeType := base.Cast[ShapeType](g.env.Type(*typeParamType.Shape).Kind)
	for _, field := range shapeType.Fields {
		if field.Name == fa.Field.Name {
			return Outcome{Handled: true, TypeID: field.Type, Status: TypeOK}, true
		}
	}
	return Outcome{}, false
}

func (g *Generics) tryResolveShapeMethodFieldAccess(
	nodeID ast.NodeID,
	fa ast.FieldAccess,
	binding *Binding,
) (Outcome, bool) {
	targetTyp, status := g.fieldAccessTargetTyp(fa)
	if status.Failed() {
		return Outcome{Handled: true, TypeID: InvalidTypeID, Status: status}, true
	}
	if _, isShape := targetTyp.Kind.(ShapeType); !isShape {
		if _, isParam := targetTyp.Kind.(TypeParamType); !isParam {
			return Outcome{}, false
		}
	}
	g.env.setNamedFunRef(nodeID, binding.Name)
	funType, status := g.methodSignature(binding, g.boundMethodContext(binding, targetTyp.ID), nodeID, fa.Field.Span)
	if status.Failed() {
		return Outcome{Handled: true, TypeID: InvalidTypeID, Status: status}, true
	}
	newID := g.env.newType(funType, 0, base.Span{}, TypeOK)
	return Outcome{Handled: true, TypeID: newID, Status: TypeOK}, true
}

func (g *Generics) resolveMethodCallFieldAccess(nodeID ast.NodeID, fa ast.FieldAccess, binding *Binding) Outcome {
	targetTyp, status := g.fieldAccessTargetTyp(fa)
	if status.Failed() {
		return Outcome{Handled: true, TypeID: InvalidTypeID, Status: status}
	}
	span := fa.Field.Span
	if _, isModule := targetTyp.Kind.(ModuleType); isModule {
		span = g.ast.Node(nodeID).Span
	}
	var providedTypeArgIDs []TypeID
	if typeArgs, ok := ImplicitTypeArgs(targetTyp.Kind); ok {
		providedTypeArgIDs = append(providedTypeArgIDs, typeArgs...)
	}
	extraArgs, status := g.queryTypeArgs(fa.TypeArgs)
	if status.Failed() {
		return Outcome{Handled: true, TypeID: InvalidTypeID, Status: status}
	}
	providedTypeArgIDs = append(providedTypeArgIDs, extraArgs...)
	typeID, status := g.solveAndMaterializeFun(binding, nodeID, span, providedTypeArgIDs)
	return Outcome{Handled: true, TypeID: typeID, Status: status}
}

func (g *Generics) fieldAccessTargetTyp(fa ast.FieldAccess) (*Type, TypeStatus) {
	targetTypeID, status := g.query(fa.Target)
	if status.Failed() {
		return nil, status
	}
	targetTyp := g.env.Type(targetTypeID)
	if refTyp, ok := targetTyp.Kind.(RefType); ok {
		targetTyp = g.env.Type(refTyp.Type)
	}
	return targetTyp, TypeOK
}

// resolveCompositeDecl binds type params into the child env keyed by node.ID
// during Phase 2 of struct/union resolution, so Engine's field walk in the
// same keyed child env sees the bindings.
func (g *Generics) resolveCompositeDecl(node *ast.Node, typeParams []ast.NodeID) Outcome {
	cached, ok := g.env.cachedNodeType(node.ID)
	if !ok {
		return Outcome{}
	}
	if cached.Status != TypeInProgress {
		return Outcome{Handled: true, TypeID: cached.Type.ID, Status: cached.Status}
	}
	defer g.enterChildEnvFor(node.ID)()
	status := g.bindTypeParams(typeParams)
	return Outcome{Handled: true, TypeID: cached.Type.ID, Status: status}
}

func (g *Generics) resolveFunDecl(node *ast.Node, fun ast.FunDecl) Outcome {
	if cached, ok := g.env.cachedNodeType(node.ID); ok {
		return Outcome{Handled: true, TypeID: cached.Type.ID, Status: cached.Status}
	}
	status := g.prepareFunTypeParams(node, fun)
	return Outcome{Handled: true, TypeID: InvalidTypeID, Status: status}
}

func (g *Generics) resolveShapeDecl(node *ast.Node, shapeNode ast.Shape) Outcome {
	cached, ok := g.env.cachedNodeType(node.ID)
	if !ok {
		return Outcome{}
	}
	if cached.Status != TypeInProgress {
		return Outcome{Handled: true, TypeID: cached.Type.ID, Status: cached.Status}
	}
	shapeType := base.Cast[ShapeType](cached.Type.Kind)
	status, newKind := g.completeShapeType(node, shapeNode, shapeType)
	cached.Type.Kind = newKind
	return Outcome{Handled: true, TypeID: cached.Type.ID, Status: status}
}

// resolveImplicitGenericRef handles a bare reference to a generic
// struct/union/shape (e.g. `Foo`). If the engine recorded implicit owner args
// for that origin, the call materializes with those args; otherwise it falls
// back to the unresolved-template form for downstream inference.
func (g *Generics) resolveImplicitGenericRef(binding *Binding, span base.Span) (TypeID, TypeStatus, bool) {
	var hasTypeParams bool
	switch g.env.Type(binding.TypeID).Kind.(type) {
	case StructType:
		if s, ok := g.ast.Node(binding.Decl).Kind.(ast.Struct); ok {
			hasTypeParams = len(s.TypeParams) > 0
		}
	case UnionType:
		if u, ok := g.ast.Node(binding.Decl).Kind.(ast.Union); ok {
			hasTypeParams = len(u.TypeParams) > 0
		}
	case ShapeType:
		if sh, ok := g.ast.Node(binding.Decl).Kind.(ast.Shape); ok {
			hasTypeParams = len(sh.TypeParams) > 0
		}
	default:
		return InvalidTypeID, TypeOK, false
	}
	if !hasTypeParams {
		return InvalidTypeID, TypeOK, false
	}
	if implicitArgs, ok := g.implicitOwnerArgs[binding.TypeID]; ok {
		typeID, status := g.materializeNamedType(binding.TypeID, implicitArgs)
		return typeID, status, true
	}
	typeID, status := g.materializeNamedTypeRef(binding.TypeID, nil, span)
	return typeID, status, true
}

func (g *Generics) resolveAssociatedType(
	nodeID ast.NodeID, baseName, projection string, typeArgs []ast.NodeID, span base.Span,
) (TypeID, TypeStatus, bool) {
	binding, ok := g.lookup(nodeID, baseName, -1)
	if !ok {
		g.diag(span, "unknown associated type: %s.%s", baseName, projection)
		return InvalidTypeID, TypeFailed, true
	}
	baseTypeID := binding.TypeID
	for {
		head, rest, hasMore := strings.Cut(projection, ".")
		if !hasMore {
			break
		}
		resolved, status, handled := g.projectAssociatedType(baseTypeID, head)
		if !handled {
			return InvalidTypeID, TypeFailed, false
		}
		if status.Failed() {
			return InvalidTypeID, status, true
		}
		baseTypeID = resolved
		projection = rest
	}
	if _, isParam := g.env.Type(baseTypeID).Kind.(TypeParamType); !isParam {
		if typeID, ok := g.lookupAssociatedType(baseTypeID, projection); ok {
			return typeID, TypeOK, true
		}
		return InvalidTypeID, TypeFailed, false
	}
	tpt := base.Cast[TypeParamType](g.env.Type(baseTypeID).Kind)
	if tpt.Shape == nil {
		g.diag(span, "type parameter %s is unconstrained: cannot project %s", baseName, projection)
		return InvalidTypeID, TypeFailed, true
	}
	if len(typeArgs) > 0 {
		g.diag(span, "associated type %s.%s does not take type arguments", baseName, projection)
		return InvalidTypeID, TypeFailed, true
	}
	resolved, status, handled := g.projectAssociatedType(baseTypeID, projection)
	if !handled {
		g.diag(span, "unknown associated type: %s.%s", baseName, projection)
		return InvalidTypeID, TypeFailed, true
	}
	return resolved, status, true
}

func (g *Generics) projectAssociatedType(
	baseTypeID TypeID, projection string,
) (TypeID, TypeStatus, bool) {
	tpt, isParam := g.env.Type(baseTypeID).Kind.(TypeParamType)
	if !isParam {
		typeID, ok := g.lookupAssociatedType(baseTypeID, projection)
		if !ok {
			return InvalidTypeID, TypeFailed, false
		}
		return typeID, TypeOK, true
	}
	if tpt.Shape == nil {
		return InvalidTypeID, TypeFailed, false
	}
	shapeTypeID := *tpt.Shape
	shapeType := base.Cast[ShapeType](g.env.Type(shapeTypeID).Kind)
	shapeOrigin := shapeTypeID
	if origin, hasOrigin := g.env.GenericOrigin(shapeTypeID); hasOrigin {
		shapeOrigin = origin
	}
	shapeNode, ok := g.ast.Node(g.env.DeclNode(shapeOrigin)).Kind.(ast.Shape)
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	for i, paramNodeID := range shapeNode.TypeParams {
		paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
		if paramNode.Name.Name != projection {
			continue
		}
		if i < len(shapeType.TypeArgs) {
			return shapeType.TypeArgs[i], TypeOK, true
		}
		if typeParamID, ok := g.env.TypeParamForNode(paramNodeID); ok {
			return typeParamID, TypeOK, true
		}
		return InvalidTypeID, TypeFailed, true
	}
	return InvalidTypeID, TypeFailed, false
}

func (g *Generics) lookupAssociatedType(concreteTypeID TypeID, projection string) (TypeID, bool) {
	bindings, ok := g.concreteAssociated[concreteTypeID]
	if !ok {
		return InvalidTypeID, false
	}
	typeID, ok := bindings[projection]
	return typeID, ok
}

func (g *Generics) scheduleBodyCheck(mat FunMaterialization, funTypeID TypeID) {
	g.pendingTasks = append(g.pendingTasks, Task{
		BodyCheck: &BodyCheckTask{Mat: mat, FunTypeID: funTypeID},
	})
}

func (g *Generics) drainTasks() []Task {
	tasks := g.pendingTasks
	g.pendingTasks = nil
	return tasks
}

func (g *Generics) resolveCallBinding(call ast.Call) (*Binding, bool) {
	calleeNode := g.ast.Node(call.Callee)
	switch kind := calleeNode.Kind.(type) {
	case ast.Ident:
		binding, ok := g.lookup(call.Callee, kind.Name, -1)
		return binding, ok && binding.Decl != 0
	case ast.FieldAccess:
		if ident, ok := g.ast.Node(kind.Target).Kind.(ast.Ident); ok {
			modBinding, ok := g.lookup(call.Callee, ident.Name, -1)
			if ok {
				if _, isMod := g.env.Type(modBinding.TypeID).Kind.(ModuleType); isMod {
					thisModuleNode, _ := g.moduleOf(call.Callee)
					importedModuleNodeID, ok := g.moduleResolution.Imports[thisModuleNode.ID][ident.Name]
					if !ok {
						return nil, false
					}
					mod := base.Cast[ast.Module](g.ast.Node(importedModuleNodeID).Kind)
					if len(mod.Decls) == 0 {
						return nil, false
					}
					binding, ok := g.env.Lookup(mod.Decls[0], kind.Field.Name, -1)
					return binding, ok
				}
			}
		}
		return g.resolveMethodCallBinding(call.Callee, kind)
	default:
		return nil, false
	}
}

func (g *Generics) resolveMethodCallBinding(calleeNodeID ast.NodeID, fieldAccess ast.FieldAccess) (*Binding, bool) {
	targetTypeID, status := g.query(fieldAccess.Target)
	if status.Failed() {
		return nil, false
	}
	return g.lookupMethodBinding(calleeNodeID, targetTypeID, fieldAccess.Field.Name)
}

func (g *Generics) lookupMethodBinding(
	scopeNodeID ast.NodeID,
	targetTypeID TypeID,
	methodName string,
) (*Binding, bool) {
	targetTyp := g.env.Type(targetTypeID)
	if refTyp, ok := targetTyp.Kind.(RefType); ok {
		targetTypeID = refTyp.Type
		targetTyp = g.env.Type(targetTypeID)
	}
	lookupType := targetTyp
	if originID, ok := g.env.GenericOrigin(targetTypeID); ok {
		lookupType = g.env.Type(originID)
	}
	lookupName, ok := g.env.methodFQN(lookupType, methodName)
	if !ok {
		return nil, false
	}
	binding, ok := g.lookup(scopeNodeID, lookupName, -1)
	if !ok {
		binding, ok = g.lookupInTypeModule(lookupType, lookupName)
	}
	if !ok || binding.Decl == 0 {
		// A subset enum inherits methods defined on its open root.
		if enumKind, isEnum := lookupType.Kind.(EnumType); isEnum && enumKind.Root != InvalidTypeID {
			return g.lookupMethodBinding(scopeNodeID, enumKind.Root, methodName)
		}
		return nil, false
	}
	return binding, true
}

type FunMaterialization struct {
	NeedsBodyCheck bool
	FunNodeID      ast.NodeID
	FunType        FunType
	CallSiteNodeID ast.NodeID
	SkipRegister   bool
	Env            *TypeEnv
}

func (g *Generics) materializeFunRef(
	binding *Binding, callSiteNodeID ast.NodeID, span base.Span, typeArgs []ast.NodeID,
) (TypeID, TypeStatus) {
	typeArgIDs, status := g.queryTypeArgs(typeArgs)
	if status.Failed() {
		return InvalidTypeID, status
	}
	return g.solveAndMaterializeFun(binding, callSiteNodeID, span, typeArgIDs)
}

func (g *Generics) solveAndMaterializeFun(
	binding *Binding, callSiteNodeID ast.NodeID, span base.Span, providedTypeArgIDs []TypeID,
) (TypeID, TypeStatus) {
	decl, _ := g.normalizeGenericDecl(binding.Decl, binding.TypeID, "")
	spec, status := g.buildGenericSpec(decl.originTypeID, decl.typeParams)
	if status.Failed() {
		return InvalidTypeID, status
	}
	typeArgIDs, status := g.solveGenericArgs(spec, providedTypeArgIDs, callSiteNodeID, span)
	if status.Failed() {
		return InvalidTypeID, status
	}
	mat, typeID, mangledName, status := g.materializeFun(
		binding.Decl, binding.TypeID, callSiteNodeID, typeArgIDs,
	)
	if status.Failed() {
		return InvalidTypeID, status
	}
	g.scheduleBodyCheck(mat, typeID)
	g.env.setNamedFunRef(callSiteNodeID, mangledName)
	return typeID, TypeOK
}

// materializeFun computes the materialized FunType for a generic function at
// a call site. The returned mat carries the env to re-enter for the body
// check, which callers schedule via scheduleBodyCheck.
// genericDepthLimit bounds how deeply a generic's type arguments may nest before
// the monomorphizer gives up. Polymorphic recursion (`depth(Box(x), n-1)`) would
// otherwise instantiate Box<Box<Box<...>>> forever. No real type nests this deep.
const genericDepthLimit = 64

// typeNestingDepth returns the structural nesting depth of a type's arguments
// (Box<Box<Int>> is 2), bounded by `budget` so it never recurses past the limit.
func (g *Generics) typeNestingDepth(typeID TypeID, budget int) int {
	if budget <= 0 || typeID == InvalidTypeID {
		return 0
	}
	var args []TypeID
	switch kind := g.env.Type(typeID).Kind.(type) {
	case StructType:
		args = kind.TypeArgs
	case UnionType:
		args = kind.TypeArgs
	default:
		return 0
	}
	maxD := 0
	for _, arg := range args {
		if d := g.typeNestingDepth(arg, budget-1); d > maxD {
			maxD = d
		}
	}
	return 1 + maxD
}

func (g *Generics) materializeFun( //nolint:funlen
	funNodeID ast.NodeID,
	genericTypeID TypeID,
	callSiteNodeID ast.NodeID,
	typeArgIDs []TypeID,
) (mat FunMaterialization, typeID TypeID, mangledName string, status TypeStatus) {
	decl, _ := g.normalizeGenericDecl(funNodeID, genericTypeID, "")
	if decl.name == "" {
		return mat, InvalidTypeID, "", TypeFailed
	}
	mangledName = decl.cacheName(g, typeArgIDs)
	if cached, ok := g.loadFunWork(mangledName); ok {
		return mat, cached.TypeID, mangledName, TypeOK
	}
	name := decl.name
	if fun, ok := g.ast.Node(funNodeID).Kind.(ast.Fun); ok {
		name = fun.Name.Name
	}
	for _, argID := range typeArgIDs {
		if g.typeNestingDepth(argID, genericDepthLimit) >= genericDepthLimit {
			g.diag(g.ast.Node(callSiteNodeID).Span,
				"generic instantiation of %s nests deeper than %d levels; likely unbounded recursion",
				name, genericDepthLimit)
			g.recursionAborted = true
			return mat, InvalidTypeID, "", TypeFailed
		}
	}
	defer g.enterChildEnv()()
	inst, status := g.prepareGenericInstance(decl, typeArgIDs)
	if status.Failed() {
		return mat, InvalidTypeID, "", status
	}
	g.bindGenericArgs(inst.Spec, inst.TypeArgIDs)
	// Implicit owner params live in the owner's scope, but the function's body
	// resolves T via its own scope. Re-bind in funScope so type expressions
	// inside the function see the concrete type arg.
	funScope := g.scopeGraph.IntroducedScope(funNodeID)
	for i, param := range inst.Spec.Params {
		g.env.bindInScope(funScope, param.NodeID, param.Name, typeArgIDs[i])
	}
	node := g.ast.Node(funNodeID)
	noescapeReturn := false
	unsafe := false
	if fun, ok := node.Kind.(ast.Fun); ok {
		noescapeReturn = fun.NoescapeReturn
		unsafe = fun.Unsafe
	}
	funTyp, status := g.rewriteCallable(
		decl.typeParams, typeArgIDs, decl.paramNodeIDs, decl.returnNodeID, noescapeReturn)
	if status.Failed() {
		return mat, InvalidTypeID, "", status
	}
	funTyp, _, status = g.rewriteFunType(funTyp, inst.Spec.Bindings(typeArgIDs))
	if status.Failed() {
		return mat, InvalidTypeID, "", status
	}
	funTyp.Unsafe = unsafe
	funTypeID := g.env.newType(funTyp, node.ID, node.Span, TypeOK)
	g.env.setGenericOrigin(funTypeID, genericTypeID)
	if !decl.builtin {
		g.recordFunWork(mangledName, FunWork{NodeID: funNodeID, TypeID: funTypeID, Name: mangledName, Env: g.env})
		mat.NeedsBodyCheck = true
		mat.FunNodeID = funNodeID
		mat.FunType = funTyp
		mat.CallSiteNodeID = callSiteNodeID
		mat.SkipRegister = g.hasUnresolvedTypeParams(typeArgIDs)
		mat.Env = g.env
	}
	return mat, funTypeID, mangledName, TypeOK
}

func (g *Generics) materializeNamedTypeRef(
	originTypeID TypeID,
	typeArgNodeIDs []ast.NodeID,
	span base.Span,
) (TypeID, TypeStatus) {
	decl, ok := g.normalizeGenericDecl(g.env.DeclNode(originTypeID), originTypeID, "")
	if !ok {
		panic(base.Errorf("type %s is not generic-instantiable", originTypeID))
	}
	typeArgIDs, status := g.resolveTypeArgs(decl.typeParams, typeArgNodeIDs, decl.declNodeID, span)
	if status.Failed() {
		return InvalidTypeID, status
	}
	return g.materializeNamedType(originTypeID, typeArgIDs)
}

func (g *Generics) materializeNamedType(originTypeID TypeID, typeArgIDs []TypeID) (TypeID, TypeStatus) { //nolint:funlen
	decl, ok := g.normalizeGenericDecl(g.env.DeclNode(originTypeID), originTypeID, "")
	if !ok {
		panic(base.Errorf("type %s is not generic-instantiable", originTypeID))
	}
	inst, status := g.prepareGenericInstance(decl, typeArgIDs)
	if status.Failed() {
		return InvalidTypeID, status
	}
	if cached, ok := decl.lookupTypeWork(g, inst.Name); ok {
		return cached.TypeID, TypeOK
	}
	var (
		typeID   TypeID
		resolved TypeKind
	)
	declNode := g.ast.Node(decl.declNodeID)
	status = g.withGenericArgs(inst, func() TypeStatus {
		decl.storeTypeWork(g, inst.Name, TypeWork{NodeID: decl.declNodeID, TypeID: InvalidTypeID, Env: g.env})
		switch kind := declNode.Kind.(type) {
		case ast.Struct:
			placeholder := StructType{Name: inst.Name, Fields: nil, TypeArgs: typeArgIDs}
			typeID = g.env.newType(placeholder, decl.declNodeID, declNode.Span, TypeInProgress)
			decl.storeTypeWork(g, inst.Name, TypeWork{NodeID: decl.declNodeID, TypeID: typeID, Env: g.env})
			fields := make([]StructField, len(kind.Fields))
			for i, fieldNodeID := range kind.Fields {
				fieldTypeID, fieldStatus := g.query(fieldNodeID)
				if fieldStatus.Failed() {
					return TypeDepFailed
				}
				fieldNode := base.Cast[ast.StructField](g.ast.Node(fieldNodeID).Kind)
				fields[i] = StructField{
					Name: fieldNode.Name.Name,
					Type: fieldTypeID,
					Pub:  fieldNode.Pub,
				}
			}
			resolved = StructType{Name: inst.Name, Fields: fields, TypeArgs: typeArgIDs}
		case ast.Union:
			placeholder := UnionType{Name: inst.Name, Variants: nil, TypeArgs: typeArgIDs}
			typeID = g.env.newType(placeholder, decl.declNodeID, declNode.Span, TypeInProgress)
			decl.storeTypeWork(g, inst.Name, TypeWork{NodeID: decl.declNodeID, TypeID: typeID, Env: g.env})
			variants := make([]TypeID, len(kind.Variants))
			for i, variantNodeID := range kind.Variants {
				variantTypeID, variantStatus := g.query(variantNodeID)
				if variantStatus.Failed() {
					return TypeDepFailed
				}
				variants[i] = variantTypeID
			}
			resolved = UnionType{Name: inst.Name, Variants: variants, TypeArgs: typeArgIDs}
		case ast.Shape:
			_ = kind
			resolved = ShapeType{Name: decl.name, DeclName: decl.declName, Fields: nil, TypeArgs: typeArgIDs}
			typeID = g.env.newType(resolved, decl.declNodeID, declNode.Span, TypeOK)
			decl.storeTypeWork(g, inst.Name, TypeWork{NodeID: decl.declNodeID, TypeID: typeID, Env: g.env})
		default:
			panic(base.Errorf("type %s is not generic-instantiable", originTypeID))
		}
		return TypeOK
	})
	if status.Failed() {
		return InvalidTypeID, status
	}
	g.env.setGenericOrigin(typeID, originTypeID)
	if decl.kind != genericDeclShape {
		cached, _ := g.env.cachedTypeInfo(typeID)
		cached.Type.Kind = resolved
		cached.Status = TypeOK
	}
	return typeID, TypeOK
}

// materializeOwnerTypeParam creates a TypeParamType for an owner's type param
// without binding its name (the owner already did that in its own scope).
func (g *Generics) materializeOwnerTypeParam(paramNodeID ast.NodeID) TypeStatus {
	if _, ok := g.env.TypeParamForNode(paramNodeID); ok {
		return TypeOK
	}
	paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
	shapeID, status := g.resolveTypeParamConstraint(paramNode.Constraint)
	if status.Failed() {
		return status
	}
	var defaultID *TypeID
	if paramNode.Default != nil {
		defID, status := g.query(*paramNode.Default)
		if status.Failed() {
			return TypeDepFailed
		}
		defaultID = &defID
	}
	typeParamID := g.env.newType(
		TypeParamType{Shape: shapeID, Default: defaultID},
		paramNodeID,
		g.ast.Node(paramNodeID).Span,
		TypeOK,
	)
	g.env.setTypeParamForNode(paramNodeID, typeParamID)
	return TypeOK
}

func (g *Generics) hasUnresolvedTypeParams(typeArgIDs []TypeID) bool {
	for _, id := range typeArgIDs {
		if _, ok := g.env.Type(id).Kind.(TypeParamType); ok {
			return true
		}
	}
	return false
}

type GenericParam struct {
	NodeID     ast.NodeID
	TypeID     TypeID
	Name       string
	Constraint *TypeID
	Default    *TypeID
	Sync       bool
}

type GenericSpec struct {
	DeclNodeID   ast.NodeID
	OriginTypeID TypeID
	Params       []GenericParam
}

type genericDeclKind uint8

const (
	genericDeclStruct genericDeclKind = iota + 1
	genericDeclUnion
	genericDeclShape
	genericDeclFun
	genericDeclShapeMethod
)

type genericDecl struct {
	kind          genericDeclKind
	originTypeID  TypeID
	declNodeID    ast.NodeID
	typeParams    []ast.NodeID
	name          string
	declName      string
	memberNodeIDs []ast.NodeID
	paramNodeIDs  []ast.NodeID
	returnNodeID  ast.NodeID
	builtin       bool
}

type genericInstance struct {
	Decl       genericDecl
	Spec       *GenericSpec
	TypeArgIDs []TypeID
	Name       string
}

func (s *GenericSpec) MinArgs() int {
	for i, param := range s.Params {
		if param.Default != nil {
			return i
		}
	}
	return len(s.Params)
}

func (s *GenericSpec) Bindings(typeArgIDs []TypeID) map[TypeID]TypeID {
	bindings := make(map[TypeID]TypeID, len(typeArgIDs))
	for i, param := range s.Params {
		if i >= len(typeArgIDs) {
			break
		}
		bindings[param.TypeID] = typeArgIDs[i]
	}
	return bindings
}

func (d genericDecl) cacheName(g *Generics, typeArgIDs []TypeID) string {
	if d.kind == genericDeclShape && len(typeArgIDs) == 0 {
		return d.name
	}
	return g.mangledName(d.name, d.originTypeID, typeArgIDs)
}

func (d genericDecl) lookupTypeWork(g *Generics, name string) (TypeWork, bool) {
	switch d.kind {
	case genericDeclStruct:
		return g.loadStructWork(name)
	case genericDeclUnion:
		return g.loadUnionWork(name)
	case genericDeclShape:
		return g.loadShapeWork(name)
	case genericDeclFun, genericDeclShapeMethod:
		return TypeWork{}, false
	default:
		return TypeWork{}, false
	}
}

func (d genericDecl) storeTypeWork(g *Generics, name string, work TypeWork) {
	switch d.kind {
	case genericDeclStruct:
		g.recordStructWork(name, work)
	case genericDeclUnion:
		g.recordUnionWork(name, work)
	case genericDeclShape:
		g.recordShapeWork(name, work)
	case genericDeclFun, genericDeclShapeMethod:
		return
	}
}

func (g *Generics) buildGenericSpec(originTypeID TypeID, typeParamNodeIDs []ast.NodeID) (*GenericSpec, TypeStatus) {
	if originTypeID != InvalidTypeID {
		if spec, ok := g.env.GenericSpec(originTypeID); ok {
			return spec, TypeOK
		}
	}
	defer g.enterChildEnv()()
	if status := g.bindTypeParams(typeParamNodeIDs); status.Failed() {
		return nil, status
	}
	params := make([]GenericParam, len(typeParamNodeIDs))
	for i, nodeID := range typeParamNodeIDs {
		typeParamNode := base.Cast[ast.TypeParam](g.ast.Node(nodeID).Kind)
		typeParamTypeID, _ := g.env.TypeParamForNode(nodeID)
		tpt := base.Cast[TypeParamType](g.env.Type(typeParamTypeID).Kind)
		params[i] = GenericParam{
			NodeID:     nodeID,
			TypeID:     typeParamTypeID,
			Name:       typeParamNode.Name.Name,
			Constraint: tpt.Shape,
			Default:    tpt.Default,
			Sync:       typeParamNode.Sync == ast.SyncSync,
		}
	}
	spec := &GenericSpec{DeclNodeID: g.env.DeclNode(originTypeID), OriginTypeID: originTypeID, Params: params}
	if originTypeID != InvalidTypeID {
		g.env.setGenericSpec(originTypeID, spec)
	}
	return spec, TypeOK
}

func (g *Generics) resolveTypeArgs(
	typeParamNodeIDs []ast.NodeID, typeArgNodeIDs []ast.NodeID, scopeNodeID ast.NodeID, span base.Span,
) ([]TypeID, TypeStatus) {
	provided, status := g.queryTypeArgs(typeArgNodeIDs)
	if status.Failed() {
		return nil, status
	}
	spec, status := g.buildGenericSpec(InvalidTypeID, typeParamNodeIDs)
	if status.Failed() {
		return nil, status
	}
	return g.solveGenericArgs(spec, provided, scopeNodeID, span)
}

func (g *Generics) solveGenericArgs(
	spec *GenericSpec, provided []TypeID, scopeNodeID ast.NodeID, span base.Span,
) ([]TypeID, TypeStatus) {
	providedCount := len(provided)
	MinArgs := spec.MinArgs()
	total := len(spec.Params)
	if providedCount < MinArgs || providedCount > total {
		if MinArgs == total {
			g.diag(span, "type argument count mismatch: expected %d, got %d", total, providedCount)
		} else {
			g.diag(span, "type argument count mismatch: expected %d to %d, got %d", MinArgs, total, providedCount)
		}
		return nil, TypeFailed
	}

	resolved := make([]TypeID, total)
	bindings := map[TypeID]TypeID{}
	for i, param := range spec.Params {
		var argTypeID TypeID
		if i < providedCount {
			argTypeID = provided[i]
		} else {
			if param.Default == nil {
				panic(base.Errorf("missing default for %s", param.Name))
			}
			var status TypeStatus
			argTypeID, status = g.rewriteType(*param.Default, bindings)
			if status.Failed() {
				return nil, status
			}
		}
		if param.Constraint != nil {
			constraintTypeID, status := g.rewriteType(*param.Constraint, bindings)
			if status.Failed() {
				return nil, status
			}
			if argTPT, isTypeParam := g.env.Type(argTypeID).Kind.(TypeParamType); isTypeParam {
				if !g.typeParamSatisfiesShape(argTPT, argTypeID, constraintTypeID, span) {
					return nil, TypeFailed
				}
			} else if !g.satisfiesShape(argTypeID, constraintTypeID, scopeNodeID, span) {
				return nil, TypeFailed
			}
		}
		if param.Sync {
			if _, isTypeParam := g.env.Type(argTypeID).Kind.(TypeParamType); !isTypeParam &&
				!g.isSync(argTypeID) {
				g.diag(span, "type argument %s must be sync, got %s", param.Name, g.env.TypeDisplay(argTypeID))
				return nil, TypeFailed
			}
		}
		resolved[i] = argTypeID
		bindings[param.TypeID] = argTypeID
	}
	return resolved, TypeOK
}

func (g *Generics) bindGenericArgs(spec *GenericSpec, typeArgIDs []TypeID) {
	for i, param := range spec.Params {
		g.bind(param.NodeID, param.Name, false, typeArgIDs[i], g.ast.Node(param.NodeID).Span, -1)
	}
}

func (g *Generics) prepareGenericInstance(decl genericDecl, typeArgIDs []TypeID) (*genericInstance, TypeStatus) {
	spec, status := g.buildGenericSpec(decl.originTypeID, decl.typeParams)
	if status.Failed() {
		return nil, status
	}
	return &genericInstance{Decl: decl, Spec: spec, TypeArgIDs: typeArgIDs, Name: decl.cacheName(g, typeArgIDs)}, TypeOK
}

func (g *Generics) queryTypeArgs(typeArgNodeIDs []ast.NodeID) ([]TypeID, TypeStatus) {
	replacements := g.visibleTypeParamBindings()
	typeArgIDs := make([]TypeID, len(typeArgNodeIDs))
	for i, typeArgNodeID := range typeArgNodeIDs {
		typeArgID, status := g.query(typeArgNodeID)
		if status.Failed() {
			return nil, TypeDepFailed
		}
		if len(replacements) > 0 {
			typeArgID, status = g.rewriteType(typeArgID, replacements)
			if status.Failed() {
				return nil, status
			}
		}
		typeArgIDs[i] = typeArgID
	}
	return typeArgIDs, TypeOK
}

func (g *Generics) normalizeGenericDecl(nodeID ast.NodeID, originTypeID TypeID, name string) (genericDecl, bool) {
	node := g.ast.Node(nodeID)
	if decl, ok := g.normalizeGenericTypeDecl(originTypeID, nodeID, node.Kind); ok {
		return decl, true
	}
	return g.normalizeGenericCallableDecl(originTypeID, nodeID, name, node.Kind)
}

func (g *Generics) normalizeGenericTypeDecl(originTypeID TypeID, nodeID ast.NodeID, node any) (genericDecl, bool) {
	switch kind := node.(type) {
	case ast.Struct:
		return genericDecl{
			kind:          genericDeclStruct,
			originTypeID:  originTypeID,
			declNodeID:    nodeID,
			typeParams:    kind.TypeParams,
			name:          base.Cast[StructType](g.env.Type(originTypeID).Kind).Name,
			declName:      kind.Name.Name,
			memberNodeIDs: kind.Fields,
			paramNodeIDs:  nil,
			returnNodeID:  0,
			builtin:       false,
		}, true
	case ast.Union:
		return genericDecl{
			kind:          genericDeclUnion,
			originTypeID:  originTypeID,
			declNodeID:    nodeID,
			typeParams:    kind.TypeParams,
			name:          base.Cast[UnionType](g.env.Type(originTypeID).Kind).Name,
			declName:      kind.Name.Name,
			memberNodeIDs: kind.Variants,
			paramNodeIDs:  nil,
			returnNodeID:  0,
			builtin:       false,
		}, true
	case ast.Shape:
		shapeType := base.Cast[ShapeType](g.env.Type(originTypeID).Kind)
		return genericDecl{
			kind:          genericDeclShape,
			originTypeID:  originTypeID,
			declNodeID:    nodeID,
			typeParams:    kind.TypeParams,
			name:          shapeType.Name,
			declName:      shapeType.DeclName,
			memberNodeIDs: nil,
			paramNodeIDs:  nil,
			returnNodeID:  0,
			builtin:       false,
		}, true
	default:
		return genericDecl{}, false
	}
}

func (g *Generics) normalizeGenericCallableDecl(
	originTypeID TypeID,
	nodeID ast.NodeID,
	name string,
	node any,
) (genericDecl, bool) {
	switch kind := node.(type) {
	case ast.Fun:
		if name == "" {
			name, _ = g.env.NamedFunRef(nodeID)
		}
		return genericDecl{
			kind:          genericDeclFun,
			originTypeID:  originTypeID,
			declNodeID:    nodeID,
			typeParams:    g.funTypeParams(nodeID),
			name:          name,
			declName:      "",
			memberNodeIDs: nil,
			paramNodeIDs:  kind.Params,
			returnNodeID:  kind.ReturnType,
			builtin:       kind.Builtin,
		}, true
	case ast.FunDecl:
		ownerTypeID, shapeNode, ok := g.shapeOwner(originTypeID)
		if !ok {
			return genericDecl{}, false
		}
		return genericDecl{
			kind:          genericDeclShapeMethod,
			originTypeID:  ownerTypeID,
			declNodeID:    nodeID,
			typeParams:    shapeNode.TypeParams,
			name:          name,
			declName:      "",
			memberNodeIDs: nil,
			paramNodeIDs:  kind.Params,
			returnNodeID:  kind.ReturnType,
			builtin:       false,
		}, true
	default:
		return genericDecl{}, false
	}
}

func (g *Generics) withGenericArgs(
	inst *genericInstance,
	fn func() TypeStatus,
) TypeStatus {
	defer g.enterChildEnv()()
	g.bindGenericArgs(inst.Spec, inst.TypeArgIDs)
	return fn()
}

func (g *Generics) rewriteType(typeID TypeID, bindings map[TypeID]TypeID) (TypeID, TypeStatus) {
	if replacement, ok := bindings[typeID]; ok {
		return replacement, TypeOK
	}
	typ := g.env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case RefType:
		inner, status := g.rewriteType(kind.Type, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		if inner == kind.Type {
			return typeID, TypeOK
		}
		return g.env.buildRefType(0, inner, kind.Mut, base.Span{}), TypeOK
	case ArrayType:
		elem, status := g.rewriteType(kind.Elem, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		if elem == kind.Elem {
			return typeID, TypeOK
		}
		return g.env.buildArrayType(elem, kind.Len, 0, base.Span{}), TypeOK
	case SliceType:
		elem, status := g.rewriteType(kind.Elem, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		if elem == kind.Elem {
			return typeID, TypeOK
		}
		return g.env.buildSliceType(elem, kind.Mut, 0, base.Span{}), TypeOK
	case FunType:
		rewritten, changed, status := g.rewriteFunType(kind, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		if !changed {
			return typeID, TypeOK
		}
		return g.env.buildFunType(rewritten, 0, base.Span{}), TypeOK
	case StructType:
		return g.rewriteNamedType(typeID, kind.TypeArgs, bindings)
	case UnionType:
		return g.rewriteNamedType(typeID, kind.TypeArgs, bindings)
	case ShapeType:
		return g.rewriteNamedType(typeID, kind.TypeArgs, bindings)
	default:
		return typeID, TypeOK
	}
}

func (g *Generics) rewriteFunType(funType FunType, bindings map[TypeID]TypeID) (FunType, bool, TypeStatus) {
	result := FunType{
		Params:         make([]TypeID, len(funType.Params)),
		Return:         funType.Return,
		Macro:          funType.Macro,
		Sync:           funType.Sync,
		Unsafe:         funType.Unsafe,
		NoescapeParams: funType.NoescapeParams,
		NoescapeReturn: funType.NoescapeReturn,
	}
	changed := false
	for i, paramTypeID := range funType.Params {
		rewritten, status := g.rewriteType(paramTypeID, bindings)
		if status.Failed() {
			return FunType{}, false, status
		}
		result.Params[i] = rewritten
		changed = changed || rewritten != paramTypeID
	}
	rewrittenReturn, status := g.rewriteType(funType.Return, bindings)
	if status.Failed() {
		return FunType{}, false, status
	}
	result.Return = rewrittenReturn
	changed = changed || rewrittenReturn != funType.Return
	return result, changed, TypeOK
}

func (g *Generics) rewriteCallable(
	typeParamNodeIDs []ast.NodeID,
	typeArgIDs []TypeID,
	paramNodeIDs []ast.NodeID,
	returnTypeNodeID ast.NodeID,
	noescapeReturn bool,
) (FunType, TypeStatus) {
	bindings := map[TypeID]TypeID{}
	if len(typeParamNodeIDs) > 0 {
		spec, status := g.buildGenericSpec(InvalidTypeID, typeParamNodeIDs)
		if status.Failed() {
			return FunType{}, status
		}
		maps.Copy(bindings, spec.Bindings(typeArgIDs))
	}
	paramTypeIDs := make([]TypeID, len(paramNodeIDs))
	noescapeParams := make([]bool, len(paramNodeIDs))
	for i, paramNodeID := range paramNodeIDs {
		paramTypeID, status := g.query(paramNodeID)
		if status.Failed() {
			return FunType{}, TypeDepFailed
		}
		if len(bindings) > 0 {
			paramTypeID, status = g.rewriteType(paramTypeID, bindings)
			if status.Failed() {
				return FunType{}, status
			}
			g.env.setNodeType(paramNodeID, &cachedType{Type: g.env.Type(paramTypeID), Status: TypeOK})
		}
		paramTypeIDs[i] = paramTypeID
		if p, ok := g.ast.Node(paramNodeID).Kind.(ast.FunParam); ok {
			noescapeParams[i] = p.Noescape
		}
	}
	retTypeID, status := g.query(returnTypeNodeID)
	if status.Failed() {
		return FunType{}, TypeDepFailed
	}
	if len(bindings) > 0 {
		retTypeID, status = g.rewriteType(retTypeID, bindings)
		if status.Failed() {
			return FunType{}, status
		}
	}
	funType := FunType{
		Params:         paramTypeIDs,
		Return:         retTypeID,
		Macro:          false,
		Sync:           false,
		Unsafe:         false,
		NoescapeParams: noescapeParams,
		NoescapeReturn: noescapeReturn,
	}
	return funType, TypeOK
}

func (g *Generics) rewriteNamedType(typeID TypeID, typeArgs []TypeID, bindings map[TypeID]TypeID) (TypeID, TypeStatus) {
	// During shape conformance the origin may be remapped (self-type to
	// concrete receiver) so the shape's method signature for `Iter<T>`
	// rewrites to the receiver rather than `Iter<arg>`.
	originTypeID := typeID
	if origin, ok := g.env.GenericOrigin(typeID); ok {
		originTypeID = origin
	}
	if replacement, ok := bindings[originTypeID]; ok && originTypeID != typeID {
		return replacement, TypeOK
	}
	if len(typeArgs) == 0 {
		return typeID, TypeOK
	}
	rewrittenArgs := make([]TypeID, len(typeArgs))
	changed := false
	for i, argTypeID := range typeArgs {
		rewritten, status := g.rewriteType(argTypeID, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		rewrittenArgs[i] = rewritten
		changed = changed || rewritten != argTypeID
	}
	if !changed {
		return typeID, TypeOK
	}
	return g.materializeNamedType(originTypeID, rewrittenArgs)
}

func (g *Generics) inferFunCall(call ast.Call, span base.Span) (TypeID, TypeStatus, bool) { //nolint:funlen
	binding, funTypeParams, ok := g.inferFunCallBinding(call)
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	funParams, _, _ := g.funParams(binding.Decl)
	genericFunType := base.Cast[FunType](g.env.Type(binding.TypeID).Kind)
	allArgNodeIDs := call.Args
	fieldAccess, isFieldAccess := g.ast.Node(call.Callee).Kind.(ast.FieldAccess)
	var methodReceiver ast.NodeID
	if isFieldAccess {
		targetTypeID, status := g.query(fieldAccess.Target)
		if status.Failed() {
			return InvalidTypeID, TypeFailed, false
		}
		_, isModule := g.env.Type(targetTypeID).Kind.(ModuleType)
		if !isModule && !g.isTypeReference(fieldAccess.Target) {
			methodReceiver = fieldAccess.Target
		}
	}
	// Named arguments are reordered into parameter order with defaults filled in,
	// so inference lines arguments up with the right parameter types. The engine
	// reports any naming errors, so a bad match silently falls back here.
	if hasNamedArgs(call.ArgNames) {
		userParams := funParams
		if methodReceiver != 0 && len(funParams) > 0 {
			userParams = funParams[1:]
		}
		if order, _, ok := orderCallArgs(g.ast, userParams, call.Args, call.ArgNames, span, noDiag); ok {
			allArgNodeIDs = order
		}
	}
	if methodReceiver != 0 {
		allArgNodeIDs = append([]ast.NodeID{methodReceiver}, allArgNodeIDs...)
	}
	if len(allArgNodeIDs) < len(genericFunType.Params) {
		for i := len(allArgNodeIDs); i < len(funParams); i++ {
			param := base.Cast[ast.FunParam](g.ast.Node(funParams[i]).Kind)
			if param.Default != nil {
				allArgNodeIDs = append(allArgNodeIDs, *param.Default)
			}
		}
	}
	args, status := g.queryArgsForInference(allArgNodeIDs, genericFunType.Params)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed, true
	}
	if slices.ContainsFunc(args, func(r ArgResult) bool { return r.Deferred }) {
		args, status = g.resolveDeferredFunLitArgs(
			funTypeParams, genericFunType.Params, allArgNodeIDs, args, call.Callee, span,
		)
		if status.Failed() {
			return InvalidTypeID, status, true
		}
	}
	seedBindings := g.ownerImplicitSeed(binding, fieldAccess, isFieldAccess, funTypeParams)
	inferred, inferOK, status := g.inferTypeArgsWithSeed(
		funTypeParams, genericFunType.Params, args, seedBindings, call.Callee, span,
	)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	if !inferOK {
		return InvalidTypeID, TypeFailed, false
	}
	typeID, status := g.solveAndMaterializeFun(binding, call.Callee, span, inferred)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	g.env.SetPathBinding(call.Callee, binding)
	g.env.setNodeType(call.Callee, &cachedType{Type: g.env.Type(typeID), Status: TypeOK})
	return typeID, TypeOK, true
}

func (g *Generics) inferFunCallBinding(call ast.Call) (*Binding, []ast.NodeID, bool) {
	switch kind := g.ast.Node(call.Callee).Kind.(type) {
	case ast.Ident:
		if len(kind.TypeArgs) > 0 {
			return nil, nil, false
		}
	case ast.FieldAccess:
		if len(kind.TypeArgs) > 0 {
			return nil, nil, false
		}
	}
	binding, ok := g.resolveCallBinding(call)
	if !ok {
		return nil, nil, false
	}
	if _, isFun := g.ast.Node(binding.Decl).Kind.(ast.Fun); !isFun {
		return nil, nil, false
	}
	tps := g.funTypeParams(binding.Decl)
	if len(tps) == 0 {
		return nil, nil, false
	}
	return binding, tps, true
}

func (g *Generics) inferTypeConstruction(
	lit ast.TypeConstruction, span base.Span, typeHint *TypeID,
) (TypeID, TypeStatus, bool) {
	ident, ok := g.ast.Node(lit.Target).Kind.(ast.Ident)
	if !ok || len(ident.TypeArgs) > 0 {
		return InvalidTypeID, TypeFailed, false
	}
	binding, ok := g.lookup(lit.Target, ident.Name, -1)
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	if typeHint != nil {
		if origin, ok := g.env.GenericOrigin(
			*typeHint,
		); ok && origin == binding.TypeID &&
			!g.env.containsTypeParam(*typeHint) {
			g.updateCachedType(g.ast.Node(lit.Target), *typeHint, TypeOK)
			return *typeHint, TypeOK, true
		}
	}
	switch kind := g.env.Type(binding.TypeID).Kind.(type) {
	case StructType:
		return g.inferNamedConstruction(binding, kind, lit, span)
	case UnionType:
		return g.inferNamedConstruction(binding, kind, lit, span)
	default:
		return InvalidTypeID, TypeFailed, false
	}
}

func (g *Generics) inferNamedConstruction(
	binding *Binding, targetKind TypeKind, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus, bool) {
	var inferred []TypeID
	var inferOK bool
	var status TypeStatus
	switch kind := g.ast.Node(binding.Decl).Kind.(type) {
	case ast.Struct:
		if len(kind.TypeParams) == 0 {
			return InvalidTypeID, TypeFailed, false
		}
		inferred, inferOK, status = g.inferStructConstruction(targetKind, lit, span, kind.TypeParams)
	case ast.Union:
		if len(kind.TypeParams) == 0 {
			return InvalidTypeID, TypeFailed, false
		}
		inferred, inferOK, status = g.inferUnionConstruction(binding, targetKind, lit, span)
	default:
		return InvalidTypeID, TypeFailed, false
	}
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	if !inferOK {
		return InvalidTypeID, TypeFailed, false
	}
	typeID, status := g.materializeNamedType(binding.TypeID, inferred)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	g.updateCachedType(g.ast.Node(lit.Target), typeID, TypeOK)
	return typeID, TypeOK, true
}

func (g *Generics) inferStructConstruction(
	targetKind TypeKind,
	lit ast.TypeConstruction,
	span base.Span,
	typeParamNodeIDs []ast.NodeID,
) (inferred []TypeID, ok bool, status TypeStatus) {
	structType := base.Cast[StructType](targetKind)
	if len(lit.Args) != len(structType.Fields) {
		g.diag(span, "argument count mismatch: expected %d, got %d", len(structType.Fields), len(lit.Args))
		return nil, true, TypeFailed
	}
	fieldTypeIDs := make([]TypeID, len(structType.Fields))
	fieldNames := make([]string, len(structType.Fields))
	for i, field := range structType.Fields {
		fieldTypeIDs[i] = field.Type
		fieldNames[i] = field.Name
	}
	// Reorder named fields into declaration order so inference pairs each value
	// with the right field type. The engine reports naming errors.
	orderedArgs := lit.Args
	if hasNamedArgs(lit.ArgNames) {
		if slots, ok := matchArgs(g.ast, fieldNames, "field", lit.Args, lit.ArgNames, span, noDiag); ok &&
			!slices.Contains(slots, ast.NodeID(0)) {
			orderedArgs = slots
		}
	}
	args, queryStatus := g.queryArgsForInference(orderedArgs, fieldTypeIDs)
	if queryStatus.Failed() {
		return nil, true, TypeDepFailed
	}
	return g.inferTypeArgsWithSeed(typeParamNodeIDs, fieldTypeIDs, args, nil, lit.Target, span)
}

func (g *Generics) inferUnionConstruction(
	binding *Binding,
	targetKind TypeKind,
	lit ast.TypeConstruction,
	span base.Span,
) ([]TypeID, bool, TypeStatus) {
	if len(lit.Args) != 1 {
		g.diag(span, "union constructor takes exactly 1 argument, got %d", len(lit.Args))
		return nil, true, TypeFailed
	}
	argTypeID, status := g.query(lit.Args[0])
	if status.Failed() {
		return nil, true, TypeDepFailed
	}
	unionType := base.Cast[UnionType](targetKind)
	bindings := map[TypeID]TypeID{}
	for _, variantTypeID := range unionType.Variants {
		g.inferTypeBindings(variantTypeID, argTypeID, bindings)
	}
	decl, _ := g.normalizeGenericDecl(g.env.DeclNode(binding.TypeID), binding.TypeID, "")
	spec, status := g.buildGenericSpec(decl.originTypeID, decl.typeParams)
	if status.Failed() {
		return nil, true, status
	}
	inferred := make([]TypeID, 0, len(spec.Params))
	for _, param := range spec.Params {
		concreteID, bound := bindings[param.TypeID]
		if !bound {
			break
		}
		inferred = append(inferred, concreteID)
	}
	if len(inferred) < spec.MinArgs() {
		return nil, false, TypeOK
	}
	solved, solveStatus := g.solveGenericArgs(spec, inferred, lit.Target, span)
	return solved, true, solveStatus
}

// ArgResult carries the per-argument result of queryArgsForInference. A
// deferred entry signals that the corresponding argument is a fun-lit whose
// types depend on inference of the other arguments.
type ArgResult struct {
	TypeID   TypeID
	Deferred bool
}

func (g *Generics) inferTypeArgsWithSeed(
	typeParamNodeIDs []ast.NodeID,
	genericTypeIDs []TypeID,
	concreteArgs []ArgResult,
	seedBindings map[TypeID]TypeID,
	scopeNodeID ast.NodeID,
	span base.Span,
) (inferred []TypeID, ok bool, status TypeStatus) {
	spec, status := g.buildGenericSpec(InvalidTypeID, typeParamNodeIDs)
	if status.Failed() {
		return nil, true, status
	}
	bindings := map[TypeID]TypeID{}
	maps.Copy(bindings, seedBindings)
	for i, genericTypeID := range genericTypeIDs {
		if i >= len(concreteArgs) {
			break
		}
		if concreteArgs[i].Deferred {
			continue
		}
		if !g.inferTypeBindings(genericTypeID, concreteArgs[i].TypeID, bindings) {
			break
		}
	}
	g.inferTypeArgsFromConstraints(spec, bindings, scopeNodeID, span)
	result := make([]TypeID, 0, len(spec.Params))
	for _, param := range spec.Params {
		concreteID, bound := bindings[param.TypeID]
		if !bound {
			break
		}
		result = append(result, concreteID)
	}
	if len(result) < spec.MinArgs() {
		return nil, false, TypeOK
	}
	solved, solveStatus := g.solveGenericArgs(spec, result, scopeNodeID, span)
	return solved, true, solveStatus
}

func (g *Generics) inferTypeArgsFromConstraints(
	spec *GenericSpec, bindings map[TypeID]TypeID, scopeNodeID ast.NodeID, span base.Span,
) {
	for _, param := range spec.Params {
		concreteTypeID, ok := bindings[param.TypeID]
		if !ok || param.Constraint == nil {
			continue
		}
		constraintTypeID, status := g.rewriteType(*param.Constraint, bindings)
		if status.Failed() {
			continue
		}
		shapeType := base.Cast[ShapeType](g.env.Type(constraintTypeID).Kind)
		if len(shapeType.TypeArgs) == 0 {
			continue
		}
		g.inferConstraintBindings(constraintTypeID, concreteTypeID, bindings, scopeNodeID, span)
	}
}

func (g *Generics) inferTypeBindings(
	patternTypeID, concreteTypeID TypeID,
	bindings map[TypeID]TypeID,
) bool {
	patternType := g.env.Type(patternTypeID)
	switch patternKind := patternType.Kind.(type) {
	case TypeParamType:
		if bound, ok := bindings[patternTypeID]; ok {
			if bound == concreteTypeID {
				return true
			}
			// Args of one enum family unify to their shared open root, so
			// eq(IOErr, AppErr) infers T = AppErr regardless of arg order.
			if g.env.sameEnumFamily(bound, concreteTypeID) {
				bindings[patternTypeID] = g.env.enumFamilyRoot(bound)
				return true
			}
			return false
		}
		bindings[patternTypeID] = concreteTypeID
		return true
	case RefType:
		concreteKind, ok := g.env.Type(concreteTypeID).Kind.(RefType)
		return ok && patternKind.Mut == concreteKind.Mut &&
			g.inferTypeBindings(patternKind.Type, concreteKind.Type, bindings)
	case ArrayType:
		concreteKind, ok := g.env.Type(concreteTypeID).Kind.(ArrayType)
		return ok && patternKind.Len == concreteKind.Len &&
			g.inferTypeBindings(patternKind.Elem, concreteKind.Elem, bindings)
	case SliceType:
		concreteKind, ok := g.env.Type(concreteTypeID).Kind.(SliceType)
		return ok && patternKind.Mut == concreteKind.Mut &&
			g.inferTypeBindings(patternKind.Elem, concreteKind.Elem, bindings)
	case FunType:
		concreteKind, ok := g.env.Type(concreteTypeID).Kind.(FunType)
		if !ok || len(patternKind.Params) != len(concreteKind.Params) || patternKind.Macro != concreteKind.Macro {
			return false
		}
		for i, patternParam := range patternKind.Params {
			if !g.inferTypeBindings(patternParam, concreteKind.Params[i], bindings) {
				return false
			}
		}
		return g.inferTypeBindings(patternKind.Return, concreteKind.Return, bindings)
	case StructType:
		concreteKind, ok := g.env.Type(concreteTypeID).Kind.(StructType)
		return ok && sameOrigin(g.env, patternTypeID, concreteTypeID) &&
			g.inferTypeArgSlice(patternKind.TypeArgs, concreteKind.TypeArgs, bindings)
	case UnionType:
		concreteKind, ok := g.env.Type(concreteTypeID).Kind.(UnionType)
		return ok && sameOrigin(g.env, patternTypeID, concreteTypeID) &&
			g.inferTypeArgSlice(patternKind.TypeArgs, concreteKind.TypeArgs, bindings)
	case ShapeType:
		concreteKind, ok := g.env.Type(concreteTypeID).Kind.(ShapeType)
		return ok && sameOrigin(g.env, patternTypeID, concreteTypeID) &&
			g.inferTypeArgSlice(patternKind.TypeArgs, concreteKind.TypeArgs, bindings)
	default:
		return patternTypeID == concreteTypeID
	}
}

func (g *Generics) inferTypeArgSlice(pattern, concrete []TypeID, bindings map[TypeID]TypeID) bool {
	if len(pattern) != len(concrete) {
		return false
	}
	for i, patternTypeID := range pattern {
		if !g.inferTypeBindings(patternTypeID, concrete[i], bindings) {
			return false
		}
	}
	return true
}

func (g *Generics) inferConstraintBindings(
	shapeTypeID TypeID,
	concreteTypeID TypeID,
	bindings map[TypeID]TypeID,
	scopeNodeID ast.NodeID,
	span base.Span,
) {
	shapeNodeID := g.env.DeclNode(typeIDOrigin(g.env, shapeTypeID))
	shapeNode := base.Cast[ast.Shape](g.ast.Node(shapeNodeID).Kind)
	for _, funDeclNodeID := range shapeNode.Funs {
		funDecl := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		shapeFunBinding, ok := g.lookupShapeMethodBinding(shapeTypeID, methodName, scopeNodeID)
		if !ok {
			continue
		}
		shapeFunType, status := g.methodSignature(
			shapeFunBinding,
			g.shapeMethodContext(shapeTypeID, concreteTypeID),
			scopeNodeID,
			span,
		)
		if status.Failed() {
			continue
		}
		binding, ok := g.lookupMethodBinding(scopeNodeID, concreteTypeID, methodName)
		if !ok {
			continue
		}
		concreteFunType, status := g.methodSignature(
			binding,
			g.boundMethodContext(binding, concreteTypeID),
			scopeNodeID,
			span,
		)
		if status.Failed() {
			continue
		}
		// Skip the receiver param (index 0): its self-type matches itself
		// trivially and would bind the shape's TypeParam to itself, blocking
		// the real inference from the return and non-self params.
		for i, paramTypeID := range shapeFunType.Params {
			if i == 0 {
				continue
			}
			if i >= len(concreteFunType.Params) {
				break
			}
			g.inferTypeBindings(paramTypeID, concreteFunType.Params[i], bindings)
		}
		g.inferTypeBindings(shapeFunType.Return, concreteFunType.Return, bindings)
	}
}

// inferFunTypeBindings unifies a pattern function type against a concrete
// one. Existing entries in `bindings` are ground-truth: positions that
// resolve to the same TypeParam must agree across the signature.
func (g *Generics) inferFunTypeBindings(pattern, concrete FunType, bindings map[TypeID]TypeID) {
	if len(pattern.Params) != len(concrete.Params) {
		return
	}
	for i, patternID := range pattern.Params {
		g.inferTypeBindings(patternID, concrete.Params[i], bindings)
	}
	g.inferTypeBindings(pattern.Return, concrete.Return, bindings)
}

func (g *Generics) queryArgsForInference(argNodeIDs []ast.NodeID, patternTypeIDs []TypeID) ([]ArgResult, TypeStatus) {
	args := make([]ArgResult, len(argNodeIDs))
	for i, argNodeID := range argNodeIDs {
		// Defer fun-lit args with inferred types when the expected param
		// type contains unresolved type params, otherwise we would cache the
		// fun-lit with the unresolved type params. resolveDeferredFunLitArgs
		// retypes them after partial inference from the other arguments.
		if i < len(patternTypeIDs) && g.env.containsTypeParam(patternTypeIDs[i]) &&
			g.isFunLitWithInferredTypes(argNodeID) {
			args[i] = ArgResult{Deferred: true}
			continue
		}
		var argTypeID TypeID
		var status TypeStatus
		if i < len(patternTypeIDs) {
			if _, isTypeParam := g.env.Type(patternTypeIDs[i]).Kind.(TypeParamType); !isTypeParam {
				argTypeID, status = g.queryWithHint(argNodeID, &patternTypeIDs[i])
			} else {
				argTypeID, status = g.query(argNodeID)
			}
		} else {
			argTypeID, status = g.query(argNodeID)
		}
		if status.Failed() {
			return nil, TypeDepFailed
		}
		args[i] = ArgResult{TypeID: argTypeID}
	}
	return args, TypeOK
}

// resolveDeferredFunLitArgs type-checks deferred fun-lit arguments using
// hints derived from partial inference over the already-resolved arguments.
func (g *Generics) resolveDeferredFunLitArgs(
	typeParamNodeIDs []ast.NodeID,
	genericTypeIDs []TypeID,
	argNodeIDs []ast.NodeID,
	args []ArgResult,
	scopeNodeID ast.NodeID,
	span base.Span,
) ([]ArgResult, TypeStatus) {
	spec, status := g.buildGenericSpec(InvalidTypeID, typeParamNodeIDs)
	if status.Failed() {
		return nil, status
	}
	bindings := map[TypeID]TypeID{}
	for i, genericTypeID := range genericTypeIDs {
		if i >= len(args) {
			break
		}
		if args[i].Deferred {
			continue
		}
		g.inferTypeBindings(genericTypeID, args[i].TypeID, bindings)
	}
	g.inferTypeArgsFromConstraints(spec, bindings, scopeNodeID, span)
	for i, arg := range args {
		if !arg.Deferred {
			continue
		}
		if i >= len(genericTypeIDs) {
			continue
		}
		resolvedHint, status := g.rewriteType(genericTypeIDs[i], bindings)
		if status.Failed() {
			return nil, status
		}
		resolvedTypeID, status := g.queryWithHint(argNodeIDs[i], &resolvedHint)
		if status.Failed() {
			return nil, TypeDepFailed
		}
		args[i] = ArgResult{TypeID: resolvedTypeID}
	}
	return args, TypeOK
}

// isFunLitWithInferredTypes reports whether nodeID is a function-literal
// block with any omitted parameter or return type.
func (g *Generics) isFunLitWithInferredTypes(nodeID ast.NodeID) bool {
	block, ok := g.ast.Node(nodeID).Kind.(ast.Block)
	if !ok || len(block.Exprs) != 2 {
		return false
	}
	fun, ok := g.ast.Node(block.Exprs[0]).Kind.(ast.Fun)
	if !ok {
		return false
	}
	return g.funLitNeedsInference(fun)
}

// ownerImplicitSeed builds initial type-arg bindings from the receiver of a
// method call, so the receiver instance (e.g. Box<Score>) provides concrete
// values for the owner's implicit type parameters.
func (g *Generics) ownerImplicitSeed(
	binding *Binding,
	fieldAccess ast.FieldAccess,
	isFieldAccess bool,
	typeParamNodeIDs []ast.NodeID,
) map[TypeID]TypeID {
	if !isFieldAccess {
		return nil
	}
	targetTypeID, status := g.query(fieldAccess.Target)
	if status.Failed() {
		return nil
	}
	receiverType := g.env.Type(targetTypeID)
	if refTyp, ok := receiverType.Kind.(RefType); ok {
		receiverType = g.env.Type(refTyp.Type)
	}
	receiverArgs, ok := ImplicitTypeArgs(receiverType.Kind)
	if !ok || len(receiverArgs) == 0 {
		return nil
	}
	structName, _, hasDot := strings.Cut(binding.Name, ".")
	if !hasDot {
		return nil
	}
	ownerBinding, ok := g.lookup(binding.Decl, structName, -1)
	if !ok {
		return nil
	}
	var ownerParams []ast.NodeID
	switch kind := g.ast.Node(ownerBinding.Decl).Kind.(type) {
	case ast.Struct:
		ownerParams = kind.TypeParams
	case ast.Union:
		ownerParams = kind.TypeParams
	case ast.Shape:
		ownerParams = kind.TypeParams
	default:
		return nil
	}
	if len(ownerParams) == 0 {
		return nil
	}
	seed := make(map[TypeID]TypeID, len(ownerParams))
	for i, paramNodeID := range ownerParams {
		if i >= len(receiverArgs) {
			break
		}
		typeParamID, ok := g.env.TypeParamForNode(paramNodeID)
		if !ok {
			continue
		}
		if !slices.Contains(typeParamNodeIDs, paramNodeID) {
			continue
		}
		seed[typeParamID] = receiverArgs[i]
	}
	if len(seed) == 0 {
		return nil
	}
	return seed
}

// satisfiesEnumConstraint implements the built-in `Enum` constraint: the
// argument must be an enum, and its `Item` slot is bound to the backing int.
func (g *Generics) satisfiesEnumConstraint(concreteTypeID TypeID, span base.Span) bool {
	typ := g.env.Type(concreteTypeID)
	if ref, ok := typ.Kind.(RefType); ok {
		concreteTypeID = ref.Type
		typ = g.env.Type(concreteTypeID)
	}
	enum, ok := typ.Kind.(EnumType)
	if !ok {
		if _, isParam := typ.Kind.(TypeParamType); isParam {
			return true
		}
		g.diag(span, "type %s does not satisfy Enum: not an enum type", g.env.TypeDisplay(concreteTypeID))
		return false
	}
	entry, ok := g.concreteAssociated[concreteTypeID]
	if !ok {
		entry = map[string]TypeID{}
		g.concreteAssociated[concreteTypeID] = entry
	}
	entry["Item"] = enum.Backing
	return true
}

func (g *Generics) satisfiesShape( //nolint:funlen
	concreteTypeID TypeID,
	shapeTypeID TypeID,
	scopeNodeID ast.NodeID,
	span base.Span,
) bool {
	shapeType := base.Cast[ShapeType](g.env.Type(shapeTypeID).Kind)
	if ast.IsPreludeNode(g.env.DeclNode(typeIDOrigin(g.env, shapeTypeID))) && shapeType.DeclName == "Enum" {
		return g.satisfiesEnumConstraint(concreteTypeID, span)
	}
	concreteTyp := g.env.Type(concreteTypeID)
	g.debug.Print(0, "satisfiesShape concrete=%s shape=%s",
		g.env.TypeDisplay(concreteTypeID), g.env.TypeDisplay(shapeTypeID))
	// Methods live on the underlying type, not its ref form.
	lookupTypeID := concreteTypeID
	lookupTyp := concreteTyp
	if refTyp, ok := concreteTyp.Kind.(RefType); ok {
		lookupTypeID = refTyp.Type
		lookupTyp = g.env.Type(lookupTypeID)
	}
	if tpt, ok := lookupTyp.Kind.(TypeParamType); ok && tpt.Shape != nil && *tpt.Shape == shapeTypeID {
		return true
	}
	concreteDisplay := g.env.TypeDisplay(concreteTypeID)
	if _, ok := g.env.methodFQN(lookupTyp, ""); !ok {
		g.diag(span, "type %s cannot satisfy shape %s", concreteDisplay, shapeType.DeclName)
		return false
	}
	if len(shapeType.Fields) > 0 {
		structType, isStruct := lookupTyp.Kind.(StructType)
		if !isStruct {
			g.diag(span, "type %s does not satisfy shape %s: not a struct", concreteDisplay, shapeType.DeclName)
			return false
		}
		for _, reqField := range shapeType.Fields {
			matched := false
			for _, field := range structType.Fields {
				if field.Name != reqField.Name {
					continue
				}
				matched = true
				if field.Type != reqField.Type {
					g.diag(span,
						"type %s does not satisfy shape %s: field %s has type %s, expected %s",
						concreteDisplay,
						shapeType.DeclName,
						field.Name,
						g.env.TypeDisplay(field.Type),
						g.env.TypeDisplay(reqField.Type),
					)
					return false
				}
				if reqField.Pub && !field.Pub {
					g.diag(span, "type %s does not satisfy shape %s: field %s must be public",
						concreteDisplay, shapeType.DeclName, field.Name)
					return false
				}
				if !reqField.Pub && field.Pub {
					g.diag(span, "type %s does not satisfy shape %s: field %s must not be public",
						concreteDisplay, shapeType.DeclName, field.Name)
					return false
				}
				break
			}
			if !matched {
				g.diag(span, "type %s does not satisfy shape %s: missing field %s",
					concreteDisplay, shapeType.DeclName, reqField.Name)
				return false
			}
		}
	}
	shapeNodeID := g.env.DeclNode(typeIDOrigin(g.env, shapeTypeID))
	shapeNode := base.Cast[ast.Shape](g.ast.Node(shapeNodeID).Kind)
	// associatedBindings carries inferred associated-type bindings across
	// the shape's methods, so a value deduced from one method's return
	// type can be validated against the next.
	associatedBindings := map[TypeID]TypeID{}
	for _, funDeclNodeID := range shapeNode.Funs {
		funDecl := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		binding, ok := g.lookupMethodBinding(scopeNodeID, lookupTypeID, methodName)
		if !ok {
			g.diag(span, "type %s does not satisfy shape %s: missing method %s",
				concreteDisplay, shapeType.DeclName, methodName)
			return false
		}
		shapeFunBinding, ok := g.lookupShapeMethodBinding(shapeTypeID, methodName, scopeNodeID)
		if !ok {
			g.diag(span, "shape %s method %s not bound", shapeType.DeclName, methodName)
			return false
		}
		expectedFunType, status := g.methodSignature(
			shapeFunBinding,
			g.shapeMethodContext(shapeTypeID, lookupTypeID),
			scopeNodeID,
			span,
		)
		if status.Failed() {
			return false
		}
		concreteFunType, status := g.methodSignature(
			binding,
			g.boundMethodContext(binding, concreteTypeID),
			scopeNodeID,
			span,
		)
		if status.Failed() {
			return false
		}
		g.debug.Print(0, "satisfiesShape method=%s expected=%s concrete=%s",
			methodName, g.funTypeDisplay(expectedFunType), g.funTypeDisplay(concreteFunType))
		concretePub := g.declIsPub(binding.Decl)
		if funDecl.Pub && !concretePub {
			g.diag(span, "type %s does not satisfy shape %s: method %s must be public",
				concreteDisplay, shapeType.DeclName, methodName)
			return false
		}
		if !funDecl.Pub && concretePub {
			g.diag(span, "type %s does not satisfy shape %s: method %s must not be public",
				concreteDisplay, shapeType.DeclName, methodName)
			return false
		}
		// Re-derive the expected signature WITHOUT substituting the shape's
		// TypeArgs, so the type params show up unbound and can be unified
		// against the concrete signature for a clean type-mismatch report.
		rawExpected, status := g.shapeMethodRawSignature(
			shapeFunBinding, shapeTypeID, lookupTypeID,
		)
		if status.Failed() {
			return false
		}
		methodBindings := map[TypeID]TypeID{}
		maps.Copy(methodBindings, associatedBindings)
		g.inferFunTypeBindings(rawExpected, concreteFunType, methodBindings)
		// Inferred bindings must agree with the shape's concrete TypeArgs,
		// so mismatches surface as "expected Source<Str>, got Source<Int>"
		// rather than a method-level signature diff.
		if len(shapeType.TypeArgs) > 0 {
			for i, paramNodeID := range shapeNode.TypeParams {
				if i >= len(shapeType.TypeArgs) {
					break
				}
				typeParamID, ok := g.env.TypeParamForNode(paramNodeID)
				if !ok {
					continue
				}
				// Placeholder slots (e.g. bare `Cell` leaving Value open)
				// have nothing to compare against; record the associated
				// value for downstream projections.
				if _, isParam := g.env.Type(shapeType.TypeArgs[i]).Kind.(TypeParamType); isParam {
					continue
				}
				inferred, ok := methodBindings[typeParamID]
				if !ok {
					continue
				}
				if inferred != shapeType.TypeArgs[i] {
					g.diag(span, "type mismatch: expected %s, got %s",
						g.env.TypeDisplay(shapeTypeID),
						g.shapeTypeDisplayWith(shapeTypeID, shapeNode, methodBindings))
					return false
				}
			}
		}
		maps.Copy(associatedBindings, methodBindings)
		if len(associatedBindings) > 0 {
			rewritten, _, rewriteStatus := g.rewriteFunType(expectedFunType, associatedBindings)
			if rewriteStatus.Failed() {
				return false
			}
			expectedFunType = rewritten
		}
		if !g.shapeMethodMatches(expectedFunType, concreteFunType) {
			g.diag(span,
				"type %s does not satisfy shape %s: method %s has signature %s, expected %s",
				concreteDisplay,
				shapeType.DeclName,
				methodName,
				g.funTypeDisplay(concreteFunType),
				g.env.TypeDisplay(shapeFunBinding.TypeID),
			)
			return false
		}
	}
	// Record the shape's TypeArgs bindings so later associated-type
	// projections (e.g. `Counter.Item` after Counter satisfies Source<Int>)
	// can resolve via concreteAssociated.
	for i, paramNodeID := range shapeNode.TypeParams {
		if i >= len(shapeType.TypeArgs) {
			break
		}
		typeParamID, ok := g.env.TypeParamForNode(paramNodeID)
		if !ok {
			continue
		}
		if _, set := associatedBindings[typeParamID]; !set {
			associatedBindings[typeParamID] = shapeType.TypeArgs[i]
		}
	}
	if len(associatedBindings) > 0 {
		g.recordConcreteAssociated(lookupTypeID, shapeNode, associatedBindings)
	}
	return true
}

// typeParamSatisfiesShape checks whether a type-parameter argument satisfies
// a shape constraint. An unconstrained type parameter is accepted; subsequent
// misuse is reported when the callee body is instantiated.
func (g *Generics) typeParamSatisfiesShape(
	argTPT TypeParamType, argTypeID, constraintTypeID TypeID, span base.Span,
) bool {
	if argTPT.Shape == nil {
		return true
	}
	if *argTPT.Shape == constraintTypeID {
		return true
	}
	if g.shapeCoversShape(*argTPT.Shape, constraintTypeID) {
		return true
	}
	// Same method names but different visibility / associated types reads
	// better as "type mismatch" than the longer "does not satisfy" wording.
	if g.shapesMatchByName(*argTPT.Shape, constraintTypeID) {
		g.diag(span, "type mismatch: expected %s, got %s",
			g.shapeDisplayName(constraintTypeID),
			g.shapeDisplayName(*argTPT.Shape))
		return false
	}
	g.diag(span, "type parameter %s with constraint %s does not satisfy shape %s",
		g.env.TypeDisplay(argTypeID),
		g.env.TypeDisplay(*argTPT.Shape),
		g.env.TypeDisplay(constraintTypeID))
	return false
}

// shapesMatchByName checks whether `have`'s method names cover `required`'s,
// used to pick between the "type mismatch" and "does not satisfy shape"
// diagnostic phrasings.
func (g *Generics) shapesMatchByName(have, required TypeID) bool {
	haveDecl := g.env.DeclNode(typeIDOrigin(g.env, have))
	reqDecl := g.env.DeclNode(typeIDOrigin(g.env, required))
	if haveDecl == 0 || reqDecl == 0 {
		return false
	}
	haveShape, ok := g.ast.Node(haveDecl).Kind.(ast.Shape)
	if !ok {
		return false
	}
	reqShape, ok := g.ast.Node(reqDecl).Kind.(ast.Shape)
	if !ok {
		return false
	}
	collectNames := func(shape ast.Shape) map[string]bool {
		out := map[string]bool{}
		for _, funDeclNodeID := range shape.Funs {
			funDecl := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
			if _, methodName, found := strings.Cut(funDecl.Name.Name, "."); found {
				out[methodName] = true
			}
		}
		return out
	}
	haveNames := collectNames(haveShape)
	reqNames := collectNames(reqShape)
	for name := range reqNames {
		if !haveNames[name] {
			return false
		}
	}
	return true
}

func (g *Generics) shapeCoversShape(have, required TypeID) bool {
	haveDecl := g.env.DeclNode(typeIDOrigin(g.env, have))
	reqDecl := g.env.DeclNode(typeIDOrigin(g.env, required))
	if haveDecl == 0 || reqDecl == 0 {
		return false
	}
	haveShape, ok := g.ast.Node(haveDecl).Kind.(ast.Shape)
	if !ok {
		return false
	}
	reqShape, ok := g.ast.Node(reqDecl).Kind.(ast.Shape)
	if !ok {
		return false
	}
	haveMethods := map[string]ast.FunDecl{}
	for _, funDeclNodeID := range haveShape.Funs {
		funDecl := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
		if _, methodName, found := strings.Cut(funDecl.Name.Name, "."); found {
			haveMethods[methodName] = funDecl
		}
	}
	for _, funDeclNodeID := range reqShape.Funs {
		funDecl := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		haveDecl, present := haveMethods[methodName]
		if !present {
			return false
		}
		if haveDecl.Pub != funDecl.Pub {
			// Pub disagreement is reported via shapesMatchByName for a
			// cleaner diagnostic.
			return false
		}
	}
	// Same origin: every concrete type argument must match. Source<Str>
	// cannot stand in for Source<Int>. Type-parameter slots are wildcards.
	if typeIDOrigin(g.env, have) == typeIDOrigin(g.env, required) {
		haveArgs := typeArgsOf(g.env.Type(have).Kind)
		reqArgs := typeArgsOf(g.env.Type(required).Kind)
		for i := 0; i < len(haveArgs) && i < len(reqArgs); i++ {
			if !g.typeArgsConsistent(haveArgs[i], reqArgs[i]) {
				return false
			}
		}
		return true
	}
	// Cross-origin: each common method's substituted signature must agree
	// on the non-self portion.
	for _, funDeclNodeID := range reqShape.Funs {
		reqFun := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(reqFun.Name.Name, ".")
		haveFunDecl, ok := haveMethods[methodName]
		if !ok {
			continue
		}
		if !g.shapeMethodSigCompatible(have, required, haveFunDecl, reqFun) {
			return false
		}
	}
	return true
}

// shapeMethodSigCompatible compares two shape methods' substituted signatures
// across different shape origins, normalizing self-typed slots on each side.
func (g *Generics) shapeMethodSigCompatible(have, required TypeID, haveFun, reqFun ast.FunDecl) bool {
	haveSig, status := g.shapeMethodSubstSignature(have, haveFun, required)
	if status.Failed() {
		return true
	}
	reqSig, status := g.shapeMethodSubstSignature(required, reqFun, required)
	if status.Failed() {
		return true
	}
	if len(haveSig.Params) != len(reqSig.Params) {
		return false
	}
	for i, p := range haveSig.Params {
		if !g.typeArgsConsistent(p, reqSig.Params[i]) {
			return false
		}
	}
	return g.typeArgsConsistent(haveSig.Return, reqSig.Return)
}

// shapeMethodMatches checks if a concrete method signature satisfies a
// shape's, allowing &mut T to &T and ref coercion on parameters.
func (g *Generics) shapeMethodMatches(expected, concrete FunType) bool {
	if expected.Return != concrete.Return || expected.Macro != concrete.Macro {
		return false
	}
	if len(expected.Params) != len(concrete.Params) {
		return false
	}
	for i, ep := range expected.Params {
		cp := concrete.Params[i]
		if g.isAssignableTo(ep, cp) {
			continue
		}
		// Concrete may wrap the parameter in a ref where the shape did not.
		if refTyp, ok := g.env.Type(cp).Kind.(RefType); ok {
			if g.isAssignableTo(ep, refTyp.Type) {
				continue
			}
		}
		return false
	}
	return true
}

// shapeMethodSubstSignature returns the FunType for `fun` on `ownerShape`
// with the shape's TypeParams substituted by its TypeArgs and the self-type
// rewritten to `selfRewrite`.
func (g *Generics) shapeMethodSubstSignature(
	ownerShape TypeID, fun ast.FunDecl, selfRewrite TypeID,
) (FunType, TypeStatus) {
	shapeOrigin, _, ok := g.shapeOwner(ownerShape)
	if !ok {
		return FunType{}, TypeFailed
	}
	shapeNode := base.Cast[ast.Shape](g.ast.Node(g.env.DeclNode(shapeOrigin)).Kind)
	defer g.enterChildEnv()()
	if status := g.bindTypeParams(shapeNode.TypeParams); status.Failed() {
		return FunType{}, status
	}
	funTypeID, status := g.shapeFunDeclType(fun)
	if status.Failed() {
		return FunType{}, status
	}
	funType := base.Cast[FunType](g.env.Type(funTypeID).Kind)
	bindings := map[TypeID]TypeID{shapeOrigin: selfRewrite}
	ownerArgs := typeArgsOf(g.env.Type(ownerShape).Kind)
	for i, paramNodeID := range shapeNode.TypeParams {
		if i >= len(ownerArgs) {
			break
		}
		tpID, ok := g.env.TypeParamForNode(paramNodeID)
		if !ok {
			continue
		}
		bindings[tpID] = ownerArgs[i]
	}
	rewritten, _, status := g.rewriteFunType(funType, bindings)
	return rewritten, status
}

// shapeMethodRawSignature returns the shape method's signature with only the
// self-type rewritten to the concrete receiver, leaving the other type
// params open for subsequent unification.
func (g *Generics) shapeMethodRawSignature(
	binding *Binding, shapeTypeID, receiverTypeID TypeID,
) (FunType, TypeStatus) {
	funDecl, ok := g.ast.Node(binding.Decl).Kind.(ast.FunDecl)
	if !ok {
		return FunType{}, TypeFailed
	}
	shapeOrigin, _, ok := g.shapeOwner(shapeTypeID)
	if !ok {
		return FunType{}, TypeFailed
	}
	shapeNodeID := g.env.DeclNode(shapeOrigin)
	shapeNode := base.Cast[ast.Shape](g.ast.Node(shapeNodeID).Kind)
	defer g.enterChildEnv()()
	if status := g.bindTypeParams(shapeNode.TypeParams); status.Failed() {
		return FunType{}, status
	}
	typeID, status := g.shapeFunDeclType(funDecl)
	if status.Failed() {
		return FunType{}, status
	}
	funType := base.Cast[FunType](g.env.Type(typeID).Kind)
	replacements := map[TypeID]TypeID{shapeOrigin: receiverTypeID}
	rewritten, _, status := g.rewriteFunType(funType, replacements)
	if status.Failed() {
		return FunType{}, status
	}
	return rewritten, TypeOK
}

// shapeTypeDisplayWith renders a shape's type display using the supplied
// associated-type bindings, for error messages.
func (g *Generics) shapeTypeDisplayWith(
	shapeTypeID TypeID, shapeNode ast.Shape, bindings map[TypeID]TypeID,
) string {
	args := make([]TypeID, 0, len(shapeNode.TypeParams))
	for _, paramNodeID := range shapeNode.TypeParams {
		typeParamID, ok := g.env.TypeParamForNode(paramNodeID)
		if !ok {
			return g.env.TypeDisplay(shapeTypeID)
		}
		bound, ok := bindings[typeParamID]
		if !ok {
			return g.env.TypeDisplay(shapeTypeID)
		}
		args = append(args, bound)
	}
	shapeType := base.Cast[ShapeType](g.env.Type(shapeTypeID).Kind)
	var sb strings.Builder
	sb.WriteString(shapeType.DeclName)
	sb.WriteByte('<')
	for i, argID := range args {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(g.env.TypeDisplay(argID))
	}
	sb.WriteByte('>')
	return sb.String()
}

// shapeDisplayName returns the shape's name for error messages, stripping
// module prefixes the user did not write.
func (g *Generics) shapeDisplayName(shapeTypeID TypeID) string {
	shapeType, ok := g.env.Type(shapeTypeID).Kind.(ShapeType)
	if !ok {
		return g.env.TypeDisplay(shapeTypeID)
	}
	if len(shapeType.TypeArgs) == 0 {
		return shapeType.DeclName
	}
	return g.env.TypeDisplay(shapeTypeID)
}

// typeArgsConsistent reports whether two type arguments could be the same in
// any reasonable instantiation. Type parameters on either side are treated as
// wildcards that bind at the call site.
func (g *Generics) typeArgsConsistent(a, b TypeID) bool {
	if a == b {
		return true
	}
	if _, isParamA := g.env.Type(a).Kind.(TypeParamType); isParamA {
		return true
	}
	if _, isParamB := g.env.Type(b).Kind.(TypeParamType); isParamB {
		return true
	}
	switch aKind := g.env.Type(a).Kind.(type) {
	case SliceType:
		bKind, ok := g.env.Type(b).Kind.(SliceType)
		return ok && aKind.Mut == bKind.Mut && g.typeArgsConsistent(aKind.Elem, bKind.Elem)
	case ArrayType:
		bKind, ok := g.env.Type(b).Kind.(ArrayType)
		return ok && aKind.Len == bKind.Len && g.typeArgsConsistent(aKind.Elem, bKind.Elem)
	case RefType:
		bKind, ok := g.env.Type(b).Kind.(RefType)
		return ok && aKind.Mut == bKind.Mut && g.typeArgsConsistent(aKind.Type, bKind.Type)
	}
	return false
}

func (g *Generics) lookupShapeMethodBinding(
	shapeTypeID TypeID,
	methodName string,
	scopeNodeID ast.NodeID,
) (*Binding, bool) {
	shapeOriginID := typeIDOrigin(g.env, shapeTypeID)
	shapeType := base.Cast[ShapeType](g.env.Type(shapeOriginID).Kind)
	bindName := shapeType.DeclName + "." + methodName
	binding, ok := g.lookup(scopeNodeID, bindName, -1)
	if !ok {
		binding, ok = g.lookupInTypeModule(g.env.Type(shapeOriginID), bindName)
	}
	return binding, ok
}

//nolint:funlen
func (g *Generics) methodSignature(
	binding *Binding,
	ctx MethodContext,
	scopeNodeID ast.NodeID,
	span base.Span,
) (FunType, TypeStatus) {
	receiverType := g.env.Type(ctx.ReceiverTypeID)
	if refTyp, ok := receiverType.Kind.(RefType); ok {
		receiverType = g.env.Type(refTyp.Type)
	}
	if funDecl, ok := g.ast.Node(binding.Decl).Kind.(ast.FunDecl); ok {
		shapeDecl, _ := g.normalizeGenericDecl(binding.Decl, ctx.DeclTypeID, binding.Name)
		shapeType := base.Cast[ShapeType](g.env.Type(ctx.OwnerTypeID).Kind)
		spec, status := g.buildGenericSpec(shapeDecl.originTypeID, shapeDecl.typeParams)
		if status.Failed() {
			return FunType{}, status
		}
		replacements := spec.Bindings(shapeType.TypeArgs)
		replacements[shapeDecl.originTypeID] = ctx.ReceiverTypeID
		defer g.enterChildEnv()()
		g.bindGenericArgs(spec, shapeType.TypeArgs)
		typeID, status := g.shapeFunDeclType(funDecl)
		if status.Failed() {
			return FunType{}, status
		}
		funType := base.Cast[FunType](g.env.Type(typeID).Kind)
		rewritten, _, status := g.rewriteFunType(funType, replacements)
		if status.Failed() {
			return FunType{}, status
		}
		return rewritten, TypeOK
	}
	if _, ok := g.ast.Node(binding.Decl).Kind.(ast.Fun); ok && len(g.funTypeParams(binding.Decl)) > 0 {
		seedArgs, ok := ImplicitTypeArgs(receiverType.Kind)
		if !ok {
			return FunType{}, TypeFailed
		}
		if len(seedArgs) > 0 {
			decl, _ := g.normalizeGenericDecl(binding.Decl, binding.TypeID, "")
			spec, status := g.buildGenericSpec(decl.originTypeID, decl.typeParams)
			if status.Failed() {
				return FunType{}, status
			}
			typeArgIDs := seedArgs
			if len(seedArgs) < len(spec.Params) {
				// The method also has its own type params: infer them by
				// matching the receiver against the first param type.
				genericFunType := base.Cast[FunType](g.env.Type(binding.TypeID).Kind)
				if len(genericFunType.Params) > 0 {
					bindings := map[TypeID]TypeID{}
					for i, paramID := range spec.Params {
						if i >= len(seedArgs) {
							break
						}
						bindings[paramID.TypeID] = seedArgs[i]
					}
					g.inferTypeBindings(genericFunType.Params[0], ctx.ReceiverTypeID, bindings)
					inferred := make([]TypeID, 0, len(spec.Params))
					for _, param := range spec.Params {
						bound, ok := bindings[param.TypeID]
						if !ok {
							break
						}
						inferred = append(inferred, bound)
					}
					if len(inferred) == len(spec.Params) {
						typeArgIDs = inferred
					}
				}
			}
			typeArgIDs, status = g.solveGenericArgs(spec, typeArgIDs, scopeNodeID, span)
			if status.Failed() {
				return FunType{}, status
			}
			mat, methodTypeID, _, status := g.materializeFun(
				binding.Decl,
				binding.TypeID,
				scopeNodeID,
				typeArgIDs,
			)
			if status.Failed() {
				return FunType{}, status
			}
			g.scheduleBodyCheck(mat, methodTypeID)
			return base.Cast[FunType](g.env.Type(methodTypeID).Kind), TypeOK
		}
	}
	return base.Cast[FunType](g.env.Type(binding.TypeID).Kind), TypeOK
}

func (g *Generics) shapeOwner(originTypeID TypeID) (TypeID, ast.Shape, bool) {
	ownerTypeID := typeIDOrigin(g.env, originTypeID)
	if tpt, ok := g.env.Type(ownerTypeID).Kind.(TypeParamType); ok && tpt.Shape != nil {
		ownerTypeID = typeIDOrigin(g.env, *tpt.Shape)
	}
	shapeNode, ok := g.ast.Node(g.env.DeclNode(ownerTypeID)).Kind.(ast.Shape)
	if !ok {
		return InvalidTypeID, ast.Shape{}, false
	}
	return ownerTypeID, shapeNode, true
}

type MethodContext struct {
	DeclTypeID     TypeID
	OwnerTypeID    TypeID
	ReceiverTypeID TypeID
}

func (g *Generics) shapeMethodContext(shapeTypeID, receiverTypeID TypeID) MethodContext {
	ownerTypeID, _, ok := g.shapeOwner(shapeTypeID)
	if !ok {
		panic(base.Errorf("shape owner not found for %s", shapeTypeID))
	}
	return MethodContext{DeclTypeID: ownerTypeID, OwnerTypeID: shapeTypeID, ReceiverTypeID: receiverTypeID}
}

func (g *Generics) boundMethodContext(binding *Binding, receiverTypeID TypeID) MethodContext {
	if _, ok := g.ast.Node(binding.Decl).Kind.(ast.FunDecl); !ok {
		return MethodContext{DeclTypeID: receiverTypeID, OwnerTypeID: receiverTypeID, ReceiverTypeID: receiverTypeID}
	}
	receiverType := g.env.Type(receiverTypeID)
	if refType, ok := receiverType.Kind.(RefType); ok {
		receiverTypeID = refType.Type
		receiverType = g.env.Type(receiverTypeID)
	}
	if tpt, ok := receiverType.Kind.(TypeParamType); ok && tpt.Shape != nil {
		return g.shapeMethodContext(*tpt.Shape, receiverTypeID)
	}
	return MethodContext{DeclTypeID: receiverTypeID, OwnerTypeID: receiverTypeID, ReceiverTypeID: receiverTypeID}
}

// recordConcreteAssociated stores the bindings learned during satisfiesShape
// and recursively verifies each associated value against the corresponding
// shape type-param's constraint, so chained projections (e.g.
// `H.Inner.Value`) are discoverable.
func (g *Generics) recordConcreteAssociated(
	concreteTypeID TypeID, shapeNode ast.Shape, bindings map[TypeID]TypeID,
) {
	entry, ok := g.concreteAssociated[concreteTypeID]
	if !ok {
		entry = map[string]TypeID{}
		g.concreteAssociated[concreteTypeID] = entry
	}
	for _, paramNodeID := range shapeNode.TypeParams {
		typeParamID, ok := g.env.TypeParamForNode(paramNodeID)
		if !ok {
			continue
		}
		bound, ok := bindings[typeParamID]
		if !ok {
			continue
		}
		paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
		entry[paramNode.Name.Name] = bound
	}
	for _, paramNodeID := range shapeNode.TypeParams {
		typeParamID, ok := g.env.TypeParamForNode(paramNodeID)
		if !ok {
			continue
		}
		bound, ok := bindings[typeParamID]
		if !ok {
			continue
		}
		// Recurse into the slot's own constraint to record bindings for
		// later chained projections.
		tpt, isParam := g.env.Type(typeParamID).Kind.(TypeParamType)
		if !isParam || tpt.Shape == nil {
			continue
		}
		if _, alreadyParam := g.env.Type(bound).Kind.(TypeParamType); alreadyParam {
			continue
		}
		if _, already := g.concreteAssociated[bound]; already {
			continue
		}
		g.satisfiesShape(bound, *tpt.Shape, g.env.DeclNode(concreteTypeID), g.ast.Node(paramNodeID).Span)
	}
}

// propagateNodeTypesToParent copies a shape method's param and return-type
// node cache from the current child env up to the parent so later passes
// (e.g. lifetime analysis) can resolve the nodes from the outer env.
func (g *Generics) propagateNodeTypesToParent(funDecl ast.FunDecl) {
	if g.env.parent == nil {
		return
	}
	copyEntry := func(nodeID ast.NodeID) {
		if nodeID == 0 {
			return
		}
		if cached, ok := g.env.nodes[nodeID]; ok {
			if _, exists := g.env.parent.nodes[nodeID]; !exists {
				g.env.parent.nodes[nodeID] = cached
			}
		}
		g.env.ast.Walk(nodeID, func(child ast.NodeID) {
			if cached, ok := g.env.nodes[child]; ok {
				if _, exists := g.env.parent.nodes[child]; !exists {
					g.env.parent.nodes[child] = cached
				}
			}
		})
	}
	for _, paramNodeID := range funDecl.Params {
		copyEntry(paramNodeID)
	}
	copyEntry(funDecl.ReturnType)
}

func (g *Generics) collectTypeParamNodeIDs(typeID TypeID, out map[ast.NodeID]bool) {
	typ := g.env.Type(typeID)
	if _, ok := typ.Kind.(TypeParamType); ok && typ.NodeID != 0 {
		out[typ.NodeID] = true
	}
	switch kind := typ.Kind.(type) {
	case StructType:
		for _, arg := range kind.TypeArgs {
			g.collectTypeParamNodeIDs(arg, out)
		}
		for _, field := range kind.Fields {
			g.collectTypeParamNodeIDs(field.Type, out)
		}
	case UnionType:
		for _, arg := range kind.TypeArgs {
			g.collectTypeParamNodeIDs(arg, out)
		}
		for _, variant := range kind.Variants {
			g.collectTypeParamNodeIDs(variant, out)
		}
	case ShapeType:
		for _, arg := range kind.TypeArgs {
			g.collectTypeParamNodeIDs(arg, out)
		}
	case FunType:
		for _, p := range kind.Params {
			g.collectTypeParamNodeIDs(p, out)
		}
		g.collectTypeParamNodeIDs(kind.Return, out)
	case RefType:
		g.collectTypeParamNodeIDs(kind.Type, out)
	case ArrayType:
		g.collectTypeParamNodeIDs(kind.Elem, out)
	case SliceType:
		g.collectTypeParamNodeIDs(kind.Elem, out)
	}
}

//nolint:funlen
func (g *Generics) completeShapeType(
	node *ast.Node,
	shapeNode ast.Shape,
	shapeType ShapeType,
) (TypeStatus, ShapeType) {
	type methodInfo struct {
		nodeID   ast.NodeID
		name     string
		nameSpan base.Span
		typeID   TypeID
	}
	var methods []methodInfo
	resolveInner := func() TypeStatus {
		if len(shapeNode.TypeParams) > 0 {
			defer g.enterChildEnv()()
			if status := g.bindTypeParams(shapeNode.TypeParams); status.Failed() {
				return status
			}
			implicitArgs := make([]TypeID, len(shapeNode.TypeParams))
			for i, paramNodeID := range shapeNode.TypeParams {
				implicitArgs[i], _ = g.env.TypeParamForNode(paramNodeID)
			}
			if cached := g.env.safeNodeType(node.ID); cached != nil && cached.Type != nil {
				g.implicitOwnerArgs[cached.Type.ID] = implicitArgs
			}
		}
		for _, funDeclNodeID := range shapeNode.Funs {
			funDecl := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
			funTypeID, status := g.shapeFunDeclType(funDecl)
			if status.Failed() {
				return status
			}
			_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
			methods = append(
				methods,
				methodInfo{nodeID: funDeclNodeID, name: methodName, nameSpan: funDecl.Name.Span, typeID: funTypeID},
			)
			// Lifetime analysis runs later in the outer env and reads these
			// nodes via TypeOfNode, so propagate the cached types from the
			// current (child) env up to the parent.
			g.propagateNodeTypesToParent(funDecl)
		}
		return TypeOK
	}
	if status := resolveInner(); status.Failed() {
		return status, shapeType
	}
	// The built-in `Enum` constraint's `Item` slot (the backing int) is bound by
	// the compiler, not by a method contract, so skip the open-slot check.
	isEnumConstraint := ast.IsPreludeNode(node.ID) && shapeType.DeclName == "Enum"
	if len(shapeNode.TypeParams) > 0 && !isEnumConstraint {
		used := map[ast.NodeID]bool{}
		for _, method := range methods {
			funType := base.Cast[FunType](g.env.Type(method.typeID).Kind)
			for _, paramTypeID := range funType.Params {
				g.collectTypeParamNodeIDs(paramTypeID, used)
			}
			g.collectTypeParamNodeIDs(funType.Return, used)
		}
		for _, paramNodeID := range shapeNode.TypeParams {
			if used[paramNodeID] {
				continue
			}
			paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
			g.diag(paramNode.Name.Span,
				"open shape slot: %s is not used by the shape contract",
				paramNode.Name.Name)
		}
	}
	parentScope := g.scopeGraph.NodeScope(node.ID)
	for _, method := range methods {
		bindName := shapeType.DeclName + "." + method.name
		if !g.env.bindInScope(parentScope, method.nodeID, bindName, method.typeID) {
			g.diag(method.nameSpan, "symbol already defined: %s", bindName)
		}
	}
	return TypeOK, shapeType
}

func (g *Generics) shapeFunDeclType(funDecl ast.FunDecl) (TypeID, TypeStatus) {
	paramTypeIDs := make([]TypeID, len(funDecl.Params))
	for i, paramNodeID := range funDecl.Params {
		paramTypeID, status := g.query(paramNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		paramTypeIDs[i] = paramTypeID
	}
	retTypeID, status := g.query(funDecl.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	funType := FunType{
		Params:         paramTypeIDs,
		Return:         retTypeID,
		Macro:          false,
		Sync:           false,
		Unsafe:         funDecl.Unsafe,
		NoescapeParams: make([]bool, len(paramTypeIDs)),
		NoescapeReturn: false,
	}
	return g.env.newType(funType, 0, base.Span{}, TypeOK), TypeOK
}

func (g *Generics) prepareFunTypeParams(node *ast.Node, fun ast.FunDecl) TypeStatus {
	implicitParams, ownerTypeID, implicitOK := g.implicitMethodOwnerParams(node.ID, fun)
	if !implicitOK {
		return TypeFailed
	}
	if len(implicitParams) > 0 {
		funScope := g.scopeGraph.IntroducedScope(node.ID)
		implicitArgs := make([]TypeID, len(implicitParams))
		for i, paramNodeID := range implicitParams {
			paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
			typeParamID, ok := g.env.TypeParamForNode(paramNodeID)
			if !ok {
				if status := g.materializeOwnerTypeParam(paramNodeID); status.Failed() {
					return status
				}
				typeParamID, _ = g.env.TypeParamForNode(paramNodeID)
			}
			implicitArgs[i] = typeParamID
			g.env.bindInScope(funScope, paramNodeID, paramNode.Name.Name, typeParamID)
		}
		extended := make([]ast.NodeID, 0, len(implicitParams)+len(fun.TypeParams))
		extended = append(extended, implicitParams...)
		extended = append(extended, fun.TypeParams...)
		g.env.setFunTypeParams(node.ID, extended)
		g.implicitOwnerArgs[ownerTypeID] = implicitArgs
	}
	return g.bindTypeParams(fun.TypeParams)
}

func (g *Generics) bindTypeParams(typeParamNodeIDs []ast.NodeID) TypeStatus {
	seen := map[string]bool{}
	for _, typeParamNodeID := range typeParamNodeIDs {
		typeParamNode := base.Cast[ast.TypeParam](g.ast.Node(typeParamNodeID).Kind)
		if seen[typeParamNode.Name.Name] {
			g.diag(typeParamNode.Name.Span, "duplicate type parameter: %s", typeParamNode.Name.Name)
			return TypeFailed
		}
		seen[typeParamNode.Name.Name] = true

		typeParamID, ok := g.env.TypeParamForNode(typeParamNodeID)
		if !ok {
			shapeID, status := g.resolveTypeParamConstraint(typeParamNode.Constraint)
			if status.Failed() {
				return status
			}

			var defaultID *TypeID
			if typeParamNode.Default != nil {
				defaultTypeID, status := g.query(*typeParamNode.Default)
				if status.Failed() {
					return TypeDepFailed
				}
				if shapeID != nil {
					span := g.ast.Node(*typeParamNode.Default).Span
					if !g.satisfiesShape(defaultTypeID, *shapeID, typeParamNodeID, span) {
						return TypeFailed
					}
				}
				defaultID = &defaultTypeID
			}

			typeParamID = g.env.newType(
				TypeParamType{Shape: shapeID, Default: defaultID},
				typeParamNodeID,
				g.ast.Node(typeParamNodeID).Span,
				TypeOK,
			)
			g.env.setTypeParamForNode(typeParamNodeID, typeParamID)
		}

		g.bind(typeParamNodeID, typeParamNode.Name.Name, false, typeParamID, typeParamNode.Name.Span, -1)
	}
	return TypeOK
}

func (g *Generics) resolveTypeParamConstraint(constraintNode *ast.NodeID) (*TypeID, TypeStatus) {
	if constraintNode == nil {
		return nil, TypeOK
	}
	constraintTypeID, status := g.query(*constraintNode)
	if status.Failed() {
		return nil, TypeDepFailed
	}
	shapeType, ok := g.env.Type(constraintTypeID).Kind.(ShapeType)
	if !ok {
		g.diag(g.ast.Node(*constraintNode).Span, "constraint must be a shape")
		return nil, TypeFailed
	}
	shapeNodeID := g.env.DeclNode(constraintTypeID)
	if shapeNodeID != 0 {
		if shapeNode, ok := g.ast.Node(shapeNodeID).Kind.(ast.Shape); ok {
			if len(shapeNode.TypeParams) > 0 && len(shapeType.TypeArgs) == 0 {
				g.diag(g.ast.Node(*constraintNode).Span,
					"shape %s requires type arguments", shapeType.DeclName)
				return nil, TypeFailed
			}
		}
	}
	return &constraintTypeID, TypeOK
}

// implicitMethodOwnerParams returns the type-param node IDs the method
// implicitly inherits from its receiver's generic owner. The bool is false
// when shadowing of the owner's params is detected.
func (g *Generics) implicitMethodOwnerParams(
	nodeID ast.NodeID, fun ast.FunDecl,
) ([]ast.NodeID, TypeID, bool) {
	structName, _, hasDot := strings.Cut(fun.Name.Name, ".")
	if !hasDot {
		return nil, InvalidTypeID, true
	}
	if isBuiltinGenericOwner(structName) {
		return g.builtinOwnerImplicitParams(nodeID, fun)
	}
	binding, ok := g.lookup(nodeID, structName, -1)
	if !ok {
		return nil, InvalidTypeID, true
	}
	declNode := g.ast.Node(binding.Decl)
	var ownerParams []ast.NodeID
	switch kind := declNode.Kind.(type) {
	case ast.Struct:
		ownerParams = kind.TypeParams
	case ast.Union:
		ownerParams = kind.TypeParams
	case ast.Shape:
		ownerParams = kind.TypeParams
	default:
		return nil, InvalidTypeID, true
	}
	if len(ownerParams) == 0 {
		return nil, binding.TypeID, true
	}
	// Detect re-entry: a previous pass already extended this FunDecl.
	if len(fun.TypeParams) >= len(ownerParams) {
		already := true
		for i, ownerNodeID := range ownerParams {
			if fun.TypeParams[i] != ownerNodeID {
				already = false
				break
			}
		}
		if already {
			return nil, binding.TypeID, true
		}
	}
	ownerParamNames := make(map[string]bool, len(ownerParams))
	for _, paramNodeID := range ownerParams {
		paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
		ownerParamNames[paramNode.Name.Name] = true
	}
	for _, paramNodeID := range fun.TypeParams {
		paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
		if ownerParamNames[paramNode.Name.Name] {
			g.diag(paramNode.Name.Span, "type parameter %s shadows owner type parameter", paramNode.Name.Name)
			return nil, binding.TypeID, false
		}
	}
	if !g.signatureUsesOwnerSlot(structName, ownerParamNames, fun) {
		return nil, binding.TypeID, true
	}
	return ownerParams, binding.TypeID, true
}

// signatureUsesOwnerSlot reports whether the method signature names the owner
// without type args or references any of the owner's TypeParam names as a
// bare type.
func (g *Generics) signatureUsesOwnerSlot(
	ownerName string, ownerParamNames map[string]bool, fun ast.FunDecl,
) bool {
	var found bool
	var visit func(nodeID ast.NodeID)
	visit = func(nodeID ast.NodeID) {
		if found || nodeID == 0 {
			return
		}
		if kind, ok := g.ast.Node(nodeID).Kind.(ast.SimpleType); ok {
			if kind.Name.Name == ownerName && len(kind.TypeArgs) == 0 {
				found = true
				return
			}
			if ownerParamNames[kind.Name.Name] {
				found = true
				return
			}
		}
		g.ast.Walk(nodeID, visit)
	}
	for _, paramNodeID := range fun.Params {
		visit(paramNodeID)
	}
	visit(fun.ReturnType)
	return found
}

// builtinOwnerImplicitParams synthesizes a TypeParam AST node for a built-in
// generic owner whose owner has no AST decl. The implicit `T` is only added
// when the method's signature actually references it, otherwise methods that
// redeclare their own type parameter would fail count checks.
func (g *Generics) builtinOwnerImplicitParams(
	funNodeID ast.NodeID, fun ast.FunDecl,
) ([]ast.NodeID, TypeID, bool) {
	if !g.builtinOwnerImplicitNeeded(fun) {
		return nil, InvalidTypeID, true
	}
	// Each method gets its own synthetic T NodeID. Sharing across methods
	// would collide in env.bindings, which is keyed by BindingID(decl).
	span := fun.Name.Span
	tpNodeID := g.ast.NewTypeParam(
		ast.Name{Name: "T", Span: span}, nil, nil, ast.SyncNone, span,
	)
	g.scopeGraph.SetNodeScope(tpNodeID, g.scopeGraph.IntroducedScope(funNodeID))
	params := []ast.NodeID{tpNodeID}
	return g.checkBuiltinOwnerShadow(fun, params)
}

func (g *Generics) builtinOwnerImplicitNeeded(fun ast.FunDecl) bool {
	declares := map[string]bool{}
	for _, paramNodeID := range fun.TypeParams {
		paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
		declares[paramNode.Name.Name] = true
	}
	if declares["T"] {
		return false
	}
	found := false
	var visit func(nodeID ast.NodeID)
	visit = func(nodeID ast.NodeID) {
		if found || nodeID == 0 {
			return
		}
		if kind, ok := g.ast.Node(nodeID).Kind.(ast.SimpleType); ok {
			if kind.Name.Name == "T" {
				found = true
				return
			}
		}
		g.ast.Walk(nodeID, visit)
	}
	for _, paramNodeID := range fun.Params {
		visit(paramNodeID)
	}
	visit(fun.ReturnType)
	return found
}

func (g *Generics) checkBuiltinOwnerShadow(
	fun ast.FunDecl, ownerParams []ast.NodeID,
) ([]ast.NodeID, TypeID, bool) {
	if len(fun.TypeParams) >= len(ownerParams) {
		already := true
		for i, ownerNodeID := range ownerParams {
			if fun.TypeParams[i] != ownerNodeID {
				already = false
				break
			}
		}
		if already {
			return nil, InvalidTypeID, true
		}
	}
	ownerNames := map[string]bool{}
	for _, paramNodeID := range ownerParams {
		paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
		ownerNames[paramNode.Name.Name] = true
	}
	for _, paramNodeID := range fun.TypeParams {
		paramNode := base.Cast[ast.TypeParam](g.ast.Node(paramNodeID).Kind)
		if ownerNames[paramNode.Name.Name] {
			g.diag(paramNode.Name.Span, "type parameter %s shadows owner type parameter", paramNode.Name.Name)
			return nil, InvalidTypeID, false
		}
	}
	return ownerParams, InvalidTypeID, true
}

// isBuiltinGenericOwner reports whether the owner has no AST decl but its
// methods still carry an implicit `T`. Slice is the only such owner;
// Ptr/PtrMut have real prelude declarations.
func isBuiltinGenericOwner(name string) bool {
	return name == "Slice"
}

func (g *Generics) visibleTypeParamBindings() map[TypeID]TypeID {
	replacements := map[TypeID]TypeID{}
	for env := g.env; env != nil; env = env.parent {
		for _, binding := range env.bindings {
			if binding.Decl == 0 {
				continue
			}
			if _, ok := g.ast.Node(binding.Decl).Kind.(ast.TypeParam); !ok {
				continue
			}
			paramTypeID, ok := g.env.TypeParamForNode(binding.Decl)
			if !ok || paramTypeID == binding.TypeID {
				continue
			}
			if _, exists := replacements[paramTypeID]; !exists {
				replacements[paramTypeID] = binding.TypeID
			}
		}
	}
	return replacements
}

func typeIDOrigin(env *TypeEnv, typeID TypeID) TypeID {
	if origin, ok := env.GenericOrigin(typeID); ok {
		return origin
	}
	return typeID
}

func sameOrigin(env *TypeEnv, lhs, rhs TypeID) bool {
	return typeIDOrigin(env, lhs) == typeIDOrigin(env, rhs)
}

func (g *Generics) mangledName(baseName string, genericTypeID TypeID, typeArgIDs []TypeID) string {
	parts := make([]string, len(typeArgIDs))
	for i, id := range typeArgIDs {
		parts[i] = id.String()
	}
	return fmt.Sprintf("%s.%s.%s", baseName, genericTypeID, strings.Join(parts, "."))
}

func (g *Generics) funTypeDisplay(funType FunType) string {
	typeID := g.env.newType(funType, 0, base.Span{}, TypeOK)
	return g.env.TypeDisplay(typeID)
}
