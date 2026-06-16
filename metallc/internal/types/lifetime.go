// Lifetime analysis prevents dangling references. A reference must not
// outlive the storage of its referent: stack refs must stay in their block,
// heap refs must stay in the allocator's scope.
//
// The model is a STORAGE DEREF CHAIN. Every value carries a Chain: chain[d] is
// the set of scope taints whose STORAGE the value reaches after d derefs.
// chain[0] is the value's own immediate borrow (for a ref, the slot it points
// at). A scope taint identifies one lexical scope; inner scopes are shorter
// lived. Heap is not special: `let @a = Arena()` in scope S makes allocations
// from @a borrow S, so a heap value carries the arena's scope taint at depth 0.
//
//   - Each scope gets a unique TaintID (its ScopeTaint).
//   - `&x` PREPENDS {scope where x lives} to x's chain. For `mut z = &mut y`
//     then `&z`, the chain is [{scope(z)}, {scope(y)}].
//   - `x.*` DROPS the head: x.chain[1:]. The depth-d slot is chain[d].
//   - A read of `x.f` carries the container's chain only if the result type
//     contains a ref/alloc; a scalar read carries {}.
//   - Writing `p.* = rhs` checks rhs against storageScopes(p.*), which is
//     chain(p)[0] (where p points). `w.*.*` consults chain(w)[1], the depth-2
//     slot, so the dangling write is caught at the exact depth.
//   - Moving a value out of a scope (return, block result, capture) escapes
//     if CarriedTaints (the union of all chain levels) contains a taint born
//     in (local to) an exited scope.
//
// Key types:
//
//   - TaintSet: scopes reached at one deref depth.
//   - Chain: per-depth TaintSets; chain[d] is the depth-d storage.
//   - VarTaint: per-variable state (its StorageTaint + its value Chain).
//   - FunEffects: how a function's params flow into return / other params.
package types

import (
	"fmt"
	"slices"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

// A TaintID labels one source of borrowable STORAGE. A value "carries" a taint
// when it might hold a reference into that storage; the whole analysis is just
// bookkeeping of which taints flow where. Two kinds (both opaque integers):
//
//   - SCOPE taint: one per lexical scope (a block, function body, loop body),
//     standing for storage that lives exactly as long as that scope. `mut a = 1`
//     tags a slot with the enclosing scope's taint and `&a` borrows it. Inner
//     scopes are shorter-lived, so a value carrying an inner scope's taint must
//     not leave that scope, that leaving is the dangling-reference ESCAPE.
//   - PARAM taint: one per reference-carrying function parameter, standing for
//     caller-provided storage of UNKNOWN lifetime. It belongs to no scope, so it
//     never escapes inside the body; whether it escapes is decided at call sites.
type TaintID int

func (t TaintID) String() string { return fmt.Sprintf("a%d", t) }

// A TaintSet is the storage a value reaches at ONE deref depth (one level of a
// Chain): `&a` reaches {scope(a)}; a value that could point at either of two
// locals reaches {scope(x), scope(y)}.
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

// Chain is a value's storage by deref depth. chain[d] is the set of scopes
// reached after dereferencing the value d times. chain[0] is the immediate
// borrow (for a ref, the slot it points at).
//
//	mut a = 1; &a       --> [{scope(a)}]
//	mut z = &mut y; &z  --> [{scope(z)}, {scope(y)}]
type Chain []TaintSet

// Prepend builds `&x` from x's chain: the new head is the scope x lives in.
//
//	z.chain = [{scope(y)}]; (&z).chain = z.chain.Prepend({scope(z)})
func (c Chain) Prepend(head TaintSet) Chain {
	out := make(Chain, 0, len(c)+1)
	out = append(out, head)
	out = append(out, c...)
	return out
}

// DropHead builds `x.*` from x's chain: drop the immediate borrow so chain[d]
// becomes the depth-(d+1) storage of x.
//
//	w.chain = [{scope(z)}, {scope(y)}]; w.chain.DropHead() = [{scope(y)}]
func (c Chain) DropHead() Chain {
	if len(c) == 0 {
		return nil
	}
	return slices.Clone(c[1:])
}

// HeadTaintSet returns chain[0], the immediate storage a ref points at. An empty
// chain (a scalar, borrowing nothing) yields {}.
//
//	storageScopes(Deref(p)) = chain(p).HeadTaintSet()
func (c Chain) HeadTaintSet() TaintSet {
	if len(c) == 0 {
		return nil
	}
	return c[0]
}

// Merge unions two chains per depth, padding the shorter with {}. Used to
// accumulate a write into a variable and to join if/match arms.
//
//	[{a}].Merge([{b},{c}]) = [{a,b}, {c}]
func (c Chain) Merge(other Chain) Chain {
	n := max(len(c), len(other))
	out := make(Chain, n)
	for i := range out {
		var level TaintSet
		if i < len(c) {
			level = level.Merge(c[i])
		}
		if i < len(other) {
			level = level.Merge(other[i])
		}
		out[i] = level
	}
	return out
}

// CarriedTaints flattens a chain into the union of every depth's scopes: the
// conservative total reach of the value. Used for escape membership (block
// result, return, capture) and as the rhs side of the write rule.
//
//	[{scope(z)}, {scope(y)}].CarriedTaints() = {scope(z), scope(y)}
func (c Chain) CarriedTaints() TaintSet {
	var out TaintSet
	for _, level := range c {
		out = out.Merge(level)
	}
	return out
}

// Without removes the given taints from every depth of a chain. Used to strip a
// block's local scope taints from the value that leaves the block.
func (c Chain) Without(exclude TaintSet) Chain {
	out := make(Chain, len(c))
	for i, level := range c {
		out[i] = level.Without(exclude)
	}
	return out
}

// VarTaint is the abstract state of a single named variable during analysis.
//
//	mut a = 123          --> VarTaint{StorageTaint: scope(a), Chain: []}
//	let b = &a           --> VarTaint{StorageTaint: scope(b), Chain: [{scope(a)}]}
//	let c = &mut b       --> VarTaint{StorageTaint: scope(c), Chain: [{scope(b)},{scope(a)}]}
//	let d = @arena Foo() --> VarTaint{StorageTaint: scope(d), Chain: [{arena}]}
type VarTaint struct {
	DiagNode     ast.NodeID // Node to blame in escape diagnostics.
	StorageTaint TaintID    // Taint of the scope that owns this variable's slot.
	Chain        Chain      // Per-depth storage the variable's value reaches.
}

// ScopeState is the analysis state for one lexical scope: the taint state of the
// variables declared in it plus the scope's own taint. The core escape rule
// reads off LocalTaints: a value leaving the scope (as a block result, return,
// or capture) escapes if it carries a taint in LocalTaints, i.e. a borrow of
// storage that dies when this scope ends.
type ScopeState struct {
	Scope       *ast.Scope
	Vars        map[string]*VarTaint // variable name --> its taint state here
	LocalTaints TaintSet             // taints whose storage dies when this scope ends
	ScopeTaint  TaintID              // this scope's own taint (its storage lifetime)
}

func (s *ScopeState) ID() ast.ScopeID { return s.Scope.ID }

func newScopeState(scope *ast.Scope, scopeTaint TaintID) *ScopeState {
	return &ScopeState{
		Scope:       scope,
		Vars:        map[string]*VarTaint{},
		LocalTaints: TaintSet{scopeTaint},
		ScopeTaint:  scopeTaint,
	}
}

// FunEffects describes how a function's parameters flow into its return value
// and into each other (side effects through &mut params).
//
//	fun foo(a &Int) &Int { a }
//	  --> ReturnTaints: [0]               (param 0 reaches the return value)
//
//	fun bar(a &mut Int, b &Int) void { a.* = b.* }
//	  --> SideEffects: {0: [1]}           (param 1 flows into param 0)
type FunEffects struct {
	ReturnTaints []int         // param indices that reach the return value
	SideEffects  map[int][]int // target param --> source params whose taints flow in
}

// analysisStatus is the per-node visitation state for the on-demand walk. Done
// results are cached; InProgress means the node is currently on the call stack,
// so a call that resolves back to it is a recursion cycle (handled pessimistically
// rather than looping forever).
type analysisStatus int

const (
	statusNotVisited analysisStatus = iota
	statusInProgress
	statusDone
)

type LifetimeCheck struct {
	Diagnostics    base.Diagnostics
	Debug          base.Debug
	ast            *ast.AST
	env            *TypeEnv
	scopeGraph     *ast.ScopeGraph
	nextTaintID    TaintID                       // next fresh param taint to hand out
	scopes         map[ast.ScopeID]*ScopeState   // analysis state per lexical scope
	scopeByTaint   map[TaintID]*ast.Scope        // scope taint --> its scope, for nesting checks
	chains         map[ast.NodeID]Chain          // each analyzed node's result: the storage its value reaches
	funEffects     map[ast.NodeID]*FunEffects    // each analyzed function's param-flow summary
	closureResults map[ast.NodeID]Chain          // closure Fun node --> chain its BODY returns
	taintOrigin    map[TaintID]ast.NodeID        // which &x created this taint (for blaming in diagnostics)
	status         map[ast.NodeID]analysisStatus // visitation state per node (cache + cycle guard)
	shapeContracts *shapeContractsCheck
	emittedEscape  map[escapeDiagKey]bool // spans already reported, to dedupe diagnostics

	// Per-instance effects for generic calls. The generic body analyzed with an
	// abstract type parameter loses borrows the parameter carries; re-analyzing
	// each concrete instantiation (its FunWork.Env binds T to e.g. &Int) fixes it.
	funWorkByType       map[TypeID]FunWork     // concrete instance type --> its monomorphization
	instanceEffectCache map[TypeID]*FunEffects // concrete instance type --> its effects
	instanceInProgress  map[TypeID]bool        // instances mid-analysis (cycle guard)
	analyzingFun        []ast.NodeID           // decls whose body is on the current analysis stack
	analyzingInstance   bool                   // true while re-analyzing a generic instance, not the decl
}

type escapeDiagKey struct {
	span   base.Span
	detail string
}

func NewLifetimeAnalyzer(a *ast.AST, g *ast.ScopeGraph, env *TypeEnv, funs []FunWork) *LifetimeCheck {
	funWorks := map[ast.NodeID][]FunWork{}
	funWorkByType := map[TypeID]FunWork{}
	for _, fw := range funs {
		funWorks[fw.NodeID] = append(funWorks[fw.NodeID], fw)
		funWorkByType[fw.TypeID] = fw
	}
	lc := &LifetimeCheck{
		Diagnostics:    nil,
		Debug:          base.NilDebug{},
		ast:            a,
		scopeGraph:     g,
		env:            env,
		nextTaintID:    1,
		scopes:         map[ast.ScopeID]*ScopeState{},
		scopeByTaint:   map[TaintID]*ast.Scope{},
		chains:         map[ast.NodeID]Chain{},
		funEffects:     map[ast.NodeID]*FunEffects{},
		closureResults: map[ast.NodeID]Chain{},
		taintOrigin:    map[TaintID]ast.NodeID{},
		status:         map[ast.NodeID]analysisStatus{},
		shapeContracts: nil,
		emittedEscape:  map[escapeDiagKey]bool{},

		funWorkByType:       funWorkByType,
		instanceEffectCache: map[TypeID]*FunEffects{},
		instanceInProgress:  map[TypeID]bool{},
		analyzingFun:        nil,
		analyzingInstance:   false,
	}
	lc.shapeContracts = &shapeContractsCheck{LifetimeCheck: lc, funWorks: funWorks}
	return lc
}

// VerifyShapeContracts checks that concrete implementations of shape methods
// don't violate the shape's effect contract. Must be called after Check().
func (a *LifetimeCheck) VerifyShapeContracts() {
	a.shapeContracts.verify()
}

// Check is the recursive entry point. It analyzes the subtree at nodeID,
// dispatching on node kind to the analyzeX helper that computes and stores the
// node's Chain (in a.chains). The status guard caches finished nodes and detects
// recursion cycles (see analysisStatus).
func (a *LifetimeCheck) Check(nodeID ast.NodeID) {
	if a.status[nodeID] == statusDone {
		return
	}
	a.status[nodeID] = statusInProgress
	defer func() { a.status[nodeID] = statusDone }()

	a.debug(2, nodeID, "analyze: %s", a.ast.Debug(nodeID, false, 0))
	defer a.Debug.Indent()()
	node := a.ast.Node(nodeID)
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
	case ast.TypeConstruction:
		a.analyzeTypeConstruction(nodeID, kind)
	case ast.ArrayLiteral:
		a.analyzeArrayLiteral(nodeID, kind)
	case ast.ArrayConstruction:
		a.analyzeArrayConstruction(nodeID, kind)
	case ast.EmptySlice:
		// No children to analyze.
	case ast.Index:
		a.analyzeIndex(nodeID, kind)
	case ast.SubSlice:
		a.analyzeSubSlice(nodeID, kind)
	case ast.Call:
		a.analyzeCall(nodeID, kind)
	case ast.For:
		a.analyzeFor(nodeID, kind)
	case ast.If:
		a.analyzeIf(nodeID, kind)
	case ast.When:
		a.analyzeWhen(nodeID, kind)
	case ast.Match:
		a.analyzeMatch(nodeID, kind)
	case ast.Var:
		a.analyzeVar(nodeID, kind)
	case ast.FunParam:
		a.analyzeFunParam(nodeID, kind)
	case ast.Assign:
		a.analyzeAssign(nodeID, kind)
	case ast.Return:
		a.analyzeReturn(nodeID, kind)
	case ast.Block:
		a.analyzeBlock(nodeID, kind)
	case ast.Fun:
		a.analyzeFun(nodeID, kind)
	default:
		a.ast.Walk(nodeID, a.Check)
	}
}

