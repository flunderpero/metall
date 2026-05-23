package types

import (
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

func (e *Engine) checkStructCreateAndBind(node *ast.Node, structNode ast.Struct) (TypeID, TypeStatus) {
	name := e.declMangledName(node.ID, structNode.Name.Name)
	typeID := e.env.newType(
		StructType{Name: name, Fields: []StructField{}, TypeArgs: nil},
		node.ID,
		node.Span,
		TypeInProgress,
	)
	if !e.bind(node.ID, structNode.Name.Name, false, typeID, structNode.Name.Span, -1) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

func (e *Engine) checkUnionCreateAndBind(node *ast.Node, unionNode ast.Union) (TypeID, TypeStatus) {
	name := e.declMangledName(node.ID, unionNode.Name.Name)
	typeID := e.env.newType(UnionType{name, nil, nil}, node.ID, node.Span, TypeInProgress)
	if !e.bind(node.ID, unionNode.Name.Name, false, typeID, unionNode.Name.Span, -1) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

func (e *Engine) checkShapeCreateAndBind(node *ast.Node, shapeNode ast.Shape) (TypeID, TypeStatus) {
	name := e.declMangledName(node.ID, shapeNode.Name.Name)
	typeID := e.env.newType(
		ShapeType{Name: name, DeclName: shapeNode.Name.Name, Fields: nil, TypeArgs: nil},
		node.ID,
		node.Span,
		TypeInProgress,
	)
	if !e.bind(node.ID, shapeNode.Name.Name, false, typeID, shapeNode.Name.Span, -1) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

// checkStructCompleteType walks fields with type params in scope. resolveGenerics
// binds the type params into the keyed child env for this decl; we re-enter the
// same child env so its enterChildEnvFor(node.ID) and ours land on the same env.
func (e *Engine) checkStructCompleteType(
	node *ast.Node, structNode ast.Struct, structType StructType,
) (TypeStatus, StructType) {
	if _, status, _ := e.resolveGenerics(node.ID, nil); status.Failed() {
		return status, structType
	}
	defer e.enterChildEnvFor(node.ID)()
	fields := make([]StructField, len(structNode.Fields))
	for i, fieldNodeID := range structNode.Fields {
		fieldTypeID, status := e.Query(fieldNodeID)
		if status.Failed() {
			return TypeDepFailed, structType
		}
		fieldNode := base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind)
		fields[i] = StructField{Name: fieldNode.Name.Name, Type: fieldTypeID, Pub: fieldNode.Pub}
	}
	structType.Fields = fields
	return TypeOK, structType
}

func (e *Engine) checkUnionCompleteType(
	node *ast.Node, unionNode ast.Union, unionType UnionType,
) (TypeStatus, UnionType) {
	if _, status, _ := e.resolveGenerics(node.ID, nil); status.Failed() {
		return status, unionType
	}
	defer e.enterChildEnvFor(node.ID)()
	variants := make([]TypeID, len(unionNode.Variants))
	for i, variantNodeID := range unionNode.Variants {
		variantTypeID, status := e.Query(variantNodeID)
		if status.Failed() {
			return TypeDepFailed, unionType
		}
		if unionNode.Pub {
			variantDeclNode := e.env.DeclNode(variantTypeID)
			if variantDeclNode != 0 {
				_, isTypeParam := e.ast.Node(variantDeclNode).Kind.(ast.TypeParam)
				if !isTypeParam && !e.declIsPub(variantDeclNode) {
					e.diag(e.ast.Node(variantNodeID).Span,
						"public union %s contains non-public variant type %s",
						unionNode.Name.Name, e.env.TypeDisplay(variantTypeID))
					return TypeFailed, unionType
				}
			}
		}
		variants[i] = variantTypeID
	}
	unionType.Variants = variants
	return TypeOK, unionType
}

//nolint:funlen
func (e *Engine) checkFunCreateAndBind(node *ast.Node, fun ast.FunDecl) (TypeID, TypeStatus) {
	if !e.checkMethodFieldCollision(node.ID, fun) {
		return InvalidTypeID, TypeFailed
	}
	if _, status, _ := e.resolveGenerics(node.ID, nil); status.Failed() {
		return InvalidTypeID, status
	}
	if fun.ReturnType == ast.InferredType {
		e.diag(node.Span, "cannot infer return type of function literal")
		return InvalidTypeID, TypeFailed
	}
	retTypeID, status := e.Query(fun.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if _, ok := e.env.Type(retTypeID).Kind.(AllocatorType); ok {
		e.diag(e.ast.Node(fun.ReturnType).Span, "cannot return an allocator from a function")
		return InvalidTypeID, TypeFailed
	}
	paramTypeIDs := make([]TypeID, len(fun.Params))
	noescapeParams := make([]bool, len(fun.Params))
	for i, paramNodeID := range fun.Params {
		paramNode := base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind)
		if paramNode.Type == ast.InferredType {
			e.diag(paramNode.Name.Span, "cannot infer type of parameter '%s'", paramNode.Name.Name)
			return InvalidTypeID, TypeFailed
		}
		paramTypeID, status := e.Query(paramNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		paramTypeIDs[i] = paramTypeID
		if paramNode.Noescape {
			noescapeParams[i] = true
		}
	}
	isSync := e.isFunDeclSync(node, paramTypeIDs, retTypeID)
	funTyp := FunType{
		Params:         paramTypeIDs,
		Return:         retTypeID,
		Macro:          false,
		Sync:           isSync,
		NoescapeParams: noescapeParams,
		NoescapeReturn: fun.NoescapeReturn,
	}
	funTypeID := e.env.buildFunType(funTyp, node.ID, node.Span)
	bindName := fun.Name.Name
	if structName, methodName, ok := strings.Cut(fun.Name.Name, "."); ok {
		resolved, ok := e.resolveMethodBindName(node.ID, structName, methodName, fun.Name.Span)
		if !ok {
			return InvalidTypeID, TypeFailed
		}
		bindName = resolved
	}
	if !e.bind(node.ID, bindName, false, funTypeID, fun.Name.Span, -1) {
		return InvalidTypeID, TypeFailed
	}
	if fun.Extern {
		e.env.setNamedFunRef(node.ID, fun.ExternName)
	} else {
		e.env.setNamedFunRef(node.ID, e.declMangledName(node.ID, fun.Name.Name))
	}
	return funTypeID, TypeOK
}

func (e *Engine) checkMethodFieldCollision(nodeID ast.NodeID, fun ast.FunDecl) bool {
	structName, methodName, hasDot := strings.Cut(fun.Name.Name, ".")
	if !hasDot {
		return true
	}
	binding, ok := e.lookup(nodeID, structName, -1)
	if !ok {
		return true
	}
	structNode, ok := e.ast.Node(binding.Decl).Kind.(ast.Struct)
	if !ok {
		return true
	}
	for _, fieldNodeID := range structNode.Fields {
		fieldNode := base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind)
		if fieldNode.Name.Name == methodName {
			e.diag(fun.Name.Span, "method name conflicts with field: %s.%s",
				structName, methodName)
			return false
		}
	}
	return true
}
