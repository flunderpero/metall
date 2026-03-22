package ast

import (
	_ "embed"
	"slices"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

//go:embed prelude.met
var preludeSrc string

//go:embed prelude_min.met
var minPreludeSrc string

const PreludeFirstID = NodeID(1_000_000_000)

func IsPreludeNode(id NodeID) bool {
	return id >= PreludeFirstID
}

// PreludeAST parses the minimal prelude (prelude_min.met) with built-in types
// and extern function stubs) and, when minimal is false, also the
// stdlib prelude (prelude.met).
func PreludeAST(minimal bool) (*AST, NodeID) {
	source := base.NewSource("prelude", "", false, []rune(minPreludeSrc))
	tokens := token.Lex(source)
	a := NewAST(PreludeFirstID)
	parser := NewParser(tokens, a)
	moduleID, ok := parser.ParseModule()
	if !ok || len(parser.Diagnostics) > 0 {
		panic("failed to parse prelude: " + parser.Diagnostics.Error())
	}
	updateMinimalPrelude(a)
	if !minimal {
		stdlibSource := base.NewSource("prelude.met", "", false, []rune(preludeSrc))
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
