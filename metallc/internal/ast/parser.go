package ast

import (
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

const ParseFailed = NodeID(0)

type Parser struct {
	*AST
	Diagnostics  base.Diagnostics
	tokens       []token.Token
	pos          int
	nextFunLitID int
}

func NewParser(tokens []token.Token, a *AST) *Parser {
	// Strip comments and whitespace tokens.
	stripped := []token.Token{}
	for _, t := range tokens {
		switch t.Kind { //nolint:exhaustive
		case token.Comment, token.Whitespace:
		default:
			stripped = append(stripped, t)
		}
	}
	return &Parser{a, base.Diagnostics{}, stripped, 0, 0}
}

func (p *Parser) ParseModule() (NodeID, bool) {
	span := p.span()
	source := span.Source
	decls, ok := p.ParseDecls()
	if !ok {
		return ParseFailed, false
	}
	return p.NewModule(source.FileName, source.Module, source.Main, decls, span.Combine(p.span())), true
}

func (p *Parser) ParseDecls() ([]NodeID, bool) { //nolint:funlen
	decls := make([]NodeID, 0)
	result := true
	for {
		t, ok := p.mayPeek()
		if !ok || t.Kind == token.HashEnd {
			return decls, result
		}
		switch {
		case p.lookAhead(token.Fun) ||
			p.lookAhead(token.Unsafe, token.Fun) ||
			p.lookAhead(token.Unsync, token.Fun) ||
			p.lookAhead(token.Unsync, token.Unsafe, token.Fun) ||
			p.lookAhead(token.Pub, token.Fun) ||
			p.lookAhead(token.Pub, token.Unsafe, token.Fun) ||
			p.lookAhead(token.Pub, token.Unsync, token.Fun) ||
			p.lookAhead(token.Pub, token.Unsync, token.Unsafe, token.Fun):
			if fun, ok := p.ParseFun(); ok {
				decls = append(decls, fun)
			}
		case p.lookAheadTypeDecl(token.Struct):
			if struct_, ok := p.ParseStruct(); ok {
				decls = append(decls, struct_)
			}
		case p.lookAhead(token.Shape) || p.lookAhead(token.Pub, token.Shape):
			if shape, ok := p.ParseShape(); ok {
				decls = append(decls, shape)
			}
		case p.lookAheadTypeDecl(token.Union):
			if union, ok := p.ParseUnion(); ok {
				decls = append(decls, union)
			}
		case p.lookAheadTypeDecl(token.Enum):
			if enum, ok := p.ParseEnum(); ok {
				decls = append(decls, enum)
			}
		case p.lookAhead(token.Extern) || p.lookAhead(token.Pub, token.Extern):
			if decl, ok := p.ParseExternFun(); ok {
				decls = append(decls, decl)
			}
		case p.lookAhead(token.Let) || p.lookAhead(token.Pub, token.Let):
			if v, ok := p.ParseVar(); ok {
				decls = append(decls, v)
			}
		case t.Kind == token.Use:
			if imp, ok := p.ParseImport(); ok {
				decls = append(decls, imp)
			}
		case t.Kind == token.Export:
			if exp, ok := p.ParseExport(); ok {
				decls = append(decls, exp)
			}
		case t.Kind == token.HashIf:
			if compIf, ok := p.parseCompIf(p.ParseDecls); ok {
				decls = append(decls, compIf)
			}
		case t.Kind == token.Ident:
			if expr, ok := p.ParseExpr(0); ok {
				decls = append(decls, expr)
			}
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return decls, false
		}
	}
}

func (p *Parser) ParseExport() (NodeID, bool) {
	t, ok := p.expect(token.Export)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	nameTok, ok := p.expect(token.Ident)
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.Eq); !ok {
		return ParseFailed, false
	}
	target, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	return p.NewExport(Name{nameTok.Value, nameTok.Span}, target, span.Combine(p.span())), true
}

func (p *Parser) ParseImport() (NodeID, bool) {
	t, ok := p.expect(token.Use)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	next, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	var alias *Name
	if next.Kind == token.Ident {
		if peek1, ok := p.mayPeek1(); ok && peek1.Kind == token.Eq {
			p.next()
			p.next()
			a := Name{next.Value, next.Span}
			alias = &a
		}
	}
	segments := []string{}
	for {
		segment, ok := p.expect(token.Ident)
		if !ok {
			return ParseFailed, false
		}
		segments = append(segments, segment.Value)
		t, ok := p.mayPeek()
		if !ok || t.Kind != token.Dot {
			break
		}
		p.next()
	}
	return p.NewImport(alias, segments, span.Combine(p.span())), true
}

func (p *Parser) ParseFunType() (NodeID, bool) {
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	sync := p.parseSyncMode()
	if _, ok := p.expect(token.Fun); !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.LParen); !ok {
		return ParseFailed, false
	}
	params := []NodeID{}
	noescapeParams := []bool{}
	for {
		t, ok := p.mustPeek()
		if !ok {
			return ParseFailed, false
		}
		if t.Kind == token.RParen {
			break
		}
		if len(params) > 0 {
			if _, ok := p.expect(token.Comma); !ok {
				return ParseFailed, false
			}
		}
		noescapeParams = append(noescapeParams, p.lookAheadConsume(token.Noescape))
		param, ok := p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		params = append(params, param)
	}
	if _, ok := p.expect(token.RParen); !ok {
		return ParseFailed, false
	}
	noescapeReturn := p.lookAheadConsume(token.Noescape)
	returnTyp, ok := p.ParseType()
	if !ok {
		return ParseFailed, false
	}
	return p.NewFunType(params, returnTyp, sync, noescapeParams, noescapeReturn, span.Combine(p.span())), ok
}

func (p *Parser) ParseFunDecl() (NodeID, bool) {
	pub := p.lookAheadConsume(token.Pub)
	decl, startSpan, ok := p.parseFunDecl()
	if !ok {
		return ParseFailed, false
	}
	return p.NewFunDecl(decl.Name, decl.TypeParams, decl.Params, decl.ReturnType,
		pub, false, false, decl.NoescapeReturn, startSpan.Combine(p.span())), true
}

func (p *Parser) ParseExternFun() (NodeID, bool) {
	pub := p.lookAheadConsume(token.Pub)
	if _, ok := p.expect(token.Extern); !ok {
		return ParseFailed, false
	}

	// Optional link name: extern("c_name") fun ...
	var externName string
	if p.lookAheadConsume(token.LParen) {
		linkTok, ok := p.expect(token.String)
		if !ok {
			return ParseFailed, false
		}
		externName = linkTok.Value
		if _, ok := p.expect(token.RParen); !ok {
			return ParseFailed, false
		}
	}

	decl, startSpan, ok := p.parseFunDecl()
	if !ok {
		return ParseFailed, false
	}
	if externName == "" {
		externName = decl.Name.Name
	}
	return p.NewExternFunDecl(decl.Name, externName, decl.TypeParams, decl.Params, decl.ReturnType,
		pub, decl.NoescapeReturn, startSpan.Combine(p.span())), true
}

func (p *Parser) ParseFun() (NodeID, bool) {
	pub := p.lookAheadConsume(token.Pub)
	sync := p.parseSyncMode()
	unsafe := p.lookAheadConsume(token.Unsafe)
	decl, startSpan, ok := p.parseFunDecl()
	if !ok {
		return ParseFailed, false
	}
	block, ok := p.ParseBlock()
	if !ok {
		return ParseFailed, false
	}
	return p.NewFun(decl.Name, decl.TypeParams, decl.Params, decl.ReturnType, block,
		pub, unsafe, decl.NoescapeReturn, sync, startSpan.Combine(p.span())), true
}

func (p *Parser) ParseReturn() (NodeID, bool) {
	t, ok := p.expect(token.Return)
	if !ok {
		return ParseFailed, false
	}
	expr, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	return p.NewReturn(expr, t.Span.Combine(p.span())), true
}

func (p *Parser) ParseStructFields(stopAt ...token.TokenKind) ([]NodeID, bool) {
	fields := []NodeID{}
	for {
		t, ok := p.mayPeek()
		if !ok {
			return fields, true
		}
		if slices.Contains(stopAt, t.Kind) {
			return fields, true
		}
		span := t.Span
		var name Name
		pub := false
		if t.Kind == token.Pub {
			// If `pub` is followed by a stop token, don't consume it --
			// the caller handles it (e.g. `pub fun` in shapes).
			if next, ok := p.mayPeek1(); ok && slices.Contains(stopAt, next.Kind) {
				return fields, true
			}
			pub = true
			p.next()
			t, ok = p.mustPeek()
			if !ok {
				return nil, false
			}
		}
		switch t.Kind { //nolint:exhaustive
		case token.Ident, token.AllocatorIdent:
			name = Name{t.Value, t.Span}
			p.next()
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return nil, false
		}
		type_, ok := p.ParseType()
		if !ok {
			return nil, false
		}
		fields = append(fields, p.NewStructField(name, type_, pub, span.Combine(p.span())))
	}
}

