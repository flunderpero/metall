package token

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/flunderpero/metall/metallc/internal/base"
)

type TokenKind int

const (
	AllocatorIdent TokenKind = iota + 1
	Amp
	AmpInfix
	And
	Break
	Caret
	ColonColon
	Comma
	Comment
	Continue
	Dot
	DotDot
	Else
	EOF
	Eq
	EqEq
	False
	For
	Fun
	Gt
	Gte
	GtGt
	Ident
	If
	In
	InvalidAllocatorIdent
	LBracket
	LBracketImmediate
	LCurly
	Let
	LParen
	Lt
	LtImmediate
	Lte
	Make
	Minus
	Mut
	Neq
	New
	NewMut
	Not
	Number
	Or
	Percent
	Pipe
	Plus
	RBracket
	RCurly
	Return
	RParen
	Rune
	Shape
	LtLt
	Slash
	Star
	String
	Struct
	Tilde
	True
	TypeIdent
	Unknown
	Use
	Void
	Whitespace
)

var tokenKindNames = map[TokenKind]string{ //nolint:gochecknoglobals
	AllocatorIdent:        "<allocator identifier>",
	Amp:                   "&",
	AmpInfix:              "<&infix>",
	And:                   "<and>",
	Break:                 "<break>",
	Caret:                 "^",
	ColonColon:            "::",
	Comma:                 ",",
	Comment:               "<comment>",
	Continue:              "<continue>",
	Dot:                   ".",
	DotDot:                "..",
	Else:                  "<else>",
	EOF:                   "<EOF>",
	Eq:                    "=",
	EqEq:                  "==",
	False:                 "false",
	For:                   "<for>",
	Gt:                    ">",
	Gte:                   ">=",
	GtGt:                  ">>",
	Fun:                   "<fun>",
	Ident:                 "<identifier>",
	If:                    "<if>",
	In:                    "<in>",
	InvalidAllocatorIdent: "<invalid allocation identifier>",
	LBracket:              "[",
	LBracketImmediate:     "<[immediate>",
	LCurly:                "{",
	Let:                   "<let>",
	LParen:                "(",
	Lt:                    "<",
	LtImmediate:           "<<immediate>",
	Lte:                   "<=",
	Make:                  "<make>",
	Minus:                 "-",
	Mut:                   "<mut>",
	Neq:                   "!=",
	New:                   "<new>",
	NewMut:                "<new_mut>",
	Not:                   "<not>",
	Number:                "<number>",
	Or:                    "<or>",
	Percent:               "%",
	Pipe:                  "|",
	Plus:                  "+",
	RBracket:              "]",
	RCurly:                "}",
	Return:                "return",
	RParen:                ")",
	Rune:                  "<rune>",
	Shape:                 "<shape>",
	LtLt:                  "<<",
	Slash:                 "/",
	Star:                  "*",
	String:                "<string>",
	Tilde:                 "~",
	Struct:                "<struct>",
	True:                  "true",
	TypeIdent:             "<type identifier>",
	Unknown:               "<unknown>",
	Use:                   "<use>",
	Void:                  "<void>",
	Whitespace:            "<whitespace>",
}

var simpleTokens = map[rune]TokenKind{ //nolint:gochecknoglobals
	'^': Caret,
	',': Comma,
	'{': LCurly,
	'(': LParen,
	'%': Percent,
	'|': Pipe,
	'+': Plus,
	']': RBracket,
	'}': RCurly,
	')': RParen,
	'/': Slash,
	'*': Star,
	'~': Tilde,
}

