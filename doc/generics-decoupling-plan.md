# Decoupling `engine_generics` from `engine`

The current layout has 99 methods on `*Engine` in `engine.go` and 48 more on
`*Engine` in `engine_generics.go`. They share the same struct and freely call
each other, so the file split is decorative — there is no real boundary.
The goal of this plan is to give `Generics` (the new monomorphization /
materialization machinery) a real boundary so it can be reasoned about,
tested, and replaced independently of the core type checker.

## Today's entanglement

Three things keep `engine_generics.go` glued to `engine.go`:

1. **Shared receiver.** Every helper is a method on `*Engine`. Generics
   helpers call `e.Query`, `e.lookup`, `e.bind`, `e.diag`,
   `e.scopeGraph.NodeScope`, `e.env.*`, `e.ast`, etc. They also mutate
   engine state (`implicitOwnerArgs`, `concreteAssociated`,
   `builtinOwnerParams`, `instantiationScope`, `funs`).
2. **Bidirectional calls.** `engine.go` calls into
   `MaterializeNamedTypeRef`, `MaterializeFun`, `InferFunCall`,
   `bindTypeParams`, `resolveTypeParamConstraint`. `engine_generics.go`
   calls back into `e.Query`, `e.checkSimpleType`, `e.checkFunBody`,
   `e.SatisfiesShape`. There is no place in the call graph where you can
   cut without leaving dangling edges.
3. **Shared env mutations.** Generics handlers `enterChildEnv` /
   `BindGenericArgs` to set up scoped bindings, then expect the body to be
   re-checked by the core checker in that same env. The lifetime pass
   needs the post-rewrite types cached in the parent env (see
   `propagateNodeTypesToParent`). The "who owns the cache" question has
   no single answer today.

## Target boundary

Introduce a `Generics` struct that owns everything generics-specific and
exposes a narrow API to the engine. The engine knows nothing about
TypeParamType internals, materialization, or inference; it just asks
"resolve this name with these type args" or "give me a callable for this
call site."

```go
// In types/generics package or types/generics.go
type Generics struct {
    core   GenericsHost   // interface back to the engine
    cache  cacheState     // typeParamTypes, funs, materialized instances
    seeds  seedState      // implicitOwnerArgs, concreteAssociated, etc.
}

type GenericsHost interface {
    AST() *ast.AST
    ScopeGraph() *ast.ScopeGraph
    Env() *TypeEnv
    Diag(span base.Span, msg string, args ...any)
    Query(nodeID ast.NodeID) (TypeID, TypeStatus)
    Lookup(nodeID ast.NodeID, name string, idx int) (*Binding, bool)
    EnterChildEnv() func()
    SatisfiesShape(concrete, shape TypeID, scope ast.NodeID, span base.Span) bool
    CheckFunBody(funNodeID ast.NodeID, funNode ast.Fun, funTypeID TypeID, funType FunType)
}
```

The engine implements `GenericsHost` and owns a `*Generics`. Anywhere
`engine.go` reaches into generic resolution, it calls a method on the
`Generics` value instead of `e`.

## Six-step migration

I'd do this in PR-sized pieces; each piece is testable on its own and
leaves the suite green.

### Step 1 — make the receiver explicit (mechanical)

Rename every `*Engine` method that currently lives in `engine_generics.go`
to a free function that takes `*Engine` as the first arg (or
`(g *generics)` with `g.e` pointing at the engine). No semantic change;
just makes the dependency direction visible. A grep for
`e.MaterializeFun(` etc. shows every spot the engine reaches across the
line.

This shakes loose accidental dependencies and lets the next steps
introduce real types without rewriting call sites.

### Step 2 — collect generics state into a `Generics` struct

Move these fields off `Engine`:
- `funs` (mangled-name cache)
- `implicitOwnerArgs`
- `concreteAssociated`
- `builtinOwnerParams`
- `instantiationScope`

Plus the registry maps that are only read/written by generics:
- `e.env.reg.typeParamTypes`
- `e.env.reg.genericSpecs`
- `e.env.reg.genericOrigin`

These are still in the shared `reg` for now — but only `Generics`
touches them, so we audit and stop sharing in step 5.

### Step 3 — define the `GenericsHost` interface

Pull the methods generics actually needs from `Engine` into an interface.
Today the surface is roughly: `Query`, `lookup`, `bind`, `Env`, `AST`,
`ScopeGraph`, `Diag`, `EnterChildEnv`, `SatisfiesShape`, `CheckFunBody`.
The shape host is small (≤ 15 methods); keeping it small is what makes
the boundary real.

Use the interface as the receiver for the relocated functions. The
engine implements it. Tests can implement a mock for isolated unit tests.

### Step 4 — flip call-sites in `engine.go`

Replace `e.MaterializeFun(...)` with `e.generics.MaterializeFun(...)`
etc. The function bodies on the generics side now live in a new file
(`generics.go` or a `generics` package). Engine becomes the only place
that knows about both halves.

After this step the file split actually means something: `engine.go` is
the host, `generics.go` is the guest.

### Step 5 — narrow the registry boundary

The biggest mess today is shared mutation of `e.env.reg`. Pick one of:

- **Move generics state out of `reg` entirely.** `typeParamTypes`,
  `genericSpecs`, `genericOrigin` become fields of `*Generics`. The
  engine queries them via `g.TypeParamFor(nodeID)` etc.
- **Read-only contract.** Engine reads from `reg.*` for display /
  lifetime; `Generics` is the only writer. Document this in `typeenv.go`.

The first option is more honest. The second is cheaper and lets us
defer the bigger refactor.

### Step 6 — extract `engine_shapes.go` similarly

Shape conformance (`SatisfiesShape`, `MethodSignature`,
`CheckShapeFunDecl`, the bidirectional inference I added) is
generics-adjacent: it depends on materialization, type-arg rewriting,
implicit owner args, and concreteAssociated tracking. The natural home
is alongside `Generics`. Move it into the same package and let the
engine ask "does this satisfy this shape?" through the host interface.

After step 6 the engine is roughly: type-check expressions, dispatch
queries, manage scopes/envs/lifetime. Everything generics-shaped lives
behind `g`.

## Side benefits

- **Testing.** With a small `GenericsHost` interface, generics can be
  exercised without spinning up a full Engine + prelude. Today the
  cheapest path to test a generic substitution rule is to write a
  full `.met` test case.
- **Replaceability.** The user's "ignore the AST-synthesis experiment"
  was hard to honor because there is no swap-in point. With the
  decoupled `Generics`, swapping the substitution-based engine for an
  AST-synth one is a single replacement.
- **State debugging.** The shared state today
  (`implicitOwnerArgs`, `concreteAssociated`, `builtinOwnerParams`) is
  spread across the engine. Putting it on a single `*Generics` struct
  makes "what does the generics machinery know right now?" answerable in
  one place.

## What I'd start with

If I had a fresh half-day, step 1 (mechanical rename to free
functions) is the cheapest experiment with the highest information
value — it surfaces the actual coupling without changing any behavior,
and the diff is reviewable. Step 2 (lift state to a struct) is the
natural follow-on and is what unlocks all the later steps.

I would not start with step 3 or 4 in isolation: defining an interface
before the code is rearranged just leads to a "GodHost" interface that
exposes everything, and the boundary doesn't actually get narrower.
