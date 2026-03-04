package token

import (
	"fmt"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/stretchr/testify/require"
)

func TestLexer(t *testing.T) {
	t.Parallel()

	type want struct {
		kind TokenKind
		val  string
		pos  string
	}
	tests := []struct {
		name string
		src  string
		want []want
	}{
		{"parens", "()", []want{{LParen, "", "1:1"}, {RParen, "", "1:2"}}},
		{"curly", "{}", []want{{LCurly, "", "1:1"}, {RCurly, "", "1:2"}}},
		{"brackets", "[]", []want{{LBracket, "", "1:1"}, {RBracket, "", "1:2"}}},
		{
			"lbracket vs lbracketindex",
			"a[ [ ([ [[ {[",
			[]want{
				{Ident, "a", "1:1"},
				{LBracketIndex, "", "1:2"},
				{Whitespace, " ", "1:3"},

				{LBracket, "", "1:4"},
				{Whitespace, " ", "1:5"},

				{LParen, "", "1:6"},
				{LBracket, "", "1:7"},
				{Whitespace, " ", "1:8"},

				{LBracket, "", "1:9"},
				{LBracket, "", "1:10"},
				{Whitespace, " ", "1:11"},

				{LCurly, "", "1:12"},
				{LBracket, "", "1:13"},
			},
		},
		{"plus", "+", []want{{Plus, "", "1:1"}}},
		{"minus", "-", []want{{Minus, "", "1:1"}}},
		{"slash", "/", []want{{Slash, "", "1:1"}}},
		{"percent", "%", []want{{Percent, "", "1:1"}}},
		{"lt", "<", []want{{Lt, "", "1:1"}}},
		{"lte", "<=", []want{{Lte, "", "1:1-1:2"}}},
		{"gt", ">", []want{{Gt, "", "1:1"}}},
		{"gte", ">=", []want{{Gte, "", "1:1-1:2"}}},
		{"eq", "=", []want{{Eq, "", "1:1"}}},
		{"eqeq", "==", []want{{EqEq, "", "1:1-1:2"}}},
		{"neq", "!=", []want{{Neq, "", "1:1-1:2"}}},
		{"and", "and", []want{{And, "", "1:1-1:3"}}},
		{"or", "or", []want{{Or, "", "1:1-1:2"}}},
		{"not", "not", []want{{Not, "", "1:1-1:3"}}},
		{"amp", "&", []want{{Amp, "", "1:1"}}},
		{"star", "*", []want{{Star, "", "1:1"}}},
		{"number (int)", `123`, []want{{Number, "123", "1:1-1:3"}}},
		{"string", `"ride"`, []want{{String, "ride", "1:1-1:6"}}},
		{"ident", `x`, []want{{Ident, "x", "1:1"}}},
		{"type ident", `Foo`, []want{{TypeIdent, "Foo", "1:1-1:3"}}},
		{"alloc ident", "@myalloc", []want{{AllocatorIdent, "@myalloc", "1:1-1:8"}}},
		{"invalid alloc ident (too short)", "@", []want{{InvalidAllocatorIdent, "@", "1:1"}}},
		{"invalid alloc ident (uppercase)", "@Myalloc", []want{{InvalidAllocatorIdent, "@Myalloc", "1:1-1:8"}}},
		{"fun", `fun`, []want{{Fun, "", "1:1-1:3"}}},
		{"if", "if", []want{{If, "", "1:1-1:2"}}},
		{"else", "else", []want{{Else, "", "1:1-1:4"}}},
		{"true", "true", []want{{True, "", "1:1-1:4"}}},
		{"false", "false", []want{{False, "", "1:1-1:5"}}},
		{"void", `void`, []want{{Void, "", "1:1-1:4"}}},
		{"mut", `mut`, []want{{Mut, "", "1:1-1:3"}}},
		{"let", `let`, []want{{Let, "", "1:1-1:3"}}},
		{"struct", `struct`, []want{{Struct, "", "1:1-1:6"}}},
		{"for", `for`, []want{{For, "", "1:1-1:3"}}},
		{"break", `break`, []want{{Break, "", "1:1-1:5"}}},
		{"continue", `continue`, []want{{Continue, "", "1:1-1:8"}}},
		{"return", "return", []want{{Return, "", "1:1-1:6"}}},
		{"dot", ".", []want{{Dot, "", "1:1"}}},
		{"whitespace", " ( \n \t)\r", []want{
			{Whitespace, " ", "1:1"},
			{LParen, "", "1:2"},
			{Whitespace, " \n \t", "1:3-2:2"},
			{RParen, "", "2:3"},
			{Whitespace, "\r", "2:4"},
		}},
		{"single line comment", "-- comment", []want{{Comment, "-- comment", "1:1-1:10"}}},
		{
			"multi line comment",
			"--- multi\n    line\n    comment ---",
			[]want{{Comment, "--- multi\n    line\n    comment ---", "1:1-3:15"}},
		},
	}

	assert := require.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := base.NewSource("test.met", "test", true, []rune(tt.src))
			tokens := Lex(source)
			for i, want := range tt.want {
				if len(tokens) <= i {
					t.Fatalf("expected %d tokens, got %d", len(tt.want), len(tokens))
				}
				token := tokens[i]
				srow, scol := token.Span.StartPos()
				erow, ecol := token.Span.EndPos()
				var pos string
				if erow == srow && ecol == scol {
					pos = fmt.Sprintf("%d:%d", srow, scol)
				} else {
					pos = fmt.Sprintf("%d:%d-%d:%d", srow, scol, erow, ecol)
				}
				msg := fmt.Sprintf(" of token #%d: %s", i, token)
				assert.Equal(want.kind, token.Kind, "kind"+msg)
				assert.Equal(want.val, token.Value, "value"+msg)
				assert.Equal(want.pos, pos, "span"+msg)
			}
			assert.Len(tokens, len(tt.want))
		})
	}
}
