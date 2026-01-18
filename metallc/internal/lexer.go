package internal

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenKind int

const (
	TAmp TokenKind = iota + 1
	TComma
	TEOF
	TEq
	TFun
	TLet
	TIdent
	TLCurly
	TLParen
	TMut
	TNumber
	TRCurly
	TRParen
	TStar
	TString
	TUnknown
	TTypeIdent
	TVoid
)

var tokenKindNames = map[TokenKind]string{ //nolint:gochecknoglobals
	TAmp:       "&",
	TComma:     ",",
	TEOF:       "<EOF>",
	TEq:        "=",
	TFun:       "<fun>",
	TLet:       "<let>",
	TIdent:     "<identifier>",
	TLCurly:    "{",
	TLParen:    "(",
	TMut:       "<mut>",
	TNumber:    "<number>",
	TRCurly:    "}",
	TRParen:    ")",
	TStar:      "*",
	TString:    "<string>",
	TTypeIdent: "<type identifier>",
	TUnknown:   "<unknown>",
	TVoid:      "<void>",
}

var simpleTokens = map[rune]TokenKind{ //nolint:gochecknoglobals
	'&': TAmp,
	',': TComma,
	'=': TEq,
	'{': TLCurly,
	'(': TLParen,
	'}': TRCurly,
	')': TRParen,
	'*': TStar,
}

var keywords = map[string]TokenKind{ //nolint:gochecknoglobals
	"fun":  TFun,
	"let":  TLet,
	"mut":  TMut,
	"void": TVoid,
}

func (k TokenKind) String() string {
	s, ok := tokenKindNames[k]
	if !ok {
		panic(Errorf("unknown token kind: %d", k))
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
	Span  Span
}

func (t Token) String() string {
	return fmt.Sprintf("%s: %s", t.Span, t.Kind)
}

func lexToken(source *Source, idx int) Token {
	start := idx
	c := source.Content[idx]
	span := NewSpan(source, start, idx)
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
				return Token{TString, string(value), span}
			}
			idx += 1
			value = append(value, c)
		}
		return Token{TEOF, "", span}
	case unicode.IsLetter(c):
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
		kind := TTypeIdent
		if unicode.IsLower(c) {
			kind = TIdent
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
		return Token{TNumber, string(value), span}
	default:
		return Token{TUnknown, string(c), span}
	}
}

func lexSkipWhitespace(source *Source, idx int) int {
	for idx < len(source.Content) {
		c := source.Content[idx]
		if !unicode.IsSpace(c) {
			return idx
		}
		idx += 1
	}
	return idx
}

func Lex(source *Source) []Token {
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
