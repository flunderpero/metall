package ast

import (
	_ "embed"
	"slices"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

//go:embed prelude.met
var stdlibPreludeSrc string

const minimalPrelude = `
struct Void_ {}
struct Arena_ {}
struct Bool_ {}
struct I8_ {}
struct I16_ {}
struct I32_ {}
struct Int_ {}
struct U8_ {}
struct U16_ {}
struct U32_ {}
struct U64_ {}
struct Rune_ {}
struct Str_ { data []U8 }
struct DebugIntern_ {}
struct None_ {}
union Option_<T> = T | None
struct Err_ { msg Str }
union Result_<T> = T | Err
struct CStr_ { data []U8 }
struct LibCIntern_ {}
fun panic_(s Str) void {}
fun Arena.new<T>(self Arena, value T) &T { value }
fun Arena.new_mut<T>(self Arena, value T) &mut T { value }
fun Arena.slice<T>(self Arena, len Int, default T) []T { default }
fun Arena.slice_mut<T>(self Arena, len Int, default T) []mut T { default }
fun Arena.slice_uninit<T>(self Arena, len Int) []T { len }
fun Arena.slice_uninit_mut<T>(self Arena, len Int) []mut T { len }
fun DebugIntern.print_str(s Str) void {}
fun DebugIntern.print_int(n Int) void {}
fun DebugIntern.print_uint(n U64) void {}
fun DebugIntern.print_bool(b Bool) void {}
fun Rune.to_u32(r Rune) U32 { return 1 }
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
fun LibCIntern.errno() I32 {}
fun LibCIntern.reset_errno() void {}
fun LibCIntern.fopen(filename CStr, mode CStr) U64 {}
fun LibCIntern.strerror(errnum I32) Str {}
fun LibCIntern.fwrite(fd U64, data []U8) Int {}
fun LibCIntern.fread(fd U64, buf []U8) Int {}
fun LibCIntern.fclose(fd U64) I32 {}
fun LibCIntern.write(fd I32, data []U8) Int {}
`

const PreludeFirstID = NodeID(1_000_000_000)

func IsPreludeNode(id NodeID) bool {
	return id >= PreludeFirstID
}

// PreludeAST parses the minimal prelude (built-in types and extern function
// stubs) and, when minimal is false, also the stdlib prelude (prelude.met).
func PreludeAST(minimal bool) (*AST, NodeID) {
	source := base.NewSource("prelude", "", false, []rune(minimalPrelude))
	tokens := token.Lex(source)
	a := NewAST(PreludeFirstID)
	parser := NewParser(tokens, a)
	moduleID, ok := parser.ParseModule()
	if !ok || len(parser.Diagnostics) > 0 {
		panic("failed to parse prelude: " + parser.Diagnostics.Error())
	}
	updateMinimalPrelude(a)
	if !minimal {
		stdlibSource := base.NewSource("prelude.met", "", false, []rune(stdlibPreludeSrc))
		stdlibTokens := token.Lex(stdlibSource)
		stdlibParser := NewParser(stdlibTokens, a)
		if _, ok := stdlibParser.ParseModule(); !ok || len(stdlibParser.Diagnostics) > 0 {
			panic("failed to parse stdlib prelude: " + stdlibParser.Diagnostics.Error())
		}
	}

	return a, moduleID
}

func updateMinimalPrelude(a *AST) {
	a.Iter(func(id NodeID) bool {
		node := a.Node(id)
		switch kind := node.Kind.(type) {
		case Struct:
			if kind.Name.Name == "Void_" {
				kind.Name.Name = "void"
				node.Kind = kind
			} else if s, ok := strings.CutSuffix(kind.Name.Name, "_"); ok {
				kind.Name.Name = s
				node.Kind = kind
			}
			if slices.Contains([]string{"None", "Err"}, kind.Name.Name) {
				return true
			}
			kind.Extern = true
			node.Kind = kind
		case Union:
			if s, ok := strings.CutSuffix(kind.Name.Name, "_"); ok {
				kind.Name.Name = s
				node.Kind = kind
			}
		case Fun:
			if s, ok := strings.CutSuffix(kind.Name.Name, "_"); ok {
				kind.Name.Name = s
			}
			kind.Extern = true
			node.Kind = kind
		case SimpleType:
			if s, ok := strings.CutSuffix(kind.Name.Name, "_"); ok {
				kind.Name.Name = s
				node.Kind = kind
			}
		}
		return true
	})
}
