package schemacheck

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typegraph"
	"github.com/jacoelho/xsd/internal/types"
)

// EffectiveContentParticle returns the effective element particle for a complex type.
// For derived types, it resolves restriction or extension content.
func EffectiveContentParticle(schema *parser.Schema, typ types.Type) types.Particle {
	return typegraph.EffectiveContentParticle(schema, typ)
}

func lookupTypeDef(schema *parser.Schema, qname types.QName) (types.Type, bool) {
	return typegraph.LookupType(schema, qname)
}

func lookupComplexType(schema *parser.Schema, qname types.QName) (*types.ComplexType, bool) {
	return typegraph.LookupComplexType(schema, qname)
}

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

// getTypeQName returns the QName of a type, or zero QName if nil
func getTypeQName(typ types.Type) types.QName {
	if typ == nil {
		return types.QName{}
	}
	return typ.Name()
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
	if df == nil || baseType == nil {
		return nil, nil
	}

	switch df.FacetName {
	case "minInclusive":
		return convertDeferredRangeFacet(df.FacetName, df.FacetValue, baseType)
	case "maxInclusive":
		return convertDeferredRangeFacet(df.FacetName, df.FacetValue, baseType)
	case "minExclusive":
		return convertDeferredRangeFacet(df.FacetName, df.FacetValue, baseType)
	case "maxExclusive":
		return convertDeferredRangeFacet(df.FacetName, df.FacetValue, baseType)
	default:
		return nil, fmt.Errorf("unknown deferred facet type: %s", df.FacetName)
	}
}

func convertDeferredRangeFacet(name, value string, baseType types.Type) (types.Facet, error) {
	var (
		facet types.Facet
		err   error
	)
	switch name {
	case "minInclusive":
		facet, err = types.NewMinInclusive(value, baseType)
	case "maxInclusive":
		facet, err = types.NewMaxInclusive(value, baseType)
	case "minExclusive":
		facet, err = types.NewMinExclusive(value, baseType)
	case "maxExclusive":
		facet, err = types.NewMaxExclusive(value, baseType)
	default:
		return nil, fmt.Errorf("unknown deferred facet type: %s", name)
	}
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
