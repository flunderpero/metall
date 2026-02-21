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
		{"eq", "=", []want{{Eq, "", "1:1"}}},
		{"amp", "&", []want{{Amp, "", "1:1"}}},
		{"star", "*", []want{{Star, "", "1:1"}}},
		{"number (int)", `123`, []want{{Number, "123", "1:1-1:3"}}},
		{"string", `"ride"`, []want{{String, "ride", "1:1-1:6"}}},
		{"ident", `foo`, []want{{Ident, "foo", "1:1-1:3"}}},
		{"type ident", `Foo`, []want{{TypeIdent, "Foo", "1:1-1:3"}}},
		{"fun", `fun`, []want{{Fun, "", "1:1-1:3"}}},
		{"if", "if", []want{{If, "", "1:1-1:2"}}},
		{"else", "else", []want{{Else, "", "1:1-1:4"}}},
		{"true", "true", []want{{True, "", "1:1-1:4"}}},
		{"false", "false", []want{{False, "", "1:1-1:5"}}},
		{"void", `void`, []want{{Void, "", "1:1-1:4"}}},
		{"mut", `mut`, []want{{Mut, "", "1:1-1:3"}}},
		{"let", `let`, []want{{Let, "", "1:1-1:3"}}},
		{"whitespace is ignored", " ( \n \t)\r", []want{{LParen, "", "1:2"}, {RParen, "", "2:3"}}},
	}

	assert := require.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := Lex(source)
			for i, want := range tt.want {
				if len(tokens) <= i {
					t.Fatalf("expected %d tokens, got %d", len(tt.want), len(tokens))
				}
				token := tokens[i]
				srow, scol := token.Span.StartPos()
				erow, ecol := token.Span.EndPos()
				var wantPos string
				if erow == srow && ecol == scol {
					wantPos = fmt.Sprintf("%d:%d", srow, scol)
				} else {
					wantPos = fmt.Sprintf("%d:%d-%d:%d", srow, scol, erow, ecol)
				}
				msg := fmt.Sprintf(" of token #%d: %s", i, token)
				assert.Equal(want.kind, token.Kind, "kind"+msg)
				assert.Equal(want.val, token.Value, "value"+msg)
				assert.Equal(wantPos, want.pos, "span"+msg)
			}
			assert.Len(tokens, len(tt.want))
		})
	}
}
