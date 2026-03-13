package types

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type CheckFunBodyFunc func(funNode ast.Fun, funTypeID TypeID, funType FunType)

type GenericsEngine struct {
	*EngineCore
	query         QueryFunc
	queryWithHint QueryWithHintFunc
	checkFunBody  CheckFunBodyFunc
}

func NewGenericsEngine(
	c *EngineCore, query QueryFunc, queryWithHint QueryWithHintFunc, checkFunBody CheckFunBodyFunc,
) *GenericsEngine {
	return &GenericsEngine{c, query, queryWithHint, checkFunBody}
}

func (g *GenericsEngine) instantiateStruct(
	generic StructType,
	genericTypeID TypeID,
	typeArgNodeIDs []ast.NodeID,
	span base.Span,
) (TypeID, TypeStatus) {
	structNodeID := g.env.DeclNode(genericTypeID)
	structNode := base.Cast[ast.Struct](g.ast.Node(structNodeID).Kind)
	providedTypeIDs, status := g.queryTypeArgs(typeArgNodeIDs)
	if status.Failed() {
		return InvalidTypeID, status
	}
	argTypeIDs, status := g.applyTypeArgDefaults(structNode.TypeParams, providedTypeIDs, span)
	if status.Failed() {
		return InvalidTypeID, status
	}
	return g.instantiateStructWithTypeArgs(generic, genericTypeID, argTypeIDs)
}

func (g *GenericsEngine) instantiateStructWithTypeArgs(
	generic StructType, genericTypeID TypeID, argTypeIDs []TypeID,
) (TypeID, TypeStatus) {
	structNodeID := g.env.DeclNode(genericTypeID)
	structNode := base.Cast[ast.Struct](g.ast.Node(structNodeID).Kind)
	mangledName := g.mangledName(generic.Name, genericTypeID, argTypeIDs)
	if cached, ok := g.structs[mangledName]; ok {
		return cached.TypeID, TypeOK
	}
	defer g.enterChildEnv()()
	g.bindTypeParamsToArgs(structNode.TypeParams, argTypeIDs)
	node := g.ast.Node(structNodeID)
	placeholder := StructType{mangledName, []StructField{}, argTypeIDs}
	typeID := g.env.newType(placeholder, node.ID, node.Span, TypeInProgress)
	g.env.reg.genericOrigin[typeID] = genericTypeID
	g.structs[mangledName] = StructWork{NodeID: structNodeID, TypeID: typeID, Env: g.env}
	status, resolved := g.resolveStructFields(structNode, placeholder)
	if status.Failed() {
		return InvalidTypeID, status
	}
	cached := g.env.reg.types[typeID]
	cached.Type.Kind = resolved
	cached.Status = TypeOK
	return typeID, TypeOK
}

func (g *GenericsEngine) resolveStructFields(
	structNode ast.Struct, structType StructType,
) (TypeStatus, StructType) {
	fields := make([]StructField, len(structNode.Fields))
	for i, fieldNodeID := range structNode.Fields {
		fieldTypeID, status := g.query(fieldNodeID)
		if status.Failed() {
			return TypeDepFailed, structType
		}
		fieldNode := base.Cast[ast.StructField](g.ast.Node(fieldNodeID).Kind)
		fields[i] = StructField{fieldNode.Name.Name, fieldTypeID, fieldNode.Mut}
	}
	structType.Fields = fields
	return TypeOK, structType
}

func (g *GenericsEngine) instantiateUnion(
	generic UnionType,
	genericTypeID TypeID,
	typeArgNodeIDs []ast.NodeID,
	span base.Span,
) (TypeID, TypeStatus) {
	unionNodeID := g.env.DeclNode(genericTypeID)
	unionNode := base.Cast[ast.Union](g.ast.Node(unionNodeID).Kind)
	providedTypeIDs, status := g.queryTypeArgs(typeArgNodeIDs)
	if status.Failed() {
		return InvalidTypeID, status
	}
	argTypeIDs, status := g.applyTypeArgDefaults(unionNode.TypeParams, providedTypeIDs, span)
	if status.Failed() {
		return InvalidTypeID, status
	}
	return g.instantiateUnionWithTypeArgs(generic, genericTypeID, argTypeIDs)
}

