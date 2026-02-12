package semanticcheck

import (
	"fmt"
	"sort"

	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// ValidateDeferredRangeFacetValues validates deferred range facets once bases resolve.
func ValidateDeferredRangeFacetValues(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedTypeQNames(sch.TypeDefs) {
		st, ok := sch.TypeDefs[qname].(*model.SimpleType)
		if !ok || st == nil || st.Restriction == nil {
			continue
		}

		baseType := st.ResolvedBase
		if baseType == nil && !st.Restriction.Base.IsZero() {
			baseType = typeresolve.ResolveSimpleTypeReferenceAllowMissing(sch, st.Restriction.Base)
		}
		if baseType == nil {
			continue
		}

		var (
			rangeFacets  []model.Facet
			seenDeferred bool
		)

		for _, facet := range st.Restriction.Facets {
			switch f := facet.(type) {
			case model.Facet:
				if isRangeFacetName(f.Name()) {
					rangeFacets = append(rangeFacets, f)
				}
			case *model.DeferredFacet:
				if !isRangeFacetName(f.FacetName) {
					continue
				}
				seenDeferred = true
				resolved, err := typeresolve.DefaultDeferredFacetConverter(f, baseType)
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
		if err := facetengine.ValidateSchemaConstraints(
			facetengine.SchemaConstraintInput{
				FacetList: rangeFacets,
				BaseType:  baseType,
				BaseQName: baseQName,
			},
			facetengine.SchemaConstraintCallbacks{
				ValidateRangeConsistency: facetengine.ValidateRangeConsistency,
				ValidateRangeValues:      facetengine.ValidateRangeValues,
				ValidateEnumerationValue: func(value string, baseType model.Type, context map[string]string) error {
					return validateValueAgainstTypeWithFacets(sch, value, baseType, context)
				},
			},
		); err != nil {
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

func sortedTypeQNames[V any](m map[model.QName]V) []model.QName {
	out := make([]model.QName, 0, len(m))
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
