package ast

import (
	"fmt"

	"github.com/flunderpero/metall/metallc/internal/base"
)

type NodeID int

func (id NodeID) String() string {
	return fmt.Sprintf("node_%d", id)
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
	Exprs []NodeID
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

func (a *AST) NewBlock(exprs []NodeID, span base.Span) NodeID {
	return a.node(Block{Exprs: exprs}, span)
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
