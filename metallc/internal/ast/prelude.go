package ast

import (
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

const Prelude = `
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
fun print_str(s Str) Void________ {}
fun print_int(n Int) Void________ {}
fun print_uint(n U64) Void________ {}
fun print_bool(b Bool) Void________ {}
`

const PreludeFirstID = NodeID(1_000_000_000)

var preludeRenames = map[string]string{ //nolint:gochecknoglobals
	"Void________": "void",
	"Arena_______": "Arena",
}

func PreludeAST() (*AST, NodeID) {
	source := base.NewSource("prelude", "", false, []rune(Prelude))
	tokens := token.Lex(source)
	parser := NewParser(tokens, PreludeFirstID)
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
