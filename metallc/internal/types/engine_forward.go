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
		case ast.Struct, ast.Shape, ast.Union, ast.Enum:
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
		enumNode, ok := decl.node.Kind.(ast.Enum)
		if !ok {
			continue
		}
		typeID, status := e.checkEnumCreateAndBind(decl.node, enumNode)
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

	// Both structs and unions are complete now, so a by-value cycle (e.g.
	// `List -> ?List -> List`) is fully formed. Diagnosing it here fails the build
	// before codegen, and isCopyable carries its own cycle guard, so the infinite
	// type is reported instead of overflowing the stack. A field or variant has
	// infinite size when it transitively contains the type itself by value; the
	// walk stops at any reference, slice, or pointer, which breaks the cycle.
	var reachesByValue func(typeID TypeID, visiting map[TypeID]bool) bool
	reachesByValue = func(typeID TypeID, visiting map[TypeID]bool) bool {
		switch kind := e.env.Type(typeID).Kind.(type) {
		case StructType:
			if visiting[typeID] {
				return true
			}
			visiting[typeID] = true
			for _, field := range kind.Fields {
				if reachesByValue(field.Type, visiting) {
					return true
				}
			}
			delete(visiting, typeID)
		case UnionType:
			if visiting[typeID] {
				return true
			}
			visiting[typeID] = true
			for _, variant := range kind.Variants {
				if reachesByValue(variant, visiting) {
					return true
				}
			}
			delete(visiting, typeID)
		case ArrayType:
			return reachesByValue(kind.Elem, visiting)
		}
		return false
	}
	for _, decl := range decls {
		if decl.status.Failed() {
			continue
		}
		switch node := decl.node.Kind.(type) {
		case ast.Struct:
			st, ok := decl.cachedType.Type.Kind.(StructType)
			if !ok {
				continue
			}
			for i, field := range st.Fields {
				if reachesByValue(field.Type, map[TypeID]bool{decl.typeID: true}) {
					e.diag(e.ast.Node(node.Fields[i]).Span,
						"recursive type %s has infinite size; break the cycle with a reference (`&` or `?&`)",
						node.Name.Name)
					break
				}
			}
		case ast.Union:
			ut, ok := decl.cachedType.Type.Kind.(UnionType)
			if !ok {
				continue
			}
			for i, variant := range ut.Variants {
				if reachesByValue(variant, map[TypeID]bool{decl.typeID: true}) {
					e.diag(e.ast.Node(node.Variants[i]).Span,
						"recursive type %s has infinite size; break the cycle with a reference (`&` or `?&`)",
						node.Name.Name)
					break
				}
			}
		}
	}

	// Complete roots/standalone enums before subsets, since a subset reads its
	// root's completed backing and schema.
	e.completeEnums(decls, false)
	e.completeEnums(decls, true)
}

func (e *Engine) completeEnums(decls []*forwardDecl, subsets bool) {
	for _, decl := range decls {
		enumNode, ok := decl.node.Kind.(ast.Enum)
		if !ok || decl.status.Failed() {
			continue
		}
		if e.enumDeclIsSubset(enumNode) != subsets {
			continue
		}
		enumType := base.Cast[EnumType](decl.cachedType.Type.Kind)
		status, newKind := e.checkEnumCompleteType(decl.node, enumNode, enumType)
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
