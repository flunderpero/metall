package types

import (
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type MethodContext struct {
	DeclTypeID     TypeID
	OwnerTypeID    TypeID
	ReceiverTypeID TypeID
}

func (e *Engine) ShapeMethodContext(shapeTypeID, receiverTypeID TypeID) MethodContext {
	ownerTypeID, _, ok := e.shapeOwner(shapeTypeID)
	if !ok {
		panic(base.Errorf("shape owner not found for %s", shapeTypeID))
	}
	return MethodContext{DeclTypeID: ownerTypeID, OwnerTypeID: shapeTypeID, ReceiverTypeID: receiverTypeID}
}

func (e *Engine) BoundMethodContext(binding *Binding, receiverTypeID TypeID) MethodContext {
	if _, ok := e.ast.Node(binding.Decl).Kind.(ast.FunDecl); !ok {
		return MethodContext{DeclTypeID: receiverTypeID, OwnerTypeID: receiverTypeID, ReceiverTypeID: receiverTypeID}
	}
	receiverType := e.env.Type(receiverTypeID)
	if refType, ok := receiverType.Kind.(RefType); ok {
		receiverTypeID = refType.Type
		receiverType = e.env.Type(receiverTypeID)
	}
	if tpt, ok := receiverType.Kind.(TypeParamType); ok && tpt.Shape != nil {
		return e.ShapeMethodContext(*tpt.Shape, receiverTypeID)
	}
	return MethodContext{DeclTypeID: receiverTypeID, OwnerTypeID: receiverTypeID, ReceiverTypeID: receiverTypeID}
}

func (e *Engine) SatisfiesShape( //nolint:funlen
	concreteTypeID TypeID,
	shapeTypeID TypeID,
	scopeNodeID ast.NodeID,
	span base.Span,
) bool {
	shapeType := base.Cast[ShapeType](e.env.Type(shapeTypeID).Kind)
	concreteTyp := e.env.Type(concreteTypeID)
	e.debug.Print(0, "SatisfiesShape concrete=%s shape=%s",
		e.env.TypeDisplay(concreteTypeID), e.env.TypeDisplay(shapeTypeID))
	// Unwrap RefType for method lookup, because methods live on the underlying type.
	lookupTypeID := concreteTypeID
	lookupTyp := concreteTyp
	if refTyp, ok := concreteTyp.Kind.(RefType); ok {
		lookupTypeID = refTyp.Type
		lookupTyp = e.env.Type(lookupTypeID)
	}
	if tpt, ok := lookupTyp.Kind.(TypeParamType); ok && tpt.Shape != nil && *tpt.Shape == shapeTypeID {
		return true
	}
	concreteDisplay := e.env.TypeDisplay(concreteTypeID)
	if _, ok := e.env.methodFQN(lookupTyp, ""); !ok {
		e.diag(span, "type %s cannot satisfy shape %s", concreteDisplay, shapeType.DeclName)
		return false
	}
	if len(shapeType.Fields) > 0 {
		structType, isStruct := lookupTyp.Kind.(StructType)
		if !isStruct {
			e.diag(span, "type %s does not satisfy shape %s: not a struct", concreteDisplay, shapeType.DeclName)
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
					e.diag(span,
						"type %s does not satisfy shape %s: field %s has type %s, expected %s",
						concreteDisplay,
						shapeType.DeclName,
						field.Name,
						e.env.TypeDisplay(field.Type),
						e.env.TypeDisplay(reqField.Type),
					)
					return false
				}
				if reqField.Pub && !field.Pub {
					e.diag(span, "type %s does not satisfy shape %s: field %s must be public",
						concreteDisplay, shapeType.DeclName, field.Name)
					return false
				}
				if !reqField.Pub && field.Pub {
					e.diag(span, "type %s does not satisfy shape %s: field %s must not be public",
						concreteDisplay, shapeType.DeclName, field.Name)
					return false
				}
				if reqField.Mut && !field.Mut {
					e.diag(span, "type %s does not satisfy shape %s: field %s must be mut",
						concreteDisplay, shapeType.DeclName, field.Name)
					return false
				}
				break
			}
			if !matched {
				e.diag(span, "type %s does not satisfy shape %s: missing field %s",
					concreteDisplay, shapeType.DeclName, reqField.Name)
				return false
			}
		}
	}
	shapeNodeID := e.env.DeclNode(typeIDOrigin(e.env, shapeTypeID))
	shapeNode := base.Cast[ast.Shape](e.ast.Node(shapeNodeID).Kind)
	for _, funDeclNodeID := range shapeNode.Funs {
		funDecl := base.Cast[ast.FunDecl](e.ast.Node(funDeclNodeID).Kind)
		_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
		binding, ok := e.lookupMethodBinding(scopeNodeID, lookupTypeID, methodName)
		if !ok {
			e.diag(span, "type %s does not satisfy shape %s: missing method %s",
				concreteDisplay, shapeType.DeclName, methodName)
			return false
		}
		shapeFunBinding, ok := e.LookupShapeMethodBinding(shapeTypeID, methodName, scopeNodeID)
		if !ok {
			panic(base.Errorf("shape method %s not found", methodName))
		}
		expectedFunType, status := e.MethodSignature(
			shapeFunBinding,
			e.ShapeMethodContext(shapeTypeID, lookupTypeID),
			scopeNodeID,
			span,
		)
		if status.Failed() {
			return false
		}
		concreteFunType, status := e.MethodSignature(
			binding,
			e.BoundMethodContext(binding, concreteTypeID),
			scopeNodeID,
			span,
		)
		if status.Failed() {
			return false
		}
		e.debug.Print(0, "SatisfiesShape method=%s expected=%s concrete=%s",
			methodName, e.FunTypeDisplay(expectedFunType), e.FunTypeDisplay(concreteFunType))
		// Check pub modifier matches.
		concretePub := e.declIsPub(binding.Decl)
		if funDecl.Pub && !concretePub {
			e.diag(span, "type %s does not satisfy shape %s: method %s must be public",
				concreteDisplay, shapeType.DeclName, methodName)
			return false
		}
		if !funDecl.Pub && concretePub {
			e.diag(span, "type %s does not satisfy shape %s: method %s must not be public",
				concreteDisplay, shapeType.DeclName, methodName)
			return false
		}
		if !e.shapeMethodMatches(expectedFunType, concreteFunType) {
			e.diag(span,
				"type %s does not satisfy shape %s: method %s has signature %s, expected %s",
				concreteDisplay,
				shapeType.DeclName,
				methodName,
				e.FunTypeDisplay(concreteFunType),
				e.env.TypeDisplay(shapeFunBinding.TypeID),
			)
			return false
		}
	}
	return true
}