func (g *GenericsEngine) instantiateUnionWithTypeArgs(
	generic UnionType, genericTypeID TypeID, argTypeIDs []TypeID,
) (TypeID, TypeStatus) {
	unionNodeID := g.env.DeclNode(genericTypeID)
	unionNode := base.Cast[ast.Union](g.ast.Node(unionNodeID).Kind)
	mangledName := g.mangledName(generic.Name, genericTypeID, argTypeIDs)
	if cached, ok := g.unions[mangledName]; ok {
		return cached.TypeID, TypeOK
	}
	defer g.enterChildEnv()()
	g.bindTypeParamsToArgs(unionNode.TypeParams, argTypeIDs)
	node := g.ast.Node(unionNodeID)
	placeholder := UnionType{mangledName, nil, argTypeIDs}
	typeID := g.env.newType(placeholder, node.ID, node.Span, TypeInProgress)
	g.env.reg.genericOrigin[typeID] = genericTypeID
	g.unions[mangledName] = UnionWork{NodeID: unionNodeID, TypeID: typeID, Env: g.env}
	resolveStatus, resolved := g.resolveUnionVariants(unionNode, placeholder)
	if resolveStatus.Failed() {
		return InvalidTypeID, resolveStatus
	}
	cached := g.env.reg.types[typeID]
	cached.Type.Kind = resolved
	cached.Status = TypeOK
	return typeID, TypeOK
}

func (g *GenericsEngine) resolveUnionVariants(unionNode ast.Union, unionType UnionType) (TypeStatus, UnionType) {
	variants := make([]TypeID, len(unionNode.Variants))
	for i, variantNodeID := range unionNode.Variants {
		variantTypeID, status := g.query(variantNodeID)
		if status.Failed() {
			return TypeDepFailed, unionType
		}
		variants[i] = variantTypeID
	}
	unionType.Variants = variants
	return TypeOK, unionType
}

func (g *GenericsEngine) queryTypeArgs(typeArgNodeIDs []ast.NodeID) ([]TypeID, TypeStatus) {
	argTypeIDs := make([]TypeID, len(typeArgNodeIDs))
	for i, typeArgNodeID := range typeArgNodeIDs {
		argTypeID, status := g.query(typeArgNodeID)
		if status.Failed() {
			return nil, TypeDepFailed
		}
		argTypeIDs[i] = argTypeID
	}
	return argTypeIDs, TypeOK
}

func (g *GenericsEngine) applyTypeArgDefaults(
	typeParamNodeIDs []ast.NodeID, argTypeIDs []TypeID, span base.Span,
) ([]TypeID, TypeStatus) {
	required := len(typeParamNodeIDs)
	for i, id := range typeParamNodeIDs {
		if base.Cast[ast.TypeParam](g.ast.Node(id).Kind).Default != nil {
			required = i
			break
		}
	}
	total := len(typeParamNodeIDs)
	provided := len(argTypeIDs)
	if provided < required || provided > total {
		if required == total {
			g.diag(span, "type argument count mismatch: expected %d, got %d", total, provided)
		} else {
			g.diag(span, "type argument count mismatch: expected %d to %d, got %d", required, total, provided)
		}
		return nil, TypeFailed
	}
	if provided == total {
		return argTypeIDs, TypeOK
	}
	filled := make([]TypeID, total)
	copy(filled, argTypeIDs)
	for i := provided; i < total; i++ {
		typeParamTypeID := g.env.reg.typeParamTypes[typeParamNodeIDs[i]]
		tpt := base.Cast[TypeParamType](g.env.Type(typeParamTypeID).Kind)
		filled[i] = *tpt.Default
	}
	return filled, TypeOK
}

