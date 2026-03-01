// Lifetime analysis prevents dangling references. A reference must not
// outlive the allocation scope of its referent: stack refs must stay in
// their block, heap refs must stay in the allocator's scope.
//
// The mechanism is taint propagation:
//
//   - Each scope gets a unique TaintID (its "ScopeTaint").
//   - Taking `&x` produces a ref that carries x's ScopeTaint.
//   - Heap-allocated values (e.g. `new(@myalloc, Foo())`) don't carry a
//     ScopeTaint because they outlive their declaring scope. Instead
//     they carry the allocator's taint, so a ref to a heap value can
//     escape the declaring function but not the allocator's scope.
//   - Taints propagate through assignments, field writes, function calls.
//   - When a value leaves a scope, we check if it carries that scope's
//     taint. If so, it is an escape and we report a diagnostic.
//
// Key types:
//
//   - Flow: what an expression carries (a set of taints + a set of aliases).
//   - VarTaint: per-variable state (its StorageTaint + its Flow).
//   - FunEffects: how a function's params flow into return / other params.
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

// Alias identifies a variable that a reference may point to.
//   - `let b = &a` --> b's PointsTo = [{scope(a), "a"}]
//   - `let c = if cond { &a } else { &b }` --> c's PointsTo = [{..,"a"}, {..,"b"}]
type Alias struct {
	ScopeID ScopeID
	Name    string
}

type AliasSet []Alias

func (r AliasSet) Merge(other AliasSet) AliasSet {
	for _, target := range other {
		if !slices.Contains(r, target) {
			r = append(r, target)
		}
	}
	return r
}

// Flow is the abstract value of an expression. It tracks which scope taints
// the value carries and which variables it may alias (point to).
//
//	let a = 1          --> flow(a) = {taints: {scope0}, pointsTo: {}}
//	let b = &a         --> flow(&a) = {taints: {scope0}, pointsTo: {a}}
//	let c = Foo(&a)    --> flow(Foo(&a)) merges all arg flows
type Flow struct {
	Taints   TaintSet
	PointsTo AliasSet
}

func (f Flow) Merge(o Flow) Flow {
	return Flow{f.Taints.Merge(o.Taints), f.PointsTo.Merge(o.PointsTo)}
}

// VarTaint is the abstract state of a single named variable during analysis.
//
//	mut a = 123          --> VarTaint{StorageTaint: scope_taint, Flow: {}}
//	let b = &a           --> VarTaint{StorageTaint: scope_taint, Flow: {taints: {scope_taint}, pointsTo: {a}}}
//	let c = @arena Foo() --> VarTaint{StorageTaint: 0, Flow: {taints: {arena_taint}}}
type VarTaint struct {
	DiagNode     ast.NodeID // Node to blame in escape diagnostics.
	StorageTaint TaintID    // Taint of the scope that owns this variable's memory.
	Flow         Flow       // Taints and aliases carried by references in this variable.
}

// ScopeState tracks all variable taints and the scope's own taint for one
// lexical scope. The ScopeTaint is added to LocalTaints and used as the
// StorageTaint for variables declared in this scope.
type ScopeState struct {
	ID          ScopeID
	Vars        map[string]*VarTaint
	LocalTaints TaintSet // Taints born in this scope (just ScopeTaint).
	ScopeTaint  TaintID  // The taint that represents this scope's storage lifetime.
}

func newScopeState(id ScopeID, scopeTaint TaintID) *ScopeState {
	return &ScopeState{
		ID:          id,
		Vars:        map[string]*VarTaint{},
		LocalTaints: TaintSet{scopeTaint},
		ScopeTaint:  scopeTaint,
	}
}

// FunEffects describes how a function's parameters flow into its return value
// and into each other (side effects through &mut params).
//
//	fun foo(a &Int) &Int { a }
//	  --> ReturnTaints:   {taint(a) --> 0}  (param 0's taints flow to return)
//	  --> ReturnAliases:  [0]              (return value may alias param 0)
//
//	fun bar(a &mut Int, b &Int) void { *a = *b }
//	  --> SideEffects: {0: [1]}            (param 1's taints flow into param 0)
type FunEffects struct {
	ReturnTaints  map[TaintID]int // taint --> param index: which param taints appear in return value
	ReturnAliases []int           // param indices whose aliases appear in return value
	SideEffects   map[int][]int   // target param --> source params: which params' taints flow into which
}

