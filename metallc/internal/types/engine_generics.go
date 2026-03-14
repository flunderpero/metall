package types

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

func (e *Engine) instantiateStruct(
	generic StructType,
	genericTypeID TypeID,
	typeArgNodeIDs []ast.NodeID,
	span base.Span,
) (TypeID, TypeStatus) {
	structNodeID := e.env.DeclNode(genericTypeID)
	structNode := base.Cast[ast.Struct](e.ast.Node(structNodeID).Kind)
	providedTypeIDs, status := e.queryTypeArgs(typeArgNodeIDs)
	if status.Failed() {
		return InvalidTypeID, status
	}
	argTypeIDs, status := e.applyTypeArgDefaults(structNode.TypeParams, providedTypeIDs, span)
	if status.Failed() {
		return InvalidTypeID, status
	}
	return e.instantiateStructWithTypeArgs(generic, genericTypeID, argTypeIDs)
}

func (e *Engine) instantiateStructWithTypeArgs(
	generic StructType, genericTypeID TypeID, argTypeIDs []TypeID,
) (TypeID, TypeStatus) {
	structNodeID := e.env.DeclNode(genericTypeID)
	structNode := base.Cast[ast.Struct](e.ast.Node(structNodeID).Kind)
	mangledName := e.mangledName(generic.Name, genericTypeID, argTypeIDs)
	if cached, ok := e.structs[mangledName]; ok {
		return cached.TypeID, TypeOK
	}
	defer e.enterChildEnv()()
	e.bindTypeParamsToArgs(structNode.TypeParams, argTypeIDs)
	node := e.ast.Node(structNodeID)
	placeholder := StructType{mangledName, []StructField{}, argTypeIDs}
	typeID := e.env.newType(placeholder, node.ID, node.Span, TypeInProgress)
	e.env.reg.genericOrigin[typeID] = genericTypeID
	e.structs[mangledName] = StructWork{NodeID: structNodeID, TypeID: typeID, Env: e.env}
	status, resolved := e.resolveStructFields(structNode, placeholder)
	if status.Failed() {
		return InvalidTypeID, status
	}
	cached := e.env.reg.types[typeID]
	cached.Type.Kind = resolved
	cached.Status = TypeOK
	return typeID, TypeOK
}

func (e *Engine) resolveStructFields(
	structNode ast.Struct, structType StructType,
) (TypeStatus, StructType) {
	fields := make([]StructField, len(structNode.Fields))
	for i, fieldNodeID := range structNode.Fields {
		fieldTypeID, status := e.Query(fieldNodeID)
		if status.Failed() {
			return TypeDepFailed, structType
		}
		fieldNode := base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind)
		fields[i] = StructField{fieldNode.Name.Name, fieldTypeID, fieldNode.Mut}
	}
	structType.Fields = fields
	return TypeOK, structType
}

func (e *Engine) instantiateUnion(
	generic UnionType,
	genericTypeID TypeID,
	typeArgNodeIDs []ast.NodeID,
	span base.Span,
) (TypeID, TypeStatus) {
	unionNodeID := e.env.DeclNode(genericTypeID)
	unionNode := base.Cast[ast.Union](e.ast.Node(unionNodeID).Kind)
	providedTypeIDs, status := e.queryTypeArgs(typeArgNodeIDs)
	if status.Failed() {
		return InvalidTypeID, status
	}
	argTypeIDs, status := e.applyTypeArgDefaults(unionNode.TypeParams, providedTypeIDs, span)
	if status.Failed() {
		return InvalidTypeID, status
	}
	return e.instantiateUnionWithTypeArgs(generic, genericTypeID, argTypeIDs)
}

