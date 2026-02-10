package semanticcheck

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/typeops"
)

// modelGroupContainsWildcard checks if a model group contains any wildcard particles
func modelGroupContainsWildcard(mg *model.ModelGroup) bool {
	for _, particle := range mg.Particles {
		if _, isWildcard := particle.(*model.AnyElement); isWildcard {
			return true
		}
		if nestedMG, isMG := particle.(*model.ModelGroup); isMG {
			if modelGroupContainsWildcard(nestedMG) {
				return true
			}
		}
	}
	return false
}

// groupKindName returns the string name of a GroupKind
func groupKindName(kind model.GroupKind) string {
	switch kind {
	case model.Sequence:
		return "sequence"
	case model.Choice:
		return "choice"
	case model.AllGroup:
		return "all"
	default:
		return "unknown"
	}
}

// validateDeferredFacetApplicability validates a deferred facet now that the base type is resolved.
// Deferred facets are range facets (min/max Inclusive/Exclusive) that couldn't be constructed
// during parsing because the base type wasn't available.
func validateDeferredFacetApplicability(df *model.DeferredFacet, baseType model.Type, baseQName model.QName) error {
	// check if facet is applicable to the base type
	switch df.FacetName {
	case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive":
		// range facets are NOT applicable to list types
		if baseType != nil {
			if baseST, ok := model.AsSimpleType(baseType); ok {
				if baseST.Variety() == model.ListVariety {
					return fmt.Errorf("facet %s is not applicable to list type %s", df.FacetName, baseQName)
				}
				if baseST.Variety() == model.UnionVariety {
					return fmt.Errorf("facet %s is not applicable to union type %s", df.FacetName, baseQName)
				}
			}
		}
	}
	return nil
}

// convertDeferredFacet converts a DeferredFacet to an actual Facet now that the base type is resolved.
// This is needed for facet inheritance validation.
func convertDeferredFacet(df *model.DeferredFacet, baseType model.Type) (model.Facet, error) {
	facet, err := typeops.DefaultDeferredFacetConverter(df, baseType)
	if errors.Is(err, model.ErrCannotDeterminePrimitiveType) {
		return nil, nil
	}
	return facet, err
}

// isNotationType checks if a type is or derives from xs:NOTATION
func isNotationType(t model.Type) bool {
	if t == nil {
		return false
	}
	primitive := t.PrimitiveType()
	if primitive == nil {
		return false
	}
	return primitive.Name().Local == string(model.TypeNameNOTATION) &&
		primitive.Name().Namespace == model.XSDNamespace
}

// hasEnumerationFacet checks if a facet list contains an enumeration facet
func hasEnumerationFacet(facetList []model.Facet) bool {
	for _, f := range facetList {
		if _, ok := f.(*model.Enumeration); ok {
			return true
		}
	}
	return false
}

// whiteSpaceName returns the string name of a WhiteSpace value
func whiteSpaceName(ws model.WhiteSpace) string {
	switch ws {
	case model.WhiteSpacePreserve:
		return "preserve"
	case model.WhiteSpaceReplace:
		return "replace"
	case model.WhiteSpaceCollapse:
		return "collapse"
	default:
		return "unknown"
	}
}
