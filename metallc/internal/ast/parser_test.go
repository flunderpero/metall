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
					a.fun("foo", nil, a.str_typ(), a.fun_block(a.string_("hello"), a.int_(123))),
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
			"fun with &mut param", "expr", `fun foo(a Int, b &mut Str) Int { 123 }`,
			func(a *TestAST) NodeID {
				return a.fun("foo",
					[]NodeID{a.fun_param("a", a.int_typ()), a.fun_param("b", a.mut_ref_typ(a.str_typ()))},
					a.int_typ(),
					a.fun_block(a.int_(123)),
				)
			},
		},
		{
			"fun inside block", "expr", `{ fun foo() Str { "hello" 123 } }`,
			func(a *TestAST) NodeID {
				return a.block(
					a.fun("foo", nil, a.str_typ(), a.fun_block(a.string_("hello"), a.int_(123))),
				)
			},
		},
		{
			"void fun", "expr", `fun foo() void {}`,
			func(a *TestAST) NodeID {
				return a.fun("foo", nil, a.void_typ(), a.fun_block())
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

		{"true", "expr", "true", func(a *TestAST) NodeID { return a.bool_(true) }},
		{"false", "expr", "false", func(a *TestAST) NodeID { return a.bool_(false) }},
		{
			"if then else", "expr", `if a { 42 } else { 123 }`,
			func(a *TestAST) NodeID {
				cond := a.ident("a")
				then := a.block(a.int_(42))
				else_ := a.block(a.int_(123))
				return a.if_(cond, then, &else_)
			},
		},

		{"struct", "expr", "struct Planet { name Str mut diameter Int }", func(a *TestAST) NodeID {
			return a.struct_("Planet", a.struct_field("name", a.str_typ()), a.mut_struct_field("diameter", a.int_typ()))
		}},
		{"struct literal", "expr", "Planet(\"Earth\", 12500)", func(a *TestAST) NodeID {
			return a.struct_lit(a.ident("Planet"), a.string_("Earth"), a.int_(12500))
		}},
		{"field read access", "expr", "earth.name", func(a *TestAST) NodeID {
			return a.field_access(a.ident("earth"), "name")
		}},
		{"field write access", "expr", "earth.name = \"Mother\"", func(a *TestAST) NodeID {
			return a.assign(a.field_access(a.ident("earth"), "name"), a.string_("Mother"))
		}},
		{"chained field access", "expr", "earth.info.name", func(a *TestAST) NodeID {
			return a.field_access(a.field_access(a.ident("earth"), "info"), "name")
		}},
		{"field access call", "expr", "earth.info.name()", func(a *TestAST) NodeID {
			return a.call(a.field_access(a.field_access(a.ident("earth"), "info"), "name"))
		}},

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
				return a.fun("foo", nil, a.ref_typ(a.int_typ()), a.fun_block())
			},
		},
		{
			"nested ref type", "expr", `fun foo() &&Int {}`,
			func(a *TestAST) NodeID {
				return a.fun("foo", nil, a.ref_typ(a.ref_typ(a.int_typ())), a.fun_block())
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
					a.fun("foo", []NodeID{a.fun_param("a", a.ref_typ(a.int_typ()))}, a.void_typ(), a.fun_block()),
					a.var_("b", a.int_(123)),
					a.call(a.ident("foo"), a.ref("b")),
				)
			},
		},

		{"declare allocator", "expr", "alloc @test = Arena(123)", func(a *TestAST) NodeID {
			return a.alloc_init("@test", "Arena", a.int_(123))
		}},
		{"alloc", "expr", `Planet@test()`, func(a *TestAST) NodeID {
			return a.struct_lit_alloc("@test", a.ident("Planet"))
		}},
		{
			"alloc as fun param", "expr", "fun foo(@alloc Arena, name Str, @alloc2 Arena) void {}",
			func(a *TestAST) NodeID {
				return a.fun("foo", []NodeID{
					a.fun_param("@alloc", a.typ("Arena")),
					a.fun_param("name", a.str_typ()),
					a.fun_param("@alloc2", a.typ("Arena")),
				}, a.void_typ(), a.fun_block())
			},
		},
		{
			"alloc in call", "expr", "foo(@a)",
			func(a *TestAST) NodeID {
				return a.call(a.ident("foo"), a.ident("@a"))
			},
		},

		{"array type", "expr", `fun foo(a [Int 5]) void {}}`, func(a *TestAST) NodeID {
			return a.fun("foo", []NodeID{a.fun_param("a", a.arr_typ(a.int_typ(), 5))}, a.void_typ(), a.fun_block())
		}},
		{"array literal", "expr", `[1, 2, 3]`, func(a *TestAST) NodeID {
			return a.arr_lit(a.int_(1), a.int_(2), a.int_(3))
		}},
		{"index read", "expr", `a[1]`, func(a *TestAST) NodeID {
			return a.index(a.ident("a"), a.int_(1))
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
		})
	}
}

