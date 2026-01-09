package validation

import (
	"fmt"
	"slices"

	schema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// isValidNCName checks if a string is a valid NCName
func isValidNCName(s string) bool {
	return types.IsValidNCName(s)
}

// elementTypesCompatible checks if two element declaration types are consistent.
// Treats nil types as compatible only when both are nil (implicit anyType).
func elementTypesCompatible(a, b types.Type) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	nameA := a.Name()
	nameB := b.Name()
	if !nameA.IsZero() || !nameB.IsZero() {
		return nameA == nameB
	}

	return a == b
}

// isBlockSuperset checks if restrictionBlock is a superset of baseBlock.
// Restriction block must contain all derivation methods in base block
// (i.e., restriction cannot allow more than base).
func isBlockSuperset(restrictionBlock, baseBlock types.DerivationSet) bool {
	if baseBlock.Has(types.DerivationExtension) && !restrictionBlock.Has(types.DerivationExtension) {
		return false
	}
	if baseBlock.Has(types.DerivationRestriction) && !restrictionBlock.Has(types.DerivationRestriction) {
		return false
	}
	if baseBlock.Has(types.DerivationSubstitution) && !restrictionBlock.Has(types.DerivationSubstitution) {
		return false
	}
	return true
}

// calculateEffectiveOccurrence calculates the effective minOccurs and maxOccurs
// for a model group by considering the group's occurrence and its children.
// For sequences: effective = group.occ * sum(children.occ)
// For choices: effective = group.occ * max(children.occ) for max, group.occ * min(children.minOcc) for min
func calculateEffectiveOccurrence(mg *types.ModelGroup) (minOcc, maxOcc int) {
	groupMinOcc := mg.MinOcc()
	groupMaxOcc := mg.MaxOcc()

	if len(mg.Particles) == 0 {
		return 0, 0
	}

	switch mg.Kind {
	case types.Sequence:
		// for sequences, sum all children's occurrences
		sumMinOcc := 0
		sumMaxOcc := 0
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			sumMinOcc += childMin
			if sumMaxOcc != types.UnboundedOccurs {
				if childMax == types.UnboundedOccurs {
					sumMaxOcc = types.UnboundedOccurs
				} else {
					sumMaxOcc += childMax
				}
			}
		}
		minOcc = groupMinOcc * sumMinOcc
		if groupMaxOcc == types.UnboundedOccurs || sumMaxOcc == types.UnboundedOccurs {
			maxOcc = types.UnboundedOccurs
		} else {
			maxOcc = groupMaxOcc * sumMaxOcc
		}
	case types.Choice:
		// for choices, take the min of children's minOccurs (since only one branch is taken)
		// and max of children's maxOccurs
		childMinOcc := -1 // will be set to actual min
		childMaxOcc := 0
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			if childMax == 0 {
				continue
			}
			if childMinOcc == -1 || childMin < childMinOcc {
				childMinOcc = childMin
			}
			if childMax == types.UnboundedOccurs {
				childMaxOcc = types.UnboundedOccurs
			} else if childMaxOcc != types.UnboundedOccurs && childMax > childMaxOcc {
				childMaxOcc = childMax
			}
		}
		if childMinOcc == -1 {
			childMinOcc = 0
		}
		minOcc = groupMinOcc * childMinOcc
		if groupMaxOcc == types.UnboundedOccurs || childMaxOcc == types.UnboundedOccurs {
			maxOcc = types.UnboundedOccurs
		} else {
			maxOcc = groupMaxOcc * childMaxOcc
		}
	case types.AllGroup:
		// for all groups, sum all children (like sequence, all must appear)
		sumMinOcc := 0
		sumMaxOcc := 0
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			sumMinOcc += childMin
			if sumMaxOcc != types.UnboundedOccurs {
				if childMax == types.UnboundedOccurs {
					sumMaxOcc = types.UnboundedOccurs
				} else {
					sumMaxOcc += childMax
				}
			}
		}
		minOcc = groupMinOcc * sumMinOcc
		if groupMaxOcc == types.UnboundedOccurs || sumMaxOcc == types.UnboundedOccurs {
			maxOcc = types.UnboundedOccurs
		} else {
			maxOcc = groupMaxOcc * sumMaxOcc
		}
	default:
		minOcc = groupMinOcc
		maxOcc = groupMaxOcc
	}
	return
}