func (e *Engine) instantiateUnionWithTypeArgs(
	generic UnionType, genericTypeID TypeID, argTypeIDs []TypeID,
) (TypeID, TypeStatus) {
	unionNodeID := e.env.DeclNode(genericTypeID)
	unionNode := base.Cast[ast.Union](e.ast.Node(unionNodeID).Kind)
	mangledName := e.mangledName(generic.Name, genericTypeID, argTypeIDs)
	if cached, ok := e.unions[mangledName]; ok {
		return cached.TypeID, TypeOK
	}
	defer e.enterChildEnv()()
	e.bindTypeParamsToArgs(unionNode.TypeParams, argTypeIDs)
	node := e.ast.Node(unionNodeID)
	placeholder := UnionType{mangledName, nil, argTypeIDs}
	typeID := e.env.newType(placeholder, node.ID, node.Span, TypeInProgress)
	e.env.reg.genericOrigin[typeID] = genericTypeID
	e.unions[mangledName] = UnionWork{NodeID: unionNodeID, TypeID: typeID, Env: e.env}
	resolveStatus, resolved := e.resolveUnionVariants(unionNode, placeholder)
	if resolveStatus.Failed() {
		return InvalidTypeID, resolveStatus
	}
	cached := e.env.reg.types[typeID]
	cached.Type.Kind = resolved
	cached.Status = TypeOK
	return typeID, TypeOK
}

func (e *Engine) resolveUnionVariants(unionNode ast.Union, unionType UnionType) (TypeStatus, UnionType) {
	variants := make([]TypeID, len(unionNode.Variants))
	for i, variantNodeID := range unionNode.Variants {
		variantTypeID, status := e.Query(variantNodeID)
		if status.Failed() {
			return TypeDepFailed, unionType
		}
		variants[i] = variantTypeID
	}
	unionType.Variants = variants
	return TypeOK, unionType
}

func (e *Engine) queryTypeArgs(typeArgNodeIDs []ast.NodeID) ([]TypeID, TypeStatus) {
	argTypeIDs := make([]TypeID, len(typeArgNodeIDs))
	for i, typeArgNodeID := range typeArgNodeIDs {
		argTypeID, status := e.Query(typeArgNodeID)
		if status.Failed() {
			return nil, TypeDepFailed
		}
		argTypeIDs[i] = argTypeID
	}
	return argTypeIDs, TypeOK
}

func (e *Engine) applyTypeArgDefaults(
	typeParamNodeIDs []ast.NodeID, argTypeIDs []TypeID, span base.Span,
) ([]TypeID, TypeStatus) {
	required := len(typeParamNodeIDs)
	for i, id := range typeParamNodeIDs {
		if base.Cast[ast.TypeParam](e.ast.Node(id).Kind).Default != nil {
			required = i
			break
		}
	}
	total := len(typeParamNodeIDs)
	provided := len(argTypeIDs)
	if provided < required || provided > total {
		if required == total {
			e.diag(span, "type argument count mismatch: expected %d, got %d", total, provided)
		} else {
			e.diag(span, "type argument count mismatch: expected %d to %d, got %d", required, total, provided)
		}
		return nil, TypeFailed
	}
	if provided == total {
		return argTypeIDs, TypeOK
	}
	filled := make([]TypeID, total)
	copy(filled, argTypeIDs)
	for i := provided; i < total; i++ {
		typeParamTypeID := e.env.reg.typeParamTypes[typeParamNodeIDs[i]]
		tpt := base.Cast[TypeParamType](e.env.Type(typeParamTypeID).Kind)
		filled[i] = *tpt.Default
	}
	return filled, TypeOK
}

func (e *Engine) mangledName(baseName string, genericTypeID TypeID, argTypeIDs []TypeID) string {
	parts := make([]string, len(argTypeIDs))
	for i, id := range argTypeIDs {
		parts[i] = id.String()
	}
	return fmt.Sprintf("%s.%s.%s", baseName, genericTypeID, strings.Join(parts, "."))
}

func (e *Engine) bindTypeParamsToArgs(typeParamNodeIDs []ast.NodeID, argTypeIDs []TypeID) {
	for i, typeParamNodeID := range typeParamNodeIDs {
		typeParamNode := base.Cast[ast.TypeParam](e.ast.Node(typeParamNodeID).Kind)
		e.bind(typeParamNodeID, typeParamNode.Name.Name, false, argTypeIDs[i], typeParamNode.Name.Span)
	}
}

