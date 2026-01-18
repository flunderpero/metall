package parse

import (
	"slices"
	"strconv"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/lex"
)

type Parser struct {
	Diagnostics base.Diagnostics
	id_         NodeID
	tokens      []lex.Token
	pos         int
}

func NewParser(tokens []lex.Token) *Parser {
	return &Parser{base.Diagnostics{}, NodeID(1), tokens, 0}
}

func (p *Parser) ParseFile() (File, bool) {
	span := p.span()
	decls, ok := p.ParseDecls()
	return File{p.base(span), decls}, ok
}

func (p *Parser) ParseDecls() ([]Decl, bool) {
	decls := make([]Decl, 0)
	result := true
	for {
		t, ok := p.peek()
		if !ok {
			return decls, result
		}
		switch t.Kind { //nolint:exhaustive
		case lex.Fun:
			if fun, ok := p.ParseFun(); ok {
				decls = append(decls, Decl{DeclFun, &fun})
			}
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return decls, false
		}
	}
}

func (p *Parser) ParseFun() (Fun, bool) {
	t, ok := p.expect(lex.Fun)
	if !ok {
		return Fun{}, false
	}
	span := t.Span
	nameToken, ok := p.expect(lex.Ident)
	if !ok {
		return Fun{}, false
	}
	name := Name{p.base(nameToken.Span), nameToken.Value}
	params, ok := p.ParseFunParams()
	if !ok {
		return Fun{}, false
	}
	t, ok = p.peek()
	if !ok {
		return Fun{}, false
	}
	var returnType Type
	if t.Kind == lex.Void {
		returnType = NewSimpleType(&SimpleType{Name{p.base(t.Span), "void"}})
		p.next()
	} else {
		returnType, ok = p.ParseType()
		if !ok {
			return Fun{}, false
		}
	}
	block, ok := p.ParseBlock()
	if !ok {
		p.diagnostic(name.Span, "failed to parse block")
	}
	return Fun{p.base(span), name, params, returnType, block}, ok
}

func (p *Parser) ParseBlock() (Block, bool) {
	t, ok := p.expect(lex.LCurly)
	if !ok {
		return Block{}, false
	}
	span := t.Span
	exprs := []Expr{}
	for {
		t, ok := p.peek()
		if !ok {
			break
		}
		if t.Kind == lex.RCurly {
			p.next()
			break
		}
		expr, ok := p.ParseExpr()
		if !ok {
			return Block{p.base(span.Combine(t.Span)), exprs}, false
		}
		exprs = append(exprs, expr)
	}
	return Block{p.base(span), exprs}, true
}

func (p *Parser) ParseExpr() (Expr, bool) {
	lhs, ok := p.ParseUnaryExpr(0)
	if !ok {
		return Expr{}, false
	}
	switch t, ok := p.peek(); {
	case !ok:
		return lhs, true
	case t.Kind == lex.Eq:
		p.next()
		rhs, ok := p.ParseExpr()
		if !ok {
			return Expr{}, false
		}
		return NewAssign(&Assign{p.base(lhs.Span()), lhs, rhs}), true
	default:
		return lhs, true
	}
}

func (p *Parser) ParseUnaryExpr(minPrecedence int) (Expr, bool) {
	t, ok := p.peek()
	if !ok {
		return Expr{}, false
	}
	var expr Expr
	switch t.Kind { //nolint:exhaustive
	case lex.Star:
		p.next()
		expr, ok = p.ParseUnaryExpr(minPrecedence)
		if !ok {
			return Expr{}, false
		}
		expr = NewDeref(&Deref{p.base(t.Span), expr})
	default:
		expr, ok = p.ParsePrimaryExpr(minPrecedence)
		if !ok {
			return Expr{}, false
		}
	}
	for {
		if t, ok := p.peek(); ok && t.Kind == lex.LParen {
			callee := expr
			args, ok := p.ParseCallArgs()
			if !ok {
				return Expr{}, false
			}
			expr = NewCall(&Call{p.base(callee.Span()), callee, args})
		} else {
			break
		}
	}
	return expr, true
}

func (p *Parser) ParsePrimaryExpr(minPrecedence int) (Expr, bool) { //nolint:funlen
	t, ok := p.peek()
	if !ok {
		return Expr{}, false
	}
	expectedTokenKinds := []lex.TokenKind{
		lex.Amp,
		lex.LCurly,
		lex.Fun,
		lex.Ident,
		lex.Number,
		lex.String,
		lex.Let,
		lex.Mut,
	}
	if !slices.Contains(expectedTokenKinds, t.Kind) {
		p.diagnostic(
			t.Span,
			"unexpected token: expected one of %s, got %s",
			lex.PrettyPrintTokenKinds(expectedTokenKinds),
			t.Kind,
		)
		return Expr{}, false
	}
	var expr Expr
	switch t.Kind { //nolint:exhaustive
	case lex.Amp:
		ref, ok := p.ParseRefExpr()
		if !ok {
			return Expr{}, false
		}
		expr = NewRef(&ref)
	case lex.Fun:
		fun, ok := p.ParseFun()
		if !ok {
			return Expr{}, false
		}
		expr = NewFun(&fun)
	case lex.Ident:
		ident, ok := p.ParseIdent()
		if !ok {
			return Expr{}, false
		}
		expr = NewIdent(&ident)
	case lex.Number:
		p.next()
		number, err := strconv.ParseInt(t.Value, 10, 64)
		if err != nil {
			p.diagnostic(t.Span, "invalid number: %s", t.Value)
			return Expr{}, false
		}
		expr = NewInt(&Int{p.base(t.Span), number})
	case lex.String:
		p.next()
		expr = NewString(&String{p.base(t.Span), t.Value})
	case lex.LCurly:
		block, ok := p.ParseBlock()
		if !ok {
			return Expr{}, false
		}
		expr = NewBlock(&block)
	case lex.Let, lex.Mut:
		var_, ok := p.ParseVar()
		if !ok {
			return Expr{}, false
		}
		expr = NewVar(&var_)
	default:
		panic(base.Errorf("this should have been catch earlier: unexpected token: %s", t.Kind))
	}
	return expr, true
}

