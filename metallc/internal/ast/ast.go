package ast

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/base"
)

type NodeID uint64

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

type ArrayType struct {
	Elem NodeID
	Len  int64
}

func (ArrayType) isKind() {}

type SliceType struct {
	Elem NodeID
}

func (SliceType) isKind() {}

type NewArray struct {
	Type         NodeID
	DefaultValue *NodeID
}

func (NewArray) isKind() {}

type MakeSlice struct {
	Allocator    NodeID
	Type         NodeID
	Len          NodeID
	DefaultValue *NodeID
}

func (MakeSlice) isKind() {}

type ArrayLiteral struct {
	Elems []NodeID
}

func (ArrayLiteral) isKind() {}

type EmptySlice struct{}

func (EmptySlice) isKind() {}

type Index struct {
	Target NodeID
	Index  NodeID
}

func (Index) isKind() {}

type RefType struct {
	Type NodeID
	Mut  bool
}

func (RefType) isKind() {}

type FunParam struct {
	Name Name
	Type NodeID
}

func (FunParam) isKind() {}

type Fun struct {
	Name       Name
	Params     []NodeID
	ReturnType NodeID
	Block      NodeID
}

func (Fun) isKind() {}

type StructField struct {
	Name Name
	Type NodeID
	Mut  bool
}

func (StructField) isKind() {}

type Struct struct {
	Name   Name
	Fields []NodeID
}

func (Struct) isKind() {}

type FieldAccess struct {
	Target NodeID
	Field  Name
}

func (FieldAccess) isKind() {}

type Int struct {
	Value *big.Int
}

func (Int) isKind() {}

type Bool struct {
	Value bool
}

func (Bool) isKind() {}

type String struct {
	Value string
}

func (String) isKind() {}

type Assign struct {
	LHS NodeID
	RHS NodeID
}

func (Assign) isKind() {}

type BinaryOp string

const (
	BinaryOpAdd BinaryOp = "+"
	BinaryOpSub BinaryOp = "-"
	BinaryOpDiv BinaryOp = "/"
	BinaryOpMod BinaryOp = "%"
	BinaryOpMul BinaryOp = "*"

	BinaryOpEq  BinaryOp = "=="
	BinaryOpNeq BinaryOp = "!="
	BinaryOpLt  BinaryOp = "<"
	BinaryOpLte BinaryOp = "<="
	BinaryOpGt  BinaryOp = ">"
	BinaryOpGte BinaryOp = ">="
	BinaryOpAnd BinaryOp = "and"
	BinaryOpOr  BinaryOp = "or"
)

type Binary struct {
	LHS NodeID
	RHS NodeID
	Op  BinaryOp
}

func (Binary) isKind() {}

type UnaryOp string

const (
	UnaryOpNot UnaryOp = "not"
)

type Unary struct {
	Expr NodeID
	Op   UnaryOp
}

func (Unary) isKind() {}

type Var struct {
	Name Name
	Expr NodeID
	Mut  bool
}

func (Var) isKind() {}

type AllocatorVar struct {
	Name      Name
	Allocator Name
	Args      []NodeID
}

func (AllocatorVar) isKind() {}

type Block struct {
	Exprs       []NodeID
	CreateScope bool
}

type If struct {
	Cond NodeID
	Then NodeID
	Else *NodeID
}

func (If) isKind() {}

type For struct {
	Cond *NodeID
	Body NodeID
}

func (For) isKind() {}

type Return struct {
	Expr NodeID
}

func (Return) isKind() {}

type Break struct{}

func (Break) isKind() {}

type Continue struct{}

func (Continue) isKind() {}

func (Block) isKind() {}

type Call struct {
	Callee NodeID
	Args   []NodeID
}

func (Call) isKind() {}

type StructLiteral struct {
	Target NodeID
	Args   []NodeID
}

func (StructLiteral) isKind() {}

type New struct {
	Allocator NodeID
	Target    NodeID
	Mut       bool
}

func (New) isKind() {}

