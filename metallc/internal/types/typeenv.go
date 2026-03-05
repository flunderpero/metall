package types

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type typeRegistry struct {
	types      map[TypeID]*cachedType
	refTypes   map[refTypeCacheKey]*cachedType
	arrayTypes map[arrayTypeCacheKey]*cachedType
	sliceTypes map[TypeID]*cachedType
	funTypes   map[string]*cachedType
	nextID     TypeID
}

type TypeEnv struct {
	ast                *ast.AST
	ScopeGraph         *ScopeGraph
	reg                *typeRegistry
	nodes              map[ast.NodeID]*cachedType
	namedFunRef        map[ast.NodeID]string
	methodCallReceiver map[ast.NodeID]ast.NodeID
	funs               []ast.NodeID
	structs            []ast.NodeID
}

func NewRootEnv(a *ast.AST) *TypeEnv {
	return &TypeEnv{ //nolint:exhaustruct
		ast:        a,
		ScopeGraph: NewScopeGraph(),
		reg: &typeRegistry{
			types:      map[TypeID]*cachedType{},
			refTypes:   map[refTypeCacheKey]*cachedType{},
			arrayTypes: map[arrayTypeCacheKey]*cachedType{},
			sliceTypes: map[TypeID]*cachedType{},
			funTypes:   map[string]*cachedType{},
			nextID:     1,
		},
		nodes:              map[ast.NodeID]*cachedType{},
		namedFunRef:        map[ast.NodeID]string{},
		methodCallReceiver: map[ast.NodeID]ast.NodeID{},
	}
}

func (e *TypeEnv) AST() *ast.AST {
	return e.ast
}

func (e *TypeEnv) TypeOfNode(id ast.NodeID) *Type {
	cached, ok := e.nodes[id]
	if !ok {
		panic(base.Errorf("type not found for %s", e.ast.Debug(id, false, 0)))
	}
	return cached.Type
}

func (e *TypeEnv) Type(id TypeID) *Type {
	cached, ok := e.reg.types[id]
	if !ok {
		panic(base.Errorf("type %s not found", id))
	}
	return cached.Type
}

func (e *TypeEnv) DeclNode(typeID TypeID) ast.NodeID {
	if cached, ok := e.reg.types[typeID]; ok {
		return cached.Type.NodeID
	}
	return 0
}

func (e *TypeEnv) NamedFunRef(id ast.NodeID) (string, bool) {
	name, ok := e.namedFunRef[id]
	return name, ok
}

func (e *TypeEnv) MethodCallReceiver(callID ast.NodeID) (ast.NodeID, bool) {
	target, ok := e.methodCallReceiver[callID]
	return target, ok
}

func (e *TypeEnv) NodeScope(id ast.NodeID) *Scope {
	return e.ScopeGraph.NodeScope(id)
}

func (e *TypeEnv) TypeDisplay(typeID TypeID) string {
	if typeID == InvalidTypeID {
		return "<invalid>"
	}
	cached, ok := e.reg.types[typeID]
	if !ok {
		panic(base.Errorf("type %s not found", typeID))
	}
	if cached.Status != TypeOK {
		return cached.Status.String()
	}
	switch kind := cached.Type.Kind.(type) {
	case IntType:
		return kind.Name
	case BoolType:
		return "Bool"
	case VoidType:
		return "void"
	case RefType:
		if kind.Mut {
			return fmt.Sprintf("&mut %s", e.TypeDisplay(kind.Type))
		}
		return fmt.Sprintf("&%s", e.TypeDisplay(kind.Type))
	case FunType:
		var sb strings.Builder
		sb.WriteString("fun(")
		for i, paramTypeID := range kind.Params {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(e.TypeDisplay(paramTypeID))
		}
		sb.WriteString(") ")
		sb.WriteString(e.TypeDisplay(kind.Return))
		return sb.String()
	case StructType:
		return kind.Name
	case ArrayType:
		return fmt.Sprintf("[%s %d]", e.TypeDisplay(kind.Elem), kind.Len)
	case SliceType:
		return fmt.Sprintf("[]%s", e.TypeDisplay(kind.Elem))
	case AllocatorType:
		return fmt.Sprintf("alloc(%s)", kind.Impl)
	default:
		panic(base.Errorf("unknown type kind: %T", kind))
	}
}

func (e *TypeEnv) IterTypes(f func(*Type, TypeStatus) bool) {
	for _, cached := range e.reg.types {
		if !f(cached.Type, cached.Status) {
			return
		}
	}
}

func (e *TypeEnv) newType(kind TypeKind, nodeID ast.NodeID, span base.Span, status TypeStatus) TypeID {
	newTypeID := e.reg.nextID
	e.reg.nextID++
	e.newTypeWithID(newTypeID, kind, nodeID, span, status)
	return newTypeID
}

func (e *TypeEnv) newTypeWithID(
	typeID TypeID, kind TypeKind, nodeID ast.NodeID, span base.Span, status TypeStatus,
) {
	if cached, ok := e.nodes[nodeID]; nodeID != 0 && ok {
		panic(base.Errorf("type already set for %s: %s", e.ast.Debug(nodeID, false, 0), cached.Type.ID))
	}
	typ := &Type{ID: typeID, NodeID: nodeID, Span: span, Kind: kind}
	cached := &cachedType{Type: typ, Status: status}
	e.reg.types[typeID] = cached
	e.nodes[nodeID] = cached
}

func (e *TypeEnv) buildRefType(nodeID ast.NodeID, innerTypeID TypeID, mut bool, span base.Span) TypeID {
	cacheKey := refTypeCacheKey{innerTypeID, mut}
	if cached, ok := e.reg.refTypes[cacheKey]; ok {
		return cached.Type.ID
	}
	if !mut {
		refTypeID := e.newType(RefType{innerTypeID, mut}, nodeID, span, TypeOK)
		e.reg.refTypes[cacheKey] = e.reg.types[refTypeID]
		return refTypeID
	}
	immutableRefTypID := e.buildRefType(0, innerTypeID, false, span)
	refTypeID := immutableRefTypID | mutableRefFlag
	e.newTypeWithID(refTypeID, RefType{innerTypeID, true}, nodeID, span, TypeOK)
	e.reg.refTypes[cacheKey] = e.reg.types[refTypeID]
	return refTypeID
}

func (e *TypeEnv) buildArrayType(elemTypeID TypeID, length int64, nodeID ast.NodeID, span base.Span) TypeID {
	cacheKey := arrayTypeCacheKey{elemTypeID, length}
	if cached, ok := e.reg.arrayTypes[cacheKey]; ok {
		return cached.Type.ID
	}
	typeID := e.newType(ArrayType{Elem: elemTypeID, Len: length}, nodeID, span, TypeOK)
	e.reg.arrayTypes[cacheKey] = e.reg.types[typeID]
	return typeID
}

func (e *TypeEnv) buildSliceType(elemTypeID TypeID, nodeID ast.NodeID, span base.Span) TypeID {
	if cached, ok := e.reg.sliceTypes[elemTypeID]; ok {
		return cached.Type.ID
	}
	typeID := e.newType(SliceType{Elem: elemTypeID}, nodeID, span, TypeOK)
	e.reg.sliceTypes[elemTypeID] = e.reg.types[typeID]
	return typeID
}

func funTypeCacheKey(typ FunType) string {
	return fmt.Sprintf("fun:%v:%v", typ.Params, typ.Return)
}

type FunWork struct {
	NodeID ast.NodeID
	Name   string
	IsMain bool
	Env    *TypeEnv
}

type StructWork struct {
	NodeID ast.NodeID
	Env    *TypeEnv
}