func (p *Parser) ParseStruct() (NodeID, bool) {
	pub := p.lookAheadConsume(token.Pub)
	nocopy := p.lookAheadConsume(token.Nocopy)
	unsafe := p.lookAheadConsume(token.Unsafe)
	sync := p.parseSyncMode()
	t, ok := p.expect(token.Struct)
	if !ok {
		return ParseFailed, false
	}
	nameToken, ok := p.expect(token.TypeIdent)
	if !ok {
		return ParseFailed, false
	}
	name := Name{nameToken.Value, nameToken.Span}
	typeParams, ok := p.parseTypeParams()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.LCurly); !ok {
		return ParseFailed, false
	}
	fields, ok := p.ParseStructFields(token.RCurly)
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.RCurly); !ok {
		return ParseFailed, false
	}
	return p.NewStruct(name, typeParams, fields, pub, nocopy, sync, unsafe, t.Span.Combine(p.span())), true
}

func (p *Parser) ParseShape() (NodeID, bool) {
	pub := p.lookAheadConsume(token.Pub)
	t, ok := p.expect(token.Shape)
	if !ok {
		return ParseFailed, false
	}
	nameToken, ok := p.expect(token.TypeIdent)
	if !ok {
		return ParseFailed, false
	}
	name := Name{nameToken.Value, nameToken.Span}
	typeParams, ok := p.parseTypeParams()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.LCurly); !ok {
		return ParseFailed, false
	}
	var funs []NodeID
	for {
		next, ok := p.mayPeek()
		if !ok {
			return ParseFailed, false
		}
		if next.Kind == token.RCurly {
			p.next()
			break
		}
		if p.lookAhead(token.Fun) || p.lookAhead(token.Pub, token.Fun) {
			funDecl, ok := p.ParseFunDecl()
			if !ok {
				return ParseFailed, false
			}
			funs = append(funs, funDecl)
			continue
		}
		if _, ok := p.expect(token.Fun); !ok {
			return ParseFailed, false
		}
	}
	return p.NewShape(name, typeParams, funs, pub, t.Span.Combine(p.span())), true
}

func (p *Parser) ParseUnion() (NodeID, bool) {
	pub := p.lookAheadConsume(token.Pub)
	nocopy := p.lookAheadConsume(token.Nocopy)
	unsafe := p.lookAheadConsume(token.Unsafe)
	sync := p.parseSyncMode()
	t, ok := p.expect(token.Union)
	if !ok {
		return ParseFailed, false
	}
	nameToken, ok := p.expect(token.TypeIdent)
	if !ok {
		return ParseFailed, false
	}
	name := Name{nameToken.Value, nameToken.Span}
	typeParams, ok := p.parseTypeParams()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.Eq); !ok {
		return ParseFailed, false
	}
	p.lookAheadConsume(token.Pipe) // optional leading `|`
	var variants []NodeID
	for {
		variant, ok := p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		variants = append(variants, variant)
		next, ok := p.mayPeek()
		if !ok || next.Kind != token.Pipe {
			break
		}
		p.next()
	}
	if len(variants) < 2 {
		p.diagnostic(p.span(), "union requires at least 2 variants")
		return ParseFailed, false
	}
	return p.NewUnion(name, typeParams, variants, pub, nocopy, sync, unsafe, t.Span.Combine(p.span())), true
}

func (p *Parser) ParseEnum() (NodeID, bool) {
	pub := p.lookAheadConsume(token.Pub)
	t, ok := p.expect(token.Enum)
	if !ok {
		return ParseFailed, false
	}
	nameToken, ok := p.expect(token.TypeIdent)
	if !ok {
		return ParseFailed, false
	}
	name := Name{nameToken.Value, nameToken.Span}
	if next, ok := p.mayPeek(); ok && (next.Kind == token.Lt || next.Kind == token.LtImmediate) {
		p.diagnostic(next.Span, "enums cannot be generic")
		return ParseFailed, false
	}
	var schema []NodeID
	if next, ok := p.mayPeek(); ok && next.Kind == token.LParen {
		schema, ok = p.ParseFunParams()
		if !ok {
			return ParseFailed, false
		}
	}
	// `U8` (backing int) or `AppErr` (open root) — the type checker classifies which.
	backing, ok := p.ParseType()
	if !ok {
		return ParseFailed, false
	}
	var variants []NodeID
	open := true
	if next, ok := p.mayPeek(); ok && next.Kind == token.Eq {
		p.next()
		open = false
		p.lookAheadConsume(token.Pipe) // optional leading `|`
		if next, ok := p.mayPeek(); !ok || next.Kind != token.Ident {
			p.diagnostic(p.span(), "enum %s: expected at least one variant after '='", name.Name)
			return ParseFailed, false
		}
		for {
			variant, ok := p.parseEnumVariant()
			if !ok {
				return ParseFailed, false
			}
			variants = append(variants, variant)
			next, ok := p.mayPeek()
			if !ok || next.Kind != token.Pipe {
				break
			}
			p.next()
		}
	}
	return p.NewEnum(
		name, backing, schema, variants, open, pub, t.Span.Combine(p.span()),
	), true
}

func (p *Parser) ParseTypeConstruction() (NodeID, bool) {
	struct_, ok := p.expect(token.TypeIdent)
	if !ok {
		return ParseFailed, false
	}
	typeArgs, ok := p.parseTypeArgs()
	if !ok {
		return ParseFailed, false
	}
	ident := p.NewIdent(struct_.Value, typeArgs, struct_.Span.Combine(p.span()))
	args, argNames, ok := p.ParseCallArgs()
	if !ok {
		return ParseFailed, false
	}
	return p.NewTypeConstruction(ident, args, argNames, struct_.Span.Combine(p.span())), true
}

func (p *Parser) ParseArrayLiteralOrConstruction() (NodeID, bool) {
	t, ok := p.expect(token.LBracket)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	first, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	// `[N of v]` and `[N uninit T]` construct a fixed array. `of`/`uninit` are
	// contextual keywords (plain idents elsewhere), recognized only right after
	// the count, so they never collide with the array-literal grammar.
	if kw, ok := p.mayPeek(); ok && kw.Kind == token.Ident && (kw.Value == "of" || kw.Value == "uninit") {
		return p.parseArrayConstruction(first, span)
	}
	elems := []NodeID{first}
	for {
		t, ok := p.next()
		if !ok {
			return ParseFailed, false
		}
		if t.Kind == token.RBracket {
			break
		}
		if t.Kind != token.Comma {
			p.diagnostic(
				t.Span,
				"unexpected token: expected on of %s, got %s",
				token.PrettyPrintTokenKinds([]token.TokenKind{token.RBracket, token.Comma}),
				t.Kind,
			)
			return ParseFailed, false
		}
		if next, ok := p.mayPeek(); ok && next.Kind == token.RBracket {
			p.next()
			break
		}
		expr, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		elems = append(elems, expr)
	}
	return p.NewArrayLiteral(elems, span.Combine(p.span())), true
}

func (p *Parser) ParseBlock() (NodeID, bool) {
	t, ok := p.expect(token.LCurly)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	// parseBody reads expressions until `}` or `#end` (without consuming
	// either). It's recursive to handle nested `#if`s within blocks.
	var parseBody func() ([]NodeID, bool)
	parseBody = func() ([]NodeID, bool) {
		exprs := []NodeID{}
		for {
			t, ok := p.mayPeek()
			if !ok || t.Kind == token.RCurly || t.Kind == token.HashEnd {
				return exprs, true
			}
			var expr NodeID
			if t.Kind == token.HashIf {
				expr, ok = p.parseCompIf(parseBody)
			} else {
				expr, ok = p.parseStmtExpr()
			}
			if !ok {
				return nil, false
			}
			exprs = append(exprs, expr)
		}
	}
	exprs, ok := parseBody()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.RCurly); !ok {
		return ParseFailed, false
	}
	return p.NewBlock(exprs, span.Combine(p.span())), true
}

