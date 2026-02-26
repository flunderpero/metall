package types

import (
	"fmt"
	"slices"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type TaintID int

func (t TaintID) String() string { return fmt.Sprintf("a%d", t) }

type TaintSet []TaintID

func (t TaintSet) Merge(other TaintSet) TaintSet {
	for _, id := range other {
		if !slices.Contains(t, id) {
			t = append(t, id)
		}
	}
	return t
}

func (t TaintSet) ContainsAny(other TaintSet) bool {
	for _, id := range t {
		if slices.Contains(other, id) {
			return true
		}
	}
	return false
}

func (t TaintSet) Without(exclude TaintSet) TaintSet {
	var result TaintSet
	for _, id := range t {
		if !slices.Contains(exclude, id) {
			result = append(result, id)
		}
	}
	return result
}

// PointsTo identifies a binding that a reference points to.
type PointsTo struct {
	ScopeID ScopeID
	Name    string
}

type PointsToSet []PointsTo

func (r PointsToSet) Merge(other PointsToSet) PointsToSet {
	for _, target := range other {
		if !slices.Contains(r, target) {
			r = append(r, target)
		}
	}
	return r
}

// Flow is the abstract value propagated through the dataflow analysis.
// It bundles which taints a node carries and which bindings its references
// point to.
type Flow struct {
	Taints   TaintSet
	PointsTo PointsToSet
}

func (f Flow) Merge(o Flow) Flow {
	return Flow{f.Taints.Merge(o.Taints), f.PointsTo.Merge(o.PointsTo)}
}

// BindingTaint is the abstract state of a single named binding.
type BindingTaint struct {
	DiagNode   ast.NodeID // Node to blame in escape diagnostics.
	AllocTaint TaintID    // Taint identifying the allocator that owns this binding's storage.
	Flow       Flow       // Taints and points-to sets carried by references in this binding.
}

// TaintScope tracks all binding taints and locally-born taints for one
// lexical scope.
type TaintScope struct {
	Bindings    map[string]*BindingTaint
	LocalTaints TaintSet
	AllocTaint  TaintID
}

func newTaintScope(allocTaint TaintID) *TaintScope {
	return &TaintScope{
		Bindings:    map[string]*BindingTaint{},
		LocalTaints: TaintSet{allocTaint},
		AllocTaint:  allocTaint,
	}
}

// FunEffect records which parameter taints flow into the return value.
type FunEffect struct {
	ReturnParamTaints map[TaintID]int
}

type LifetimeCheck struct {
	Diagnostics base.Diagnostics
	Debug       base.Debug
	e           *Engine
	nextTaintID TaintID
	scopes      map[ScopeID]*TaintScope
	flows       map[ast.NodeID]Flow
	funEffects  map[TypeID]*FunEffect
	taintOrigin map[TaintID]ast.NodeID // Which &x expression created each taint.
}

func NewLifetimeAnalyzer(e *Engine) *LifetimeCheck {
	return &LifetimeCheck{
		Diagnostics: nil,
		Debug:       base.NilDebug{},
		e:           e,
		nextTaintID: 1,
		scopes:      map[ScopeID]*TaintScope{},
		flows:       map[ast.NodeID]Flow{},
		funEffects:  map[TypeID]*FunEffect{},
		taintOrigin: map[TaintID]ast.NodeID{},
	}
}

func (a *LifetimeCheck) Check(nodeID ast.NodeID) {
	defer a.Debug.Print(2, "analyze: node=%s", a.e.AST.Debug(nodeID, false, 0)).Indent()()
	a.e.Walk(nodeID, a.Check)
	node := a.e.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.Ref:
		a.analyzeRef(nodeID, kind)
	case ast.Ident:
		a.analyzeIdent(nodeID, kind)
	case ast.Deref:
		a.analyzeDeref(nodeID, kind)
	case ast.FieldAccess:
		a.analyzeFieldAccess(nodeID, kind)
	case ast.AllocInit:
		a.analyzeAllocInit(nodeID, kind)
	case ast.StructLiteral:
		a.analyzeStructLiteral(nodeID, kind)
	case ast.ArrayLiteral:
		a.analyzeArrayLiteral(nodeID, kind)
	case ast.Index:
		a.analyzeIndex(nodeID, kind)
	case ast.Call:
		a.analyzeCall(nodeID, kind)
	case ast.If:
		a.analyzeIf(nodeID, kind)
	case ast.Var:
		a.analyzeVar(nodeID, kind)
	case ast.FunParam:
		a.analyzeFunParam(nodeID, kind)
	case ast.Assign:
		a.analyzeAssign(nodeID, kind)
	case ast.Block:
		a.analyzeBlock(nodeID, kind)
	case ast.Fun:
		a.analyzeFun(nodeID, kind)
	default:
	}
}