func (e *Engine) collectTypeArgBindings(genericTypeID, concreteTypeID TypeID, bindings map[TypeID]TypeID) {
	genericTyp := e.env.Type(genericTypeID)
	switch kind := genericTyp.Kind.(type) {
	case TypeParamType:
		bindings[genericTypeID] = concreteTypeID
	case StructType:
		if concrete, ok := e.env.Type(concreteTypeID).Kind.(StructType); ok {
			for i, typeArg := range kind.TypeArgs {
				if i < len(concrete.TypeArgs) {
					e.collectTypeArgBindings(typeArg, concrete.TypeArgs[i], bindings)
				}
			}
		}
	case UnionType:
		if concrete, ok := e.env.Type(concreteTypeID).Kind.(UnionType); ok {
			for i, typeArg := range kind.TypeArgs {
				if i < len(concrete.TypeArgs) {
					e.collectTypeArgBindings(typeArg, concrete.TypeArgs[i], bindings)
				}
			}
		}
	case RefType:
		if concrete, ok := e.env.Type(concreteTypeID).Kind.(RefType); ok {
			e.collectTypeArgBindings(kind.Type, concrete.Type, bindings)
		}
	case ArrayType:
		if concrete, ok := e.env.Type(concreteTypeID).Kind.(ArrayType); ok {
			e.collectTypeArgBindings(kind.Elem, concrete.Elem, bindings)
		}
	case SliceType:
		if concrete, ok := e.env.Type(concreteTypeID).Kind.(SliceType); ok {
			e.collectTypeArgBindings(kind.Elem, concrete.Elem, bindings)
		}
	case FunType:
		if concrete, ok := e.env.Type(concreteTypeID).Kind.(FunType); ok {
			for i, param := range kind.Params {
				if i < len(concrete.Params) {
					e.collectTypeArgBindings(param, concrete.Params[i], bindings)
				}
			}
			e.collectTypeArgBindings(kind.Return, concrete.Return, bindings)
		}
	}
}

func (e *Engine) inferTypeArgs(
	typeParamNodeIDs []ast.NodeID, genericTypeIDs, concreteTypeIDs []TypeID, span base.Span,
) ([]TypeID, TypeStatus) {
	bindings := map[TypeID]TypeID{}
	for i, genericTypeID := range genericTypeIDs {
		if i < len(concreteTypeIDs) {
			e.collectTypeArgBindings(genericTypeID, concreteTypeIDs[i], bindings)
		}
	}
	inferred := make([]TypeID, 0, len(typeParamNodeIDs))
	for _, nodeID := range typeParamNodeIDs {
		typeParamTypeID := e.env.reg.typeParamTypes[nodeID]
		if concreteID, ok := bindings[typeParamTypeID]; ok {
			inferred = append(inferred, concreteID)
		} else {
			break
		}
	}
	return e.applyTypeArgDefaults(typeParamNodeIDs, inferred, span)
}

func (e *Engine) inferTypeConstruction(
	lit ast.TypeConstruction, span base.Span, typeHint *TypeID,
) (TypeID, TypeStatus, bool) {
	ident, ok := e.ast.Node(lit.Target).Kind.(ast.Ident)
	if !ok || len(ident.TypeArgs) > 0 {
		return InvalidTypeID, TypeFailed, false
	}
	binding, ok := e.lookup(lit.Target, ident.Name)
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	if typeHint != nil {
		if origin, ok := e.env.GenericOrigin(*typeHint); ok && origin == binding.TypeID {
			if !e.env.containsTypeParam(*typeHint) {
				e.updateCachedType(e.ast.Node(lit.Target), *typeHint, TypeOK)
				return *typeHint, TypeOK, true
			}
		}
	}
	switch kind := e.env.Type(binding.TypeID).Kind.(type) {
	case StructType:
		return e.inferStructConstruction(binding, kind, lit, span)
	case UnionType:
		return e.inferUnionConstruction(binding, kind, lit, span)
	default:
		return InvalidTypeID, TypeFailed, false
	}
}

