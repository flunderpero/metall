package types

import (
	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

func (e *Engine) checkMatch(match ast.Match, span base.Span, typeHint *TypeID) (TypeID, TypeStatus) {
	exprTypeID, status := e.Query(match.Expr)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	// Matching on a reference to a union/enum projects through the ref: arms bind
	// references into the matched value's storage (`case Foo &x` / `case Foo &mut x`).
	matchTypeID := exprTypeID
	var matchedRefMut *bool
	if ref, ok := e.env.Type(exprTypeID).Kind.(RefType); ok {
		matchTypeID = ref.Type
		mut := ref.Mut
		matchedRefMut = &mut
	}
	switch kind := e.env.Type(matchTypeID).Kind.(type) {
	case UnionType:
		if len(kind.Variants) == 0 {
			return InvalidTypeID, TypeDepFailed
		}
		if len(match.Arms) == 0 && match.Else == nil {
			e.diag(span, "match requires at least one arm")
			return InvalidTypeID, TypeFailed
		}
		return e.checkUnionMatchArms(match, kind, matchTypeID, matchedRefMut, span, typeHint)
	case EnumType:
		if len(match.Arms) == 0 && match.Else == nil {
			e.diag(span, "match requires at least one arm")
			return InvalidTypeID, TypeFailed
		}
		return e.checkEnumMatchArms(match, kind, matchTypeID, matchedRefMut, span, typeHint)
	default:
		e.diag(e.ast.Node(match.Expr).Span, "match expression must be a union or enum type, got %s",
			e.env.TypeDisplay(exprTypeID))
		return InvalidTypeID, TypeFailed
	}
}

// matchBindingType computes the type bound by a match arm or else binding, given
// the variant (or narrowed) type and the binding's `&`/`&mut` markers. It
// enforces the reference rules: a reference matched value requires reference
// bindings, a value matched value forbids them, and a `&mut` binding requires a
// `&mut` matched value (a `&` binding may coerce down from `&mut`). matchedRefMut
// is nil for a value matched value, else points at its ref mutability.
// It returns ok=false (after emitting a diagnostic) on a rule violation.
func (e *Engine) matchBindingType(
	span base.Span, baseTypeID TypeID, bindingRef, bindingMut bool, matchedRefMut *bool,
) (TypeID, bool) {
	if matchedRefMut == nil {
		if bindingRef {
			e.diag(span, "cannot bind a reference here: the matched value is not a reference; "+
				"match on `&value` to bind references")
			return InvalidTypeID, false
		}
		return baseTypeID, true
	}
	if !bindingRef {
		e.diag(span, "the matched value is a reference; bind with `&` or `&mut`, not by value")
		return InvalidTypeID, false
	}
	if bindingMut && !*matchedRefMut {
		e.diag(span, "cannot take a `&mut` binding from a `&` value")
		return InvalidTypeID, false
	}
	// nodeID 0: this ref type belongs to no AST node (the binding is a Name, not a
	// node), so it must not clobber the arm body's or pattern's recorded type.
	return e.env.buildRefType(0, baseTypeID, bindingMut, span), true
}

func (e *Engine) checkUnionMatchArms( //nolint:funlen
	match ast.Match, union UnionType, unionTypeID TypeID, matchedRefMut *bool,
	span base.Span, typeHint *TypeID,
) (TypeID, TypeStatus) {
	// covered[i] is the set of coverage keys seen for union variant i: "" for a
	// whole non-enum variant, the enum variant names for an enum component, and "*"
	// for a whole/root arm that catches an open enum (see variantCovered).
	covered := make([]map[string]bool, len(union.Variants))
	for i := range covered {
		covered[i] = map[string]bool{}
	}
	type armBody struct {
		body   ast.NodeID
		typeID TypeID
	}
	var bodies []armBody

	for _, arm := range match.Arms {
		isTry := len(arm.Patterns) == 1
		if isTry {
			_, isTry = e.ast.Node(arm.Patterns[0]).Kind.(ast.TryPattern)
		}
		var firstVariantTypeID TypeID
		for pi, pat := range arm.Patterns {
			var matchedIdx int
			var keys []string
			var bindTypeID TypeID
			if isTry {
				matchedIdx, keys, bindTypeID = 0, []string{""}, union.Variants[0]
				variantCached, _ := e.env.cachedTypeInfo(union.Variants[0])
				e.env.setNodeType(pat, variantCached)
			} else {
				patTypeID, varStatus := e.Query(pat)
				if varStatus.Failed() {
					return InvalidTypeID, TypeDepFailed
				}
				var ok bool
				matchedIdx, keys, bindTypeID, ok = e.unionArmKeys(pat, patTypeID, union)
				if !ok {
					e.diag(e.ast.Node(pat).Span, "type %s is not a variant of %s",
						e.env.TypeDisplay(patTypeID), e.env.TypeDisplay(unionTypeID))
					return InvalidTypeID, TypeFailed
				}
			}
			if arm.Guard == nil {
				fresh := false
				for _, k := range keys {
					if !covered[matchedIdx][k] {
						covered[matchedIdx][k] = true
						fresh = true
					}
				}
				if !fresh {
					e.diag(e.ast.Node(pat).Span, "duplicate match arm for variant %s",
						e.env.TypeDisplay(union.Variants[matchedIdx]))
					return InvalidTypeID, TypeFailed
				}
			}
			if pi == 0 {
				firstVariantTypeID = bindTypeID
			}
		}
		if arm.Binding != nil {
			// One pattern binds that variant; an or-pattern binds the whole union.
			bindBase := unionTypeID
			if len(arm.Patterns) == 1 {
				bindBase = firstVariantTypeID
			}
			bindTypeID, ok := e.matchBindingType(
				e.ast.Node(arm.Patterns[0]).Span, bindBase, arm.Ref, arm.Mut, matchedRefMut)
			if !ok {
				return InvalidTypeID, TypeFailed
			}
			bodyScope := e.scopeGraph.IntroducedScope(arm.Body)
			e.env.bindInScope(bodyScope, arm.Body, arm.Binding.Name, bindTypeID)
		}
		if arm.Guard != nil {
			if status := e.checkGuard(*arm.Guard); status.Failed() {
				return InvalidTypeID, status
			}
		}
		bodies = append(bodies, armBody{arm.Body, InvalidTypeID})
	}
	if match.Else != nil {
		// A `try` desugars to an else that propagates the error variant, so it is
		// never redundant. Otherwise an else over an already-exhaustive match is.
		if !match.Try && len(e.uncoveredUnionVariants(covered, union)) == 0 {
			e.diag(span, "else is not allowed in a match on %s; it is exhaustive",
				e.env.TypeDisplay(unionTypeID))
			return InvalidTypeID, TypeFailed
		}
		if match.Else.Binding != nil {
			bindTypeID := unionTypeID
			if uncovered := e.uncoveredUnionVariants(covered, union); len(uncovered) == 1 {
				bindTypeID = uncovered[0]
			}
			bindTypeID, ok := e.matchBindingType(
				span, bindTypeID, match.Else.Ref, match.Else.Mut, matchedRefMut)
			if !ok {
				return InvalidTypeID, TypeFailed
			}
			bodyScope := e.scopeGraph.IntroducedScope(match.Else.Body)
			e.env.bindInScope(bodyScope, match.Else.Body, match.Else.Binding.Name, bindTypeID)
		}
		bodies = append(bodies, armBody{match.Else.Body, InvalidTypeID})
	} else {
		for i, vID := range union.Variants {
			if !e.variantCovered(covered[i], vID) {
				e.diag(span, "non-exhaustive match: missing variant %s",
					e.env.TypeDisplay(vID))
				return InvalidTypeID, TypeFailed
			}
		}
	}

	for i, ab := range bodies {
		hint := typeHint
		// For `try`, the success arm body should not receive the outer
		// type hint. It yields the unwrapped value, not the Result.
		if match.Try && i < len(match.Arms) {
			hint = nil
		}
		bodyTypeID, bodyStatus := e.queryWithHint(ab.body, hint)
		if bodyStatus.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		bodies[i].typeID = bodyTypeID
	}
	if match.Try {
		if match.Else == nil {
			panic(base.Errorf("try must have an else branch"))
		}
		elseTyp, status := e.Query(match.Else.Body)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if elseTyp != e.neverTyp {
			e.diag(e.ast.Node(match.Else.Body).Span, "try else block must break control flow")
			return InvalidTypeID, TypeFailed
		}
	}

	var resultTypeID TypeID
	for _, ab := range bodies {
		if ab.typeID == InvalidTypeID || ab.typeID == e.neverTyp {
			continue
		}
		if resultTypeID == 0 {
			resultTypeID = ab.typeID
		} else if ab.typeID != resultTypeID {
			e.diag(e.ast.Node(ab.body).Span, "match arm type mismatch: expected %s, got %s",
				e.env.TypeDisplay(resultTypeID), e.env.TypeDisplay(ab.typeID))
			return InvalidTypeID, TypeFailed
		}
	}
	if resultTypeID == 0 {
		return e.neverTyp, TypeOK
	}
	return resultTypeID, TypeOK
}

func (e *Engine) checkGuard(guardID ast.NodeID) TypeStatus {
	guardTypeID, status := e.Query(guardID)
	if status.Failed() {
		return TypeDepFailed
	}
	if guardTypeID != e.boolTyp {
		e.diag(e.ast.Node(guardID).Span, "guard condition must be Bool, got %s",
			e.env.TypeDisplay(guardTypeID))
		return TypeFailed
	}
	return TypeOK
}

// unionArmKeys resolves one match-arm pattern against a union. matchedIdx is the
// union variant it targets. keys is what it covers: [""] for a whole non-enum
// variant; for an enum variant, the variant names (a whole-enum arm covers all of
// them, plus "*" when the enum is open). bindTypeID is the type the arm's binding
// takes. ok=false (no diagnostic) when the pattern is neither a variant nor an
// enum-component of the union.
func (e *Engine) unionArmKeys(
	pat ast.NodeID, patTypeID TypeID, union UnionType,
) (matchedIdx int, keys []string, bindTypeID TypeID, ok bool) {
	if enumID, variant, isVariantRef := e.env.EnumVariantRef(pat); isVariantRef {
		// `case Color.red` / `case IOErr.not_found`: one variant of an enum component.
		patEnum, ok := e.env.Type(enumID).Kind.(EnumType)
		if !ok {
			panic(base.Errorf("unionArmKeys: enum variant ref %q has a non-enum type", variant))
		}
		for i, vID := range union.Variants {
			if enumID == vID || patEnum.Root == vID {
				return i, []string{variant}, enumID, true
			}
		}
		return -1, nil, enumID, false
	}
	// A whole-variant match: `case Int n` / `case Color c` / `case Err e`.
	for i, vID := range union.Variants {
		if patTypeID == vID {
			return i, e.wholeVariantKeys(vID), patTypeID, true
		}
	}
	// A bare subset of an open enum component: `case IOErr io`.
	if patEnum, isEnum := e.env.Type(patTypeID).Kind.(EnumType); isEnum {
		for i, vID := range union.Variants {
			if patEnum.Root == vID {
				keys = make([]string, len(patEnum.Variants))
				for j, v := range patEnum.Variants {
					keys[j] = v.Name
				}
				return i, keys, patTypeID, true
			}
		}
	}
	return -1, nil, patTypeID, false
}

// wholeVariantKeys is the coverage-key set a whole-variant arm covers: "" for a
// non-enum variant, every variant name for an enum, plus "*" when the enum is
// open (the catch that makes an open enum exhaustive).
func (e *Engine) wholeVariantKeys(vID TypeID) []string {
	enum, isEnum := e.env.Type(vID).Kind.(EnumType)
	if !isEnum {
		return []string{""}
	}
	keys := make([]string, 0, len(enum.Variants)+1)
	for _, v := range enum.Variants {
		keys = append(keys, v.Name)
	}
	if enum.IsOpen {
		keys = append(keys, "*")
	}
	return keys
}

// variantCovered reports whether the keys seen for union variant vID exhaust it:
// "" for a non-enum variant, every variant name for a closed enum, or "*" (from a
// whole/root arm) for an open enum.
func (e *Engine) variantCovered(coveredKeys map[string]bool, vID TypeID) bool {
	enum, isEnum := e.env.Type(vID).Kind.(EnumType)
	if !isEnum {
		return coveredKeys[""]
	}
	if enum.IsOpen {
		return coveredKeys["*"]
	}
	for _, v := range enum.Variants {
		if !coveredKeys[v.Name] {
			return false
		}
	}
	return true
}

func (e *Engine) uncoveredUnionVariants(covered []map[string]bool, union UnionType) []TypeID {
	var result []TypeID
	for i, vID := range union.Variants {
		if !e.variantCovered(covered[i], vID) {
			result = append(result, vID)
		}
	}
	return result
}

// uncoveredVariants is the lifetime analyzer's coarse whole-variant view (an
// enum-component arm leaves its variant uncovered here, which is conservative).
func uncoveredVariants(covered []bool, union UnionType) []TypeID {
	var result []TypeID
	for i, c := range covered {
		if !c {
			result = append(result, union.Variants[i])
		}
	}
	return result
}

func (e *Engine) checkEnumMatchArms( //nolint:funlen
	match ast.Match, enum EnumType, enumTypeID TypeID, matchedRefMut *bool,
	span base.Span, typeHint *TypeID,
) (TypeID, TypeStatus) {
	if match.Else != nil && match.Else.Binding != nil {
		bindTypeID, ok := e.matchBindingType(
			span, enumTypeID, match.Else.Ref, match.Else.Mut, matchedRefMut)
		if !ok {
			return InvalidTypeID, TypeFailed
		}
		bodyScope := e.scopeGraph.IntroducedScope(match.Else.Body)
		e.env.bindInScope(bodyScope, match.Else.Body, match.Else.Binding.Name, bindTypeID)
	}
	if match.Try {
		// A `try` desugars to one narrowing arm plus an else that must break
		// control flow. Bare `try` leaves a TryPattern, which has no subset to
		// narrow to, so reject it here instead of in the arm loop.
		if _, ok := e.ast.Node(match.Arms[0].Patterns[0]).Kind.(ast.TryPattern); ok {
			e.diag(e.ast.Node(match.Arms[0].Patterns[0]).Span,
				"`try` on an enum requires a subset pattern, e.g. `try e is IOErr`")
			return InvalidTypeID, TypeFailed
		}
		elseTyp, status := e.Query(match.Else.Body)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if elseTyp != e.neverTyp {
			e.diag(e.ast.Node(match.Else.Body).Span, "try else block must break control flow")
			return InvalidTypeID, TypeFailed
		}
	}
	covered := map[string]bool{}
	var bodies []ast.NodeID
	for _, arm := range match.Arms {
		// Resolve each pattern to the variant keys it covers and the type its
		// binding takes. `IOErr.broken_pipe` is one variant, one key. A bare
		// subset like `IOErr` covers every variant of the subset. An or-pattern
		// (`case a or b:`) covers the union of its patterns' keys.
		var keys []string
		var firstPatTypeID TypeID
		for pi, pat := range arm.Patterns {
			patTypeID, status := e.Query(pat)
			if status.Failed() {
				return InvalidTypeID, TypeDepFailed
			}
			patEnum, ok := e.env.Type(patTypeID).Kind.(EnumType)
			if !ok {
				e.diag(e.ast.Node(pat).Span, "%s is not an enum variant or subset of %s",
					e.env.TypeDisplay(patTypeID), e.env.TypeDisplay(enumTypeID))
				return InvalidTypeID, TypeFailed
			}
			if _, variant, isVariant := e.env.EnumVariantRef(pat); isVariant {
				inFamily := patTypeID == enumTypeID
				if enum.IsOpen {
					inFamily = patEnum.Root == enumTypeID
				}
				if !inFamily {
					e.diag(e.ast.Node(pat).Span, "%s.%s is not a variant of %s",
						e.env.TypeDisplay(patTypeID), variant, e.env.TypeDisplay(enumTypeID))
					return InvalidTypeID, TypeFailed
				}
				keys = append(keys, patEnum.Name+"."+variant)
			} else {
				if !enum.IsOpen || patEnum.Root != enumTypeID {
					e.diag(e.ast.Node(pat).Span, "%s is not a subset of %s",
						e.env.TypeDisplay(patTypeID), e.env.TypeDisplay(enumTypeID))
					return InvalidTypeID, TypeFailed
				}
				for _, v := range patEnum.Variants {
					keys = append(keys, patEnum.Name+"."+v.Name)
				}
			}
			if pi == 0 {
				firstPatTypeID = patTypeID
			}
		}
		seen := map[string]bool{}
		for _, k := range keys {
			if seen[k] {
				e.diag(e.ast.Node(arm.Patterns[0]).Span, "duplicate variant %s in or-pattern", k)
				return InvalidTypeID, TypeFailed
			}
			seen[k] = true
		}
		if arm.Binding != nil {
			// One pattern binds that variant/subset; an or-pattern binds the enum.
			bindBase := enumTypeID
			if len(arm.Patterns) == 1 {
				bindBase = firstPatTypeID
			}
			bindTypeID, ok := e.matchBindingType(
				e.ast.Node(arm.Patterns[0]).Span, bindBase, arm.Ref, arm.Mut, matchedRefMut)
			if !ok {
				return InvalidTypeID, TypeFailed
			}
			bodyScope := e.scopeGraph.IntroducedScope(arm.Body)
			e.env.bindInScope(bodyScope, arm.Body, arm.Binding.Name, bindTypeID)
		}
		if arm.Guard != nil {
			// A guarded arm proves nothing. It may fall through, so it covers no
			// tags, mirroring the union rule.
			if status := e.checkGuard(*arm.Guard); status.Failed() {
				return InvalidTypeID, status
			}
		} else {
			fresh := false
			for _, k := range keys {
				if !covered[k] {
					covered[k] = true
					fresh = true
				}
			}
			if !fresh {
				e.diag(e.ast.Node(arm.Patterns[0]).Span, "unreachable match arm: all variants already covered")
				return InvalidTypeID, TypeFailed
			}
		}
		bodies = append(bodies, arm.Body)
	}
	if enum.IsOpen {
		if match.Else == nil {
			e.diag(span, "non-exhaustive match on open enum %s: an else arm is required",
				e.env.TypeDisplay(enumTypeID))
			return InvalidTypeID, TypeFailed
		}
	} else {
		missing := ""
		for _, v := range enum.Variants {
			if !covered[enum.Name+"."+v.Name] {
				missing = v.Name
				break
			}
		}
		if match.Else != nil {
			// A `try` desugars to an else that propagates, so it is never redundant.
			// Otherwise an else over an already-exhaustive match is.
			if !match.Try && missing == "" {
				e.diag(span, "else is not allowed in a match on closed enum %s; it is exhaustive",
					e.env.TypeDisplay(enumTypeID))
				return InvalidTypeID, TypeFailed
			}
		} else if missing != "" {
			e.diag(span, "non-exhaustive match: missing variant %s.%s",
				e.env.TypeDisplay(enumTypeID), missing)
			return InvalidTypeID, TypeFailed
		}
	}
	if match.Else != nil {
		bodies = append(bodies, match.Else.Body)
	}
	// Unify the body types. Diverging (`never`) bodies drop out. The rest must
	// agree, and that shared type is the match's type.
	var resultTypeID TypeID
	for _, body := range bodies {
		bodyTypeID, status := e.queryWithHint(body, typeHint)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if bodyTypeID == e.neverTyp {
			continue
		}
		if resultTypeID == 0 {
			resultTypeID = bodyTypeID
		} else if bodyTypeID != resultTypeID {
			e.diag(e.ast.Node(body).Span, "match arm type mismatch: expected %s, got %s",
				e.env.TypeDisplay(resultTypeID), e.env.TypeDisplay(bodyTypeID))
			return InvalidTypeID, TypeFailed
		}
	}
	if resultTypeID == 0 {
		return e.neverTyp, TypeOK
	}
	return resultTypeID, TypeOK
}