func (p *Parser) ParseExpr(minPrecedence int) (NodeID, bool) { //nolint:funlen
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	var lhs NodeID
	next, hasNext := p.mayPeek1()
	// Unary prefix operators `not`, `~`, `-` bind tighter than every binary operator
	// (operand at precedence 9, above the highest binary at 8) and feed the binary
	// loop below, so `~a | b` is `(~a) | b` and `not a == b` is `(not a) == b`
	// (matching Rust/Zig). `-<number literal>` is folded as a primary instead, so
	// postfix binds to it (`-5.abs()` is `(-5).abs()`), hence the Number guard.
	isMinus := t.Kind == token.Minus || t.Kind == token.MinusAfterNewline
	unaryMinus := isMinus && (!hasNext || next.Kind != token.Number)
	if t.Kind == token.Not || t.Kind == token.Tilde || unaryMinus {
		p.next()
		operand, opOk := p.ParseExpr(9)
		if !opOk {
			return ParseFailed, false
		}
		op := map[token.TokenKind]UnaryOp{
			token.Not:               UnaryOpNot,
			token.Tilde:             UnaryOpBitNot,
			token.Minus:             UnaryOpNeg,
			token.MinusAfterNewline: UnaryOpNeg,
		}[t.Kind]
		lhs = p.NewUnary(op, operand, span.Combine(p.span()))
	} else {
		lhs, ok = p.ParsePostfixExpr(0)
		if !ok {
			return ParseFailed, false
		}
	}
	t, ok = p.mayPeek()
	if !ok {
		return lhs, true
	}
	if t.Kind == token.Eq {
		p.next()
		rhs, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		return p.NewAssign(lhs, rhs, span.Combine(p.span())), true
	}
	if op, ok := map[token.TokenKind]BinaryOp{
		token.PlusEq:         BinaryOpAdd,
		token.MinusEq:        BinaryOpSub,
		token.StarEq:         BinaryOpMul,
		token.SlashEq:        BinaryOpDiv,
		token.PercentEq:      BinaryOpMod,
		token.PlusPercentEq:  BinaryOpWrapAdd,
		token.MinusPercentEq: BinaryOpWrapSub,
		token.StarPercentEq:  BinaryOpWrapMul,
		token.AmpEq:          BinaryOpBitAnd,
		token.PipeEq:         BinaryOpBitOr,
		token.CaretEq:        BinaryOpBitXor,
		token.LtLtEq:         BinaryOpShl,
		token.GtGtEq:         BinaryOpShr,
	}[t.Kind]; ok {
		p.next()
		rhs, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		return p.NewCompoundAssign(op, lhs, rhs, span.Combine(p.span())), true
	}
	for {
		t, ok = p.mayPeek()
		if !ok {
			return lhs, true
		}
		op, ok := map[token.TokenKind]BinaryOp{
			token.Plus:         BinaryOpAdd,
			token.Minus:        BinaryOpSub,
			token.Star:         BinaryOpMul,
			token.Slash:        BinaryOpDiv,
			token.Percent:      BinaryOpMod,
			token.PlusPercent:  BinaryOpWrapAdd,
			token.MinusPercent: BinaryOpWrapSub,
			token.StarPercent:  BinaryOpWrapMul,

			token.EqEq: BinaryOpEq,
			token.Neq:  BinaryOpNeq,
			token.Lt:   BinaryOpLt,
			token.Lte:  BinaryOpLte,
			token.Gt:   BinaryOpGt,
			token.Gte:  BinaryOpGte,
			token.And:  BinaryOpAnd,
			token.Or:   BinaryOpOr,

			token.Amp:   BinaryOpBitAnd,
			token.Pipe:  BinaryOpBitOr,
			token.Caret: BinaryOpBitXor,
			token.LtLt:  BinaryOpShl,
			token.GtGt:  BinaryOpShr,
		}[t.Kind]
		if !ok {
			// `..`/`..=` is the lowest-precedence operator and builds a Range value.
			// Handled only at the outermost level (minPrecedence 0) so it binds looser
			// than every binary operator; chaining `a..b..c` is rejected downstream.
			if minPrecedence == 0 && t.Kind == token.DotDot {
				return p.parseRangeRHS(&lhs)
			}
			return lhs, true
		}
		precedence := map[BinaryOp]int{
			// Bitwise & ^ | bind tighter than comparison (Rust/Zig order), not
			// looser as in C. `x & 1 == 0` is `(x & 1) == 0`.
			BinaryOpOr:      0,
			BinaryOpAnd:     1,
			BinaryOpEq:      2,
			BinaryOpNeq:     2,
			BinaryOpLt:      2,
			BinaryOpLte:     2,
			BinaryOpGt:      2,
			BinaryOpGte:     2,
			BinaryOpBitOr:   3,
			BinaryOpBitXor:  4,
			BinaryOpBitAnd:  5,
			BinaryOpShl:     6,
			BinaryOpShr:     6,
			BinaryOpAdd:     7,
			BinaryOpSub:     7,
			BinaryOpWrapAdd: 7,
			BinaryOpWrapSub: 7,
			BinaryOpMul:     8,
			BinaryOpDiv:     8,
			BinaryOpMod:     8,
			BinaryOpWrapMul: 8,
		}[op]
		if precedence < minPrecedence {
			return lhs, true
		}
		p.next()
		rhs, ok := p.ParseExpr(precedence + 1)
		if !ok {
			return ParseFailed, false
		}
		span = span.Combine(p.span())
		lhs = p.NewBinary(op, lhs, rhs, span)
	}
}

func (p *Parser) ParsePostfixExpr(minPrecedence int) (NodeID, bool) { //nolint:funlen
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	expr, ok := p.ParsePrimaryExpr(minPrecedence)
	if !ok {
		return ParseFailed, false
	}
	for {
		t, ok := p.mayPeek()
		if !ok {
			break
		}
		switch t.Kind { //nolint:exhaustive
		case token.LParen:
			callee := expr
			args, argNames, ok := p.ParseCallArgs()
			if !ok {
				return ParseFailed, false
			}
			if p.isStructTarget(callee) {
				expr = p.NewTypeConstruction(callee, args, argNames, span.Combine(p.span()))
			} else {
				expr = p.NewCall(callee, args, argNames, false, span.Combine(p.span()))
			}
			continue
		case token.LBracketImmediate:
			result, ok := p.ParseIndexOrSubSlice(expr, span)
			if !ok {
				return ParseFailed, false
			}
			expr = result
			continue
		case token.Dot:
			p.next()
			next, ok := p.mustPeek()
			if !ok {
				return ParseFailed, false
			}
			if next.Kind == token.Star {
				p.next()
				expr = p.NewDeref(expr, span.Combine(p.span()))
				continue
			}
			if next.Kind == token.AllocatorIdent {
				p.next()
				expr = p.NewFieldAccess(expr, Name{next.Value, next.Span}, nil, span.Combine(p.span()))
				continue
			}
			if next.Kind != token.Ident && next.Kind != token.TypeIdent {
				p.diagnostic(next.Span, "unexpected token: expected <identifier> or *, got %s", next.Kind)
				return ParseFailed, false
			}
			p.next()
			typeArgs, ok := p.parseTypeArgs()
			if !ok {
				return ParseFailed, false
			}
			expr = p.NewFieldAccess(expr, Name{next.Value, next.Span}, typeArgs, span.Combine(p.span()))
			continue
		}
		break
	}
	return expr, true
}

func (p *Parser) ParsePrimaryExpr(minPrecedence int) (NodeID, bool) { //nolint:funlen
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	var expr NodeID
	switch t.Kind { //nolint:exhaustive
	case token.LParen:
		p.next()
		expr, ok = p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		if _, ok := p.expect(token.RParen); !ok {
			return ParseFailed, false
		}
	case token.Amp, token.AmpAfterNewline:
		ref, ok := p.ParseRefExpr()
		if !ok {
			return ParseFailed, false
		}
		expr = ref
	case token.Fun:
		var fun NodeID
		var ok bool
		if next, peekOK := p.mayPeek1(); peekOK &&
			(next.Kind == token.LParen || next.Kind == token.LBracket || next.Kind == token.LBracketImmediate) {
			fun, ok = p.parseFunLiteral(SyncNone)
		} else {
			fun, ok = p.ParseFun()
		}
		if !ok {
			return ParseFailed, false
		}
		expr = fun
	case token.Struct:
		struct_, ok := p.ParseStruct()
		if !ok {
			return ParseFailed, false
		}
		expr = struct_
	case token.Nocopy:
		// nocopy can precede struct or union.
		if p.lookAheadTypeDecl(token.Struct) {
			struct_, ok := p.ParseStruct()
			if !ok {
				return ParseFailed, false
			}
			expr = struct_
		} else {
			union, ok := p.ParseUnion()
			if !ok {
				return ParseFailed, false
			}
			expr = union
		}
	case token.Shape:
		shape, ok := p.ParseShape()
		if !ok {
			return ParseFailed, false
		}
		expr = shape
	case token.Union:
		union, ok := p.ParseUnion()
		if !ok {
			return ParseFailed, false
		}
		expr = union
	case token.Enum:
		enum, ok := p.ParseEnum()
		if !ok {
			return ParseFailed, false
		}
		expr = enum
	case token.Match:
		match, ok := p.ParseMatch()
		if !ok {
			return ParseFailed, false
		}
		expr = match
	case token.When:
		when, ok := p.ParseWhen()
		if !ok {
			return ParseFailed, false
		}
		expr = when
	case token.Try:
		try_, ok := p.parseTry()
		if !ok {
			return ParseFailed, false
		}
		expr = try_
	case token.If:
		if_, ok := p.ParseIf()
		if !ok {
			return ParseFailed, false
		}
		expr = if_
	case token.For:
		for_, ok := p.ParseFor()
		if !ok {
			return ParseFailed, false
		}
		expr = for_
	case token.Break:
		p.diagnostic(t.Span, "break may only be used as a statement, not inside an expression")
		return ParseFailed, false
	case token.Continue:
		p.diagnostic(t.Span, "continue may only be used as a statement, not inside an expression")
		return ParseFailed, false
	case token.Defer:
		p.next()
		block, ok := p.ParseBlock()
		if !ok {
			return ParseFailed, false
		}
		expr = p.NewDefer(block, t.Span.Combine(p.span()))
	case token.Return:
		p.diagnostic(t.Span, "return may only be used as a statement, not inside an expression")
		return ParseFailed, false
	case token.Ident:
		ident, ok := p.ParseIdent()
		if !ok {
			return ParseFailed, false
		}
		expr = ident
	case token.TypeIdent:
		// Peek ahead: if the next token is `.`, this is a qualified name
		// (e.g. `Foo.bar`), not a type construction.
		if next, ok := p.mayPeek1(); ok && next.Kind == token.Dot {
			p.next()
			p.next()
			methodToken, ok := p.expect(token.Ident)
			if !ok {
				return ParseFailed, false
			}
			qualifiedName := t.Value + "." + methodToken.Value
			typeArgs, ok := p.parseTypeArgs()
			if !ok {
				return ParseFailed, false
			}
			expr = p.NewIdent(qualifiedName, typeArgs, t.Span.Combine(p.span()))
		} else {
			construction, ok := p.ParseTypeConstruction()
			if !ok {
				return ParseFailed, false
			}
			expr = construction
		}
	case token.AllocatorIdent:
		p.next()
		expr = p.NewIdent(t.Value, nil, t.Span)
	case token.LBracket:
		if next, ok := p.mayPeek1(); ok && next.Kind == token.RBracket {
			p.next()
			p.next()
			expr = p.NewEmptySlice(t.Span.Combine(next.Span))
		} else {
			array, ok := p.ParseArrayLiteralOrConstruction()
			if !ok {
				return ParseFailed, false
			}
			expr = array
		}
	case token.Minus, token.MinusAfterNewline, token.Number:
		// A number literal, with an optional `-` folded in. Folding here (rather
		// than at the unary-minus level) keeps `-5` a primary, so postfix binds to
		// the negative literal: `-5.abs()` is `(-5).abs()`.
		num, ok := p.ParseNumber()
		if !ok {
			return ParseFailed, false
		}
		expr = num
	case token.Float:
		f, ok := p.ParseFloat()
		if !ok {
			return ParseFailed, false
		}
		expr = f
	case token.True:
		p.next()
		expr = p.NewBool(true, t.Span)
	case token.False:
		p.next()
		expr = p.NewBool(false, t.Span)
	case token.String:
		p.next()
		expr = p.NewString(t.Value, t.Span.Combine(p.span()))
	case token.Bytes:
		p.next()
		expr = p.NewBytes(t.Value, t.Span.Combine(p.span()))
	case token.Rune:
		p.next()
		runes := []rune(t.Value)
		expr = p.NewRuneLiteral(uint32(runes[0]), t.Span.Combine(p.span()))
	case token.LCurly:
		block, ok := p.ParseBlock()
		if !ok {
			return ParseFailed, false
		}
		expr = block
	case token.Let, token.Mut:
		var_, ok := p.ParseVar()
		if !ok {
			return ParseFailed, false
		}
		expr = var_
	case token.Unsync:
		// `unsync fun[...]()` or `unsync fun()` -> literal; `unsync fun name(...)` -> declaration.
		if p.lookAhead(token.Unsync, token.Fun, token.LParen) ||
			p.lookAhead(token.Unsync, token.Fun, token.LBracket) ||
			p.lookAhead(token.Unsync, token.Fun, token.LBracketImmediate) {
			p.next() // consume unsync
			fun, ok := p.parseFunLiteral(SyncUnsync)
			if !ok {
				return ParseFailed, false
			}
			expr = fun
		} else {
			return p.ParseFun()
		}
	case token.Unsafe:
		if p.lookAhead(token.Unsafe, token.Fun) {
			return p.ParseFun()
		}
		p.next()
		span := t.Span
		inner, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		switch kind := p.Node(inner).Kind.(type) {
		case Call:
			kind.Unsafe = true
			p.Node(inner).Kind = kind
		case ArrayConstruction:
			kind.Unsafe = true
			p.Node(inner).Kind = kind
		default:
			p.diagnostic(span, "unsafe can only be applied to function calls")
			return ParseFailed, false
		}
		return inner, true
	default:
		p.diagnostic(t.Span, "unexpected token: expected start of an expression, got %s", t.Kind)
		return ParseFailed, false
	}
	return expr, true
}

