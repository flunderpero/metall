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
	Case
	Colon
	Comma
	Comment
	Continue
	Defer
	Dot
	DotDot
	Else
	EOF
	Error
	Eq
	EqEq
	Excl
	Extern
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
	Is
	LBracket
	LBracketImmediate
	LCurly
	Let
	LParen
	Lt
	LtImmediate
	Lte
	Match
	Minus
	MinusPercent
	Mut
	Neq
	Noescape
	Not
	Number
	Or
	Percent
	Pipe
	Plus
	PlusPercent
	Pub
	Question
	RBracket
	RCurly
	Return
	RParen
	Rune
	Shape
	LtLt
	Slash
	Star
	StarPercent
	String
	Struct
	Sync
	Tilde
	Unsync
	True
	Try
	TypeIdent
	Unsafe
	Union
	Unknown
	Use
	When
	Whitespace
)

var tokenKindNames = map[TokenKind]string{ //nolint:gochecknoglobals
	AllocatorIdent:        "<allocator identifier>",
	Amp:                   "&",
	AmpInfix:              "<&infix>",
	And:                   "<and>",
	Break:                 "<break>",
	Caret:                 "^",
	Case:                  "<case>",
	Colon:                 ":",
	Comma:                 ",",
	Comment:               "<comment>",
	Continue:              "<continue>",
	Defer:                 "<defer>",
	Dot:                   ".",
	DotDot:                "..",
	Else:                  "<else>",
	EOF:                   "<EOF>",
	Error:                 "<error>",
	Eq:                    "=",
	EqEq:                  "==",
	Excl:                  "!",
	Extern:                "<extern>",
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
	Is:                    "<is>",
	LBracket:              "[",
	LBracketImmediate:     "<[immediate>",
	LCurly:                "{",
	Let:                   "<let>",
	LParen:                "(",
	Lt:                    "<",
	LtImmediate:           "<<immediate>",
	Lte:                   "<=",
	Match:                 "<match>",
	Minus:                 "-",
	MinusPercent:          "-%",
	Mut:                   "<mut>",
	Neq:                   "!=",
	Not:                   "<not>",
	Number:                "<number>",
	Or:                    "<or>",
	Percent:               "%",
	Pipe:                  "|",
	Plus:                  "+",
	PlusPercent:           "+%",
	Pub:                   "<pub>",
	Question:              "?",
	RBracket:              "]",
	RCurly:                "}",
	Return:                "return",
	RParen:                ")",
	Rune:                  "<rune>",
	Shape:                 "<shape>",
	LtLt:                  "<<",
	Slash:                 "/",
	Star:                  "*",
	StarPercent:           "*%",
	String:                "<string>",
	Tilde:                 "~",
	Noescape:              "<noescape>",
	Struct:                "<struct>",
	Sync:                  "<sync>",
	True:                  "true",
	Unsync:                "<unsync>",
	Try:                   "<try>",
	TypeIdent:             "<type identifier>",
	Unsafe:                "<unsafe>",
	Union:                 "<union>",
	Unknown:               "<unknown>",
	Use:                   "<use>",
	When:                  "<when>",
	Whitespace:            "<whitespace>",
}

var simpleTokens = map[rune]TokenKind{ //nolint:gochecknoglobals
	'^': Caret,
	',': Comma,
	'{': LCurly,
	'(': LParen,
	'%': Percent,
	'/': Slash,
	'|': Pipe,
	'?': Question,
	']': RBracket,
	'}': RCurly,
	')': RParen,
	'~': Tilde,
}

// KeywordNames maps keyword token kinds back to their string names.
var KeywordNames map[TokenKind]string //nolint:gochecknoglobals

func init() { //nolint:gochecknoinits
	KeywordNames = make(map[TokenKind]string, len(keywords))
	for name, kind := range keywords {
		KeywordNames[kind] = name
	}
}

var keywords = map[string]TokenKind{ //nolint:gochecknoglobals
	"and":      And,
	"break":    Break,
	"case":     Case,
	"continue": Continue,
	"defer":    Defer,
	"else":     Else,
	"extern":   Extern,
	"false":    False,
	"for":      For,
	"fun":      Fun,
	"if":       If,
	"in":       In,
	"is":       Is,
	"let":      Let,
	"match":    Match,
	"mut":      Mut,
	"noescape": Noescape,
	"pub":      Pub,
	"not":      Not,
	"or":       Or,
	"return":   Return,
	"shape":    Shape,
	"struct":   Struct,
	"sync":     Sync,
	"true":     True,
	"unsync":   Unsync,
	"try":      Try,
	"unsafe":   Unsafe,
	"union":    Union,
	"use":      Use,
	"when":     When,
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
	if t.Kind == Error {
		return fmt.Sprintf("%s: %s: %s", t.Span, t.Kind, t.Value)
	}
	return fmt.Sprintf("%s: %s", t.Span, t.Kind)
}

