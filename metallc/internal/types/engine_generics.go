package types

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

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
	if s == nil {
		return 0
	}
	for i, param := range s.Params {
		if param.Default != nil {
			return i
		}
	}
	return len(s.Params)
}

func (s *GenericSpec) Bindings(typeArgIDs []TypeID) map[TypeID]TypeID {
	bindings := make(map[TypeID]TypeID, len(typeArgIDs))
	if s == nil {
		return bindings
	}
	for i, param := range s.Params {
		if i >= len(typeArgIDs) {
			break
		}
		bindings[param.TypeID] = typeArgIDs[i]
	}
	return bindings
}

// typeParamSatisfiesShape checks whether a type parameter argument satisfies a
// shape constraint required by the callee. An unconstrained type parameter is
// accepted here and any subsequent misuse is reported when the callee body is
// instantiated. When the argument carries its own shape constraint, the
// argument's shape must structurally cover the required constraint: every
// method declared by the required shape must also be declared by the
// argument's shape.
func (e *Engine) typeParamSatisfiesShape(
	argTPT TypeParamType, argTypeID, constraintTypeID TypeID, span base.Span,
) bool {
	if argTPT.Shape == nil {
		return true
	}
	if *argTPT.Shape == constraintTypeID {
		return true
	}
	if e.shapeCoversShape(*argTPT.Shape, constraintTypeID) {
		return true
	}
	e.diag(span, "type parameter %s with constraint %s does not satisfy shape %s",
		e.env.TypeDisplay(argTypeID),
		e.env.TypeDisplay(*argTPT.Shape),
		e.env.TypeDisplay(constraintTypeID))
	return false
}