// analyzeRef: `&<target>` or `&mut <target>`. The result is placeChain(target):
//
//	mut a = 1; &a        --> [{scope(a)}]
//	mut z = &mut y; &z   --> [{scope(z)}, {scope(y)}]  (z.chain = [{scope(y)}])
func (a *LifetimeCheck) analyzeRef(nodeID ast.NodeID, ref ast.Ref) {
	a.ast.Walk(nodeID, a.Check)
	chain := a.refChain(ref.Target, nodeID)
	a.chains[nodeID] = chain
	a.debug(1, nodeID, "analyzeRef: %s", chain)
}

// refChain is the chain carried by `&target`/`&mut target` taken at refNode:
// chain[0] is the slot the new reference points at, chain[1:] is whatever that
// slot's value reaches onward. `&temp` materializes the temporary into a fresh
// slot in refNode's scope.
func (a *LifetimeCheck) refChain(target, refNode ast.NodeID) Chain {
	chain := a.placeChain(target)
	if isTemporaryExpr(a.ast.Node(target).Kind) {
		chain = Chain{TaintSet{a.scopeState(refNode).ScopeTaint}}
	}
	for _, t := range chain.HeadTaintSet() {
		if _, ok := a.taintOrigin[t]; !ok {
			a.taintOrigin[t] = refNode
		}
	}
	return chain
}

// projection classifies `x.f` / `x[i]`: it returns the container, whether the
// container is reached through a ref (so the projection already points into the
// referent), and ok=false for non-projections and for module member accesses
// (which have no lifetime). placeChain and storageScopes share this so the
// ref-vs-value branch is written once.
func (a *LifetimeCheck) projection(nodeID ast.NodeID) (target ast.NodeID, throughRef, ok bool) {
	switch kind := a.ast.Node(nodeID).Kind.(type) {
	case ast.FieldAccess:
		if a.isModuleFieldAccess(nodeID, kind) {
			return 0, false, false
		}
		target = kind.Target
	case ast.Index:
		target = kind.Target
		// A slice value is itself a pointer to its data, so `s[i]` projects into
		// the referent (like indexing through a ref), not into the slice's own
		// storage. An array stores its elements inline, so `a[i]` stays in the
		// array's storage and is not through-ref.
		if _, isSlice := a.env.TypeOfNode(target).Kind.(SliceType); isSlice {
			return target, true, true
		}
	default:
		return 0, false, false
	}
	_, throughRef = a.env.TypeOfNode(target).Kind.(RefType)
	return target, throughRef, true
}

// placeChain returns the chain that `&place` carries: chain[0] is the slot the
// new ref points at, chain[1:] is whatever that slot's value reaches onward.
//
//	x          (var)      --> x.chain.Prepend({scope(x)})
//	x.f / x[i] (x value)  --> root.chain.Prepend({storageScope(root)})
//	x.f / x[i] (x a ref)  --> x.chain (the ref already points into the referent)
//	x.*                   --> x.chain.DropHead() (one level past x's referent)
func (a *LifetimeCheck) placeChain(nodeID ast.NodeID) Chain {
	if target, throughRef, ok := a.projection(nodeID); ok {
		// `&p.f` through a ref points at storage in the referent, same depth-0
		// as p points at, and reaches whatever the referent reaches: p.chain.
		if throughRef {
			return a.flow(target)
		}
		return a.placeChain(target)
	}
	switch kind := a.ast.Node(nodeID).Kind.(type) {
	case ast.Ident:
		vt := a.lookupVar(nodeID, kind.Name)
		if vt == nil {
			return nil
		}
		return vt.Chain.Prepend(TaintSet{vt.StorageTaint})
	case ast.Deref:
		// `&x.*` points one level past x's referent: x.chain.DropHead().
		return a.flow(kind.Expr).DropHead()
	default:
		return nil
	}
}

// storageScopes returns the TaintSet of the slot a write to `place` targets.
//
//	x          (var)      --> {scope(x)}
//	x.f / x[i] (x value)  --> storageScopes(container) (same slot as container)
//	x.f / x[i] (x a ref)  --> chain(ref)[0] (the slot the ref points at)
//	x.*  (Deref(p))       --> chain(p)[0]. So w.*.* = Deref(Deref(w)) targets
//	                          chain(Deref(w))[0] = chain(w)[1], the depth-2 slot.
func (a *LifetimeCheck) storageScopes(nodeID ast.NodeID) TaintSet {
	if target, throughRef, ok := a.projection(nodeID); ok {
		if throughRef {
			return a.flow(target).HeadTaintSet()
		}
		return a.storageScopes(target)
	}
	switch kind := a.ast.Node(nodeID).Kind.(type) {
	case ast.Ident:
		vt := a.lookupVar(nodeID, kind.Name)
		if vt == nil {
			return nil
		}
		return TaintSet{vt.StorageTaint}
	case ast.Deref:
		return a.flow(kind.Expr).HeadTaintSet()
	case ast.ArrayConstruction:
		// A `[N of v]`/`[N uninit T]` temporary is a fresh stack array owned by
		// this scope, so a slice into it cannot outlive the scope.
		return TaintSet{a.scopeState(nodeID).ScopeTaint}
	case ast.ArrayLiteral:
		// A const literal is promoted to a global, so a slice into it is unscoped.
		// A non-const (runtime-valued) literal is a fresh stack array like the
		// `[N of v]` case above, so a slice into it cannot outlive the scope.
		if a.env.IsConstArray(nodeID) {
			return nil
		}
		return TaintSet{a.scopeState(nodeID).ScopeTaint}
	default:
		return nil
	}
}

// analyzeIdent: reading a variable yields its value chain.
func (a *LifetimeCheck) analyzeIdent(nodeID ast.NodeID, ident ast.Ident) {
	a.ast.Walk(nodeID, a.Check)
	vt := a.lookupVar(nodeID, ident.Name)
	if vt == nil {
		return
	}
	a.chains[nodeID] = vt.Chain
	a.debug(1, nodeID, "analyzeIdent: %s %s", ident.Name, vt.Chain)
}

// analyzeDeref: `x.*` drops x's head so chain[d] becomes x's depth-(d+1)
// storage, gated by the result type so a scalar deref carries nothing:
//
//	sum + r.*   (r &Int)         --> [] : reading the Int does not poison sum
//	w.* (w &mut &Int)            --> w.chain.DropHead() : still a ref chain
func (a *LifetimeCheck) analyzeDeref(nodeID ast.NodeID, deref ast.Deref) {
	a.ast.Walk(nodeID, a.Check)
	if !a.nodeCanEscape(nodeID) {
		a.chains[nodeID] = nil
		a.debug(1, nodeID, "analyzeDeref: [] (gated)")
		return
	}
	chain := a.flow(deref.Expr).DropHead()
	a.chains[nodeID] = chain
	a.debug(1, nodeID, "analyzeDeref: %s", chain)
}

// nodeCanEscape reports whether the value at nodeID has a type that can carry a
// reference out (ref/alloc/slice/ffi ptr), OR is a closure that may carry
// captured borrows. The type gate that makes a scalar read (`sum + x.f`, `r.*`
// of an Int) carry nothing. A FunType is added on top of typeCanEscape (which is
// left alone, the noescape checks depend on its current behavior) so that
// reading a closure out of a struct field or array element carries the
// container's chain (the captures live there).
//
//	struct Holder { f fun() &Int }; let fn = h.f
//	  --> h.f carries Holder's chain, which holds the captured borrow.
func (a *LifetimeCheck) nodeCanEscape(nodeID ast.NodeID) bool {
	resultType := a.env.TypeOfNode(nodeID)
	if resultType == nil {
		return false
	}
	return a.typeCanEscape(resultType.ID) || a.typeReachesClosure(resultType.ID)
}

