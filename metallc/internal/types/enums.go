package types

import (
	"math/big"
	"slices"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/base"
)

// enumVariantDebugName is the bare variant name for a closed enum and the
// dotted fully-qualified name for a subset of an open enum.
func enumVariantDebugName(enumType EnumType, isSubset bool, variantName string) string {
	if isSubset {
		return strings.ReplaceAll(enumType.Name, "::", ".") + "." + variantName
	}
	return variantName
}

// AssignEnumDiscriminants packs every open enum's subset variants into one
// dense, whole-program discriminant pool sized by the root's integer type.
// Standalone closed enums are numbered at declaration time, so they are left
// untouched here. Runs after type checking and before IR generation.
func (e *Engine) AssignEnumDiscriminants() {
	rootSubsets := map[TypeID][]TypeID{}
	for typeID, cached := range e.env.reg.types {
		et, ok := cached.Type.Kind.(EnumType)
		if !ok || et.Root == InvalidTypeID {
			continue
		}
		rootSubsets[et.Root] = append(rootSubsets[et.Root], typeID)
	}
	roots := make([]TypeID, 0, len(rootSubsets))
	for root := range rootSubsets {
		roots = append(roots, root)
	}
	slices.SortFunc(roots, func(a, b TypeID) int { return e.cmpEnumName(a, b) })
	for _, root := range roots {
		e.assignRootPool(root, rootSubsets[root])
	}
}

func (e *Engine) assignRootPool(root TypeID, subsets []TypeID) {
	slices.SortFunc(subsets, func(a, b TypeID) int { return e.cmpEnumName(a, b) })
	backing := base.Cast[IntType](e.env.Type(base.Cast[EnumType](e.env.Type(root).Kind).Backing).Kind)
	var counter int64
	for _, subsetID := range subsets {
		et := base.Cast[EnumType](e.env.Type(subsetID).Kind)
		for i := range et.Variants {
			et.Variants[i].Discriminant = big.NewInt(counter)
			counter++
		}
	}
	if counter > 0 && big.NewInt(counter-1).Cmp(backing.Max) > 0 {
		e.diag(e.env.Type(root).Span,
			"open enum %s has %d variants across all subsets, exceeding backing type %s",
			e.env.TypeDisplay(root), counter, backing.Name)
	}
}

func (e *Engine) cmpEnumName(a, b TypeID) int {
	return strings.Compare(
		base.Cast[EnumType](e.env.Type(a).Kind).Name,
		base.Cast[EnumType](e.env.Type(b).Kind).Name,
	)
}
