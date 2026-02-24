package types

import (
	"fmt"
	"slices"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type TaintID int

func (t TaintID) String() string {
	return fmt.Sprintf("a%d", t)
}

type TaintIDs []TaintID

func (t TaintIDs) Merge(other TaintIDs) TaintIDs {
	for _, taint := range other {
		if slices.Contains(t, taint) {
			continue
		}
		t = append(t, taint)
	}
	return t
}

type RefTarget struct {
	ScopeID ScopeID
	Name    string
}

type RefTargets []RefTarget

func (r RefTargets) Merge(other RefTargets) RefTargets {
	for _, target := range other {
		if slices.Contains(r, target) {
			continue
		}
		r = append(r, target)
	}
	return r
}

type BindingTaints struct {
	DiagNodeID ast.NodeID
	Slot       TaintID
	Refs       TaintIDs
	RefTargets RefTargets
}

type TaintScope struct {
	Bindings   map[string]*BindingTaints
	Taints     TaintIDs
	StackTaint TaintID
}

func NewTaintScope(stackTaint TaintID) *TaintScope {
	return &TaintScope{map[string]*BindingTaints{}, TaintIDs{stackTaint}, stackTaint}
}

func (t *TaintScope) HasLocalTaints(taints TaintIDs) bool {
	for _, taint := range taints {
		if slices.Contains(t.Taints, taint) {
			return true
		}
	}
	return false
}

type LifetimeCheck struct {
	Diagnostics base.Diagnostics
	Debug       base.Debug
	e           *Engine
	taintScopes map[ScopeID]*TaintScope
	refTaints   map[ast.NodeID]TaintIDs
	refTargets  map[ast.NodeID]RefTargets
}

func NewLifetimeAnalyzer(e *Engine) *LifetimeCheck {
	return &LifetimeCheck{
		nil,
		base.NilDebug{},
		e,
		map[ScopeID]*TaintScope{},
		map[ast.NodeID]TaintIDs{},
		map[ast.NodeID]RefTargets{},
	}
}

func (a *LifetimeCheck) Check(nodeID ast.NodeID) {
	defer a.Debug.Print(2, "analyze: node=%s", a.e.AST.Debug(nodeID, false, 0)).Indent()()
	a.e.Walk(nodeID, a.Check)
	node := a.e.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.Assign:
		a.analyzeAssign(nodeID, kind)
	case ast.Block:
		a.analyzeBlock(nodeID, kind)
	case ast.Fun:
		a.analyzeFun(nodeID, kind)
	case ast.Deref:
		a.analyzeDeref(nodeID, kind)
	case ast.FunParam:
		a.analyzeFunParam(nodeID, kind)
	case ast.Ident:
		a.analyzeIdent(nodeID, kind)
	case ast.Ref:
		a.analyzeRef(nodeID, kind)
	case ast.Var:
		a.analyzeVar(nodeID, kind)
	default:
	}
}

func (a *LifetimeCheck) analyzeAssign(nodeID ast.NodeID, assign ast.Assign) {
	// Only the last assignment ever matters at the end of a block.
	rhsRefTaints := a.nodeRefTaints(assign.RHS)
	lhsNode := a.e.Node(assign.LHS)
	switch lhsKind := lhsNode.Kind.(type) {
	case ast.Ident:
		taintScope := a.taintScope(nodeID)
		rhsRefTargets := a.refTargets[assign.RHS]
		taintScope.Bindings[lhsKind.Name] = &BindingTaints{nodeID, 0, rhsRefTaints, rhsRefTargets}
	case ast.FieldAccess:
		// A field write (e.g., `foo.bar.baz = val`) taints the root struct binding.
		// Walk the chain of FieldAccess nodes to find the root Ident.
		target := lhsKind.Target
		for {
			node := a.e.Node(target)
			if fa, ok := node.Kind.(ast.FieldAccess); ok {
				target = fa.Target
				continue
			}
			break
		}
		targetIdent, ok := a.e.Node(target).Kind.(ast.Ident)
		if !ok {
			panic(base.Errorf("field access root target is not an ident: %T", a.e.Node(target).Kind))
		}
		taintScope := a.taintScope(nodeID)
		rhsRefTargets := a.refTargets[assign.RHS]
		b, ok := taintScope.Bindings[targetIdent.Name]
		if !ok {
			taintScope.Bindings[targetIdent.Name] = &BindingTaints{nodeID, 0, rhsRefTaints, rhsRefTargets}
		} else {
			b.Refs = b.Refs.Merge(rhsRefTaints)
			b.RefTargets = b.RefTargets.Merge(rhsRefTargets)
		}
	case ast.Deref:
		targets := a.refTargets[lhsKind.Expr]
		rhsTargets := a.refTargets[assign.RHS]
		a.Debug.Print(1, "analyzeAssign: rhsTargets=%s, targets=%s", rhsTargets, targets)
		localTaintScope := a.taintScope(nodeID)
		for _, target := range targets {
			taintScope := a.taintScopeByScope(target.ScopeID)
			// Check if assigning a local ref to an outer scope binding.
			if taintScope != localTaintScope && localTaintScope.HasLocalTaints(rhsRefTaints) {
				node := a.e.Node(nodeID)
				a.diag(node.Span, "reference escaping its allocation scope")
			}
			b, ok := taintScope.Bindings[target.Name]
			if !ok {
				taintScope.Bindings[target.Name] = &BindingTaints{nodeID, 0, rhsRefTaints, rhsTargets}
				continue
			}
			b.Refs = rhsRefTaints
		}
	default:
		panic(base.Errorf("unknown LHS kind: %T", lhsKind))
	}
}