var keywords = map[string]TokenKind{ //nolint:gochecknoglobals
	"and":      And,
	"break":    Break,
	"continue": Continue,
	"else":     Else,
	"false":    False,
	"for":      For,
	"fun":      Fun,
	"if":       If,
	"in":       In,
	"let":      Let,
	"make":     Make,
	"mut":      Mut,
	"new":      New,
	"new_mut":  NewMut,
	"not":      Not,
	"or":       Or,
	"return":   Return,
	"shape":    Shape,
	"struct":   Struct,
	"true":     True,
	"use":      Use,
	"void":     Void,
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
	case c == '\'':
		if idx < len(source.Content) && source.Content[idx] != '\'' {
			value := source.Content[idx]
			idx += 1
			if idx < len(source.Content) && source.Content[idx] == '\'' {
				span.End = idx
				return Token{Rune, string(value), span}
			}
		}
		return Token{Unknown, string(c), span}
	case c == '=':
		kind := Eq
		if idx < len(source.Content) && source.Content[idx] == '=' {
			kind = EqEq
			span = base.NewSpan(source, start, idx)
		}
		return Token{Kind: kind, Value: "", Span: span}
	case c == '<':
		if idx < len(source.Content) && source.Content[idx] == '=' {
			return Token{Kind: Lte, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		if idx < len(source.Content) && source.Content[idx] == '<' {
			return Token{Kind: LtLt, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		kind := Lt
		if idx > 1 {
			prev := source.Content[idx-2]
			if unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '_' {
				kind = LtImmediate
			}
		}
		return Token{Kind: kind, Value: "", Span: span}
	case c == '>':
		if idx < len(source.Content) && source.Content[idx] == '=' {
			return Token{Kind: Gte, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		if idx < len(source.Content) && source.Content[idx] == '>' {
			return Token{Kind: GtGt, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Gt, Value: "", Span: span}
	case c == '-':
		if idx >= len(source.Content) || source.Content[idx] != '-' {
			return Token{Kind: Minus, Value: "", Span: span}
		}
		value := "--"
		end := "\n"
		idx += 1
		if idx < len(source.Content) && source.Content[idx] == '-' {
			idx += 1
			value = "---"
			end = "---"
		}
		for idx < len(source.Content) {
			if len(value) != len(end) && strings.HasSuffix(value, end) {
				break
			}
			value += string(source.Content[idx])
			idx += 1
		}
		span.End = idx - 1
		return Token{Kind: Comment, Value: value, Span: span}
	case c == '.':
		if idx < len(source.Content) && source.Content[idx] == '.' {
			return Token{Kind: DotDot, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Dot, Value: "", Span: span}
	case c == ':':
		if idx < len(source.Content) && source.Content[idx] == ':' {
			return Token{Kind: ColonColon, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Unknown, Value: string(c), Span: span}
	case c == '!':
		if idx < len(source.Content) && source.Content[idx] == '=' {
			return Token{Kind: Neq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Unknown, Value: fmt.Sprint(c), Span: span}
	case c == '&':
		kind := Amp
		if idx < len(source.Content) && unicode.IsSpace(source.Content[idx]) {
			kind = AmpInfix
		}
		return Token{Kind: kind, Value: "", Span: span}
	case c == '[':
		kind := LBracket
		if idx > 1 {
			prev := source.Content[idx-2]
			if unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '_' || prev == ')' || prev == ']' {
				kind = LBracketImmediate
			}
		}
		return Token{Kind: kind, Value: "", Span: span}
	case unicode.IsSpace(c):
		value := []rune{c}
		for idx < len(source.Content) {
			c = source.Content[idx]
			if !unicode.IsSpace(c) {
				break
			}
			idx += 1
			value = append(value, c)
		}
		span.End = idx - 1
		return Token{Kind: Whitespace, Value: string(value), Span: span}
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
			kind = AllocatorIdent
			if len(value) < 2 || !unicode.IsLower(value[1]) {
				kind = InvalidAllocatorIdent
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

func Lex(source *base.Source) []Token {
	tokens := []Token{}
	idx := 0
	for idx < len(source.Content) {
		token := lexToken(source, idx)
		tokens = append(tokens, token)
		idx = token.Span.End + 1
	}
	return tokens
}
