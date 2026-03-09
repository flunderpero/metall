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