func (e *Engine) LookupShapeMethodBinding(
	shapeTypeID TypeID,
	methodName string,
	scopeNodeID ast.NodeID,
) (*Binding, bool) {
	shapeOriginID := typeIDOrigin(e.env, shapeTypeID)
	shapeType := base.Cast[ShapeType](e.env.Type(shapeOriginID).Kind)
	bindName := shapeType.DeclName + "." + methodName
	binding, ok := e.lookup(scopeNodeID, bindName)
	if !ok {
		binding, ok = e.lookupInTypeModule(e.env.Type(shapeOriginID), bindName)
	}
	return binding, ok
}

// shapeMethodMatches checks if a concrete method signature satisfies a shape's
// expected signature, allowing &mut T → &T and ref coercion on parameters.
func (e *Engine) shapeMethodMatches(expected, concrete FunType) bool {
	if expected.Return != concrete.Return || expected.Macro != concrete.Macro {
		return false
	}
	if len(expected.Params) != len(concrete.Params) {
		return false
	}
	for i, ep := range expected.Params {
		cp := concrete.Params[i]
		if e.isAssignableTo(ep, cp) {
			continue
		}
		// Allow ref coercion: the concrete method may wrap the parameter
		// in a ref (e.g. shape says `S`, concrete takes `&Foo`).
		if refTyp, ok := e.env.Type(cp).Kind.(RefType); ok {
			if e.isAssignableTo(ep, refTyp.Type) {
				continue
			}
		}
		return false
	}
	return true
}

