package ast

import (
	"math/big"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

// fstrVar is the name bound to the StrWriter in a desugared f-string. The `$`
// keeps it out of the user's identifier space, so an interpolation can never
// capture it by accident.
const fstrVar = "$fstr"

// fstrSeg is one piece of an f-string: either raw literal text (not yet escape
// decoded) or a parsed interpolation expression. An interpolation may carry a `:`
// format specifier; its arguments (fmtArgs, with parallel fmtNames) are then
// dispatched to `.fmt_ext` instead of the default `.fmt`. A non-empty fmtArgs is
// what marks a segment as having a specifier.
type fstrSeg struct {
	lit      []rune
	expr     NodeID
	isLit    bool
	fmtArgs  []NodeID
	fmtNames []*Name
}

// fstrMode is how a format string is consumed.
type fstrMode int

const (
	fstrBuild   fstrMode = iota // .build(@a): a Str via a temporary StrWriter
	fstrWriteTo                 // .write_to(sw): write straight into sw, no alloc
)

// fstrSuffix is the parsed `.build(@a)` / `.write_to(sw)` that every f-string
// requires. For build, allocName/allocSpan name the arena; for write_to, target
// is the destination StrWriter expression.
type fstrSuffix struct {
	mode      fstrMode
	allocName string
	allocSpan base.Span
	target    NodeID
}

// parseFString consumes the structured f-string tokens the lexer produced
// (FStringStart, fragments, interpolations, FStringEnd) plus the required
// `.build(@a)` / `.write_to(sw)` suffix, and desugars them into a block so type
// checking and codegen see only ordinary calls. The interpolations were lexed
// inline, so each `{expr}` is parsed by the real parser with accurate spans.
func (p *Parser) parseFString(start *token.Token) (NodeID, bool) {
	_, bytes, multiline, ok := p.stringModifiers(start)
	if !ok {
		return ParseFailed, false
	}
	segs, ok := p.collectFStringSegs(start)
	if !ok {
		return ParseFailed, false
	}
	if multiline {
		segs, ok = p.dedentFStringSegs(segs, start.Span)
		if !ok {
			return ParseFailed, false
		}
	}
	sfx, ok := p.parseFStringSuffix(start, bytes)
	if !ok {
		return ParseFailed, false
	}
	return p.buildFString(start, segs, bytes, multiline, sfx)
}

// collectFStringSegs walks the lexer's f-string tokens into literal/interpolation
// segments, parsing each interpolation as an ordinary expression.
func (p *Parser) collectFStringSegs(start *token.Token) ([]fstrSeg, bool) {
	var segs []fstrSeg
	for {
		t, ok := p.mayPeek()
		if !ok {
			p.diagnostic(start.Span, "unterminated format string")
			return nil, false
		}
		switch t.Kind { //nolint:exhaustive
		case token.FStringText:
			p.next()
			segs = append(segs, fstrSeg{
				lit: []rune(t.Value), expr: ParseFailed, isLit: true, fmtArgs: nil, fmtNames: nil,
			})
		case token.FExprStart:
			p.next()
			if end, ok := p.mayPeek(); ok && end.Kind == token.FExprEnd {
				p.diagnostic(t.Span, "empty interpolation in format string")
				return nil, false
			}
			expr, ok := p.ParseExpr(0)
			if !ok {
				return nil, false
			}
			// A `:` after the expression begins a format specifier whose args are
			// parsed like call args and dispatched to `.fmt_ext`. A top-level `:` is
			// unambiguous: the grammar's only other `:` (when/match cases) is always
			// nested inside its own braces.
			var fmtArgs []NodeID
			var fmtNames []*Name
			if c, ok := p.mayPeek(); ok && c.Kind == token.Colon {
				p.next()
				if end, ok := p.mayPeek(); ok && end.Kind == token.FExprEnd {
					p.diagnostic(c.Span, "empty format specifier")
					return nil, false
				}
				fmtArgs, fmtNames, ok = p.parseArgEntries(token.FExprEnd)
				if !ok {
					return nil, false
				}
			} else if _, ok := p.expect(token.FExprEnd); !ok {
				return nil, false
			}
			segs = append(segs, fstrSeg{lit: nil, expr: expr, isLit: false, fmtArgs: fmtArgs, fmtNames: fmtNames})
		case token.FStringEnd:
			p.next()
			return segs, true
		case token.Error:
			p.next()
			p.diagnostic(t.Span, "%s", t.Value)
			return nil, false
		default:
			p.diagnostic(t.Span, "unexpected token in format string: %s", t.Kind)
			return nil, false
		}
	}
}

// fstrInterpSentinel marks an interpolation's position while reusing the plain
// multi-line dedent on the literal text. A raw NUL never appears in source (a `\0`
// escape is the two runes `\` and `0`), so it cannot collide with literal text.
const fstrInterpSentinel = '\x00'

// dedentFStringSegs applies the m"..." dedent to an f-string's literal text without
// touching the interpolated expressions. It reuses dedentMultiline by building a
// template in which each interpolation is one sentinel rune, dedenting that, then
// splitting the result back into literal/interpolation segments.
func (p *Parser) dedentFStringSegs(segs []fstrSeg, span base.Span) ([]fstrSeg, bool) {
	var template []rune
	var interps []fstrSeg
	for _, s := range segs {
		if s.isLit {
			template = append(template, s.lit...)
		} else {
			template = append(template, fstrInterpSentinel)
			interps = append(interps, s)
		}
	}
	dedented, ok := p.dedentMultiline(template, span)
	if !ok {
		return nil, false
	}
	var out []fstrSeg
	var lit []rune
	i := 0
	flush := func() {
		if len(lit) > 0 {
			out = append(out, fstrSeg{
				lit: lit, expr: ParseFailed, isLit: true, fmtArgs: nil, fmtNames: nil,
			})
			lit = nil
		}
	}
	for _, r := range dedented {
		if r == fstrInterpSentinel {
			flush()
			out = append(out, interps[i])
			i++
			continue
		}
		lit = append(lit, r)
	}
	flush()
	return out, true
}

// parseFStringSuffix consumes the required `.build(@a)` or `.write_to(sw)` after a
// format string. A bytes f-string (`b`) only makes sense with `.build`, since
// `.write_to` produces no returned value to reinterpret.
func (p *Parser) parseFStringSuffix(t *token.Token, bytes bool) (fstrSuffix, bool) {
	if dot, ok := p.mayPeek(); !ok || dot.Kind != token.Dot {
		p.diagnostic(t.Span, `a format string must be followed by .build(@a) or .write_to(sw)`)
		return fstrSuffix{}, false
	}
	p.next()
	method, ok := p.next()
	if !ok {
		return fstrSuffix{}, false
	}
	if method.Kind != token.Ident {
		p.diagnostic(method.Span, "expected .build or .write_to after a format string, got %s", method.Kind)
		return fstrSuffix{}, false
	}
	switch method.Value {
	case "build":
		if _, ok := p.expect(token.LParen); !ok {
			return fstrSuffix{}, false
		}
		at, ok := p.next()
		if !ok {
			return fstrSuffix{}, false
		}
		if at.Kind != token.AllocatorIdent {
			p.diagnostic(at.Span, "expected an allocator @-identifier in .build(), got %s", at.Kind)
			return fstrSuffix{}, false
		}
		if _, ok := p.expect(token.RParen); !ok {
			return fstrSuffix{}, false
		}
		return fstrSuffix{mode: fstrBuild, allocName: at.Value, allocSpan: at.Span, target: ParseFailed}, true
	case "write_to":
		if bytes {
			p.diagnostic(method.Span, "a bytes format string (b) cannot be used with .write_to; use .build(@a)")
			return fstrSuffix{}, false
		}
		if _, ok := p.expect(token.LParen); !ok {
			return fstrSuffix{}, false
		}
		target, ok := p.ParseExpr(0)
		if !ok {
			return fstrSuffix{}, false
		}
		if _, ok := p.expect(token.RParen); !ok {
			return fstrSuffix{}, false
		}
		return fstrSuffix{mode: fstrWriteTo, allocName: "", allocSpan: base.Span{}, target: target}, true
	default:
		p.diagnostic(method.Span, "expected .build or .write_to after a format string, got .%s", method.Value)
		return fstrSuffix{}, false
	}
}

// buildFString assembles the desugared block. Both modes bind the StrWriter to
// `$fstr` and append every segment with `$fstr.write(...)`. build wraps that with
// a fresh temporary StrWriter and an `as_str` (or `as_bytes`) result; write_to
// binds the caller's StrWriter instead, evaluated once, and yields void.
func (p *Parser) buildFString(
	t *token.Token, segs []fstrSeg, bytes, multiline bool, sfx fstrSuffix,
) (NodeID, bool) {
	span := t.Span
	writes, ok := p.emitWrites(segs, bytes, multiline, span)
	if !ok {
		return ParseFailed, false
	}
	if sfx.mode == fstrWriteTo {
		exprs := make([]NodeID, 0, len(writes)+1)
		exprs = append(exprs, p.NewVar(Name{fstrVar, span}, nil, sfx.target, false, false, span))
		exprs = append(exprs, writes...)
		return p.NewBlock(exprs, span), true
	}
	newCall := p.NewCall(
		p.NewIdent("StrWriter.new", nil, span),
		[]NodeID{p.NewInt(big.NewInt(int64(fstrCapacity(segs))), span), p.NewIdent(sfx.allocName, nil, sfx.allocSpan)},
		nil, false, span,
	)
	exprs := make([]NodeID, 0, len(writes)+2)
	exprs = append(exprs, p.NewVar(Name{fstrVar, span}, nil, newCall, false, false, span))
	exprs = append(exprs, writes...)
	result := p.fstrCall("as_str", nil, span)
	if bytes {
		result = p.NewCall(p.NewFieldAccess(result, Name{"as_bytes", span}, nil, span), nil, nil, false, span)
	}
	exprs = append(exprs, result)
	return p.NewBlock(exprs, span), true
}

// emitWrites turns each segment into a `$fstr.write(...)`. write is generic over
// HasFmt, so a literal Str and an interpolated value use the same call. A segment
// with a format specifier instead becomes `expr.fmt_ext($fstr, <args>)`: the
// StrWriter leads the parsed spec args, so the type's fmt_ext signature decides
// which names and positions are valid and ordinary type-checking validates them.
func (p *Parser) emitWrites(segs []fstrSeg, bytes, multiline bool, span base.Span) ([]NodeID, bool) {
	var writes []NodeID
	for _, s := range segs {
		if !s.isLit {
			if len(s.fmtArgs) == 0 {
				writes = append(writes, p.fstrCall("write", []NodeID{s.expr}, span))
				continue
			}
			callArgs := append([]NodeID{p.NewIdent(fstrVar, nil, span)}, s.fmtArgs...)
			var callNames []*Name
			if s.fmtNames != nil {
				callNames = append([]*Name{nil}, s.fmtNames...)
			}
			callee := p.NewFieldAccess(s.expr, Name{"fmt_ext", span}, nil, span)
			writes = append(writes, p.NewCall(callee, callArgs, callNames, false, span))
			continue
		}
		value, ok := p.unescapeSegment(s.lit, bytes, multiline, span)
		if !ok {
			return nil, false
		}
		if value == "" {
			continue
		}
		writes = append(writes, p.fstrCall("write", []NodeID{p.NewString(value, false, span)}, span))
	}
	return writes, true
}

// fstrCapacity estimates the StrWriter capacity for a build: literals contribute
// their length, interpolations a flat guess.
func fstrCapacity(segs []fstrSeg) int {
	capacity := 16
	for _, s := range segs {
		if s.isLit {
			capacity += len(s.lit)
		} else {
			capacity += 16
		}
	}
	return capacity
}

func (p *Parser) fstrCall(method string, args []NodeID, span base.Span) NodeID {
	recv := p.NewIdent(fstrVar, nil, span)
	return p.NewCall(p.NewFieldAccess(recv, Name{method, span}, nil, span), args, nil, false, span)
}