func (g *GenericsEngine) mangledName(baseName string, genericTypeID TypeID, argTypeIDs []TypeID) string {
	parts := make([]string, len(argTypeIDs))
	for i, id := range argTypeIDs {
		parts[i] = id.String()
	}
	return fmt.Sprintf("%s.%s.%s", baseName, genericTypeID, strings.Join(parts, "."))
}

func (g *GenericsEngine) bindTypeParamsToArgs(typeParamNodeIDs []ast.NodeID, argTypeIDs []TypeID) {
	for i, typeParamNodeID := range typeParamNodeIDs {
		typeParamNode := base.Cast[ast.TypeParam](g.ast.Node(typeParamNodeID).Kind)
		g.bind(typeParamNodeID, typeParamNode.Name.Name, false, argTypeIDs[i], typeParamNode.Name.Span)
	}
}

func (g *GenericsEngine) collectTypeArgBindings(genericTypeID, concreteTypeID TypeID, bindings map[TypeID]TypeID) {
	genericTyp := g.env.Type(genericTypeID)
	switch kind := genericTyp.Kind.(type) {
	case TypeParamType:
		bindings[genericTypeID] = concreteTypeID
	case StructType:
		if concrete, ok := g.env.Type(concreteTypeID).Kind.(StructType); ok {
			for i, typeArg := range kind.TypeArgs {
				if i < len(concrete.TypeArgs) {
					g.collectTypeArgBindings(typeArg, concrete.TypeArgs[i], bindings)
				}
			}
		}
	case UnionType:
		if concrete, ok := g.env.Type(concreteTypeID).Kind.(UnionType); ok {
			for i, typeArg := range kind.TypeArgs {
				if i < len(concrete.TypeArgs) {
					g.collectTypeArgBindings(typeArg, concrete.TypeArgs[i], bindings)
				}
			}
		}
	case RefType:
		if concrete, ok := g.env.Type(concreteTypeID).Kind.(RefType); ok {
			g.collectTypeArgBindings(kind.Type, concrete.Type, bindings)
		}
	case ArrayType:
		if concrete, ok := g.env.Type(concreteTypeID).Kind.(ArrayType); ok {
			g.collectTypeArgBindings(kind.Elem, concrete.Elem, bindings)
		}
	case SliceType:
		if concrete, ok := g.env.Type(concreteTypeID).Kind.(SliceType); ok {
			g.collectTypeArgBindings(kind.Elem, concrete.Elem, bindings)
		}
	case FunType:
		if concrete, ok := g.env.Type(concreteTypeID).Kind.(FunType); ok {
			for i, param := range kind.Params {
				if i < len(concrete.Params) {
					g.collectTypeArgBindings(param, concrete.Params[i], bindings)
				}
			}
			g.collectTypeArgBindings(kind.Return, concrete.Return, bindings)
		}
	}
}

func (g *GenericsEngine) inferTypeArgs(
	typeParamNodeIDs []ast.NodeID, genericTypeIDs, concreteTypeIDs []TypeID, span base.Span,
) ([]TypeID, TypeStatus) {
	bindings := map[TypeID]TypeID{}
	for i, genericTypeID := range genericTypeIDs {
		if i < len(concreteTypeIDs) {
			g.collectTypeArgBindings(genericTypeID, concreteTypeIDs[i], bindings)
		}
	}
	inferred := make([]TypeID, 0, len(typeParamNodeIDs))
	for _, nodeID := range typeParamNodeIDs {
		typeParamTypeID := g.env.reg.typeParamTypes[nodeID]
		if concreteID, ok := bindings[typeParamTypeID]; ok {
			inferred = append(inferred, concreteID)
		} else {
			break
		}
	}
	return g.applyTypeArgDefaults(typeParamNodeIDs, inferred, span)
}

