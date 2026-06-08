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
	covered := make([]bool, len(union.Variants))
	type armBody struct {
		body   ast.NodeID
		typeID TypeID
	}
	var bodies []armBody

	for _, arm := range match.Arms {
		var variantTypeID TypeID
		if _, ok := e.ast.Node(arm.Pattern).Kind.(ast.TryPattern); ok {
			variantTypeID = union.Variants[0]
			variantCached, _ := e.env.cachedTypeInfo(variantTypeID)
			e.env.setNodeType(arm.Pattern, variantCached)
		} else {
			var varStatus TypeStatus
			variantTypeID, varStatus = e.Query(arm.Pattern)
			if varStatus.Failed() {
				return InvalidTypeID, TypeDepFailed
			}
		}
		matchedIdx := -1
		for i, vID := range union.Variants {
			if variantTypeID == vID {
				matchedIdx = i
				break
			}
		}
		if matchedIdx < 0 {
			e.diag(e.ast.Node(arm.Pattern).Span, "type %s is not a variant of %s",
				e.env.TypeDisplay(variantTypeID), e.env.TypeDisplay(unionTypeID))
			return InvalidTypeID, TypeFailed
		}
		if arm.Guard == nil {
			if covered[matchedIdx] {
				e.diag(e.ast.Node(arm.Pattern).Span, "duplicate match arm for variant %s",
					e.env.TypeDisplay(variantTypeID))
				return InvalidTypeID, TypeFailed
			}
			covered[matchedIdx] = true
		}
		if arm.Binding != nil {
			bindTypeID, ok := e.matchBindingType(
				e.ast.Node(arm.Pattern).Span, variantTypeID, arm.Ref, arm.Mut, matchedRefMut)
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
		if !match.Try && len(uncoveredVariants(covered, union)) == 0 {
			e.diag(span, "else is not allowed in a match on %s; it is exhaustive",
				e.env.TypeDisplay(unionTypeID))
			return InvalidTypeID, TypeFailed
		}
		if match.Else.Binding != nil {
			bindTypeID := unionTypeID
			if uncovered := uncoveredVariants(covered, union); len(uncovered) == 1 {
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
		for i, c := range covered {
			if !c {
				e.diag(span, "non-exhaustive match: missing variant %s",
					e.env.TypeDisplay(union.Variants[i]))
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
		if _, ok := e.ast.Node(match.Arms[0].Pattern).Kind.(ast.TryPattern); ok {
			e.diag(e.ast.Node(match.Arms[0].Pattern).Span,
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
		// Resolve the pattern to the variant keys it covers and the type its
		// binding takes. `IOErr.broken_pipe` is one variant, one key. A bare
		// subset like `IOErr` covers every variant of the subset.
		patTypeID, status := e.Query(arm.Pattern)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		patEnum, ok := e.env.Type(patTypeID).Kind.(EnumType)
		if !ok {
			e.diag(e.ast.Node(arm.Pattern).Span, "%s is not an enum variant or subset of %s",
				e.env.TypeDisplay(patTypeID), e.env.TypeDisplay(enumTypeID))
			return InvalidTypeID, TypeFailed
		}
		var keys []string
		if _, variant, isVariant := e.env.EnumVariantRef(arm.Pattern); isVariant {
			inFamily := patTypeID == enumTypeID
			if enum.IsOpen {
				inFamily = patEnum.Root == enumTypeID
			}
			if !inFamily {
				e.diag(e.ast.Node(arm.Pattern).Span, "%s.%s is not a variant of %s",
					e.env.TypeDisplay(patTypeID), variant, e.env.TypeDisplay(enumTypeID))
				return InvalidTypeID, TypeFailed
			}
			keys = []string{patEnum.Name + "." + variant}
		} else {
			if !enum.IsOpen || patEnum.Root != enumTypeID {
				e.diag(e.ast.Node(arm.Pattern).Span, "%s is not a subset of %s",
					e.env.TypeDisplay(patTypeID), e.env.TypeDisplay(enumTypeID))
				return InvalidTypeID, TypeFailed
			}
			keys = make([]string, len(patEnum.Variants))
			for i, v := range patEnum.Variants {
				keys[i] = patEnum.Name + "." + v.Name
			}
		}
		if arm.Binding != nil {
			bindTypeID, ok := e.matchBindingType(
				e.ast.Node(arm.Pattern).Span, patTypeID, arm.Ref, arm.Mut, matchedRefMut)
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
				e.diag(e.ast.Node(arm.Pattern).Span, "unreachable match arm: all variants already covered")
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