// typeReachesClosure reports whether typeID IS a function type or transitively
// CONTAINS one (a closure stored in a struct field, union variant, array or
// slice element). typeCanEscape is left alone (the noescape checks depend on its
// behavior), so this is the extra gate that lets a struct/array READ which holds
// a closure carry the container's chain, where the captures live.
//
//	struct Holder { f fun() &Int }; let fn = h.f
//	  --> h.f reaches a closure, so it carries Holder's chain (the captured borrow).
func (a *LifetimeCheck) typeReachesClosure(typeID TypeID) bool {
	if typeID == InvalidTypeID {
		return false
	}
	switch kind := a.env.Type(typeID).Kind.(type) {
	case FunType:
		return true
	case StructType:
		for _, field := range kind.Fields {
			if a.typeReachesClosure(field.Type) {
				return true
			}
		}
	case UnionType:
		return slices.ContainsFunc(kind.Variants, a.typeReachesClosure)
	case ArrayType:
		return a.typeReachesClosure(kind.Elem)
	case SliceType:
		return a.typeReachesClosure(kind.Elem)
	}
	return false
}

// typeContainsTypeParam reports whether typeID is, or transitively contains, a
// type parameter (an abstract `T`). It gates the "meaningless noescape" check:
// `noescape T` or `noescape ?T` is left alone because it MIGHT carry a reference
// once T is bound (T = &Int), even though some instantiations (T = Int) make it
// carry nothing. Only fully concrete types are judged.
func (a *LifetimeCheck) typeContainsTypeParam(typeID TypeID) bool {
	if typeID == InvalidTypeID {
		return false
	}
	switch kind := a.env.Type(typeID).Kind.(type) {
	case TypeParamType:
		return true
	case RefType:
		return a.typeContainsTypeParam(kind.Type)
	case StructType:
		return slices.ContainsFunc(kind.TypeArgs, a.typeContainsTypeParam) ||
			slices.ContainsFunc(kind.Fields, func(f StructField) bool {
				return a.typeContainsTypeParam(f.Type)
			})
	case UnionType:
		return slices.ContainsFunc(kind.TypeArgs, a.typeContainsTypeParam) ||
			slices.ContainsFunc(kind.Variants, a.typeContainsTypeParam)
	case ArrayType:
		return a.typeContainsTypeParam(kind.Elem)
	case SliceType:
		return a.typeContainsTypeParam(kind.Elem)
	case FunType:
		return slices.ContainsFunc(kind.Params, a.typeContainsTypeParam) ||
			a.typeContainsTypeParam(kind.Return)
	}
	return false
}

// analyzeFieldAccess: `x.f`. Type-gated like deref: a scalar field read carries
// nothing, a ref/alloc-typed field read carries the container's taints. Module
// member accesses have no lifetime.
func (a *LifetimeCheck) analyzeFieldAccess(nodeID ast.NodeID, fa ast.FieldAccess) {
	a.ast.Walk(nodeID, a.Check)
	if a.isModuleFieldAccess(nodeID, fa) {
		return
	}
	chain := a.gatedRead(nodeID, fa.Target)
	a.chains[nodeID] = chain
	a.debug(1, nodeID, "analyzeFieldAccess: .%s %s", fa.Field.Name, chain)
}

// gatedRead implements the TYPE-GATED read for `x.f`, `x[i]`. If the result
// type can escape (ref/alloc/slice/ffi ptr), the read yields the container's
// chain (conservative: the field reaches whatever the container reaches); a
// scalar read yields []. This keeps scalar reads (`sum + x.f`) from poisoning
// accumulators while still tracking borrowed storage that leaves through a
// slice or ref field.
func (a *LifetimeCheck) gatedRead(resultNodeID, containerNodeID ast.NodeID) Chain {
	if !a.nodeCanEscape(resultNodeID) {
		return nil
	}
	return a.flow(containerNodeID)
}

// isModuleFieldAccess returns true if the FieldAccess targets a module.
// Uses scope lookup rather than TypeOfNode because module ident nodes
// may not have a cached type (e.g. inside struct field types resolved
// in a child environment).
func (a *LifetimeCheck) isModuleFieldAccess(_ ast.NodeID, fa ast.FieldAccess) bool {
	switch target := a.ast.Node(fa.Target).Kind.(type) {
	case ast.Ident:
		if binding, ok := a.env.Lookup(fa.Target, target.Name, -1); ok {
			if _, isMod := a.env.Type(binding.TypeID).Kind.(ModuleType); isMod {
				return true
			}
		}
		return false
	case ast.FieldAccess:
		return a.isModuleFieldAccess(fa.Target, target)
	default:
		return false
	}
}

func (a *LifetimeCheck) analyzeAllocatorVar(nodeID ast.NodeID, alloc ast.AllocatorVar) {
	a.ast.Walk(nodeID, a.Check)
	ss := a.scopeState(nodeID)
	// A freshly constructed arena (`let @a = Arena()`) is owned by this scope.
	// An aliased allocator (`let @b = h.@a`) borrows storage that lives
	// elsewhere, so it inherits the source's chain.
	ss.Vars[alloc.Name.Name] = &VarTaint{nodeID, ss.ScopeTaint, a.flow(alloc.Expr)}
}

// isArenaConstruction reports whether the expression freshly constructs an
// allocator (e.g. `Arena()`) rather than aliasing an existing one.
func (a *LifetimeCheck) isArenaConstruction(nodeID ast.NodeID) bool {
	if _, ok := a.ast.Node(nodeID).Kind.(ast.TypeConstruction); !ok {
		return false
	}
	typ := a.env.TypeOfNode(nodeID)
	if typ == nil {
		return false
	}
	_, ok := typ.Kind.(AllocatorType)
	return ok
}

// analyzeTypeConstruction: `Foo(a, b)` unions all argument chains per depth.
func (a *LifetimeCheck) analyzeTypeConstruction(nodeID ast.NodeID, lit ast.TypeConstruction) {
	a.ast.Walk(nodeID, a.Check)
	// A fresh arena lives in the scope where it is constructed.
	if a.isArenaConstruction(nodeID) {
		ss := a.scopeState(nodeID)
		a.chains[nodeID] = Chain{TaintSet{ss.ScopeTaint}}
		a.debug(1, nodeID, "analyzeTypeConstruction (arena): %s", a.chains[nodeID])
		return
	}
	// A named construction's arguments are reordered into field order.
	args := lit.Args
	if order, ok := a.env.ArgOrder(nodeID); ok {
		args = order
	}
	merged := a.mergeFlows(args)
	// Check noescape for struct fields that are function types.
	resultType := a.env.TypeOfNode(nodeID)
	if resultType != nil {
		if st, ok := resultType.Kind.(StructType); ok {
			for i, argNodeID := range args {
				if i < len(st.Fields) {
					a.checkNoescapeValueAssignment(argNodeID, st.Fields[i].Type)
				}
			}
		}
	}
	a.chains[nodeID] = merged
	a.debug(1, nodeID, "analyzeTypeConstruction: %s", merged)
}

func (a *LifetimeCheck) isArenaAllocCall(nodeID ast.NodeID) bool {
	call, ok := a.ast.Node(nodeID).Kind.(ast.Call)
	if !ok {
		return false
	}
	fa, ok := a.ast.Node(call.Callee).Kind.(ast.FieldAccess)
	if !ok {
		return false
	}
	targetType := a.env.TypeOfNode(fa.Target)
	_, ok = targetType.Kind.(AllocatorType)
	return ok
}

// analyzeArenaAllocCall: `@a.new<T>(v)` / `@a.slice(...)` borrows @a's storage
// plus the storage of any arg that itself carries refs/allocs. Scalar args
// (e.g. a size) do not contribute.
func (a *LifetimeCheck) analyzeArenaAllocCall(nodeID ast.NodeID, call ast.Call) {
	fa := base.Cast[ast.FieldAccess](a.ast.Node(call.Callee).Kind)
	result := a.flow(fa.Target)
	for _, argNodeID := range call.Args {
		if a.typeContainsRefOrAlloc(a.env.TypeOfNode(argNodeID).ID) {
			result = result.Merge(a.flow(argNodeID))
		}
	}
	a.chains[nodeID] = result
	a.debug(1, nodeID, "analyzeArenaAllocCall: %s", result)
}

func (a *LifetimeCheck) analyzeArrayLiteral(nodeID ast.NodeID, lit ast.ArrayLiteral) {
	a.ast.Walk(nodeID, a.Check)
	merged := a.mergeFlows(lit.Elems)
	a.chains[nodeID] = merged
	a.debug(1, nodeID, "analyzeArrayLiteral: %s", merged)
}

// analyzeArrayConstruction: `[N of v]` carries v's chain (every element is a
// copy of v); `[N uninit T]` has no value and so no chain. The fresh array's
// own stack storage is scoped where it is built (see storageScopes), so only a
// slice into it is scope-bound; the array used by value is freely copyable.
func (a *LifetimeCheck) analyzeArrayConstruction(nodeID ast.NodeID, ac ast.ArrayConstruction) {
	a.ast.Walk(nodeID, a.Check)
	if ac.Fill != nil {
		a.chains[nodeID] = a.mergeFlows([]ast.NodeID{*ac.Fill})
	}
	a.debug(1, nodeID, "analyzeArrayConstruction: %s", a.chains[nodeID])
}

// analyzeIndex: `arr[i]` reads an element. Type-gated like a field read.
func (a *LifetimeCheck) analyzeIndex(nodeID ast.NodeID, index ast.Index) {
	a.ast.Walk(nodeID, a.Check)
	chain := a.gatedRead(nodeID, index.Target)
	a.chains[nodeID] = chain
	a.debug(1, nodeID, "analyzeIndex: %s", chain)
}

func (a *LifetimeCheck) isReferenceType(typeID TypeID) bool {
	if typeID == InvalidTypeID {
		return false
	}
	typ := a.env.Type(typeID)
	switch typ.Kind.(type) {
	case RefType, SliceType:
		return true
	}
	return false
}

// analyzeSubSlice: `arr[lo..hi]` produces a fat pointer into the target's
// storage. Subslicing a slice/ref does NOT add the local storage scope (the
// result points at the same data); subslicing a value array does.
func (a *LifetimeCheck) analyzeSubSlice(nodeID ast.NodeID, sub ast.SubSlice) {
	a.ast.Walk(nodeID, a.Check)
	chain := a.flow(sub.Target)

	// Only a value array contributes its own storage scope at depth 0.
	//   let s = arr[1..3]   (arr a value array) --> [{scope(arr)}] ++ arr.chain
	//   slice[1..3]         (slice a ref)        --> what slice borrowed
	if !a.isReferenceType(a.env.TypeOfNode(sub.Target).ID) {
		chain = chain.Prepend(a.storageScopes(sub.Target))
	}
	for _, t := range chain.CarriedTaints() {
		if _, ok := a.taintOrigin[t]; !ok {
			a.taintOrigin[t] = nodeID
		}
	}
	a.chains[nodeID] = chain
	a.debug(1, nodeID, "analyzeSubSlice: %s target=%s", chain, a.ast.Debug(sub.Target, false, 0))
}