func (p *Parser) ParseRefExpr() (NodeID, bool) {
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	if t.Kind != token.Amp && t.Kind != token.AmpAfterNewline {
		p.diagnostic(t.Span, "unexpected token: expected %s, got %s", token.Amp, t.Kind)
		return ParseFailed, false
	}
	p.next()
	span := t.Span
	mut := false
	t, ok = p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	if t.Kind == token.Mut {
		p.next()
		mut = true
	}
	target, ok := p.ParsePostfixExpr(0)
	if !ok {
		return ParseFailed, false
	}
	return p.NewRef(target, mut, span.Combine(p.span())), true
}

// ParseCallArgs parses `(arg, ...)` where each arg is positional or named
// (`name = expr`). Returns the value expressions and a parallel names slice
// (nil when no argument is named; otherwise names[i] is non-nil for a named arg).
func (p *Parser) ParseCallArgs() ([]NodeID, []*Name, bool) {
	if _, ok := p.expect(token.LParen); !ok {
		return nil, nil, false
	}
	args := []NodeID{}
	var names []*Name // lazily allocated on the first named argument
	for {
		t, ok := p.mayPeek()
		if !ok {
			return args, names, true
		}
		if t.Kind == token.RParen {
			p.next()
			return args, names, true
		}
		// A named argument is `ident = expr`. `==` is a comparison, so only a
		// single `=` separates a name from its value.
		var name *Name
		if t.Kind == token.Ident {
			if next, ok := p.mayPeek1(); ok && next.Kind == token.Eq {
				p.next()
				p.next()
				name = &Name{t.Value, t.Span}
			}
		}
		if name == nil && names != nil {
			p.diagnostic(t.Span, "positional argument after named argument")
			return args, names, false
		}
		if name != nil && names == nil {
			names = make([]*Name, len(args))
		}
		var expr NodeID
		if peek, ok := p.mayPeek(); ok && peek.Kind == token.AllocatorIdent {
			p.next()
			expr = p.NewIdent(peek.Value, nil, peek.Span)
		} else {
			expr, ok = p.ParseExpr(0)
			if !ok {
				return args, names, false
			}
		}
		args = append(args, expr)
		if names != nil {
			names = append(names, name)
		}
		t, ok = p.next()
		if !ok {
			return args, names, true
		}
		switch t.Kind { //nolint:exhaustive
		case token.Comma:
		case token.RParen:
			return args, names, true
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return args, names, false
		}
	}
}

func (p *Parser) ParseAllocatorVar(span base.Span) (NodeID, bool) {
	name, ok := p.expect(token.AllocatorIdent)
	if !ok {
		return ParseFailed, false
	}
	if _, ok = p.expect(token.Eq); !ok {
		return ParseFailed, false
	}
	expr, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	return p.NewAllocatorVar(
		Name{Name: name.Value, Span: name.Span},
		expr,
		span.Combine(p.span()),
	), true
}

func (p *Parser) ParseVar() (NodeID, bool) {
	pub := p.lookAheadConsume(token.Pub)
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	mut := t.Kind == token.Mut
	if mut {
		p.next()
	} else {
		if _, ok := p.expect(token.Let); !ok {
			return ParseFailed, false
		}
	}
	span := t.Span
	// Check for allocator variable: `let @name = Arena(...)`.
	next, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	if next.Kind == token.AllocatorIdent {
		if mut {
			p.diagnostic(next.Span, "allocator variables cannot be mutable")
			return ParseFailed, false
		}
		return p.ParseAllocatorVar(span)
	}
	nameToken, ok := p.expect(token.Ident)
	if !ok {
		return ParseFailed, false
	}
	name := Name{nameToken.Value, nameToken.Span}
	var type_ *NodeID
	if next, ok := p.mustPeek(); ok && next.Kind != token.Eq {
		t, ok := p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		type_ = &t
	}
	if _, ok := p.expect(token.Eq); !ok {
		return ParseFailed, false
	}
	init, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	return p.NewVar(name, type_, init, pub, mut, span.Combine(p.span())), true
}

func (p *Parser) ParseFunParams() ([]NodeID, bool) {
	if _, ok := p.expect(token.LParen); !ok {
		return nil, false
	}
	funParams := []NodeID{}
	seenDefault := false
	for {
		t, ok := p.mayPeek()
		if !ok {
			return funParams, true
		}
		switch t.Kind { //nolint:exhaustive
		case token.Ident, token.AllocatorIdent:
			nameToken, ok := p.next()
			if !ok {
				return funParams, false
			}
			name := Name{nameToken.Value, nameToken.Span}
			noescape := p.lookAheadConsume(token.Noescape)
			type_, ok := p.ParseType()
			if !ok {
				p.diagnostic(t.Span, "expected type, got %s", t.Kind)
				return funParams, false
			}
			var defaultVal *NodeID
			if next, ok := p.mayPeek(); ok && next.Kind == token.Eq {
				p.next()
				expr, ok := p.ParseExpr(0)
				if !ok {
					return funParams, false
				}
				defaultVal = &expr
				seenDefault = true
			} else if seenDefault {
				p.diagnostic(name.Span, "parameters with default values must be last")
				return funParams, false
			}
			param := p.NewFunParam(name, type_, defaultVal, noescape, name.Span.Combine(p.span()))
			funParams = append(funParams, param)
		case token.Comma:
			p.next()
		case token.RParen:
			p.next()
			return funParams, true
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return funParams, false
		}
	}
}

func (p *Parser) ParseArrayOrSliceType() (NodeID, bool) {
	t, ok := p.next()
	if !ok {
		return ParseFailed, false
	}
	if t.Kind != token.LBracket && t.Kind != token.LBracketImmediate {
		p.diagnostic(t.Span, "unexpected token: expected [, got %s", t.Kind)
		return ParseFailed, false
	}
	span := t.Span
	var len_ *int64
	if next, ok := p.mayPeek(); ok && next.Kind == token.Number {
		n, ok := p.expectNumber()
		if !ok {
			return ParseFailed, false
		}
		if !n.IsInt64() || n.Int64() <= 0 {
			p.diagnostic(span, "invalid array length: %s", n)
			return ParseFailed, false
		}
		v := n.Int64()
		len_ = &v
	}
	if _, ok := p.expect(token.RBracket); !ok {
		return ParseFailed, false
	}
	mut := false
	if len_ == nil {
		if next, ok := p.mayPeek(); ok && next.Kind == token.Mut {
			mut = true
			p.next()
		}
	}
	typ, ok := p.ParseType()
	if !ok {
		return ParseFailed, false
	}
	if len_ == nil {
		return p.NewSliceType(typ, mut, span.Combine(p.span())), true
	}
	return p.NewArrayType(typ, *len_, span.Combine(p.span())), true
}

