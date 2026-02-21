package ast

import (
	"slices"
	"strconv"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

const ParseFailed = NodeID(0)

type Parser struct {
	*AST
	Diagnostics base.Diagnostics
	tokens      []token.Token
	pos         int
}

func NewParser(tokens []token.Token) *Parser {
	return &Parser{NewAST(), base.Diagnostics{}, tokens, 0}
}

func (p *Parser) ParseFile() (NodeID, bool) {
	span := p.span()
	decls, ok := p.ParseDecls()
	if !ok {
		return ParseFailed, false
	}
	return p.NewFile(decls, span.Combine(p.span())), ok
}

func (p *Parser) ParseDecls() ([]NodeID, bool) {
	decls := make([]NodeID, 0)
	result := true
	for {
		t, ok := p.peek()
		if !ok {
			return decls, result
		}
		switch t.Kind { //nolint:exhaustive
		case token.Fun:
			if fun, ok := p.ParseFun(); ok {
				decls = append(decls, fun)
			}
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return decls, false
		}
	}
}

func (p *Parser) ParseFun() (NodeID, bool) {
	t, ok := p.expect(token.Fun)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	nameToken, ok := p.expect(token.Ident)
	if !ok {
		return ParseFailed, false
	}
	name := Name{nameToken.Value, nameToken.Span}
	params, ok := p.ParseFunParams()
	if !ok {
		return ParseFailed, false
	}
	t, ok = p.peek()
	if !ok {
		return ParseFailed, false
	}
	var returnType NodeID
	if t.Kind == token.Void {
		returnType = p.NewSimpleType(Name{"void", t.Span}, t.Span)
		p.next()
	} else {
		returnType, ok = p.ParseType()
		if !ok {
			return ParseFailed, false
		}
	}
	block, ok := p.parseBlock(false) // Function creates its own scope for params and body.
	if !ok {
		return ParseFailed, false
	}
	return p.NewFun(name, params, returnType, block, span.Combine(p.span())), true
}

func (p *Parser) ParseBlock() (NodeID, bool) {
	return p.parseBlock(true)
}

func (p *Parser) ParseExpr() (NodeID, bool) {
	t, ok := p.peek()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	lhs, ok := p.ParseUnaryExpr(0)
	if !ok {
		return ParseFailed, false
	}
	switch t, ok := p.peek(); {
	case !ok:
		return lhs, true
	case t.Kind == token.Eq:
		p.next()
		rhs, ok := p.ParseExpr()
		if !ok {
			return ParseFailed, false
		}
		return p.NewAssign(lhs, rhs, span.Combine(p.span())), true
	default:
		return lhs, true
	}
}

func (p *Parser) ParseUnaryExpr(minPrecedence int) (NodeID, bool) {
	t, ok := p.peek()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	var expr NodeID
	switch t.Kind { //nolint:exhaustive
	case token.Star:
		p.next()
		expr, ok = p.ParseUnaryExpr(minPrecedence)
		if !ok {
			return ParseFailed, false
		}
		expr = p.NewDeref(expr, span.Combine(p.span()))
	default:
		expr, ok = p.ParsePrimaryExpr(minPrecedence)
		if !ok {
			return ParseFailed, false
		}
	}
	for {
		if t, ok := p.peek(); ok && t.Kind == token.LParen {
			callee := expr
			args, ok := p.ParseCallArgs()
			if !ok {
				return ParseFailed, false
			}
			expr = p.NewCall(callee, args, span.Combine(p.span()))
		} else {
			break
		}
	}
	return expr, true
}

func (p *Parser) ParsePrimaryExpr(minPrecedence int) (NodeID, bool) { //nolint:funlen
	t, ok := p.peek()
	if !ok {
		return ParseFailed, false
	}
	expectedTokenKinds := []token.TokenKind{
		token.Amp,
		token.LCurly,
		token.True,
		token.False,
		token.If,
		token.Fun,
		token.Ident,
		token.Number,
		token.String,
		token.Let,
		token.Mut,
	}
	if !slices.Contains(expectedTokenKinds, t.Kind) {
		p.diagnostic(
			t.Span,
			"unexpected token: expected one of %s, got %s",
			token.PrettyPrintTokenKinds(expectedTokenKinds),
			t.Kind,
		)
		return ParseFailed, false
	}
	var expr NodeID
	switch t.Kind { //nolint:exhaustive
	case token.Amp:
		ref, ok := p.ParseRefExpr()
		if !ok {
			return ParseFailed, false
		}
		expr = ref
	case token.Fun:
		fun, ok := p.ParseFun()
		if !ok {
			return ParseFailed, false
		}
		expr = fun
	case token.If:
		if_, ok := p.ParseIf()
		if !ok {
			return ParseFailed, false
		}
		expr = if_
	case token.Ident:
		ident, ok := p.ParseIdent()
		if !ok {
			return ParseFailed, false
		}
		expr = ident
	case token.Number:
		p.next()
		number, err := strconv.ParseInt(t.Value, 10, 64)
		if err != nil {
			p.diagnostic(t.Span, "invalid number: %s", t.Value)
			return ParseFailed, false
		}
		expr = p.NewInt(number, t.Span.Combine(p.span()))
	case token.True:
		p.next()
		expr = p.NewBool(true, t.Span)
	case token.False:
		p.next()
		expr = p.NewBool(false, t.Span)
	case token.String:
		p.next()
		expr = p.NewString(t.Value, t.Span.Combine(p.span()))
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
		panic(base.Errorf("this should have been catch earlier: unexpected token: %s", t.Kind))
	}
	return expr, true
}

func (p *Parser) ParseRefExpr() (NodeID, bool) {
	t, ok := p.expect(token.Amp)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	t, ok = p.expect(token.Ident)
	if !ok {
		return ParseFailed, false
	}
	name := Name{t.Value, t.Span}
	return p.NewRef(name, span.Combine(p.span())), true
}

func (p *Parser) ParseCallArgs() ([]NodeID, bool) {
	if _, ok := p.expect(token.LParen); !ok {
		return nil, false
	}
	args := []NodeID{}
	for {
		t, ok := p.peek()
		if !ok {
			return args, true
		}
		if t.Kind == token.RParen {
			p.next()
			return args, true
		}
		expr, ok := p.ParseExpr()
		if !ok {
			return args, true
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

func (p *Parser) ParseVar() (NodeID, bool) {
	t, ok := p.peek()
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
	nameToken, ok := p.expect(token.Ident)
	name := Name{nameToken.Value, nameToken.Span}
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.Eq); !ok {
		return ParseFailed, false
	}
	init, ok := p.ParseExpr()
	if !ok {
		return ParseFailed, false
	}
	return p.NewVar(name, init, mut, span.Combine(p.span())), true
}

func (p *Parser) ParseFunParams() ([]NodeID, bool) {
	if _, ok := p.expect(token.LParen); !ok {
		return nil, false
	}
	funParams := []NodeID{}
	for {
		t, ok := p.peek()
		if !ok {
			return funParams, true
		}
		switch t.Kind { //nolint:exhaustive
		case token.Ident, token.Mut:
			mut := false
			if t.Kind == token.Mut {
				mut = true
				p.next()
			}
			nameToken, ok := p.expect(token.Ident)
			name := Name{nameToken.Value, nameToken.Span}
			if !ok {
				return funParams, false
			}
			type_, ok := p.ParseType()
			if !ok {
				p.diagnostic(t.Span, "expected type, got %s", t.Kind)
				return funParams, false
			}
			param := p.NewFunParam(name, type_, mut, name.Span.Combine(p.span()))
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

func (p *Parser) ParseType() (NodeID, bool) {
	t, ok := p.next()
	if !ok {
		return ParseFailed, false
	}
	switch t.Kind { //nolint:exhaustive
	case token.TypeIdent:
		return p.NewSimpleType(Name{t.Value, t.Span}, t.Span.Combine(p.span())), true
	case token.Amp:
		inner, ok := p.ParseType()
		if !ok {
			return ParseFailed, false
		}
		return p.NewRefType(inner, t.Span.Combine(p.span())), true
	default:
		p.diagnostic(t.Span, "unexpected token: expected <type identifier> or &, got %s", t.Kind)
		return ParseFailed, false
	}
}

func (p *Parser) ParseIf() (NodeID, bool) {
	t, ok := p.expect(token.If)
	if !ok {
		return ParseFailed, false
	}
	cond, ok := p.ParseExpr()
	if !ok {
		return ParseFailed, false
	}
	then, ok := p.ParseBlock()
	if !ok {
		return ParseFailed, false
	}
	et, ok := p.peek()
	if !ok {
		return ParseFailed, false
	}
	if et.Kind != token.Else {
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
	return p.NewIdent(t.Value, t.Span.Combine(p.span())), true
}

func (p *Parser) parseBlock(createScope bool) (NodeID, bool) {
	t, ok := p.expect(token.LCurly)
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	exprs := []NodeID{}
	for {
		t, ok := p.peek()
		if !ok {
			break
		}
		if t.Kind == token.RCurly {
			p.next()
			break
		}
		expr, ok := p.ParseExpr()
		if !ok {
			return ParseFailed, false
		}
		exprs = append(exprs, expr)
	}
	return p.NewBlock(exprs, createScope, span.Combine(p.span())), true
}

func (p *Parser) diagnostic(span base.Span, msg string, msgArgs ...any) {
	p.Diagnostics = append(p.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}

func (p *Parser) next() (*token.Token, bool) {
	if p.pos >= len(p.tokens) {
		return nil, false
	}
	token := &p.tokens[p.pos]
	p.pos++
	return token, true
}

func (p *Parser) peek() (*token.Token, bool) {
	if p.pos >= len(p.tokens) {
		return nil, false
	}
	return &p.tokens[p.pos], true
}

func (p *Parser) expect(kind token.TokenKind) (*token.Token, bool) {
	t, ok := p.next()
	if ok && t.Kind == kind {
		return t, true
	}
	p.diagnostic(p.span(), "unexpected token: expected %s, got %s", kind, t.Kind)
	return nil, false
}

func (p *Parser) span() base.Span {
	token := p.tokens[min(max(p.pos-1, 0), len(p.tokens)-1)]
	return token.Span
}