func (p *Parser) ParseRefExpr() (Ref, bool) {
	t, ok := p.expect(lex.Amp)
	if !ok {
		return Ref{}, false
	}
	ident, ok := p.ParseIdent()
	if !ok {
		return Ref{}, false
	}
	return Ref{p.base(t.Span), ident}, true
}

func (p *Parser) ParseCallArgs() ([]Expr, bool) {
	if _, ok := p.expect(lex.LParen); !ok {
		return nil, false
	}
	args := []Expr{}
	for {
		t, ok := p.peek()
		if !ok {
			return args, true
		}
		if t.Kind == lex.RParen {
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
		case lex.Comma:
		case lex.RParen:
			return args, true
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return args, false
		}
	}
}

func (p *Parser) ParseVar() (Var, bool) {
	t, ok := p.peek()
	if !ok {
		return Var{}, false
	}
	mut := t.Kind == lex.Mut
	if mut {
		p.next()
	} else {
		if _, ok := p.expect(lex.Let); !ok {
			return Var{}, false
		}
	}
	span := t.Span
	nameToken, ok := p.expect(lex.Ident)
	name := Name{p.base(nameToken.Span), nameToken.Value}
	if !ok {
		return Var{}, false
	}
	if _, ok := p.expect(lex.Eq); !ok {
		return Var{}, false
	}
	init, ok := p.ParseExpr()
	if !ok {
		return Var{}, false
	}
	return Var{p.base(span), name, init, mut}, true
}

func (p *Parser) ParseFunParams() ([]FunParam, bool) {
	if _, ok := p.expect(lex.LParen); !ok {
		return nil, false
	}
	funParams := make([]FunParam, 0)
	for {
		t, ok := p.peek()
		if !ok {
			return funParams, true
		}
		switch t.Kind { //nolint:exhaustive
		case lex.Ident, lex.Mut:
			mut := false
			if t.Kind == lex.Mut {
				mut = true
				p.next()
			}
			nameToken, ok := p.expect(lex.Ident)
			name := Name{p.base(nameToken.Span), nameToken.Value}
			if !ok {
				return funParams, false
			}
			type_, ok := p.ParseType()
			if !ok {
				p.diagnostic(t.Span, "expected type, got %s", t.Kind)
				return funParams, false
			}
			funParams = append(funParams, FunParam{p.base(name.Span), name, type_, mut})
		case lex.Comma:
			p.next()
		case lex.RParen:
			p.next()
			return funParams, true
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return funParams, false
		}
	}
}

func (p *Parser) ParseType() (Type, bool) {
	t, ok := p.next()
	if !ok {
		return Type{}, false
	}
	switch t.Kind { //nolint:exhaustive
	case lex.TypeIdent:
		return NewSimpleType(&SimpleType{Name{p.base(t.Span), t.Value}}), true
	case lex.Amp:
		inner, ok := p.ParseType()
		if !ok {
			return Type{}, false
		}
		return NewRefType(&RefType{p.base(t.Span), inner}), true
	default:
		p.diagnostic(t.Span, "unexpected token: expected <type identifier> or &, got %s", t.Kind)
		return Type{}, false
	}
}

func (p *Parser) ParseIdent() (Ident, bool) {
	t, ok := p.expect(lex.Ident)
	if !ok {
		return Ident{}, false
	}
	return Ident{p.base(t.Span), t.Value}, true
}

func (p *Parser) diagnostic(span base.Span, msg string, msgArgs ...any) {
	p.Diagnostics = append(p.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}

func (p *Parser) base(span base.Span) astBase {
	span = span.Combine(p.span())
	return astBase{p.id(), span}
}

func (p *Parser) id() NodeID {
	id := p.id_
	p.id_++
	return id
}

func (p *Parser) next() (*lex.Token, bool) {
	if p.pos >= len(p.tokens) {
		return nil, false
	}
	token := &p.tokens[p.pos]
	p.pos++
	return token, true
}

func (p *Parser) peek() (*lex.Token, bool) {
	if p.pos >= len(p.tokens) {
		return nil, false
	}
	return &p.tokens[p.pos], true
}

func (p *Parser) expect(kind lex.TokenKind) (*lex.Token, bool) {
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
