package token

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/flunderpero/metall/metallc/internal/base"
)

type TokenKind int

const (
	AllocatorIdent TokenKind = iota + 1
	Amp
	AmpImmediate
	AmpEq
	And
	Break
	Bytes
	Caret
	CaretEq
	Case
	Colon
	Comma
	Comment
	Continue
	Defer
	Dot
	DotDot
	Else
	Enum
	EOF
	Error
	Eq
	EqEq
	Excl
	Export
	Extern
	False
	For
	Fun
	Gt
	Gte
	GtGt
	GtGtEq
	HashEnd
	HashIf
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
	MinusEq
	MinusPercent
	MinusPercentEq
	Mut
	Neq
	Noescape
	Nocopy
	Not
	Number
	Or
	Percent
	PercentEq
	Pipe
	PipeEq
	Plus
	PlusEq
	PlusPercent
	PlusPercentEq
	Pub
	Question
	RBracket
	RCurly
	Return
	RParen
	Rune
	Shape
	LtLt
	LtLtEq
	Slash
	SlashEq
	Star
	StarEq
	StarPercent
	StarPercentEq
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
	AmpImmediate:          "<&immediate>",
	AmpEq:                 "&=",
	And:                   "<and>",
	Break:                 "<break>",
	Bytes:                 "<bytes>",
	Caret:                 "^",
	CaretEq:               "^=",
	Case:                  "<case>",
	Colon:                 ":",
	Comma:                 ",",
	Comment:               "<comment>",
	Continue:              "<continue>",
	Defer:                 "<defer>",
	Dot:                   ".",
	DotDot:                "..",
	Else:                  "<else>",
	Enum:                  "<enum>",
	EOF:                   "<EOF>",
	Error:                 "<error>",
	Eq:                    "=",
	EqEq:                  "==",
	Excl:                  "!",
	Export:                "<export>",
	Extern:                "<extern>",
	False:                 "false",
	For:                   "<for>",
	Gt:                    ">",
	Gte:                   ">=",
	GtGt:                  ">>",
	GtGtEq:                ">>=",
	Fun:                   "<fun>",
	HashEnd:               "#end",
	HashIf:                "#if",
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
	MinusEq:               "-=",
	MinusPercent:          "-%",
	MinusPercentEq:        "-%=",
	Mut:                   "<mut>",
	Neq:                   "!=",
	Nocopy:                "<nocopy>",
	Not:                   "<not>",
	Number:                "<number>",
	Or:                    "<or>",
	Percent:               "%",
	PercentEq:             "%=",
	Pipe:                  "|",
	PipeEq:                "|=",
	Plus:                  "+",
	PlusEq:                "+=",
	PlusPercent:           "+%",
	PlusPercentEq:         "+%=",
	Pub:                   "<pub>",
	Question:              "?",
	RBracket:              "]",
	RCurly:                "}",
	Return:                "return",
	RParen:                ")",
	Rune:                  "<rune>",
	Shape:                 "<shape>",
	LtLt:                  "<<",
	LtLtEq:                "<<=",
	Slash:                 "/",
	SlashEq:               "/=",
	Star:                  "*",
	StarEq:                "*=",
	StarPercent:           "*%",
	StarPercentEq:         "*%=",
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
	',': Comma,
	'{': LCurly,
	'(': LParen,
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
	"enum":     Enum,
	"export":   Export,
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
	"nocopy":   Nocopy,
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

func isDecDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

func isHexDigit(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isOctDigit(c rune) bool {
	return c >= '0' && c <= '7'
}

func isBinDigit(c rune) bool {
	return c == '0' || c == '1'
}

func peek(source *base.Source, idx int, r rune) bool {
	return idx < len(source.Content) && source.Content[idx] == r
}

// lexNumber reads a number literal. start may point at '-' or at the leading
// digit. Supported forms: decimal, hex (0x), octal (0o), binary (0b).
// Underscores are allowed between digits as separators.
func lexNumber(source *base.Source, start int) Token {
	idx := start
	value := []rune{}
	if peek(source, idx, '-') {
		value = append(value, '-')
		idx++
	}
	digitOk := isDecDigit
	baseName := "decimal"
	if idx+1 < len(source.Content) && source.Content[idx] == '0' {
		switch source.Content[idx+1] {
		case 'x':
			digitOk = isHexDigit
			baseName = "hex"
		case 'o':
			digitOk = isOctDigit
			baseName = "octal"
		case 'b':
			digitOk = isBinDigit
			baseName = "binary"
		}
		if baseName != "decimal" {
			value = append(value, source.Content[idx], source.Content[idx+1])
			idx += 2
		}
	}
	digitsStart := len(value)
	for idx < len(source.Content) {
		c := source.Content[idx]
		if !digitOk(c) && c != '_' {
			break
		}
		value = append(value, c)
		idx++
	}
	span := base.NewSpan(source, start, idx-1)
	digits := value[digitsStart:]
	if len(digits) == 0 {
		return Token{Error, fmt.Sprintf("expected %s digit after '0%c'", baseName, value[digitsStart-1]), span}
	}
	if digits[0] == '_' || digits[len(digits)-1] == '_' {
		return Token{Error, fmt.Sprintf("invalid %s literal: %s", baseName, string(value)), span}
	}
	for i := 1; i < len(digits); i++ {
		if digits[i] == '_' && digits[i-1] == '_' {
			return Token{Error, fmt.Sprintf("invalid %s literal: %s", baseName, string(value)), span}
		}
	}
	return Token{Number, string(value), span}
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
	if !peek(source, idx, '{') {
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

func lexStringBody(source *base.Source, idx int, span base.Span, kind TokenKind) Token {
	value := []byte{}
	for idx < len(source.Content) {
		c := source.Content[idx]
		if c == '"' {
			span.End = idx
			return Token{kind, string(value), span}
		}
		if c == '\\' {
			r, newIdx, errMsg := parseEscape(source, idx, '"')
			if errMsg != "" {
				span.End = skipToClosingQuote(source, idx+1, '"')
				return Token{Error, errMsg, span}
			}
			// In a bytes literal `\xNN` is a raw byte instead of a utf-8 encoded rune.
			if kind == Bytes && source.Content[idx+1] == 'x' {
				value = append(value, byte(r))
			} else {
				value = utf8.AppendRune(value, r)
			}
			idx = newIdx
		} else {
			idx += 1
			value = utf8.AppendRune(value, c)
		}
	}
	return Token{EOF, "", span}
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
		return lexStringBody(source, idx, span, String)
	case c == 'b' && peek(source, idx, '"'):
		return lexStringBody(source, idx+1, span, Bytes)
	case c == '\'':
		if idx < len(source.Content) && source.Content[idx] != '\'' {
			if source.Content[idx] == '\\' {
				r, newIdx, errMsg := parseEscape(source, idx, '\'')
				if errMsg != "" {
					span.End = skipToClosingQuote(source, idx+1, '\'')
					return Token{Error, errMsg, span}
				}
				if peek(source, newIdx, '\'') {
					span.End = newIdx
					return Token{Rune, string([]rune{r}), span}
				}
			} else {
				value := source.Content[idx]
				idx += 1
				if peek(source, idx, '\'') {
					span.End = idx
					return Token{Rune, string([]rune{value}), span}
				}
			}
		}
		return Token{Unknown, string(c), span}
	case c == '=':
		kind := Eq
		if peek(source, idx, '=') {
			kind = EqEq
			span = base.NewSpan(source, start, idx)
		}
		return Token{Kind: kind, Value: "", Span: span}
	case c == '<':
		if peek(source, idx, '=') {
			return Token{Kind: Lte, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		if peek(source, idx, '<') {
			if peek(source, idx+1, '=') {
				return Token{Kind: LtLtEq, Value: "", Span: base.NewSpan(source, start, idx+1)}
			}
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
		if peek(source, idx, '=') {
			return Token{Kind: Gte, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		if peek(source, idx, '>') {
			if peek(source, idx+1, '=') {
				return Token{Kind: GtGtEq, Value: "", Span: base.NewSpan(source, start, idx+1)}
			}
			return Token{Kind: GtGt, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Gt, Value: "", Span: span}
	case c == '/':
		if peek(source, idx, '=') {
			return Token{Kind: SlashEq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Slash, Value: "", Span: span}
	case c == '%':
		if peek(source, idx, '=') {
			return Token{Kind: PercentEq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Percent, Value: "", Span: span}
	case c == '|':
		if peek(source, idx, '=') {
			return Token{Kind: PipeEq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Pipe, Value: "", Span: span}
	case c == '^':
		if peek(source, idx, '=') {
			return Token{Kind: CaretEq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Caret, Value: "", Span: span}
	case c == '+':
		if peek(source, idx, '%') {
			if peek(source, idx+1, '=') {
				return Token{Kind: PlusPercentEq, Value: "", Span: base.NewSpan(source, start, idx+1)}
			}
			return Token{Kind: PlusPercent, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		if peek(source, idx, '=') {
			return Token{Kind: PlusEq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Plus, Value: "", Span: span}
	case c == '*':
		if peek(source, idx, '%') {
			if peek(source, idx+1, '=') {
				return Token{Kind: StarPercentEq, Value: "", Span: base.NewSpan(source, start, idx+1)}
			}
			return Token{Kind: StarPercent, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		if peek(source, idx, '=') {
			return Token{Kind: StarEq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Star, Value: "", Span: span}
	case c == '-':
		if peek(source, idx, '%') {
			if peek(source, idx+1, '=') {
				return Token{Kind: MinusPercentEq, Value: "", Span: base.NewSpan(source, start, idx+1)}
			}
			return Token{Kind: MinusPercent, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		if peek(source, idx, '=') {
			return Token{Kind: MinusEq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		if idx < len(source.Content) && unicode.IsDigit(source.Content[idx]) {
			return lexNumber(source, start)
		}
		if !peek(source, idx, '-') {
			return Token{Kind: Minus, Value: "", Span: span}
		}
		value := "--"
		end := "\n"
		idx += 1
		if peek(source, idx, '-') {
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
		if peek(source, idx, '.') {
			return Token{Kind: DotDot, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Dot, Value: "", Span: span}
	case c == ':':
		return Token{Kind: Colon, Value: "", Span: span}
	case c == '!':
		if peek(source, idx, '=') {
			return Token{Kind: Neq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		return Token{Kind: Excl, Value: "", Span: span}
	case c == '&':
		if peek(source, idx, '=') {
			return Token{Kind: AmpEq, Value: "", Span: base.NewSpan(source, start, idx)}
		}
		kind := AmpImmediate
		if idx < len(source.Content) && unicode.IsSpace(source.Content[idx]) {
			kind = Amp
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
	case c == '#':
		value := []rune{}
		for idx < len(source.Content) {
			c := source.Content[idx]
			if !unicode.IsLetter(c) {
				break
			}
			idx += 1
			value = append(value, c)
		}
		span.End = idx - 1
		word := string(value)
		switch word {
		case "if":
			return Token{Kind: HashIf, Value: "", Span: span}
		case "end":
			return Token{Kind: HashEnd, Value: "", Span: span}
		default:
			return Token{Kind: Error, Value: fmt.Sprintf("unknown directive: #%s", word), Span: span}
		}
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
		return lexNumber(source, start)
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