func (g *GenericsEngine) inferTypeConstruction(
	lit ast.TypeConstruction, span base.Span, typeHint *TypeID,
) (TypeID, TypeStatus, bool) {
	ident, ok := g.ast.Node(lit.Target).Kind.(ast.Ident)
	if !ok || len(ident.TypeArgs) > 0 {
		return InvalidTypeID, TypeFailed, false
	}
	binding, ok := g.lookup(lit.Target, ident.Name)
	if !ok {
		return InvalidTypeID, TypeFailed, false
	}
	if typeHint != nil {
		if origin, ok := g.env.GenericOrigin(*typeHint); ok && origin == binding.TypeID {
			if !g.env.containsTypeParam(*typeHint) {
				g.updateCachedType(g.ast.Node(lit.Target), *typeHint, TypeOK)
				return *typeHint, TypeOK, true
			}
		}
	}
	switch kind := g.env.Type(binding.TypeID).Kind.(type) {
	case StructType:
		return g.inferStructConstruction(binding, kind, lit, span)
	case UnionType:
		return g.inferUnionConstruction(binding, kind, lit, span)
	default:
		return InvalidTypeID, TypeFailed, false
	}
}

func (g *GenericsEngine) queryArgsForInference(
	argNodeIDs []ast.NodeID,
	genericParamTypeIDs []TypeID,
) ([]TypeID, TypeStatus) {
	argTypeIDs := make([]TypeID, len(argNodeIDs))
	for i, argNodeID := range argNodeIDs {
		var argTypeID TypeID
		var status TypeStatus
		if i < len(genericParamTypeIDs) {
			if _, isTypeParam := g.env.Type(genericParamTypeIDs[i]).Kind.(TypeParamType); !isTypeParam {
				argTypeID, status = g.queryWithHint(argNodeID, genericParamTypeIDs[i])
			} else {
				argTypeID, status = g.query(argNodeID)
			}
		} else {
			argTypeID, status = g.query(argNodeID)
		}
		if status.Failed() {
			return nil, TypeDepFailed
		}
		argTypeIDs[i] = argTypeID
	}
	return argTypeIDs, TypeOK
}

func (g *GenericsEngine) inferStructConstruction(
	binding *Binding, structType StructType, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus, bool) {
	structNode, ok := g.ast.Node(binding.Decl).Kind.(ast.Struct)
	if !ok || len(structNode.TypeParams) == 0 {
		return InvalidTypeID, TypeFailed, false
	}
	fieldTypeIDs := make([]TypeID, len(structType.Fields))
	for i, field := range structType.Fields {
		fieldTypeIDs[i] = field.Type
	}
	argTypeIDs, status := g.queryArgsForInference(lit.Args, fieldTypeIDs)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed, true
	}
	inferred, status := g.inferTypeArgs(structNode.TypeParams, fieldTypeIDs, argTypeIDs, span)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	typeID, status := g.instantiateStructWithTypeArgs(structType, binding.TypeID, inferred)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	g.updateCachedType(g.ast.Node(lit.Target), typeID, TypeOK)
	return typeID, TypeOK, true
}

func (g *GenericsEngine) inferUnionConstruction(
	binding *Binding, unionType UnionType, lit ast.TypeConstruction, span base.Span,
) (TypeID, TypeStatus, bool) {
	unionNode, ok := g.ast.Node(binding.Decl).Kind.(ast.Union)
	if !ok || len(unionNode.TypeParams) == 0 {
		return InvalidTypeID, TypeFailed, false
	}
	if len(lit.Args) != 1 {
		return InvalidTypeID, TypeFailed, false
	}
	argTypeID, status := g.query(lit.Args[0])
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed, true
	}
	bindings := map[TypeID]TypeID{}
	for _, variant := range unionType.Variants {
		g.collectTypeArgBindings(variant, argTypeID, bindings)
	}
	inferred := make([]TypeID, 0, len(unionNode.TypeParams))
	for _, nodeID := range unionNode.TypeParams {
		typeParamTypeID := g.env.reg.typeParamTypes[nodeID]
		if concreteID, ok := bindings[typeParamTypeID]; ok {
			inferred = append(inferred, concreteID)
		} else {
			break
		}
	}
	filled, status := g.applyTypeArgDefaults(unionNode.TypeParams, inferred, span)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	typeID, status := g.instantiateUnionWithTypeArgs(unionType, binding.TypeID, filled)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	g.updateCachedType(g.ast.Node(lit.Target), typeID, TypeOK)
	return typeID, TypeOK, true
}