func (e *Engine) FunTypeDisplay(funType FunType) string {
	typeID := e.env.newType(funType, 0, base.Span{}, TypeOK)
	return e.env.TypeDisplay(typeID)
}

func (e *Engine) MethodSignature(
	binding *Binding,
	ctx MethodContext,
	scopeNodeID ast.NodeID,
	span base.Span,
) (FunType, TypeStatus) {
	receiverType := e.env.Type(ctx.ReceiverTypeID)
	if refTyp, ok := receiverType.Kind.(RefType); ok {
		receiverType = e.env.Type(refTyp.Type)
	}
	if funDecl, ok := e.ast.Node(binding.Decl).Kind.(ast.FunDecl); ok {
		shapeDecl, _ := e.NormalizeGenericDecl(binding.Decl, ctx.DeclTypeID, binding.Name)
		shapeType := base.Cast[ShapeType](e.env.Type(ctx.OwnerTypeID).Kind)
		spec, status := e.BuildGenericSpec(shapeDecl.originTypeID, shapeDecl.typeParams)
		if status.Failed() {
			return FunType{}, status
		}
		replacements := spec.Bindings(shapeType.TypeArgs)
		replacements[shapeDecl.originTypeID] = ctx.ReceiverTypeID
		defer e.enterChildEnv()()
		e.BindGenericArgs(spec, shapeType.TypeArgs)
		typeID, status := e.CheckShapeFunDecl(funDecl)
		if status.Failed() {
			return FunType{}, status
		}
		funType := base.Cast[FunType](e.env.Type(typeID).Kind)
		rewritten, _, status := e.RewriteFunType(funType, replacements)
		if status.Failed() {
			return FunType{}, status
		}
		return rewritten, TypeOK
	}
	if funNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Fun); ok && len(funNode.TypeParams) > 0 {
		seedArgs, ok := ImplicitTypeArgs(receiverType.Kind)
		if !ok {
			return FunType{}, TypeFailed
		}
		if len(seedArgs) > 0 {
			decl, _ := e.NormalizeGenericDecl(binding.Decl, binding.TypeID, "")
			spec, status := e.BuildGenericSpec(decl.originTypeID, decl.typeParams)
			if status.Failed() {
				return FunType{}, status
			}
			argTypeIDs, status := e.SolveGenericArgs(spec, seedArgs, scopeNodeID, span)
			if status.Failed() {
				return FunType{}, status
			}
			methodTypeID, _, status := e.MaterializeFun(
				binding.Decl,
				binding.TypeID,
				scopeNodeID,
				span,
				argTypeIDs,
			)
			if status.Failed() {
				return FunType{}, status
			}
			return base.Cast[FunType](e.env.Type(methodTypeID).Kind), TypeOK
		}
	}
	return base.Cast[FunType](e.env.Type(binding.TypeID).Kind), TypeOK
}