// instanceEffects returns the effects of a generic call's CONCRETE instantiation.
// The generic body analyzed with an abstract type parameter drops borrows the
// parameter carries (e.g. `Option.or_err` returns its T, but for abstract T
// typeContainsRefOrAlloc(T) is false so the borrow is lost). Re-analyzing the
// instance with its concrete env (which binds T to e.g. &Int) restores them.
// Returns nil for a non-generic call (use the generic path) or on a cycle.
//
//	or_err<&Int>(opt, err)  --> analyzed with T = &Int, so ReturnTaints = [0]
func (a *LifetimeCheck) instanceEffects(typeID TypeID) *FunEffects {
	if _, isInstance := a.env.GenericOrigin(typeID); !isInstance {
		return nil
	}
	if eff, cached := a.instanceEffectCache[typeID]; cached {
		return eff
	}
	fw, ok := a.funWorkByType[typeID]
	if !ok {
		return nil
	}
	fun, isFun := a.ast.Node(fw.NodeID).Kind.(ast.Fun)
	if !isFun || fun.Builtin || fun.Extern {
		return nil
	}
	if a.instanceInProgress[typeID] {
		return nil // cycle: the caller falls back to pessimistic effects
	}
	a.instanceInProgress[typeID] = true
	prevEnv := a.env
	prevInstance := a.analyzingInstance
	a.env = fw.Env
	a.analyzingInstance = true
	a.resetAnalysisStatus(fw.NodeID)
	a.analyzeFun(fw.NodeID, fun)
	a.analyzingInstance = prevInstance
	a.env = prevEnv
	delete(a.instanceInProgress, typeID)

	eff := a.funEffects[fw.NodeID]
	a.instanceEffectCache[typeID] = eff
	return eff
}

// analyzeCall applies the callee's FunEffects to map argument taints into the
// call result and into side-effected arguments.
//
// If the function hasn't been analyzed yet, we analyze it on demand.
// If we detect a cycle (mutual recursion), we apply pessimistic effects.
func (a *LifetimeCheck) analyzeCall(nodeID ast.NodeID, call ast.Call) { //nolint:funlen
	a.ast.Walk(nodeID, a.Check)
	// A method receiver passed by value to a `&`/`&mut` parameter is implicitly
	// borrowed: analyze it as `&receiver` so the call's effects see the borrow at
	// the right depth (a returned `&self.field` stays bounded by the receiver's
	// scope, and a write through `&mut self` accumulates into the receiver).
	if receiver, ok := a.env.MethodCallReceiver(nodeID); ok {
		if _, autoRef := a.env.MethodReceiverAutoRef(nodeID); autoRef {
			a.chains[receiver] = a.refChain(receiver, receiver)
		}
	}
	if a.isArenaAllocCall(nodeID) {
		a.analyzeArenaAllocCall(nodeID, call)
		return
	}
	calleeType := a.env.TypeOfNode(call.Callee)
	if _, ok := calleeType.Kind.(FunType); !ok {
		return
	}
	if ref, ok := a.env.NamedFunRef(call.Callee); ok {
		if effects := BuiltinFunEffects(BuiltinName(ref)); effects != nil {
			a.applyBuiltinEffects(nodeID, *effects)
			return
		}
	}

	effectsTypeID := calleeType.ID
	if origin, ok := a.env.GenericOrigin(effectsTypeID); ok {
		effectsTypeID = origin
	}
	declID, _ := a.env.FunDeclNode(call.Callee)
	if declID == 0 {
		declID = a.env.DeclNode(effectsTypeID)
	}
	// If we still can't find the declaration, try looking up the binding
	// by name (handles shape method calls like S.do).
	if declID == 0 {
		if ref, ok := a.env.NamedFunRef(call.Callee); ok {
			if binding, ok := a.env.Lookup(call.Callee, ref, -1); ok {
				declID = binding.Decl
			}
		}
	}
	// A call resolving to a decl whose body is on the analysis stack is recursion
	// (direct or mutual): its effects aren't computed yet, so fall back to
	// pessimistic effects rather than recursing forever.
	recursive := slices.Contains(a.analyzingFun, declID)
	// For a generic call, use the CONCRETE instantiation's effects: the generic
	// body analyzed with an abstract type parameter drops borrows the parameter
	// carries. Falls back to the generic path for non-generic calls.
	effects := a.instanceEffects(calleeType.ID)
	ok := effects != nil
	if !ok {
		effects, ok = a.funEffects[declID]
	}
	if !ok && declID != 0 {
		// Shape method declarations (FunDecl) have no body to analyze.
		// Use the shape's expected effects contract instead.
		if funDecl, isFunDecl := a.ast.Node(declID).Kind.(ast.FunDecl); isFunDecl {
			effects = a.shapeContracts.expectedEffects(declID, funDecl, base.Cast[FunType](calleeType.Kind))
			ok = true
		} else if recursive {
			a.debug(1, nodeID, "analyzeCall: cycle detected, pessimistic fallback")
			a.applyPessimisticEffects(nodeID, call)
			return
		} else {
			a.Check(declID)
			effects, ok = a.funEffects[declID]
		}
	}
	if !ok {
		a.applyPessimisticEffects(nodeID, call)
		return
	}

	args := a.env.CallArgNodes(nodeID)

	// Check noescape compatibility for function-typed arguments.
	calleeFun := base.Cast[FunType](calleeType.Kind)
	for i, argNodeID := range args {
		if i < len(calleeFun.Params) {
			a.checkNoescapeValueAssignment(argNodeID, calleeFun.Params[i])
		}
	}

	// Map param chains into the return value (per-depth union).
	var result Chain
	for _, paramIdx := range effects.ReturnTaints {
		result = result.Merge(a.flow(args[paramIdx]))
	}
	// Apply side effects: write each source arg through its target arg.
	//   fun foo(a &mut Foo, b &Int) { a.one = b }
	//   --> foo(&mut y, &z) checks &z against y's storage and merges it in.
	for targetIdx, srcIndices := range effects.SideEffects {
		var srcChain Chain
		for _, srcIdx := range srcIndices {
			srcChain = srcChain.Merge(a.flow(args[srcIdx]))
		}
		a.writeThroughArg(nodeID, args[targetIdx], srcChain)
	}
	// If the callee returns noescape, the result must not escape the scope where
	// the call was made: tag it with that scope's taint (a LocalTaint), so it is
	// caught the moment it reaches a longer-lived place (an outer binding, a
	// store, a return). Flowing DOWN into child scopes or callees is fine. Only
	// meaningful when the return can carry a reference; on a value return
	// (`noescape Int`) nothing is reachable to dangle.
	if calleeFun.NoescapeReturn && a.typeCanEscape(calleeFun.Return) {
		result = result.Merge(Chain{TaintSet{a.scopeState(nodeID).ScopeTaint}})
	}
	// Carry whatever the callee contributes through its OWN chain (not via any
	// argument): the captures a closure returns, or a fun-typed param's
	// caller-taint. See closureCallContribution for the precise-vs-fallback split.
	//   fun bar() &Int { mut local = 99  let g = fun[&local]() &Int { local }  g() }
	// g()'s result carries g's returned capture of `local`, so the dangle is caught.
	result = result.Merge(a.closureCallContribution(nodeID, call.Callee, declID, calleeFun.Return))
	a.chains[nodeID] = result
	a.debug(1, nodeID, "analyzeCall: %s", result)
}

// closureCallContribution returns what a call carries through its CALLEE chain,
// separate from any argument. Two cases:
//
//   - The callee resolves to a known closure Fun node (declID has a
//     closureResults entry): carry the PRECISE chain its body returns, so an
//     unreturned capture is not falsely flagged.
//     fun bar(p &Int) &Int { mut local = 99  let g = fun[&local, p]() &Int { p }  g() }
//     g returns p, not local, so g() carries only p's reach: no escape.
//   - The callee is hidden behind a projection (`h.f`, `arr[i]`) or is a
//     fun-typed param, so declID is not a known closure: conservatively carry
//     flow(callee) when the return type can escape. For a projection that is the
//     container's reach (which holds the captures, since nodeCanEscape lets a
//     closure read carry them); for a fun-typed param it is the caller-taint that
//     makes higher-order functions propagate.
//
// The fallback is skipped for METHOD calls: there the receiver is an explicit
// argument already routed through effects.ReturnTaints, and the method-binding
// callee (`r.read`) is a FieldAccess that nodeCanEscape now lets carry the
// receiver's chain, so merging flow(callee) would double-count the receiver and
// wrongly drag its taint into the result.
//
//	let read = try r.read(buf[0..n])   -- r.read is a method binding, not a closure read.
func (a *LifetimeCheck) closureCallContribution(callID, callee, declID ast.NodeID, calleeReturn TypeID) Chain {
	if chain, ok := a.closureResults[declID]; ok {
		return chain
	}
	if _, isMethod := a.env.MethodCallReceiver(callID); isMethod {
		return nil
	}
	if a.typeCanEscape(calleeReturn) {
		return a.flow(callee)
	}
	return nil
}

// applyBuiltinEffects applies pre-defined lifetime effects for a builtin function call.
func (a *LifetimeCheck) applyBuiltinEffects(nodeID ast.NodeID, effects FunEffects) {
	args := a.env.CallArgNodes(nodeID)
	var result Chain
	for _, paramIdx := range effects.ReturnTaints {
		result = result.Merge(a.flow(args[paramIdx]))
	}
	a.chains[nodeID] = result
}

// applyPessimisticEffects: assume every arg flows into the return value and
// into every &mut arg. Used when we can't determine the actual effects.
func (a *LifetimeCheck) applyPessimisticEffects(nodeID ast.NodeID, call ast.Call) {
	args := a.env.CallArgNodes(nodeID)
	allArgs := a.mergeFlows(args)
	funType := base.Cast[FunType](a.env.TypeOfNode(call.Callee).Kind)
	if a.typeContainsRefOrAlloc(funType.Return) {
		result := allArgs
		if funType.NoescapeReturn && a.typeCanEscape(funType.Return) {
			result = result.Merge(Chain{TaintSet{a.scopeState(nodeID).ScopeTaint}})
		}
		// Carry closure captures / fun-typed-param caller-taints through the
		// call (see closureCallContribution). Empty for ordinary named/method calls.
		declID, _ := a.env.FunDeclNode(call.Callee)
		result = result.Merge(a.closureCallContribution(nodeID, call.Callee, declID, funType.Return))
		a.chains[nodeID] = result
	}
	for _, arg := range args {
		if ref, ok := a.env.TypeOfNode(arg).Kind.(RefType); ok && ref.Mut {
			a.writeThroughArg(nodeID, arg, allArgs)
		}
	}
}