func TestParseErr(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"unexpected token", `=`, []string{
			"test.met:1:1: unexpected token: expected start of an expression, got =\n" +
				`    =` + "\n" +
				"    ^",
		}},
		{"assign to type", `{ Str = "hello" }`, []string{
			"test.met:1:7: unexpected token: expected (, got =\n" +
				`    { Str = "hello" }` + "\n" +
				"          ^",
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

		{"Arena is a reserved work", `struct Arena{name Str}`, []string{
			"test.met:1:8: reserved word: Arena\n" +
				`    struct Arena{name Str}` + "\n" +
				"           ^^^^^",
		}},
		{"alloc ident is not an expression", `@test`, []string{
			"test.met:1:1: unexpected token: expected start of an expression, got <allocator identifier>\n" +
				`    @test` + "\n" +
				"    ^^^^^",
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
	return a.NewFunParam(Name{name, a.span}, typ, a.span)
}

func (a *TestAST) fun(name string, params []NodeID, return_type NodeID, block NodeID) NodeID {
	if params == nil {
		params = []NodeID{}
	}
	return a.NewFun(Name{name, a.span}, params, return_type, block, a.span)
}

func (a *TestAST) struct_field(name string, typ NodeID) NodeID {
	return a.NewStructField(Name{name, a.span}, typ, false, a.span)
}

func (a *TestAST) mut_struct_field(name string, typ NodeID) NodeID {
	return a.NewStructField(Name{name, a.span}, typ, true, a.span)
}

func (a *TestAST) struct_(name string, fields ...NodeID) NodeID {
	if fields == nil {
		fields = []NodeID{}
	}
	return a.NewStruct(Name{name, a.span}, fields, a.span)
}

func (a *TestAST) struct_lit(struct_ NodeID, args ...NodeID) NodeID {
	if args == nil {
		args = []NodeID{}
	}
	return a.NewStructLiteral(nil, struct_, args, a.span)
}

func (a *TestAST) struct_lit_alloc(alloc string, struct_ NodeID, args ...NodeID) NodeID {
	if args == nil {
		args = []NodeID{}
	}
	return a.NewStructLiteral(&Name{alloc, a.span}, struct_, args, a.span)
}

func (a *TestAST) field_access(base NodeID, field string) NodeID {
	return a.NewFieldAccess(base, Name{field, a.span}, a.span)
}

func (a *TestAST) if_(cond NodeID, then NodeID, else_ *NodeID) NodeID {
	return a.NewIf(cond, then, else_, a.span)
}

func (a *TestAST) bool_(value bool) NodeID {
	return a.NewBool(value, a.span)
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
	return a.NewBlock(exprs, true, a.span)
}

func (a *TestAST) fun_block(exprs ...NodeID) NodeID {
	if exprs == nil {
		exprs = []NodeID{}
	}
	return a.NewBlock(exprs, false, a.span)
}

func (a *TestAST) str_typ() NodeID {
	return a.NewSimpleType(Name{"Str", a.span}, a.span)
}

func (a *TestAST) int_typ() NodeID {
	return a.NewSimpleType(Name{"Int", a.span}, a.span)
}

func (a *TestAST) arr_typ(typ NodeID, size int) NodeID {
	return a.NewArrayType(typ, int64(size), a.span)
}

func (a *TestAST) arr_lit(elems ...NodeID) NodeID {
	if elems == nil {
		elems = []NodeID{}
	}
	return a.NewArrayLiteral(elems, a.span)
}

func (a *TestAST) index(base NodeID, index NodeID) NodeID {
	return a.NewIndex(base, index, a.span)
}

func (a *TestAST) void_typ() NodeID {
	return a.NewSimpleType(Name{"void", a.span}, a.span)
}

func (a *TestAST) typ(name string) NodeID {
	return a.NewSimpleType(Name{name, a.span}, a.span)
}

func (a *TestAST) ident(name string) NodeID {
	return a.NewIdent(name, a.span)
}

func (a *TestAST) alloc_init(name string, allocator string, args ...NodeID) NodeID {
	if args == nil {
		args = []NodeID{}
	}
	return a.NewAllocInit(Name{name, a.span}, Name{allocator, a.span}, args, a.span)
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
	return a.NewRefType(typ, false, a.span)
}

func (a *TestAST) mut_ref_typ(typ NodeID) NodeID {
	return a.NewRefType(typ, true, a.span)
}

func ast_to_list(ast *AST, nodeID NodeID) []*Node {
	var nodes []*Node
	var f func(NodeID)
	f = func(nodeID NodeID) {
		node := ast.Node(nodeID)
		node.Span = base.Span{}
		switch kind := node.Kind.(type) {
		case SimpleType:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case FunParam:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case Fun:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case StructField:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case Struct:
			kind.Name.Span = base.Span{}
			node.Kind = kind
		case StructLiteral:
			if kind.Alloc != nil {
				kind.Alloc.Span = base.Span{}
			}
			node.Kind = kind
		case AllocInit:
			kind.Name.Span = base.Span{}
			kind.Allocator.Span = base.Span{}
			node.Kind = kind
		case FieldAccess:
			kind.Field.Span = base.Span{}
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
