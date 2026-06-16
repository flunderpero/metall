#include "tree_sitter/parser.h"
#include <stdlib.h>

// External scanner for Metall string literals. It handles sigils without
// counting them: a string opened with one or more `#` is closed by `"` followed
// by *any* number of `#` (we only highlight correct programs, so the counts
// always match). The opener's modifiers+sigils (`fm#`) and the closing sigils
// (`#`) are emitted as their own tokens so they can be highlighted apart from
// the string body. f-strings are split into fragments and interpolations so each
// `{expr}` is a real expression node.

enum TokenType {
    STRING_PREFIX,   // modifiers+sigils of a non-f string (`b#`), opaque
    FSTRING_PREFIX,  // modifiers+sigils of an f-string (`fm#`), always present
    STRING_CONTENT,  // the `"..."` body (quotes included) of a non-f string
    STRING_SUFFIX,   // the closing sigils (`#`) of any string
    FSTRING_START,   // the opening `"` of an f-string
    STRING_FRAGMENT, // a run of literal text inside an f-string
    INTERP_START,    // `{` (no sigils) or `#...{` (sigils)
    INTERP_END,      // `}` (no sigils) or `#...}` (sigils)
    FSTRING_END,     // the closing `"` of an f-string
};

#define MAX_DEPTH 32

typedef struct {
    // had_sigils[i] records whether the i-th open f-string used sigils, so we
    // know whether a bare `"`/`{`/`}` is a delimiter or literal text.
    unsigned char had_sigils[MAX_DEPTH];
    unsigned depth;
    // pending_sigils: the prefix just emitted used sigils, so the upcoming body
    // (fstring_start / string_content) is a sigil string.
    unsigned char pending_sigils;
    // pending_suffix: the body/fstring_end just emitted was a sigil string, so a
    // closing-sigil suffix follows.
    unsigned char pending_suffix;
} Scanner;

static void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

// skip_ws discards leading whitespace. Tree-sitter invokes the scanner *before*
// skipping extras, so an opener preceded by whitespace (`= f"..."`) would
// otherwise be missed: the scanner sees the space, bails, and the internal
// lexer then claims `f` as an identifier. Only the opener and interp-end use
// this; inside a string body whitespace is significant content.
static void skip_ws(TSLexer *lexer) {
    while (lexer->lookahead == ' ' || lexer->lookahead == '\t' ||
           lexer->lookahead == '\n' || lexer->lookahead == '\r') {
        lexer->advance(lexer, true);
    }
}

static bool top_had_sigils(Scanner *s) {
    return s->depth > 0 && s->depth <= MAX_DEPTH && s->had_sigils[s->depth - 1];
}

// scan_string_content consumes the `"..."` body of a non-f string. The opening
// quote is the current lookahead; closing sigils (if any) are left for the
// suffix token.
static bool scan_string_content(Scanner *s, TSLexer *lexer) {
    bool had_sigils = s->pending_sigils;
    advance(lexer); // opening "
    for (;;) {
        if (lexer->eof(lexer)) {
            return false;
        }
        if (lexer->lookahead == '\\') {
            advance(lexer);
            if (!lexer->eof(lexer)) {
                advance(lexer);
            }
            continue;
        }
        if (lexer->lookahead == '"') {
            advance(lexer);
            if (had_sigils && lexer->lookahead != '#') {
                continue; // a bare `"` is literal in a sigil string
            }
            // Terminator. The closing sigils (if any) become the suffix token.
            s->pending_suffix = had_sigils ? 1 : 0;
            lexer->result_symbol = STRING_CONTENT;
            return true;
        }
        advance(lexer);
    }
}

