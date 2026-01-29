package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// EffectiveContentParticle returns the effective element particle for a complex type.
// For derived types, it resolves restriction or extension content.
func EffectiveContentParticle(schema *parser.Schema, typ types.Type) types.Particle {
	ct, ok := types.AsComplexType(typ)
	if !ok || ct == nil {
		return nil
	}
	visited := make(map[*types.ComplexType]bool)
	return effectiveContentParticleForComplexType(schema, ct, visited)
}

func effectiveContentParticleForComplexType(schema *parser.Schema, ct *types.ComplexType, visited map[*types.ComplexType]bool) types.Particle {
	if ct == nil {
		return nil
	}
	if visited[ct] {
		return nil
	}
	visited[ct] = true
	defer delete(visited, ct)

	switch content := ct.Content().(type) {
	case *types.ElementContent:
		return content.Particle
	case *types.SimpleContent, *types.EmptyContent:
		return nil
	case *types.ComplexContent:
		if content.Restriction != nil {
			return content.Restriction.Particle
		}
		if content.Extension != nil {
			baseCT := resolveBaseComplexType(schema, ct, content.BaseTypeQName())
			baseParticle := effectiveContentParticleForComplexType(schema, baseCT, visited)
			extParticle := content.Extension.Particle
			return combineExtensionParticles(baseParticle, extParticle)
		}
	}
	return nil
}

func lookupTypeDef(schema *parser.Schema, qname types.QName) (types.Type, bool) {
	if schema == nil {
		return nil, false
	}
	typ, ok := schema.TypeDefs[qname]
	return typ, ok
}

func lookupComplexType(schema *parser.Schema, qname types.QName) (*types.ComplexType, bool) {
	typ, ok := lookupTypeDef(schema, qname)
	if !ok {
		return nil, false
	}
	ct, ok := types.AsComplexType(typ)
	return ct, ok
}

func resolveBaseComplexType(schema *parser.Schema, ct *types.ComplexType, baseQName types.QName) *types.ComplexType {
	if ct != nil && ct.ResolvedBase != nil {
		if baseCT, ok := types.AsComplexType(ct.ResolvedBase); ok {
			return baseCT
		}
		if isAnyTypeQName(ct.ResolvedBase.Name()) {
			return types.NewAnyTypeComplexType()
		}
	}
	if schema != nil && !baseQName.IsZero() {
		if isAnyTypeQName(baseQName) {
			return types.NewAnyTypeComplexType()
		}
		if baseCT, ok := lookupComplexType(schema, baseQName); ok {
			return baseCT
		}
	}
	return nil
}

func combineExtensionParticles(baseParticle, extParticle types.Particle) types.Particle {
	if baseParticle == nil {
		return extParticle
	}
	if extParticle == nil {
		return baseParticle
	}
	return &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
		Particles: []types.Particle{baseParticle, extParticle},
	}
}

func isAnyTypeQName(qname types.QName) bool {
	return qname.Namespace == types.XSDNamespace && qname.Local == string(types.TypeNameAnyType)
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

func isIDOnlyType(qname types.QName) bool {
	return qname.Namespace == types.XSDNamespace && qname.Local == string(types.TypeNameID)
}

// isIDOnlyDerivedType checks if a SimpleType is derived from ID (not IDREF/IDREFS).
func isIDOnlyDerivedType(schema *parser.Schema, st *types.SimpleType) bool {
	return isIDOnlyDerivedTypeVisited(schema, st, make(map[*types.SimpleType]bool))
}

func isIDOnlyDerivedTypeVisited(schema *parser.Schema, st *types.SimpleType, visited map[*types.SimpleType]bool) bool {
	if st == nil || st.Restriction == nil {
		return false
	}
	if visited[st] {
		return false
	}
	visited[st] = true
	defer delete(visited, st)

	baseQName := st.Restriction.Base
	if isIDOnlyType(baseQName) {
		return true
	}

	var baseType types.Type
	if st.ResolvedBase != nil {
		baseType = st.ResolvedBase
	} else if !baseQName.IsZero() {
		baseType = resolveSimpleTypeReference(schema, baseQName)
	}

	switch typed := baseType.(type) {
	case *types.SimpleType:
		return isIDOnlyDerivedTypeVisited(schema, typed, visited)
	case *types.BuiltinType:
		return isIDOnlyType(typed.Name())
	default:
		return false
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
	if df == nil || baseType == nil {
		return nil, nil
	}

	switch df.FacetName {
	case "minInclusive":
		return types.NewMinInclusive(df.FacetValue, baseType)
	case "maxInclusive":
		return types.NewMaxInclusive(df.FacetValue, baseType)
	case "minExclusive":
		return types.NewMinExclusive(df.FacetValue, baseType)
	case "maxExclusive":
		return types.NewMaxExclusive(df.FacetValue, baseType)
	default:
		return nil, fmt.Errorf("unknown deferred facet type: %s", df.FacetName)
	}
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
