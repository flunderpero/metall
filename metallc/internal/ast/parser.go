package ast

import (
	"fmt"
	"math/big"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

var ReservedWords = []string{"Arena", "panic"} //nolint:gochecknoglobals

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
	var imports []NodeID
	for {
		t, ok := p.mayPeek()
		if !ok || t.Kind != token.Use {
			break
		}
		imp, ok := p.ParseImport()
		if !ok {
			return ParseFailed, false
		}
		imports = append(imports, imp)
	}
	decls, ok := p.ParseDecls()
	if !ok {
		return ParseFailed, false
	}
	return p.NewModule(source.FileName, source.Module, source.Main, imports, decls, span.Combine(p.span())), true
}

func (p *Parser) ParseDecls() ([]NodeID, bool) {
	decls := make([]NodeID, 0)
	result := true
	for {
		t, ok := p.mayPeek()
		if !ok {
			return decls, result
		}
		switch t.Kind { //nolint:exhaustive
		case token.Fun:
			if fun, ok := p.ParseFun(); ok {
				decls = append(decls, fun)
			}
		case token.Struct:
			if struct_, ok := p.ParseStruct(); ok {
				decls = append(decls, struct_)
			}
		case token.Shape:
			if shape, ok := p.ParseShape(); ok {
				decls = append(decls, shape)
			}
		case token.Union:
			if union, ok := p.ParseUnion(); ok {
				decls = append(decls, union)
			}
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return decls, false
		}
	}
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
		if !ok || t.Kind != token.ColonColon {
			break
		}
		p.next()
	}
	return p.NewImport(alias, segments, span.Combine(p.span())), true
}

func (p *Parser) ParseFunType() (NodeID, bool) {
	t, ok := p.expect(token.Fun)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	if _, ok := p.expect(token.LParen); !ok {
		return ParseFailed, false
	}
	params := []NodeID{}
	for {
		t, ok := p.mustPeek()
		if !ok {
			return ParseFailed, false
		}
		if t.Kind == token.RParen {
			break
		}
		if len(params) > 0 {
			_, ok := p.expect(token.Comma)
			if !ok {
				return ParseFailed, false
			}
		}
		param, ok := p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		params = append(params, param)
	}
	if _, ok := p.expect(token.RParen); !ok {
		return ParseFailed, false
	}
	returnTyp, ok := p.parseFunReturnType()
	if !ok {
		return ParseFailed, false
	}
	return p.NewFunType(params, returnTyp, span.Combine(p.span())), ok
}

func (p *Parser) ParseFunDecl() (NodeID, bool) {
	decl, startSpan, ok := p.parseFunDecl()
	if !ok {
		return ParseFailed, false
	}
	return p.NewFunDecl(decl.Name, decl.TypeParams, decl.Params, decl.ReturnType,
		startSpan.Combine(p.span())), true
}

func (p *Parser) ParseFun() (NodeID, bool) {
	decl, startSpan, ok := p.parseFunDecl()
	if !ok {
		return ParseFailed, false
	}
	block, ok := p.ParseBlock()
	if !ok {
		return ParseFailed, false
	}
	return p.NewFun(decl.Name, decl.TypeParams, decl.Params, decl.ReturnType, block,
		startSpan.Combine(p.span())), true
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
		mut := false
		switch t.Kind { //nolint:exhaustive
		case token.Ident, token.AllocatorIdent:
			name = Name{t.Value, t.Span}
			p.next()
		case token.Mut:
			mut = true
			p.next()
			nt, ok := p.expect(token.Ident)
			if !ok {
				return nil, false
			}
			name = Name{nt.Value, nt.Span}
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return nil, false
		}
		type_, ok := p.ParseType()
		if !ok {
			return nil, false
		}
		fields = append(fields, p.NewStructField(name, type_, mut, span.Combine(p.span())))
	}
}

