package types

import (
	"fmt"
	"math/big"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

const InvalidTypeID = TypeID(0)

// DeferredTypeID is a sentinel used during generic type inference to mark
// arguments whose type-checking was deferred (e.g. function literals with
// inferred types that depend on other arguments being resolved first).
const DeferredTypeID = TypeID(^uint64(0))

type TypeID uint64

func (id TypeID) String() string {
	return fmt.Sprintf("t%d", id)
}

type Type struct {
	ID     TypeID
	NodeID ast.NodeID
	Span   base.Span
	Kind   TypeKind
}

type TypeKind interface {
	isTypeKind()
}

type IntType struct {
	Name   string
	Signed bool
	Bits   int
	Min    *big.Int
	Max    *big.Int
}

func (IntType) isTypeKind() {}

type BoolType struct{}

func (BoolType) isTypeKind() {}

type VoidType struct{}

func (VoidType) isTypeKind() {}

type NeverType struct{}

func (NeverType) isTypeKind() {}

type RefType struct {
	Type TypeID
	Mut  bool
}

func (RefType) isTypeKind() {}

type FunType struct {
	Params         []TypeID
	Return         TypeID
	Macro          bool
	Sync           bool
	NoescapeParams []bool // nil if no params are noescape
	NoescapeReturn bool
}

func (f FunType) IsNoescape(paramIdx int) bool {
	return paramIdx < len(f.NoescapeParams) && f.NoescapeParams[paramIdx]
}

func (FunType) isTypeKind() {}

// ImplicitTypeArgs returns the type arguments that parameterize a type.
// For structs/unions these are explicit TypeArgs; for slices/arrays it's the element type.
func ImplicitTypeArgs(kind TypeKind) ([]TypeID, bool) {
	switch k := kind.(type) {
	case StructType:
		return k.TypeArgs, true
	case UnionType:
		return k.TypeArgs, true
	case SliceType:
		return []TypeID{k.Elem}, true
	default:
		return nil, false
	}
}

type StructField struct {
	Name string
	Type TypeID
	Pub  bool
}

type StructType struct {
	Name     string
	Fields   []StructField
	TypeArgs []TypeID
}

func (StructType) isTypeKind() {}

type ArrayType struct {
	Elem TypeID
	Len  int64
}

func (ArrayType) isTypeKind() {}

type SliceType struct {
	Elem TypeID
	Mut  bool
}

func (SliceType) isTypeKind() {}

type UnionType struct {
	Name     string
	Variants []TypeID
	TypeArgs []TypeID
}

func (UnionType) isTypeKind() {}

type TypeParamType struct {
	Shape   *TypeID // nil = unconstrained
	Default *TypeID // nil = no default
}

func (TypeParamType) isTypeKind() {}

type ShapeType struct {
	Name     string // namespaced name (e.g. "test.HasFields")
	DeclName string // declared local name (e.g. "HasFields"), used by typeName
	Fields   []StructField
	TypeArgs []TypeID
}

func (ShapeType) isTypeKind() {}

type ModuleType struct {
	Name  string
	Macro bool
}

func (ModuleType) isTypeKind() {}

type AllocatorImpl int

const (
	AllocatorArena AllocatorImpl = iota + 1
)

func (a AllocatorImpl) String() string {
	switch a {
	case AllocatorArena:
		return "Arena"
	default:
		panic(base.Errorf("unknown allocator impl: %d", a))
	}
}

type AllocatorType struct {
	Impl AllocatorImpl
}

func (AllocatorType) isTypeKind() {}

//nolint:gochecknoglobals
var intTypes = []IntType{
	{"I8", true, 8, big.NewInt(-128), big.NewInt(127)},
	{"I16", true, 16, big.NewInt(-32768), big.NewInt(32767)},
	{"I32", true, 32, big.NewInt(-2147483648), big.NewInt(2147483647)},
	{"Int", true, 64, big.NewInt(-9223372036854775808), big.NewInt(9223372036854775807)},
	{"U8", false, 8, big.NewInt(0), big.NewInt(255)},
	{"U16", false, 16, big.NewInt(0), big.NewInt(65535)},
	{"U32", false, 32, big.NewInt(0), big.NewInt(4294967295)},
	{"U64", false, 64, big.NewInt(0), new(big.Int).SetUint64(18446744073709551615)},
	{"Rune", false, 32, big.NewInt(0), big.NewInt(4294967295)},
}