type analysisStatus int

const (
	statusNotVisited analysisStatus = iota
	statusInProgress
	statusDone
)

type LifetimeCheck struct {
	Diagnostics base.Diagnostics
	Debug       base.Debug
	e           *Engine
	nextTaintID TaintID
	scopes      map[ScopeID]*ScopeState
	flows       map[ast.NodeID]Flow
	funEffects  map[TypeID]*FunEffects
	taintOrigin map[TaintID]ast.NodeID // Which &x created this taint (for diagnostics).
	status      map[ast.NodeID]analysisStatus
}

func NewLifetimeAnalyzer(e *Engine) *LifetimeCheck {
	return &LifetimeCheck{
		Diagnostics: nil,
		Debug:       base.NilDebug{},
		e:           e,
		nextTaintID: 1,
		scopes:      map[ScopeID]*ScopeState{},
		flows:       map[ast.NodeID]Flow{},
		funEffects:  map[TypeID]*FunEffects{},
		taintOrigin: map[TaintID]ast.NodeID{},
		status:      map[ast.NodeID]analysisStatus{},
	}
}

func (a *LifetimeCheck) Check(nodeID ast.NodeID) {
	if a.status[nodeID] == statusDone {
		return
	}
	a.status[nodeID] = statusInProgress
	defer func() { a.status[nodeID] = statusDone }()

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
	case ast.AllocatorVar:
		a.analyzeAllocatorVar(nodeID, kind)
	case ast.StructLiteral:
		a.analyzeStructLiteral(nodeID, kind)
	case ast.New:
		a.analyzeNew(nodeID, kind)
	case ast.MakeSlice:
		a.analyzeMakeSlice(nodeID, kind)
	case ast.ArrayLiteral:
		a.analyzeArrayLiteral(nodeID, kind)
	case ast.Index:
		a.analyzeIndex(nodeID, kind)
	case ast.Call:
		a.analyzeCall(nodeID, kind)
	case ast.For:
		a.analyzeFor(nodeID, kind)
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

// analyzeRef: `&x` or `&mut x`.
// The ref carries x's storage taint (so it dies with x's scope) plus any
// taints x already holds (if x itself stores refs).
//   - `mut a = 1; &a`      --> taints: {scope_taint(a)}, pointsTo: {a}
//   - `mut a = &b; &a`     --> taints: {scope_taint(a), scope_taint(b)}, pointsTo: {a}
func (a *LifetimeCheck) analyzeRef(nodeID ast.NodeID, ref ast.Ref) {
	vt, foundIn := a.lookupVarWithScope(nodeID, ref.Name.Name)
	if vt == nil {
		return
	}
	taints := vt.Flow.Taints.Merge(TaintSet{vt.StorageTaint})
	for _, t := range taints {
		if _, ok := a.taintOrigin[t]; !ok {
			a.taintOrigin[t] = nodeID
		}
	}
	// PointsTo uses the scope where the variable is *declared*, not where
	// the & expression appears. This matters for shadowed variables.
	a.flows[nodeID] = Flow{
		Taints:   taints,
		PointsTo: AliasSet{{foundIn.ID, ref.Name.Name}},
	}
}

// analyzeIdent: reading a variable propagates its flow.
func (a *LifetimeCheck) analyzeIdent(nodeID ast.NodeID, ident ast.Ident) {
	vt := a.lookupVar(nodeID, ident.Name)
	if vt == nil {
		return
	}
	a.flows[nodeID] = vt.Flow
}

// analyzeDeref: `*x` follows x's aliases to find the target variable's flow.
//   - `mut a = 1; mut b = &a; *b` --> flow of a
func (a *LifetimeCheck) analyzeDeref(nodeID ast.NodeID, deref ast.Deref) {
	a.flows[nodeID] = a.derefFlow(a.flow(deref.Expr).PointsTo)
}

// analyzeFieldAccess: `x.foo`.
// If x is a ref, auto-deref through x's aliases first.
func (a *LifetimeCheck) analyzeFieldAccess(nodeID ast.NodeID, fa ast.FieldAccess) {
	targetType := a.e.TypeOfNode(fa.Target)
	if _, ok := targetType.Kind.(RefType); ok {
		a.flows[nodeID] = a.derefFlow(a.flow(fa.Target).PointsTo)
		return
	}
	a.flows[nodeID] = a.flow(fa.Target)
}

func (a *LifetimeCheck) analyzeAllocatorVar(nodeID ast.NodeID, alloc ast.AllocatorVar) {
	ss := a.scopeState(nodeID)
	ss.Vars[alloc.Name.Name] = &VarTaint{
		nodeID, ss.ScopeTaint,
		Flow{Taints: TaintSet{ss.ScopeTaint}, PointsTo: nil},
	}
}

// analyzeStructLiteral: `Foo(a, b)` merges all argument flows.
func (a *LifetimeCheck) analyzeStructLiteral(nodeID ast.NodeID, lit ast.StructLiteral) {
	merged := Flow{}
	for _, argNodeID := range lit.Args {
		merged = merged.Merge(a.flow(argNodeID))
	}
	a.flows[nodeID] = merged
}

// analyzeNew: `new(@alloc, Foo(...))` merges the target's flow with the allocator's.
func (a *LifetimeCheck) analyzeNew(nodeID ast.NodeID, alloc ast.New) {
	merged := a.flow(alloc.Target)
	merged = merged.Merge(a.flow(alloc.Allocator))
	a.flows[nodeID] = merged
}

// analyzeMakeSlice: `make(@alloc, []T(len))` merges the allocator's flow.
func (a *LifetimeCheck) analyzeMakeSlice(nodeID ast.NodeID, makeSlice ast.MakeSlice) {
	merged := a.flow(makeSlice.Allocator)
	merged = merged.Merge(a.flow(makeSlice.Len))
	a.flows[nodeID] = merged
}

func (a *LifetimeCheck) analyzeArrayLiteral(nodeID ast.NodeID, lit ast.ArrayLiteral) {
	merged := Flow{}
	for _, elemNodeID := range lit.Elems {
		merged = merged.Merge(a.flow(elemNodeID))
	}
	a.flows[nodeID] = merged
}

// analyzeIndex: `arr[i]` propagates the array's flow (conservative: any element).
func (a *LifetimeCheck) analyzeIndex(nodeID ast.NodeID, index ast.Index) {
	a.flows[nodeID] = a.flow(index.Target)
}

// analyzeCall applies the callee's FunEffects to map argument flows into
// the call result's flow and into side-effected arguments.
//
// If the function hasn't been analyzed yet, we analyze it on demand.
// If we detect a cycle (mutual recursion), we apply pessimistic effects.
func (a *LifetimeCheck) analyzeCall(nodeID ast.NodeID, call ast.Call) {
	calleeType := a.e.TypeOfNode(call.Callee)
	if _, ok := calleeType.Kind.(FunType); !ok {
		return
	}

	effects, ok := a.funEffects[calleeType.ID]
	if !ok {
		declID := a.e.DeclNode(calleeType.ID)
		if declID != 0 {
			if a.status[declID] == statusInProgress {
				a.Debug.Print(1, "analyzeCall: cycle detected for %s, pessimistic fallback",
					a.e.AST.Debug(call.Callee, false, 0))
				a.applyPessimisticEffects(nodeID, call)
				return
			}
			a.Check(declID)
			effects, ok = a.funEffects[calleeType.ID]
		}
	}
	if !ok {
		return
	}

	// Apply the effects: map param flows --> return flow.
	result := Flow{}
	for _, paramIdx := range effects.ReturnTaints {
		result.Taints = result.Taints.Merge(a.flow(call.Args[paramIdx]).Taints)
	}
	for _, paramIdx := range effects.ReturnAliases {
		result.PointsTo = result.PointsTo.Merge(a.flow(call.Args[paramIdx]).PointsTo)
	}
	// Apply side effects: param-to-param taint flow.
	//   fun swap(a &mut Int, b &Int) { *a = *b }
	//   --> side effect: param 1's flow merges into param 0's target
	for targetIdx, srcIndices := range effects.SideEffects {
		srcFlow := Flow{}
		for _, srcIdx := range srcIndices {
			srcFlow = srcFlow.Merge(a.flow(call.Args[srcIdx]))
		}
		a.mergeIntoTarget(nodeID, call.Args[targetIdx], srcFlow)
	}
	a.flows[nodeID] = result
	a.Debug.Print(1, "analyzeCall: %s taints=%s pointsTo=%s",
		a.e.AST.Debug(call.Callee, false, 0), result.Taints, result.PointsTo)
}

// applyPessimisticEffects: assume every arg flows into the return value and
// into every &mut arg. Used when we can't determine the actual effects
// (recursive calls).
func (a *LifetimeCheck) applyPessimisticEffects(nodeID ast.NodeID, call ast.Call) {
	allArgs := Flow{}
	for _, arg := range call.Args {
		allArgs = allArgs.Merge(a.flow(arg))
	}
	a.flows[nodeID] = allArgs
	for _, arg := range call.Args {
		if ref, ok := a.e.TypeOfNode(arg).Kind.(RefType); ok && ref.Mut {
			a.mergeIntoTarget(nodeID, arg, allArgs)
		}
	}
}

func (a *LifetimeCheck) analyzeIf(nodeID ast.NodeID, ifNode ast.If) {
	merged := a.flow(ifNode.Then)
	if ifNode.Else != nil {
		merged = merged.Merge(a.flow(*ifNode.Else))
	}
	a.flows[nodeID] = merged
}

// analyzeFor: `for cond { body }`.
// The type checker creates a scope for the loop body (the body block has
// CreateScope=false). We check for mutations to outer-scope variables
// that carry the body-scope's taint, just like analyzeBlock does for
// CreateScope=true blocks. For-loops are always void, so there is no
// result flow to check.
func (a *LifetimeCheck) analyzeFor(nodeID ast.NodeID, forNode ast.For) {
	outerScope := a.e.ScopeGraph.NodeScope(nodeID)
	ss := a.scopeState(forNode.Body)
	parentState := a.scopeByID(outerScope.ID)

	innerScope := a.e.ScopeGraph.NodeScope(forNode.Body)
	for name, vt := range ss.Vars {
		if _, foundIn, ok := innerScope.Lookup(name); !ok || foundIn == innerScope {
			continue
		}
		if vt.Flow.Taints.ContainsAny(ss.LocalTaints) {
			a.diagEscape(vt.DiagNode, vt.Flow.Taints, ss)
		}
		if pvt, ok := parentState.Vars[name]; ok {
			pvt.Flow = pvt.Flow.Merge(vt.Flow)
		} else {
			parentState.Vars[name] = &VarTaint{vt.DiagNode, 0, vt.Flow}
		}
	}
}

// analyzeVar: `let x = expr` or `mut x = expr`.
// Arena-allocated vars get StorageTaint=0 because they outlive their scope.
func (a *LifetimeCheck) analyzeVar(nodeID ast.NodeID, varNode ast.Var) {
	ss := a.scopeState(nodeID)
	f := a.flow(varNode.Expr)
	storageTaint := ss.ScopeTaint
	switch a.e.Node(varNode.Expr).Kind.(type) {
	case ast.New, ast.MakeSlice:
		storageTaint = 0
	}
	ss.Vars[varNode.Name.Name] = &VarTaint{nodeID, storageTaint, f}
}

// analyzeFunParam: function parameters with ref/alloc types carry a caller
// taint (they came from outside), not the function's own scope taint.
// A self-referencing alias is added so that side-effect tracking can find
// the param by name.
func (a *LifetimeCheck) analyzeFunParam(nodeID ast.NodeID, param ast.FunParam) {
	ss := a.scopeState(nodeID)
	callerTaints := TaintSet{}
	if a.typeContainsRefOrAlloc(a.e.TypeOfNode(nodeID).ID) {
		callerTaints = TaintSet{a.newTaintID()}
	}
	ss.Vars[param.Name.Name] = &VarTaint{
		nodeID, ss.ScopeTaint,
		Flow{
			Taints:   callerTaints,
			PointsTo: AliasSet{{ss.ID, param.Name.Name}},
		},
	}
}

// typeContainsRefOrAlloc returns true if the type is, or recursively contains,
// a RefType or AllocatorType. Used to decide whether a param needs a caller taint.
//   - `Int` --> false
//   - `&Int` --> true
//   - `struct Foo { ptr &Int }` --> true
func (a *LifetimeCheck) typeContainsRefOrAlloc(typeID TypeID) bool {
	if typeID == InvalidTypeID {
		return false
	}
	typ := a.e.Type(typeID)
	switch kind := typ.Kind.(type) {
	case RefType, AllocatorType:
		return true
	case StructType:
		for _, field := range kind.Fields {
			if a.typeContainsRefOrAlloc(field.Type) {
				return true
			}
		}
	case ArrayType:
		return a.typeContainsRefOrAlloc(kind.Elem)
	case SliceType:
		return a.typeContainsRefOrAlloc(kind.Elem)
	}
	return false
}

func (a *LifetimeCheck) analyzeAssign(nodeID ast.NodeID, assign ast.Assign) {
	rhs := a.flow(assign.RHS)
	lhsNode := a.e.Node(assign.LHS)
	a.Debug.Print(1, "analyzeAssign: lhs=%s rhsTaints=%s rhsPointsTo=%s",
		a.e.AST.Debug(assign.LHS, false, 0), rhs.Taints, rhs.PointsTo)
	switch lhsKind := lhsNode.Kind.(type) {
	case ast.Ident:
		// `x = expr` - replace x's flow, preserve its storage taint.
		ss := a.scopeState(nodeID)
		storageTaint := ss.ScopeTaint
		if vt := a.lookupVar(nodeID, lhsKind.Name); vt != nil {
			storageTaint = vt.StorageTaint
		}
		ss.Vars[lhsKind.Name] = &VarTaint{nodeID, storageTaint, rhs}
	case ast.FieldAccess:
		// `x.foo = expr` - merge into x.
		a.mergeIntoTarget(nodeID, lhsKind.Target, rhs)
	case ast.Index:
		// `x[i] = expr` - merge into x.
		a.mergeIntoTarget(nodeID, lhsKind.Target, rhs)
	case ast.Deref:
		// `*x = expr` - write through the pointer.
		a.analyzeDerefAssign(nodeID, lhsKind, rhs)
	default:
		panic(base.Errorf("unknown LHS kind: %T", lhsKind))
	}
}

// mergeIntoTarget handles field/index writes like `foo.bar = expr` or `arr[i] = expr`.
//
// We track taints at the granularity of the root variable, not individual
// fields/elements. So `foo.bar = &x` adds x's taint to foo's flow. A later
// `foo.baz = "safe"` does NOT erase the taint from the first write - we merge.
//
// If the root is a ref (e.g. `(&mut foo).bar = expr` via auto-deref), we follow
// its aliases to find the actual variables being mutated.
func (a *LifetimeCheck) mergeIntoTarget(nodeID ast.NodeID, target ast.NodeID, rhs Flow) {
	// Peel off intermediate field/index access to find the root.
	//   `foo.bar.baz` --> root is `foo`
	//   `arr[0].field` --> root is `arr`
	root := target
	for {
		switch inner := a.e.Node(root).Kind.(type) {
		case ast.FieldAccess:
			root = inner.Target
			continue
		case ast.Index:
			root = inner.Target
			continue
		}
		break
	}

	// Resolve which variables the root refers to.
	var aliases AliasSet
	switch kind := a.e.Node(root).Kind.(type) {
	case ast.Ident:
		aliases = AliasSet{{a.scopeState(root).ID, kind.Name}}
		if vt := a.lookupVar(root, kind.Name); vt != nil && len(vt.Flow.PointsTo) > 0 {
			aliases = vt.Flow.PointsTo
		}
	case ast.Ref:
		aliases = AliasSet{{a.scopeState(root).ID, kind.Name.Name}}
		if vt := a.lookupVar(root, kind.Name.Name); vt != nil && len(vt.Flow.PointsTo) > 0 {
			aliases = vt.Flow.PointsTo
		}
	default:
		aliases = a.flow(root).PointsTo
	}

	a.Debug.Print(1, "mergeIntoTarget: root=%s aliases=%s", a.e.AST.Debug(root, false, 0), aliases)
	ss := a.scopeState(nodeID)

	for _, alias := range aliases {
		storageTaint := ss.ScopeTaint
		merged := rhs
		if vt := a.lookupVar(nodeID, alias.Name); vt != nil {
			storageTaint = vt.StorageTaint
			merged = vt.Flow.Merge(rhs)
		}
		ss.Vars[alias.Name] = &VarTaint{nodeID, storageTaint, merged}
	}
}

// analyzeDerefAssign handles `*x = expr`. The write goes into x's target,
// not into x itself. If the target lives in an outer scope and the RHS
// carries a local taint, that's an escape.
//
//	{ mut a = 1; mut b = &mut a; { mut c = 2; *b = c } }
//	--> *b writes into a (outer scope), c's taint escapes.
func (a *LifetimeCheck) analyzeDerefAssign(nodeID ast.NodeID, deref ast.Deref, rhs Flow) {
	targets := a.flow(deref.Expr).PointsTo
	a.Debug.Print(1, "analyzeDerefAssign: rhsPointsTo=%s, targets=%s", rhs.PointsTo, targets)
	localScope := a.scopeState(nodeID)
	for _, target := range targets {
		targetScope := a.scopeByID(target.ScopeID)
		if targetScope != localScope && rhs.Taints.ContainsAny(localScope.LocalTaints) {
			a.diagEscape(nodeID, rhs.Taints, localScope)
		}
		if vt, ok := targetScope.Vars[target.Name]; ok {
			vt.Flow = rhs
		} else {
			targetScope.Vars[target.Name] = &VarTaint{nodeID, 0, rhs}
		}
	}
}

// analyzeBlock checks the block result and any mutations to outer-scope
// variables for escaping local taints.
//
//	{                          <-- outer scope
//	    mut a = 1
//	    {                      <-- inner scope
//	        mut c = 2
//	        a = &c             <-- mutation of outer var with inner taint --> escape!
//	    }
//	}
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

	ss := a.scopeState(lastExpr)
	parentState := a.scopeByID(outerScope.ID)

	// Propagate mutations to outer-scope variables back to the parent.
	// Skip variables declared in this block's own scope (they shadow, not mutate).
	innerScope := a.e.ScopeGraph.NodeScope(lastExpr)
	for name, vt := range ss.Vars {
		if _, foundIn, ok := innerScope.Lookup(name); !ok || foundIn == innerScope {
			continue
		}
		if vt.Flow.Taints.ContainsAny(ss.LocalTaints) {
			a.diagEscape(vt.DiagNode, vt.Flow.Taints, ss)
		}
		if pvt, ok := parentState.Vars[name]; ok {
			pvt.Flow = pvt.Flow.Merge(vt.Flow)
		} else {
			parentState.Vars[name] = &VarTaint{vt.DiagNode, 0, vt.Flow}
		}
	}

	// Check the block result for escaping local taints, then strip them.
	if lastFlow.Taints.ContainsAny(ss.LocalTaints) {
		a.diagEscape(lastExpr, lastFlow.Taints, ss)
	}
	a.flows[nodeID] = Flow{lastFlow.Taints.Without(ss.LocalTaints), lastFlow.PointsTo}
}

