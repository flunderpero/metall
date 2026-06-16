package token

import (
	"fmt"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
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
		{"defer keyword", "defer", []want{{Defer, "", "1:1-1:5"}}},
		{"parens", "()", []want{{LParen, "", "1:1"}, {RParen, "", "1:2"}}},
		{"curly", "{}", []want{{LCurly, "", "1:1"}, {RCurly, "", "1:2"}}},
		{"brackets", "[]", []want{{LBracket, "", "1:1"}, {RBracket, "", "1:2"}}},
		{
			"lbracket vs lbracketimmediate",
			"a[ s1[ [ ([ )[ ][ [[ {[",
			[]want{
				{Ident, "a", "1:1"},
				{LBracketImmediate, "", "1:2"},
				{Whitespace, " ", "1:3"},

				{Ident, "s1", "1:4-1:5"},
				{LBracketImmediate, "", "1:6"},
				{Whitespace, " ", "1:7"},

				{LBracket, "", "1:8"},
				{Whitespace, " ", "1:9"},

				{LParen, "", "1:10"},
				{LBracket, "", "1:11"},
				{Whitespace, " ", "1:12"},

				{RParen, "", "1:13"},
				{LBracketImmediate, "", "1:14"},
				{Whitespace, " ", "1:15"},

				{RBracket, "", "1:16"},
				{LBracketImmediate, "", "1:17"},
				{Whitespace, " ", "1:18"},

				{LBracket, "", "1:19"},
				{LBracket, "", "1:20"},
				{Whitespace, " ", "1:21"},

				{LCurly, "", "1:22"},
				{LBracket, "", "1:23"},
			},
		},
		{"plus", "+", []want{{Plus, "", "1:1"}}},
		{"minus", "-", []want{{MinusAfterNewline, "", "1:1"}}},
		{"slash", "/", []want{{Slash, "", "1:1"}}},
		{"percent", "%", []want{{Percent, "", "1:1"}}},
		{"lt", "<", []want{{Lt, "", "1:1"}}},
		{
			"lt vs ltimmediate",
			"a< s1< < )< a<= <=",
			[]want{
				{Ident, "a", "1:1"},
				{LtImmediate, "", "1:2"},
				{Whitespace, " ", "1:3"},

				{Ident, "s1", "1:4-1:5"},
				{LtImmediate, "", "1:6"},
				{Whitespace, " ", "1:7"},

				{Lt, "", "1:8"},
				{Whitespace, " ", "1:9"},

				{RParen, "", "1:10"},
				{Lt, "", "1:11"},
				{Whitespace, " ", "1:12"},

				{Ident, "a", "1:13"},
				{Lte, "", "1:14-1:15"},
				{Whitespace, " ", "1:16"},

				{Lte, "", "1:17-1:18"},
			},
		},
		{"lte", "<=", []want{{Lte, "", "1:1-1:2"}}},
		{"gt", ">", []want{{Gt, "", "1:1"}}},
		{"gte", ">=", []want{{Gte, "", "1:1-1:2"}}},
		{"eq", "=", []want{{Eq, "", "1:1"}}},
		{"eqeq", "==", []want{{EqEq, "", "1:1-1:2"}}},
		{"neq", "!=", []want{{Neq, "", "1:1-1:2"}}},
		{"excl", "!", []want{{Excl, "", "1:1"}}},
		{"question", "?", []want{{Question, "", "1:1"}}},
		{"and", "and", []want{{And, "", "1:1-1:3"}}},
		{"or", "or", []want{{Or, "", "1:1-1:2"}}},
		{"not", "not", []want{{Not, "", "1:1-1:3"}}},
		{"amp at line start", "&", []want{{AmpAfterNewline, "", "1:1"}}},
		{"amp ref at line start", "&x", []want{
			{AmpAfterNewline, "", "1:1"},
			{Ident, "x", "1:2"},
		}},
		{"amp mut ref at line start", "&mut x", []want{
			{AmpAfterNewline, "", "1:1"},
			{Mut, "", "1:2-1:4"},
			{Whitespace, " ", "1:5"},
			{Ident, "x", "1:6"},
		}},
		{"amp before mut at line start", "& mut", []want{
			{AmpAfterNewline, "", "1:1"},
			{Whitespace, " ", "1:2"},
			{Mut, "", "1:3-1:5"},
		}},
		{"amp infix spaced", "a & b", []want{
			{Ident, "a", "1:1"},
			{Whitespace, " ", "1:2"},
			{Amp, "", "1:3"},
			{Whitespace, " ", "1:4"},
			{Ident, "b", "1:5"},
		}},
		{"amp infix glued lexes the same as spaced", "a &b", []want{
			{Ident, "a", "1:1"},
			{Whitespace, " ", "1:2"},
			{Amp, "", "1:3"},
			{Ident, "b", "1:4"},
		}},
		{"pipe", "|", []want{{Pipe, "", "1:1"}}},
		{"caret", "^", []want{{Caret, "", "1:1"}}},
		{"tilde", "~", []want{{Tilde, "", "1:1"}}},
		{"ltlt", "<<", []want{{LtLt, "", "1:1-1:2"}}},
		{"gtgt", ">>", []want{{GtGt, "", "1:1-1:2"}}},
		{"star", "*", []want{{Star, "", "1:1"}}},
		{"plus eq", "+=", []want{{PlusEq, "", "1:1-1:2"}}},
		{"plus percent eq", "+%=", []want{{PlusPercentEq, "", "1:1-1:3"}}},
		{"minus eq", "-=", []want{{MinusEq, "", "1:1-1:2"}}},
		{"minus percent eq", "-%=", []want{{MinusPercentEq, "", "1:1-1:3"}}},
		{"star eq", "*=", []want{{StarEq, "", "1:1-1:2"}}},
		{"star percent eq", "*%=", []want{{StarPercentEq, "", "1:1-1:3"}}},
		{"slash eq", "/=", []want{{SlashEq, "", "1:1-1:2"}}},
		{"percent eq", "%=", []want{{PercentEq, "", "1:1-1:2"}}},
		{"amp eq", "&=", []want{{AmpEq, "", "1:1-1:2"}}},
		{"pipe eq", "|=", []want{{PipeEq, "", "1:1-1:2"}}},
		{"caret eq", "^=", []want{{CaretEq, "", "1:1-1:2"}}},
		{"ltlt eq", "<<=", []want{{LtLtEq, "", "1:1-1:3"}}},
		{"gtgt eq", ">>=", []want{{GtGtEq, "", "1:1-1:3"}}},
		{"compound assign in context", "x += 1", []want{
			{Ident, "x", "1:1"},
			{Whitespace, " ", "1:2"},
			{PlusEq, "", "1:3-1:4"},
			{Whitespace, " ", "1:5"},
			{Number, "1", "1:6"},
		}},
		{"wrap compound assign in context", "x +%= 1", []want{
			{Ident, "x", "1:1"},
			{Whitespace, " ", "1:2"},
			{PlusPercentEq, "", "1:3-1:5"},
			{Whitespace, " ", "1:6"},
			{Number, "1", "1:7"},
		}},
		{"minus eq vs minus number", "x -= -5", []want{
			{Ident, "x", "1:1"},
			{Whitespace, " ", "1:2"},
			{MinusEq, "", "1:3-1:4"},
			{Whitespace, " ", "1:5"},
			{Minus, "", "1:6"},
			{Number, "5", "1:7"},
		}},
		{"amp eq vs amp infix", "a &= b & c", []want{
			{Ident, "a", "1:1"},
			{Whitespace, " ", "1:2"},
			{AmpEq, "", "1:3-1:4"},
			{Whitespace, " ", "1:5"},
			{Ident, "b", "1:6"},
			{Whitespace, " ", "1:7"},
			{Amp, "", "1:8"},
			{Whitespace, " ", "1:9"},
			{Ident, "c", "1:10"},
		}},
		{"shift eq vs shift", "a <<= b >> c", []want{
			{Ident, "a", "1:1"},
			{Whitespace, " ", "1:2"},
			{LtLtEq, "", "1:3-1:5"},
			{Whitespace, " ", "1:6"},
			{Ident, "b", "1:7"},
			{Whitespace, " ", "1:8"},
			{GtGt, "", "1:9-1:10"},
			{Whitespace, " ", "1:11"},
			{Ident, "c", "1:12"},
		}},
		{"number (int)", `123`, []want{{Number, "123", "1:1-1:3"}}},
		{"minus before int", `-123`, []want{{MinusAfterNewline, "", "1:1"}, {Number, "123", "1:2-1:4"}}},
		{"number (dec underscore)", `1_000_000`, []want{{Number, "1_000_000", "1:1-1:9"}}},
		{"number (hex)", `0xFF`, []want{{Number, "0xFF", "1:1-1:4"}}},
		{"number (hex lowercase)", `0xff`, []want{{Number, "0xff", "1:1-1:4"}}},
		{"number (hex mixed)", `0xDeAdBeEf`, []want{{Number, "0xDeAdBeEf", "1:1-1:10"}}},
		{"number (hex underscore)", `0xDEAD_BEEF`, []want{{Number, "0xDEAD_BEEF", "1:1-1:11"}}},
		{"minus before hex", `-0xff`, []want{{MinusAfterNewline, "", "1:1"}, {Number, "0xff", "1:2-1:5"}}},
		{"number (octal)", `0o755`, []want{{Number, "0o755", "1:1-1:5"}}},
		{"minus before octal", `-0o17`, []want{{MinusAfterNewline, "", "1:1"}, {Number, "0o17", "1:2-1:5"}}},
		{"number (binary)", `0b1010_1010`, []want{{Number, "0b1010_1010", "1:1-1:11"}}},
		{"minus before binary", `-0b101`, []want{{MinusAfterNewline, "", "1:1"}, {Number, "0b101", "1:2-1:6"}}},
		{"number (hex zero)", `0x0`, []want{{Number, "0x0", "1:1-1:3"}}},
		{"number (hex empty)", `0x`, []want{{Error, "expected hex digit after '0x'", "1:1-1:2"}}},
		{"number (octal empty)", `0o`, []want{{Error, "expected octal digit after '0o'", "1:1-1:2"}}},
		{"number (binary empty)", `0b`, []want{{Error, "expected binary digit after '0b'", "1:1-1:2"}}},
		{"number (hex leading _)", `0x_FF`, []want{{Error, "invalid hex literal: 0x_FF", "1:1-1:5"}}},
		{"number (hex trailing _)", `0xFF_`, []want{{Error, "invalid hex literal: 0xFF_", "1:1-1:5"}}},
		{"number (hex double _)", `0xF__F`, []want{{Error, "invalid hex literal: 0xF__F", "1:1-1:6"}}},
		{"number (dec trailing _)", `100_`, []want{{Error, "invalid decimal literal: 100_", "1:1-1:4"}}},
		{"number (hex stops at non-digit)", `0xFF.x`, []want{
			{Number, "0xFF", "1:1-1:4"},
			{Dot, "", "1:5"},
			{Ident, "x", "1:6"},
		}},
		{"number (octal stops at 8)", `0o178`, []want{
			{Number, "0o17", "1:1-1:4"},
			{Number, "8", "1:5"},
		}},
		{"number (binary stops at 2)", `0b1012`, []want{
			{Number, "0b101", "1:1-1:5"},
			{Number, "2", "1:6"},
		}},
		{"float (basic)", `3.14`, []want{{Float, "3.14", "1:1-1:4"}}},
		{"float (zero)", `0.0`, []want{{Float, "0.0", "1:1-1:3"}}},
		{"minus before float", `-2.5`, []want{{MinusAfterNewline, "", "1:1"}, {Float, "2.5", "1:2-1:4"}}},
		{"float (exponent)", `1e10`, []want{{Float, "1e10", "1:1-1:4"}}},
		{"float (exponent upper sign)", `1E+5`, []want{{Float, "1E+5", "1:1-1:4"}}},
		{"float (frac and exponent)", `1.5e-3`, []want{{Float, "1.5e-3", "1:1-1:6"}}},
		{"float (underscores)", `1_000.000_1`, []want{{Float, "1_000.000_1", "1:1-1:11"}}},
		{"float (dot is method access)", `1.foo`, []want{
			{Number, "1", "1:1"},
			{Dot, "", "1:2"},
			{Ident, "foo", "1:3-1:5"},
		}},
		{"float (range is not a float)", `1..10`, []want{
			{Number, "1", "1:1"},
			{DotDot, "", "1:2-1:3"},
			{Number, "10", "1:4-1:5"},
		}},
		{"float (hex has no float)", `0xFF.5`, []want{
			{Number, "0xFF", "1:1-1:4"},
			{Dot, "", "1:5"},
			{Number, "5", "1:6"},
		}},
		{"float (incomplete exponent is int)", `1e`, []want{
			{Number, "1", "1:1"},
			{Ident, "e", "1:2"},
		}},
		{"float (trailing dot is int and dot)", `1.`, []want{
			{Number, "1", "1:1"},
			{Dot, "", "1:2"},
		}},
		{"float (frac trailing underscore)", `1.5_`, []want{
			{Error, "invalid decimal literal: 1.5_", "1:1-1:4"},
		}},
		// The lexer keeps the text between the quotes verbatim; the parser decodes
		// escapes, dedents, and enforces the multi-line rules.
		{"string", `"ride"`, []want{{String, "ride", "1:1-1:6"}}},
		{"string empty", `""`, []want{{String, "", "1:1-1:2"}}},
		{"string keeps escapes raw", `"line1\nline2"`, []want{{String, `line1\nline2`, "1:1-1:14"}}},
		{"string keeps backslash escapes raw", `"path\\to\\file"`, []want{{String, `path\\to\\file`, "1:1-1:16"}}},
		{"string keeps escaped quote raw", `"\""`, []want{{String, `\"`, "1:1-1:4"}}},
		{"string with single quotes", `"'hello'"`, []want{{String, "'hello'", "1:1-1:9"}}},
		{"string modifier", `b"abc"`, []want{{String, "abc", "1:1-1:6"}}},
		{"string modifier empty", `b""`, []want{{String, "", "1:1-1:3"}}},
		{"string multi-letter modifier", `bm"x"`, []want{{String, "x", "1:1-1:5"}}},
		{"string modifier on identifier", `boo"x"`, []want{{String, "x", "1:1-1:6"}}},
		// Sigils let a string carry bare quotes; the closing run must match the open.
		{"string one sigil", `#"a"b"#`, []want{{String, `a"b`, "1:1-1:7"}}},
		{"string three sigils", `###"x"###`, []want{{String, "x", "1:1-1:9"}}},
		{"string modifier with sigils", `b###"q"###`, []want{{String, "q", "1:1-1:10"}}},
		{"string sigil with embedded quotes", `#"he said "hi""#`, []want{{String, `he said "hi"`, "1:1-1:16"}}},
		{"string line continuation kept raw", "\"a\\\n   b\"", []want{{String, "a\\\n   b", "1:1-2:5"}}},
		{"multi-line string kept raw", "m\"\n  hi\n  \"", []want{{String, "\n  hi\n  ", "1:1-3:3"}}},
		{"string keeps backslash whitespace raw", `"a\ b"`, []want{{String, `a\ b`, "1:1-1:6"}}},
		{"string unterminated", `"abc`, []want{{Error, "unterminated string literal", "1:1-1:4"}}},
		{"string sigil mismatch", `#"abc"`, []want{{Error, "unterminated string literal", "1:1-1:6"}}},
		// An f-string is split into structured tokens; interpolations are lexed as
		// ordinary tokens, so each `{expr}` is real code.
		{"f-string simple", `f"hi {name}"`, []want{
			{FStringStart, `f"`, "1:1-1:2"},
			{FStringText, "hi ", "1:3-1:5"},
			{FExprStart, "{", "1:6"},
			{Ident, "name", "1:7-1:10"},
			{FExprEnd, "}", "1:11"},
			{FStringEnd, `"`, "1:12"},
		}},
		{"f-string empty", `f""`, []want{
			{FStringStart, `f"`, "1:1-1:2"},
			{FStringEnd, `"`, "1:3"},
		}},
		{"f-string sigil opener, plain closer", `f#"a #{x}"#`, []want{
			{FStringStart, `f#"`, "1:1-1:3"},
			{FStringText, "a ", "1:4-1:5"},
			{FExprStart, "#{", "1:6-1:7"},
			{Ident, "x", "1:8"},
			{FExprEnd, "}", "1:9"},
			{FStringEnd, `"#`, "1:10-1:11"},
		}},
		{"f-string interpolation holds a string literal", `f"v={g("a")}"`, []want{
			{FStringStart, `f"`, "1:1-1:2"},
			{FStringText, "v=", "1:3-1:4"},
			{FExprStart, "{", "1:5"},
			{Ident, "g", "1:6"},
			{LParen, "", "1:7"},
			{String, "a", "1:8-1:10"},
			{RParen, "", "1:11"},
			{FExprEnd, "}", "1:12"},
			{FStringEnd, `"`, "1:13"},
		}},
		{"f-string keeps a unicode escape literal, not an opener", `f"x\u{41}"`, []want{
			{FStringStart, `f"`, "1:1-1:2"},
			{FStringText, `x\u{41}`, "1:3-1:9"},
			{FStringEnd, `"`, "1:10"},
		}},
		{"f-string collapses literal braces", `f"a{{b}}c"`, []want{
			{FStringStart, `f"`, "1:1-1:2"},
			{FStringText, "a{b}c", "1:3-1:9"},
			{FStringEnd, `"`, "1:10"},
		}},
		// An interpolation balances its own braces; the `}` that closes a nested
		// block is an RCurly, only the outermost `}` ends the interpolation.
		{"f-string balances braces in an interpolation", `f"{a{b}c}"`, []want{
			{FStringStart, `f"`, "1:1-1:2"},
			{FExprStart, "{", "1:3"},
			{Ident, "a", "1:4"},
			{LCurly, "", "1:5"},
			{Ident, "b", "1:6"},
			{RCurly, "", "1:7"},
			{Ident, "c", "1:8"},
			{FExprEnd, "}", "1:9"},
			{FStringEnd, `"`, "1:10"},
		}},
		{"f-string unterminated", `f"hi {x}`, []want{
			{FStringStart, `f"`, "1:1-1:2"},
			{FStringText, "hi ", "1:3-1:5"},
			{FExprStart, "{", "1:6"},
			{Ident, "x", "1:7"},
			{FExprEnd, "}", "1:8"},
			{Error, "unterminated string literal", "1:1-1:8"},
		}},
		{"f-string unmatched closing brace", `f"a}b"`, []want{
			{FStringStart, `f"`, "1:1-1:2"},
			{FStringText, "a", "1:3"},
			{Error, "unmatched '}' in format string; write '}}' for a literal brace", "1:4"},
			{FStringText, "b", "1:5"},
			{FStringEnd, `"`, "1:6"},
		}},
		{"f-string sigil mismatch is unterminated", `f#"abc"`, []want{
			{FStringStart, `f#"`, "1:1-1:3"},
			{FStringText, `abc"`, "1:4-1:7"},
			{Error, "unterminated string literal", "1:1-1:7"},
		}},
		{"rune", `'a'`, []want{{Rune, "a", "1:1-1:3"}}},
		{"rune escape newline", `'\n'`, []want{{Rune, "\n", "1:1-1:4"}}},
		{"rune escape tab", `'\t'`, []want{{Rune, "\t", "1:1-1:4"}}},
		{"rune escape null", `'\0'`, []want{{Rune, "\x00", "1:1-1:4"}}},
		{"rune escape carriage return", `'\r'`, []want{{Rune, "\r", "1:1-1:4"}}},
		{"rune escape backslash", `'\\'`, []want{{Rune, "\\", "1:1-1:4"}}},
		{"rune escape single quote", "'\\''", []want{{Rune, "'", "1:1-1:4"}}},
		{"rune escape hex", `'\xFF'`, []want{{Rune, "\u00FF", "1:1-1:6"}}},
		{"rune escape hex lowercase", `'\x0a'`, []want{{Rune, "\n", "1:1-1:6"}}},
		{"rune escape unicode", `'\u{20AC}'`, []want{{Rune, "\u20AC", "1:1-1:10"}}},
		{"rune escape unicode supplementary", `'\u{1F600}'`, []want{{Rune, "\U0001F600", "1:1-1:11"}}},
		{"rune unknown escape", `'\a'`, []want{{Error, `unknown escape sequence '\a'`, "1:1-1:4"}}},
		{"rune invalid hex escape", `'\xGG'`, []want{{Error, "invalid byte escape sequence", "1:1-1:6"}}},
		{"rune invalid unicode escape", `'\u{ZZZZ}'`, []want{{Error, "invalid unicode escape sequence", "1:1-1:10"}}},
		{"rune unicode", `'é'`, []want{{Rune, "é", "1:1-1:3"}}},
		{"rune unclosed", `'a`, []want{{Unknown, "'", "1:1"}, {Ident, "a", "1:2"}}},
		{"rune empty", `''`, []want{{Unknown, "'", "1:1"}, {Unknown, "'", "1:2"}}},
		{"ident", `x`, []want{{Ident, "x", "1:1"}}},
		{"type ident", `Foo`, []want{{TypeIdent, "Foo", "1:1-1:3"}}},
		{"alloc ident", "@myalloc", []want{{AllocatorIdent, "@myalloc", "1:1-1:8"}}},
		{"invalid alloc ident (too short)", "@", []want{{InvalidAllocatorIdent, "@", "1:1"}}},
		{"invalid alloc ident (uppercase)", "@Myalloc", []want{{InvalidAllocatorIdent, "@Myalloc", "1:1-1:8"}}},
		{"fun", `fun`, []want{{Fun, "", "1:1-1:3"}}},
		{"if", "if", []want{{If, "", "1:1-1:2"}}},
		{"in", "in", []want{{In, "", "1:1-1:2"}}},
		{"else", "else", []want{{Else, "", "1:1-1:4"}}},
		{"true", "true", []want{{True, "", "1:1-1:4"}}},
		{"false", "false", []want{{False, "", "1:1-1:5"}}},
		{"pub", `pub`, []want{{Pub, "", "1:1-1:3"}}},
		{"mut", `mut`, []want{{Mut, "", "1:1-1:3"}}},
		{"let", `let`, []want{{Let, "", "1:1-1:3"}}},
		{"struct", `struct`, []want{{Struct, "", "1:1-1:6"}}},
		{"sync", `sync`, []want{{Sync, "", "1:1-1:4"}}},
		{"unsync", `unsync`, []want{{Unsync, "", "1:1-1:6"}}},
		{"noescape", `noescape`, []want{{Noescape, "", "1:1-1:8"}}},
		{"shape", `shape`, []want{{Shape, "", "1:1-1:5"}}},
		{"extern", `extern`, []want{{Extern, "", "1:1-1:6"}}},
		{"unsafe", `unsafe`, []want{{Unsafe, "", "1:1-1:6"}}},
		{"union", `union`, []want{{Union, "", "1:1-1:5"}}},
		{"enum", `enum`, []want{{Enum, "", "1:1-1:4"}}},
		{"match", `match`, []want{{Match, "", "1:1-1:5"}}},
		{"case", `case`, []want{{Case, "", "1:1-1:4"}}},
		{"when", `when`, []want{{When, "", "1:1-1:4"}}},
		{"hash if", `#if`, []want{{HashIf, "", "1:1-1:3"}}},
		{"hash end", `#end`, []want{{HashEnd, "", "1:1-1:4"}}},
		{"hash unknown", `#foo`, []want{{Error, "unknown directive: #foo", "1:1-1:4"}}},
		{"underscore is ident", `_`, []want{{Ident, "_", "1:1"}}},
		{"for", `for`, []want{{For, "", "1:1-1:3"}}},
		{"break", `break`, []want{{Break, "", "1:1-1:5"}}},
		{"continue", `continue`, []want{{Continue, "", "1:1-1:8"}}},
		{"return", "return", []want{{Return, "", "1:1-1:6"}}},
		{"use", "use", []want{{Use, "", "1:1-1:3"}}},
		{"colon", ":", []want{{Colon, "", "1:1"}}},
		{"dot", ".", []want{{Dot, "", "1:1"}}},
		{"dotdot", "..", []want{{DotDot, "", "1:1-1:2"}}},
		{"dot vs dotdot", "a.b..c", []want{
			{Ident, "a", "1:1"},
			{Dot, "", "1:2"},
			{Ident, "b", "1:3"},
			{DotDot, "", "1:4-1:5"},
			{Ident, "c", "1:6"},
		}},
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

	assert := base.NewAssert(t)
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
			assert.Equal(len(tokens)-1, len(tt.want)) // -1 for trailing EOF
			assert.Equal(EOF, tokens[len(tokens)-1].Kind, "last token should be EOF")
		})
	}
}