func (e *Engine) queryArgsForInference(
	argNodeIDs []ast.NodeID,
	genericParamTypeIDs []TypeID,
) ([]TypeID, TypeStatus) {
	argTypeIDs := make([]TypeID, len(argNodeIDs))
	for i, argNodeID := range argNodeIDs {
		var argTypeID TypeID
		var status TypeStatus
		if i < len(genericParamTypeIDs) {
			if _, isTypeParam := e.env.Type(genericParamTypeIDs[i]).Kind.(TypeParamType); !isTypeParam {
				argTypeID, status = e.queryWithHint(argNodeID, &genericParamTypeIDs[i])
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

func (e *Engine) inferStructConstruction(
	binding *Binding, structType StructType, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus, bool) {
	structNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Struct)
	if !ok || len(structNode.TypeParams) == 0 {
		return InvalidTypeID, TypeFailed, false
	}
	fieldTypeIDs := make([]TypeID, len(structType.Fields))
	for i, field := range structType.Fields {
		fieldTypeIDs[i] = field.Type
	}
	argTypeIDs, status := e.queryArgsForInference(lit.Args, fieldTypeIDs)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed, true
	}
	inferred, status := e.inferTypeArgs(structNode.TypeParams, fieldTypeIDs, argTypeIDs, span)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	typeID, status := e.instantiateStructWithTypeArgs(structType, binding.TypeID, inferred)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	e.updateCachedType(e.ast.Node(lit.Target), typeID, TypeOK)
	return typeID, TypeOK, true
}

func (e *Engine) inferUnionConstruction(
	binding *Binding, unionType UnionType, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus, bool) {
	unionNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Union)
	if !ok || len(unionNode.TypeParams) == 0 {
		return InvalidTypeID, TypeFailed, false
	}
	if len(lit.Args) != 1 {
		return InvalidTypeID, TypeFailed, false
	}
	argTypeID, status := e.Query(lit.Args[0])
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed, true
	}
	bindings := map[TypeID]TypeID{}
	for _, variant := range unionType.Variants {
		e.collectTypeArgBindings(variant, argTypeID, bindings)
	}
	inferred := make([]TypeID, 0, len(unionNode.TypeParams))
	for _, nodeID := range unionNode.TypeParams {
		typeParamTypeID := e.env.reg.typeParamTypes[nodeID]
		if concreteID, ok := bindings[typeParamTypeID]; ok {
			inferred = append(inferred, concreteID)
		} else {
			break
		}
	}
	filled, status := e.applyTypeArgDefaults(unionNode.TypeParams, inferred, span)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	typeID, status := e.instantiateUnionWithTypeArgs(unionType, binding.TypeID, filled)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	e.updateCachedType(e.ast.Node(lit.Target), typeID, TypeOK)
	return typeID, TypeOK, true
}

func (e *Engine) inferFunCallBinding(call ast.Call) (*Binding, bool) {
	calleeNode := e.ast.Node(call.Callee)
	switch kind := calleeNode.Kind.(type) {
	case ast.Ident:
		if len(kind.TypeArgs) > 0 {
			return nil, false
		}
		binding, ok := e.lookup(call.Callee, kind.Name)
		if !ok || binding.Decl == 0 {
			return nil, false
		}
		return binding, true
	case ast.Path:
		if len(kind.TypeArgs) > 0 || len(kind.Segments) != 2 {
			return nil, false
		}
		moduleName := kind.Segments[0]
		modBinding, ok := e.lookup(call.Callee, moduleName)
		if !ok {
			return nil, false
		}
		if _, ok := e.env.Type(modBinding.TypeID).Kind.(ModuleType); !ok {
			return nil, false
		}
		thisModuleNode, _ := e.moduleOf(call.Callee)
		importedModuleNodeID, ok := e.moduleResolution.Imports[thisModuleNode.ID][moduleName]
		if !ok {
			return nil, false
		}
		mod := base.Cast[ast.Module](e.ast.Node(importedModuleNodeID).Kind)
		if len(mod.Decls) == 0 {
			return nil, false
		}
		binding, ok := e.env.Lookup(mod.Decls[0], kind.Segments[1])
		if !ok {
			return nil, false
		}
		return binding, true
	case ast.FieldAccess:
		return e.inferMethodCallBinding(call.Callee, kind)
	default:
		return nil, false
	}
}