// analyzeFun checks for locals escaping through the return value and
// builds a FunEffects that describes how parameter flows map to the
// return value and to each other (side effects).
func (a *LifetimeCheck) analyzeFun(nodeID ast.NodeID, fun ast.Fun) {
	blockFlow := a.flow(fun.Block)

	// Check for locals escaping through the return value.
	ss := a.scopeState(fun.Block)
	if blockFlow.Taints.ContainsAny(ss.LocalTaints) {
		block := base.Cast[ast.Block](a.e.Node(fun.Block).Kind)
		lastExpr := block.Exprs[len(block.Exprs)-1]
		a.diagEscape(lastExpr, blockFlow.Taints, ss)
	}

	// Build reverse maps: taint --> param index, alias --> param index.
	paramTaintToIdx := map[TaintID]int{}
	paramAliasToIdx := map[Alias]int{}
	paramScope := a.e.ScopeGraph.NodeScope(fun.Block)
	for i, paramNodeID := range fun.Params {
		name := base.Cast[ast.FunParam](a.e.Node(paramNodeID).Kind).Name.Name
		paramAliasToIdx[Alias{paramScope.ID, name}] = i
		if vt := a.lookupVar(paramNodeID, name); vt != nil {
			for _, t := range vt.Flow.Taints {
				paramTaintToIdx[t] = i
			}
		}
	}

	effects := &FunEffects{
		ReturnTaints:  map[TaintID]int{},
		ReturnAliases: []int{},
		SideEffects:   map[int][]int{},
	}

	// Which param taints appear in the return value?
	for _, t := range blockFlow.Taints {
		if idx, ok := paramTaintToIdx[t]; ok {
			effects.ReturnTaints[t] = idx
		}
	}

	// Which param aliases appear in the return value?
	for _, alias := range blockFlow.PointsTo {
		if idx, ok := paramAliasToIdx[alias]; ok {
			if !slices.Contains(effects.ReturnAliases, idx) {
				effects.ReturnAliases = append(effects.ReturnAliases, idx)
			}
		}
	}

	// Which params had foreign taints merged into them (side effects)?
	// e.g. `fun foo(a &mut Int, b &Int) { *a = *b }` --> param 0 got param 1's taint.
	for i, paramNodeID := range fun.Params {
		name := base.Cast[ast.FunParam](a.e.Node(paramNodeID).Kind).Name.Name
		if vt := a.lookupVar(paramNodeID, name); vt != nil {
			for _, t := range vt.Flow.Taints {
				if srcIdx, ok := paramTaintToIdx[t]; ok && srcIdx != i {
					if !slices.Contains(effects.SideEffects[i], srcIdx) {
						effects.SideEffects[i] = append(effects.SideEffects[i], srcIdx)
					}
				}
			}
		}
	}

	if len(effects.ReturnTaints) > 0 || len(effects.ReturnAliases) > 0 || len(effects.SideEffects) > 0 {
		a.funEffects[a.e.TypeOfNode(nodeID).ID] = effects
		a.Debug.Print(1, "analyzeFun: effects for %s: taints=%v aliases=%v sideEffects=%v",
			fun.Name.Name, effects.ReturnTaints, effects.ReturnAliases, effects.SideEffects)
	}
}

