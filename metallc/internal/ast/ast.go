package ast

import (
	"fmt"
	"maps"
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
	Name     string
	TypeArgs []NodeID
}

func (Ident) isKind() {}

type Name struct {
	Name string
	Span base.Span
}

type Module struct {
	FileName string
	Name     string
	Main     bool
	Imports  []NodeID
	Decls    []NodeID
}

func (Module) isKind() {}

type Import struct {
	Alias    *Name
	Segments []string
}

func (Import) isKind() {}

type Path struct {
	Segments []string
	TypeArgs []NodeID
}

func (Path) isKind() {}

type SimpleType struct {
	Name     Name
	TypeArgs []NodeID
}

func (SimpleType) isKind() {}

type ArrayType struct {
	Elem NodeID
	Len  int64
}

func (ArrayType) isKind() {}

type SliceType struct {
	Elem NodeID
	Mut  bool
}

func (SliceType) isKind() {}

type FunType struct {
	Params     []NodeID
	ReturnType NodeID
}

func (FunType) isKind() {}

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

type SubSlice struct {
	Target NodeID
	Range  NodeID
}

func (SubSlice) isKind() {}

type Range struct {
	Lo        *NodeID
	Hi        *NodeID
	Inclusive bool
}

func (Range) isKind() {}

type RefType struct {
	Type NodeID
	Mut  bool
}

func (RefType) isKind() {}

type FunParam struct {
	Name    Name
	Type    NodeID
	Default *NodeID
}

func (FunParam) isKind() {}

type FunDecl struct {
	Name       Name
	TypeParams []NodeID
	Params     []NodeID
	ReturnType NodeID
}

func (FunDecl) isKind() {}

type Fun struct {
	FunDecl
	Block  NodeID
	Extern bool
}

func (Fun) isKind() {}

type StructField struct {
	Name Name
	Type NodeID
	Mut  bool
}

func (StructField) isKind() {}

type Struct struct {
	Name       Name
	TypeParams []NodeID
	Fields     []NodeID
	Extern     bool
}

func (Struct) isKind() {}

type TypeParam struct {
	Name       Name
	Constraint *NodeID
	Default    *NodeID
}

func (TypeParam) isKind() {}

type Shape struct {
	Name       Name
	TypeParams []NodeID
	Fields     []NodeID // StructField nodes
	Funs       []NodeID // FunDecl nodes
}

func (Shape) isKind() {}

type Union struct {
	Name       Name
	TypeParams []NodeID
	Variants   []NodeID // Type nodes (SimpleType, RefType, etc.)
}

func (Union) isKind() {}

type FieldAccess struct {
	Target   NodeID
	Field    Name
	TypeArgs []NodeID
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

type RuneLiteral struct {
	Value uint32
}

func (RuneLiteral) isKind() {}

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

	BinaryOpBitAnd BinaryOp = "&"
	BinaryOpBitOr  BinaryOp = "|"
	BinaryOpBitXor BinaryOp = "^"
	BinaryOpShl    BinaryOp = "<<"
	BinaryOpShr    BinaryOp = ">>"
)

type Binary struct {
	LHS NodeID
	RHS NodeID
	Op  BinaryOp
}

func (Binary) isKind() {}

type UnaryOp string

const (
	UnaryOpNot    UnaryOp = "not"
	UnaryOpBitNot UnaryOp = "~"
)

type Unary struct {
	Expr NodeID
	Op   UnaryOp
}

func (Unary) isKind() {}

type Var struct {
	Name Name
	Type *NodeID
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
	Exprs []NodeID
}

type If struct {
	Cond NodeID
	Then NodeID
	Else *NodeID
}

func (If) isKind() {}

