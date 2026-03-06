package types

import (
	"fmt"
	"math/big"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

const InvalidTypeID = TypeID(0)

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

type RefType struct {
	Type TypeID
	Mut  bool
}

func (RefType) isTypeKind() {}

type FunType struct {
	Params []TypeID
	Return TypeID
}

func (FunType) isTypeKind() {}

type StructField struct {
	Name string
	Type TypeID
	Mut  bool
}

type StructType struct {
	Name   string
	Fields []StructField
}

func (StructType) isTypeKind() {}

type ArrayType struct {
	Elem TypeID
	Len  int64
}

func (ArrayType) isTypeKind() {}

type SliceType struct {
	Elem TypeID
}

func (SliceType) isTypeKind() {}

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
}