func (a *LifetimeCheck) analyzeBlock(nodeID ast.NodeID, block ast.Block) {
	outerScope := a.e.ScopeGraph.NodeScope(nodeID)
	defer a.Debug.Print(0, "analyzeBlock: scope=%s node=%s", outerScope.ID, a.e.AST.Debug(nodeID, false, 0)).Indent()()
	if !block.CreateScope {
		// This must be handled by the outer scope.
		return
	}
	if len(block.Exprs) == 0 {
		return
	}
	// Build a list of everything that survives the block.
	toCheck := map[ast.NodeID]TaintIDs{}
	toBubble := map[string]*BindingTaints{}
	lastExprNodeID := block.Exprs[len(block.Exprs)-1]
	if _, ok := a.e.TypeOfNode(lastExprNodeID).Kind.(RefType); ok {
		lastExprTaints := a.nodeRefTaints(lastExprNodeID)
		a.Debug.Print(1, "analyzeBlock: lastExprTaints=%s", lastExprTaints)
		toCheck[lastExprNodeID] = lastExprTaints
	}
	taintScope := a.taintScope(lastExprNodeID)
	for name, bindingTaints := range taintScope.Bindings {
		if _, _, ok := outerScope.Lookup(name); ok {
			a.Debug.Print(2, "binding escapes: %s", name)
			toCheck[bindingTaints.DiagNodeID] = bindingTaints.Refs
			toBubble[name] = bindingTaints
		}
	}
	a.Debug.Print(2, "taintScope.Taints=%s", taintScope.Taints)

	// Now make sure no taints escape the block.
	for nodeID, taints := range toCheck {
		a.Debug.Print(1, "checking: node=%s taints=%s", a.e.AST.Debug(nodeID, false, 0), taints)
		if taintScope.HasLocalTaints(taints) {
			node := a.e.Node(nodeID)
			a.diag(node.Span, "reference escaping its allocation scope")
		}
	}

	// Bubble up the binding taints.
	parentTaintScope := a.taintScopeByScope(outerScope.ID)
	for name, bindingTaints := range toBubble {
		b, ok := parentTaintScope.Bindings[name]
		if !ok {
			parentTaintScope.Bindings[name] = &BindingTaints{
				bindingTaints.DiagNodeID,
				0,
				bindingTaints.Refs,
				bindingTaints.RefTargets,
			}
			continue
		}
		b.Refs = bindingTaints.Refs.Merge(b.Refs)
		b.RefTargets = bindingTaints.RefTargets.Merge(b.RefTargets)
	}
}

func (a *LifetimeCheck) analyzeFun(nodeID ast.NodeID, fun ast.Fun) {
	// Check if the function's return value is a ref that escapes the function scope.
	// The function body block has CreateScope=false, so analyzeBlock skips it.
	// We need to check here instead.
	funType := base.Cast[FunType](a.e.TypeOfNode(nodeID).Kind)
	retType := a.e.Type(funType.Return)
	if _, ok := retType.Kind.(RefType); !ok {
		return
	}
	block := base.Cast[ast.Block](a.e.Node(fun.Block).Kind)
	if len(block.Exprs) == 0 {
		return
	}
	lastExprNodeID := block.Exprs[len(block.Exprs)-1]
	lastExprTaints := a.nodeRefTaints(lastExprNodeID)
	// The function's scope contains all local variables. Any ref taint from
	// this scope escaping as a return value is a dangling reference.
	taintScope := a.taintScope(lastExprNodeID)
	if taintScope.HasLocalTaints(lastExprTaints) {
		node := a.e.Node(lastExprNodeID)
		a.diag(node.Span, "reference escaping its allocation scope")
	}
}