func (a *LifetimeCheck) analyzeRef(nodeID ast.NodeID, ref ast.Ref) {
	bt := a.lookupBinding(nodeID, ref.Name.Name)
	if bt == nil {
		return
	}
	// Use flow taints if present (arena-allocated values carry allocator taint).
	// Fall back to AllocTaint for stack values — the scope taint only matters
	// at the point of & (ref-taking), not at variable declaration.
	taints := bt.Flow.Taints
	if len(taints) == 0 {
		taints = TaintSet{bt.AllocTaint}
		a.taintOrigin[bt.AllocTaint] = nodeID
	} else {
		for _, t := range taints {
			a.taintOrigin[t] = nodeID
		}
	}
	scope := a.e.ScopeGraph.NodeScope(nodeID)
	a.flows[nodeID] = Flow{
		Taints:   taints,
		PointsTo: PointsToSet{{scope.ID, ref.Name.Name}},
	}
}

func (a *LifetimeCheck) analyzeIdent(nodeID ast.NodeID, ident ast.Ident) {
	bt := a.lookupBinding(nodeID, ident.Name)
	if bt == nil {
		return
	}
	a.flows[nodeID] = bt.Flow
}

func (a *LifetimeCheck) analyzeDeref(nodeID ast.NodeID, deref ast.Deref) {
	a.flows[nodeID] = a.derefFlow(a.flow(deref.Expr).PointsTo)
}

func (a *LifetimeCheck) analyzeFieldAccess(nodeID ast.NodeID, fa ast.FieldAccess) {
	targetType := a.e.TypeOfNode(fa.Target)
	if _, ok := targetType.Kind.(RefType); ok {
		// Auto-deref: follow the ref's points-to set to reach the struct
		// binding, then propagate its stored refs.
		a.flows[nodeID] = a.derefFlow(a.flow(fa.Target).PointsTo)
		return
	}
	a.flows[nodeID] = a.flow(fa.Target)
}

func (a *LifetimeCheck) analyzeAllocInit(nodeID ast.NodeID, alloc ast.AllocInit) {
	ts := a.scope(nodeID)
	ts.Bindings[alloc.Name.Name] = &BindingTaint{
		nodeID, ts.AllocTaint,
		Flow{Taints: TaintSet{ts.AllocTaint}, PointsTo: nil},
	}
}

func (a *LifetimeCheck) analyzeStructLiteral(nodeID ast.NodeID, lit ast.StructLiteral) {
	merged := Flow{}
	for _, argNodeID := range lit.Args {
		merged = merged.Merge(a.flow(argNodeID))
	}
	if lit.Alloc != nil {
		if bt := a.lookupBinding(nodeID, lit.Alloc.Name); bt != nil {
			merged = merged.Merge(bt.Flow)
		}
	}
	a.flows[nodeID] = merged
}

func (a *LifetimeCheck) analyzeArrayLiteral(nodeID ast.NodeID, lit ast.ArrayLiteral) {
	merged := Flow{}
	for _, elemNodeID := range lit.Elems {
		merged = merged.Merge(a.flow(elemNodeID))
	}
	a.flows[nodeID] = merged
}

func (a *LifetimeCheck) analyzeIndex(nodeID ast.NodeID, index ast.Index) {
	a.flows[nodeID] = a.flow(index.Target)
}

func (a *LifetimeCheck) analyzeCall(nodeID ast.NodeID, call ast.Call) {
	calleeType := a.e.TypeOfNode(call.Callee)
	if _, ok := calleeType.Kind.(FunType); !ok {
		return
	}
	effect, ok := a.funEffects[calleeType.ID]
	if !ok {
		return
	}
	merged := Flow{}
	for _, paramIdx := range effect.ReturnParamTaints {
		merged = merged.Merge(a.flow(call.Args[paramIdx]))
	}
	a.flows[nodeID] = merged
	a.Debug.Print(1, "analyzeCall: %s taints=%s", a.e.AST.Debug(call.Callee, false, 0), merged.Taints)
}

func (a *LifetimeCheck) analyzeIf(nodeID ast.NodeID, ifNode ast.If) {
	merged := a.flow(ifNode.Then)
	if ifNode.Else != nil {
		merged = merged.Merge(a.flow(*ifNode.Else))
	}
	a.flows[nodeID] = merged
}