// getParticleEffectiveOccurrence gets the effective occurrence of a single particle
func getParticleEffectiveOccurrence(p types.Particle) (minOcc, maxOcc int) {
	switch particle := p.(type) {
	case *types.ModelGroup:
		return calculateEffectiveOccurrence(particle)
	case *types.ElementDecl:
		return particle.MinOcc(), particle.MaxOcc()
	case *types.AnyElement:
		return particle.MinOccurs, particle.MaxOccurs
	default:
		return p.MinOcc(), p.MaxOcc()
	}
}

// isEffectivelyOptional checks if a ModelGroup is effectively optional
// (all its particles are optional, making the group itself effectively optional)
func isEffectivelyOptional(mg *types.ModelGroup) bool {
	if len(mg.Particles) == 0 {
		return true
	}
	for _, particle := range mg.Particles {
		if particle.MinOcc() > 0 {
			return false
		}
		// recursively check nested model groups
		if nestedMG, ok := particle.(*types.ModelGroup); ok {
			if !isEffectivelyOptional(nestedMG) {
				return false
			}
		}
	}
	return true
}

// isEmptiableParticle reports whether a particle can match the empty sequence.
// Per XSD 1.0 Structures, a particle is emptiable if it can be satisfied without
// consuming any element information items.
func isEmptiableParticle(p types.Particle) bool {
	if p == nil {
		return true
	}
	// maxOccurs=0 means the particle contributes nothing.
	if p.MaxOcc() == 0 {
		return true
	}
	// minOccurs=0 means we can choose zero occurrences.
	if p.MinOcc() == 0 {
		return true
	}

	switch pt := p.(type) {
	case *types.ModelGroup:
		switch pt.Kind {
		case types.Sequence, types.AllGroup:
			for _, child := range pt.Particles {
				if !isEmptiableParticle(child) {
					return false
				}
			}
			return true
		case types.Choice:
			return slices.ContainsFunc(pt.Particles, isEmptiableParticle)
		}
	}

	return false
}

// effectiveContentParticle returns the effective element particle for a complex type.
// For derived types, this resolves restriction/extension content.
func effectiveContentParticle(schema *schema.Schema, typ types.Type) types.Particle {
	ct, ok := typ.(*types.ComplexType)
	if !ok || ct == nil {
		return nil
	}
	visited := make(map[*types.ComplexType]bool)
	return effectiveContentParticleForComplexType(schema, ct, visited)
}

func effectiveContentParticleForComplexType(schema *schema.Schema, ct *types.ComplexType, visited map[*types.ComplexType]bool) types.Particle {
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

func resolveBaseComplexType(schema *schema.Schema, ct *types.ComplexType, baseQName types.QName) *types.ComplexType {
	if ct != nil && ct.ResolvedBase != nil {
		if baseCT, ok := ct.ResolvedBase.(*types.ComplexType); ok {
			return baseCT
		}
	}
	if schema != nil && !baseQName.IsZero() {
		if baseType, ok := schema.TypeDefs[baseQName]; ok {
			if baseCT, ok := baseType.(*types.ComplexType); ok {
				return baseCT
			}
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
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{baseParticle, extParticle},
	}
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
	return qname.Namespace == types.XSDNamespace && qname.Local == "ID"
}

// isIDOnlyDerivedType checks if a SimpleType is derived from ID (not IDREF/IDREFS).
func isIDOnlyDerivedType(st *types.SimpleType) bool {
	if st == nil || st.Restriction == nil {
		return false
	}
	base := st.Restriction.Base
	return base.Namespace == types.XSDNamespace && base.Local == "ID"
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
			if baseST, ok := baseType.(*types.SimpleType); ok {
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