// writeThroughArg applies the WRITE RULE for a function side effect: the source
// chain is written through a pointer argument (a `&mut` arg). The storage
// written into is what the arg points at (chain(arg)[0]); the accumulation
// goes into the variable the arg references.
//
//	fun foo(a &mut Foo, b &Int) { a.one = b }
//	foo(&mut y, &z)   --> &z checked against scope(y), then merged into y
func (a *LifetimeCheck) writeThroughArg(nodeID, arg ast.NodeID, src Chain) {
	a.checkEscape(nodeID, src.CarriedTaints(), a.flow(arg).HeadTaintSet(), "via mutation of outer variable")
	// Peel a leading `&`/`&mut` to find the referenced variable to accumulate
	// into. A computed pointer (e.g. `identity(&mut y)`) has no root ident, so
	// the escape check above is the only effect.
	target := arg
	if ref, ok := a.ast.Node(arg).Kind.(ast.Ref); ok {
		target = ref.Target
	}
	a.accumulateIntoRoot(nodeID, a.placeRoot(target), src)
}

// analyzeMatch: the result is the union of the arm bodies (like analyzeIf). A
// `case T v:` binding makes v carry the matched value's reach, so unwrapping a ref
// out of an optional/union keeps the borrow (and a later escape of v is caught).
func (a *LifetimeCheck) analyzeMatch(nodeID ast.NodeID, match ast.Match) {
	a.Check(match.Expr)
	exprChain := a.flow(match.Expr)
	for _, arm := range match.Arms {
		for _, p := range arm.Patterns {
			a.Check(p)
		}
		if arm.Binding != nil {
			ss := a.scopeFor(a.scopeGraph.IntroducedScope(arm.Body))
			// A reference binding (`case Foo &x`) aliases the matched value's storage,
			// so it always carries the matched value's reach. A value binding only
			// propagates taint when a matched variant type can itself carry a borrow
			// (a ref, allocator, or raw ffi pointer): `case Err e:` on a union that
			// also holds `&mut File` must not taint `e`, but a value that borrows an
			// arena through an ffi pointer must keep that borrow when unwrapped.
			bindingChain := exprChain
			if !arm.Ref {
				carries := false
				for _, p := range arm.Patterns {
					if pt := a.env.TypeOfNode(p); pt != nil && a.typeContainsRefAllocOrFfiPtr(pt.ID) {
						carries = true
						break
					}
				}
				if !carries {
					bindingChain = nil
				}
			}
			ss.Vars[arm.Binding.Name] = &VarTaint{arm.Body, ss.ScopeTaint, bindingChain}
		}
		if arm.Guard != nil {
			a.Check(*arm.Guard)
		}
		a.Check(arm.Body)
	}
	if match.Else != nil {
		if match.Else.Binding != nil {
			ss := a.scopeFor(a.scopeGraph.IntroducedScope(match.Else.Body))
			// A reference else binding aliases the matched value; otherwise only
			// propagate taint if an uncovered variant can carry references.
			bindingChain := a.elseBindingChain(match, exprChain)
			if match.Else.Ref {
				bindingChain = exprChain
			}
			ss.Vars[match.Else.Binding.Name] = &VarTaint{match.Else.Body, ss.ScopeTaint, bindingChain}
		}
		a.Check(match.Else.Body)
	}
	// A diverging arm (`return`/`break`/`panic`) never yields a value through the
	// match, so its borrows do not flow to the match result. Dropping it mirrors
	// the type checker, which excludes `never` arms from the result type, and
	// avoids a redundant "via block result" escape alongside the real "via return".
	var merged Chain
	for _, arm := range match.Arms {
		if a.bodyDiverges(arm.Body) {
			continue
		}
		merged = merged.Merge(a.flow(arm.Body))
	}
	if match.Else != nil && !a.bodyDiverges(match.Else.Body) {
		merged = merged.Merge(a.flow(match.Else.Body))
	}
	a.chains[nodeID] = merged
	a.debug(1, nodeID, "analyzeMatch: %s", merged)
}

// bodyDiverges reports whether a branch body is `never`-typed, i.e. it returns,
// breaks, or panics and so contributes no value (and no borrow) to its
// enclosing expression's result.
func (a *LifetimeCheck) bodyDiverges(nodeID ast.NodeID) bool {
	t := a.env.TypeOfNode(nodeID)
	if t == nil {
		return false
	}
	_, ok := t.Kind.(NeverType)
	return ok
}

func (a *LifetimeCheck) elseBindingChain(match ast.Match, exprChain Chain) Chain {
	exprType := a.env.TypeOfNode(match.Expr)
	if exprType == nil {
		return exprChain
	}
	union, ok := exprType.Kind.(UnionType)
	if !ok {
		return exprChain
	}
	covered := make([]bool, len(union.Variants))
	for _, arm := range match.Arms {
		for _, p := range arm.Patterns {
			if pt := a.env.TypeOfNode(p); pt != nil {
				for i, v := range union.Variants {
					if v == pt.ID {
						covered[i] = true
					}
				}
			}
		}
	}
	if slices.ContainsFunc(uncoveredVariants(covered, union), a.typeContainsRefAllocOrFfiPtr) {
		return exprChain
	}
	return nil
}

// analyzeIf: an `if` used as an expression yields a value from one branch or the
// other, so conservatively its chain is the union (Merge) of the branch chains.
// A diverging branch yields no value, so it does not contribute (see analyzeMatch).
func (a *LifetimeCheck) analyzeIf(nodeID ast.NodeID, ifNode ast.If) {
	a.ast.Walk(nodeID, a.Check)
	var merged Chain
	if !a.bodyDiverges(ifNode.Then) {
		merged = a.flow(ifNode.Then)
	}
	if ifNode.Else != nil && !a.bodyDiverges(*ifNode.Else) {
		merged = merged.Merge(a.flow(*ifNode.Else))
	}
	a.chains[nodeID] = merged
	a.debug(1, nodeID, "analyzeIf: %s", merged)
}

// analyzeWhen: like analyzeIf, the result can come from any case (or the else),
// so its chain is the union of all the non-diverging branch chains.
func (a *LifetimeCheck) analyzeWhen(nodeID ast.NodeID, when ast.When) {
	a.ast.Walk(nodeID, a.Check)
	var merged Chain
	for _, case_ := range when.Cases {
		if a.bodyDiverges(case_.Body) {
			continue
		}
		merged = merged.Merge(a.flow(case_.Body))
	}
	if when.Else != nil && !a.bodyDiverges(*when.Else) {
		merged = merged.Merge(a.flow(*when.Else))
	}
	a.chains[nodeID] = merged
	a.debug(1, nodeID, "analyzeWhen: %s", merged)
}

// analyzeFor: `for cond { body }`. The for-binding (and optional element ref
// for `for &x in ...`) lives in the body scope. Writes to outer variables are
// caught by the write rule at the assignment site; moving the binding out of
// the loop is caught by the block-result / return checks.
func (a *LifetimeCheck) analyzeFor(nodeID ast.NodeID, forNode ast.For) {
	if forNode.Binding != nil {
		forScope := a.scopeState(forNode.Body)
		var bindChain Chain
		if forNode.Ref {
			// `for &x in xs` binds x to `&elem`: it borrows where elem lives,
			// a fresh body-local slot. Reading x stays local; moving it out
			// of the loop is an escape.
			bindChain = Chain{TaintSet{forScope.ScopeTaint}}
		}
		forScope.Vars[forNode.Binding.Name] = &VarTaint{nodeID, forScope.ScopeTaint, bindChain}
		if forNode.Index != nil {
			forScope.Vars[forNode.Index.Name] = &VarTaint{nodeID, forScope.ScopeTaint, nil}
		}
	}
	a.ast.Walk(nodeID, a.Check)
}

// analyzeVar: `let x = expr` or `mut x = expr`. The variable's slot lives in
// this scope; its value carries whatever expr borrows. Heap allocs are not
// special: the value carries the arena's scope taint, the slot carries this
// scope's taint.
func (a *LifetimeCheck) analyzeVar(nodeID ast.NodeID, varNode ast.Var) {
	a.ast.Walk(nodeID, a.Check)
	ss := a.scopeState(nodeID)
	chain := a.flow(varNode.Expr)
	// Check noescape when the binding has an explicit function type with noescape.
	if varNode.Type != nil {
		if declType := a.env.TypeOfNode(*varNode.Type); declType != nil {
			a.checkNoescapeValueAssignment(varNode.Expr, declType.ID)
		}
	}
	ss.Vars[varNode.Name.Name] = &VarTaint{nodeID, ss.ScopeTaint, chain}
	a.debug(1, nodeID, "analyzeVar: %s scope=%s %s", varNode.Name.Name, ss.ID(), chain)
}

// analyzeFunParam: a param whose type can escape (ref, alloc, slice, ffi ptr)
// gets a unique caller PARAM-TAINT (distinct from the function body's scope
// taint). A param-taint is not any scope's LocalTaint, so a value carrying it
// never escapes inside the body; escapes are only decided at call sites. Slices
// get a param-taint so returning one is caught by noescape, but they are not
// side-effect SOURCES (see analyzeFun). A `fun() &Int` param also gets a
// param-taint so calling it carries an identity ReturnTaints can pick up: in
// `fun apply(f fun() &Int) &Int { f() }`, `f()` must reach the return value.
func (a *LifetimeCheck) analyzeFunParam(nodeID ast.NodeID, param ast.FunParam) {
	a.ast.Walk(nodeID, a.Check)
	ss := a.scopeState(nodeID)
	var callerChain Chain
	paramTypeID := a.env.TypeOfNode(nodeID).ID
	if a.typeCanEscape(paramTypeID) || a.typeContainsRefOrAlloc(paramTypeID) ||
		a.funTypeReturnCanEscape(paramTypeID) {
		callerChain = Chain{TaintSet{a.newTaintID()}}
	}
	ss.Vars[param.Name.Name] = &VarTaint{nodeID, ss.ScopeTaint, callerChain}
	a.debug(1, nodeID, "analyzeFunParam: %s scope=%s callerChain=%s", param.Name.Name, ss.ID(), callerChain)
}

// funTypeReturnCanEscape reports whether typeID is a function type whose return
// value can escape (e.g. `fun() &Int`). Such a param carries an identity so its
// call result flows into ReturnTaints (the higher-order-function escape path).
func (a *LifetimeCheck) funTypeReturnCanEscape(typeID TypeID) bool {
	if typeID == InvalidTypeID {
		return false
	}
	funType, ok := a.env.Type(typeID).Kind.(FunType)
	return ok && a.typeCanEscape(funType.Return)
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
	typ := a.env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case RefType, AllocatorType:
		return true
	case StructType:
		for _, field := range kind.Fields {
			if a.typeContainsRefOrAlloc(field.Type) {
				return true
			}
		}
	case UnionType:
		if slices.ContainsFunc(kind.Variants, a.typeContainsRefOrAlloc) {
			return true
		}
	case TypeParamType:
		if kind.Shape == nil {
			return false
		}
		shape := base.Cast[ShapeType](a.env.Type(*kind.Shape).Kind)
		for _, field := range shape.Fields {
			if a.typeContainsRefOrAlloc(field.Type) {
				return true
			}
		}
		// A shape method that yields refs or takes refs makes T effectively
		// ref-carrying: passing/storing a T means we may be moving refs around,
		// so the param needs a caller taint for side-effect tracking.
		shapeDeclID := a.env.DeclNode(*kind.Shape)
		if shapeDeclID != 0 {
			if shapeNode, ok := a.ast.Node(shapeDeclID).Kind.(ast.Shape); ok {
				for _, funDeclNodeID := range shapeNode.Funs {
					funDecl, ok := a.ast.Node(funDeclNodeID).Kind.(ast.FunDecl)
					if !ok {
						continue
					}
					if a.funDeclTypeContainsRefOrAlloc(funDecl) {
						return true
					}
				}
			}
		}
	case ArrayType:
		return a.typeContainsRefOrAlloc(kind.Elem)
	case SliceType:
		return a.typeContainsRefOrAlloc(kind.Elem)
	}
	return false
}

