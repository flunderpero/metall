package ast

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/base"
)

type NodeID int

func (id NodeID) String() string {
	return fmt.Sprintf("n%d", id)
}

type Node struct {
	ID   NodeID
	Kind Kind
	Span base.Span
}

type Kind interface {
	isKind()
}

type Ident struct {
	Name string
}

func (Ident) isKind() {}

type Name struct {
	Name string
	Span base.Span
}

type File struct {
	Decls []NodeID
}

func (File) isKind() {}

type SimpleType struct {
	Name Name
}

func (SimpleType) isKind() {}

type RefType struct {
	Type NodeID
}

func (RefType) isKind() {}

type FunParam struct {
	Name Name
	Type NodeID
	Mut  bool
}

func (FunParam) isKind() {}

type Fun struct {
	Name       Name
	Params     []NodeID
	ReturnType NodeID
	Block      NodeID
}

func (Fun) isKind() {}

type Int struct {
	Value int64
}

func (Int) isKind() {}

type String struct {
	Value string
}

func (String) isKind() {}

type Assign struct {
	LHS NodeID
	RHS NodeID
}

func (Assign) isKind() {}

type Var struct {
	Name Name
	Expr NodeID
	Mut  bool
}

func (Var) isKind() {}

type Block struct {
	Exprs       []NodeID
	CreateScope bool
}

func (Block) isKind() {}

type Call struct {
	Callee NodeID
	Args   []NodeID
}

func (Call) isKind() {}

type Ref struct {
	Name Name
}

func (Ref) isKind() {}

type Deref struct {
	Expr NodeID
}

func (Deref) isKind() {}

type AST struct {
	nodes     map[NodeID]*Node
	nextID_   NodeID
	onNewNode func(*Node)
}

func NewAST() *AST {
	return &AST{nextID_: 1, nodes: make(map[NodeID]*Node), onNewNode: nil}
}

func (a *AST) NewAssign(lhs NodeID, value NodeID, span base.Span) NodeID {
	return a.node(Assign{LHS: lhs, RHS: value}, span)
}

func (a *AST) NewBlock(exprs []NodeID, createScope bool, span base.Span) NodeID {
	return a.node(Block{Exprs: exprs, CreateScope: createScope}, span)
}

func (a *AST) NewCall(callee NodeID, args []NodeID, span base.Span) NodeID {
	return a.node(Call{Callee: callee, Args: args}, span)
}

func (a *AST) NewDeref(expr NodeID, span base.Span) NodeID {
	return a.node(Deref{Expr: expr}, span)
}

func (a *AST) NewFile(decls []NodeID, span base.Span) NodeID {
	return a.node(File{Decls: decls}, span)
}

func (a *AST) NewFun(name Name, params []NodeID, returnType NodeID, block NodeID, span base.Span) NodeID {
	return a.node(Fun{Name: name, Params: params, ReturnType: returnType, Block: block}, span)
}

func (a *AST) NewFunParam(name Name, type_ NodeID, mut bool, span base.Span) NodeID {
	return a.node(FunParam{Name: name, Type: type_, Mut: mut}, span)
}

func (a *AST) NewIdent(name string, span base.Span) NodeID {
	return a.node(Ident{Name: name}, span)
}

func (a *AST) NewInt(value int64, span base.Span) NodeID {
	return a.node(Int{Value: value}, span)
}

func (a *AST) NewRef(name Name, span base.Span) NodeID {
	return a.node(Ref{Name: name}, span)
}

func (a *AST) NewString(value string, span base.Span) NodeID {
	return a.node(String{Value: value}, span)
}

func (a *AST) NewSimpleType(name Name, span base.Span) NodeID {
	return a.node(SimpleType{Name: name}, span)
}

func (a *AST) NewRefType(type_ NodeID, span base.Span) NodeID {
	return a.node(RefType{Type: type_}, span)
}

func (a *AST) NewVar(name Name, expr NodeID, mut bool, span base.Span) NodeID {
	return a.node(Var{Name: name, Expr: expr, Mut: mut}, span)
}

func (a *AST) Node(id NodeID) *Node {
	node, ok := a.nodes[id]
	if !ok {
		panic(base.Errorf("unknown node id: %d", id))
	}
	return node
}

func (a *AST) Iter(f func(NodeID) bool) {
	for id := range a.nodes {
		if !f(id) {
			return
		}
	}
}

func (a *AST) Walk(id NodeID, f func(NodeID)) {
	node := a.Node(id)
	switch kind := node.Kind.(type) {
	case Assign:
		f(kind.LHS)
		f(kind.RHS)
	case Block:
		for i := range len(kind.Exprs) {
			f(kind.Exprs[i])
		}
	case Call:
		f(kind.Callee)
		for i := range len(kind.Args) {
			f(kind.Args[i])
		}
	case Deref:
		f(kind.Expr)
	case File:
		for i := range len(kind.Decls) {
			f(kind.Decls[i])
		}
	case Fun:
		for i := range len(kind.Params) {
			f(kind.Params[i])
		}
		f(kind.ReturnType)
		f(kind.Block)
	case FunParam:
		f(kind.Type)
	case Ident:
	case Int:
	case String:
	case Var:
		f(kind.Expr)
	case SimpleType:
	case Ref:
	case RefType:
		f(kind.Type)
	default:
		panic(base.Errorf("unknown node kind: %T", kind))
	}
}