func (p *Parser) ParseStruct() (NodeID, bool) {
	t, ok := p.expect(token.Struct)
	if !ok {
		return ParseFailed, false
	}
	nameToken, ok := p.expect(token.TypeIdent)
	if !ok {
		return ParseFailed, false
	}
	if slices.Contains(ReservedWords, nameToken.Value) {
		p.diagnostic(nameToken.Span, "reserved word: %s", nameToken.Value)
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
	return p.NewStruct(name, typeParams, fields, t.Span.Combine(p.span())), true
}

func (p *Parser) ParseShape() (NodeID, bool) {
	t, ok := p.expect(token.Shape)
	if !ok {
		return ParseFailed, false
	}
	nameToken, ok := p.expect(token.TypeIdent)
	if !ok {
		return ParseFailed, false
	}
	name := Name{nameToken.Value, nameToken.Span}
	if _, ok := p.expect(token.LCurly); !ok {
		return ParseFailed, false
	}
	fields, ok := p.ParseStructFields(token.RCurly, token.Fun)
	if !ok {
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
		funDecl, ok := p.ParseFunDecl()
		if !ok {
			return ParseFailed, false
		}
		funs = append(funs, funDecl)
	}
	return p.NewShape(name, fields, funs, t.Span.Combine(p.span())), true
}

func (p *Parser) ParseUnion() (NodeID, bool) {
	t, ok := p.expect(token.Union)
	if !ok {
		return ParseFailed, false
	}
	nameToken, ok := p.expect(token.TypeIdent)
	if !ok {
		return ParseFailed, false
	}
	if slices.Contains(ReservedWords, nameToken.Value) {
		p.diagnostic(nameToken.Span, "reserved word: %s", nameToken.Value)
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
	return p.NewUnion(name, typeParams, variants, t.Span.Combine(p.span())), true
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
	args, ok := p.ParseCallArgs()
	if !ok {
		return ParseFailed, false
	}
	return p.NewTypeConstruction(ident, args, struct_.Span.Combine(p.span())), true
}

func (p *Parser) ParseArrayLiteral() (NodeID, bool) {
	t, ok := p.expect(token.LBracket)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	elems := []NodeID{}
	for {
		t, ok := p.mustPeek()
		if !ok {
			return ParseFailed, false
		}
		if t.Kind == token.RBracket {
			p.next()
			break
		}
		expr, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		elems = append(elems, expr)
		t, ok = p.next()
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
	}
	return p.NewArrayLiteral(elems, span.Combine(p.span())), true
}

func (p *Parser) ParseBlock() (NodeID, bool) {
	t, ok := p.expect(token.LCurly)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	exprs := []NodeID{}
	for {
		t, ok := p.mustPeek()
		if !ok {
			break
		}
		if t.Kind == token.RCurly {
			p.next()
			break
		}
		expr, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		exprs = append(exprs, expr)
	}
	return p.NewBlock(exprs, span.Combine(p.span())), true
}

func (p *Parser) ParseExpr(minPrecedence int) (NodeID, bool) { //nolint:funlen
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	if t.Kind == token.Not || t.Kind == token.Tilde {
		p.next()
		expr, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		op := UnaryOpNot
		if t.Kind == token.Tilde {
			op = UnaryOpBitNot
		}
		return p.NewUnary(op, expr, t.Span.Combine(p.span())), true
	}
	span := t.Span
	lhs, ok := p.ParsePostfixExpr(0)
	if !ok {
		return ParseFailed, false
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
	for {
		t, ok = p.mayPeek()
		if !ok {
			return lhs, true
		}
		op, ok := map[token.TokenKind]BinaryOp{
			token.Plus:    BinaryOpAdd,
			token.Minus:   BinaryOpSub,
			token.Star:    BinaryOpMul,
			token.Slash:   BinaryOpDiv,
			token.Percent: BinaryOpMod,

			token.EqEq: BinaryOpEq,
			token.Neq:  BinaryOpNeq,
			token.Lt:   BinaryOpLt,
			token.Lte:  BinaryOpLte,
			token.Gt:   BinaryOpGt,
			token.Gte:  BinaryOpGte,
			token.And:  BinaryOpAnd,
			token.Or:   BinaryOpOr,

			token.AmpInfix: BinaryOpBitAnd,
			token.Pipe:     BinaryOpBitOr,
			token.Caret:    BinaryOpBitXor,
			token.LtLt:     BinaryOpShl,
			token.GtGt:     BinaryOpShr,
		}[t.Kind]
		if !ok {
			return lhs, true
		}
		precedence := map[BinaryOp]int{
			BinaryOpOr:     0,
			BinaryOpAnd:    1,
			BinaryOpBitOr:  2,
			BinaryOpBitXor: 3,
			BinaryOpBitAnd: 4,
			BinaryOpEq:     5,
			BinaryOpNeq:    5,
			BinaryOpLt:     5,
			BinaryOpLte:    5,
			BinaryOpGt:     5,
			BinaryOpGte:    5,
			BinaryOpShl:    6,
			BinaryOpShr:    6,
			BinaryOpAdd:    7,
			BinaryOpSub:    7,
			BinaryOpMul:    8,
			BinaryOpDiv:    8,
			BinaryOpMod:    8,
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
			args, ok := p.ParseCallArgs()
			if !ok {
				return ParseFailed, false
			}
			if p.isStructTarget(callee) {
				expr = p.NewTypeConstruction(callee, args, span.Combine(p.span()))
			} else {
				expr = p.NewCall(callee, args, span.Combine(p.span()))
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
			if next.Kind != token.Ident {
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
	case token.Amp:
		ref, ok := p.ParseRefExpr()
		if !ok {
			return ParseFailed, false
		}
		expr = ref
	case token.Fun:
		var fun NodeID
		var ok bool
		if next, peekOK := p.mayPeek1(); peekOK && next.Kind == token.LParen {
			fun, ok = p.parseFunLiteral()
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
	case token.Match:
		match, ok := p.ParseMatch()
		if !ok {
			return ParseFailed, false
		}
		expr = match
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
		p.next()
		expr = p.NewBreak(t.Span)
	case token.Continue:
		p.next()
		expr = p.NewContinue(t.Span)
	case token.Void:
		p.next()
		expr = p.NewIdent("void", nil, t.Span)
	case token.Return:
		return_, ok := p.ParseReturn()
		if !ok {
			return ParseFailed, false
		}
		expr = return_
	case token.Ident:
		if next, ok := p.mayPeek1(); ok && next.Kind == token.ColonColon {
			path, ok := p.ParsePath()
			if !ok {
				return ParseFailed, false
			}
			expr = path
		} else {
			ident, ok := p.ParseIdent()
			if !ok {
				return ParseFailed, false
			}
			expr = ident
		}
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
			array, ok := p.ParseArrayLiteral()
			if !ok {
				return ParseFailed, false
			}
			expr = array
		}
	case token.Number:
		num, ok := p.ParseNumber()
		if !ok {
			return ParseFailed, false
		}
		expr = num
	case token.True:
		p.next()
		expr = p.NewBool(true, t.Span)
	case token.False:
		p.next()
		expr = p.NewBool(false, t.Span)
	case token.String:
		p.next()
		expr = p.NewString(t.Value, t.Span.Combine(p.span()))
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
	default:
		p.diagnostic(t.Span, "unexpected token: expected start of an expression, got %s", t.Kind)
		return ParseFailed, false
	}
	return expr, true
}

func (p *Parser) ParseRefExpr() (NodeID, bool) {
	t, ok := p.expect(token.Amp)
	if !ok {
		return ParseFailed, false
	}
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
	switch p.AST.Node(target).Kind.(type) {
	case Ident, FieldAccess, Index, Deref:
	default:
		p.diagnostic(p.AST.Node(target).Span, "expected a place expression (variable, field, index, or deref)")
		return ParseFailed, false
	}
	return p.NewRef(target, mut, span.Combine(p.span())), true
}

func (p *Parser) ParseCallArgs() ([]NodeID, bool) {
	if _, ok := p.expect(token.LParen); !ok {
		return nil, false
	}
	args := []NodeID{}
	for {
		t, ok := p.mayPeek()
		if !ok {
			return args, true
		}
		if t.Kind == token.RParen {
			p.next()
			return args, true
		}
		var expr NodeID
		if t.Kind == token.AllocatorIdent {
			p.next()
			expr = p.NewIdent(t.Value, nil, t.Span)
		} else {
			expr, ok = p.ParseExpr(0)
			if !ok {
				return args, false
			}
		}
		args = append(args, expr)
		t, ok = p.next()
		if !ok {
			return args, true
		}
		switch t.Kind { //nolint:exhaustive
		case token.Comma:
		case token.RParen:
			return args, true
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return args, false
		}
	}
}

func (p *Parser) ParseAllocatorVar(span base.Span) (NodeID, bool) {
	name, ok := p.expect(token.AllocatorIdent)
	if !ok {
		return ParseFailed, false
	}
	_, ok = p.expect(token.Eq)
	if !ok {
		return ParseFailed, false
	}
	allocator, ok := p.expect(token.TypeIdent)
	if !ok {
		return ParseFailed, false
	}
	args, ok := p.ParseCallArgs()
	if !ok {
		return ParseFailed, false
	}
	return p.NewAllocatorVar(
		Name{Name: name.Value, Span: name.Span},
		Name{Name: allocator.Value, Span: allocator.Span},
		args,
		span.Combine(p.span()),
	), true
}

func (p *Parser) ParseVar() (NodeID, bool) {
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
	if slices.Contains(ReservedWords, nameToken.Value) {
		p.diagnostic(nameToken.Span, "reserved word: %s", nameToken.Value)
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
	return p.NewVar(name, type_, init, mut, span.Combine(p.span())), true
}

func (p *Parser) ParseFunParams() ([]NodeID, bool) {
	if _, ok := p.expect(token.LParen); !ok {
		return nil, false
	}
	funParams := []NodeID{}
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
			type_, ok := p.ParseType()
			if !ok {
				p.diagnostic(t.Span, "expected type, got %s", t.Kind)
				return funParams, false
			}
			param := p.NewFunParam(name, type_, name.Span.Combine(p.span()))
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

func (p *Parser) ParseType() (NodeID, bool) {
	return p.parseBaseType()
}

func (p *Parser) ParseFor() (NodeID, bool) {
	t, ok := p.expect(token.For)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	t, ok = p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	// `for x in <range> { ... }`
	if t.Kind == token.Ident {
		if next, ok := p.mayPeek1(); ok && next.Kind == token.In {
			ident := t
			p.next()
			p.next()
			binding := Name{Name: ident.Value, Span: ident.Span}
			range_, isRange, ok := p.ParseRange()
			if !ok {
				return ParseFailed, false
			}
			if !isRange {
				p.diagnostic(p.span(), "expected range expression (e.g. 0..10)")
				return ParseFailed, false
			}
			body, ok := p.ParseBlock()
			if !ok {
				return ParseFailed, false
			}
			return p.NewFor(&binding, &range_, body, span.Combine(p.span())), true
		}
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
	return p.NewFor(nil, cond, body, span.Combine(p.span())), true
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
	if after, ok := p.mayPeek1(); ok && (after.Kind == token.Ident || after.Kind == token.Colon) {
		return p.NewIf(cond, then, nil, t.Span.Combine(p.span())), true
	}
	p.next()
	else_, ok := p.ParseBlock()
	if !ok {
		return ParseFailed, false
	}
	return p.NewIf(cond, then, &else_, t.Span.Combine(p.span())), true
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

func (p *Parser) ParseNumber() (NodeID, bool) {
	n, ok := p.expectNumber()
	if !ok {
		return ParseFailed, false
	}
	return p.NewInt(n, p.span()), true
}

func (p *Parser) ParsePath() (NodeID, bool) {
	segments := []string{}
	var span base.Span
	for {
		segment, ok := p.expect(token.Ident)
		if !ok {
			return ParseFailed, false
		}
		if len(segments) == 0 {
			span = segment.Span
		}
		segments = append(segments, segment.Value)
		t, ok := p.mayPeek()
		if !ok || t.Kind != token.ColonColon {
			break
		}
		p.next()
		t, ok = p.mustPeek()
		if !ok {
			return ParseFailed, false
		}
		if t.Kind == token.TypeIdent && len(segments) >= 1 {
			p.next()
			lastSegment := t.Value
			// If followed by `.ident`, this is a method reference like `lib::Foo.bar`.
			if dot, ok := p.mayPeek(); ok && dot.Kind == token.Dot {
				p.next()
				method, ok := p.expect(token.Ident)
				if !ok {
					return ParseFailed, false
				}
				lastSegment += "." + method.Value
			}
			segments = append(segments, lastSegment)
			break
		}
	}
	typeArgs, ok := p.parseTypeArgs()
	if !ok {
		return ParseFailed, false
	}
	return p.NewPath(segments, typeArgs, span.Combine(p.span())), true
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
	} else {
		pattern = p.NewTryPattern(span)
	}
	successBinding := Name{"__try", span}
	successIdent := p.NewIdent("__try", nil, span)
	successBody := p.NewBlock([]NodeID{successIdent}, span)
	arm := MatchArm{Pattern: pattern, Binding: &successBinding, Guard: nil, Body: successBody}
	var else_ *MatchElse
	if next, ok := p.mayPeek(); ok && next.Kind == token.Else {
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
		else_ = &MatchElse{Binding: binding, Body: body}
	} else {
		elseBinding := Name{"__try_e", span}
		elseIdent := p.NewIdent("__try_e", nil, span)
		elseReturn := p.NewReturn(elseIdent, span)
		elseBody := p.NewBlock([]NodeID{elseReturn}, span)
		else_ = &MatchElse{Binding: &elseBinding, Body: elseBody}
	}
	matchID := p.NewMatch(expr, []MatchArm{arm}, else_, span.Combine(p.span()))
	p.Node(matchID).Kind = Match{
		Expr: expr, Arms: []MatchArm{arm}, Else: else_, Try: true,
	}
	return matchID, true
}

func (p *Parser) parseBaseType() (NodeID, bool) { //nolint:funlen
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	switch t.Kind { //nolint:exhaustive
	case token.Ident:
		if next, ok := p.mayPeek1(); ok && next.Kind == token.ColonColon {
			path, ok := p.ParsePath()
			if !ok {
				return ParseFailed, false
			}
			pathNode := base.Cast[Path](p.AST.Node(path).Kind)
			last := pathNode.Segments[len(pathNode.Segments)-1]
			firstRune, _ := utf8.DecodeRuneInString(last)
			if firstRune == utf8.RuneError || !unicode.IsUpper(firstRune) {
				p.diagnostic(p.span(), "expected a type identifier, got %s", last)
				return ParseFailed, false
			}
			return path, true
		}
		p.diagnostic(span, "unexpected token: expected <type identifier> or &, got %s", t.Kind)
		return ParseFailed, false
	case token.TypeIdent:
		p.next()
		typeArgs, ok := p.parseTypeArgs()
		if !ok {
			return ParseFailed, false
		}
		return p.NewSimpleType(Name{t.Value, span}, typeArgs, span.Combine(p.span())), true
	case token.Void:
		p.next()
		return p.NewSimpleType(Name{"void", span}, nil, span), true
	case token.LBracket, token.LBracketImmediate:
		return p.ParseArrayOrSliceType()
	case token.Amp:
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
	case token.Fun:
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
		pattern, ok := p.ParseType()
		if !ok {
			return nil, nil, false
		}
		binding, guard, body, ok := p.parseMatchArmBindingAndBody()
		if !ok {
			return nil, nil, false
		}
		arms = append(arms, MatchArm{Pattern: pattern, Binding: binding, Guard: guard, Body: body})
	}
}

func (p *Parser) parseMatchElse() (*MatchElse, bool) {
	binding, guard, body, ok := p.parseMatchArmBindingAndBody()
	if !ok {
		return nil, false
	}
	if guard != nil {
		p.diagnostic(p.Node(*guard).Span, "else arm cannot have a guard condition")
		return nil, false
	}
	return &MatchElse{Binding: binding, Body: body}, true
}

func (p *Parser) parseMatchArmBindingAndBody() (*Name, *NodeID, NodeID, bool) {
	var binding *Name
	if next, ok := p.mayPeek(); ok && next.Kind == token.Ident {
		p.next()
		b := Name{next.Value, next.Span}
		binding = &b
	}
	var guard *NodeID
	if next, ok := p.mayPeek(); ok && next.Kind == token.If {
		p.next()
		guardExpr, ok := p.ParseExpr(0)
		if !ok {
			return nil, nil, ParseFailed, false
		}
		guard = &guardExpr
	}
	if _, ok := p.expect(token.Colon); !ok {
		return nil, nil, ParseFailed, false
	}
	bodyExprs, ok := p.parseMatchArmBody()
	if !ok {
		return nil, nil, ParseFailed, false
	}
	bodySpan := p.span()
	if len(bodyExprs) > 0 {
		bodySpan = p.Node(bodyExprs[0]).Span.Combine(p.span())
	}
	body := p.NewBlock(bodyExprs, bodySpan)
	return binding, guard, body, true
}

func (p *Parser) parseMatchArmBody() ([]NodeID, bool) {
	var exprs []NodeID
	for {
		t, ok := p.mayPeek()
		if !ok || t.Kind == token.RCurly || t.Kind == token.Case || t.Kind == token.Else {
			return exprs, true
		}
		expr, ok := p.ParseExpr(0)
		if !ok {
			return nil, false
		}
		exprs = append(exprs, expr)
	}
}

func (p *Parser) isStructTarget(nodeID NodeID) bool {
	path, ok := p.Node(nodeID).Kind.(Path)
	if !ok {
		return false
	}
	last := path.Segments[len(path.Segments)-1]
	if strings.Contains(last, ".") {
		return false
	}
	return len(last) > 0 && unicode.IsUpper(rune(last[0]))
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
		t, ok = p.expect(token.TypeIdent)
		if !ok {
			return nil, false
		}
		var constraint *NodeID
		if next, ok := p.mayPeek(); ok && next.Kind == token.TypeIdent {
			p.next()
			c := p.NewSimpleType(Name{next.Value, next.Span}, nil, next.Span)
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
		params = append(params, p.NewTypeParam(Name{t.Value, t.Span}, constraint, defaultType, t.Span))
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
		method, ok := p.expect(token.Ident)
		if !ok {
			return FunDecl{}, base.Span{}, false
		}
		name = Name{ns.Value + "." + method.Value, peek.Span.Combine(method.Span)}
	} else {
		ident, ok := p.expect(token.Ident)
		if !ok {
			return FunDecl{}, base.Span{}, false
		}
		if slices.Contains(ReservedWords, ident.Value) {
			p.diagnostic(ident.Span, "reserved word: %s", ident.Value)
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
	returnType, ok := p.parseFunReturnType()
	if !ok {
		return FunDecl{}, base.Span{}, false
	}
	return FunDecl{Name: name, TypeParams: typeParams, Params: params, ReturnType: returnType}, t.Span, true
}

func (p *Parser) parseFunLiteral() (NodeID, bool) {
	t, ok := p.expect(token.Fun)
	if !ok {
		return ParseFailed, false
	}
	params, ok := p.ParseFunParams()
	if !ok {
		return ParseFailed, false
	}
	returnType, ok := p.parseFunReturnType()
	if !ok {
		return ParseFailed, false
	}
	body, ok := p.ParseBlock()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span.Combine(p.span())
	name := fmt.Sprintf("__fun_lit_%d", p.nextFunLitID)
	p.nextFunLitID++
	funNode := p.NewFun(Name{name, span}, nil, params, returnType, body, span)
	ident := p.NewIdent(name, nil, span)
	return p.NewBlock([]NodeID{funNode, ident}, span), true
}

func (p *Parser) parseFunReturnType() (NodeID, bool) {
	return p.ParseType()
}

func (p *Parser) diagnostic(span base.Span, msg string, msgArgs ...any) {
	p.Diagnostics = append(p.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}

func (p *Parser) next() (*token.Token, bool) {
	if p.pos >= len(p.tokens) {
		p.diagnostic(p.span(), "unexpected end of file")
		return nil, false
	}
	token := &p.tokens[p.pos]
	p.pos++
	return token, true
}

func (p *Parser) mayPeek() (*token.Token, bool) {
	if p.pos >= len(p.tokens) {
		return nil, false
	}
	return &p.tokens[p.pos], true
}

// mayPeek1 peeks at the token after the current one (2-token lookahead).
// The grammar is LL(1)-equivalent (can be left-factored), but a bounded
// 2-token lookahead is sometimes more practical.
func (p *Parser) mayPeek1() (*token.Token, bool) {
	if p.pos+1 >= len(p.tokens) {
		return nil, false
	}
	return &p.tokens[p.pos+1], true
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
		p.diagnostic(p.span(), "unexpected end of file")
		return nil, false
	}
	if t.Kind != kind {
		p.diagnostic(p.span(), "unexpected token: expected %s, got %s", kind, t.Kind)
		return nil, false
	}
	return t, true
}

func (p *Parser) expectNumber() (*big.Int, bool) {
	t, ok := p.expect(token.Number)
	if !ok {
		return nil, false
	}
	n, valid := new(big.Int).SetString(t.Value, 10)
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
