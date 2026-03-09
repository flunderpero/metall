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
	query        QueryFunc
	checkFunBody CheckFunBodyFunc
}

func NewGenericsEngine(c *EngineCore, query QueryFunc, checkFunBody CheckFunBodyFunc) *GenericsEngine {
	return &GenericsEngine{c, query, checkFunBody}
}

func (g *GenericsEngine) instantiateStruct(
	generic StructType,
	genericTypeID TypeID,
	typeArgNodeIDs []ast.NodeID,
	span base.Span,
) (TypeID, TypeStatus) {
	structNodeID := g.env.DeclNode(genericTypeID)
	structNode := base.Cast[ast.Struct](g.ast.Node(structNodeID).Kind)
	if len(typeArgNodeIDs) != len(structNode.TypeParams) {
		g.diag(span, "type argument count mismatch: expected %d, got %d",
			len(structNode.TypeParams), len(typeArgNodeIDs))
		return InvalidTypeID, TypeFailed
	}
	mangledParts := make([]string, len(typeArgNodeIDs))
	argTypeIDs := make([]TypeID, len(typeArgNodeIDs))
	for i, typeArgNodeID := range typeArgNodeIDs {
		argTypeID, status := g.query(typeArgNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		argTypeIDs[i] = argTypeID
		mangledParts[i] = argTypeID.String()
	}
	mangledName := fmt.Sprintf("%s.%s.%s", generic.Name, genericTypeID, strings.Join(mangledParts, "."))
	if cached, ok := g.structs[mangledName]; ok {
		return cached.TypeID, TypeOK
	}
	defer g.enterChildEnv()()
	for i, typeParamNodeID := range structNode.TypeParams {
		typeParamNode := base.Cast[ast.TypeParam](g.ast.Node(typeParamNodeID).Kind)
		g.bind(typeParamNodeID, typeParamNode.Name.Name, false, argTypeIDs[i], typeParamNode.Name.Span)
	}
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

func (g *GenericsEngine) resolveTypeArgs(typeArgNodeIDs []ast.NodeID) ([]TypeID, TypeStatus) {
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

func (g *GenericsEngine) instantiateFun(
	genericTypeID TypeID,
	argTypeIDs []TypeID,
	span base.Span,
) (typeID TypeID, mangledName string, status TypeStatus) {
	funNodeID := g.env.DeclNode(genericTypeID)
	funNode := base.Cast[ast.Fun](g.ast.Node(funNodeID).Kind)
	if len(argTypeIDs) != len(funNode.TypeParams) {
		g.diag(span, "type argument count mismatch: expected %d, got %d",
			len(funNode.TypeParams), len(argTypeIDs))
		return InvalidTypeID, "", TypeFailed
	}
	mangledParts := make([]string, len(argTypeIDs))
	for i, argTypeID := range argTypeIDs {
		mangledParts[i] = argTypeID.String()
	}
	genericName, ok := g.env.NamedFunRef(funNodeID)
	if !ok {
		panic(base.Errorf("no namespaced name for function node %s", funNodeID))
	}
	mangledName = fmt.Sprintf("%s.%s.%s", genericName, genericTypeID, strings.Join(mangledParts, "."))
	if cached, ok := g.funs[mangledName]; ok {
		return cached.TypeID, mangledName, TypeOK
	}
	defer g.enterChildEnv()()
	for i, typeParamNodeID := range funNode.TypeParams {
		typeParamNode := base.Cast[ast.TypeParam](g.ast.Node(typeParamNodeID).Kind)
		g.bind(typeParamNodeID, typeParamNode.Name.Name, false, argTypeIDs[i], typeParamNode.Name.Span)
		if typeParamNode.Constraint != nil {
			constraintTypeID, _ := g.query(*typeParamNode.Constraint)
			if !g.satisfiesShape(argTypeIDs[i], constraintTypeID, typeParamNodeID, span) {
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
	g.funs[mangledName] = FunWork{NodeID: funNodeID, TypeID: funTypeID, Name: mangledName, Env: g.env}
	g.checkFunBody(funNode, funTypeID, funTyp)
	return funTypeID, mangledName, TypeOK
}

func (g *GenericsEngine) satisfiesShape(
	concreteTypeID TypeID, shapeTypeID TypeID, scopeNodeID ast.NodeID, span base.Span,
) bool {
	shapeType := base.Cast[ShapeType](g.env.Type(shapeTypeID).Kind)
	concreteTyp := g.env.Type(concreteTypeID)
	structType, isStruct := concreteTyp.Kind.(StructType)
	if !isStruct {
		g.diag(span, "type %s does not satisfy shape %s: not a struct",
			g.env.TypeDisplay(concreteTypeID), shapeType.DeclName)
		return false
	}
	for _, reqField := range shapeType.Fields {
		found := false
		for _, field := range structType.Fields {
			if field.Name == reqField.Name {
				found = true
				if field.Type != reqField.Type {
					g.diag(span, "type %s does not satisfy shape %s: field %s has type %s, expected %s",
						structType.Name, shapeType.DeclName, field.Name,
						g.env.TypeDisplay(field.Type), g.env.TypeDisplay(reqField.Type))
					return false
				}
				if reqField.Mut && !field.Mut {
					g.diag(span, "type %s does not satisfy shape %s: field %s must be mut",
						structType.Name, shapeType.DeclName, field.Name)
					return false
				}
				break
			}
		}
		if !found {
			g.diag(span, "type %s does not satisfy shape %s: missing field %s",
				structType.Name, shapeType.DeclName, reqField.Name)
			return false
		}
	}
	shapeNodeID := g.env.DeclNode(shapeTypeID)
	shapeNode := base.Cast[ast.Shape](g.ast.Node(shapeNodeID).Kind)
	concreteName := g.env.typeName(concreteTyp)
	for _, funDeclNodeID := range shapeNode.Funs {
		funDecl := base.Cast[ast.FunDecl](g.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		binding, ok := g.lookup(scopeNodeID, concreteName+"."+methodName)
		if !ok {
			g.diag(span, "type %s does not satisfy shape %s: missing method %s",
				structType.Name, shapeType.DeclName, methodName)
			return false
		}
		shapeFunBinding, _ := g.lookup(scopeNodeID, shapeType.DeclName+"."+methodName)
		shapeFunType := base.Cast[FunType](g.env.Type(shapeFunBinding.TypeID).Kind)
		expectedFunType := g.env.substituteFunType(shapeFunType, shapeTypeID, concreteTypeID)
		concreteFunType := base.Cast[FunType](g.env.Type(binding.TypeID).Kind)
		if !expectedFunType.Equal(concreteFunType) {
			g.diag(span,
				"type %s does not satisfy shape %s: method %s has signature %s, expected %s",
				structType.Name, shapeType.DeclName, methodName,
				g.env.TypeDisplay(binding.TypeID), g.env.TypeDisplay(shapeFunBinding.TypeID))
			return false
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
		typeParamID := g.env.newType(
			TypeParamType{Shape: shapeID}, typeParamNodeID, g.ast.Node(typeParamNodeID).Span, TypeOK,
		)
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
	if structType, isStruct := targetTyp.Kind.(StructType); isStruct {
		argTypeIDs = append(argTypeIDs, structType.TypeArgs...)
	}
	extraArgs, status := g.resolveTypeArgs(fieldAccess.TypeArgs)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	argTypeIDs = append(argTypeIDs, extraArgs...)
	typeID, mangledName, status := g.instantiateFun(binding.TypeID, argTypeIDs, fieldAccess.Field.Span)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	g.env.setNamedFunRef(nodeID, mangledName)
	return typeID, TypeOK, true
}
