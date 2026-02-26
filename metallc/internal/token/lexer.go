package token

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/flunderpero/metall/metallc/internal/base"
)

type TokenKind int

const (
	Alloc TokenKind = iota + 1
	AllocIdent
	Amp
	Comma
	Dot
	Else
	EOF
	Eq
	False
	Fun
	Ident
	If
	InvalidAllocIdent
	LBracket
	LBracketIndex
	LCurly
	Let
	LParen
	Mut
	Number
	RBracket
	RCurly
	RParen
	Star
	String
	Struct
	True
	TypeIdent
	Unknown
	Void
)

var tokenKindNames = map[TokenKind]string{ //nolint:gochecknoglobals
	Alloc:             "<alloc>",
	AllocIdent:        "<allocator identifier>",
	Amp:               "&",
	Comma:             ",",
	Dot:               ".",
	Else:              "<else>",
	EOF:               "<EOF>",
	Eq:                "=",
	False:             "false",
	Fun:               "<fun>",
	Ident:             "<identifier>",
	If:                "<if>",
	InvalidAllocIdent: "<invalid allocation identifier>",
	LBracket:          "[",
	LBracketIndex:     "<[index>",
	LCurly:            "{",
	Let:               "<let>",
	LParen:            "(",
	Mut:               "<mut>",
	Number:            "<number>",
	RBracket:          "]",
	RCurly:            "}",
	RParen:            ")",
	Star:              "*",
	String:            "<string>",
	Struct:            "<struct>",
	True:              "true",
	TypeIdent:         "<type identifier>",
	Unknown:           "<unknown>",
	Void:              "<void>",
}

var simpleTokens = map[rune]TokenKind{ //nolint:gochecknoglobals
	'&': Amp,
	',': Comma,
	'.': Dot,
	'=': Eq,
	'{': LCurly,
	'(': LParen,
	']': RBracket,
	'}': RCurly,
	')': RParen,
	'*': Star,
}

var keywords = map[string]TokenKind{ //nolint:gochecknoglobals
	"alloc":  Alloc,
	"else":   Else,
	"false":  False,
	"fun":    Fun,
	"if":     If,
	"let":    Let,
	"mut":    Mut,
	"struct": Struct,
	"true":   True,
	"void":   Void,
}

func (k TokenKind) String() string {
	s, ok := tokenKindNames[k]
	if !ok {
		panic(base.Errorf("unknown token kind: %d", k))
	}
	return s
}

func PrettyPrintTokenKinds(kinds []TokenKind) string {
	var sb strings.Builder
	for i, kind := range kinds {
		if i > 0 {
			sb.WriteString(", ")
		}
		s := kind.String()
		if s[0] != '<' {
			sb.WriteString("'")
			sb.WriteString(s)
			sb.WriteString("'")
		} else {
			sb.WriteString(s)
		}
	}
	return sb.String()
}

type Token struct {
	Kind  TokenKind
	Value string
	Span  base.Span
}

func (t Token) String() string {
	return fmt.Sprintf("%s: %s", t.Span, t.Kind)
}

func lexToken(source *base.Source, idx int) Token { //nolint:funlen
	start := idx
	c := source.Content[idx]
	span := base.NewSpan(source, start, idx)
	idx += 1
	if kind, ok := simpleTokens[c]; ok {
		return Token{Kind: kind, Value: "", Span: span}
	}
	switch {
	case c == '"':
		value := []rune{}
		for idx < len(source.Content) {
			c := source.Content[idx]
			if c == '"' {
				span.End = idx
				return Token{String, string(value), span}
			}
			idx += 1
			value = append(value, c)
		}
		return Token{EOF, "", span}
	case c == '[':
		kind := LBracket
		if idx > 1 {
			prev := source.Content[idx-2]
			if unicode.IsLetter(prev) || prev == '_' || prev == ')' || prev == ']' {
				kind = LBracketIndex
			}
		}
		return Token{Kind: kind, Value: "", Span: span}
	case unicode.IsLetter(c), c == '@':
		value := []rune{c}
		for idx < len(source.Content) {
			c := source.Content[idx]
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
				break
			}
			idx += 1
			value = append(value, c)
		}
		span.End = idx - 1
		if kind, ok := keywords[string(value)]; ok {
			return Token{Kind: kind, Value: "", Span: span}
		}
		kind := TypeIdent
		if c == '@' {
			kind = AllocIdent
			if len(value) < 2 || !unicode.IsLower(value[1]) {
				kind = InvalidAllocIdent
			}
		} else if unicode.IsLower(c) {
			kind = Ident
		}
		return Token{kind, string(value), span}
	case unicode.IsDigit(c):
		value := []rune{c}
		for idx < len(source.Content) {
			c := source.Content[idx]
			if !unicode.IsDigit(c) {
				break
			}
			idx += 1
			value = append(value, c)
		}
		span.End = idx - 1
		return Token{Number, string(value), span}
	default:
		return Token{Unknown, string(c), span}
	}
}

func lexSkipWhitespace(source *base.Source, idx int) int {
	for idx < len(source.Content) {
		c := source.Content[idx]
		if !unicode.IsSpace(c) {
			return idx
		}
		idx += 1
	}
	return idx
}

func Lex(source *base.Source) []Token {
	tokens := []Token{}
	idx := 0
	for {
		idx = lexSkipWhitespace(source, idx)
		if idx >= len(source.Content) {
			break
		}
		token := lexToken(source, idx)
		tokens = append(tokens, token)
		idx = token.Span.End + 1
	}
	return tokens
}
