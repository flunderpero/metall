package types

import (
	"fmt"
	"slices"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

const (
	mutableRefFlag   TypeID = 1 << 62
	mutableSliceFlag TypeID = 1 << 61
)

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

type sliceTypeCacheKey struct {
	TypeID
	Mut bool
}

type arrayTypeCacheKey struct {
	Elem TypeID
	Len  int64
}

type TypeRegistry struct {
	types          map[TypeID]*cachedType
	typeParamTypes map[ast.NodeID]TypeID // TypeParam NodeID → TypeParamType TypeID
	refTypes       map[refTypeCacheKey]*cachedType
	arrayTypes     map[arrayTypeCacheKey]*cachedType
	sliceTypes     map[sliceTypeCacheKey]*cachedType
	funTypes       map[string]*cachedType
	genericOrigin  map[TypeID]TypeID // monomorphized type ID → generic type ID
	nextID         TypeID
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
	unionWraps         map[ast.NodeID]TypeID // nodeID → union TypeID (auto-wrap variant → union)
}

func NewRootEnv(a *ast.AST, g *ast.ScopeGraph) *TypeEnv {
	return &TypeEnv{
		parent:     nil,
		ast:        a,
		scopeGraph: g,
		reg: &TypeRegistry{
			types:          map[TypeID]*cachedType{},
			typeParamTypes: map[ast.NodeID]TypeID{},
			refTypes:       map[refTypeCacheKey]*cachedType{},
			arrayTypes:     map[arrayTypeCacheKey]*cachedType{},
			sliceTypes:     map[sliceTypeCacheKey]*cachedType{},
			funTypes:       map[string]*cachedType{},
			genericOrigin:  map[TypeID]TypeID{},
			nextID:         1,
		},
		bindings:           map[ast.BindingID]*Binding{},
		nodes:              map[ast.NodeID]*cachedType{},
		namedFunRef:        map[ast.NodeID]string{},
		methodCallReceiver: map[ast.NodeID]ast.NodeID{},
		unionWraps:         map[ast.NodeID]TypeID{},
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
		unionWraps:         map[ast.NodeID]TypeID{},
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

func (e *TypeEnv) GenericOrigin(typeID TypeID) (TypeID, bool) {
	origin, ok := e.reg.genericOrigin[typeID]
	return origin, ok
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

func (e *TypeEnv) TypeDisplay(typeID TypeID) string { //nolint:funlen
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
		astStruct := base.Cast[ast.Struct](e.ast.Node(cached.Type.NodeID).Kind)
		scope := e.scopeGraph.NodeScope(cached.Type.NodeID)
		return e.typeNameAndTypeArgsString(scope.NamespacedName(astStruct.Name.Name), kind.TypeArgs)
	case UnionType:
		astUnion := base.Cast[ast.Union](e.ast.Node(cached.Type.NodeID).Kind)
		scope := e.scopeGraph.NodeScope(cached.Type.NodeID)
		return e.typeNameAndTypeArgsString(scope.NamespacedName(astUnion.Name.Name), kind.TypeArgs)
	case ArrayType:
		return fmt.Sprintf("[%s %d]", e.TypeDisplay(kind.Elem), kind.Len)
	case SliceType:
		if kind.Mut {
			return fmt.Sprintf("[]mut %s", e.TypeDisplay(kind.Elem))
		}
		return fmt.Sprintf("[]%s", e.TypeDisplay(kind.Elem))
	case TypeParamType:
		astTypeParam := base.Cast[ast.TypeParam](e.ast.Node(cached.Type.NodeID).Kind)
		return astTypeParam.Name.Name
	case ShapeType:
		return kind.Name
	case AllocatorType:
		return fmt.Sprint(kind.Impl)
	case ModuleType:
		return kind.Name
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

func (e *TypeEnv) UnionWrap(nodeID ast.NodeID) (TypeID, bool) {
	id, ok := e.unionWraps[nodeID]
	if ok {
		return id, true
	}
	if e.parent != nil {
		return e.parent.UnionWrap(nodeID)
	}
	return 0, false
}

func (e *TypeEnv) recordUnionWrap(nodeID ast.NodeID, unionTypeID TypeID) {
	e.unionWraps[nodeID] = unionTypeID
}

func (e *TypeEnv) typeNameAndTypeArgsString(name string, typeArgs []TypeID) string {
	var sb strings.Builder
	sb.WriteString(name)
	if len(typeArgs) > 0 {
		sb.WriteString("<")
		for i, typeID := range typeArgs {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(e.TypeDisplay(typeID))
		}
		sb.WriteString(">")
	}
	return sb.String()
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

func (e *TypeEnv) buildSliceType(elemTypeID TypeID, mut bool, nodeID ast.NodeID, span base.Span) TypeID {
	cacheKey := sliceTypeCacheKey{elemTypeID, mut}
	if cached, ok := e.reg.sliceTypes[cacheKey]; ok {
		return cached.Type.ID
	}
	if !mut {
		typeID := e.newType(SliceType{Elem: elemTypeID, Mut: false}, nodeID, span, TypeOK)
		e.reg.sliceTypes[cacheKey] = e.reg.types[typeID]
		return typeID
	}
	immutableID := e.buildSliceType(elemTypeID, false, 0, span)
	mutableID := immutableID | mutableSliceFlag
	e.newTypeWithID(mutableID, SliceType{Elem: elemTypeID, Mut: true}, nodeID, span, TypeOK)
	e.reg.sliceTypes[cacheKey] = e.reg.types[mutableID]
	return mutableID
}

func (e *TypeEnv) bind(decl ast.NodeID, name string, mut bool, typeID TypeID) bool {
	scope := e.scopeGraph.NodeScope(decl)
	b, isNew := scope.Bind(name, decl)
	e.bindings[b.ID] = &Binding{b, typeID, mut}
	return isNew
}

func (e *TypeEnv) bindInScope(scope *ast.Scope, decl ast.NodeID, name string, typeID TypeID) bool {
	b, isNew := scope.Bind(name, decl)
	e.bindings[b.ID] = &Binding{b, typeID, false}
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

func (e *TypeEnv) isAssignableTo(got TypeID, expected TypeID) bool {
	if got == expected {
		return true
	}
	// A &mut T is assignable to &T (coerce by masking off the mutable flag).
	if got&mutableRefFlag != 0 && got&^mutableRefFlag == expected {
		return true
	}
	// A []mut T is assignable to []T (coerce by masking off the mutable slice flag).
	return got&mutableSliceFlag != 0 && got&^mutableSliceFlag == expected
}

func (e *TypeEnv) isIntType(typeID TypeID) bool {
	_, ok := e.Type(typeID).Kind.(IntType)
	return ok
}

func (e *TypeEnv) hasTypeParam(typeIDs []TypeID) bool {
	return slices.ContainsFunc(typeIDs, e.containsTypeParam)
}

func (e *TypeEnv) containsTypeParam(id TypeID) bool {
	typ := e.Type(id)
	switch kind := typ.Kind.(type) {
	case TypeParamType:
		return true
	case RefType:
		return e.containsTypeParam(kind.Type)
	case StructType:
		return e.hasTypeParam(kind.TypeArgs)
	case UnionType:
		return e.hasTypeParam(kind.TypeArgs)
	case ArrayType:
		return e.containsTypeParam(kind.Elem)
	case SliceType:
		return e.containsTypeParam(kind.Elem)
	case FunType:
		return e.hasTypeParam(kind.Params) || e.containsTypeParam(kind.Return)
	default:
		return false
	}
}

func (e *TypeEnv) isSafeUninitialized(typeID TypeID) bool {
	typ := e.Type(typeID)
	switch kind := typ.Kind.(type) {
	case IntType:
		return true
	case StructType:
		for _, field := range kind.Fields {
			if !e.isSafeUninitialized(field.Type) {
				return false
			}
		}
		return len(kind.Fields) > 0
	case ArrayType:
		return e.isSafeUninitialized(kind.Elem)
	default:
		return false
	}
}

func (e *TypeEnv) substituteType(srcTypeID, searchTypeID, replaceTypeID TypeID) TypeID {
	if srcTypeID == searchTypeID {
		return replaceTypeID
	}
	if refTyp, ok := e.Type(srcTypeID).Kind.(RefType); ok {
		inner := e.substituteType(refTyp.Type, searchTypeID, replaceTypeID)
		if inner != refTyp.Type {
			return e.buildRefType(0, inner, refTyp.Mut, base.Span{})
		}
	}
	return srcTypeID
}

func (e *TypeEnv) substituteFunType(funType FunType, searchTypeID, replaceTypeID TypeID) FunType {
	result := FunType{
		Params: make([]TypeID, len(funType.Params)),
		Return: e.substituteType(funType.Return, searchTypeID, replaceTypeID),
	}
	for i, p := range funType.Params {
		result.Params[i] = e.substituteType(p, searchTypeID, replaceTypeID)
	}
	return result
}

func (e *TypeEnv) typeName(typ *Type) string {
	switch kind := typ.Kind.(type) {
	case ModuleType:
		return kind.Name
	case StructType:
		return kind.Name
	case UnionType:
		return kind.Name
	case IntType:
		return kind.Name
	case BoolType:
		return "Bool"
	case AllocatorType:
		return "Arena"
	case TypeParamType:
		if kind.Shape != nil {
			return base.Cast[ShapeType](e.Type(*kind.Shape).Kind).DeclName
		}
		panic(base.Errorf("typeName: unconstrained type parameter"))
	default:
		panic(base.Errorf("typeName: unsupported type kind: %T", typ.Kind))
	}
}

func funTypeCacheKey(typ FunType) string {
	return fmt.Sprintf("fun:%v:%v", typ.Params, typ.Return)
}
