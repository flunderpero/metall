package types

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

const mutableRefFlag = 1 << 62

type Binding struct {
	*ast.Binding
	TypeID TypeID
	Mut    bool
}

type TypeStatus int

const (
	TypeOK TypeStatus = iota + 1
	TypeInProgress
	TypeFailed
	TypeDepFailed
)

func (s TypeStatus) Failed() bool {
	return s == TypeFailed || s == TypeDepFailed
}

func (s TypeStatus) String() string {
	switch s {
	case TypeOK:
		return "<ok>"
	case TypeInProgress:
		return "<in progress>"
	case TypeFailed:
		return "<failed>"
	case TypeDepFailed:
		return "<failed dependency>"
	default:
		panic(base.Errorf("unknown type status: %d", s))
	}
}

type cachedType struct {
	Type   *Type
	Status TypeStatus
}

type refTypeCacheKey struct {
	TypeID
	Mut bool
}

type arrayTypeCacheKey struct {
	Elem TypeID
	Len  int64
}

type TypeRegistry struct {
	types      map[TypeID]*cachedType
	refTypes   map[refTypeCacheKey]*cachedType
	arrayTypes map[arrayTypeCacheKey]*cachedType
	sliceTypes map[TypeID]*cachedType
	funTypes   map[string]*cachedType
	nextID     TypeID
}

type TypeEnv struct {
	parent             *TypeEnv
	ast                *ast.AST
	scopeGraph         *ast.ScopeGraph
	reg                *TypeRegistry
	bindings           map[ast.BindingID]*Binding
	nodes              map[ast.NodeID]*cachedType
	namedFunRef        map[ast.NodeID]string
	methodCallReceiver map[ast.NodeID]ast.NodeID
}

func NewRootEnv(a *ast.AST, g *ast.ScopeGraph) *TypeEnv {
	return &TypeEnv{
		parent:     nil,
		ast:        a,
		scopeGraph: g,
		reg: &TypeRegistry{
			types:      map[TypeID]*cachedType{},
			refTypes:   map[refTypeCacheKey]*cachedType{},
			arrayTypes: map[arrayTypeCacheKey]*cachedType{},
			sliceTypes: map[TypeID]*cachedType{},
			funTypes:   map[string]*cachedType{},
			nextID:     1,
		},
		bindings:           map[ast.BindingID]*Binding{},
		nodes:              map[ast.NodeID]*cachedType{},
		namedFunRef:        map[ast.NodeID]string{},
		methodCallReceiver: map[ast.NodeID]ast.NodeID{},
	}
}

func (e *TypeEnv) NewChildEnv() *TypeEnv {
	return &TypeEnv{
		parent:             e,
		ast:                e.ast,
		scopeGraph:         e.scopeGraph,
		reg:                e.reg,
		bindings:           map[ast.BindingID]*Binding{},
		nodes:              map[ast.NodeID]*cachedType{},
		namedFunRef:        map[ast.NodeID]string{},
		methodCallReceiver: map[ast.NodeID]ast.NodeID{},
	}
}

func (e *TypeEnv) IsRoot() bool {
	return e.parent == nil
}

func (e *TypeEnv) TypeOfNode(id ast.NodeID) *Type {
	cached, ok := e.nodes[id]
	if ok {
		return cached.Type
	}
	if e.parent != nil {
		return e.parent.TypeOfNode(id)
	}
	panic(base.Errorf("type not found for %s", e.ast.Debug(id, false, 0)))
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
	if ok {
		return name, true
	}
	if e.parent != nil {
		return e.parent.NamedFunRef(id)
	}
	return "", false
}

func (e *TypeEnv) MethodCallReceiver(callID ast.NodeID) (ast.NodeID, bool) {
	target, ok := e.methodCallReceiver[callID]
	if ok {
		return target, true
	}
	if e.parent != nil {
		return e.parent.MethodCallReceiver(callID)
	}
	return 0, false
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
	case TypeParamType:
		return "<typeparam>"
	case AllocatorType:
		return fmt.Sprintf("alloc(%s)", kind.Impl)
	default:
		panic(base.Errorf("unknown type kind: %T", kind))
	}
}

func (e *TypeEnv) Lookup(nodeID ast.NodeID, name string) (*Binding, bool) {
	scope := e.scopeGraph.NodeScope(nodeID)
	scopeBinding, _, ok := scope.Lookup(name)
	if !ok {
		return nil, false
	}
	b, ok := e.bindings[scopeBinding.ID]
	if ok {
		return b, true
	}
	if e.parent != nil {
		return e.parent.Lookup(nodeID, name)
	}
	return nil, false
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

func (e *TypeEnv) bind(decl ast.NodeID, name string, mut bool, typeID TypeID) bool {
	scope := e.scopeGraph.NodeScope(decl)
	b, isNew := scope.Bind(name, decl)
	e.bindings[b.ID] = &Binding{b, typeID, mut}
	return isNew
}

func (e *TypeEnv) cachedNodeType(nodeID ast.NodeID) (*cachedType, bool) {
	cached, ok := e.nodes[nodeID]
	return cached, ok
}

func (e *TypeEnv) setNodeType(nodeID ast.NodeID, cached *cachedType) {
	e.nodes[nodeID] = cached
}

func (e *TypeEnv) cachedTypeInfo(typeID TypeID) (*cachedType, bool) {
	cached, ok := e.reg.types[typeID]
	return cached, ok
}

func (e *TypeEnv) cachedFunType(key string) (*cachedType, bool) {
	cached, ok := e.reg.funTypes[key]
	return cached, ok
}

func (e *TypeEnv) cacheFunType(key string, typeID TypeID) {
	e.reg.funTypes[key] = e.reg.types[typeID]
}

func (e *TypeEnv) setNamedFunRef(nodeID ast.NodeID, name string) {
	e.namedFunRef[nodeID] = name
}

func (e *TypeEnv) copyNamedFunRef(dst ast.NodeID, src ast.NodeID) {
	name, ok := e.NamedFunRef(src)
	if !ok {
		panic(base.Errorf("named fun ref not found for %s", src))
	}
	e.namedFunRef[dst] = name
}

func (e *TypeEnv) isNamedFun(nodeID ast.NodeID) bool {
	_, ok := e.namedFunRef[nodeID]
	if ok {
		return true
	}
	if e.parent != nil {
		return e.parent.isNamedFun(nodeID)
	}
	return false
}

func (e *TypeEnv) setMethodCallReceiver(callID ast.NodeID, targetID ast.NodeID) {
	e.methodCallReceiver[callID] = targetID
}

func funTypeCacheKey(typ FunType) string {
	return fmt.Sprintf("fun:%v:%v", typ.Params, typ.Return)
}