func (e *Engine) inferMethodCallBinding(
	calleeNodeID ast.NodeID, fieldAccess ast.FieldAccess,
) (*Binding, bool) {
	if len(fieldAccess.TypeArgs) > 0 {
		return nil, false
	}
	targetTypeID, status := e.Query(fieldAccess.Target)
	if status.Failed() {
		return nil, false
	}
	targetTyp := e.env.Type(targetTypeID)
	if refTyp, ok := targetTyp.Kind.(RefType); ok {
		targetTyp = e.env.Type(refTyp.Type)
	}
	switch targetTyp.Kind.(type) {
	case StructType, UnionType, IntType, BoolType, AllocatorType:
	case TypeParamType:
		if base.Cast[TypeParamType](targetTyp.Kind).Shape == nil {
			return nil, false
		}
	default:
		return nil, false
	}
	methodName := fieldAccess.Field.Name
	lookupName := e.env.typeName(targetTyp) + "." + methodName
	binding, ok := e.lookup(calleeNodeID, lookupName)
	if !ok {
		binding, ok = e.lookupInTypeModule(targetTyp, lookupName)
	}
	if !ok {
		if structOrUnion, isStructOrUnion := IsStructOrUnion(
			targetTyp.Kind,
		); isStructOrUnion &&
			len(structOrUnion.TypeArgs) > 0 {
			if originID, hasOrigin := e.env.GenericOrigin(targetTypeID); hasOrigin {
				originName := e.env.typeName(e.env.Type(originID))
				lookupName = originName + "." + methodName
				binding, ok = e.lookup(calleeNodeID, lookupName)
				if !ok {
					binding, ok = e.lookupInTypeModule(targetTyp, lookupName)
				}
			}
		}
	}
	if !ok || binding.Decl == 0 {
		return nil, false
	}
	return binding, true
}

func (e *Engine) inferFunCall(call ast.Call, span base.Span) (TypeID, TypeStatus, bool) {
	binding, ok := e.inferFunCallBinding(call)
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	funNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Fun)
	if !ok || len(funNode.TypeParams) == 0 {
		return InvalidTypeID, TypeFailed, false
	}
	genericFunType := base.Cast[FunType](e.env.Type(binding.TypeID).Kind)
	// For method calls, the receiver is an implicit first argument.
	allArgNodeIDs := call.Args
	fieldAccess, isFieldAccess := e.ast.Node(call.Callee).Kind.(ast.FieldAccess)
	if isFieldAccess {
		allArgNodeIDs = append([]ast.NodeID{fieldAccess.Target}, call.Args...)
	}
	argTypeIDs, status := e.queryArgsForInference(allArgNodeIDs, genericFunType.Params)
	if status.Failed() {
		if isFieldAccess {
			return InvalidTypeID, TypeFailed, false
		}
		return InvalidTypeID, TypeDepFailed, true
	}
	// For method calls, inference is best-effort — if it fails, let the normal
	// resolveMethod path handle error reporting with proper spans.
	diagCount := len(e.diagnostics)
	inferred, status := e.inferTypeArgs(funNode.TypeParams, genericFunType.Params, argTypeIDs, span)
	if status.Failed() {
		if isFieldAccess {
			e.diagnostics = e.diagnostics[:diagCount]
			return InvalidTypeID, TypeFailed, false
		}
		return InvalidTypeID, status, true
	}
	typeID, mangledName, status := e.instantiateFunWithTypeArgs(binding.TypeID, call.Callee, span, inferred)
	if status.Failed() {
		if isFieldAccess {
			e.diagnostics = e.diagnostics[:diagCount]
			return InvalidTypeID, TypeFailed, false
		}
		return InvalidTypeID, status, true
	}
	e.env.setNamedFunRef(call.Callee, mangledName)
	e.env.setNodeType(call.Callee, &cachedType{Type: e.env.Type(typeID), Status: TypeOK})
	return typeID, TypeOK, true
}

func (e *Engine) instantiateFun(
	genericTypeID TypeID,
	argTypeIDs []TypeID,
	callSiteNodeID ast.NodeID,
	span base.Span,
) (typeID TypeID, mangledName string, status TypeStatus) {
	funNodeID := e.env.DeclNode(genericTypeID)
	funNode := base.Cast[ast.Fun](e.ast.Node(funNodeID).Kind)
	argTypeIDs, status = e.applyTypeArgDefaults(funNode.TypeParams, argTypeIDs, span)
	if status.Failed() {
		return InvalidTypeID, "", status
	}
	return e.instantiateFunWithTypeArgs(genericTypeID, callSiteNodeID, span, argTypeIDs)
}

