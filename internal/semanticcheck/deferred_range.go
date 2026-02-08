package semanticcheck

import (
	"fmt"
	"sort"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateDeferredRangeFacetValues validates deferred range facets once bases resolve.
func ValidateDeferredRangeFacetValues(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedTypeQNames(sch.TypeDefs) {
		st, ok := sch.TypeDefs[qname].(*types.SimpleType)
		if !ok || st == nil || st.Restriction == nil {
			continue
		}

		baseType := st.ResolvedBase
		if baseType == nil && !st.Restriction.Base.IsZero() {
			baseType = typeops.ResolveSimpleTypeReferenceAllowMissing(sch, st.Restriction.Base)
		}
		if baseType == nil {
			continue
		}

		var (
			rangeFacets  []types.Facet
			seenDeferred bool
		)

		for _, facet := range st.Restriction.Facets {
			switch f := facet.(type) {
			case types.Facet:
				if isRangeFacetName(f.Name()) {
					rangeFacets = append(rangeFacets, f)
				}
			case *types.DeferredFacet:
				if !isRangeFacetName(f.FacetName) {
					continue
				}
				seenDeferred = true
				resolved, err := typeops.DefaultDeferredFacetConverter(f, baseType)
				if err != nil {
					errs = append(errs, fmt.Errorf("type %s: restriction: %w", qname, err))
					continue
				}
				if resolved != nil {
					rangeFacets = append(rangeFacets, resolved)
				}
			}
		}

		if !seenDeferred || len(rangeFacets) == 0 {
			continue
		}

		baseQName := st.Restriction.Base
		if baseQName.IsZero() {
			baseQName = baseType.Name()
		}
		if err := ValidateFacetConstraints(sch, rangeFacets, baseType, baseQName); err != nil {
			errs = append(errs, fmt.Errorf("type %s: restriction: %w", qname, err))
		}
	}

	return errs
}

func isRangeFacetName(name string) bool {
	switch name {
	case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive":
		return true
	default:
		return false
	}
}

func sortedTypeQNames[V any](m map[types.QName]V) []types.QName {
	out := make([]types.QName, 0, len(m))
	for qname := range m {
		out = append(out, qname)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Local < out[j].Local
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}