// scan_opener handles the start of any string at expression position: it emits
// the prefix (modifiers+sigils) when present, else the body directly.
static bool scan_opener(Scanner *s, TSLexer *lexer, const bool *valid) {
    bool is_format = false;
    bool has_mods = false;
    bool had_sigils = false;
    while (lexer->lookahead == 'f' || lexer->lookahead == 'b' || lexer->lookahead == 'm') {
        if (lexer->lookahead == 'f') {
            is_format = true;
        }
        has_mods = true;
        advance(lexer);
    }
    while (lexer->lookahead == '#') {
        had_sigils = true;
        advance(lexer);
    }
    if (lexer->lookahead != '"') {
        return false;
    }
    if (is_format) {
        if (!valid[FSTRING_PREFIX]) {
            return false;
        }
        s->pending_sigils = had_sigils ? 1 : 0;
        lexer->mark_end(lexer); // the prefix ends before the opening quote
        lexer->result_symbol = FSTRING_PREFIX;
        return true;
    }
    if (has_mods || had_sigils) {
        if (!valid[STRING_PREFIX]) {
            return false;
        }
        s->pending_sigils = had_sigils ? 1 : 0;
        lexer->mark_end(lexer);
        lexer->result_symbol = STRING_PREFIX;
        return true;
    }
    if (!valid[STRING_CONTENT]) {
        return false;
    }
    s->pending_sigils = 0;
    return scan_string_content(s, lexer);
}

static bool scan_fstring_start(Scanner *s, TSLexer *lexer) {
    if (lexer->lookahead != '"') {
        return false;
    }
    advance(lexer);
    if (s->depth < MAX_DEPTH) {
        s->had_sigils[s->depth] = s->pending_sigils ? 1 : 0;
    }
    s->depth++;
    lexer->mark_end(lexer);
    lexer->result_symbol = FSTRING_START;
    return true;
}

// scan_body emits the next f-string token: a content fragment, an interpolation
// opener, or the closing quote.
static bool scan_body(Scanner *s, TSLexer *lexer) {
    bool had_sigils = top_had_sigils(s);
    bool has_content = false;
    for (;;) {
        if (lexer->eof(lexer)) {
            break;
        }
        int32_t c = lexer->lookahead;
        if (c == '"') {
            if (has_content) {
                break;
            }
            advance(lexer); // closing "
            if (had_sigils) {
                if (lexer->lookahead != '#') {
                    lexer->mark_end(lexer); // a bare `"` is literal in a sigil string
                    has_content = true;
                    continue;
                }
                lexer->mark_end(lexer); // tentatively: a fragment may end after the `"`
                while (lexer->lookahead == '#') {
                    advance(lexer);
                }
                if (lexer->lookahead == '{') {
                    // `"#...{` is a literal `"` followed by an interpolation opener,
                    // not the terminator. Emit the `"` as its own fragment; the
                    // `#...{` is re-read as INTERP_START on the next scan.
                    has_content = true;
                    break;
                }
                // Terminator: the closing sigils become the suffix token.
                if (s->depth > 0) {
                    s->depth--;
                }
                s->pending_suffix = 1;
                lexer->result_symbol = FSTRING_END;
                return true;
            }
            if (s->depth > 0) {
                s->depth--;
            }
            s->pending_suffix = 0;
            lexer->mark_end(lexer);
            lexer->result_symbol = FSTRING_END;
            return true;
        }
        if (had_sigils && c == '#') {
            if (has_content) {
                break;
            }
            while (lexer->lookahead == '#') {
                advance(lexer);
            }
            if (lexer->lookahead == '{') {
                advance(lexer);
                lexer->mark_end(lexer);
                lexer->result_symbol = INTERP_START;
                return true;
            }
            lexer->mark_end(lexer); // `#` not opening an interpolation is literal
            has_content = true;
            continue;
        }
        if (!had_sigils && c == '{') {
            advance(lexer);
            if (lexer->lookahead == '{') {
                advance(lexer);
                lexer->mark_end(lexer); // `{{` is a literal brace
                has_content = true;
                continue;
            }
            if (has_content) {
                break; // un-consumed: the `{` is handled by the next scan
            }
            lexer->mark_end(lexer);
            lexer->result_symbol = INTERP_START;
            return true;
        }
        if (!had_sigils && c == '}') {
            advance(lexer);
            if (lexer->lookahead == '}') {
                advance(lexer); // `}}` is a literal brace
            }
            lexer->mark_end(lexer);
            has_content = true;
            continue;
        }
        if (c == '\\') {
            advance(lexer);
            if (!lexer->eof(lexer)) {
                advance(lexer);
            }
            lexer->mark_end(lexer);
            has_content = true;
            continue;
        }
        advance(lexer);
        lexer->mark_end(lexer);
        has_content = true;
    }
    if (has_content) {
        lexer->result_symbol = STRING_FRAGMENT;
        return true;
    }
    return false;
}