func (g *GenericsEngine) inferFunCall(call ast.Call, span base.Span) (TypeID, TypeStatus, bool) {
	ident, ok := g.ast.Node(call.Callee).Kind.(ast.Ident)
	if !ok || len(ident.TypeArgs) > 0 {
		return InvalidTypeID, TypeFailed, false
	}
	binding, ok := g.lookup(call.Callee, ident.Name)
	if !ok || binding.Decl == 0 {
		return InvalidTypeID, TypeFailed, false
	}
	funNode, ok := g.ast.Node(binding.Decl).Kind.(ast.Fun)
	if !ok || len(funNode.TypeParams) == 0 {
		return InvalidTypeID, TypeFailed, false
	}
	genericFunType := base.Cast[FunType](g.env.Type(binding.TypeID).Kind)
	argTypeIDs, status := g.queryArgsForInference(call.Args, genericFunType.Params)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed, true
	}
	inferred, status := g.inferTypeArgs(funNode.TypeParams, genericFunType.Params, argTypeIDs, span)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	typeID, mangledName, status := g.instantiateFunWithTypeArgs(binding.TypeID, call.Callee, span, inferred)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	g.env.setNamedFunRef(call.Callee, mangledName)
	g.env.setNodeType(call.Callee, &cachedType{Type: g.env.Type(typeID), Status: TypeOK})
	return typeID, TypeOK, true
}

func (g *GenericsEngine) instantiateFun(
	genericTypeID TypeID,
	argTypeIDs []TypeID,
	callSiteNodeID ast.NodeID,
	span base.Span,
) (typeID TypeID, mangledName string, status TypeStatus) {
	funNodeID := g.env.DeclNode(genericTypeID)
	funNode := base.Cast[ast.Fun](g.ast.Node(funNodeID).Kind)
	argTypeIDs, status = g.applyTypeArgDefaults(funNode.TypeParams, argTypeIDs, span)
	if status.Failed() {
		return InvalidTypeID, "", status
	}
	return g.instantiateFunWithTypeArgs(genericTypeID, callSiteNodeID, span, argTypeIDs)
}

func (g *GenericsEngine) instantiateFunWithTypeArgs(
	genericTypeID TypeID,
	callSiteNodeID ast.NodeID,
	span base.Span,
	argTypeIDs []TypeID,
) (typeID TypeID, mangledName string, status TypeStatus) {
	funNodeID := g.env.DeclNode(genericTypeID)
	funNode := base.Cast[ast.Fun](g.ast.Node(funNodeID).Kind)
	genericName, ok := g.env.NamedFunRef(funNodeID)
	if !ok {
		return InvalidTypeID, "", TypeFailed
	}
	mangledName = g.mangledName(genericName, genericTypeID, argTypeIDs)
	if cached, ok := g.funs[mangledName]; ok {
		return cached.TypeID, mangledName, TypeOK
	}
	defer g.enterChildEnv()()
	for i, typeParamNodeID := range funNode.TypeParams {
		typeParamNode := base.Cast[ast.TypeParam](g.ast.Node(typeParamNodeID).Kind)
		g.bind(typeParamNodeID, typeParamNode.Name.Name, false, argTypeIDs[i], typeParamNode.Name.Span)
		if typeParamNode.Constraint != nil {
			constraintTypeID, _ := g.query(*typeParamNode.Constraint)
			if !g.satisfiesShape(argTypeIDs[i], constraintTypeID, callSiteNodeID, span) {
				return InvalidTypeID, "", TypeFailed
			}
		}
	}
	paramTypeIDs := make([]TypeID, len(funNode.Params))
	for i, paramNodeID := range funNode.Params {
		paramTypeID, qStatus := g.query(paramNodeID)
		if qStatus.Failed() {
			return InvalidTypeID, "", TypeDepFailed
		}
		paramTypeIDs[i] = paramTypeID
	}
	retTypeID, qStatus := g.query(funNode.ReturnType)
	if qStatus.Failed() {
		return InvalidTypeID, "", TypeDepFailed
	}
	funTyp := FunType{paramTypeIDs, retTypeID}
	node := g.ast.Node(funNodeID)
	funTypeID := g.env.newType(funTyp, node.ID, node.Span, TypeOK)
	g.env.reg.genericOrigin[funTypeID] = genericTypeID
	if !funNode.Extern {
		g.funs[mangledName] = FunWork{NodeID: funNodeID, TypeID: funTypeID, Name: mangledName, Env: g.env}
		g.checkFunBody(funNode, funTypeID, funTyp)
	}
	return funTypeID, mangledName, TypeOK
}