func hexDigit(c rune) (rune, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// parseHexEscape parses \xNN byte escapes. idx points to the first hex digit after \x.
func parseHexEscape(source *base.Source, idx int) (rune, int, bool) {
	if idx+2 > len(source.Content) {
		return 0, idx, false
	}
	val := rune(0)
	for i := range 2 {
		d, ok := hexDigit(source.Content[idx+i])
		if !ok {
			return 0, idx, false
		}
		val = val*16 + d
	}
	return val, idx + 2, true
}

// parseUnicodeEscape parses \u{NNNNNN} unicode escapes. idx points to the '{' after \u.
func parseUnicodeEscape(source *base.Source, idx int) (rune, int, bool) {
	if idx >= len(source.Content) || source.Content[idx] != '{' {
		return 0, idx, false
	}
	idx++ // skip {
	val := rune(0)
	digits := 0
	for idx < len(source.Content) && source.Content[idx] != '}' {
		d, ok := hexDigit(source.Content[idx])
		if !ok {
			return 0, idx, false
		}
		digits++
		if digits > 6 {
			return 0, idx, false
		}
		val = val*16 + d
		idx++
	}
	if idx >= len(source.Content) || digits == 0 || val > 0x10FFFF {
		return 0, idx, false
	}
	idx++ // skip }
	return val, idx, true
}

func parseEscape(source *base.Source, idx int, quote rune) (rune, int, string) {
	if idx+1 >= len(source.Content) {
		return 0, idx, "unexpected end of escape sequence"
	}
	escapeChar := source.Content[idx+1]
	switch escapeChar {
	case 'n':
		return '\n', idx + 2, ""
	case 't':
		return '\t', idx + 2, ""
	case '0':
		return '\000', idx + 2, ""
	case 'r':
		return '\r', idx + 2, ""
	case '\\':
		return '\\', idx + 2, ""
	case '\'':
		if quote == '\'' {
			return '\'', idx + 2, ""
		}
	case '"':
		if quote == '"' {
			return '"', idx + 2, ""
		}
	case 'x':
		if r, newIdx, ok := parseHexEscape(source, idx+2); ok {
			return r, newIdx, ""
		}
		return 0, idx, "invalid byte escape sequence"
	case 'u':
		if r, newIdx, ok := parseUnicodeEscape(source, idx+2); ok {
			return r, newIdx, ""
		}
		return 0, idx, "invalid unicode escape sequence"
	}
	return 0, idx, fmt.Sprintf(`unknown escape sequence '\%c'`, escapeChar)
}

// skipToClosingQuote advances idx past the closing quote character, skipping escaped characters.
// Returns the index of the closing quote, or len(source.Content) if not found.
func skipToClosingQuote(source *base.Source, idx int, quote rune) int {
	for idx < len(source.Content) {
		if source.Content[idx] == quote {
			return idx
		}
		if source.Content[idx] == '\\' {
			idx++ // skip the escaped character
		}
		idx++
	}
	return idx
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
			if c == '\\' {
				r, newIdx, errMsg := parseEscape(source, idx, '"')
				if errMsg != "" {
					span.End = skipToClosingQuote(source, idx+1, '"')
					return Token{Error, errMsg, span}
				}
				value = append(value, r)
				idx = newIdx
			} else {
				idx += 1
				value = append(value, c)
			}
		}
		return Token{EOF, "", span}
	case c == '\'':
		if idx < len(source.Content) && source.Content[idx] != '\'' {
			if source.Content[idx] == '\\' {
				r, newIdx, errMsg := parseEscape(source, idx, '\'')
				if errMsg != "" {
					span.End = skipToClosingQuote(source, idx+1, '\'')
					return Token{Error, errMsg, span}
				}
				if newIdx < len(source.Content) && source.Content[newIdx] == '\'' {
					span.End = newIdx
					return Token{Rune, string([]rune{r}), span}
				}
			} else {
				value := source.Content[idx]
				idx += 1
				if idx < len(source.Content) && source.Content[idx] == '\'' {
					span.End = idx
					return Token{Rune, string([]rune{value}), span}
				}
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
	case c == '+':
		if idx < len(source.Content) && source.Content[idx] == '%' {
			return Token{Kind: PlusPercent, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Plus, Value: "", Span: span}
	case c == '*':
		if idx < len(source.Content) && source.Content[idx] == '%' {
			return Token{Kind: StarPercent, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Star, Value: "", Span: span}
	case c == '-':
		if idx < len(source.Content) && source.Content[idx] == '%' {
			return Token{Kind: MinusPercent, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		if idx < len(source.Content) && unicode.IsDigit(source.Content[idx]) {
			value := []rune{c, source.Content[idx]}
			idx += 1
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
		}
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
		return Token{Kind: Colon, Value: "", Span: span}
	case c == '!':
		if idx < len(source.Content) && source.Content[idx] == '=' {
			return Token{Kind: Neq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Excl, Value: "", Span: span}
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
	case unicode.IsLetter(c), c == '@', c == '_':
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
		} else if unicode.IsLower(c) || c == '_' {
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
	end := len(source.Content)
	if end > 0 {
		end--
	}
	tokens = append(tokens, Token{EOF, "", base.NewSpan(source, end, end)})
	return tokens
}