func (a *LifetimeCheck) analyzeDeref(nodeID ast.NodeID, deref ast.Deref) {
	// When we deref a pointer to a ref (e.g., *x where x: &&T), we get a ref.
	// We need to propagate the taints from what the deref resolves to.
	exprRefTargets := a.refTargets[deref.Expr]
	for _, target := range exprRefTargets {
		taintScope := a.taintScopeByScope(target.ScopeID)
		if b, ok := taintScope.Bindings[target.Name]; ok {
			a.refTaints[nodeID] = a.refTaints[nodeID].Merge(b.Refs)
			a.refTargets[nodeID] = a.refTargets[nodeID].Merge(b.RefTargets)
		}
	}
}

func (a *LifetimeCheck) analyzeFunParam(nodeID ast.NodeID, funParam ast.FunParam) {
	// Function parameters get their own taint from the function's scope.
	taintScope := a.taintScope(nodeID)
	taintScope.Bindings[funParam.Name.Name] = &BindingTaints{nodeID, taintScope.StackTaint, TaintIDs{}, RefTargets{}}
}

func (a *LifetimeCheck) analyzeIdent(nodeID ast.NodeID, ident ast.Ident) {
	bindingTaints := a.lookupBindingTaints(nodeID, ident.Name)
	if bindingTaints == nil {
		// Not a tracked binding (e.g., function name).
		return
	}
	a.setNodeRefTaints(nodeID, bindingTaints.Refs)
	a.refTargets[nodeID] = bindingTaints.RefTargets
}

func (a *LifetimeCheck) analyzeRef(nodeID ast.NodeID, ref ast.Ref) {
	bindingTaints := a.lookupBindingTaints(nodeID, ref.Name.Name)
	if bindingTaints == nil {
		// Should not happen for refs, but handle gracefully.
		return
	}
	a.setNodeRefTaints(nodeID, TaintIDs{bindingTaints.Slot})
	scope := a.e.ScopeGraph.NodeScope(nodeID)
	a.refTargets[nodeID] = append(a.refTargets[nodeID], RefTarget{scope.ID, ref.Name.Name})
}

func (a *LifetimeCheck) analyzeVar(nodeID ast.NodeID, varNode ast.Var) {
	exprRefTaints := a.nodeRefTaints(varNode.Expr)
	exprRefTargets := a.refTargets[varNode.Expr]
	taintScope := a.taintScope(nodeID)
	taintScope.Bindings[varNode.Name.Name] = &BindingTaints{
		nodeID,
		taintScope.StackTaint,
		exprRefTaints,
		exprRefTargets,
	}
}

func (a *LifetimeCheck) taintScope(nodeID ast.NodeID) *TaintScope {
	scope := a.e.ScopeGraph.NodeScope(nodeID)
	return a.taintScopeByScope(scope.ID)
}

func (a *LifetimeCheck) taintScopeByScope(scopeID ScopeID) *TaintScope {
	taintScope, ok := a.taintScopes[scopeID]
	if !ok {
		taintScope = NewTaintScope(TaintID(scopeID))
		a.taintScopes[scopeID] = taintScope
	}
	return taintScope
}

func (a *LifetimeCheck) lookupBindingTaints(nodeID ast.NodeID, name string) *BindingTaints {
	scope := a.e.ScopeGraph.NodeScope(nodeID)
	for scope != nil {
		taintScope := a.taintScopeByScope(scope.ID)
		if bindingTaints, ok := taintScope.Bindings[name]; ok {
			return bindingTaints
		}
		scope = scope.Parent
	}
	// Binding not found - could be a function or other non-tracked binding.
	return nil
}

func (a *LifetimeCheck) nodeRefTaints(nodeID ast.NodeID) TaintIDs {
	if taints, ok := a.refTaints[nodeID]; ok {
		return taints
	}
	return TaintIDs{}
}

func (a *LifetimeCheck) setNodeRefTaints(nodeID ast.NodeID, newTaints TaintIDs) {
	if _, ok := a.refTaints[nodeID]; ok {
		panic(base.Errorf("node %s already has ref taints", nodeID))
	}
	a.refTaints[nodeID] = newTaints
}

func (a *LifetimeCheck) diag(span base.Span, msg string, msgArgs ...any) {
	a.Diagnostics = append(a.Diagnostics, *base.NewDiagnostic(span, msg, msgArgs...))
}