func (g *GenericsEngine) satisfiesShape( //nolint:funlen
	concreteTypeID TypeID,
	shapeTypeID TypeID,
	scopeNodeID ast.NodeID,
	span base.Span,
) bool {
	shapeType := base.Cast[ShapeType](g.env.Type(shapeTypeID).Kind)
	concreteTyp := g.env.Type(concreteTypeID)
	// A type parameter constrained by the same shape trivially satisfies it.
	if tpt, ok := concreteTyp.Kind.(TypeParamType); ok && tpt.Shape != nil && *tpt.Shape == shapeTypeID {
		return true
	}
	concreteDisplay := g.env.TypeDisplay(concreteTypeID)
	// For method lookup we need the scope-resolvable name. For monomorphized
	// generics (e.g. Pair<Str, Int>) methods are defined on the generic type
	// (Pair), so we look up via the origin type's name.
	methodLookupName := g.env.typeName(concreteTyp)
	if originID, ok := g.env.GenericOrigin(concreteTypeID); ok {
		methodLookupName = g.env.typeName(g.env.Type(originID))
	}
	// Field requirements need a StructType.
	if len(shapeType.Fields) > 0 {
		structType, isStruct := concreteTyp.Kind.(StructType)
		if !isStruct {
			g.diag(span, "type %s does not satisfy shape %s: not a struct",
				concreteDisplay, shapeType.DeclName)
			return false
		}
		for _, reqField := range shapeType.Fields {
			found := false
			for _, field := range structType.Fields {
				if field.Name == reqField.Name {
					found = true
					if field.Type != reqField.Type {
						g.diag(span, "type %s does not satisfy shape %s: field %s has type %s, expected %s",
							concreteDisplay, shapeType.DeclName, field.Name,
							g.env.TypeDisplay(field.Type), g.env.TypeDisplay(reqField.Type))
						return false
					}
					if reqField.Mut && !field.Mut {
						g.diag(span, "type %s does not satisfy shape %s: field %s must be mut",
							concreteDisplay, shapeType.DeclName, field.Name)
						return false
					}
					break
				}
			}
			if !found {
				g.diag(span, "type %s does not satisfy shape %s: missing field %s",
					concreteDisplay, shapeType.DeclName, reqField.Name)
				return false
			}
		}
	}
	// Method requirements — any type can have methods.
	shapeNodeID := g.env.DeclNode(shapeTypeID)
	shapeNode := base.Cast[ast.Shape](g.ast.Node(shapeNodeID).Kind)
	// For monomorphized generic types, the concrete type args need to be
	// applied to generic methods defined on the type.
	var concreteTypeArgs []TypeID
	if typ, ok := IsStructOrUnion(concreteTyp.Kind); ok {
		concreteTypeArgs = typ.TypeArgs
	}
	for _, funDeclNodeID := range shapeNode.Funs {
		funDecl := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		binding, ok := g.lookup(scopeNodeID, methodLookupName+"."+methodName)
		if !ok {
			g.diag(span, "type %s does not satisfy shape %s: missing method %s",
				concreteDisplay, shapeType.DeclName, methodName)
			return false
		}
		shapeFunBinding, _ := g.lookup(scopeNodeID, shapeType.DeclName+"."+methodName)
		shapeFunType := base.Cast[FunType](g.env.Type(shapeFunBinding.TypeID).Kind)
		expectedFunType := g.env.substituteFunType(shapeFunType, shapeTypeID, concreteTypeID)
		// When the method is a generic function on a generic type, instantiate
		// it with the type's type args to get the concrete signature.
		methodFunNode, isFun := g.ast.Node(binding.Decl).Kind.(ast.Fun)
		if isFun && len(methodFunNode.TypeParams) > 0 && len(concreteTypeArgs) > 0 {
			methodTypeID, _, status := g.instantiateFun(binding.TypeID, concreteTypeArgs, scopeNodeID, span)
			if status.Failed() {
				return false
			}
			concreteFunType := base.Cast[FunType](g.env.Type(methodTypeID).Kind)
			if !expectedFunType.Equal(concreteFunType) {
				g.diag(span,
					"type %s does not satisfy shape %s: method %s has signature %s, expected %s",
					concreteDisplay, shapeType.DeclName, methodName,
					g.env.TypeDisplay(methodTypeID), g.env.TypeDisplay(shapeFunBinding.TypeID))
				return false
			}
		} else {
			concreteFunType := base.Cast[FunType](g.env.Type(binding.TypeID).Kind)
			if !expectedFunType.Equal(concreteFunType) {
				g.diag(span,
					"type %s does not satisfy shape %s: method %s has signature %s, expected %s",
					concreteDisplay, shapeType.DeclName, methodName,
					g.env.TypeDisplay(binding.TypeID), g.env.TypeDisplay(shapeFunBinding.TypeID))
				return false
			}
		}
	}
	return true
}