func (p *Parser) ParseType() (NodeID, bool) { //nolint:funlen
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	switch t.Kind { //nolint:exhaustive
	case token.Ident:
		if t.Value == "void" || t.Value == "never" {
			p.next()
			return p.NewSimpleType(Name{t.Value, span}, nil, span), true
		}
		if p.lookAheadConsume(token.Ident, token.Dot) {
			// Module-qualified type: e.g. `io.Printer`, `ffi.Ptr<Int>`
			typeIdent, ok := p.expect(token.TypeIdent)
			if !ok {
				return ParseFailed, false
			}
			typeArgs, ok := p.parseTypeArgs()
			if !ok {
				return ParseFailed, false
			}
			target := p.NewIdent(t.Value, nil, t.Span)
			return p.NewFieldAccess(
				target,
				Name{typeIdent.Value, typeIdent.Span},
				typeArgs,
				t.Span.Combine(p.span()),
			), true
		}
		p.diagnostic(span, "unexpected token: expected <type identifier> or &, got %s", t.Kind)
		return ParseFailed, false
	case token.TypeIdent:
		p.next()
		var name strings.Builder
		name.WriteString(t.Value)
		nameSpan := t.Span
		// Only continue the dotted-type chain for an uppercase segment; a
		// `.lowercase` (e.g. an enum variant) is left for the caller.
		for p.lookAhead(token.Dot, token.TypeIdent) {
			p.next()
			next, ok := p.expect(token.TypeIdent)
			if !ok {
				return ParseFailed, false
			}
			name.WriteByte('.')
			name.WriteString(next.Value)
			nameSpan = nameSpan.Combine(next.Span)
		}
		typeArgs, ok := p.parseTypeArgs()
		if !ok {
			return ParseFailed, false
		}
		return p.NewSimpleType(Name{name.String(), nameSpan}, typeArgs, span.Combine(p.span())), true
	case token.LBracket, token.LBracketImmediate:
		return p.ParseArrayOrSliceType()
	case token.Amp, token.AmpAfterNewline:
		p.next()
		mut := false
		if next, ok := p.mayPeek(); ok && next.Kind == token.Mut {
			mut = true
			p.next()
		}
		inner, ok := p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		return p.NewRefType(inner, mut, span.Combine(p.span())), true
	case token.Sync, token.Unsync, token.Fun:
		return p.ParseFunType()
	case token.Question:
		p.next()
		inner, ok := p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		return p.NewSimpleType(Name{"Option", span}, []NodeID{inner}, span.Combine(p.span())), true
	case token.Excl:
		p.next()
		inner, ok := p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		return p.NewSimpleType(Name{"Result", span}, []NodeID{inner}, span.Combine(p.span())), true
	default:
		p.diagnostic(span, "unexpected token: expected <type identifier> or &, got %s", t.Kind)
		return ParseFailed, false
	}
}

func (p *Parser) ParseFor() (NodeID, bool) { //nolint:funlen
	t, ok := p.expect(token.For)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	t, ok = p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	// Three forms:
	//   for { ... }
	//   for <cond> { ... }
	//   for [&[mut]] x [, i] in <iterable> { ... }
	// A leading `&` could be the start of a boolean-condition loop or an
	// iterating loop.
	forIn := t.Kind == token.Amp || t.Kind == token.AmpAfterNewline
	if t.Kind == token.Ident {
		next, hasNext := p.mayPeek1()
		forIn = hasNext && (next.Kind == token.In || next.Kind == token.Comma)
	}
	if forIn {
		var ref, mut bool
		if t.Kind == token.Amp || t.Kind == token.AmpAfterNewline {
			p.next()
			ref = true
			if m, ok := p.mayPeek(); ok && m.Kind == token.Mut {
				p.next()
				mut = true
			}
		}
		bindTok, ok := p.expect(token.Ident)
		if !ok {
			return ParseFailed, false
		}
		binding := Name{Name: bindTok.Value, Span: bindTok.Span}
		var index *Name
		if comma, ok := p.mayPeek(); ok && comma.Kind == token.Comma {
			p.next()
			idx, ok := p.expect(token.Ident)
			if !ok {
				return ParseFailed, false
			}
			index = &Name{Name: idx.Value, Span: idx.Span}
		}
		if _, ok := p.expect(token.In); !ok {
			return ParseFailed, false
		}
		iterable, _, ok := p.ParseRange()
		if !ok {
			return ParseFailed, false
		}
		body, ok := p.ParseBlock()
		if !ok {
			return ParseFailed, false
		}
		return p.NewFor(&binding, ref, mut, index, &iterable, body, span.Combine(p.span())), true
	}
	var cond *NodeID
	if t.Kind != token.LCurly {
		expr, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		cond = &expr
	}
	body, ok := p.ParseBlock()
	if !ok {
		return ParseFailed, false
	}
	return p.NewFor(nil, false, false, nil, cond, body, span.Combine(p.span())), true
}

func (p *Parser) ParseMatch() (NodeID, bool) {
	t, ok := p.expect(token.Match)
	if !ok {
		return ParseFailed, false
	}
	expr, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.LCurly); !ok {
		return ParseFailed, false
	}
	arms, else_, ok := p.parseMatchArms()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.RCurly); !ok {
		return ParseFailed, false
	}
	if len(arms) == 0 && else_ == nil {
		p.diagnostic(t.Span, "match requires at least one arm")
		return ParseFailed, false
	}
	return p.NewMatch(expr, arms, else_, t.Span.Combine(p.span())), true
}

func (p *Parser) ParseIf() (NodeID, bool) {
	t, ok := p.expect(token.If)
	if !ok {
		return ParseFailed, false
	}
	cond, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	then, ok := p.ParseBlock()
	if !ok {
		return ParseFailed, false
	}
	et, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	if et.Kind != token.Else {
		return p.NewIf(cond, then, nil, t.Span.Combine(p.span())), true
	}
	if p.isMatchElse() {
		return p.NewIf(cond, then, nil, t.Span.Combine(p.span())), true
	}
	p.next()
	else_, ok := p.ParseBlock()
	if !ok {
		return ParseFailed, false
	}
	return p.NewIf(cond, then, &else_, t.Span.Combine(p.span())), true
}

func (p *Parser) ParseWhen() (NodeID, bool) { //nolint:funlen
	t, ok := p.expect(token.When)
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.LCurly); !ok {
		return ParseFailed, false
	}
	var cases []WhenCase
	var elseBody *NodeID
	for {
		next, ok := p.mayPeek()
		if !ok || next.Kind == token.RCurly {
			break
		}
		switch next.Kind { //nolint:exhaustive
		case token.Case:
			p.next()
			cond, ok := p.ParseExpr(0)
			if !ok {
				return ParseFailed, false
			}
			if _, ok := p.expect(token.Colon); !ok {
				return ParseFailed, false
			}
			bodyExprs, ok := p.parseCaseBody()
			if !ok {
				return ParseFailed, false
			}
			bodySpan := p.span()
			if len(bodyExprs) > 0 {
				bodySpan = p.Node(bodyExprs[0]).Span.Combine(p.span())
			}
			cases = append(cases, WhenCase{Cond: cond, Body: p.NewBlock(bodyExprs, bodySpan)})
		case token.Else:
			p.next()
			if _, ok := p.expect(token.Colon); !ok {
				return ParseFailed, false
			}
			bodyExprs, ok := p.parseCaseBody()
			if !ok {
				return ParseFailed, false
			}
			bodySpan := p.span()
			if len(bodyExprs) > 0 {
				bodySpan = p.Node(bodyExprs[0]).Span.Combine(p.span())
			}
			body := p.NewBlock(bodyExprs, bodySpan)
			elseBody = &body
			goto done
		default:
			p.diagnostic(next.Span, "expected case or else, got %s", next.Kind)
			return ParseFailed, false
		}
	}

done:
	if _, ok := p.expect(token.RCurly); !ok {
		return ParseFailed, false
	}
	if len(cases) == 0 {
		p.diagnostic(t.Span, "when requires at least one case")
		return ParseFailed, false
	}
	return p.NewWhen(cases, elseBody, t.Span.Combine(p.span())), true
}

func (p *Parser) ParseIdent() (NodeID, bool) {
	t, ok := p.expect(token.Ident)
	if !ok {
		return ParseFailed, false
	}
	typeArgs, ok := p.parseTypeArgs()
	if !ok {
		return ParseFailed, false
	}
	return p.NewIdent(t.Value, typeArgs, t.Span.Combine(p.span())), true
}

// ParseNumber parses an integer literal with an optional leading `-`, folding it
// into one Int node. The lexer makes `-` a Minus token, so contexts that take a
// bare integer (a negative literal, an enum tag) reassemble it here. Folding the
// sign keeps signed minimums (`I8 -128`) range-checking as the negated value.
func (p *Parser) ParseNumber() (NodeID, bool) {
	start, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	neg := start.Kind == token.Minus || start.Kind == token.MinusAfterNewline
	if neg {
		p.next()
	}
	n, ok := p.expectNumber()
	if !ok {
		return ParseFailed, false
	}
	if neg {
		n.Neg(n)
	}
	return p.NewInt(n, start.Span.Combine(p.span())), true
}