func (a *LifetimeCheck) flow(nodeID ast.NodeID) Flow {
	if f, ok := a.flows[nodeID]; ok {
		return f
	}
	return Flow{}
}

// derefFlow follows aliases to find the target variables and returns
// their merged flow. This is what `*x` evaluates to.
func (a *LifetimeCheck) derefFlow(targets AliasSet) Flow {
	result := Flow{}
	for _, target := range targets {
		ss := a.scopeByID(target.ScopeID)
		if vt, ok := ss.Vars[target.Name]; ok {
			result = result.Merge(vt.Flow)
		}
	}
	return result
}

func (a *LifetimeCheck) scopeState(nodeID ast.NodeID) *ScopeState {
	return a.scopeByID(a.e.ScopeGraph.NodeScope(nodeID).ID)
}

func (a *LifetimeCheck) scopeByID(id ScopeID) *ScopeState {
	if ss, ok := a.scopes[id]; ok {
		return ss
	}
	ss := newScopeState(id, a.newTaintID())
	a.scopes[id] = ss
	return ss
}

func (a *LifetimeCheck) lookupVar(nodeID ast.NodeID, name string) *VarTaint {
	vt, _ := a.lookupVarWithScope(nodeID, name)
	return vt
}

func (a *LifetimeCheck) lookupVarWithScope(nodeID ast.NodeID, name string) (*VarTaint, *ScopeState) {
	scope := a.e.ScopeGraph.NodeScope(nodeID)
	for scope != nil {
		if ss, ok := a.scopes[scope.ID]; ok {
			if vt, ok := ss.Vars[name]; ok {
				return vt, ss
			}
		}
		scope = scope.Parent
	}
	return nil, nil
}

func (a *LifetimeCheck) newTaintID() TaintID {
	id := a.nextTaintID
	a.nextTaintID++
	return id
}

func (a *LifetimeCheck) diagEscape(fallbackNodeID ast.NodeID, taints TaintSet, ss *ScopeState) {
	reported := map[ast.NodeID]bool{}
	for _, t := range taints {
		if !slices.Contains(ss.LocalTaints, t) {
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