func (a *LifetimeCheck) analyzeVar(nodeID ast.NodeID, varNode ast.Var) {
	ts := a.scope(nodeID)
	f := a.flow(varNode.Expr)
	ts.Bindings[varNode.Name.Name] = &BindingTaint{nodeID, ts.AllocTaint, f}
}

func (a *LifetimeCheck) analyzeFunParam(nodeID ast.NodeID, funParam ast.FunParam) {
	ts := a.scope(nodeID)
	refs := TaintSet{}
	// Ref and allocator params carry a taint from the caller, not the callee.
	switch a.e.TypeOfNode(nodeID).Kind.(type) {
	case RefType, AllocType:
		refs = TaintSet{a.newTaintID()}
	}
	ts.Bindings[funParam.Name.Name] = &BindingTaint{nodeID, ts.AllocTaint, Flow{Taints: refs, PointsTo: nil}}
}

func (a *LifetimeCheck) analyzeAssign(nodeID ast.NodeID, assign ast.Assign) {
	rhs := a.flow(assign.RHS)
	lhsNode := a.e.Node(assign.LHS)
	switch lhsKind := lhsNode.Kind.(type) {
	case ast.Ident:
		ts := a.scope(nodeID)
		ts.Bindings[lhsKind.Name] = &BindingTaint{nodeID, 0, rhs}
	case ast.FieldAccess:
		a.analyzeAssignToField(nodeID, lhsKind, rhs)
	case ast.Deref:
		a.analyzeAssignThroughDeref(nodeID, lhsKind, rhs)
	default:
		panic(base.Errorf("unknown LHS kind: %T", lhsKind))
	}
}

func (a *LifetimeCheck) analyzeAssignToField(nodeID ast.NodeID, fa ast.FieldAccess, rhs Flow) {
	// Walk field-access chain to find the root binding.
	root := fa.Target
	for {
		if inner, ok := a.e.Node(root).Kind.(ast.FieldAccess); ok {
			root = inner.Target
			continue
		}
		break
	}
	rootName := base.Cast[ast.Ident](a.e.Node(root).Kind).Name
	ts := a.scope(nodeID)
	if bt, ok := ts.Bindings[rootName]; ok {
		bt.Flow = bt.Flow.Merge(rhs)
	} else {
		ts.Bindings[rootName] = &BindingTaint{nodeID, 0, rhs}
	}
}

func (a *LifetimeCheck) analyzeAssignThroughDeref(nodeID ast.NodeID, deref ast.Deref, rhs Flow) {
	targets := a.flow(deref.Expr).PointsTo
	a.Debug.Print(1, "analyzeAssign: rhsTargets=%s, targets=%s", rhs.PointsTo, targets)
	localScope := a.scope(nodeID)
	for _, target := range targets {
		targetScope := a.scopeByID(target.ScopeID)
		if targetScope != localScope && rhs.Taints.ContainsAny(localScope.LocalTaints) {
			a.diag(a.e.Node(nodeID).Span, "reference escaping its allocation scope")
		}
		if bt, ok := targetScope.Bindings[target.Name]; ok {
			bt.Flow = rhs
		} else {
			targetScope.Bindings[target.Name] = &BindingTaint{nodeID, 0, rhs}
		}
	}
}

func (a *LifetimeCheck) analyzeBlock(nodeID ast.NodeID, block ast.Block) {
	outerScope := a.e.ScopeGraph.NodeScope(nodeID)
	defer a.Debug.Print(0, "analyzeBlock: scope=%s node=%s", outerScope.ID, a.e.AST.Debug(nodeID, false, 0)).Indent()()
	if len(block.Exprs) == 0 {
		return
	}
	lastExpr := block.Exprs[len(block.Exprs)-1]
	lastFlow := a.flow(lastExpr)

	// Function bodies (CreateScope=false) just bubble up the last expression's flow.
	if !block.CreateScope {
		a.flows[nodeID] = lastFlow
		return
	}

	ts := a.scope(lastExpr)
	parentScope := a.scopeByID(outerScope.ID)

	// Propagate mutations of outer-scope bindings back to the parent scope.
	// Skip bindings that are declared directly in the block's own scope
	// (they shadow outer bindings and are not mutations).
	innerScope := a.e.ScopeGraph.NodeScope(lastExpr)
	for name, bt := range ts.Bindings {
		if _, foundIn, ok := innerScope.Lookup(name); !ok || foundIn == innerScope {
			continue
		}
		if bt.Flow.Taints.ContainsAny(ts.LocalTaints) {
			a.diag(a.e.Node(bt.DiagNode).Span, "reference escaping its allocation scope")
		}
		if pbt, ok := parentScope.Bindings[name]; ok {
			pbt.Flow = pbt.Flow.Merge(bt.Flow)
		} else {
			parentScope.Bindings[name] = &BindingTaint{bt.DiagNode, 0, bt.Flow}
		}
	}

	// Check the block result for escaping local taints, then bubble up survivors.
	if lastFlow.Taints.ContainsAny(ts.LocalTaints) {
		a.diagEscape(lastExpr, lastFlow.Taints, ts)
	}
	a.flows[nodeID] = Flow{lastFlow.Taints.Without(ts.LocalTaints), lastFlow.PointsTo}
}

