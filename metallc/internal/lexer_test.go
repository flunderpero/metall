package internal

import (
	"fmt"
	"testing"

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
		{"parens", "()", []want{{TLParen, "", "1:1"}, {TRParen, "", "1:2"}}},
		{"curly", "{}", []want{{TLCurly, "", "1:1"}, {TRCurly, "", "1:2"}}},
		{"eq", "=", []want{{TEq, "", "1:1"}}},
		{"amp", "&", []want{{TAmp, "", "1:1"}}},
		{"star", "*", []want{{TStar, "", "1:1"}}},
		{"number (int)", `123`, []want{{TNumber, "123", "1:1-1:3"}}},
		{"string", `"ride"`, []want{{TString, "ride", "1:1-1:6"}}},
		{"ident", `foo`, []want{{TIdent, "foo", "1:1-1:3"}}},
		{"type ident", `Foo`, []want{{TTypeIdent, "Foo", "1:1-1:3"}}},
		{"fun", `fun`, []want{{TFun, "", "1:1-1:3"}}},
		{"void", `void`, []want{{TVoid, "", "1:1-1:4"}}},
		{"mut", `mut`, []want{{TMut, "", "1:1-1:3"}}},
		{"let", `let`, []want{{TLet, "", "1:1-1:3"}}},
		{"whitespace is ignored", " ( \n \t)\r", []want{{TLParen, "", "1:2"}, {TRParen, "", "2:3"}}},
	}

	assert := require.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := NewSource("test.met", []rune(tt.src))
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
