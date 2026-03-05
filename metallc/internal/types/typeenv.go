package types

import (
	"fmt"
	"maps"
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

type Builtins struct {
	Names     map[string]TypeID
	VoidType  TypeID
	BoolType  TypeID
	StrType   TypeID
	ArenaType TypeID
	IntType   TypeID
	IntTypes  map[string]TypeID
}

type TypeEnv struct {
	ast                *ast.AST
	ScopeGraph         *ScopeGraph
	builtins           *Builtins
	reg                *typeRegistry
	nodes              map[ast.NodeID]*cachedType
	namedFunRef        map[ast.NodeID]string
	methodCallReceiver map[ast.NodeID]ast.NodeID
	funs               []ast.NodeID
	structs            []ast.NodeID
}

func NewRootEnv(a *ast.AST) *TypeEnv {
	e := &TypeEnv{ //nolint:exhaustruct
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
	e.builtins = e.initBuiltins()
	return e
}

func (e *TypeEnv) AST() *ast.AST {
	return e.ast
}

func (e *TypeEnv) Builtins() *Builtins {
	return e.builtins
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

func (e *TypeEnv) IntTypeInfo(id TypeID) (IntTypeInfo, bool) {
	typ := e.Type(id)
	builtin, ok := typ.Kind.(BuiltInType)
	if !ok {
		return IntTypeInfo{}, false
	}
	info, ok := intTypeInfos[builtin.Name]
	return info, ok
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
	case BuiltInType:
		return kind.Name
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
		if typeID == e.builtins.StrType {
			return "Str"
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "struct %s(", kind.Name)
		for i, field := range kind.Fields {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%s %s", field.Name, e.TypeDisplay(field.Type))
		}
		sb.WriteString(")")
		return sb.String()
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

func (e *TypeEnv) initBuiltins() *Builtins {
	span := base.NewSpan(base.NewSource("builtin", "", false, []rune{}), 0, 0)
	voidType := e.newType(BuiltInType{"void"}, 0, span, TypeOK)
	boolType := e.newType(BuiltInType{"Bool"}, 0, span, TypeOK)
	arenaType := e.newType(AllocatorType{AllocatorArena}, 0, span, TypeOK)

	intTypeNames := []string{"I8", "I16", "I32", "Int", "U8", "U16", "U32", "U64"}
	intTypes := map[string]TypeID{}
	for _, name := range intTypeNames {
		intTypes[name] = e.newType(BuiltInType{name}, 0, span, TypeOK)
	}
	intType := intTypes["Int"]

	u8SliceType := e.buildSliceType(intTypes["U8"], 0, span)
	strType := e.newType(StructType{
		Name:   "Str",
		Fields: []StructField{{Name: "data", Type: u8SliceType, Mut: false}},
	}, 0, span, TypeOK)

	names := map[string]TypeID{
		"Str":   strType,
		"Bool":  boolType,
		"Arena": arenaType,
		"void":  voidType,
	}
	maps.Copy(names, intTypes)

	printStrFun := e.newBuiltinFun(FunType{[]TypeID{strType}, voidType}, span)
	printIntFun := e.newBuiltinFun(FunType{[]TypeID{intType}, voidType}, span)
	printUintFun := e.newBuiltinFun(FunType{[]TypeID{intTypes["U64"]}, voidType}, span)
	printBoolFun := e.newBuiltinFun(FunType{[]TypeID{boolType}, voidType}, span)
	names["print_str"] = printStrFun
	names["print_int"] = printIntFun
	names["print_uint"] = printUintFun
	names["print_bool"] = printBoolFun

	return &Builtins{
		Names:     names,
		VoidType:  voidType,
		BoolType:  boolType,
		StrType:   strType,
		ArenaType: arenaType,
		IntType:   intType,
		IntTypes:  intTypes,
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

func (e *TypeEnv) newBuiltinFun(funTyp FunType, span base.Span) TypeID {
	typeID := e.newType(funTyp, 0, span, TypeOK)
	cacheKey := funTypeCacheKey(funTyp)
	e.reg.funTypes[cacheKey] = e.reg.types[typeID]
	return typeID
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