type Ref struct {
	Name Name
	Mut  bool
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

func (a *AST) NewIf(cond NodeID, then NodeID, else_ *NodeID, span base.Span) NodeID {
	return a.node(If{Cond: cond, Then: then, Else: else_}, span)
}

func (a *AST) NewFor(cond *NodeID, body NodeID, span base.Span) NodeID {
	return a.node(For{Cond: cond, Body: body}, span)
}

func (a *AST) NewBreak(span base.Span) NodeID {
	return a.node(Break{}, span)
}

func (a *AST) NewContinue(span base.Span) NodeID {
	return a.node(Continue{}, span)
}

func (a *AST) NewReturn(expr NodeID, span base.Span) NodeID {
	return a.node(Return{Expr: expr}, span)
}

func (a *AST) NewBool(value bool, span base.Span) NodeID {
	return a.node(Bool{Value: value}, span)
}

func (a *AST) NewBlock(exprs []NodeID, createScope bool, span base.Span) NodeID {
	return a.node(Block{Exprs: exprs, CreateScope: createScope}, span)
}

func (a *AST) NewCall(callee NodeID, args []NodeID, span base.Span) NodeID {
	return a.node(Call{Callee: callee, Args: args}, span)
}

func (a *AST) NewStructLiteral(target NodeID, args []NodeID, span base.Span) NodeID {
	return a.node(StructLiteral{Target: target, Args: args}, span)
}

func (a *AST) NewNew(alloc NodeID, target NodeID, mut bool, span base.Span) NodeID {
	return a.node(New{Allocator: alloc, Target: target, Mut: mut}, span)
}

func (a *AST) NewDeref(expr NodeID, span base.Span) NodeID {
	return a.node(Deref{Expr: expr}, span)
}

func (a *AST) NewBinary(op BinaryOp, lhs NodeID, rhs NodeID, span base.Span) NodeID {
	return a.node(Binary{LHS: lhs, RHS: rhs, Op: op}, span)
}

func (a *AST) NewUnary(op UnaryOp, expr NodeID, span base.Span) NodeID {
	return a.node(Unary{Expr: expr, Op: op}, span)
}

func (a *AST) NewFile(decls []NodeID, span base.Span) NodeID {
	return a.node(File{Decls: decls}, span)
}

func (a *AST) NewFun(
	name Name, params []NodeID, returnType NodeID, block NodeID, span base.Span,
) NodeID {
	return a.node(Fun{Name: name, Params: params, ReturnType: returnType, Block: block}, span)
}

func (a *AST) NewFunParam(name Name, type_ NodeID, span base.Span) NodeID {
	return a.node(FunParam{Name: name, Type: type_}, span)
}

func (a *AST) NewStruct(name Name, fields []NodeID, span base.Span) NodeID {
	return a.node(Struct{Name: name, Fields: fields}, span)
}

func (a *AST) NewStructField(name Name, type_ NodeID, mut bool, span base.Span) NodeID {
	return a.node(StructField{Name: name, Type: type_, Mut: mut}, span)
}

func (a *AST) NewFieldAccess(target NodeID, field Name, span base.Span) NodeID {
	return a.node(FieldAccess{Target: target, Field: field}, span)
}

func (a *AST) NewIdent(name string, span base.Span) NodeID {
	return a.node(Ident{Name: name}, span)
}

func (a *AST) NewAllocatorVar(name Name, allocator Name, args []NodeID, span base.Span) NodeID {
	return a.node(AllocatorVar{Name: name, Allocator: allocator, Args: args}, span)
}

func (a *AST) NewInt(value *big.Int, span base.Span) NodeID {
	return a.node(Int{Value: value}, span)
}

func (a *AST) NewRef(name Name, mut bool, span base.Span) NodeID {
	return a.node(Ref{Name: name, Mut: mut}, span)
}

func (a *AST) NewString(value string, span base.Span) NodeID {
	return a.node(String{Value: value}, span)
}

func (a *AST) NewSimpleType(name Name, span base.Span) NodeID {
	return a.node(SimpleType{Name: name}, span)
}

func (a *AST) NewArrayType(elemType NodeID, len_ int64, span base.Span) NodeID {
	return a.node(ArrayType{elemType, len_}, span)
}

func (a *AST) NewSliceType(elemType NodeID, span base.Span) NodeID {
	return a.node(SliceType{Elem: elemType}, span)
}

func (a *AST) NewNewArray(typ NodeID, defaultValue *NodeID, span base.Span) NodeID {
	return a.node(NewArray{Type: typ, DefaultValue: defaultValue}, span)
}

func (a *AST) NewMakeSlice(alloc NodeID, typ NodeID, len_ NodeID, defaultValue *NodeID, span base.Span) NodeID {
	return a.node(MakeSlice{Allocator: alloc, Type: typ, Len: len_, DefaultValue: defaultValue}, span)
}

func (a *AST) NewArrayLiteral(elems []NodeID, span base.Span) NodeID {
	return a.node(ArrayLiteral{Elems: elems}, span)
}

func (a *AST) NewEmptySlice(span base.Span) NodeID {
	return a.node(EmptySlice{}, span)
}

func (a *AST) NewIndex(target NodeID, index NodeID, span base.Span) NodeID {
	return a.node(Index{Target: target, Index: index}, span)
}

func (a *AST) NewRefType(type_ NodeID, mut bool, span base.Span) NodeID {
	return a.node(RefType{Type: type_, Mut: mut}, span)
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

func (a *AST) Walk(id NodeID, f func(NodeID)) { //nolint:funlen
	node := a.Node(id)
	switch kind := node.Kind.(type) {
	case Assign:
		f(kind.LHS)
		f(kind.RHS)
	case Binary:
		f(kind.LHS)
		f(kind.RHS)
	case Unary:
		f(kind.Expr)
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
	case If:
		f(kind.Cond)
		f(kind.Then)
		if kind.Else != nil {
			f(*kind.Else)
		}
	case For:
		if kind.Cond != nil {
			f(*kind.Cond)
		}
		f(kind.Body)
	case Fun:
		for i := range len(kind.Params) {
			f(kind.Params[i])
		}
		f(kind.ReturnType)
		f(kind.Block)
	case FunParam:
		f(kind.Type)
	case Struct:
		for i := range len(kind.Fields) {
			f(kind.Fields[i])
		}
	case StructField:
		f(kind.Type)
	case FieldAccess:
		f(kind.Target)
	case StructLiteral:
		f(kind.Target)
		for i := range len(kind.Args) {
			f(kind.Args[i])
		}
	case New:
		f(kind.Allocator)
		f(kind.Target)
	case AllocatorVar:
		for i := range len(kind.Args) {
			f(kind.Args[i])
		}
	case ArrayType:
		f(kind.Elem)
	case SliceType:
		f(kind.Elem)
	case NewArray:
		f(kind.Type)
		if kind.DefaultValue != nil {
			f(*kind.DefaultValue)
		}
	case MakeSlice:
		f(kind.Allocator)
		f(kind.Type)
		f(kind.Len)
		if kind.DefaultValue != nil {
			f(*kind.DefaultValue)
		}
	case ArrayLiteral:
		for i := range len(kind.Elems) {
			f(kind.Elems[i])
		}
	case EmptySlice:
	case Index:
		f(kind.Target)
		f(kind.Index)
	case Return:
		f(kind.Expr)
	case Break:
	case Continue:
	case Ident:
	case Int:
	case Bool:
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
	case Unary:
		addAttr("op", fmt.Sprint(kind.Op))
		if !children {
			addAttr("expr", nodeIDKind(kind.Expr))
		} else {
			addChild("expr", kind.Expr)
		}
	case Binary:
		addAttr("op", fmt.Sprint(kind.Op))
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
	case If:
		if !children {
			addAttr("cond", nodeIDKind(kind.Cond))
			addAttr("then", nodeIDKind(kind.Then))
			if kind.Else != nil {
				addAttr("else", nodeIDKind(*kind.Else))
			}
		} else {
			addChild("cond", kind.Cond)
			addChild("then", kind.Then)
			if kind.Else != nil {
				addChild("else", *kind.Else)
			}
		}
	case For:
		if !children {
			if kind.Cond != nil {
				addAttr("cond", nodeIDKind(*kind.Cond))
			}
			addAttr("body", nodeIDKind(kind.Body))
		} else {
			if kind.Cond != nil {
				addChild("cond", *kind.Cond)
			}
			addChild("body", kind.Body)
		}
	case Return:
		if !children {
			addAttr("expr", nodeIDKind(kind.Expr))
		} else {
			addChild("expr", kind.Expr)
		}
	case Break, Continue:
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
		if !children {
			addAttr("type", nodeIDKind(kind.Type))
		} else {
			addChild("type", kind.Type)
		}
	case Struct:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		if !children {
			addAttr("fields", nodeIDList(kind.Fields))
		} else {
			addChild("fields", kind.Fields...)
		}
	case StructField:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
		if !children {
			addAttr("type", nodeIDKind(kind.Type))
		} else {
			addChild("type", kind.Type)
		}
	case StructLiteral:
		if !children {
			addAttr("target", nodeIDKind(kind.Target))
			addAttr("args", nodeIDList(kind.Args))
		} else {
			addChild("target", kind.Target)
			addChild("args", kind.Args...)
		}
	case New:
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
		if !children {
			addAttr("allocator", nodeIDKind(kind.Allocator))
			addAttr("target", nodeIDKind(kind.Target))
		} else {
			addChild("allocator", kind.Allocator)
			addChild("target", kind.Target)
		}
	case AllocatorVar:
		addAttr("name", kind.Name.Name)
		addAttr("allocator", kind.Allocator.Name)
		if !children {
			addAttr("args", nodeIDList(kind.Args))
		} else {
			addChild("args", kind.Args...)
		}
	case FieldAccess:
		if !children {
			addAttr("target", nodeIDKind(kind.Target))
			addAttr("field", kind.Field.Name)
		} else {
			addChild("target", kind.Target)
			addAttr("field", kind.Field.Name)
		}
	case Ident:
		addAttr("name", fmt.Sprintf("%q", kind.Name))
	case Int:
		addAttr("value", kind.Value.String())
	case Bool:
		addAttr("value", fmt.Sprintf("%t", kind.Value))
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
	case ArrayType:
		addAttr("len", fmt.Sprintf("%d", kind.Len))
		if !children {
			addAttr("type", nodeIDKind(kind.Elem))
		} else {
			addChild("type", kind.Elem)
		}
	case SliceType:
		if !children {
			addAttr("type", nodeIDKind(kind.Elem))
		} else {
			addChild("type", kind.Elem)
		}
	case NewArray:
		if !children {
			addAttr("type", nodeIDKind(kind.Type))
			if kind.DefaultValue != nil {
				addAttr("default", nodeIDKind(*kind.DefaultValue))
			}
		} else {
			addChild("type", kind.Type)
			if kind.DefaultValue != nil {
				addChild("default", *kind.DefaultValue)
			}
		}
	case MakeSlice:
		addAttr("len", nodeIDKind(kind.Len))
		if !children {
			addAttr("allocator", nodeIDKind(kind.Allocator))
			addAttr("type", nodeIDKind(kind.Type))
			if kind.DefaultValue != nil {
				addAttr("default", nodeIDKind(*kind.DefaultValue))
			}
		} else {
			addChild("allocator", kind.Allocator)
			addChild("type", kind.Type)
			if kind.DefaultValue != nil {
				addChild("default", *kind.DefaultValue)
			}
		}
	case ArrayLiteral:
		addAttr("len", fmt.Sprintf("%d", len(kind.Elems)))
		if len(kind.Elems) > 0 {
			if !children {
				addAttr("first", nodeIDKind(kind.Elems[0]))
			} else {
				addChild("first", kind.Elems[0])
			}
		}
	case EmptySlice:
	case Index:
		if !children {
			addAttr("target", nodeIDKind(kind.Target))
			addAttr("index", nodeIDKind(kind.Index))
		} else {
			addChild("target", kind.Target)
			addChild("index", kind.Index)
		}
	case SimpleType:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
	case Ref:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
	case RefType:
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
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