func (e *Engine) instantiateFunWithTypeArgs(
	genericTypeID TypeID,
	callSiteNodeID ast.NodeID,
	span base.Span,
	argTypeIDs []TypeID,
) (typeID TypeID, mangledName string, status TypeStatus) {
	funNodeID := e.env.DeclNode(genericTypeID)
	funNode := base.Cast[ast.Fun](e.ast.Node(funNodeID).Kind)
	genericName, ok := e.env.NamedFunRef(funNodeID)
	if !ok {
		return InvalidTypeID, "", TypeFailed
	}
	mangledName = e.mangledName(genericName, genericTypeID, argTypeIDs)
	if cached, ok := e.funs[mangledName]; ok {
		return cached.TypeID, mangledName, TypeOK
	}
	defer e.enterChildEnv()()
	for i, typeParamNodeID := range funNode.TypeParams {
		typeParamNode := base.Cast[ast.TypeParam](e.ast.Node(typeParamNodeID).Kind)
		e.bind(typeParamNodeID, typeParamNode.Name.Name, false, argTypeIDs[i], typeParamNode.Name.Span)
		if typeParamNode.Constraint != nil {
			constraintTypeID, _ := e.Query(*typeParamNode.Constraint)
			if !e.satisfiesShape(argTypeIDs[i], constraintTypeID, callSiteNodeID, span) {
				return InvalidTypeID, "", TypeFailed
			}
		}
	}
	paramTypeIDs := make([]TypeID, len(funNode.Params))
	for i, paramNodeID := range funNode.Params {
		paramTypeID, qStatus := e.Query(paramNodeID)
		if qStatus.Failed() {
			return InvalidTypeID, "", TypeDepFailed
		}
		paramTypeIDs[i] = paramTypeID
	}
	retTypeID, qStatus := e.Query(funNode.ReturnType)
	if qStatus.Failed() {
		return InvalidTypeID, "", TypeDepFailed
	}
	funTyp := FunType{paramTypeIDs, retTypeID}
	node := e.ast.Node(funNodeID)
	funTypeID := e.env.newType(funTyp, node.ID, node.Span, TypeOK)
	e.env.reg.genericOrigin[funTypeID] = genericTypeID
	if !funNode.Extern {
		e.funs[mangledName] = FunWork{NodeID: funNodeID, TypeID: funTypeID, Name: mangledName, Env: e.env}
		e.checkFunBody(funNode, funTypeID, funTyp)
	}
	return funTypeID, mangledName, TypeOK
}