func (a *LifetimeCheck) analyzeFun(nodeID ast.NodeID, fun ast.Fun) {
	blockFlow := a.flow(fun.Block)

	// Check for locals escaping through the return value.
	ts := a.scope(fun.Block)
	if blockFlow.Taints.ContainsAny(ts.LocalTaints) {
		block := base.Cast[ast.Block](a.e.Node(fun.Block).Kind)
		lastExpr := block.Exprs[len(block.Exprs)-1]
		a.diagEscape(lastExpr, blockFlow.Taints, ts)
	}

	// Derive function effect: which param taints appear in the return value.
	paramTaintToIndex := map[TaintID]int{}
	for i, paramNodeID := range fun.Params {
		paramName := base.Cast[ast.FunParam](a.e.Node(paramNodeID).Kind).Name.Name
		if bt := a.lookupBinding(paramNodeID, paramName); bt != nil {
			for _, t := range bt.Flow.Taints {
				paramTaintToIndex[t] = i
			}
		}
	}
	effect := &FunEffect{ReturnParamTaints: map[TaintID]int{}}
	for _, t := range blockFlow.Taints {
		if idx, ok := paramTaintToIndex[t]; ok {
			effect.ReturnParamTaints[t] = idx
		}
	}
	if len(effect.ReturnParamTaints) > 0 {
		a.funEffects[a.e.TypeOfNode(nodeID).ID] = effect
		a.Debug.Print(1, "analyzeFun: effect for %s: %v", fun.Name.Name, effect.ReturnParamTaints)
	}
}

func (a *LifetimeCheck) flow(nodeID ast.NodeID) Flow {
	if f, ok := a.flows[nodeID]; ok {
		return f
	}
	return Flow{}
}

// derefFlow follows a points-to set to the bindings they reference,
// collecting the refs stored in those bindings.
func (a *LifetimeCheck) derefFlow(targets PointsToSet) Flow {
	result := Flow{}
	for _, target := range targets {
		ts := a.scopeByID(target.ScopeID)
		if bt, ok := ts.Bindings[target.Name]; ok {
			result = result.Merge(bt.Flow)
		}
	}
	return result
}

func (a *LifetimeCheck) scope(nodeID ast.NodeID) *TaintScope {
	return a.scopeByID(a.e.ScopeGraph.NodeScope(nodeID).ID)
}

func (a *LifetimeCheck) scopeByID(id ScopeID) *TaintScope {
	if ts, ok := a.scopes[id]; ok {
		return ts
	}
	ts := newTaintScope(a.newTaintID())
	a.scopes[id] = ts
	return ts
}

func (a *LifetimeCheck) lookupBinding(nodeID ast.NodeID, name string) *BindingTaint {
	scope := a.e.ScopeGraph.NodeScope(nodeID)
	for scope != nil {
		if ts, ok := a.scopes[scope.ID]; ok {
			if bt, ok := ts.Bindings[name]; ok {
				return bt
			}
		}
		scope = scope.Parent
	}
	return nil
}

func (a *LifetimeCheck) newTaintID() TaintID {
	id := a.nextTaintID
	a.nextTaintID++
	return id
}

func (a *LifetimeCheck) diagEscape(fallbackNodeID ast.NodeID, taints TaintSet, ts *TaintScope) {
	reported := map[ast.NodeID]bool{}
	for _, t := range taints {
		if !slices.Contains(ts.LocalTaints, t) {
			continue
		}
		diagNode := fallbackNodeID
		if origin, ok := a.taintOrigin[t]; ok {
			diagNode = origin
		}
		if reported[diagNode] {
			continue
		}
		reported[diagNode] = true
		a.diag(a.e.Node(diagNode).Span, "reference escaping its allocation scope")
	}
}

func (a *LifetimeCheck) diag(span base.Span, msg string, msgArgs ...any) {
	a.Diagnostics = append(a.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}