// typeContainsRefAllocOrFfiPtr extends typeContainsRefOrAlloc with raw ffi
// pointers (`ffi.Ptr`/`ffi.PtrMut`, the empty builtin-ptr structs). An ffi
// pointer derived from Metall memory carries that memory's lifetime, so a union
// value holding one (e.g. in a struct field) must keep the borrow when unwrapped
// via `try`/`match`. Bare slices are deliberately NOT added here: their taint
// attribution through container methods is currently too coarse to propagate
// through a value binding without false positives (it would flag returning a
// slice read out of a local iterator that actually borrows a longer-lived param).
func (a *LifetimeCheck) typeContainsRefAllocOrFfiPtr(typeID TypeID) bool {
	if typeID == InvalidTypeID {
		return false
	}
	switch kind := a.env.Type(typeID).Kind.(type) {
	case RefType, AllocatorType:
		return true
	case StructType:
		if IsBuiltinPtrStruct(kind) {
			return true
		}
		return slices.ContainsFunc(kind.Fields, func(f StructField) bool {
			return a.typeContainsRefAllocOrFfiPtr(f.Type)
		})
	case UnionType:
		return slices.ContainsFunc(kind.Variants, a.typeContainsRefAllocOrFfiPtr)
	case ArrayType:
		return a.typeContainsRefAllocOrFfiPtr(kind.Elem)
	case SliceType:
		return a.typeContainsRefAllocOrFfiPtr(kind.Elem)
	}
	return false
}

// typeContainsSlice reports whether the type is, or recursively contains by
// value, a SliceType (so `Str`, which wraps a `[]U8`, counts). A slice borrows
// its backing storage, so storing one into a &mut param leaks that borrow.
// Refs are already covered by typeContainsRefOrAlloc; this adds the by-value
// slice case it omits (design-review F04).
func (a *LifetimeCheck) typeContainsSlice(typeID TypeID) bool {
	if typeID == InvalidTypeID {
		return false
	}
	switch kind := a.env.Type(typeID).Kind.(type) {
	case SliceType:
		return true
	case StructType:
		return slices.ContainsFunc(kind.Fields, func(f StructField) bool {
			return a.typeContainsSlice(f.Type)
		})
	case UnionType:
		return slices.ContainsFunc(kind.Variants, a.typeContainsSlice)
	case ArrayType:
		return a.typeContainsSlice(kind.Elem)
	}
	return false
}

// funDeclTypeContainsRefOrAlloc reports whether any parameter or return type
// in the given FunDecl is, or contains, a reference or allocator. Used to
// decide whether a TypeParam constrained by a shape is potentially ref-bearing.
func (a *LifetimeCheck) funDeclTypeContainsRefOrAlloc(funDecl ast.FunDecl) bool {
	check := func(nodeID ast.NodeID) bool {
		if nodeID == 0 {
			return false
		}
		switch kind := a.ast.Node(nodeID).Kind.(type) {
		case ast.RefType:
			return true
		case ast.SimpleType:
			if kind.Name.Name == "Arena" {
				return true
			}
		}
		return false
	}
	for _, paramNodeID := range funDecl.Params {
		paramNode, ok := a.ast.Node(paramNodeID).Kind.(ast.FunParam)
		if !ok {
			continue
		}
		if check(paramNode.Type) {
			return true
		}
	}
	return check(funDecl.ReturnType)
}

// typeCanEscape is like typeContainsRefOrAlloc but also considers slices
// and ffi::Ptr as pointer-carrying types. Used for noescape validation where
// a param's slice or ffi pointer must not escape.
func (a *LifetimeCheck) typeCanEscape(typeID TypeID) bool {
	if typeID == InvalidTypeID {
		return false
	}
	typ := a.env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case RefType, AllocatorType, SliceType, FunType:
		// A FunType (closure) can carry captured references, so a noescape return
		// of one must be confined like any other reference-bearing value.
		return true
	case StructType:
		if IsBuiltinPtrStruct(kind) {
			return true
		}
		for _, field := range kind.Fields {
			if a.typeCanEscape(field.Type) {
				return true
			}
		}
	case UnionType:
		if slices.ContainsFunc(kind.Variants, a.typeCanEscape) {
			return true
		}
	case ArrayType:
		return a.typeCanEscape(kind.Elem)
	}
	return false
}

func (a *LifetimeCheck) analyzeAssign(nodeID ast.NodeID, assign ast.Assign) {
	a.ast.Walk(nodeID, a.Check)
	rhs := a.flow(assign.RHS)
	lhsNode := a.ast.Node(assign.LHS)
	a.debug(1, nodeID, "analyzeAssign: lhs=%s rhs=%s", a.ast.Debug(assign.LHS, false, 0), rhs)
	switch lhsKind := lhsNode.Kind.(type) {
	case ast.Ident:
		if lhsKind.Name == "_" {
			return // discard
		}
		// `x = expr` REPLACES x's chain, preserving its storage taint. The escape
		// check is the write rule applied to the variable place {scope(x)}.
		ss := a.scopeState(nodeID)
		storageTaint := ss.ScopeTaint
		if vt := a.lookupVar(nodeID, lhsKind.Name); vt != nil {
			storageTaint = vt.StorageTaint
		}
		a.checkEscape(nodeID, rhs.CarriedTaints(), TaintSet{storageTaint}, "via mutation of outer variable")
		ss.Vars[lhsKind.Name] = &VarTaint{nodeID, storageTaint, rhs}
	case ast.FieldAccess:
		a.writeInto(nodeID, assign.LHS, rhs, "via mutation of outer variable")
	case ast.Index:
		a.writeInto(nodeID, assign.LHS, rhs, "via mutation of outer variable")
	case ast.Deref:
		// `p.* = expr` is check-only for the pointee: escape if rhs borrows a
		// scope strictly nested inside the slot p points at (chain(p)[0]). For
		// `w.*.*` that slot is chain(w)[1], so the deep write is caught at its
		// exact depth. We accumulate rhs into the root pointer's chain but do not
		// write back into the specific pointee (unobservable without aliases).
		a.checkEscape(nodeID, rhs.CarriedTaints(), a.storageScopes(assign.LHS), "via deref assignment")
		a.accumulateIntoRoot(nodeID, a.placeRoot(assign.LHS), rhs)
	default:
		panic(base.Errorf("unknown LHS kind: %T", lhsKind))
	}
	// Check noescape when assigning to a function-typed target.
	if lhsType := a.env.TypeOfNode(assign.LHS); lhsType != nil {
		a.checkNoescapeValueAssignment(assign.RHS, lhsType.ID)
	}
}

// writeInto implements the WRITE RULE for `place = rhs` where place is a
// field/index projection. It checks rhs against the storage the place writes
// into, then accumulates rhs into the root variable so the mutation is visible
// everywhere (this replaces the block-end propagation pass).
//
//	struct Foo { one &Int }
//	mut y = Foo(&x); { mut z = 99; y.one = &z }  --> &z escapes y's scope
func (a *LifetimeCheck) writeInto(nodeID ast.NodeID, place ast.NodeID, rhs Chain, detail string) {
	a.checkEscape(nodeID, rhs.CarriedTaints(), a.storageScopes(place), detail)
	root := a.placeRoot(place)
	a.accumulateIntoRoot(nodeID, root, rhs)
}

// placeRoot peels field/index/deref projections to the root expression.
//
//	foo.bar.baz  --> foo
//	arr[0].field --> arr
//	p.*.f        --> p
func (a *LifetimeCheck) placeRoot(place ast.NodeID) ast.NodeID {
	root := place
	for {
		switch inner := a.ast.Node(root).Kind.(type) {
		case ast.FieldAccess:
			root = inner.Target
		case ast.Index:
			root = inner.Target
		case ast.Deref:
			root = inner.Expr
		default:
			return root
		}
	}
}

// accumulateIntoRoot merges rhs into the root variable's chain (per depth),
// writing into the variable's DECLARING scope so the mutation is visible
// everywhere. This powers function SideEffects and closure capture writes, and
// replaces the block-end propagation.
func (a *LifetimeCheck) accumulateIntoRoot(nodeID ast.NodeID, root ast.NodeID, rhs Chain) {
	ident, ok := a.ast.Node(root).Kind.(ast.Ident)
	if !ok {
		return
	}
	vt := a.lookupVar(root, ident.Name)
	if vt == nil {
		return
	}
	vt.Chain = vt.Chain.Merge(rhs)
	a.debug(1, nodeID, "accumulateIntoRoot: %s += %s --> %s", ident.Name, rhs, vt.Chain)
}

// checkEscape reports an escape when rhs borrows storage that outlives the
// place being written. An rhs taint escapes when its scope is strictly nested
// inside a storage scope, or when the storage is a param-pointer (which outlives
// the whole body) and rhs borrows a body-local scope.
//
//	{ mut a; { mut z; y.one = &z } }   --> scope(z) nests inside scope(a): escape
//	fun caller(t &mut T) { let a; t.do(&a) }  --> &a written through param t: escape
func (a *LifetimeCheck) checkEscape(nodeID ast.NodeID, rhs, storage TaintSet, detail string) {
	for _, rt := range rhs {
		if a.scopeByTaint[rt] == nil {
			continue // param-taint or non-scope rhs: never the shorter-lived side
		}
		for _, st := range storage {
			if rt == st {
				continue
			}
			// A param-taint storage outlives every body scope, so a body-local
			// rhs always escapes through it. Otherwise require strict nesting.
			if a.scopeByTaint[st] == nil || a.strictlyNested(rt, st) {
				a.diagEscapeTaint(nodeID, rt, detail)
				return
			}
		}
	}
}

// strictlyNested reports whether scope(inner) is a strict descendant of
// scope(outer): inner is shorter-lived than outer. Callers pass scope taints
// (both map to a scope).
func (a *LifetimeCheck) strictlyNested(inner, outer TaintID) bool {
	scope := a.scopeByTaint[inner]
	outerScope := a.scopeByTaint[outer]
	for scope = scope.Parent; scope != nil; scope = scope.Parent {
		if scope.ID == outerScope.ID {
			return true
		}
	}
	return false
}

