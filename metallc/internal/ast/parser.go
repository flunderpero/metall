package ast

import (
	"math/big"
	"slices"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

var ReservedIdents = []string{"Arena"} //nolint:gochecknoglobals

const ParseFailed = NodeID(0)

type Parser struct {
	*AST
	Diagnostics base.Diagnostics
	tokens      []token.Token
	pos         int
}

func NewParser(tokens []token.Token, firstNodeID NodeID) *Parser {
	// Strip comments and whitespace tokens.
	stripped := []token.Token{}
	for _, t := range tokens {
		switch t.Kind { //nolint:exhaustive
		case token.Comment, token.Whitespace:
		default:
			stripped = append(stripped, t)
		}
	}
	return &Parser{NewAST(firstNodeID), base.Diagnostics{}, stripped, 0}
}

func (p *Parser) ParseModule() (NodeID, bool) {
	span := p.span()
	source := span.Source
	decls, ok := p.ParseDecls()
	if !ok {
		return ParseFailed, false
	}
	return p.NewModule(source.FileName, source.Module, source.Main, decls, span.Combine(p.span())), ok
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
		default:
			p.diagnostic(t.Span, "unexpected token: %s", t.Kind)
			return decls, false
		}
	}
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
	if slices.Contains(ReservedIdents, nameToken.Value) {
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

func (p *Parser) ParseStructLiteral() (NodeID, bool) {
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
	return p.NewStructLiteral(ident, args, struct_.Span.Combine(p.span())), true
}

func (p *Parser) ParseNew(mut bool) (NodeID, bool) { //nolint:funlen
	var newToken *token.Token
	var ok bool
	if mut {
		newToken, ok = p.expect(token.NewMut)
	} else {
		newToken, ok = p.expect(token.New)
	}
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.LParen); !ok {
		return ParseFailed, false
	}
	alloc, ok := p.parseAllocator()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.Comma); !ok {
		return ParseFailed, false
	}
	// Parse the target: struct literal or array alloc.
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	var target NodeID
	switch t.Kind { //nolint:exhaustive
	case token.LBracket:
		arrType, ok := p.ParseArrayOrSliceType()
		if !ok {
			return ParseFailed, false
		}
		if _, ok := p.Node(arrType).Kind.(ArrayType); !ok {
			p.diagnostic(p.Node(arrType).Span, "use `make` to allocate slices")
			return ParseFailed, false
		}
		if _, ok := p.expect(token.LParen); !ok {
			return ParseFailed, false
		}
		var defaultValue *NodeID
		if next, ok := p.mayPeek(); ok && next.Kind != token.RParen {
			val, ok := p.ParseExpr(0)
			if !ok {
				return ParseFailed, false
			}
			defaultValue = &val
		}
		if _, ok := p.expect(token.RParen); !ok {
			return ParseFailed, false
		}
		target = p.NewNewArray(arrType, defaultValue, t.Span.Combine(p.span()))
	case token.TypeIdent:
		target, ok = p.ParseStructLiteral()
		if !ok {
			return ParseFailed, false
		}
	default:
		p.diagnostic(
			t.Span,
			"unexpected token: expected one of %s, got %s",
			token.PrettyPrintTokenKinds([]token.TokenKind{token.LBracket, token.TypeIdent}),
			t.Kind,
		)
		return ParseFailed, false
	}
	if _, ok := p.expect(token.RParen); !ok {
		return ParseFailed, false
	}
	return p.NewNew(alloc, target, mut, newToken.Span.Combine(p.span())), true
}

func (p *Parser) ParseMakeSlice() (NodeID, bool) {
	makeToken, ok := p.expect(token.Make)
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.LParen); !ok {
		return ParseFailed, false
	}
	alloc, ok := p.parseAllocator()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.Comma); !ok {
		return ParseFailed, false
	}
	sliceType, ok := p.ParseArrayOrSliceType()
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.Node(sliceType).Kind.(SliceType); !ok {
		p.diagnostic(p.Node(sliceType).Span, "make only supports slice types")
		return ParseFailed, false
	}
	if _, ok := p.expect(token.LParen); !ok {
		return ParseFailed, false
	}
	lenExpr, ok := p.ParseExpr(0)
	if !ok {
		return ParseFailed, false
	}
	var defaultValue *NodeID
	if next, ok := p.mayPeek(); ok && next.Kind == token.Comma {
		p.next()
		val, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		defaultValue = &val
	}
	if _, ok := p.expect(token.RParen); !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.RParen); !ok {
		return ParseFailed, false
	}
	return p.NewMakeSlice(alloc, sliceType, lenExpr, defaultValue, makeToken.Span.Combine(p.span())), true
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
		t, ok := p.mayPeek()
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
	if t.Kind == token.Not {
		p.next()
		expr, ok := p.ParseExpr(0)
		if !ok {
			return ParseFailed, false
		}
		return p.NewUnary(UnaryOpNot, expr, t.Span.Combine(p.span())), true
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
		}[t.Kind]
		if !ok {
			return lhs, true
		}
		precedence := map[BinaryOp]int{
			BinaryOpOr:  0,
			BinaryOpAnd: 1,
			BinaryOpEq:  2,
			BinaryOpNeq: 2,
			BinaryOpLt:  2,
			BinaryOpLte: 2,
			BinaryOpGt:  2,
			BinaryOpGte: 2,
			BinaryOpAdd: 3,
			BinaryOpSub: 3,
			BinaryOpMul: 4,
			BinaryOpDiv: 4,
			BinaryOpMod: 4,
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

func (p *Parser) ParsePostfixExpr(minPrecedence int) (NodeID, bool) {
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
			expr = p.NewCall(callee, args, span.Combine(p.span()))
			continue
		case token.LBracketImmediate:
			p.next()
			index, ok := p.ParseExpr(minPrecedence)
			if !ok {
				return ParseFailed, false
			}
			if _, ok := p.expect(token.RBracket); !ok {
				return ParseFailed, false
			}
			expr = p.NewIndex(expr, index, span.Combine(p.span()))
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
		fun, ok := p.ParseFun()
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
		ident, ok := p.ParseIdent()
		if !ok {
			return ParseFailed, false
		}
		expr = ident
	case token.TypeIdent:
		// Peek ahead: if the next token is `.`, this is a qualified name
		// (e.g. `Foo.bar`), not a struct literal.
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
			struct_literal, ok := p.ParseStructLiteral()
			if !ok {
				return ParseFailed, false
			}
			expr = struct_literal
		}
	case token.New:
		allocation, ok := p.ParseNew(false)
		if !ok {
			return ParseFailed, false
		}
		expr = allocation
	case token.NewMut:
		allocation, ok := p.ParseNew(true)
		if !ok {
			return ParseFailed, false
		}
		expr = allocation
	case token.Make:
		makeSlice, ok := p.ParseMakeSlice()
		if !ok {
			return ParseFailed, false
		}
		expr = makeSlice
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
	t, ok = p.expect(token.Ident)
	if !ok {
		return ParseFailed, false
	}
	name := Name{t.Value, t.Span}
	return p.NewRef(name, mut, span.Combine(p.span())), true
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
	name := Name{nameToken.Value, nameToken.Span}
	if !ok {
		return ParseFailed, false
	}
	if _, ok := p.expect(token.Eq); !ok {
		return ParseFailed, false
	}
	init, ok := p.ParseExpr(0)
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
	typ, ok := p.ParseType()
	if !ok {
		return ParseFailed, false
	}
	if len_ == nil {
		return p.NewSliceType(typ, span.Combine(p.span())), true
	}
	return p.NewArrayType(typ, *len_, span.Combine(p.span())), true
}