func (e *Engine) satisfiesShape( //nolint:funlen
	concreteTypeID TypeID,
	shapeTypeID TypeID,
	scopeNodeID ast.NodeID,
	span base.Span,
) bool {
	shapeType := base.Cast[ShapeType](e.env.Type(shapeTypeID).Kind)
	concreteTyp := e.env.Type(concreteTypeID)
	// A type parameter constrained by the same shape trivially satisfies it.
	if tpt, ok := concreteTyp.Kind.(TypeParamType); ok && tpt.Shape != nil && *tpt.Shape == shapeTypeID {
		return true
	}
	concreteDisplay := e.env.TypeDisplay(concreteTypeID)
	// For method lookup we need the scope-resolvable name. For monomorphized
	// generics (e.g. Pair<Str, Int>) methods are defined on the generic type
	// (Pair), so we look up via the origin type's name.
	methodLookupName := e.env.typeName(concreteTyp)
	if originID, ok := e.env.GenericOrigin(concreteTypeID); ok {
		methodLookupName = e.env.typeName(e.env.Type(originID))
	}
	// Field requirements need a StructType.
	if len(shapeType.Fields) > 0 {
		structType, isStruct := concreteTyp.Kind.(StructType)
		if !isStruct {
			e.diag(span, "type %s does not satisfy shape %s: not a struct",
				concreteDisplay, shapeType.DeclName)
			return false
		}
		for _, reqField := range shapeType.Fields {
			found := false
			for _, field := range structType.Fields {
				if field.Name == reqField.Name {
					found = true
					if field.Type != reqField.Type {
						e.diag(span, "type %s does not satisfy shape %s: field %s has type %s, expected %s",
							concreteDisplay, shapeType.DeclName, field.Name,
							e.env.TypeDisplay(field.Type), e.env.TypeDisplay(reqField.Type))
						return false
					}
					if reqField.Mut && !field.Mut {
						e.diag(span, "type %s does not satisfy shape %s: field %s must be mut",
							concreteDisplay, shapeType.DeclName, field.Name)
						return false
					}
					break
				}
			}
			if !found {
				e.diag(span, "type %s does not satisfy shape %s: missing field %s",
					concreteDisplay, shapeType.DeclName, reqField.Name)
				return false
			}
		}
	}
	// Method requirements — any type can have methods.
	shapeNodeID := e.env.DeclNode(shapeTypeID)
	shapeNode := base.Cast[ast.Shape](e.ast.Node(shapeNodeID).Kind)
	// For monomorphized generic types, the concrete type args need to be
	// applied to generic methods defined on the type.
	var concreteTypeArgs []TypeID
	if typ, ok := IsStructOrUnion(concreteTyp.Kind); ok {
		concreteTypeArgs = typ.TypeArgs
	}
	for _, funDeclNodeID := range shapeNode.Funs {
		funDecl := base.Cast[ast.FunDecl](e.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		binding, ok := e.lookup(scopeNodeID, methodLookupName+"."+methodName)
		if !ok {
			e.diag(span, "type %s does not satisfy shape %s: missing method %s",
				concreteDisplay, shapeType.DeclName, methodName)
			return false
		}
		shapeFunBinding, _ := e.lookup(scopeNodeID, shapeType.DeclName+"."+methodName)
		shapeFunType := base.Cast[FunType](e.env.Type(shapeFunBinding.TypeID).Kind)
		expectedFunType := e.env.substituteFunType(shapeFunType, shapeTypeID, concreteTypeID)
		// When the method is a generic function on a generic type, instantiate
		// it with the type's type args to get the concrete signature.
		methodFunNode, isFun := e.ast.Node(binding.Decl).Kind.(ast.Fun)
		if isFun && len(methodFunNode.TypeParams) > 0 && len(concreteTypeArgs) > 0 {
			methodTypeID, _, status := e.instantiateFun(binding.TypeID, concreteTypeArgs, scopeNodeID, span)
			if status.Failed() {
				return false
			}
			concreteFunType := base.Cast[FunType](e.env.Type(methodTypeID).Kind)
			if !expectedFunType.Equal(concreteFunType) {
				e.diag(span,
					"type %s does not satisfy shape %s: method %s has signature %s, expected %s",
					concreteDisplay, shapeType.DeclName, methodName,
					e.env.TypeDisplay(methodTypeID), e.env.TypeDisplay(shapeFunBinding.TypeID))
				return false
			}
		} else {
			concreteFunType := base.Cast[FunType](e.env.Type(binding.TypeID).Kind)
			if !expectedFunType.Equal(concreteFunType) {
				e.diag(span,
					"type %s does not satisfy shape %s: method %s has signature %s, expected %s",
					concreteDisplay, shapeType.DeclName, methodName,
					e.env.TypeDisplay(binding.TypeID), e.env.TypeDisplay(shapeFunBinding.TypeID))
				return false
			}
		}
	}
	return true
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
		var shapeID *TypeID
		if typeParamNode.Constraint != nil {
			constraintTypeID, status := e.Query(*typeParamNode.Constraint)
			if status.Failed() {
				return TypeDepFailed
			}
			if _, ok := e.env.Type(constraintTypeID).Kind.(ShapeType); !ok {
				e.diag(e.ast.Node(*typeParamNode.Constraint).Span, "constraint must be a shape")
				return TypeFailed
			}
			shapeID = &constraintTypeID
		}
		var defaultID *TypeID
		if typeParamNode.Default != nil {
			defaultTypeID, status := e.Query(*typeParamNode.Default)
			if status.Failed() {
				return TypeDepFailed
			}
			if shapeID != nil {
				span := e.ast.Node(*typeParamNode.Default).Span
				if !e.satisfiesShape(defaultTypeID, *shapeID, typeParamNodeID, span) {
					return TypeFailed
				}
			}
			defaultID = &defaultTypeID
		}
		typeParamID := e.env.newType(
			TypeParamType{Shape: shapeID, Default: defaultID},
			typeParamNodeID, e.ast.Node(typeParamNodeID).Span, TypeOK,
		)
		e.env.reg.typeParamTypes[typeParamNodeID] = typeParamID
		e.bind(typeParamNodeID, typeParamNode.Name.Name, false, typeParamID, typeParamNode.Name.Span)
	}
	return TypeOK
}

