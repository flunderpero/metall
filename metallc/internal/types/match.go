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
	exprTyp := e.env.Type(exprTypeID)
	union, ok := exprTyp.Kind.(UnionType)
	if !ok {
		e.diag(e.ast.Node(match.Expr).Span, "match expression must be a union type, got %s",
			e.env.TypeDisplay(exprTypeID))
		return InvalidTypeID, TypeFailed
	}
	if len(match.Arms) == 0 && match.Else == nil {
		e.diag(span, "match requires at least one arm")
		return InvalidTypeID, TypeFailed
	}
	return e.checkMatchArms(match, union, exprTypeID, span, typeHint)
}

func (e *Engine) checkMatchArms( //nolint:funlen
	match ast.Match, union UnionType, unionTypeID TypeID, span base.Span, typeHint *TypeID,
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
			e.env.setNodeType(arm.Pattern, e.env.reg.types[variantTypeID])
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
			bodyScope := e.scopeGraph.IntroducedScope(arm.Body)
			e.env.bindInScope(bodyScope, arm.Body, arm.Binding.Name, variantTypeID)
		}
		if arm.Guard != nil {
			guardTypeID, guardStatus := e.Query(*arm.Guard)
			if guardStatus.Failed() {
				return InvalidTypeID, TypeDepFailed
			}
			if guardTypeID != e.boolTyp {
				e.diag(e.ast.Node(*arm.Guard).Span, "guard condition must be Bool, got %s",
					e.env.TypeDisplay(guardTypeID))
				return InvalidTypeID, TypeFailed
			}
		}
		bodies = append(bodies, armBody{arm.Body, InvalidTypeID})
	}
	if match.Else != nil {
		if match.Else.Binding != nil {
			bindTypeID := unionTypeID
			if uncovered := uncoveredVariants(covered, union); len(uncovered) == 1 {
				bindTypeID = uncovered[0]
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
		// type hint — it yields the unwrapped value, not the Result.
		if match.Try && i < len(match.Arms) {
			hint = nil
		}
		bodyTypeID, bodyStatus := e.queryWithHint(ab.body, hint)
		if bodyStatus.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.ast.BlockBreaksControlFlow(ab.body, false) {
			bodies[i].typeID = bodyTypeID
		}
	}
	if match.Try {
		if match.Else == nil {
			panic(base.Errorf("try must have an else branch"))
		}
		if !e.ast.BlockBreaksControlFlow(match.Else.Body, false) {
			e.diag(e.ast.Node(match.Else.Body).Span, "try else block must break control flow")
			return InvalidTypeID, TypeFailed
		}
	}

	var resultTypeID TypeID
	for _, ab := range bodies {
		if ab.typeID == InvalidTypeID {
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
		return e.voidTyp, TypeOK
	}
	return resultTypeID, TypeOK
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