func (p *Parser) ParseType() (NodeID, bool) {
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	span := t.Span
	switch t.Kind { //nolint:exhaustive
	case token.TypeIdent:
		p.next()
		typeArgs, ok := p.parseTypeArgs()
		if !ok {
			return ParseFailed, false
		}
		return p.NewSimpleType(Name{t.Value, span}, typeArgs, span.Combine(p.span())), true
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
	default:
		p.diagnostic(span, "unexpected token: expected <type identifier> or &, got %s", t.Kind)
		return ParseFailed, false
	}
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
	return p.NewFor(cond, body, span.Combine(p.span())), true
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

// parseAllocator parses `@a` or a field access chain ending in an
// AllocatorIdent, e.g. `holder.@a` or `a.b.@c`.
func (p *Parser) parseAllocator() (NodeID, bool) {
	var alloc NodeID
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	switch t.Kind { //nolint:exhaustive
	case token.AllocatorIdent:
		p.next()
		alloc = p.NewIdent(t.Value, nil, t.Span)
	case token.Ident:
		p.next()
		alloc = p.NewIdent(t.Value, nil, t.Span)
		for {
			if _, ok := p.expect(token.Dot); !ok {
				return ParseFailed, false
			}
			field, ok := p.mustPeek()
			if !ok {
				return ParseFailed, false
			}
			switch field.Kind { //nolint:exhaustive
			case token.AllocatorIdent:
				p.next()
				alloc = p.NewFieldAccess(alloc, Name{field.Value, field.Span}, nil, t.Span.Combine(field.Span))
			case token.Ident:
				p.next()
				alloc = p.NewFieldAccess(alloc, Name{field.Value, field.Span}, nil, t.Span.Combine(field.Span))
				continue
			default:
				p.diagnostic(field.Span, "expected field name or allocator, got %s", field.Kind)
				return ParseFailed, false
			}
			break
		}
	default:
		p.diagnostic(t.Span, "expected allocator, got %s", t.Kind)
		return ParseFailed, false
	}
	return alloc, true
}

func (p *Parser) parseTypeParams() ([]NodeID, bool) {
	if t, ok := p.mayPeek(); !ok || t.Kind != token.LtImmediate {
		return nil, true
	}
	open, _ := p.next()
	params := []NodeID{}
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
		params = append(params, p.NewTypeParam(Name{t.Value, t.Span}, constraint, t.Span))
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
		if t.Kind == token.Gt {
			if len(args) == 0 {
				p.diagnostic(open.Span.Combine(t.Span), "empty type argument list")
				return nil, false
			}
			p.next()
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
		p.next()
		if _, ok := p.expect(token.Dot); !ok {
			return FunDecl{}, base.Span{}, false
		}
		method, ok := p.expect(token.Ident)
		if !ok {
			return FunDecl{}, base.Span{}, false
		}
		name = Name{peek.Value + "." + method.Value, peek.Span.Combine(method.Span)}
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
	returnType, ok := p.parseFunReturnType()
	if !ok {
		return FunDecl{}, base.Span{}, false
	}
	return FunDecl{Name: name, TypeParams: typeParams, Params: params, ReturnType: returnType}, t.Span, true
}

func (p *Parser) parseFunReturnType() (NodeID, bool) {
	t, ok := p.mustPeek()
	if !ok {
		return ParseFailed, false
	}
	if t.Kind == token.Void {
		p.next()
		return p.NewSimpleType(Name{"void", t.Span}, nil, t.Span), true
	} else {
		return p.ParseType()
	}
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