// scan_interp_end closes an interpolation at `}` for both plain and sigil
// f-strings (sigils only mark the opener). Braces belonging to blocks or struct
// literals inside the expression are balanced by the grammar, so the parser only
// asks for INTERP_END once the expression is complete.
static bool scan_interp_end(TSLexer *lexer) {
    if (lexer->lookahead != '}') {
        return false;
    }
    advance(lexer);
    lexer->result_symbol = INTERP_END;
    return true;
}

static bool scan_suffix(Scanner *s, TSLexer *lexer) {
    while (lexer->lookahead == '#') {
        advance(lexer);
    }
    s->pending_suffix = 0;
    lexer->result_symbol = STRING_SUFFIX;
    return true;
}

bool tree_sitter_metall_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
    Scanner *s = (Scanner *)payload;
    if (valid_symbols[INTERP_END] && !valid_symbols[STRING_FRAGMENT]) {
        skip_ws(lexer);
        return scan_interp_end(lexer);
    }
    if (valid_symbols[STRING_FRAGMENT] || valid_symbols[INTERP_START] || valid_symbols[FSTRING_END]) {
        return scan_body(s, lexer);
    }
    if (valid_symbols[STRING_SUFFIX]) {
        if (s->pending_suffix && lexer->lookahead == '#') {
            return scan_suffix(s, lexer);
        }
        s->pending_suffix = 0;
        return false;
    }
    if (valid_symbols[FSTRING_START]) {
        return scan_fstring_start(s, lexer);
    }
    if (valid_symbols[STRING_CONTENT] && !valid_symbols[STRING_PREFIX] && !valid_symbols[FSTRING_PREFIX]) {
        return scan_string_content(s, lexer); // after a string_prefix
    }
    if (valid_symbols[STRING_PREFIX] || valid_symbols[FSTRING_PREFIX] || valid_symbols[STRING_CONTENT]) {
        skip_ws(lexer);
        return scan_opener(s, lexer, valid_symbols);
    }
    return false;
}

void *tree_sitter_metall_external_scanner_create(void) {
    return calloc(1, sizeof(Scanner));
}

void tree_sitter_metall_external_scanner_destroy(void *payload) { free(payload); }

unsigned tree_sitter_metall_external_scanner_serialize(void *payload, char *buffer) {
    Scanner *s = (Scanner *)payload;
    unsigned n = 0;
    unsigned depth = s->depth <= MAX_DEPTH ? s->depth : MAX_DEPTH;
    buffer[n++] = (char)s->depth;
    buffer[n++] = (char)s->pending_sigils;
    buffer[n++] = (char)s->pending_suffix;
    for (unsigned i = 0; i < depth; i++) {
        buffer[n++] = (char)s->had_sigils[i];
    }
    return n;
}

void tree_sitter_metall_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
    Scanner *s = (Scanner *)payload;
    s->depth = 0;
    s->pending_sigils = 0;
    s->pending_suffix = 0;
    if (length == 0) {
        return;
    }
    unsigned n = 0;
    s->depth = (unsigned char)buffer[n++];
    s->pending_sigils = (unsigned char)buffer[n++];
    s->pending_suffix = (unsigned char)buffer[n++];
    unsigned depth = s->depth <= MAX_DEPTH ? s->depth : MAX_DEPTH;
    for (unsigned i = 0; i < depth && n < length; i++) {
        s->had_sigils[i] = (unsigned char)buffer[n++];
    }
}