func (a *AST) Debug(id NodeID, children bool, indent int) string { //nolint:funlen
	node := a.Node(id)
	prefix := strings.Repeat(" ", indent)
	kindName := strings.TrimPrefix(fmt.Sprintf("%T", node.Kind), "ast.")
	attrs := []string{}
	type childAttr struct {
		name string
		ids  []NodeID
	}
	childAttrs := []childAttr{}
	addAttr := func(name, value string) {
		attrs = append(attrs, fmt.Sprintf("%s=%s", name, value))
	}
	addChild := func(name string, ids ...NodeID) {
		childAttrs = append(childAttrs, childAttr{name: name, ids: ids})
	}
	nodeIDKind := func(id NodeID) string {
		kindName := strings.TrimPrefix(fmt.Sprintf("%T", a.Node(id).Kind), "ast.")
		return fmt.Sprintf("%s:%s", id, kindName)
	}
	nodeIDList := func(ids []NodeID) string {
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			parts = append(parts, nodeIDKind(id))
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ","))
	}
	blockExprList := func(ids []NodeID) string {
		if len(ids) <= 10 {
			return nodeIDList(ids)
		}
		parts := make([]string, 0, 10)
		for i := range 8 {
			parts = append(parts, nodeIDKind(ids[i]))
		}
		parts = append(parts, fmt.Sprintf("... %d more ...", len(ids)-9))
		parts = append(parts, nodeIDKind(ids[len(ids)-1]))
		return fmt.Sprintf("[%s]", strings.Join(parts, ","))
	}
	switch kind := node.Kind.(type) {
	case Assign:
		if !children {
			addAttr("lhs", nodeIDKind(kind.LHS))
			addAttr("rhs", nodeIDKind(kind.RHS))
		} else {
			addChild("lhs", kind.LHS)
			addChild("rhs", kind.RHS)
		}
	case Block:
		addAttr("createScope", fmt.Sprintf("%t", kind.CreateScope))
		if !children {
			addAttr("exprs", blockExprList(kind.Exprs))
		} else {
			addChild("exprs", kind.Exprs...)
		}
	case Call:
		if !children {
			addAttr("callee", nodeIDKind(kind.Callee))
			addAttr("args", nodeIDList(kind.Args))
		} else {
			addChild("callee", kind.Callee)
			addChild("args", kind.Args...)
		}
	case Deref:
		if !children {
			addAttr("expr", nodeIDKind(kind.Expr))
		} else {
			addChild("expr", kind.Expr)
		}
	case File:
		if !children {
			addAttr("decls", nodeIDList(kind.Decls))
		} else {
			addChild("decls", kind.Decls...)
		}
	case Fun:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		if !children {
			addAttr("params", nodeIDList(kind.Params))
			addAttr("returnType", nodeIDKind(kind.ReturnType))
			addAttr("block", nodeIDKind(kind.Block))
		} else {
			addChild("params", kind.Params...)
			addChild("returnType", kind.ReturnType)
			addChild("block", kind.Block)
		}
	case FunParam:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
		if !children {
			addAttr("type", nodeIDKind(kind.Type))
		} else {
			addChild("type", kind.Type)
		}
	case Ident:
		addAttr("name", fmt.Sprintf("%q", kind.Name))
	case Int:
		addAttr("value", fmt.Sprintf("%d", kind.Value))
	case String:
		addAttr("value", fmt.Sprintf("%q", kind.Value))
	case Var:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
		if !children {
			addAttr("expr", nodeIDKind(kind.Expr))
		} else {
			addChild("expr", kind.Expr)
		}
	case SimpleType:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
	case Ref:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
	case RefType:
		if !children {
			addAttr("type", nodeIDKind(kind.Type))
		} else {
			addChild("type", kind.Type)
		}
	default:
		panic(base.Errorf("unknown node kind: %T", kind))
	}
	line := fmt.Sprintf("%s%s:%s(%s)", prefix, id, kindName, strings.Join(attrs, ","))
	if !children || len(childAttrs) == 0 {
		return line
	}
	lines := []string{line}
	childPrefix := strings.Repeat(" ", indent+2)
	for _, child := range childAttrs {
		for i, childID := range child.ids {
			name := child.name
			if len(child.ids) > 1 {
				name = fmt.Sprintf("%s[%d]", name, i)
			}
			childDebug := a.Debug(childID, children, indent+2)
			childDebug = strings.TrimPrefix(childDebug, childPrefix)
			lines = append(lines, fmt.Sprintf("%s%s=%s", childPrefix, name, childDebug))
		}
	}
	return strings.Join(lines, "\n")
}

func (a *AST) node(kind Kind, span base.Span) NodeID {
	id := a.nextID_
	node := &Node{ID: id, Span: span, Kind: kind}
	a.nodes[id] = &Node{ID: id, Span: span, Kind: kind}
	a.nextID_++
	if a.onNewNode != nil {
		a.onNewNode(node)
	}
	return id
}
