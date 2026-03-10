package ast

import (
	_ "embed"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

const minimalPrelude = `
struct Void________ {}
struct Arena_______ {}
struct Bool {}
struct I8 {}
struct I16 {}
struct I32 {}
struct Int {}
struct U8 {}
struct U16 {}
struct U32 {}
struct U64 {}
struct Str { data []U8 }
fun print_str(s Str) void {}
fun print_int(n Int) void {}
fun print_uint(n U64) void {}
fun print_bool(b Bool) void {}
fun I8.to_i16(self I8) I16 { return 1 }
fun I8.to_i32(self I8) I32 { return 1 }
fun I8.to_int(self I8) Int { return 1 }
fun I16.to_i32(self I16) I32 { return 1 }
fun I16.to_int(self I16) Int { return 1 }
fun I32.to_int(self I32) Int { return 1 }
fun U8.to_u16(self U8) U16 { return 1 }
fun U8.to_u32(self U8) U32 { return 1 }
fun U8.to_u64(self U8) U64 { return 1 }
fun U16.to_u32(self U16) U32 { return 1 }
fun U16.to_u64(self U16) U64 { return 1 }
fun U32.to_u64(self U32) U64 { return 1 }
fun U8.to_i16(self U8) I16 { return 1 }
fun U8.to_i32(self U8) I32 { return 1 }
fun U8.to_int(self U8) Int { return 1 }
fun U16.to_i32(self U16) I32 { return 1 }
fun U16.to_int(self U16) Int { return 1 }
fun U32.to_int(self U32) Int { return 1 }
fun I16.to_i8_wrapping(self I16) I8 { return 1 }
fun I16.to_i8_clamped(self I16) I8 { return 1 }
fun I32.to_i8_wrapping(self I32) I8 { return 1 }
fun I32.to_i8_clamped(self I32) I8 { return 1 }
fun Int.to_i8_wrapping(self Int) I8 { return 1 }
fun Int.to_i8_clamped(self Int) I8 { return 1 }
fun I32.to_i16_wrapping(self I32) I16 { return 1 }
fun I32.to_i16_clamped(self I32) I16 { return 1 }
fun Int.to_i16_wrapping(self Int) I16 { return 1 }
fun Int.to_i16_clamped(self Int) I16 { return 1 }
fun Int.to_i32_wrapping(self Int) I32 { return 1 }
fun Int.to_i32_clamped(self Int) I32 { return 1 }
fun U16.to_u8_wrapping(self U16) U8 { return 1 }
fun U16.to_u8_clamped(self U16) U8 { return 1 }
fun U32.to_u8_wrapping(self U32) U8 { return 1 }
fun U32.to_u8_clamped(self U32) U8 { return 1 }
fun U64.to_u8_wrapping(self U64) U8 { return 1 }
fun U64.to_u8_clamped(self U64) U8 { return 1 }
fun U32.to_u16_wrapping(self U32) U16 { return 1 }
fun U32.to_u16_clamped(self U32) U16 { return 1 }
fun U64.to_u16_wrapping(self U64) U16 { return 1 }
fun U64.to_u16_clamped(self U64) U16 { return 1 }
fun U64.to_u32_wrapping(self U64) U32 { return 1 }
fun U64.to_u32_clamped(self U64) U32 { return 1 }
fun U8.to_i8_wrapping(self U8) I8 { return 1 }
fun U8.to_i8_clamped(self U8) I8 { return 1 }
fun U16.to_i16_wrapping(self U16) I16 { return 1 }
fun U16.to_i16_clamped(self U16) I16 { return 1 }
fun U16.to_i8_wrapping(self U16) I8 { return 1 }
fun U16.to_i8_clamped(self U16) I8 { return 1 }
fun U32.to_i32_wrapping(self U32) I32 { return 1 }
fun U32.to_i32_clamped(self U32) I32 { return 1 }
fun U32.to_i16_wrapping(self U32) I16 { return 1 }
fun U32.to_i16_clamped(self U32) I16 { return 1 }
fun U32.to_i8_wrapping(self U32) I8 { return 1 }
fun U32.to_i8_clamped(self U32) I8 { return 1 }
fun U64.to_int_wrapping(self U64) Int { return 1 }
fun U64.to_int_clamped(self U64) Int { return 1 }
fun U64.to_i32_wrapping(self U64) I32 { return 1 }
fun U64.to_i32_clamped(self U64) I32 { return 1 }
fun I8.to_u8_wrapping(self I8) U8 { return 1 }
fun I8.to_u8_clamped(self I8) U8 { return 1 }
fun I8.to_u16_wrapping(self I8) U16 { return 1 }
fun I8.to_u16_clamped(self I8) U16 { return 1 }
fun I8.to_u32_wrapping(self I8) U32 { return 1 }
fun I8.to_u32_clamped(self I8) U32 { return 1 }
fun I8.to_u64_wrapping(self I8) U64 { return 1 }
fun I8.to_u64_clamped(self I8) U64 { return 1 }
fun I16.to_u8_wrapping(self I16) U8 { return 1 }
fun I16.to_u8_clamped(self I16) U8 { return 1 }
fun I16.to_u16_wrapping(self I16) U16 { return 1 }
fun I16.to_u16_clamped(self I16) U16 { return 1 }
fun I16.to_u32_wrapping(self I16) U32 { return 1 }
fun I16.to_u32_clamped(self I16) U32 { return 1 }
fun I16.to_u64_wrapping(self I16) U64 { return 1 }
fun I16.to_u64_clamped(self I16) U64 { return 1 }
fun I32.to_u8_wrapping(self I32) U8 { return 1 }
fun I32.to_u8_clamped(self I32) U8 { return 1 }
fun I32.to_u16_wrapping(self I32) U16 { return 1 }
fun I32.to_u16_clamped(self I32) U16 { return 1 }
fun I32.to_u32_wrapping(self I32) U32 { return 1 }
fun I32.to_u32_clamped(self I32) U32 { return 1 }
fun I32.to_u64_wrapping(self I32) U64 { return 1 }
fun I32.to_u64_clamped(self I32) U64 { return 1 }
fun Int.to_u8_wrapping(self Int) U8 { return 1 }
fun Int.to_u8_clamped(self Int) U8 { return 1 }
fun Int.to_u16_wrapping(self Int) U16 { return 1 }
fun Int.to_u16_clamped(self Int) U16 { return 1 }
fun Int.to_u32_wrapping(self Int) U32 { return 1 }
fun Int.to_u32_clamped(self Int) U32 { return 1 }
fun Int.to_u64_wrapping(self Int) U64 { return 1 }
fun Int.to_u64_clamped(self Int) U64 { return 1 }
`

//go:embed prelude.met
var fullPreludeSrc string

const PreludeFirstID = NodeID(1_000_000_000)

var preludeRenames = map[string]string{ //nolint:gochecknoglobals
	"Void________": "void",
	"Arena_______": "Arena",
}

func PreludeAST(minimal bool) (*AST, NodeID) {
	src := minimalPrelude
	if !minimal {
		src += fullPreludeSrc
	}
	source := base.NewSource("prelude", "", false, []rune(src))
	tokens := token.Lex(source)
	parser := NewParser(tokens, NewAST(PreludeFirstID))
	moduleID, ok := parser.ParseModule()
	if !ok || len(parser.Diagnostics) > 0 {
		panic("failed to parse prelude: " + parser.Diagnostics.Error())
	}
	preludeRenameKeywords(parser.AST)
	return parser.AST, moduleID
}

func preludeRenameKeywords(a *AST) {
	a.Iter(func(id NodeID) bool {
		node := a.Node(id)
		switch kind := node.Kind.(type) {
		case Struct:
			if renamed, ok := preludeRenames[kind.Name.Name]; ok {
				kind.Name.Name = renamed
				node.Kind = kind
			}
		case SimpleType:
			if renamed, ok := preludeRenames[kind.Name.Name]; ok {
				kind.Name.Name = renamed
				node.Kind = kind
			}
		}
		return true
	})
}
