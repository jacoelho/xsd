package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// ValidateDeferredRangeFacetValues validates deferred range facets once bases
// resolve.
func ValidateDeferredRangeFacetValues(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedTypeQNames(sch.TypeDefs) {
		st, ok := sch.TypeDefs[qname].(*model.SimpleType)
		if !ok || st == nil || st.Restriction == nil {
			continue
		}

		baseType := st.ResolvedBase
		if baseType == nil && !st.Restriction.Base.IsZero() {
			baseType = parser.ResolveSimpleTypeReferenceAllowMissing(sch, st.Restriction.Base)
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
				resolved, err := parser.DefaultDeferredFacetConverter(f, baseType)
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
		if err := ValidateSchemaConstraints(
			SchemaConstraintInput{
				FacetList: rangeFacets,
				BaseType:  baseType,
				BaseQName: baseQName,
			},
			SchemaConstraintCallbacks{
				ValidateRangeConsistency: ValidateRangeConsistency,
				ValidateRangeValues:      ValidateRangeValues,
				ValidateEnumerationValue: func(value string, baseType model.Type, context map[string]string) error {
					return ValidateWithFacets(sch, value, baseType, context, nil)
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
	}
	return false
}

func sortedTypeQNames[V any](m map[model.QName]V) []model.QName {
	return model.SortedMapKeys(m)
}
