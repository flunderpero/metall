package internal

import (
	"strconv"
)

type Parser struct {
	Diagnostics Diagnostics
	id_         ASTID
	tokens      []Token
	pos         int
}

func NewParser(tokens []Token) *Parser {
	return &Parser{Diagnostics{}, ASTID(1), tokens, 0}
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
		case TFun:
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
	t, ok := p.expect(TFun)
	if !ok {
		return Fun{}, false
	}
	span := t.Span
	nameToken, ok := p.expect(TIdent)
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
	var returnType ASTType
	if t.Kind == TVoid {
		returnType = ASTType{TypeIdent{p.base(t.Span), "void"}}
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
	t, ok := p.expect(TLCurly)
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
		if t.Kind == TRCurly {
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
	lhs, ok := p.ParseSimpleExpr(0)
	if !ok {
		return Expr{}, false
	}
	switch t, ok := p.peek(); {
	case !ok:
		return lhs, true
	case t.Kind == TEq:
		p.next()
		if lhs.Kind != ExprIdent {
			p.diagnostic(lhs.Span(), "lhs of assignment must be an identifier, got %s", lhs.Kind)
			return Expr{}, false
		}
		right, ok := p.ParseSimpleExpr(0)
		if !ok {
			return Expr{}, false
		}
		return NewAssign(&Assign{p.base(lhs.Span()), *lhs.Ident, right}), true
	default:
		return lhs, true
	}
}

func (p *Parser) ParseSimpleExpr(minPrecedence int) (Expr, bool) {
	t, ok := p.peek()
	if !ok {
		return Expr{}, false
	}
	var expr Expr
	switch t.Kind { //nolint:exhaustive
	case TFun:
		fun, ok := p.ParseFun()
		if !ok {
			return Expr{}, false
		}
		expr = NewFun(&fun)
	case TIdent:
		ident, ok := p.ParseIdent()
		if !ok {
			return Expr{}, false
		}
		expr = NewIdent(&ident)
	case TNumber:
		p.next()
		number, err := strconv.ParseInt(t.Value, 10, 64)
		if err != nil {
			p.diagnostic(t.Span, "invalid number: %s", t.Value)
			return Expr{}, false
		}
		expr = NewInt(&IntExpr{p.base(t.Span), number})
	case TString:
		p.next()
		expr = NewString(&StringExpr{p.base(t.Span), t.Value})
	case TLCurly:
		block, ok := p.ParseBlock()
		if !ok {
			return Expr{}, false
		}
		expr = NewBlock(&block)
	case TLet, TMut:
		var_, ok := p.ParseVar()
		if !ok {
			return Expr{}, false
		}
		expr = NewVar(&var_)
	default:
		p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
		return Expr{}, false
	}
	if t, ok := p.peek(); ok && t.Kind == TLParen {
		callee := expr
		args, ok := p.ParseCallArgs()
		if !ok {
			return Expr{}, false
		}
		expr = NewCall(&Call{p.base(callee.Span()), callee, args})
	}
	return expr, true
}

func (p *Parser) ParseCallArgs() ([]Expr, bool) {
	if _, ok := p.expect(TLParen); !ok {
		return nil, false
	}
	args := []Expr{}
	for {
		t, ok := p.peek()
		if !ok {
			return args, true
		}
		if t.Kind == TRParen {
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
		case TComma:
		case TRParen:
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
	mut := t.Kind == TMut
	if mut {
		p.next()
	} else {
		if _, ok := p.expect(TLet); !ok {
			return Var{}, false
		}
	}
	span := t.Span
	nameToken, ok := p.expect(TIdent)
	name := Name{p.base(nameToken.Span), nameToken.Value}
	if !ok {
		return Var{}, false
	}
	if _, ok := p.expect(TEq); !ok {
		return Var{}, false
	}
	init, ok := p.ParseExpr()
	if !ok {
		return Var{}, false
	}
	return Var{p.base(span), name, init, mut}, true
}

func (p *Parser) ParseFunParams() ([]FunParam, bool) {
	if _, ok := p.expect(TLParen); !ok {
		return nil, false
	}
	funParams := make([]FunParam, 0)
	for {
		t, ok := p.peek()
		if !ok {
			return funParams, true
		}
		switch t.Kind { //nolint:exhaustive
		case TIdent, TMut:
			mut := false
			if t.Kind == TMut {
				mut = true
				p.next()
			}
			nameToken, ok := p.expect(TIdent)
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
		case TComma:
			p.next()
		case TRParen:
			p.next()
			return funParams, true
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return funParams, false
		}
	}
}

func (p *Parser) ParseType() (ASTType, bool) {
	t, ok := p.expect(TTypeIdent)
	if !ok {
		return ASTType{}, false
	}
	return ASTType{TypeIdent: TypeIdent{p.base(t.Span), t.Value}}, true
}

func (p *Parser) ParseIdent() (Ident, bool) {
	t, ok := p.expect(TIdent)
	if !ok {
		return Ident{}, false
	}
	return Ident{p.base(t.Span), t.Value}, true
}

func (p *Parser) diagnostic(span Span, msg string, msgArgs ...any) {
	p.Diagnostics = append(p.Diagnostics, *NewDiagnostic(span, msg, msgArgs...))
}

func (p *Parser) base(span Span) astBase {
	span = span.Combine(p.span())
	return astBase{p.id(), span}
}

func (p *Parser) id() ASTID {
	id := p.id_
	p.id_++
	return id
}

func (p *Parser) next() (*Token, bool) {
	if p.pos >= len(p.tokens) {
		return nil, false
	}
	token := &p.tokens[p.pos]
	p.pos++
	return token, true
}

func (p *Parser) peek() (*Token, bool) {
	if p.pos >= len(p.tokens) {
		return nil, false
	}
	return &p.tokens[p.pos], true
}

func (p *Parser) expect(kind TokenKind) (*Token, bool) {
	t, ok := p.next()
	if ok && t.Kind == kind {
		return t, true
	}
	p.diagnostic(p.span(), "expected token %s, got %s(%s)", kind, t.Kind, t.Value)
	return nil, false
}

func (p *Parser) span() Span {
	token := p.tokens[min(max(p.pos-1, 0), len(p.tokens)-1)]
	return token.Span
}