func (e *Engine) checkShapeCreateAndBind(node *ast.Node, shapeNode ast.Shape) (TypeID, TypeStatus) {
	name := e.namespacedName(node.ID, shapeNode.Name.Name)
	typeID := e.env.newType(
		ShapeType{Name: name, DeclName: shapeNode.Name.Name, Fields: nil}, node.ID, node.Span, TypeInProgress,
	)
	if !e.bind(node.ID, shapeNode.Name.Name, false, typeID, shapeNode.Name.Span) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

func (e *Engine) checkShapeCompleteType(
	node *ast.Node,
	shapeNode ast.Shape,
	shapeType ShapeType,
) (TypeStatus, ShapeType) {
	fields := make([]StructField, len(shapeNode.Fields))
	for i, fieldNodeID := range shapeNode.Fields {
		fieldTypeID, status := e.Query(fieldNodeID)
		if status.Failed() {
			return TypeDepFailed, shapeType
		}
		fieldNode := base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind)
		fields[i] = StructField{fieldNode.Name.Name, fieldTypeID, fieldNode.Mut}
	}
	shapeType.Fields = fields
	parentScope := e.scopeGraph.NodeScope(node.ID)
	for _, funDeclNodeID := range shapeNode.Funs {
		funDecl := base.Cast[ast.FunDecl](e.ast.Node(funDeclNodeID).Kind)
		funTypeID, status := e.checkShapeFunDecl(funDecl)
		if status.Failed() {
			return status, shapeType
		}
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		bindName := shapeType.DeclName + "." + methodName
		if !e.env.bindInScope(parentScope, funDeclNodeID, bindName, funTypeID) {
			e.diag(funDecl.Name.Span, "symbol already defined: %s", bindName)
		}
	}
	return TypeOK, shapeType
}

func (e *Engine) checkShapeFunDecl(funDecl ast.FunDecl) (TypeID, TypeStatus) {
	paramTypeIDs := make([]TypeID, len(funDecl.Params))
	for i, paramNodeID := range funDecl.Params {
		paramTypeID, status := e.Query(paramNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		paramTypeIDs[i] = paramTypeID
	}
	retTypeID, status := e.Query(funDecl.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	funType := FunType{Params: paramTypeIDs, Return: retTypeID}
	return e.env.newType(funType, 0, base.Span{}, TypeOK), TypeOK
}

func (e *Engine) checkShapeFieldAccess(
	targetTyp *Type, fieldName string,
) (TypeID, TypeStatus, bool) {
	typeParamType, ok := targetTyp.Kind.(TypeParamType)
	if !ok || typeParamType.Shape == nil {
		return InvalidTypeID, TypeFailed, false
	}
	shapeType := base.Cast[ShapeType](e.env.Type(*typeParamType.Shape).Kind)
	for _, field := range shapeType.Fields {
		if field.Name == fieldName {
			return field.Type, TypeOK, true
		}
	}
	return InvalidTypeID, TypeFailed, false
}

func (e *Engine) resolveShapeMethod(
	nodeID ast.NodeID, binding *Binding, targetTyp *Type,
) (TypeID, TypeStatus, bool) {
	if _, isFunDecl := e.ast.Node(binding.Decl).Kind.(ast.FunDecl); !isFunDecl {
		return InvalidTypeID, TypeFailed, false
	}
	e.env.setNamedFunRef(nodeID, binding.Name)
	tpt := base.Cast[TypeParamType](targetTyp.Kind)
	shapeFunType := base.Cast[FunType](e.env.Type(binding.TypeID).Kind)
	substFunType := e.env.substituteFunType(shapeFunType, *tpt.Shape, targetTyp.ID)
	funTypeID := e.env.newType(substFunType, 0, base.Span{}, TypeOK)
	return funTypeID, TypeOK, true
}

func (e *Engine) resolveGenericMethod(
	nodeID ast.NodeID, fieldAccess ast.FieldAccess, targetTyp *Type, binding *Binding,
) (TypeID, TypeStatus, bool) {
	var argTypeIDs []TypeID
	if typ, ok := IsStructOrUnion(targetTyp.Kind); ok {
		argTypeIDs = append(argTypeIDs, typ.TypeArgs...)
	}
	extraArgs, status := e.queryTypeArgs(fieldAccess.TypeArgs)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	argTypeIDs = append(argTypeIDs, extraArgs...)
	typeID, mangledName, status := e.instantiateFun(binding.TypeID, argTypeIDs, nodeID, fieldAccess.Field.Span)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	e.env.setNamedFunRef(nodeID, mangledName)
	return typeID, TypeOK, true
}