func (g *GenericsEngine) bindTypeParams(typeParamNodeIDs []ast.NodeID) TypeStatus {
	seen := map[string]bool{}
	for _, typeParamNodeID := range typeParamNodeIDs {
		typeParamNode := base.Cast[ast.TypeParam](g.ast.Node(typeParamNodeID).Kind)
		if seen[typeParamNode.Name.Name] {
			g.diag(typeParamNode.Name.Span, "duplicate type parameter: %s", typeParamNode.Name.Name)
			return TypeFailed
		}
		seen[typeParamNode.Name.Name] = true
		var shapeID *TypeID
		if typeParamNode.Constraint != nil {
			constraintTypeID, status := g.query(*typeParamNode.Constraint)
			if status.Failed() {
				return TypeDepFailed
			}
			if _, ok := g.env.Type(constraintTypeID).Kind.(ShapeType); !ok {
				g.diag(g.ast.Node(*typeParamNode.Constraint).Span, "constraint must be a shape")
				return TypeFailed
			}
			shapeID = &constraintTypeID
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
		typeParamID := g.env.newType(
			TypeParamType{Shape: shapeID, Default: defaultID},
			typeParamNodeID, g.ast.Node(typeParamNodeID).Span, TypeOK,
		)
		g.env.reg.typeParamTypes[typeParamNodeID] = typeParamID
		g.bind(typeParamNodeID, typeParamNode.Name.Name, false, typeParamID, typeParamNode.Name.Span)
	}
	return TypeOK
}

func (g *GenericsEngine) checkShapeCreateAndBind(node *ast.Node, shapeNode ast.Shape) (TypeID, TypeStatus) {
	name := g.namespacedName(node.ID, shapeNode.Name.Name)
	typeID := g.env.newType(
		ShapeType{Name: name, DeclName: shapeNode.Name.Name, Fields: nil}, node.ID, node.Span, TypeInProgress,
	)
	if !g.bind(node.ID, shapeNode.Name.Name, false, typeID, shapeNode.Name.Span) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

func (g *GenericsEngine) checkShapeCompleteType(
	node *ast.Node,
	shapeNode ast.Shape,
	shapeType ShapeType,
) (TypeStatus, ShapeType) {
	fields := make([]StructField, len(shapeNode.Fields))
	for i, fieldNodeID := range shapeNode.Fields {
		fieldTypeID, status := g.query(fieldNodeID)
		if status.Failed() {
			return TypeDepFailed, shapeType
		}
		fieldNode := base.Cast[ast.StructField](g.ast.Node(fieldNodeID).Kind)
		fields[i] = StructField{fieldNode.Name.Name, fieldTypeID, fieldNode.Mut}
	}
	shapeType.Fields = fields
	parentScope := g.scopeGraph.NodeScope(node.ID)
	for _, funDeclNodeID := range shapeNode.Funs {
		funDecl := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
		funTypeID, status := g.checkShapeFunDecl(funDecl)
		if status.Failed() {
			return status, shapeType
		}
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		bindName := shapeType.DeclName + "." + methodName
		if !g.env.bindInScope(parentScope, funDeclNodeID, bindName, funTypeID) {
			g.diag(funDecl.Name.Span, "symbol already defined: %s", bindName)
		}
	}
	return TypeOK, shapeType
}

func (g *GenericsEngine) checkShapeFunDecl(funDecl ast.FunDecl) (TypeID, TypeStatus) {
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
	funType := FunType{Params: paramTypeIDs, Return: retTypeID}
	return g.env.newType(funType, 0, base.Span{}, TypeOK), TypeOK
}

func (g *GenericsEngine) checkShapeFieldAccess(
	targetTyp *Type, fieldName string,
) (TypeID, TypeStatus, bool) {
	typeParamType, ok := targetTyp.Kind.(TypeParamType)
	if !ok || typeParamType.Shape == nil {
		return InvalidTypeID, TypeFailed, false
	}
	shapeType := base.Cast[ShapeType](g.env.Type(*typeParamType.Shape).Kind)
	for _, field := range shapeType.Fields {
		if field.Name == fieldName {
			return field.Type, TypeOK, true
		}
	}
	return InvalidTypeID, TypeFailed, false
}

func (g *GenericsEngine) resolveShapeMethod(
	nodeID ast.NodeID, binding *Binding, targetTyp *Type,
) (TypeID, TypeStatus, bool) {
	if _, isFunDecl := g.ast.Node(binding.Decl).Kind.(ast.FunDecl); !isFunDecl {
		return InvalidTypeID, TypeFailed, false
	}
	g.env.setNamedFunRef(nodeID, binding.Name)
	tpt := base.Cast[TypeParamType](targetTyp.Kind)
	shapeFunType := base.Cast[FunType](g.env.Type(binding.TypeID).Kind)
	substFunType := g.env.substituteFunType(shapeFunType, *tpt.Shape, targetTyp.ID)
	funTypeID := g.env.newType(substFunType, 0, base.Span{}, TypeOK)
	return funTypeID, TypeOK, true
}

func (g *GenericsEngine) resolveGenericMethod(
	nodeID ast.NodeID, fieldAccess ast.FieldAccess, targetTyp *Type, binding *Binding,
) (TypeID, TypeStatus, bool) {
	var argTypeIDs []TypeID
	if typ, ok := IsStructOrUnion(targetTyp.Kind); ok {
		argTypeIDs = append(argTypeIDs, typ.TypeArgs...)
	}
	extraArgs, status := g.queryTypeArgs(fieldAccess.TypeArgs)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	argTypeIDs = append(argTypeIDs, extraArgs...)
	typeID, mangledName, status := g.instantiateFun(binding.TypeID, argTypeIDs, nodeID, fieldAccess.Field.Span)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	g.env.setNamedFunRef(nodeID, mangledName)
	return typeID, TypeOK, true
}
