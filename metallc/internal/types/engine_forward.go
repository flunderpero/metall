package types

import (
	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type forwardDecl struct {
	node       *ast.Node
	typeID     TypeID
	status     TypeStatus
	cachedType *cachedType
}

func (e *Engine) forwardDeclareTypes(nodeIDs []ast.NodeID) { //nolint:funlen
	var decls []*forwardDecl
	for _, nodeID := range nodeIDs {
		node := e.ast.Node(nodeID)
		switch node.Kind.(type) {
		case ast.Struct, ast.Shape, ast.Union:
			decls = append(decls, &forwardDecl{node, InvalidTypeID, TypeFailed, nil})
		}
	}

	for _, decl := range decls {
		shapeNode, ok := decl.node.Kind.(ast.Shape)
		if !ok {
			continue
		}
		typeID, status := e.checkShapeCreateAndBind(decl.node, shapeNode)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.cachedTypeInfo(typeID)
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}

	for _, decl := range decls {
		structNode, ok := decl.node.Kind.(ast.Struct)
		if !ok {
			continue
		}
		typeID, status := e.checkStructCreateAndBind(decl.node, structNode)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.cachedTypeInfo(typeID)
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}

	for _, decl := range decls {
		unionNode, ok := decl.node.Kind.(ast.Union)
		if !ok {
			continue
		}
		typeID, status := e.checkUnionCreateAndBind(decl.node, unionNode)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.cachedTypeInfo(typeID)
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}

	for _, decl := range decls {
		if _, ok := decl.node.Kind.(ast.Shape); !ok {
			continue
		}
		if decl.status.Failed() {
			continue
		}
		typeID, status, _ := e.resolveGenerics(decl.node.ID, nil)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
	}

	for _, decl := range decls {
		structNode, ok := decl.node.Kind.(ast.Struct)
		if !ok {
			continue
		}
		if decl.status.Failed() {
			continue
		}
		structType := base.Cast[StructType](decl.cachedType.Type.Kind)
		status, newKind := e.checkStructCompleteType(decl.node, structNode, structType)
		decl.cachedType.Type.Kind = newKind
		decl.typeID, decl.status = e.updateCachedType(decl.node, decl.typeID, status)
		if ast.IsPreludeNode(decl.node.ID) {
			e.fixPreludeType(decl.node, decl.cachedType)
		}
	}

	for _, decl := range decls {
		unionNode, ok := decl.node.Kind.(ast.Union)
		if !ok {
			continue
		}
		if decl.status.Failed() {
			continue
		}
		unionType := base.Cast[UnionType](decl.cachedType.Type.Kind)
		status, newKind := e.checkUnionCompleteType(decl.node, unionNode, unionType)
		decl.cachedType.Type.Kind = newKind
		decl.typeID, decl.status = e.updateCachedType(decl.node, decl.typeID, status)
	}
}

func (e *Engine) forwardDeclareFuns(nodeIDs []ast.NodeID) {
	var decls []*forwardDecl
	for _, nodeID := range nodeIDs {
		node := e.ast.Node(nodeID)
		switch node.Kind.(type) {
		case ast.Fun, ast.FunDecl:
			decls = append(decls, &forwardDecl{node, InvalidTypeID, TypeFailed, nil})
		}
	}
	for _, decl := range decls {
		var funDecl ast.FunDecl
		switch kind := decl.node.Kind.(type) {
		case ast.Fun:
			funDecl = kind.FunDecl
		case ast.FunDecl:
			funDecl = kind
		}
		typeID, status := e.checkFunCreateAndBind(decl.node, funDecl)
		decl.typeID, decl.status = e.updateCachedType(decl.node, typeID, status)
		if typeID != InvalidTypeID {
			cachedType, ok := e.env.cachedTypeInfo(typeID)
			if !ok {
				panic(base.Errorf("type %s not found", typeID))
			}
			decl.cachedType = cachedType
		}
	}
	for _, decl := range decls {
		if decl.status.Failed() {
			continue
		}
		funKind, isFun := decl.node.Kind.(ast.Fun)
		if isFun && !funKind.Builtin && !funKind.Extern {
			funType := base.Cast[FunType](decl.cachedType.Type.Kind)
			e.debug.Print(
				0,
				"forwardDeclare checkFunBody: %s (node=%s, type=%s)",
				funKind.Name.Name,
				decl.node.ID,
				decl.cachedType.Type.ID,
			)
			prev := e.skipRegisterWork
			if e.env.containsTypeParam(decl.cachedType.Type.ID) {
				e.skipRegisterWork = true
			}
			e.checkFunBody(decl.node.ID, funKind, decl.cachedType.Type.ID, funType)
			e.skipRegisterWork = prev
		}
	}
}
