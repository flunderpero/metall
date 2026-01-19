//nolint:unparam
package ast

import (
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestParseOK(t *testing.T) {
	tests := []struct {
		name string
		kind string
		src  string
		want func(*TestAST) NodeID
	}{
		{
			"happy path", "file", `fun foo() Str { "hello" 123 } `,
			func(a *TestAST) NodeID {
				return a.file(
					a.fun("foo", nil, a.str_typ(), a.block(a.string_("hello"), a.int_(123))),
				)
			},
		},
		{
			"assign expr", "expr", `foo = 123`,
			func(a *TestAST) NodeID {
				return a.assign(a.ident("foo"), a.int_(123))
			},
		},
		{
			"var expr", "expr", `let foo = 123`,
			func(a *TestAST) NodeID {
				return a.var_("foo", a.int_(123))
			},
		},
		{
			"mut expr", "expr", `mut foo = 123`,
			func(a *TestAST) NodeID {
				return a.mut_var("foo", a.int_(123))
			},
		},
		{
			"block expr", "expr", `{ 0 "hello" }`,
			func(a *TestAST) NodeID {
				return a.block(a.int_(0), a.string_("hello"))
			},
		},
		{
			"empty block", "expr", `{ }`,
			func(a *TestAST) NodeID {
				return a.block()
			},
		},
		{
			"fun with (mut) params", "expr", `fun foo(a Int, mut b Str) Int { 123 }`,
			func(a *TestAST) NodeID {
				return a.fun("foo",
					[]NodeID{a.fun_param("a", a.int_typ()), a.mut_fun_param("b", a.str_typ())},
					a.int_typ(),
					a.block(a.int_(123)),
				)
			},
		},
		{
			"fun inside block", "expr", `{ fun foo() Str { "hello" 123 } }`,
			func(a *TestAST) NodeID {
				return a.block(
					a.fun("foo", nil, a.str_typ(), a.block(a.string_("hello"), a.int_(123))),
				)
			},
		},
		{
			"void fun", "expr", `fun foo() void {}`,
			func(a *TestAST) NodeID {
				return a.fun("foo", nil, a.void_typ(), a.block())
			},
		},
		{
			"call", "expr", `foo(123, "hello")`,
			func(a *TestAST) NodeID {
				return a.call(a.ident("foo"), a.int_(123), a.string_("hello"))
			},
		},
		{
			"call w/o args", "expr", `foo()`,
			func(a *TestAST) NodeID {
				return a.call(a.ident("foo"))
			},
		},
		{
			"chained calls", "expr", `foo()()`,
			func(a *TestAST) NodeID {
				return a.call(a.call(a.ident("foo")))
			},
		},

		{
			"ref ident expr", "expr", `&foo`,
			func(a *TestAST) NodeID {
				return a.ref("foo")
			},
		},
		{
			"deref expr", "expr", `*foo`,
			func(a *TestAST) NodeID {
				return a.deref(a.ident("foo"))
			},
		},
		{
			"nested deref expr", "expr", `**foo`,
			func(a *TestAST) NodeID {
				return a.deref(a.deref(a.ident("foo")))
			},
		},
		{
			"ref type", "expr", `fun foo() &Int {}`,
			func(a *TestAST) NodeID {
				return a.fun("foo", nil, a.ref_typ(a.int_typ()), a.block())
			},
		},
		{
			"nested ref type", "expr", `fun foo() &&Int {}`,
			func(a *TestAST) NodeID {
				return a.fun("foo", nil, a.ref_typ(a.ref_typ(a.int_typ())), a.block())
			},
		},
		{
			"deref assign", "expr", `*foo = bar`,
			func(a *TestAST) NodeID {
				return a.assign(a.deref(a.ident("foo")), a.ident("bar"))
			},
		},
		{
			"nested deref assign", "expr", `***foo = bar`,
			func(a *TestAST) NodeID {
				return a.assign(a.deref(a.deref(a.deref(a.ident("foo")))), a.ident("bar"))
			},
		},
		{
			"ref param", "expr", `{ fun foo(a &Int) void {} let b = 123 foo(&b) }`,
			func(a *TestAST) NodeID {
				return a.block(
					a.fun("foo", []NodeID{a.fun_param("a", a.ref_typ(a.int_typ()))}, a.void_typ(), a.block()),
					a.var_("b", a.int_(123)),
					a.call(a.ident("foo"), a.ref("b")),
				)
			},
		},
	}

	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	assert := base.NewAssert(t)
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := token.Lex(source)
			parser := NewParser(tokens)
			var gotRoot NodeID
			var ok bool
			switch tt.kind {
			case "expr":
				gotRoot, ok = parser.ParseExpr()
			case "file":
				gotRoot, ok = parser.ParseFile()
			default:
				t.Fatalf("unknown kind: %s", tt.kind)
			}
			assert.Equal(0, len(parser.Diagnostics), "diagnostics: %s", parser.Diagnostics)
			assert.Equal(true, ok, "parse function returned false")
			wantAST := NewTestAST()
			wantRoot := tt.want(wantAST)
			want := ast_to_list(wantAST.AST, wantRoot)
			got := ast_to_list(parser.AST, gotRoot)
			assert.Equal(want, got)
			// Make sure every node has a scope.
			for _, node := range got {
				scope := parser.ScopeGraph.NodeScope(node.ID)
				assert.NotNil(scope, "no scope for node %d", node.ID)
			}
		})
	}
}