func (p *Parser) ParseFloat() (NodeID, bool) {
	t, ok := p.expect(token.Float)
	if !ok {
		return ParseFailed, false
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(t.Value, "_", ""), 64)
	if err != nil {
		p.diagnostic(t.Span, "invalid float literal: %s", t.Value)
		return ParseFailed, false
	}
	return p.NewFloat(f, p.span()), true
}

func (p *Parser) ParseIndexOrSubSlice(target NodeID, span base.Span) (NodeID, bool) {
	if _, ok := p.expect(token.LBracketImmediate); !ok {
		return ParseFailed, false
	}
	range_, isRange, ok := p.ParseRange()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.RBracket); !ok {
		return ParseFailed, false
	}
	if isRange {
		return p.NewSubSlice(target, range_, span.Combine(p.span())), true
	}
	return p.NewIndex(target, range_, span.Combine(p.span())), true
}

// ParseRange parses a range (`..hi`, `lo..hi`, `lo..=hi`, `lo..`) or a plain expression.
// Returns (nodeID, isRange, ok). When isRange is false, nodeID is the parsed expression.
func (p *Parser) ParseRange() (nodeID NodeID, isRange bool, ok bool) {
	// `..hi` or `..=hi` — range without lo.
	if next, ok := p.mayPeek(); ok && next.Kind == token.DotDot {
		range_, ok := p.parseRangeRHS(nil)
		return range_, true, ok
	}
	lo, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false, false
	}
	// ParseExpr already folds a trailing `..` into a Range value (general expression
	// position); recognize that so `s[lo..hi]` is a subslice, not an index.
	if _, isRange := p.Node(lo).Kind.(Range); isRange {
		return lo, true, true
	}
	if next, ok := p.mayPeek(); ok && next.Kind == token.DotDot {
		range_, ok := p.parseRangeRHS(&lo)
		return range_, true, ok
	}
	return lo, false, true
}

func (p *Parser) parseRangeRHS(lo *NodeID) (NodeID, bool) {
	dotDot, ok := p.expect(token.DotDot)
	if !ok {
		return ParseFailed, false
	}
	rangeSpan := dotDot.Span
	if lo != nil {
		rangeSpan = p.Node(*lo).Span.Combine(rangeSpan)
	}
	inclusive := false
	if next, ok := p.mayPeek(); ok && next.Kind == token.Eq {
		p.next()
		inclusive = true
	}
	var hi *NodeID
	if next, ok := p.mayPeek(); ok && next.Kind != token.RBracket && next.Kind != token.LCurly {
		hiExpr, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		hi = &hiExpr
		rangeSpan = rangeSpan.Combine(p.Node(hiExpr).Span)
	}
	if inclusive && hi == nil {
		p.diagnostic(dotDot.Span, "inclusive range (..=) requires an upper bound")
		return ParseFailed, false
	}
	return p.NewRange(lo, hi, inclusive, rangeSpan), true
}

// parseFunLitParams parses function parameters for function literals,
// allowing parameter types to be omitted (inferred from context).
// When a type is omitted, InferredType is used as the type sentinel.
func (p *Parser) parseFunLitParams() ([]NodeID, bool) {
	if _, ok := p.expect(token.LParen); !ok {
		return nil, false
	}
	funParams := []NodeID{}
	seenDefault := false
	for {
		t, ok := p.mayPeek()
		if !ok {
			return funParams, true
		}
		switch t.Kind { //nolint:exhaustive
		case token.Ident, token.AllocatorIdent:
			nameToken, ok := p.next()
			if !ok {
				return funParams, false
			}
			name := Name{nameToken.Value, nameToken.Span}
			// Check if a type follows. If the next token is ',', ')', or '=', the type is omitted.
			type_ := InferredType
			noescape := false
			if next, ok := p.mayPeek(); ok &&
				next.Kind != token.Comma && next.Kind != token.RParen && next.Kind != token.Eq {
				noescape = p.lookAheadConsume(token.Noescape)
				parsed, ok := p.ParseType()
				if !ok {
					p.diagnostic(t.Span, "expected type, got %s", t.Kind)
					return funParams, false
				}
				type_ = parsed
			}
			var defaultVal *NodeID
			if next, ok := p.mayPeek(); ok && next.Kind == token.Eq {
				p.next()
				expr, ok := p.ParseExpr(0)
				if !ok {
					return funParams, false
				}
				defaultVal = &expr
				seenDefault = true
			} else if seenDefault {
				p.diagnostic(name.Span, "parameters with default values must be last")
				return funParams, false
			}
			param := p.NewFunParam(name, type_, defaultVal, noescape, name.Span.Combine(p.span()))
			funParams = append(funParams, param)
		case token.Comma:
			p.next()
		case token.RParen:
			p.next()
			return funParams, true
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return funParams, false
		}
	}
}

// lookAheadTypeDecl checks if the next tokens form a type declaration with the
// given keyword (struct or union). Handles [pub] [nocopy] [unsafe] [sync|unsync].
// `unsafe` only appears paired with `sync` (enforced when checking the decl).
func (p *Parser) lookAheadTypeDecl(kw token.TokenKind) bool {
	return p.lookAhead(kw) ||
		p.lookAhead(token.Pub, kw) ||
		p.lookAhead(token.Nocopy, kw) ||
		p.lookAhead(token.Pub, token.Nocopy, kw) ||
		p.lookAhead(token.Sync, kw) ||
		p.lookAhead(token.Pub, token.Sync, kw) ||
		p.lookAhead(token.Unsync, kw) ||
		p.lookAhead(token.Pub, token.Unsync, kw) ||
		p.lookAhead(token.Nocopy, token.Sync, kw) ||
		p.lookAhead(token.Pub, token.Nocopy, token.Sync, kw) ||
		p.lookAhead(token.Nocopy, token.Unsync, kw) ||
		p.lookAhead(token.Pub, token.Nocopy, token.Unsync, kw) ||
		p.lookAhead(token.Unsafe, token.Sync, kw) ||
		p.lookAhead(token.Pub, token.Unsafe, token.Sync, kw) ||
		p.lookAhead(token.Nocopy, token.Unsafe, token.Sync, kw) ||
		p.lookAhead(token.Pub, token.Nocopy, token.Unsafe, token.Sync, kw)
}

func (p *Parser) parseSyncMode() SyncMode {
	if p.lookAheadConsume(token.Sync) {
		return SyncSync
	}
	if p.lookAheadConsume(token.Unsync) {
		return SyncUnsync
	}
	return SyncNone
}

func (p *Parser) parseTry() (NodeID, bool) {
	t, ok := p.expect(token.Try)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	expr, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	var pattern NodeID
	if next, ok := p.mayPeek(); ok && next.Kind == token.Is {
		p.next()
		pattern, ok = p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		if p.lookAhead(token.Dot, token.Ident) {
			p.diagnostic(p.span(), "`try ... is` expects a whole enum subset, not a qualified variant")
			return ParseFailed, false
		}
	} else {
		pattern = p.NewTryPattern(span)
	}
	successBinding := Name{"__try", span}
	successIdent := p.NewIdent("__try", nil, span)
	successBody := p.NewBlock([]NodeID{successIdent}, span)
	arm := MatchArm{
		Patterns: []NodeID{pattern}, Binding: &successBinding, Ref: false, Mut: false, Guard: nil, Body: successBody,
	}
	var else_ *MatchElse
	if next, ok := p.mayPeek(); ok && next.Kind == token.Else && !p.isMatchElse() {
		p.next()
		var binding *Name
		if next, ok := p.mayPeek(); ok && next.Kind == token.Ident {
			p.next()
			b := Name{next.Value, next.Span}
			binding = &b
		}
		body, ok := p.ParseBlock()
		if !ok {
			return ParseFailed, false
		}
		else_ = &MatchElse{Binding: binding, Ref: false, Mut: false, Body: body}
	} else {
		elseBinding := Name{"__try_e", span}
		elseIdent := p.NewIdent("__try_e", nil, span)
		elseReturn := p.NewReturn(elseIdent, span)
		elseBody := p.NewBlock([]NodeID{elseReturn}, span)
		else_ = &MatchElse{Binding: &elseBinding, Ref: false, Mut: false, Body: elseBody}
	}
	matchID := p.NewMatch(expr, []MatchArm{arm}, else_, span.Combine(p.span()))
	p.Node(matchID).Kind = Match{
		Expr: expr, Arms: []MatchArm{arm}, Else: else_, Try: true,
	}
	return matchID, true
}