// shapeCoversShape returns true when every method declared in `required` is
// also declared in `have`. This is a name-only check; signature compatibility
// will be verified later when the callee body is materialized against the
// concrete type that ultimately substitutes the type parameter.
func (e *Engine) shapeCoversShape(have, required TypeID) bool {
	haveDecl := e.env.DeclNode(typeIDOrigin(e.env, have))
	reqDecl := e.env.DeclNode(typeIDOrigin(e.env, required))
	if haveDecl == 0 || reqDecl == 0 {
		return false
	}
	haveShape, ok := e.ast.Node(haveDecl).Kind.(ast.Shape)
	if !ok {
		return false
	}
	reqShape, ok := e.ast.Node(reqDecl).Kind.(ast.Shape)
	if !ok {
		return false
	}
	haveMethods := map[string]bool{}
	for _, funDeclNodeID := range haveShape.Funs {
		funDecl := base.Cast[ast.FunDecl](e.ast.Node(funDeclNodeID).Kind)
		if _, methodName, found := strings.Cut(funDecl.Name.Name, "."); found {
			haveMethods[methodName] = true
		}
	}
	for _, funDeclNodeID := range reqShape.Funs {
		funDecl := base.Cast[ast.FunDecl](e.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		if !haveMethods[methodName] {
			return false
		}
	}
	return true
}

func (e *Engine) resolveTypeParamConstraint(constraintNode *ast.NodeID) (*TypeID, TypeStatus) {
	if constraintNode == nil {
		return nil, TypeOK
	}
	constraintTypeID, status := e.Query(*constraintNode)
	if status.Failed() {
		return nil, TypeDepFailed
	}
	shapeType, ok := e.env.Type(constraintTypeID).Kind.(ShapeType)
	if !ok {
		e.diag(e.ast.Node(*constraintNode).Span, "constraint must be a shape")
		return nil, TypeFailed
	}
	shapeNodeID := e.env.DeclNode(constraintTypeID)
	if shapeNodeID != 0 {
		if shapeNode, ok := e.ast.Node(shapeNodeID).Kind.(ast.Shape); ok {
			if len(shapeNode.TypeParams) > 0 && len(shapeType.TypeArgs) == 0 {
				e.diag(e.ast.Node(*constraintNode).Span,
					"shape %s requires type arguments", shapeType.DeclName)
				return nil, TypeFailed
			}
		}
	}
	return &constraintTypeID, TypeOK
}

func (e *Engine) bindTypeParams(typeParamNodeIDs []ast.NodeID) TypeStatus {
	seen := map[string]bool{}
	for _, typeParamNodeID := range typeParamNodeIDs {
		typeParamNode := base.Cast[ast.TypeParam](e.ast.Node(typeParamNodeID).Kind)
		if seen[typeParamNode.Name.Name] {
			e.diag(typeParamNode.Name.Span, "duplicate type parameter: %s", typeParamNode.Name.Name)
			return TypeFailed
		}
		seen[typeParamNode.Name.Name] = true

		typeParamID, ok := e.env.reg.typeParamTypes[typeParamNodeID]
		if !ok {
			shapeID, status := e.resolveTypeParamConstraint(typeParamNode.Constraint)
			if status.Failed() {
				return status
			}

			var defaultID *TypeID
			if typeParamNode.Default != nil {
				defaultTypeID, status := e.Query(*typeParamNode.Default)
				if status.Failed() {
					return TypeDepFailed
				}
				if shapeID != nil {
					span := e.ast.Node(*typeParamNode.Default).Span
					if !e.SatisfiesShape(defaultTypeID, *shapeID, typeParamNodeID, span) {
						return TypeFailed
					}
				}
				defaultID = &defaultTypeID
			}

			typeParamID = e.env.newType(
				TypeParamType{Shape: shapeID, Default: defaultID},
				typeParamNodeID,
				e.ast.Node(typeParamNodeID).Span,
				TypeOK,
			)
			e.env.reg.typeParamTypes[typeParamNodeID] = typeParamID
		}

		e.bind(typeParamNodeID, typeParamNode.Name.Name, false, typeParamID, typeParamNode.Name.Span, -1)
	}
	return TypeOK
}

func (e *Engine) BuildGenericSpec(originTypeID TypeID, typeParamNodeIDs []ast.NodeID) (*GenericSpec, TypeStatus) {
	if originTypeID != InvalidTypeID {
		if spec, ok := e.env.GenericSpec(originTypeID); ok {
			return spec, TypeOK
		}
	}
	defer e.enterChildEnv()()
	if status := e.bindTypeParams(typeParamNodeIDs); status.Failed() {
		return nil, status
	}
	params := make([]GenericParam, len(typeParamNodeIDs))
	for i, nodeID := range typeParamNodeIDs {
		typeParamNode := base.Cast[ast.TypeParam](e.ast.Node(nodeID).Kind)
		typeParamTypeID := e.env.reg.typeParamTypes[nodeID]
		tpt := base.Cast[TypeParamType](e.env.Type(typeParamTypeID).Kind)
		params[i] = GenericParam{
			NodeID:     nodeID,
			TypeID:     typeParamTypeID,
			Name:       typeParamNode.Name.Name,
			Constraint: tpt.Shape,
			Default:    tpt.Default,
			Sync:       typeParamNode.Sync == ast.SyncSync,
		}
	}
	spec := &GenericSpec{DeclNodeID: e.env.DeclNode(originTypeID), OriginTypeID: originTypeID, Params: params}
	if originTypeID != InvalidTypeID {
		e.env.reg.genericSpecs[originTypeID] = spec
	}
	return spec, TypeOK
}

func (e *Engine) ResolveTypeArgs(
	typeParamNodeIDs []ast.NodeID, typeArgNodeIDs []ast.NodeID, scopeNodeID ast.NodeID, span base.Span,
) ([]TypeID, TypeStatus) {
	provided, status := e.QueryTypeArgs(typeArgNodeIDs)
	if status.Failed() {
		return nil, status
	}
	spec, status := e.BuildGenericSpec(InvalidTypeID, typeParamNodeIDs)
	if status.Failed() {
		return nil, status
	}
	return e.SolveGenericArgs(spec, provided, scopeNodeID, span)
}

func (e *Engine) SolveGenericArgs(
	spec *GenericSpec, provided []TypeID, scopeNodeID ast.NodeID, span base.Span,
) ([]TypeID, TypeStatus) {
	if spec == nil {
		return nil, TypeOK
	}
	providedCount := len(provided)
	minArgs := spec.MinArgs()
	total := len(spec.Params)
	if providedCount < minArgs || providedCount > total {
		if minArgs == total {
			e.diag(span, "type argument count mismatch: expected %d, got %d", total, providedCount)
		} else {
			e.diag(span, "type argument count mismatch: expected %d to %d, got %d", minArgs, total, providedCount)
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
			argTypeID, status = e.RewriteType(*param.Default, bindings)
			if status.Failed() {
				return nil, status
			}
		}
		if param.Constraint != nil {
			constraintTypeID, status := e.RewriteType(*param.Constraint, bindings)
			if status.Failed() {
				return nil, status
			}
			if argTPT, isTypeParam := e.env.Type(argTypeID).Kind.(TypeParamType); isTypeParam {
				if !e.typeParamSatisfiesShape(argTPT, argTypeID, constraintTypeID, span) {
					return nil, TypeFailed
				}
			} else if !e.SatisfiesShape(argTypeID, constraintTypeID, scopeNodeID, span) {
				return nil, TypeFailed
			}
		}
		if param.Sync {
			if _, isTypeParam := e.env.Type(argTypeID).Kind.(TypeParamType); !isTypeParam &&
				!e.isSync(argTypeID) {
				e.diag(span, "type argument %s must be sync, got %s", param.Name, e.env.TypeDisplay(argTypeID))
				return nil, TypeFailed
			}
		}
		resolved[i] = argTypeID
		bindings[param.TypeID] = argTypeID
	}
	return resolved, TypeOK
}

func (e *Engine) BindGenericArgs(spec *GenericSpec, typeArgIDs []TypeID) {
	for i, param := range spec.Params {
		e.bind(param.NodeID, param.Name, false, typeArgIDs[i], e.ast.Node(param.NodeID).Span, -1)
	}
}

func (e *Engine) genericDeclSpec(decl genericDecl) (*GenericSpec, TypeStatus) {
	return e.BuildGenericSpec(decl.originTypeID, decl.typeParams)
}

func (e *Engine) PrepareGenericInstance(decl genericDecl, typeArgIDs []TypeID) (*genericInstance, TypeStatus) {
	spec, status := e.genericDeclSpec(decl)
	if status.Failed() {
		return nil, status
	}
	return &genericInstance{Decl: decl, Spec: spec, TypeArgIDs: typeArgIDs, Name: decl.cacheName(e, typeArgIDs)}, TypeOK
}

func (e *Engine) withGenericArgs(
	inst *genericInstance,
	fn func() TypeStatus,
) TypeStatus {
	if inst == nil {
		return TypeFailed
	}
	defer e.enterChildEnv()()
	e.BindGenericArgs(inst.Spec, inst.TypeArgIDs)
	return fn()
}

func (e *Engine) RewriteType(typeID TypeID, bindings map[TypeID]TypeID) (TypeID, TypeStatus) {
	if replacement, ok := bindings[typeID]; ok {
		return replacement, TypeOK
	}
	typ := e.env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case RefType:
		inner, status := e.RewriteType(kind.Type, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		if inner == kind.Type {
			return typeID, TypeOK
		}
		return e.env.buildRefType(0, inner, kind.Mut, base.Span{}), TypeOK
	case ArrayType:
		elem, status := e.RewriteType(kind.Elem, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		if elem == kind.Elem {
			return typeID, TypeOK
		}
		return e.env.buildArrayType(elem, kind.Len, 0, base.Span{}), TypeOK
	case SliceType:
		elem, status := e.RewriteType(kind.Elem, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		if elem == kind.Elem {
			return typeID, TypeOK
		}
		return e.env.buildSliceType(elem, kind.Mut, 0, base.Span{}), TypeOK
	case FunType:
		rewritten, changed, status := e.RewriteFunType(kind, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		if !changed {
			return typeID, TypeOK
		}
		return e.env.buildFunType(rewritten, 0, base.Span{}), TypeOK
	case StructType:
		return e.rewriteNamedType(typeID, kind.TypeArgs, bindings)
	case UnionType:
		return e.rewriteNamedType(typeID, kind.TypeArgs, bindings)
	case ShapeType:
		return e.rewriteNamedType(typeID, kind.TypeArgs, bindings)
	default:
		return typeID, TypeOK
	}
}

func (e *Engine) RewriteFunType(funType FunType, bindings map[TypeID]TypeID) (FunType, bool, TypeStatus) {
	result := FunType{
		Params:         make([]TypeID, len(funType.Params)),
		Return:         funType.Return,
		Macro:          funType.Macro,
		Sync:           funType.Sync,
		NoescapeParams: funType.NoescapeParams,
		NoescapeReturn: funType.NoescapeReturn,
	}
	changed := false
	for i, paramTypeID := range funType.Params {
		rewritten, status := e.RewriteType(paramTypeID, bindings)
		if status.Failed() {
			return FunType{}, false, status
		}
		result.Params[i] = rewritten
		changed = changed || rewritten != paramTypeID
	}
	rewrittenReturn, status := e.RewriteType(funType.Return, bindings)
	if status.Failed() {
		return FunType{}, false, status
	}
	result.Return = rewrittenReturn
	changed = changed || rewrittenReturn != funType.Return
	return result, changed, TypeOK
}

func (e *Engine) rewriteNamedType(typeID TypeID, typeArgs []TypeID, bindings map[TypeID]TypeID) (TypeID, TypeStatus) {
	if len(typeArgs) == 0 {
		return typeID, TypeOK
	}
	rewrittenArgs := make([]TypeID, len(typeArgs))
	changed := false
	for i, argTypeID := range typeArgs {
		rewritten, status := e.RewriteType(argTypeID, bindings)
		if status.Failed() {
			return InvalidTypeID, status
		}
		rewrittenArgs[i] = rewritten
		changed = changed || rewritten != argTypeID
	}
	if !changed {
		return typeID, TypeOK
	}
	originTypeID := typeID
	if origin, ok := e.env.GenericOrigin(typeID); ok {
		originTypeID = origin
	}
	return e.materializeNamedType(originTypeID, rewrittenArgs)
}

func (e *Engine) inferTypeBindings(
	patternTypeID, concreteTypeID TypeID,
	bindings map[TypeID]TypeID,
) bool {
	patternType := e.env.Type(patternTypeID)
	switch patternKind := patternType.Kind.(type) {
	case TypeParamType:
		if bound, ok := bindings[patternTypeID]; ok {
			return bound == concreteTypeID
		}
		bindings[patternTypeID] = concreteTypeID
		return true
	case RefType:
		concreteKind, ok := e.env.Type(concreteTypeID).Kind.(RefType)
		return ok && patternKind.Mut == concreteKind.Mut &&
			e.inferTypeBindings(patternKind.Type, concreteKind.Type, bindings)
	case ArrayType:
		concreteKind, ok := e.env.Type(concreteTypeID).Kind.(ArrayType)
		return ok && patternKind.Len == concreteKind.Len &&
			e.inferTypeBindings(patternKind.Elem, concreteKind.Elem, bindings)
	case SliceType:
		concreteKind, ok := e.env.Type(concreteTypeID).Kind.(SliceType)
		return ok && patternKind.Mut == concreteKind.Mut &&
			e.inferTypeBindings(patternKind.Elem, concreteKind.Elem, bindings)
	case FunType:
		concreteKind, ok := e.env.Type(concreteTypeID).Kind.(FunType)
		if !ok || len(patternKind.Params) != len(concreteKind.Params) || patternKind.Macro != concreteKind.Macro {
			return false
		}
		for i, patternParam := range patternKind.Params {
			if !e.inferTypeBindings(patternParam, concreteKind.Params[i], bindings) {
				return false
			}
		}
		return e.inferTypeBindings(patternKind.Return, concreteKind.Return, bindings)
	case StructType:
		concreteKind, ok := e.env.Type(concreteTypeID).Kind.(StructType)
		return ok && sameGenericFamily(typeIDOrigin(e.env, patternTypeID), typeIDOrigin(e.env, concreteTypeID)) &&
			e.inferTypeArgSlice(patternKind.TypeArgs, concreteKind.TypeArgs, bindings)
	case UnionType:
		concreteKind, ok := e.env.Type(concreteTypeID).Kind.(UnionType)
		return ok && sameGenericFamily(typeIDOrigin(e.env, patternTypeID), typeIDOrigin(e.env, concreteTypeID)) &&
			e.inferTypeArgSlice(patternKind.TypeArgs, concreteKind.TypeArgs, bindings)
	case ShapeType:
		concreteKind, ok := e.env.Type(concreteTypeID).Kind.(ShapeType)
		return ok && sameGenericFamily(typeIDOrigin(e.env, patternTypeID), typeIDOrigin(e.env, concreteTypeID)) &&
			e.inferTypeArgSlice(patternKind.TypeArgs, concreteKind.TypeArgs, bindings)
	default:
		return patternTypeID == concreteTypeID
	}
}

func (e *Engine) inferTypeArgSlice(pattern, concrete []TypeID, bindings map[TypeID]TypeID) bool {
	if len(pattern) != len(concrete) {
		return false
	}
	for i, patternTypeID := range pattern {
		if !e.inferTypeBindings(patternTypeID, concrete[i], bindings) {
			return false
		}
	}
	return true
}

func typeIDOrigin(env *TypeEnv, typeID TypeID) TypeID {
	if origin, ok := env.GenericOrigin(typeID); ok {
		return origin
	}
	return typeID
}

func sameGenericFamily(lhs, rhs TypeID) bool {
	return lhs == rhs
}

func (e *Engine) mangledName(baseName string, genericTypeID TypeID, typeArgIDs []TypeID) string {
	parts := make([]string, len(typeArgIDs))
	for i, id := range typeArgIDs {
		parts[i] = id.String()
	}
	return fmt.Sprintf("%s.%s.%s", baseName, genericTypeID, strings.Join(parts, "."))
}

func (e *Engine) QueryTypeArgs(typeArgNodeIDs []ast.NodeID) ([]TypeID, TypeStatus) {
	replacements := e.visibleTypeParamBindings()
	typeArgIDs := make([]TypeID, len(typeArgNodeIDs))
	for i, typeArgNodeID := range typeArgNodeIDs {
		typeArgID, status := e.Query(typeArgNodeID)
		if status.Failed() {
			return nil, TypeDepFailed
		}
		if len(replacements) > 0 {
			typeArgID, status = e.RewriteType(typeArgID, replacements)
			if status.Failed() {
				return nil, status
			}
		}
		typeArgIDs[i] = typeArgID
	}
	return typeArgIDs, TypeOK
}

func (e *Engine) visibleTypeParamBindings() map[TypeID]TypeID {
	replacements := map[TypeID]TypeID{}
	for env := e.env; env != nil; env = env.parent {
		for _, binding := range env.bindings {
			if binding.Decl == 0 {
				continue
			}
			if _, ok := e.ast.Node(binding.Decl).Kind.(ast.TypeParam); !ok {
				continue
			}
			paramTypeID, ok := e.env.reg.typeParamTypes[binding.Decl]
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

func (e *Engine) shapeOwner(originTypeID TypeID) (TypeID, ast.Shape, bool) {
	ownerTypeID := typeIDOrigin(e.env, originTypeID)
	if tpt, ok := e.env.Type(ownerTypeID).Kind.(TypeParamType); ok && tpt.Shape != nil {
		ownerTypeID = typeIDOrigin(e.env, *tpt.Shape)
	}
	shapeNode, ok := e.ast.Node(e.env.DeclNode(ownerTypeID)).Kind.(ast.Shape)
	if !ok {
		return InvalidTypeID, ast.Shape{}, false
	}
	return ownerTypeID, shapeNode, true
}

func (e *Engine) normalizeGenericTypeDecl(originTypeID TypeID, nodeID ast.NodeID, node any) (genericDecl, bool) {
	switch kind := node.(type) {
	case ast.Struct:
		return genericDecl{
			kind:          genericDeclStruct,
			originTypeID:  originTypeID,
			declNodeID:    nodeID,
			typeParams:    kind.TypeParams,
			name:          base.Cast[StructType](e.env.Type(originTypeID).Kind).Name,
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
			name:          base.Cast[UnionType](e.env.Type(originTypeID).Kind).Name,
			declName:      kind.Name.Name,
			memberNodeIDs: kind.Variants,
			paramNodeIDs:  nil,
			returnNodeID:  0,
			builtin:       false,
		}, true
	case ast.Shape:
		shapeType := base.Cast[ShapeType](e.env.Type(originTypeID).Kind)
		return genericDecl{
			kind:          genericDeclShape,
			originTypeID:  originTypeID,
			declNodeID:    nodeID,
			typeParams:    kind.TypeParams,
			name:          shapeType.Name,
			declName:      shapeType.DeclName,
			memberNodeIDs: kind.Fields,
			paramNodeIDs:  nil,
			returnNodeID:  0,
			builtin:       false,
		}, true
	default:
		return genericDecl{}, false
	}
}

func (e *Engine) normalizeGenericCallableDecl(
	originTypeID TypeID,
	nodeID ast.NodeID,
	name string,
	node any,
) (genericDecl, bool) {
	switch kind := node.(type) {
	case ast.Fun:
		if name == "" {
			name, _ = e.env.NamedFunRef(nodeID)
		}
		return genericDecl{
			kind:          genericDeclFun,
			originTypeID:  originTypeID,
			declNodeID:    nodeID,
			typeParams:    kind.TypeParams,
			name:          name,
			declName:      "",
			memberNodeIDs: nil,
			paramNodeIDs:  kind.Params,
			returnNodeID:  kind.ReturnType,
			builtin:       kind.Builtin,
		}, true
	case ast.FunDecl:
		ownerTypeID, shapeNode, ok := e.shapeOwner(originTypeID)
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

func (e *Engine) NormalizeGenericDecl(nodeID ast.NodeID, originTypeID TypeID, name string) (genericDecl, bool) {
	node := e.ast.Node(nodeID)
	if decl, ok := e.normalizeGenericTypeDecl(originTypeID, nodeID, node.Kind); ok {
		return decl, true
	}
	return e.normalizeGenericCallableDecl(originTypeID, nodeID, name, node.Kind)
}

func (d genericDecl) cacheName(e *Engine, typeArgIDs []TypeID) string {
	if d.kind == genericDeclShape && len(typeArgIDs) == 0 {
		return d.name
	}
	return e.mangledName(d.name, d.originTypeID, typeArgIDs)
}

func (d genericDecl) lookupTypeWork(e *Engine, name string) (TypeWork, bool) {
	switch d.kind {
	case genericDeclStruct:
		work, ok := e.structs[name]
		return work, ok
	case genericDeclUnion:
		work, ok := e.unions[name]
		return work, ok
	case genericDeclShape:
		work, ok := e.shapes[name]
		return work, ok
	case genericDeclFun, genericDeclShapeMethod:
		return TypeWork{}, false
	default:
		return TypeWork{}, false
	}
}

func (d genericDecl) storeTypeWork(e *Engine, name string, work TypeWork) {
	switch d.kind {
	case genericDeclStruct:
		e.structs[name] = work
	case genericDeclUnion:
		e.unions[name] = work
	case genericDeclShape:
		e.shapes[name] = work
	case genericDeclFun, genericDeclShapeMethod:
		return
	}
}

func (e *Engine) MaterializeNamedTypeRef(
	originTypeID TypeID,
	typeArgNodeIDs []ast.NodeID,
	span base.Span,
) (TypeID, TypeStatus) {
	decl, ok := e.NormalizeGenericDecl(e.env.DeclNode(originTypeID), originTypeID, "")
	if !ok {
		panic(base.Errorf("type %s is not generic-instantiable", originTypeID))
	}
	typeArgIDs, status := e.ResolveTypeArgs(decl.typeParams, typeArgNodeIDs, decl.declNodeID, span)
	if status.Failed() {
		return InvalidTypeID, status
	}
	return e.materializeNamedType(originTypeID, typeArgIDs)
}

func (e *Engine) materializeNamedType(originTypeID TypeID, typeArgIDs []TypeID) (TypeID, TypeStatus) { //nolint:funlen
	decl, ok := e.NormalizeGenericDecl(e.env.DeclNode(originTypeID), originTypeID, "")
	if !ok {
		panic(base.Errorf("type %s is not generic-instantiable", originTypeID))
	}
	inst, status := e.PrepareGenericInstance(decl, typeArgIDs)
	if status.Failed() {
		return InvalidTypeID, status
	}
	if cached, ok := decl.lookupTypeWork(e, inst.Name); ok {
		return cached.TypeID, TypeOK
	}
	var (
		typeID   TypeID
		resolved TypeKind
	)
	declNode := e.ast.Node(decl.declNodeID)
	status = e.withGenericArgs(inst, func() TypeStatus {
		decl.storeTypeWork(e, inst.Name, TypeWork{NodeID: decl.declNodeID, TypeID: InvalidTypeID, Env: e.env})
		switch kind := declNode.Kind.(type) {
		case ast.Struct:
			placeholder := StructType{Name: inst.Name, Fields: nil, TypeArgs: typeArgIDs}
			typeID = e.env.newType(placeholder, decl.declNodeID, declNode.Span, TypeInProgress)
			decl.storeTypeWork(e, inst.Name, TypeWork{NodeID: decl.declNodeID, TypeID: typeID, Env: e.env})
			fields := make([]StructField, len(kind.Fields))
			for i, fieldNodeID := range kind.Fields {
				fieldTypeID, fieldStatus := e.Query(fieldNodeID)
				if fieldStatus.Failed() {
					return TypeDepFailed
				}
				fieldNode := base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind)
				fields[i] = StructField{
					Name: fieldNode.Name.Name,
					Type: fieldTypeID,
					Pub:  fieldNode.Pub,
				}
			}
			resolved = StructType{Name: inst.Name, Fields: fields, TypeArgs: typeArgIDs}
		case ast.Union:
			placeholder := UnionType{Name: inst.Name, Variants: nil, TypeArgs: typeArgIDs}
			typeID = e.env.newType(placeholder, decl.declNodeID, declNode.Span, TypeInProgress)
			decl.storeTypeWork(e, inst.Name, TypeWork{NodeID: decl.declNodeID, TypeID: typeID, Env: e.env})
			variants := make([]TypeID, len(kind.Variants))
			for i, variantNodeID := range kind.Variants {
				variantTypeID, variantStatus := e.Query(variantNodeID)
				if variantStatus.Failed() {
					return TypeDepFailed
				}
				variants[i] = variantTypeID
			}
			resolved = UnionType{Name: inst.Name, Variants: variants, TypeArgs: typeArgIDs}
		case ast.Shape:
			fields := make([]StructField, len(kind.Fields))
			for i, fieldNodeID := range kind.Fields {
				fieldTypeID, fieldStatus := e.Query(fieldNodeID)
				if fieldStatus.Failed() {
					return TypeDepFailed
				}
				fieldNode := base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind)
				fields[i] = StructField{
					Name: fieldNode.Name.Name,
					Type: fieldTypeID,
					Pub:  fieldNode.Pub,
				}
			}
			resolved = ShapeType{Name: decl.name, DeclName: decl.declName, Fields: fields, TypeArgs: typeArgIDs}
			typeID = e.env.newType(resolved, decl.declNodeID, declNode.Span, TypeOK)
			decl.storeTypeWork(e, inst.Name, TypeWork{NodeID: decl.declNodeID, TypeID: typeID, Env: e.env})
		default:
			panic(base.Errorf("type %s is not generic-instantiable", originTypeID))
		}
		return TypeOK
	})
	if status.Failed() {
		return InvalidTypeID, status
	}
	e.env.reg.genericOrigin[typeID] = originTypeID
	if decl.kind != genericDeclShape {
		cached := e.env.reg.types[typeID]
		cached.Type.Kind = resolved
		cached.Status = TypeOK
	}
	return typeID, TypeOK
}

// inferTypeArgs attempts to determine type arguments from concrete argument types.
// Returns (inferred, true, status) on success or diagnosed failure,
// and (nil, false, _) when inference was inconclusive.
func (e *Engine) inferTypeArgs(
	typeParamNodeIDs []ast.NodeID, genericTypeIDs, concreteTypeIDs []TypeID, scopeNodeID ast.NodeID, span base.Span,
) (inferred []TypeID, ok bool, status TypeStatus) {
	spec, status := e.BuildGenericSpec(InvalidTypeID, typeParamNodeIDs)
	if status.Failed() {
		return nil, true, status
	}
	bindings := map[TypeID]TypeID{}
	for i, genericTypeID := range genericTypeIDs {
		if i >= len(concreteTypeIDs) {
			break
		}
		// Skip arguments that were deferred (function literals with inferred types).
		if concreteTypeIDs[i] == DeferredTypeID {
			continue
		}
		if !e.inferTypeBindings(genericTypeID, concreteTypeIDs[i], bindings) {
			break
		}
	}
	e.inferTypeArgsFromConstraints(spec, bindings, scopeNodeID, span)
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
	solved, solveStatus := e.SolveGenericArgs(spec, result, scopeNodeID, span)
	return solved, true, solveStatus
}

func (e *Engine) inferTypeArgsFromConstraints(
	spec *GenericSpec, bindings map[TypeID]TypeID, scopeNodeID ast.NodeID, span base.Span,
) {
	for _, param := range spec.Params {
		concreteTypeID, ok := bindings[param.TypeID]
		if !ok || param.Constraint == nil {
			continue
		}
		constraintTypeID, status := e.RewriteType(*param.Constraint, bindings)
		if status.Failed() {
			continue
		}
		shapeType := base.Cast[ShapeType](e.env.Type(constraintTypeID).Kind)
		if len(shapeType.TypeArgs) == 0 {
			continue
		}
		e.inferConstraintBindings(constraintTypeID, concreteTypeID, bindings, scopeNodeID, span)
	}
}

func (e *Engine) inferConstraintBindings(
	shapeTypeID TypeID,
	concreteTypeID TypeID,
	bindings map[TypeID]TypeID,
	scopeNodeID ast.NodeID,
	span base.Span,
) {
	shapeNodeID := e.env.DeclNode(typeIDOrigin(e.env, shapeTypeID))
	shapeNode := base.Cast[ast.Shape](e.ast.Node(shapeNodeID).Kind)
	for _, funDeclNodeID := range shapeNode.Funs {
		funDecl := base.Cast[ast.FunDecl](e.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		shapeFunBinding, ok := e.LookupShapeMethodBinding(shapeTypeID, methodName, scopeNodeID)
		if !ok {
			continue
		}
		shapeFunType, status := e.MethodSignature(
			shapeFunBinding,
			e.ShapeMethodContext(shapeTypeID, concreteTypeID),
			scopeNodeID,
			span,
		)
		if status.Failed() {
			continue
		}
		binding, ok := e.lookupMethodBinding(scopeNodeID, concreteTypeID, methodName)
		if !ok {
			continue
		}
		concreteFunType, status := e.MethodSignature(
			binding,
			e.BoundMethodContext(binding, concreteTypeID),
			scopeNodeID,
			span,
		)
		if status.Failed() {
			continue
		}
		for i, paramTypeID := range shapeFunType.Params {
			if i >= len(concreteFunType.Params) {
				break
			}
			e.inferTypeBindings(paramTypeID, concreteFunType.Params[i], bindings)
		}
		e.inferTypeBindings(shapeFunType.Return, concreteFunType.Return, bindings)
	}
}

func (e *Engine) InferTypeConstruction(
	lit ast.TypeConstruction, span base.Span, typeHint *TypeID,
) (TypeID, TypeStatus, bool) {
	ident, ok := e.ast.Node(lit.Target).Kind.(ast.Ident)
	if !ok || len(ident.TypeArgs) > 0 {
		return InvalidTypeID, TypeFailed, false
	}
	binding, ok := e.lookup(lit.Target, ident.Name, -1)
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	if typeHint != nil {
		if origin, ok := e.env.GenericOrigin(
			*typeHint,
		); ok && origin == binding.TypeID &&
			!e.env.containsTypeParam(*typeHint) {
			e.updateCachedType(e.ast.Node(lit.Target), *typeHint, TypeOK)
			return *typeHint, TypeOK, true
		}
	}
	switch kind := e.env.Type(binding.TypeID).Kind.(type) {
	case StructType:
		return e.inferNamedConstruction(binding, kind, lit, span)
	case UnionType:
		return e.inferNamedConstruction(binding, kind, lit, span)
	default:
		return InvalidTypeID, TypeFailed, false
	}
}

func (e *Engine) queryArgsForInference(argNodeIDs []ast.NodeID, patternTypeIDs []TypeID) ([]TypeID, TypeStatus) {
	argTypeIDs := make([]TypeID, len(argNodeIDs))
	for i, argNodeID := range argNodeIDs {
		// When the argument is a function literal with inferred types and the
		// expected parameter type contains unresolved type params, skip
		// type-checking it now. Querying it here would cache it with the
		// unresolved type params. Instead, resolveDeferredFunLitArgs will
		// resolve them after partial inference from the other arguments.
		if i < len(patternTypeIDs) && e.env.containsTypeParam(patternTypeIDs[i]) &&
			e.isFunLitWithInferredTypes(argNodeID) {
			argTypeIDs[i] = DeferredTypeID
			continue
		}
		var argTypeID TypeID
		var status TypeStatus
		if i < len(patternTypeIDs) {
			if _, isTypeParam := e.env.Type(patternTypeIDs[i]).Kind.(TypeParamType); !isTypeParam {
				argTypeID, status = e.queryWithHint(argNodeID, &patternTypeIDs[i])
			} else {
				argTypeID, status = e.Query(argNodeID)
			}
		} else {
			argTypeID, status = e.Query(argNodeID)
		}
		if status.Failed() {
			return nil, TypeDepFailed
		}
		argTypeIDs[i] = argTypeID
	}
	return argTypeIDs, TypeOK
}

// isFunLitWithInferredTypes reports whether nodeID is a function literal block
// (the Fun+Ident wrapper the parser emits) where at least one parameter type
// or the return type has been omitted.
func (e *Engine) isFunLitWithInferredTypes(nodeID ast.NodeID) bool {
	block, ok := e.ast.Node(nodeID).Kind.(ast.Block)
	if !ok || len(block.Exprs) != 2 {
		return false
	}
	fun, ok := e.ast.Node(block.Exprs[0]).Kind.(ast.Fun)
	if !ok {
		return false
	}
	return e.funLitNeedsInference(fun)
}

// resolveDeferredFunLitArgs performs partial type inference from the already-resolved
// arguments, then type-checks deferred function literal arguments using hints
// where type parameters have been replaced with their inferred concrete types.
func (e *Engine) resolveDeferredFunLitArgs(
	typeParamNodeIDs []ast.NodeID,
	genericTypeIDs []TypeID,
	argNodeIDs []ast.NodeID,
	argTypeIDs []TypeID,
	scopeNodeID ast.NodeID,
	span base.Span,
) ([]TypeID, TypeStatus) {
	// Build partial bindings from the already-resolved arguments.
	spec, status := e.BuildGenericSpec(InvalidTypeID, typeParamNodeIDs)
	if status.Failed() {
		return nil, status
	}
	bindings := map[TypeID]TypeID{}
	for i, genericTypeID := range genericTypeIDs {
		if i >= len(argTypeIDs) {
			break
		}
		if argTypeIDs[i] == DeferredTypeID {
			continue
		}
		e.inferTypeBindings(genericTypeID, argTypeIDs[i], bindings)
	}
	e.inferTypeArgsFromConstraints(spec, bindings, scopeNodeID, span)
	// Now resolve each deferred argument using the partial bindings.
	for i, argTypeID := range argTypeIDs {
		if argTypeID != DeferredTypeID {
			continue
		}
		if i >= len(genericTypeIDs) {
			continue
		}
		// Rewrite the generic pattern type, substituting known bindings.
		resolvedHint, status := e.RewriteType(genericTypeIDs[i], bindings)
		if status.Failed() {
			return nil, status
		}
		argTypeIDs[i], status = e.queryWithHint(argNodeIDs[i], &resolvedHint)
		if status.Failed() {
			return nil, TypeDepFailed
		}
	}
	return argTypeIDs, TypeOK
}

func (e *Engine) inferStructConstruction(
	targetKind TypeKind,
	lit ast.TypeConstruction,
	span base.Span,
	typeParamNodeIDs []ast.NodeID,
) (inferred []TypeID, ok bool, status TypeStatus) {
	structType := base.Cast[StructType](targetKind)
	if len(lit.Args) != len(structType.Fields) {
		e.diag(span, "argument count mismatch: expected %d, got %d", len(structType.Fields), len(lit.Args))
		return nil, true, TypeFailed
	}
	fieldTypeIDs := make([]TypeID, len(structType.Fields))
	for i, field := range structType.Fields {
		fieldTypeIDs[i] = field.Type
	}
	argTypeIDs, queryStatus := e.queryArgsForInference(lit.Args, fieldTypeIDs)
	if queryStatus.Failed() {
		return nil, true, TypeDepFailed
	}
	return e.inferTypeArgs(typeParamNodeIDs, fieldTypeIDs, argTypeIDs, lit.Target, span)
}

func (e *Engine) inferUnionConstruction(
	binding *Binding,
	targetKind TypeKind,
	lit ast.TypeConstruction,
	span base.Span,
) ([]TypeID, bool, TypeStatus) {
	if len(lit.Args) != 1 {
		e.diag(span, "union constructor takes exactly 1 argument, got %d", len(lit.Args))
		return nil, true, TypeFailed
	}
	argTypeID, status := e.Query(lit.Args[0])
	if status.Failed() {
		return nil, true, TypeDepFailed
	}
	unionType := base.Cast[UnionType](targetKind)
	bindings := map[TypeID]TypeID{}
	for _, variantTypeID := range unionType.Variants {
		e.inferTypeBindings(variantTypeID, argTypeID, bindings)
	}
	decl, _ := e.NormalizeGenericDecl(e.env.DeclNode(binding.TypeID), binding.TypeID, "")
	spec, status := e.BuildGenericSpec(decl.originTypeID, decl.typeParams)
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
	solved, solveStatus := e.SolveGenericArgs(spec, inferred, lit.Target, span)
	return solved, true, solveStatus
}

func (e *Engine) inferNamedConstruction(
	binding *Binding, targetKind TypeKind, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus, bool) {
	var inferred []TypeID
	var inferOK bool
	var status TypeStatus
	switch kind := e.ast.Node(binding.Decl).Kind.(type) {
	case ast.Struct:
		if len(kind.TypeParams) == 0 {
			return InvalidTypeID, TypeFailed, false
		}
		inferred, inferOK, status = e.inferStructConstruction(targetKind, lit, span, kind.TypeParams)
	case ast.Union:
		if len(kind.TypeParams) == 0 {
			return InvalidTypeID, TypeFailed, false
		}
		inferred, inferOK, status = e.inferUnionConstruction(binding, targetKind, lit, span)
	default:
		return InvalidTypeID, TypeFailed, false
	}
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	if !inferOK {
		return InvalidTypeID, TypeFailed, false
	}
	typeID, status := e.materializeNamedType(binding.TypeID, inferred)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	e.updateCachedType(e.ast.Node(lit.Target), typeID, TypeOK)
	return typeID, TypeOK, true
}

func (e *Engine) resolveCallBinding(call ast.Call) (*Binding, bool) {
	calleeNode := e.ast.Node(call.Callee)
	switch kind := calleeNode.Kind.(type) {
	case ast.Ident:
		binding, ok := e.lookup(call.Callee, kind.Name, -1)
		return binding, ok && binding.Decl != 0
	case ast.FieldAccess:
		// Check if the target is a module (module member call): module.fun(...)
		if ident, ok := e.ast.Node(kind.Target).Kind.(ast.Ident); ok {
			modBinding, ok := e.lookup(call.Callee, ident.Name, -1)
			if ok {
				if _, isMod := e.env.Type(modBinding.TypeID).Kind.(ModuleType); isMod {
					thisModuleNode, _ := e.moduleOf(call.Callee)
					importedModuleNodeID, ok := e.moduleResolution.Imports[thisModuleNode.ID][ident.Name]
					if !ok {
						return nil, false
					}
					mod := base.Cast[ast.Module](e.ast.Node(importedModuleNodeID).Kind)
					if len(mod.Decls) == 0 {
						return nil, false
					}
					binding, ok := e.env.Lookup(mod.Decls[0], kind.Field.Name, -1)
					return binding, ok
				}
			}
		}
		return e.resolveMethodCallBinding(call.Callee, kind)
	default:
		return nil, false
	}
}

func (e *Engine) inferFunCallBinding(call ast.Call) (*Binding, bool) {
	switch kind := e.ast.Node(call.Callee).Kind.(type) {
	case ast.Ident:
		if len(kind.TypeArgs) > 0 {
			return nil, false
		}
	case ast.FieldAccess:
		if len(kind.TypeArgs) > 0 {
			return nil, false
		}
	}
	return e.resolveCallBinding(call)
}

func (e *Engine) resolveMethodCallBinding(calleeNodeID ast.NodeID, fieldAccess ast.FieldAccess) (*Binding, bool) {
	targetTypeID, status := e.Query(fieldAccess.Target)
	if status.Failed() {
		return nil, false
	}
	return e.lookupMethodBinding(calleeNodeID, targetTypeID, fieldAccess.Field.Name)
}

func (e *Engine) lookupMethodBinding(scopeNodeID ast.NodeID, targetTypeID TypeID, methodName string) (*Binding, bool) {
	targetTyp := e.env.Type(targetTypeID)
	if refTyp, ok := targetTyp.Kind.(RefType); ok {
		targetTypeID = refTyp.Type
		targetTyp = e.env.Type(targetTypeID)
	}
	lookupType := targetTyp
	if originID, ok := e.env.GenericOrigin(targetTypeID); ok {
		lookupType = e.env.Type(originID)
	}
	lookupName, ok := e.env.methodFQN(lookupType, methodName)
	if !ok {
		return nil, false
	}
	binding, ok := e.lookup(scopeNodeID, lookupName, -1)
	if !ok {
		binding, ok = e.lookupInTypeModule(lookupType, lookupName)
	}
	if !ok || binding.Decl == 0 {
		return nil, false
	}
	return binding, true
}

//nolint:funlen
func (e *Engine) InferFunCall(call ast.Call, span base.Span) (TypeID, TypeStatus, bool) {
	binding, ok := e.inferFunCallBinding(call)
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	funNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Fun)
	if !ok || len(funNode.TypeParams) == 0 {
		return InvalidTypeID, TypeFailed, false
	}
	genericFunType := base.Cast[FunType](e.env.Type(binding.TypeID).Kind)
	allArgNodeIDs := call.Args
	fieldAccess, isFieldAccess := e.ast.Node(call.Callee).Kind.(ast.FieldAccess)
	if isFieldAccess {
		targetTypeID, status := e.Query(fieldAccess.Target)
		if status.Failed() {
			return InvalidTypeID, TypeFailed, false
		}
		_, isModule := e.env.Type(targetTypeID).Kind.(ModuleType)
		if !isModule && !e.isTypeReference(fieldAccess.Target) {
			allArgNodeIDs = append([]ast.NodeID{fieldAccess.Target}, call.Args...)
		}
	}
	if len(allArgNodeIDs) < len(genericFunType.Params) {
		for i := len(allArgNodeIDs); i < len(funNode.Params); i++ {
			param := base.Cast[ast.FunParam](e.ast.Node(funNode.Params[i]).Kind)
			if param.Default != nil {
				allArgNodeIDs = append(allArgNodeIDs, *param.Default)
			}
		}
	}
	argTypeIDs, status := e.queryArgsForInference(allArgNodeIDs, genericFunType.Params)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed, true
	}
	// If any arguments were deferred (function literals with inferred types),
	// do partial inference from the resolved arguments first, then resolve the
	// deferred arguments' hints using the partial bindings.
	if slices.Contains(argTypeIDs, DeferredTypeID) {
		argTypeIDs, status = e.resolveDeferredFunLitArgs(
			funNode.TypeParams, genericFunType.Params, allArgNodeIDs, argTypeIDs, call.Callee, span,
		)
		if status.Failed() {
			return InvalidTypeID, status, true
		}
	}
	inferred, inferOK, status := e.inferTypeArgs(
		funNode.TypeParams,
		genericFunType.Params,
		argTypeIDs,
		call.Callee,
		span,
	)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	if !inferOK {
		return InvalidTypeID, TypeFailed, false
	}
	typeID, mangledName, status := e.MaterializeFun(
		binding.Decl,
		binding.TypeID,
		call.Callee,
		span,
		inferred,
	)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	e.env.setNamedFunRef(call.Callee, mangledName)
	e.env.setNodeType(call.Callee, &cachedType{Type: e.env.Type(typeID), Status: TypeOK})
	return typeID, TypeOK, true
}

func (e *Engine) MaterializeFun(
	funNodeID ast.NodeID,
	genericTypeID TypeID,
	callSiteNodeID ast.NodeID,
	span base.Span,
	typeArgIDs []TypeID,
) (typeID TypeID, mangledName string, status TypeStatus) {
	decl, _ := e.NormalizeGenericDecl(funNodeID, genericTypeID, "")
	if decl.name == "" {
		return InvalidTypeID, "", TypeFailed
	}
	mangledName = decl.cacheName(e, typeArgIDs)
	if cached, ok := e.funs[mangledName]; ok {
		return cached.TypeID, mangledName, TypeOK
	}
	defer e.enterChildEnv()()
	var funTyp FunType
	inst, status := e.PrepareGenericInstance(decl, typeArgIDs)
	if status.Failed() {
		return InvalidTypeID, "", status
	}
	e.BindGenericArgs(inst.Spec, inst.TypeArgIDs)
	funTyp, status = e.RewriteCallable(decl.typeParams, typeArgIDs, decl.paramNodeIDs, decl.returnNodeID, nil)
	if status.Failed() {
		return InvalidTypeID, "", status
	}
	funTyp, _, status = e.RewriteFunType(funTyp, inst.Spec.Bindings(typeArgIDs))
	if status.Failed() {
		return InvalidTypeID, "", status
	}
	node := e.ast.Node(funNodeID)
	funTypeID := e.env.newType(funTyp, node.ID, node.Span, TypeOK)
	e.env.reg.genericOrigin[funTypeID] = genericTypeID
	if !decl.builtin {
		e.funs[mangledName] = FunWork{NodeID: funNodeID, TypeID: funTypeID, Name: mangledName, Env: e.env}
		prevScope := e.instantiationScope
		prevSkip := e.skipRegisterWork
		e.instantiationScope = &callSiteNodeID
		e.skipRegisterWork = e.hasUnresolvedTypeParams(typeArgIDs)
		funNode := base.Cast[ast.Fun](e.ast.Node(funNodeID).Kind)
		e.checkFunBody(funNodeID, funNode, funTypeID, funTyp)
		e.skipRegisterWork = prevSkip
		e.instantiationScope = prevScope
	}
	return funTypeID, mangledName, TypeOK
}

func (e *Engine) hasUnresolvedTypeParams(typeArgIDs []TypeID) bool {
	for _, id := range typeArgIDs {
		if _, ok := e.env.Type(id).Kind.(TypeParamType); ok {
			return true
		}
	}
	return false
}

func (e *Engine) RewriteCallable(
	typeParamNodeIDs []ast.NodeID,
	typeArgIDs []TypeID,
	paramNodeIDs []ast.NodeID,
	returnTypeNodeID ast.NodeID,
	replacements map[TypeID]TypeID,
) (FunType, TypeStatus) {
	bindings := map[TypeID]TypeID{}
	if len(typeParamNodeIDs) > 0 {
		spec, status := e.BuildGenericSpec(InvalidTypeID, typeParamNodeIDs)
		if status.Failed() {
			return FunType{}, status
		}
		maps.Copy(bindings, spec.Bindings(typeArgIDs))
	}
	maps.Copy(bindings, replacements)
	paramTypeIDs := make([]TypeID, len(paramNodeIDs))
	for i, paramNodeID := range paramNodeIDs {
		paramTypeID, status := e.Query(paramNodeID)
		if status.Failed() {
			return FunType{}, TypeDepFailed
		}
		if len(bindings) > 0 {
			paramTypeID, status = e.RewriteType(paramTypeID, bindings)
			if status.Failed() {
				return FunType{}, status
			}
			e.env.setNodeType(paramNodeID, &cachedType{Type: e.env.Type(paramTypeID), Status: TypeOK})
		}
		paramTypeIDs[i] = paramTypeID
	}
	retTypeID, status := e.Query(returnTypeNodeID)
	if status.Failed() {
		return FunType{}, TypeDepFailed
	}
	if len(bindings) > 0 {
		retTypeID, status = e.RewriteType(retTypeID, bindings)
		if status.Failed() {
			return FunType{}, status
		}
	}
	funType := FunType{
		Params:         paramTypeIDs,
		Return:         retTypeID,
		Macro:          false,
		Sync:           false,
		NoescapeParams: make([]bool, len(paramTypeIDs)),
		NoescapeReturn: false,
	}
	if len(bindings) == 0 {
		return funType, TypeOK
	}
	return funType, TypeOK
}