func TestParseErr(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"rebind var", `{ let foo = 123 let foo = 321 }`, []string{
			"test.met:1:21: symbol already defined: foo\n" +
				`    { let foo = 123 let foo = 321 }` + "\n" +
				"                        ^^^",
		}},
		{"rebind fun", `{ let foo = 123 fun foo() void {} }`, []string{
			"test.met:1:21: symbol already defined: foo\n" +
				`    { let foo = 123 fun foo() void {} }` + "\n" +
				"                        ^^^",
		}},
		{"rebind fun param", `fun foo(bar Int) void { let bar = 123 }`, []string{
			"test.met:1:29: symbol already defined: bar\n" +
				`    fun foo(bar Int) void { let bar = 123 }` + "\n" +
				"                                ^^^",
		}},
		{"unexpected token", `=`, []string{
			"test.met:1:1: unexpected token: expected one of '&', '{', <fun>, <identifier>, <number>, <string>, <let>, <mut>, got =\n" +
				`    =` + "\n" +
				"    ^",
		}},
		{"assign to type", `{ Str = "hello" }`, []string{
			"test.met:1:3: unexpected token: expected one of '&', '{', <fun>, <identifier>, <number>, <string>, <let>, <mut>, got <type identifier>\n" +
				`    { Str = "hello" }` + "\n" +
				"      ^^^",
		}},
		{"nested ref expr", `{ &&foo }`, []string{
			"test.met:1:4: unexpected token: expected <identifier>, got &\n" +
				`    { &&foo }` + "\n" +
				"       ^",
		}},
		{"ref to literal", `{ &123 }`, []string{
			"test.met:1:4: unexpected token: expected <identifier>, got <number>\n" +
				`    { &123 }` + "\n" +
				"       ^^^",
		}},
	}

	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}

	assert := base.NewAssert(t)
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", []rune(tt.src))
			tokens := token.Lex(source)
			parser := NewParser(tokens)
			_, parseOK := parser.ParseExpr()
			diagnostics := parser.Diagnostics
			for i, want := range tt.want {
				if i >= len(diagnostics) {
					t.Fatalf("no more diagnostics, but wanted: %s", want)
				}
				assert.Equal(want, diagnostics[i].Display())
			}
			if len(diagnostics) > len(tt.want) {
				t.Fatalf("there are more diagnostics than expected: %s", diagnostics[len(tt.want):])
			}
			assert.Equal(false, parseOK, "ParseExpr should have failed")
		})
	}
}

type TestAST struct {
	*AST
	span base.Span
}

func NewTestAST() *TestAST {
	return &TestAST{AST: NewAST(), span: base.Span{}}
}

func (a *TestAST) file(decls ...NodeID) NodeID {
	return a.NewFile(decls, a.span)
}

func (a *TestAST) fun_param(name string, typ NodeID) NodeID {
	return a.NewFunParam(Name{name, a.span}, typ, false, a.span)
}

func (a *TestAST) mut_fun_param(name string, typ NodeID) NodeID {
	return a.NewFunParam(Name{name, a.span}, typ, true, a.span)
}

func (a *TestAST) fun(name string, params []NodeID, return_type NodeID, block NodeID) NodeID {
	if params == nil {
		params = []NodeID{}
	}
	return a.NewFun(Name{name, a.span}, params, return_type, block, a.span)
}

func (a *TestAST) string_(value string) NodeID {
	return a.NewString(value, a.span)
}

func (a *TestAST) int_(value int64) NodeID {
	return a.NewInt(value, a.span)
}

func (a *TestAST) block(exprs ...NodeID) NodeID {
	if exprs == nil {
		exprs = []NodeID{}
	}
	return a.NewBlock(exprs, a.span)
}

func (a *TestAST) str_typ() NodeID {
	return a.NewSimpleType(Name{"Str", a.span}, a.span)
}

func (a *TestAST) int_typ() NodeID {
	return a.NewSimpleType(Name{"Int", a.span}, a.span)
}

func (a *TestAST) void_typ() NodeID {
	return a.NewSimpleType(Name{"void", a.span}, a.span)
}

func (a *TestAST) ident(name string) NodeID {
	return a.NewIdent(name, a.span)
}

func (a *TestAST) assign(lhs NodeID, rhs NodeID) NodeID {
	return a.NewAssign(lhs, rhs, a.span)
}

func (a *TestAST) var_(name string, expr NodeID) NodeID {
	return a.NewVar(Name{name, a.span}, expr, false, a.span)
}

func (a *TestAST) mut_var(name string, expr NodeID) NodeID {
	return a.NewVar(Name{name, a.span}, expr, true, a.span)
}

func (a *TestAST) call(callee NodeID, args ...NodeID) NodeID {
	if args == nil {
		args = []NodeID{}
	}
	return a.NewCall(callee, args, a.span)
}

func (a *TestAST) ref(name string) NodeID {
	return a.NewRef(Name{name, a.span}, a.span)
}

func (a *TestAST) deref(expr NodeID) NodeID {
	return a.NewDeref(expr, a.span)
}

func (a *TestAST) ref_typ(typ NodeID) NodeID {
	return a.NewRefType(typ, a.span)
}

func ast_to_list(ast *AST, nodeID NodeID) []*Node {
	var nodes []*Node
	var f func(NodeID)
	f = func(nodeID NodeID) {
		node := ast.Node(nodeID)
		node.Span = base.Span{}
		switch kind := node.Kind.(type) {
		case FunParam:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case SimpleType:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case Fun:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case Var:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case Ref:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		}
		nodes = append(nodes, node)
		ast.Walk(nodeID, f)
	}
	f(nodeID)
	return nodes
}