func (e *Engine) CheckShapeCreateAndBind(node *ast.Node, shapeNode ast.Shape) (TypeID, TypeStatus) {
	name := e.namespacedName(node.ID, shapeNode.Name.Name)
	typeID := e.env.newType(
		ShapeType{Name: name, DeclName: shapeNode.Name.Name, Fields: nil, TypeArgs: nil},
		node.ID,
		node.Span,
		TypeInProgress,
	)
	if !e.bind(node.ID, shapeNode.Name.Name, false, typeID, shapeNode.Name.Span) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

func (e *Engine) CheckShapeCompleteType(
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
			defer e.enterChildEnv()()
			if status := e.bindTypeParams(shapeNode.TypeParams); status.Failed() {
				return status
			}
		}
		fields := make([]StructField, len(shapeNode.Fields))
		for i, fieldNodeID := range shapeNode.Fields {
			fieldTypeID, status := e.Query(fieldNodeID)
			if status.Failed() {
				return TypeDepFailed
			}
			fieldNode := base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind)
			fields[i] = StructField{
				Name: fieldNode.Name.Name,
				Type: fieldTypeID,
				Pub:  fieldNode.Pub,
				Mut:  fieldNode.Mut,
			}
		}
		shapeType.Fields = fields
		for _, funDeclNodeID := range shapeNode.Funs {
			funDecl := base.Cast[ast.FunDecl](e.ast.Node(funDeclNodeID).Kind)
			funTypeID, status := e.CheckShapeFunDecl(funDecl)
			if status.Failed() {
				return status
			}
			_, methodName, _ := strings.Cut(funDecl.Name.Name, ".")
			methods = append(
				methods,
				methodInfo{nodeID: funDeclNodeID, name: methodName, nameSpan: funDecl.Name.Span, typeID: funTypeID},
			)
		}
		return TypeOK
	}
	if status := resolveInner(); status.Failed() {
		return status, shapeType
	}
	parentScope := e.scopeGraph.NodeScope(node.ID)
	for _, method := range methods {
		bindName := shapeType.DeclName + "." + method.name
		if !e.env.bindInScope(parentScope, method.nodeID, bindName, method.typeID) {
			e.diag(method.nameSpan, "symbol already defined: %s", bindName)
		}
	}
	return TypeOK, shapeType
}

func (e *Engine) CheckShapeFunDecl(funDecl ast.FunDecl) (TypeID, TypeStatus) {
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
	funType := FunType{Params: paramTypeIDs, Return: retTypeID, Macro: false}
	return e.env.newType(funType, 0, base.Span{}, TypeOK), TypeOK
}

func (e *Engine) CheckShapeFieldAccess(targetTyp *Type, fieldName string) (TypeID, TypeStatus, bool) {
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

func (e *Engine) ResolveMethodBinding(
	nodeID ast.NodeID,
	fieldAccess ast.FieldAccess,
	targetTyp *Type,
	binding *Binding,
) (TypeID, TypeStatus, bool) {
	if _, isFunDecl := e.ast.Node(binding.Decl).Kind.(ast.FunDecl); isFunDecl {
		e.env.setNamedFunRef(nodeID, binding.Name)
		funType, status := e.MethodSignature(
			binding,
			e.BoundMethodContext(binding, targetTyp.ID),
			nodeID,
			fieldAccess.Field.Span,
		)
		if status.Failed() {
			return InvalidTypeID, status, true
		}
		return e.env.newType(funType, 0, base.Span{}, TypeOK), TypeOK, true
	}
	funNode, isFun := e.ast.Node(binding.Decl).Kind.(ast.Fun)
	if !isFun || (len(funNode.TypeParams) == 0 && len(fieldAccess.TypeArgs) == 0) {
		e.env.copyNamedFunRef(nodeID, binding.Decl)
		e.registerFun(binding.Decl)
		return binding.TypeID, TypeOK, true
	}
	var argTypeIDs []TypeID
	if typeArgs, ok := ImplicitTypeArgs(targetTyp.Kind); ok {
		argTypeIDs = append(argTypeIDs, typeArgs...)
	}
	extraArgs, status := e.QueryTypeArgs(fieldAccess.TypeArgs)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	argTypeIDs = append(argTypeIDs, extraArgs...)
	decl, _ := e.NormalizeGenericDecl(binding.Decl, binding.TypeID, "")
	spec, status := e.BuildGenericSpec(decl.originTypeID, decl.typeParams)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	argTypeIDs, status = e.SolveGenericArgs(spec, argTypeIDs, nodeID, fieldAccess.Field.Span)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	typeID, mangledName, status := e.MaterializeFun(
		binding.Decl,
		binding.TypeID,
		nodeID,
		fieldAccess.Field.Span,
		argTypeIDs,
	)
	if status.Failed() {
		return InvalidTypeID, status, true
	}
	e.env.setNamedFunRef(nodeID, mangledName)
	return typeID, TypeOK, true
}