// analyzeBlock checks the block result for escaping local taints, then strips
// them. The block result escapes if it carries a taint born in this block (a
// LocalTaint), which means a borrow of block-local storage is leaving.
//
//	let x = { let y = 123; &y }   --> &y borrows {scope(y)}, a block local: escape
func (a *LifetimeCheck) analyzeBlock(nodeID ast.NodeID, block ast.Block) {
	a.ast.Walk(nodeID, a.Check)
	if len(block.Exprs) == 0 {
		return
	}
	lastExpr := block.Exprs[len(block.Exprs)-1]
	lastChain := a.flow(lastExpr)
	lastTaints := lastChain.CarriedTaints()
	ss := a.scopeState(lastExpr)

	// Skip if the last expression is a return: analyzeReturn already checked it.
	_, lastIsReturn := a.ast.Node(lastExpr).Kind.(ast.Return)
	if !lastIsReturn && lastTaints.ContainsAny(ss.LocalTaints) {
		a.diagEscape(lastExpr, lastTaints, ss, "via block result")
	}
	resultChain := lastChain.Without(ss.LocalTaints)
	a.chains[nodeID] = resultChain
	a.debug(1, nodeID, "analyzeBlock: scope=%s result=%s", ss.ID(), resultChain)
}

func (a *LifetimeCheck) analyzeReturn(nodeID ast.NodeID, ret ast.Return) {
	a.ast.Walk(nodeID, a.Check)
	chain := a.flow(ret.Expr)
	a.chains[nodeID] = chain
	taints := chain.CarriedTaints()
	// Walk up scopes from the return to the enclosing function. At each scope,
	// the return escapes if it carries a taint local to that scope.
	scope := a.scopeGraph.NodeScope(nodeID)
	for scope != nil {
		if _, isFun := a.ast.Node(scope.Node).Kind.(ast.Fun); isFun {
			break
		}
		ss := a.scopeFor(scope)
		if taints.ContainsAny(ss.LocalTaints) {
			a.diagEscape(ret.Expr, taints, ss, "via return")
		}
		scope = scope.Parent
	}
}

// analyzeFun builds a FunEffects that describes how parameter taints map to the
// return value and to each other (side effects).
//
//nolint:funlen
func (a *LifetimeCheck) analyzeFun(nodeID ast.NodeID, fun ast.Fun) {
	if fun.Builtin {
		return
	}
	// Track the body on the analysis stack so a call inside it that resolves to
	// this same decl is recognised as recursion (status timing is unreliable
	// across the generic-body vs per-instance re-analysis).
	a.analyzingFun = append(a.analyzingFun, nodeID)
	defer func() { a.analyzingFun = a.analyzingFun[:len(a.analyzingFun)-1] }()
	// Walk type params and return type (no special handling needed).
	for _, tp := range fun.TypeParams {
		a.Check(tp)
	}
	if fun.ReturnType != ast.InferredType {
		a.Check(fun.ReturnType)
	}

	// Analyze params first and capture their initial caller (param) taints.
	// sideEffectSrc marks taints of params that borrow storage (ref/alloc OR a
	// by-value slice/Str), the valid side-effect SOURCES: storing one into a &mut
	// param leaks the borrow. A slice borrows its backing just like a ref, so
	// `fun store(dst &mut Buf, src []Int) { dst.* = Buf(src) }` is a side effect.
	paramTaintToIdx := map[TaintID]int{}
	sideEffectSrc := map[TaintID]bool{}
	for i, paramNodeID := range fun.Params {
		a.Check(paramNodeID)
		name := base.Cast[ast.FunParam](a.ast.Node(paramNodeID).Kind).Name.Name
		paramTypeID := a.env.TypeOfNode(paramNodeID).ID
		isSrc := a.typeContainsRefOrAlloc(paramTypeID) || a.typeContainsSlice(paramTypeID)
		if vt := a.lookupVar(paramNodeID, name); vt != nil {
			for _, t := range vt.Chain.CarriedTaints() {
				paramTaintToIdx[t] = i
				sideEffectSrc[t] = isSrc
			}
		}
	}

	// Rebind captures in the BODY scope so the body sees the captured borrow,
	// mirroring engine.go bindCapture (by-ref capture of x has type &x inside the
	// body; by-value has x's type). Without this, a by-ref-captured `local` would
	// resolve to the OUTER var with an empty chain, so the body's RETURN of it
	// would carry nothing and a real dangle would be missed.
	//   fun bar() &Int { mut local = 99  let g = fun[&local]() &Int { local }  g() }
	// Inside the body `local` is &local with chain [{scope(local)}], so the body
	// result carries scope(local) and g() escapes.
	if len(fun.Captures) > 0 {
		bodyScope := a.scopeState(fun.Block)
		for _, capNodeID := range fun.Captures {
			capture := base.Cast[ast.Capture](a.ast.Node(capNodeID).Kind)
			outer := a.lookupVar(nodeID, capture.Name.Name)
			if outer == nil {
				continue
			}
			var chain Chain
			switch capture.Mode {
			case ast.CaptureByValue:
				chain = outer.Chain
			case ast.CaptureByRef, ast.CaptureByMutRef:
				chain = outer.Chain.Prepend(TaintSet{outer.StorageTaint})
			}
			bodyScope.Vars[capture.Name.Name] = &VarTaint{capNodeID, bodyScope.ScopeTaint, chain}
		}
	}

	// Now analyze the body.
	a.Check(fun.Block)
	blockTaints := a.flow(fun.Block).CarriedTaints()

	effects := &FunEffects{
		SideEffects:  map[int][]int{},
		ReturnTaints: nil,
	}
	paramScope := a.scopeGraph.NodeScope(fun.Block)

	// Which param taints appear in the return value?
	// A taint that is NOT a param taint is function-local (e.g. the scope taint
	// of a by-value param's stack slot) and must not escape.
	for _, t := range blockTaints {
		if idx, ok := paramTaintToIdx[t]; ok {
			if !slices.Contains(effects.ReturnTaints, idx) {
				effects.ReturnTaints = append(effects.ReturnTaints, idx)
			}
		} else {
			a.diagEscape(fun.Block, blockTaints, a.scopeFor(paramScope), "via block result")
			break
		}
	}

	// Which params had foreign param taints merged into them (side effects)?
	// e.g. `fun foo(a &mut Int, b &Int) { a.* = b.* }` --> param 0 got param 1's taint.
	for i, paramNodeID := range fun.Params {
		name := base.Cast[ast.FunParam](a.ast.Node(paramNodeID).Kind).Name.Name
		vt := a.lookupVar(paramNodeID, name)
		if vt == nil {
			continue
		}
		for _, t := range vt.Chain.CarriedTaints() {
			if srcIdx, ok := paramTaintToIdx[t]; ok && srcIdx != i && sideEffectSrc[t] {
				if !slices.Contains(effects.SideEffects[i], srcIdx) {
					effects.SideEffects[i] = append(effects.SideEffects[i], srcIdx)
				}
			}
		}
	}

	a.funEffects[nodeID] = effects
	a.debug(1, nodeID, "analyzeFun: effects for %s (nodeID=%d): taints=%v sideEffects=%v",
		fun.Name.Name, nodeID, effects.ReturnTaints, effects.SideEffects)

	// Check noescape constraints on the function's own parameters.
	funType := base.Cast[FunType](a.env.TypeOfNode(nodeID).Kind)
	for i, paramNodeID := range fun.Params {
		if !funType.IsNoescape(i) {
			continue
		}
		paramName := base.Cast[ast.FunParam](a.ast.Node(paramNodeID).Kind).Name.Name
		a.checkNoescapeEffects(a.ast.Node(paramNodeID).Span, paramName, i, funType, effects)
	}
	a.checkMeaninglessNoescape(fun, funType)

	// For closures with captures, the closure value carries the chains of its
	// captures. A by-ref capture contributes `&capturedVar` (prepends its storage
	// scope); a by-value capture contributes the captured var's chain. Register a
	// VarTaint for the closure name so the ident reference picks it up; the escape
	// is caught by the normal block-result / return check. Writes the body makes
	// THROUGH a captured &mut are handled at the assignment site (the captured var
	// resolves in its outer declaring scope, so the normal write rule applies).
	if len(fun.Captures) > 0 {
		// A closure CALL yields what its BODY returns, not all its captures. The
		// block result chain already has the closure's own locals stripped by
		// analyzeBlock; captures (which live in outer scopes) survive. Recording
		// this precise chain lets a direct call carry ONLY the returned captures.
		//   fun bar(p &Int) &Int { mut local = 99  let g = fun[&local, p]() &Int { p }  g() }
		// g's body returns p (the param), not local, so g() must not be rejected.
		a.closureResults[nodeID] = a.flow(fun.Block)

		// Seed with the closure's own context taint: the capture context is
		// alloca'd on the enclosing function's frame, so a closure with ANY capture
		// borrows that frame even when every capture is a plain value (`fun[n]` with
		// n Int copies n into the context). Without this, returning such a closure
		// would dangle the context undetected. The taint is the enclosing FUNCTION
		// body, not the closure's immediate (desugared wrapper) scope, so moving the
		// closure to an outer block within the same function is not a false escape.
		bindScope := a.scopeFor(a.scopeGraph.NodeScope(nodeID))
		closureChain := Chain{TaintSet{a.enclosingFunScope(nodeID).ScopeTaint}}
		for _, capNodeID := range fun.Captures {
			capture := base.Cast[ast.Capture](a.ast.Node(capNodeID).Kind)
			vt := a.lookupVar(nodeID, capture.Name.Name)
			if vt == nil {
				continue
			}
			switch capture.Mode {
			case ast.CaptureByRef, ast.CaptureByMutRef:
				closureChain = closureChain.Merge(vt.Chain.Prepend(TaintSet{vt.StorageTaint}))
			case ast.CaptureByValue:
				closureChain = closureChain.Merge(vt.Chain)
			}
		}
		bindScope.Vars[fun.Name.Name] = &VarTaint{nodeID, bindScope.ScopeTaint, closureChain}
	}
}

// flow returns the chain a node's value carries.
func (a *LifetimeCheck) flow(nodeID ast.NodeID) Chain {
	return a.chains[nodeID]
}

// mergeFlows unions the chains of every node per depth: the storage an aggregate
// (struct literal, array literal, pessimistic call) reaches is whatever any of
// its parts reaches.
func (a *LifetimeCheck) mergeFlows(nodeIDs []ast.NodeID) Chain {
	var merged Chain
	for _, nodeID := range nodeIDs {
		merged = merged.Merge(a.flow(nodeID))
	}
	return merged
}

// scopeState returns the state of the lexical scope that contains nodeID.
func (a *LifetimeCheck) scopeState(nodeID ast.NodeID) *ScopeState {
	return a.scopeFor(a.scopeGraph.NodeScope(nodeID))
}