func (p *Parser) parseMatchArms() ([]MatchArm, *MatchElse, bool) {
	var arms []MatchArm
	for {
		t, ok := p.mayPeek()
		if !ok || t.Kind == token.RCurly {
			return arms, nil, true
		}
		if t.Kind == token.Else {
			p.next()
			else_, ok := p.parseMatchElse()
			if !ok {
				return nil, nil, false
			}
			return arms, else_, true
		}
		if t.Kind != token.Case {
			p.diagnostic(t.Span, "expected case or else, got %s", t.Kind)
			return nil, nil, false
		}
		p.next()
		pattern, ok := p.parseMatchPattern()
		if !ok {
			return nil, nil, false
		}
		patterns := []NodeID{pattern}
		// `case A or B:` groups variants into one arm.
		for next, ok := p.mayPeek(); ok && next.Kind == token.Or; next, ok = p.mayPeek() {
			p.next()
			alt, ok := p.parseMatchPattern()
			if !ok {
				return nil, nil, false
			}
			patterns = append(patterns, alt)
		}
		binding, ref, mut, guard, body, ok := p.parseMatchArmBindingAndBody()
		if !ok {
			return nil, nil, false
		}
		if len(patterns) > 1 && guard != nil {
			p.diagnostic(p.Node(*guard).Span, "a guard cannot be combined with an or-pattern")
			return nil, nil, false
		}
		arms = append(arms, MatchArm{
			Patterns: patterns, Binding: binding, Ref: ref, Mut: mut, Guard: guard, Body: body,
		})
	}
}

func (p *Parser) parseEnumVariant() (NodeID, bool) {
	nameTok, ok := p.expect(token.Ident)
	if !ok {
		return ParseFailed, false
	}
	span := nameTok.Span
	name := Name{nameTok.Value, nameTok.Span}
	var args []NodeID
	var argNames []*Name
	if next, ok := p.mayPeek(); ok && next.Kind == token.LParen {
		args, argNames, ok = p.ParseCallArgs()
		if !ok {
			return ParseFailed, false
		}
	}
	var tag *NodeID
	if next, ok := p.mayPeek(); ok && next.Kind == token.Eq {
		p.next()
		// A bare int literal, not an expression: `|` would otherwise be parsed as bit-or.
		num, ok := p.ParseNumber()
		if !ok {
			return ParseFailed, false
		}
		tag = &num
	}
	return p.NewEnumVariant(name, args, argNames, tag, span.Combine(p.span())), true
}

// parseMatchPattern parses a match arm pattern: a type (union variant or whole
// enum subset) optionally followed by `.variant` for a qualified enum variant
// (`Color.red`, `ord.Color.red`).
func (p *Parser) parseMatchPattern() (NodeID, bool) {
	pattern, ok := p.ParseType()
	if !ok {
		return ParseFailed, false
	}
	if p.lookAhead(token.Dot, token.Ident) {
		_, _ = p.next()
		variant, _ := p.next()
		return p.NewFieldAccess(
			pattern, Name{variant.Value, variant.Span}, nil, p.Node(pattern).Span.Combine(variant.Span),
		), true
	}
	return pattern, true
}

func (p *Parser) parseMatchElse() (*MatchElse, bool) {
	binding, ref, mut, guard, body, ok := p.parseMatchArmBindingAndBody()
	if !ok {
		return nil, false
	}
	if guard != nil {
		p.diagnostic(p.Node(*guard).Span, "else arm cannot have a guard condition")
		return nil, false
	}
	return &MatchElse{Binding: binding, Ref: ref, Mut: mut, Body: body}, true
}

func (p *Parser) parseMatchArmBindingAndBody() (*Name, bool, bool, *NodeID, NodeID, bool) {
	var ref, mut bool
	if next, ok := p.mayPeek(); ok && (next.Kind == token.Amp || next.Kind == token.AmpAfterNewline) {
		p.next()
		ref = true
		if m, ok := p.mayPeek(); ok && m.Kind == token.Mut {
			p.next()
			mut = true
		}
	}
	var binding *Name
	if next, ok := p.mayPeek(); ok && next.Kind == token.Ident {
		p.next()
		b := Name{next.Value, next.Span}
		binding = &b
	} else if ref {
		p.diagnostic(p.span(), "expected a binding name after `&` in match arm")
		return nil, false, false, nil, ParseFailed, false
	}
	var guard *NodeID
	if next, ok := p.mayPeek(); ok && next.Kind == token.If {
		p.next()
		guardExpr, ok := p.ParseExpr(0)
		if !ok {
			return nil, false, false, nil, ParseFailed, false
		}
		guard = &guardExpr
	}
	if _, ok := p.expect(token.Colon); !ok {
		return nil, false, false, nil, ParseFailed, false
	}
	bodyExprs, ok := p.parseCaseBody()
	if !ok {
		return nil, false, false, nil, ParseFailed, false
	}
	bodySpan := p.span()
	if len(bodyExprs) > 0 {
		bodySpan = p.Node(bodyExprs[0]).Span.Combine(p.span())
	}
	body := p.NewBlock(bodyExprs, bodySpan)
	return binding, ref, mut, guard, body, true
}

func (p *Parser) parseCaseBody() ([]NodeID, bool) {
	var exprs []NodeID
	for {
		t, ok := p.mayPeek()
		if !ok || t.Kind == token.RCurly || t.Kind == token.Case || t.Kind == token.Else {
			return exprs, true
		}
		expr, ok := p.parseStmtExpr()
		if !ok {
			return nil, false
		}
		exprs = append(exprs, expr)
	}
}

// parseStmtExpr parses an expression in statement position: a block element or a
// match/when arm body. break, continue, and return are control flow that may
// only appear here, never nested inside a larger expression. ParsePrimaryExpr
// rejects them, so `return break`, `x + break`, `f(continue)`, etc. are parse
// errors rather than producing ill-formed code.
func (p *Parser) parseStmtExpr() (NodeID, bool) {
	t, ok := p.mayPeek()
	if !ok {
		return p.ParseExpr(0)
	}
	switch t.Kind { //nolint:exhaustive
	case token.Break:
		p.next()
		return p.NewBreak(t.Span), true
	case token.Continue:
		p.next()
		return p.NewContinue(t.Span), true
	case token.Return:
		return p.ParseReturn()
	default:
		return p.ParseExpr(0)
	}
}

func (p *Parser) isStructTarget(nodeID NodeID) bool {
	switch kind := p.Node(nodeID).Kind.(type) {
	case FieldAccess:
		name := kind.Field.Name
		return len(name) > 0 && unicode.IsUpper(rune(name[0]))
	default:
		return false
	}
}

// parseArrayConstruction parses the tail of `[N of v]` / `[N uninit T]` after the
// count expression. `of`/`uninit` are contextual keywords lexed as identifiers.
func (p *Parser) parseArrayConstruction(count NodeID, span base.Span) (NodeID, bool) {
	kw, ok := p.next()
	if !ok {
		return ParseFailed, false
	}
	if kw.Kind != token.Ident || (kw.Value != "of" && kw.Value != "uninit") {
		p.diagnostic(kw.Span, "unexpected token: expected of or uninit, got %s", kw.Kind)
		return ParseFailed, false
	}
	intKind, ok := p.Node(count).Kind.(Int)
	if !ok || !intKind.Value.IsInt64() || intKind.Value.Int64() <= 0 {
		p.diagnostic(p.Node(count).Span, "array length must be a positive integer literal")
		return ParseFailed, false
	}
	length := intKind.Value.Int64()
	if kw.Value == "of" {
		value, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		if _, ok := p.expect(token.RBracket); !ok {
			return ParseFailed, false
		}
		return p.NewArrayFill(length, value, span.Combine(p.span())), true
	}
	elem, ok := p.ParseType()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.RBracket); !ok {
		return ParseFailed, false
	}
	return p.NewArrayUninit(length, elem, span.Combine(p.span())), true
}

func (p *Parser) parseTypeParams() ([]NodeID, bool) {
	if t, ok := p.mayPeek(); !ok || t.Kind != token.LtImmediate {
		return nil, true
	}
	open, _ := p.next()
	params := []NodeID{}
	hasDefault := false
	for {
		t, ok := p.mustPeek()
		if !ok {
			return nil, false
		}
		if t.Kind == token.Gt {
			if len(params) == 0 {
				p.diagnostic(open.Span.Combine(t.Span), "empty type parameter list")
				return nil, false
			}
			p.next()
			return params, true
		}
		if len(params) > 0 {
			if _, ok := p.expect(token.Comma); !ok {
				return nil, false
			}
		}
		isSync := p.parseSyncMode()
		t, ok = p.expect(token.TypeIdent)
		if !ok {
			return nil, false
		}
		var constraint *NodeID
		if next, ok := p.mayPeek(); ok && (next.Kind == token.TypeIdent || p.lookAhead(token.Ident, token.Dot)) {
			c, ok := p.ParseType()
			if !ok {
				return nil, false
			}
			constraint = &c
		}
		var defaultType *NodeID
		if next, ok := p.mayPeek(); ok && next.Kind == token.Eq {
			p.next()
			d, ok := p.ParseType()
			if !ok {
				return nil, false
			}
			defaultType = &d
			hasDefault = true
		} else if hasDefault {
			p.diagnostic(t.Span, "type parameters with defaults must be last")
			return nil, false
		}
		params = append(params, p.NewTypeParam(Name{t.Value, t.Span}, constraint, defaultType, isSync, t.Span))
	}
}

func (p *Parser) parseTypeArgs() ([]NodeID, bool) {
	if t, ok := p.mayPeek(); !ok || t.Kind != token.LtImmediate {
		return nil, true
	}
	open, _ := p.next()
	args := []NodeID{}
	for {
		t, ok := p.mustPeek()
		if !ok {
			return nil, false
		}
		if t.Kind == token.Gt || t.Kind == token.GtGt {
			if len(args) == 0 {
				p.diagnostic(open.Span.Combine(t.Span), "empty type argument list")
				return nil, false
			}
			if t.Kind == token.GtGt {
				p.tokens[p.pos].Kind = token.Gt
			} else {
				p.next()
			}
			return args, true
		}
		if len(args) > 0 {
			if _, ok := p.expect(token.Comma); !ok {
				return nil, false
			}
		}
		typ, ok := p.ParseType()
		if !ok {
			return nil, false
		}
		args = append(args, typ)
	}
}

