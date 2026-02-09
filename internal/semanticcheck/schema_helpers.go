package semanticcheck

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// modelGroupContainsWildcard checks if a model group contains any wildcard particles
func modelGroupContainsWildcard(mg *types.ModelGroup) bool {
	for _, particle := range mg.Particles {
		if _, isWildcard := particle.(*types.AnyElement); isWildcard {
			return true
		}
		if nestedMG, isMG := particle.(*types.ModelGroup); isMG {
			if modelGroupContainsWildcard(nestedMG) {
				return true
			}
		}
	}
	return false
}

// groupKindName returns the string name of a GroupKind
func groupKindName(kind types.GroupKind) string {
	switch kind {
	case types.Sequence:
		return "sequence"
	case types.Choice:
		return "choice"
	case types.AllGroup:
		return "all"
	default:
		return "unknown"
	}
}

// validateDeferredFacetApplicability validates a deferred facet now that the base type is resolved.
// Deferred facets are range facets (min/max Inclusive/Exclusive) that couldn't be constructed
// during parsing because the base type wasn't available.
func validateDeferredFacetApplicability(df *types.DeferredFacet, baseType types.Type, baseQName types.QName) error {
	// check if facet is applicable to the base type
	switch df.FacetName {
	case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive":
		// range facets are NOT applicable to list types
		if baseType != nil {
			if baseST, ok := types.AsSimpleType(baseType); ok {
				if baseST.Variety() == types.ListVariety {
					return fmt.Errorf("facet %s is not applicable to list type %s", df.FacetName, baseQName)
				}
				if baseST.Variety() == types.UnionVariety {
					return fmt.Errorf("facet %s is not applicable to union type %s", df.FacetName, baseQName)
				}
			}
		}
	}
	return nil
}

// convertDeferredFacet converts a DeferredFacet to an actual Facet now that the base type is resolved.
// This is needed for facet inheritance validation.
func convertDeferredFacet(df *types.DeferredFacet, baseType types.Type) (types.Facet, error) {
	facet, err := typeops.DefaultDeferredFacetConverter(df, baseType)
	if errors.Is(err, types.ErrCannotDeterminePrimitiveType) {
		return nil, nil
	}
	return facet, err
}

// isNotationType checks if a type is or derives from xs:NOTATION
func isNotationType(t types.Type) bool {
	if t == nil {
		return false
	}
	primitive := t.PrimitiveType()
	if primitive == nil {
		return false
	}
	return primitive.Name().Local == string(types.TypeNameNOTATION) &&
		primitive.Name().Namespace == types.XSDNamespace
}

// hasEnumerationFacet checks if a facet list contains an enumeration facet
func hasEnumerationFacet(facetList []types.Facet) bool {
	for _, f := range facetList {
		if _, ok := f.(*types.Enumeration); ok {
			return true
		}
	}
	return false
}

// whiteSpaceName returns the string name of a WhiteSpace value
func whiteSpaceName(ws types.WhiteSpace) string {
	switch ws {
	case types.WhiteSpacePreserve:
		return "preserve"
	case types.WhiteSpaceReplace:
		return "replace"
	case types.WhiteSpaceCollapse:
		return "collapse"
	default:
		return "unknown"
	}
}