// enclosingFunScope returns the body scope of the function that lexically
// encloses nodeID: the outermost scope still inside that function (the one whose
// parent is the ast.Fun). A value tagged with this scope's taint escapes only
// when it leaves the function, not when it moves to an outer block within it.
func (a *LifetimeCheck) enclosingFunScope(nodeID ast.NodeID) *ScopeState {
	scope := a.scopeGraph.NodeScope(nodeID)
	for scope.Parent != nil {
		if _, isFun := a.ast.Node(scope.Parent.Node).Kind.(ast.Fun); isFun {
			break
		}
		scope = scope.Parent
	}
	return a.scopeFor(scope)
}

// scopeFor returns a scope's state, lazily creating it (and minting its scope
// taint) the first time the scope is touched.
func (a *LifetimeCheck) scopeFor(scope *ast.Scope) *ScopeState {
	if ss, ok := a.scopes[scope.ID]; ok {
		return ss
	}
	ss := newScopeState(scope, a.newTaintID())
	a.scopes[scope.ID] = ss
	a.scopeByTaint[ss.ScopeTaint] = scope
	return ss
}

// lookupVar finds a variable's taint state by name, walking scopes OUTWARD from
// nodeID (innermost first), so an inner declaration shadows an outer one.
func (a *LifetimeCheck) lookupVar(nodeID ast.NodeID, name string) *VarTaint {
	vt, _ := a.lookupVarWithScope(nodeID, name)
	return vt
}

func (a *LifetimeCheck) lookupVarWithScope(nodeID ast.NodeID, name string) (*VarTaint, *ScopeState) {
	scope := a.scopeGraph.NodeScope(nodeID)
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

// newTaintID hands out the next fresh taint (used for a new scope's taint or a
// reference-carrying parameter's param taint).
func (a *LifetimeCheck) newTaintID() TaintID {
	id := a.nextTaintID
	a.nextTaintID++
	return id
}

func (a *LifetimeCheck) debug(level int, nodeID ast.NodeID, msg string, args ...any) {
	d := a.Debug.Print(level, "%s", fmt.Sprintf(msg, args...))
	indent := d.Indent()
	d.Print(2, "at %s", a.ast.Node(nodeID).Span.DebugLine())
	indent()
}

// diagEscapeTaint blames a single escaping taint (used by the write rule, where
// the escaping scope is known directly rather than via LocalTaints membership).
func (a *LifetimeCheck) diagEscapeTaint(fallbackNodeID ast.NodeID, taint TaintID, detail string) {
	diagNode := fallbackNodeID
	if origin, ok := a.taintOrigin[taint]; ok {
		diagNode = origin
	}
	span := a.ast.Node(diagNode).Span
	key := escapeDiagKey{span, detail}
	if a.emittedEscape[key] {
		return
	}
	a.emittedEscape[key] = true
	a.diag(span, "reference escaping its allocation scope (%s)", detail)
}

func (a *LifetimeCheck) diagEscape(fallbackNodeID ast.NodeID, taints TaintSet, ss *ScopeState, detail string) {
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
		span := a.ast.Node(diagNode).Span
		key := escapeDiagKey{span, detail}
		if a.emittedEscape[key] {
			continue
		}
		a.emittedEscape[key] = true
		a.diag(span, "reference escaping its allocation scope (%s)", detail)
	}
}

// checkMeaninglessNoescape rejects `noescape` on a CONCRETE type that cannot
// carry a reference: the annotation can never confine anything, so it is almost
// certainly a mistake. A type that mentions a type parameter is exempt, it may
// carry a reference once instantiated. The declaration is judged once, on its
// declared (possibly generic) type, hence the per-instance re-analysis is skipped.
//
//	fun a() noescape Int { ... }    --> error: Int cannot carry a reference
//	fun a() noescape &Int { ... }   --> ok
//	fun a<T>() noescape T { ... }   --> ok (T might be a reference)
func (a *LifetimeCheck) checkMeaninglessNoescape(fun ast.Fun, funType FunType) {
	if a.analyzingInstance {
		return
	}
	meaningless := func(typeID TypeID) bool {
		return !a.typeContainsTypeParam(typeID) && !a.typeCanEscape(typeID)
	}
	if funType.NoescapeReturn && fun.ReturnType != 0 && meaningless(funType.Return) {
		a.diag(a.ast.Node(fun.ReturnType).Span,
			"noescape is meaningless on a return type that cannot carry a reference")
	}
	for i, paramNodeID := range fun.Params {
		if i < len(funType.Params) && funType.IsNoescape(i) && meaningless(funType.Params[i]) {
			a.diag(a.ast.Node(paramNodeID).Span,
				"noescape is meaningless on a parameter that cannot carry a reference")
		}
	}
}

// checkNoescapeEffects verifies that a noescape param doesn't escape through
// the return value or other parameters, given computed effects.
func (a *LifetimeCheck) checkNoescapeEffects(
	span base.Span, paramName string, paramIdx int, funType FunType, effects *FunEffects,
) {
	if a.typeCanEscape(funType.Return) && slices.Contains(effects.ReturnTaints, paramIdx) {
		a.diag(span, "noescape parameter %q must not escape through the return value", paramName)
	}
	for targetIdx, srcIndices := range effects.SideEffects {
		if targetIdx == paramIdx {
			continue
		}
		targetTypeID := funType.Params[targetIdx]
		if ref, ok := a.env.Type(targetTypeID).Kind.(RefType); ok {
			targetTypeID = ref.Type
		}
		if !a.typeCanEscape(targetTypeID) {
			continue
		}
		if slices.Contains(srcIndices, paramIdx) {
			a.diag(span, "noescape parameter %q must not escape through other parameters", paramName)
		}
	}
}

// checkNoescapeValueAssignment checks if targetTypeID is a function type with
// noescape params or noescape return, and if so verifies the function value at
// valueNodeID respects the noescape contract. Called from call args, struct
// construction, var bindings, and assignments.
func (a *LifetimeCheck) checkNoescapeValueAssignment(valueNodeID ast.NodeID, targetTypeID TypeID) {
	targetType := a.env.Type(targetTypeID)
	targetFun, ok := targetType.Kind.(FunType)
	if !ok {
		return
	}
	if slices.Contains(targetFun.NoescapeParams, true) {
		a.checkNoescapeAssignment(valueNodeID, targetFun)
	}
	if targetFun.NoescapeReturn {
		a.checkNoescapeReturnAssignment(valueNodeID)
	}
}

// checkNoescapeAssignment verifies that a concrete function assigned to a
// function type with noescape params respects the noescape contract.
func (a *LifetimeCheck) checkNoescapeAssignment(argNodeID ast.NodeID, targetFun FunType) {
	var declID ast.NodeID
	if d, ok := a.env.FunDeclNode(argNodeID); ok {
		declID = d
	}
	if declID == 0 {
		argType := a.env.TypeOfNode(argNodeID)
		if argType != nil {
			effectsTypeID := argType.ID
			if origin, ok := a.env.GenericOrigin(effectsTypeID); ok {
				effectsTypeID = origin
			}
			declID = a.env.DeclNode(effectsTypeID)
		}
	}
	if declID == 0 {
		if _, ok := a.ast.Node(argNodeID).Kind.(ast.Fun); ok {
			declID = argNodeID
		}
	}
	if declID == 0 {
		return
	}
	effects, ok := a.funEffects[declID]
	if !ok {
		return
	}
	span := a.ast.Node(argNodeID).Span
	for i := range targetFun.Params {
		if !targetFun.IsNoescape(i) {
			continue
		}
		a.checkNoescapeEffects(span, fmt.Sprintf("param %d", i), i, targetFun, effects)
	}
}

// checkNoescapeReturnAssignment verifies that a concrete function assigned to a
// function type with noescape return also has noescape return.
func (a *LifetimeCheck) checkNoescapeReturnAssignment(argNodeID ast.NodeID) {
	argType := a.env.TypeOfNode(argNodeID)
	if argType == nil {
		return
	}
	argFun, ok := argType.Kind.(FunType)
	if !ok {
		return
	}
	if !argFun.NoescapeReturn {
		a.diag(a.ast.Node(argNodeID).Span, "function does not return noescape")
	}
}

func (a *LifetimeCheck) diag(span base.Span, msg string, msgArgs ...any) {
	a.Diagnostics = append(a.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}

// shapeContractsCheck verifies that shape method contracts are consistent
// with their concrete implementations. The shape contract specifies that
// parameters do not flow into each other (no side effects), but all
// parameters may flow to the return value.
type shapeContractsCheck struct {
	*LifetimeCheck
	funWorks map[ast.NodeID][]FunWork
}

// expectedEffects computes the FunEffects implied by a shape method declaration.
func (s *shapeContractsCheck) expectedEffects(declID ast.NodeID, funDecl ast.FunDecl, calleeType FunType) *FunEffects {
	effects := &FunEffects{
		SideEffects:  map[int][]int{},
		ReturnTaints: nil,
	}
	// All parameters may flow to the return value, but only if the return type
	// can carry references. No parameters flow into each other (no side effects).
	if s.typeContainsRefOrAlloc(calleeType.Return) {
		for i := range funDecl.Params {
			effects.ReturnTaints = append(effects.ReturnTaints, i)
		}
	}
	s.funEffects[declID] = effects
	s.debug(1, declID, "shapeContractsCheck.expectedEffects: taints=%v", effects.ReturnTaints)
	return effects
}

// verify checks that concrete implementations of shape methods don't violate
// the shape's effect contract. Must be called after Check() so that all
// FunEffects are computed.
func (s *shapeContractsCheck) verify() {
	for _, works := range s.funWorks {
		for _, fw := range works {
			s.verifyFunWork(fw)
		}
	}
}

func (s *shapeContractsCheck) verifyFunWork(fw FunWork) {
	fun, ok := s.ast.Node(fw.NodeID).Kind.(ast.Fun)
	if !ok || fun.Builtin || fun.Extern {
		return
	}
	if len(fun.TypeParams) == 0 {
		return
	}
	prevEnv := s.env
	prevInstance := s.analyzingInstance
	s.env = fw.Env
	s.analyzingInstance = true
	defer func() { s.env = prevEnv; s.analyzingInstance = prevInstance }()
	s.resetAnalysisStatus(fw.NodeID)
	s.analyzeFun(fw.NodeID, fun)
}

// resetAnalysisStatus marks the given fun's body subtree as "not visited" so
// a follow-up Check pass will re-run the analyzers on it (with a different
// env or context).
func (a *LifetimeCheck) resetAnalysisStatus(nodeID ast.NodeID) {
	fun, ok := a.ast.Node(nodeID).Kind.(ast.Fun)
	if !ok {
		return
	}
	var visit func(id ast.NodeID)
	visit = func(id ast.NodeID) {
		if id == 0 {
			return
		}
		delete(a.status, id)
		a.ast.Walk(id, visit)
	}
	visit(fun.Block)
	for _, p := range fun.Params {
		visit(p)
	}
}