func (p *Parser) parseFunDecl() (FunDecl, base.Span, bool) {
	t, ok := p.expect(token.Fun)
	if !ok {
		return FunDecl{}, base.Span{}, false
	}
	var name Name
	peek, ok := p.mustPeek()
	if !ok {
		return FunDecl{}, base.Span{}, false
	}
	if peek.Kind == token.TypeIdent {
		ns, ok := p.expect(token.TypeIdent)
		if !ok {
			return FunDecl{}, base.Span{}, false
		}
		if _, ok := p.expect(token.Dot); !ok {
			return FunDecl{}, base.Span{}, false
		}
		method, ok := p.expectIdentOrKeyword()
		if !ok {
			return FunDecl{}, base.Span{}, false
		}
		name = Name{ns.Value + "." + method.Value, peek.Span.Combine(method.Span)}
	} else {
		ident, ok := p.expect(token.Ident)
		if !ok {
			return FunDecl{}, base.Span{}, false
		}
		name = Name{ident.Value, ident.Span}
	}
	typeParams, ok := p.parseTypeParams()
	if !ok {
		return FunDecl{}, base.Span{}, false
	}
	params, ok := p.ParseFunParams()
	if !ok {
		return FunDecl{}, base.Span{}, false
	}
	noescapeReturn := p.lookAheadConsume(token.Noescape)
	returnType, ok := p.ParseType()
	if !ok {
		return FunDecl{}, base.Span{}, false
	}
	return FunDecl{
		Name: name, ExternName: "", TypeParams: typeParams, Params: params, ReturnType: returnType,
		Pub: false, Builtin: false, Extern: false, Unsafe: false, Sync: SyncNone,
		NoescapeReturn: noescapeReturn,
	}, t.Span, true
}

func (p *Parser) parseFunLiteral(sync SyncMode) (NodeID, bool) { //nolint:funlen
	t, ok := p.expect(token.Fun)
	if !ok {
		return ParseFailed, false
	}
	// Parse optional capture list: fun[a, &b, &mut c](params) ...
	var captures []NodeID
	if peek, peekOK := p.mayPeek(); peekOK && (peek.Kind == token.LBracket || peek.Kind == token.LBracketImmediate) {
		p.next()
		for {
			if peek, peekOK := p.mayPeek(); peekOK && peek.Kind == token.RBracket {
				p.next()
				break
			}
			if len(captures) > 0 {
				if _, ok := p.expect(token.Comma); !ok {
					return ParseFailed, false
				}
			}
			capSpan := p.span()
			mode := CaptureByValue
			if peek, peekOK := p.mayPeek(); peekOK && (peek.Kind == token.Amp || peek.Kind == token.AmpAfterNewline) {
				p.next()
				capSpan = peek.Span
				if peek2, peekOK2 := p.mayPeek(); peekOK2 && peek2.Kind == token.Mut {
					p.next()
					mode = CaptureByMutRef
				} else {
					mode = CaptureByRef
				}
			}
			var name Name
			if peek, peekOK := p.mayPeek(); peekOK && peek.Kind == token.AllocatorIdent {
				p.next()
				name = Name{peek.Value, peek.Span}
			} else {
				ident, ok := p.expect(token.Ident)
				if !ok {
					return ParseFailed, false
				}
				name = Name{ident.Value, ident.Span}
			}
			captures = append(captures, p.NewCapture(name, mode, capSpan.Combine(name.Span)))
		}
	}
	params, ok := p.parseFunLitParams()
	if !ok {
		return ParseFailed, false
	}
	// Return type is optional for function literals. If the next token starts
	// a block, the return type will be inferred from context.
	returnType := InferredType
	noescapeReturn := false
	if peek, peekOK := p.mayPeek(); peekOK && peek.Kind != token.LCurly {
		noescapeReturn = p.lookAheadConsume(token.Noescape)
		parsed, ok := p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		returnType = parsed
	}
	body, ok := p.ParseBlock()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span.Combine(p.span())
	name := fmt.Sprintf("__fun_lit_%d", p.nextFunLitID)
	p.nextFunLitID++
	funID := p.NewFun(Name{name, span}, nil, params, returnType, body, false, false, noescapeReturn, sync, span)
	if len(captures) > 0 {
		node := p.Node(funID)
		fun, _ := node.Kind.(Fun)
		fun.Captures = captures
		node.Kind = fun
	}
	ident := p.NewIdent(name, nil, span)
	return p.NewBlock([]NodeID{funID, ident}, span), true
}

func (p *Parser) parseCompIf(parseBody func() ([]NodeID, bool)) (NodeID, bool) {
	t, ok := p.expect(token.HashIf)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	cond, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	body, ok := parseBody()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.HashEnd); !ok {
		return ParseFailed, false
	}
	return p.NewCompIf(cond, body, span.Combine(p.span())), true
}

func (p *Parser) diagnostic(span base.Span, msg string, msgArgs ...any) {
	p.Diagnostics = append(p.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}

func (p *Parser) next() (*token.Token, bool) {
	if p.pos >= len(p.tokens) || p.tokens[p.pos].Kind == token.EOF {
		p.diagnostic(p.span(), "unexpected end of file")
		return nil, false
	}
	token := &p.tokens[p.pos]
	p.pos++
	return token, true
}

func (p *Parser) mayPeek() (*token.Token, bool) {
	if p.pos >= len(p.tokens) || p.tokens[p.pos].Kind == token.EOF {
		return nil, false
	}
	return &p.tokens[p.pos], true
}

func (p *Parser) isMatchElse() bool {
	after, ok := p.mayPeek1()
	if !ok {
		return false
	}
	if after.Kind == token.Colon {
		return true
	}
	if after.Kind == token.Ident && p.pos+2 < len(p.tokens) {
		return p.tokens[p.pos+2].Kind == token.Colon
	}
	return false
}

// mayPeek1 peeks at the token after the current one (2-token lookahead).
// The grammar is LL(1)-equivalent (can be left-factored), but a bounded
// 2-token lookahead is sometimes more practical.
func (p *Parser) mayPeek1() (*token.Token, bool) {
	if p.pos+1 >= len(p.tokens) || p.tokens[p.pos+1].Kind == token.EOF {
		return nil, false
	}
	return &p.tokens[p.pos+1], true
}

// lookAhead checks whether the next tokens match the given kinds.
func (p *Parser) lookAhead(kinds ...token.TokenKind) bool {
	for i, kind := range kinds {
		pos := p.pos + i
		if pos >= len(p.tokens) || p.tokens[pos].Kind == token.EOF {
			return false
		}
		if p.tokens[pos].Kind != kind {
			return false
		}
	}
	return true
}

// lookAheadConsume checks whether the next tokens match the given kinds
// and consumes them if they do.
func (p *Parser) lookAheadConsume(kinds ...token.TokenKind) bool {
	if !p.lookAhead(kinds...) {
		return false
	}
	p.pos += len(kinds)
	return true
}

// Same as `peek()` but adds a diagnostic if there are no more tokens.
func (p *Parser) mustPeek() (*token.Token, bool) {
	t, ok := p.mayPeek()
	if !ok {
		p.diagnostic(p.span(), "unexpected end of file")
		return nil, false
	}
	return t, ok
}

func (p *Parser) expect(kind token.TokenKind) (*token.Token, bool) {
	t, ok := p.next()
	if !ok {
		// `next()` already emitted the EOF diagnostic; don't duplicate it.
		return nil, false
	}
	if t.Kind != kind {
		p.diagnostic(p.span(), "unexpected token: expected %s, got %s", kind, t.Kind)
		return nil, false
	}
	return t, true
}

// expectIdentOrKeyword consumes the next token if it is an identifier or a keyword,
// returning it as an Ident token with the keyword's name as its value.
func (p *Parser) expectIdentOrKeyword() (*token.Token, bool) {
	t, ok := p.next()
	if !ok {
		return nil, false
	}
	if t.Kind == token.Ident {
		return t, true
	}
	if name, ok := token.KeywordNames[t.Kind]; ok {
		t.Value = name
		return t, true
	}
	p.diagnostic(p.span(), "unexpected token: expected %s, got %s", token.Ident, t.Kind)
	return nil, false
}

func (p *Parser) expectNumber() (*big.Int, bool) {
	t, ok := p.expect(token.Number)
	if !ok {
		return nil, false
	}
	s := t.Value
	intBase := 10
	switch {
	case strings.HasPrefix(s, "0x"):
		s = s[2:]
		intBase = 16
	case strings.HasPrefix(s, "0o"):
		s = s[2:]
		intBase = 8
	case strings.HasPrefix(s, "0b"):
		s = s[2:]
		intBase = 2
	}
	s = strings.ReplaceAll(s, "_", "")
	n, valid := new(big.Int).SetString(s, intBase)
	if !valid {
		p.diagnostic(t.Span, "invalid number: %s", t.Value)
		return nil, false
	}
	return n, true
}

func (p *Parser) span() base.Span {
	token := p.tokens[min(max(p.pos-1, 0), len(p.tokens)-1)]
	return token.Span
}