type For struct {
	Binding *Name
	Cond    *NodeID
	Body    NodeID
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

type Match struct {
	Expr NodeID
	Arms []MatchArm
	Else *MatchElse // nil if no else arm
	Try  bool       // true if desugared from `try`
}

func (Match) isKind() {}

type MatchArm struct {
	Pattern NodeID  // SimpleType (variant)
	Binding *Name   // optional binding (nil if absent)
	Guard   *NodeID // optional guard condition (nil if absent)
	Body    NodeID  // Block
}

type MatchElse struct {
	Binding *Name  // optional binding (nil if absent)
	Body    NodeID // Block
}

// TryPattern is used as the pattern in a match arm desugared from `try` without `is`.
// It signals the type checker to use the first variant of the union.
type TryPattern struct{}

func (TryPattern) isKind() {}

type Call struct {
	Callee NodeID
	Args   []NodeID
}

func (Call) isKind() {}

type TypeConstruction struct {
	Target NodeID
	Args   []NodeID
}

func (TypeConstruction) isKind() {}

type Ref struct {
	Target NodeID
	Mut    bool
}

func (Ref) isKind() {}

type Deref struct {
	Expr NodeID
}

func (Deref) isKind() {}

type AST struct {
	Roots     []NodeID
	nodes     map[NodeID]*Node
	firstID   NodeID
	nextID_   NodeID
	onNewNode func(*Node)
}

func NewAST(firstNodeID NodeID) *AST {
	return &AST{firstID: firstNodeID, nextID_: firstNodeID, nodes: make(map[NodeID]*Node), Roots: nil, onNewNode: nil}
}

func (a *AST) Merge(other *AST) (*AST, error) {
	if other.firstID >= a.firstID && other.firstID < a.nextID_ {
		return nil, base.Errorf(
			"cannot merge: other firstID %d overlaps with [%d, %d)",
			other.firstID,
			a.firstID,
			a.nextID_,
		)
	}
	if a.firstID >= other.firstID && a.firstID < other.nextID_ {
		return nil, base.Errorf(
			"cannot merge: firstID %d overlaps with [%d, %d)",
			a.firstID,
			other.firstID,
			other.nextID_,
		)
	}
	nodes := make(map[NodeID]*Node, len(a.nodes)+len(other.nodes))
	maps.Copy(nodes, a.nodes)
	maps.Copy(nodes, other.nodes)
	roots := append([]NodeID{}, a.Roots...)
	roots = append(roots, other.Roots...)
	firstID := min(a.firstID, other.firstID)
	nextID_ := max(a.nextID_, other.nextID_)
	return &AST{firstID: firstID, nextID_: nextID_, nodes: nodes, Roots: roots, onNewNode: nil}, nil
}

func (a *AST) NewAssign(lhs NodeID, value NodeID, span base.Span) NodeID {
	return a.node(Assign{LHS: lhs, RHS: value}, span)
}

func (a *AST) NewMatch(expr NodeID, arms []MatchArm, else_ *MatchElse, span base.Span) NodeID {
	return a.node(Match{Expr: expr, Arms: arms, Else: else_, Try: false}, span)
}

func (a *AST) NewTryPattern(span base.Span) NodeID {
	return a.node(TryPattern{}, span)
}

func (a *AST) NewIf(cond NodeID, then NodeID, else_ *NodeID, span base.Span) NodeID {
	return a.node(If{Cond: cond, Then: then, Else: else_}, span)
}

func (a *AST) NewFor(binding *Name, cond *NodeID, body NodeID, span base.Span) NodeID {
	return a.node(For{Binding: binding, Cond: cond, Body: body}, span)
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

func (a *AST) NewBlock(exprs []NodeID, span base.Span) NodeID {
	return a.node(Block{Exprs: exprs}, span)
}

func (a *AST) NewCall(callee NodeID, args []NodeID, span base.Span) NodeID {
	return a.node(Call{Callee: callee, Args: args}, span)
}

func (a *AST) NewTypeConstruction(target NodeID, args []NodeID, span base.Span) NodeID {
	return a.node(TypeConstruction{Target: target, Args: args}, span)
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

func (a *AST) NewModule(
	fileName string,
	name string,
	main bool,
	imports []NodeID,
	decls []NodeID,
	span base.Span,
) NodeID {
	node := a.node(Module{FileName: fileName, Name: name, Main: main, Imports: imports, Decls: decls}, span)
	a.Roots = append(a.Roots, node)
	return node
}

func (a *AST) NewImport(alias *Name, segments []string, span base.Span) NodeID {
	return a.node(Import{Alias: alias, Segments: segments}, span)
}

func (a *AST) NewPath(segments []string, typeArgs []NodeID, span base.Span) NodeID {
	return a.node(Path{Segments: segments, TypeArgs: typeArgs}, span)
}

func (a *AST) NewFunDecl(
	name Name, typeParams []NodeID, params []NodeID, returnType NodeID, span base.Span,
) NodeID {
	return a.node(FunDecl{Name: name, TypeParams: typeParams, Params: params, ReturnType: returnType}, span)
}

func (a *AST) NewFun(
	name Name, typeParams []NodeID, params []NodeID, returnType NodeID, block NodeID, span base.Span,
) NodeID {
	return a.node(
		Fun{
			FunDecl: FunDecl{Name: name, TypeParams: typeParams, Params: params, ReturnType: returnType},
			Extern:  false,
			Block:   block,
		},
		span,
	)
}

func (a *AST) NewFunParam(name Name, type_ NodeID, defaultVal *NodeID, span base.Span) NodeID {
	return a.node(FunParam{Name: name, Type: type_, Default: defaultVal}, span)
}

func (a *AST) NewStruct(name Name, typeParams []NodeID, fields []NodeID, span base.Span) NodeID {
	return a.node(Struct{Name: name, TypeParams: typeParams, Fields: fields, Extern: false}, span)
}

func (a *AST) NewStructField(name Name, type_ NodeID, mut bool, span base.Span) NodeID {
	return a.node(StructField{Name: name, Type: type_, Mut: mut}, span)
}

func (a *AST) NewTypeParam(name Name, constraint *NodeID, defaultType *NodeID, span base.Span) NodeID {
	return a.node(TypeParam{Name: name, Constraint: constraint, Default: defaultType}, span)
}

func (a *AST) NewShape(name Name, typeParams []NodeID, fields []NodeID, funs []NodeID, span base.Span) NodeID {
	return a.node(Shape{Name: name, TypeParams: typeParams, Fields: fields, Funs: funs}, span)
}

func (a *AST) NewUnion(name Name, typeParams []NodeID, variants []NodeID, span base.Span) NodeID {
	return a.node(Union{Name: name, TypeParams: typeParams, Variants: variants}, span)
}

func (a *AST) NewFieldAccess(target NodeID, field Name, typeArgs []NodeID, span base.Span) NodeID {
	return a.node(FieldAccess{Target: target, Field: field, TypeArgs: typeArgs}, span)
}

func (a *AST) NewIdent(name string, typeArgs []NodeID, span base.Span) NodeID {
	return a.node(Ident{Name: name, TypeArgs: typeArgs}, span)
}

func (a *AST) NewAllocatorVar(name Name, allocator Name, args []NodeID, span base.Span) NodeID {
	return a.node(AllocatorVar{Name: name, Allocator: allocator, Args: args}, span)
}

func (a *AST) NewInt(value *big.Int, span base.Span) NodeID {
	return a.node(Int{Value: value}, span)
}

func (a *AST) NewRef(target NodeID, mut bool, span base.Span) NodeID {
	return a.node(Ref{Target: target, Mut: mut}, span)
}

func (a *AST) NewString(value string, span base.Span) NodeID {
	return a.node(String{Value: value}, span)
}

func (a *AST) NewRuneLiteral(value uint32, span base.Span) NodeID {
	return a.node(RuneLiteral{Value: value}, span)
}

func (a *AST) NewSimpleType(name Name, typeArgs []NodeID, span base.Span) NodeID {
	return a.node(SimpleType{Name: name, TypeArgs: typeArgs}, span)
}

func (a *AST) NewArrayType(elemType NodeID, len_ int64, span base.Span) NodeID {
	return a.node(ArrayType{elemType, len_}, span)
}

func (a *AST) NewSliceType(elemType NodeID, mut bool, span base.Span) NodeID {
	return a.node(SliceType{Elem: elemType, Mut: mut}, span)
}

func (a *AST) NewFunType(params []NodeID, returnType NodeID, span base.Span) NodeID {
	return a.node(FunType{Params: params, ReturnType: returnType}, span)
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

func (a *AST) NewSubSlice(target NodeID, range_ NodeID, span base.Span) NodeID {
	return a.node(SubSlice{Target: target, Range: range_}, span)
}

func (a *AST) NewRange(lo *NodeID, hi *NodeID, inclusive bool, span base.Span) NodeID {
	return a.node(Range{Lo: lo, Hi: hi, Inclusive: inclusive}, span)
}

func (a *AST) NewRefType(type_ NodeID, mut bool, span base.Span) NodeID {
	return a.node(RefType{Type: type_, Mut: mut}, span)
}

func (a *AST) NewVar(name Name, type_ *NodeID, expr NodeID, mut bool, span base.Span) NodeID {
	return a.node(Var{Name: name, Type: type_, Expr: expr, Mut: mut}, span)
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
	case Import:
	case Path:
		for i := range len(kind.TypeArgs) {
			f(kind.TypeArgs[i])
		}
	case Module:
		for i := range len(kind.Imports) {
			f(kind.Imports[i])
		}
		for i := range len(kind.Decls) {
			f(kind.Decls[i])
		}
	case Match:
		f(kind.Expr)
		for _, arm := range kind.Arms {
			f(arm.Pattern)
			if arm.Guard != nil {
				f(*arm.Guard)
			}
			f(arm.Body)
		}
		if kind.Else != nil {
			f(kind.Else.Body)
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

	case FunDecl:
		for i := range len(kind.TypeParams) {
			f(kind.TypeParams[i])
		}
		for i := range len(kind.Params) {
			f(kind.Params[i])
		}
		f(kind.ReturnType)
	case Fun:
		for i := range len(kind.TypeParams) {
			f(kind.TypeParams[i])
		}
		for i := range len(kind.Params) {
			f(kind.Params[i])
		}
		f(kind.ReturnType)
		f(kind.Block)
	case FunType:
		for i := range len(kind.Params) {
			f(kind.Params[i])
		}
		f(kind.ReturnType)
	case FunParam:
		f(kind.Type)
		if kind.Default != nil {
			f(*kind.Default)
		}
	case Struct:
		for i := range len(kind.TypeParams) {
			f(kind.TypeParams[i])
		}
		for i := range len(kind.Fields) {
			f(kind.Fields[i])
		}
	case Shape:
		for i := range len(kind.TypeParams) {
			f(kind.TypeParams[i])
		}
		for i := range len(kind.Fields) {
			f(kind.Fields[i])
		}
		for i := range len(kind.Funs) {
			f(kind.Funs[i])
		}
	case Union:
		for i := range len(kind.TypeParams) {
			f(kind.TypeParams[i])
		}
		for i := range len(kind.Variants) {
			f(kind.Variants[i])
		}
	case TypeParam:
		if kind.Constraint != nil {
			f(*kind.Constraint)
		}
		if kind.Default != nil {
			f(*kind.Default)
		}
	case StructField:
		f(kind.Type)
	case FieldAccess:
		f(kind.Target)
		for _, typeArg := range kind.TypeArgs {
			f(typeArg)
		}
	case TypeConstruction:
		f(kind.Target)
		for i := range len(kind.Args) {
			f(kind.Args[i])
		}
	case AllocatorVar:
		for i := range len(kind.Args) {
			f(kind.Args[i])
		}
	case ArrayType:
		f(kind.Elem)
	case SliceType:
		f(kind.Elem)
	case ArrayLiteral:
		for i := range len(kind.Elems) {
			f(kind.Elems[i])
		}
	case EmptySlice:
	case Index:
		f(kind.Target)
		f(kind.Index)
	case SubSlice:
		f(kind.Target)
		f(kind.Range)
	case Range:
		if kind.Lo != nil {
			f(*kind.Lo)
		}
		if kind.Hi != nil {
			f(*kind.Hi)
		}
	case Return:
		f(kind.Expr)
	case Break:
	case Continue:
	case Ident:
		for i := range len(kind.TypeArgs) {
			f(kind.TypeArgs[i])
		}
	case Int:
	case Bool:
	case String:
	case RuneLiteral:
	case Var:
		if kind.Type != nil {
			f(*kind.Type)
		}
		f(kind.Expr)
	case SimpleType:
		for i := range len(kind.TypeArgs) {
			f(kind.TypeArgs[i])
		}
	case Ref:
		f(kind.Target)
	case RefType:
		f(kind.Type)
	case TryPattern:
	default:
		panic(base.Errorf("unknown node kind: %T", kind))
	}
}

func (a *AST) Debug(id NodeID, children bool, indent int, skipIDs ...bool) string { //nolint:funlen
	skipNodeIDs := len(skipIDs) > 0 && skipIDs[0]
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
	case Match:
		addAttr("arms", fmt.Sprintf("%d", len(kind.Arms)))
		if !children {
			addAttr("expr", nodeIDKind(kind.Expr))
		} else {
			addChild("expr", kind.Expr)
			for i, arm := range kind.Arms {
				addAttr(fmt.Sprintf("arm[%d].pattern", i), nodeIDKind(arm.Pattern))
				if arm.Binding != nil {
					addAttr(fmt.Sprintf("arm[%d].binding", i), arm.Binding.Name)
				}
				if arm.Guard != nil {
					addChild(fmt.Sprintf("arm[%d].guard", i), *arm.Guard)
				}
				addChild(fmt.Sprintf("arm[%d].body", i), arm.Body)
			}
			if kind.Else != nil {
				if kind.Else.Binding != nil {
					addAttr("else.binding", kind.Else.Binding.Name)
				}
				addChild("else.body", kind.Else.Body)
			}
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
		if kind.Binding != nil {
			addAttr("binding", kind.Binding.Name)
		}
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
	case Import:
		if kind.Alias != nil {
			addAttr("alias", fmt.Sprintf("%q", kind.Alias.Name))
		}
		addAttr("path", strings.Join(kind.Segments, "::"))
	case Path:
		addAttr("segments", strings.Join(kind.Segments, "::"))
		if children {
			for _, arg := range kind.TypeArgs {
				addChild("typeArg", arg)
			}
		}
	case Module:
		addAttr("fileName", fmt.Sprintf("%q", kind.FileName))
		addAttr("name", fmt.Sprintf("%q", kind.Name))
		addAttr("main", fmt.Sprintf("%t", kind.Main))
		if !children {
			if len(kind.Imports) > 0 {
				addAttr("imports", nodeIDList(kind.Imports))
			}
			addAttr("decls", nodeIDList(kind.Decls))
		} else {
			if len(kind.Imports) > 0 {
				addChild("imports", kind.Imports...)
			}
			addChild("decls", kind.Decls...)
		}
	case FunDecl:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		if !children {
			if len(kind.TypeParams) > 0 {
				addAttr("typeParams", nodeIDList(kind.TypeParams))
			}
			addAttr("params", nodeIDList(kind.Params))
			addAttr("returnType", nodeIDKind(kind.ReturnType))
		} else {
			if len(kind.TypeParams) > 0 {
				addChild("typeParams", kind.TypeParams...)
			}
			addChild("params", kind.Params...)
			addChild("returnType", kind.ReturnType)
		}
	case Fun:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		if !children {
			if len(kind.TypeParams) > 0 {
				addAttr("typeParams", nodeIDList(kind.TypeParams))
			}
			addAttr("params", nodeIDList(kind.Params))
			addAttr("returnType", nodeIDKind(kind.ReturnType))
			addAttr("block", nodeIDKind(kind.Block))
		} else {
			if len(kind.TypeParams) > 0 {
				addChild("typeParams", kind.TypeParams...)
			}
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
		if kind.Default != nil {
			if !children {
				addAttr("default", nodeIDKind(*kind.Default))
			} else {
				addChild("default", *kind.Default)
			}
		}
	case FunType:
		if !children {
			addAttr("params", nodeIDList(kind.Params))
			addAttr("returnType", nodeIDKind(kind.ReturnType))
		} else {
			addChild("params", kind.Params...)
			addChild("returnType", kind.ReturnType)
		}
	case Struct:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		if !children {
			if len(kind.TypeParams) > 0 {
				addAttr("typeParams", nodeIDList(kind.TypeParams))
			}
			addAttr("fields", nodeIDList(kind.Fields))
		} else {
			if len(kind.TypeParams) > 0 {
				addChild("typeParams", kind.TypeParams...)
			}
			addChild("fields", kind.Fields...)
		}
	case Shape:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		if !children {
			if len(kind.TypeParams) > 0 {
				addAttr("typeParams", nodeIDList(kind.TypeParams))
			}
			addAttr("fields", nodeIDList(kind.Fields))
			addAttr("funs", nodeIDList(kind.Funs))
		} else {
			if len(kind.TypeParams) > 0 {
				addChild("typeParams", kind.TypeParams...)
			}
			addChild("fields", kind.Fields...)
			addChild("funs", kind.Funs...)
		}
	case Union:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		if !children {
			if len(kind.TypeParams) > 0 {
				addAttr("typeParams", nodeIDList(kind.TypeParams))
			}
			addAttr("variants", nodeIDList(kind.Variants))
		} else {
			if len(kind.TypeParams) > 0 {
				addChild("typeParams", kind.TypeParams...)
			}
			addChild("variants", kind.Variants...)
		}
	case TypeParam:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		if kind.Constraint != nil {
			if !children {
				addAttr("constraint", nodeIDKind(*kind.Constraint))
			} else {
				addChild("constraint", *kind.Constraint)
			}
		}
		if kind.Default != nil {
			if !children {
				addAttr("default", nodeIDKind(*kind.Default))
			} else {
				addChild("default", *kind.Default)
			}
		}
	case StructField:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
		if !children {
			addAttr("type", nodeIDKind(kind.Type))
		} else {
			addChild("type", kind.Type)
		}
	case TypeConstruction:
		if !children {
			addAttr("target", nodeIDKind(kind.Target))
			addAttr("args", nodeIDList(kind.Args))
		} else {
			addChild("target", kind.Target)
			addChild("args", kind.Args...)
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
		if len(kind.TypeArgs) > 0 {
			if !children {
				addAttr("typeArgs", nodeIDList(kind.TypeArgs))
			} else {
				addChild("typeArgs", kind.TypeArgs...)
			}
		}
	case Int:
		addAttr("value", kind.Value.String())
	case Bool:
		addAttr("value", fmt.Sprintf("%t", kind.Value))
	case String:
		addAttr("value", fmt.Sprintf("%q", kind.Value))
	case RuneLiteral:
		addAttr("value", fmt.Sprintf("'%c'(%d)", rune(kind.Value), kind.Value))
	case Var:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
		if kind.Type != nil {
			if !children {
				addAttr("type", nodeIDKind(*kind.Type))
			} else {
				addChild("type", *kind.Type)
			}
		}
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
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
		if !children {
			addAttr("type", nodeIDKind(kind.Elem))
		} else {
			addChild("type", kind.Elem)
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
	case SubSlice:
		if !children {
			addAttr("target", nodeIDKind(kind.Target))
			addAttr("range", nodeIDKind(kind.Range))
		} else {
			addChild("target", kind.Target)
			addChild("range", kind.Range)
		}
	case Range:
		addAttr("inclusive", fmt.Sprintf("%t", kind.Inclusive))
		if !children {
			if kind.Lo != nil {
				addAttr("lo", nodeIDKind(*kind.Lo))
			}
			if kind.Hi != nil {
				addAttr("hi", nodeIDKind(*kind.Hi))
			}
		} else {
			if kind.Lo != nil {
				addChild("lo", *kind.Lo)
			}
			if kind.Hi != nil {
				addChild("hi", *kind.Hi)
			}
		}
	case SimpleType:
		addAttr("name", fmt.Sprintf("%q", kind.Name.Name))
		if len(kind.TypeArgs) > 0 {
			if !children {
				addAttr("typeArgs", nodeIDList(kind.TypeArgs))
			} else {
				addChild("typeArgs", kind.TypeArgs...)
			}
		}
	case TryPattern:
	case Ref:
		addAttr("mut", fmt.Sprintf("%t", kind.Mut))
		if !children {
			addAttr("target", nodeIDKind(kind.Target))
		} else {
			addChild("target", kind.Target)
		}
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
	var line string
	if skipNodeIDs {
		line = fmt.Sprintf("%s%s(%s)", prefix, kindName, strings.Join(attrs, ","))
	} else {
		line = fmt.Sprintf("%s%s:%s(%s)", prefix, id, kindName, strings.Join(attrs, ","))
	}
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
			childDebug := a.Debug(childID, children, indent+2, skipNodeIDs)
			childDebug = strings.TrimPrefix(childDebug, childPrefix)
			lines = append(lines, fmt.Sprintf("%s%s=%s", childPrefix, name, childDebug))
		}
	}
	return strings.Join(lines, "\n")
}

func (a *AST) BlockReturns(blockID NodeID) bool {
	return a.blockBreaksControlFlow(blockID, true)
}

func (a *AST) BlockBreaksControlFlow(blockID NodeID, checkReturnOnly bool) bool {
	return a.blockBreaksControlFlow(blockID, checkReturnOnly)
}

func (a *AST) blockBreaksControlFlow(blockID NodeID, checkForReturnOnly bool) bool {
	block := base.Cast[Block](a.Node(blockID).Kind)
	if len(block.Exprs) == 0 {
		return false
	}
	lastExpr := a.Node(block.Exprs[len(block.Exprs)-1])
	switch lastExpr.Kind.(type) {
	case Break, Continue:
		return !checkForReturnOnly
	case Return:
		return true
	default:
		if ifNode, ok := lastExpr.Kind.(If); ok {
			return ifNode.Else != nil && a.blockBreaksControlFlow(ifNode.Then, checkForReturnOnly) &&
				a.blockBreaksControlFlow(*ifNode.Else, checkForReturnOnly)
		}
		if matchNode, ok := lastExpr.Kind.(Match); ok {
			for _, arm := range matchNode.Arms {
				if !a.blockBreaksControlFlow(arm.Body, checkForReturnOnly) {
					return false
				}
			}
			if matchNode.Else != nil && !a.blockBreaksControlFlow(matchNode.Else.Body, checkForReturnOnly) {
				return false
			}
			return len(matchNode.Arms) > 0
		}
		return false
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
