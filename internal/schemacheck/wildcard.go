package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateWildcardDerivation validates wildcard constraints in type derivation
func validateWildcardDerivation(schema *parser.Schema, ct *types.ComplexType) error {
	baseQName := ct.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil // no derivation
	}

	// check if base type exists
	baseCT, ok := lookupComplexType(schema, baseQName)
	if !ok {
		// base type not found or not complex
		return nil
	}

	baseWildcards := collectWildcardsFromContent(baseCT.Content())
	derivedWildcards := collectWildcardsFromContent(ct.Content())

	if ct.IsExtension() {
		// extension: new wildcards must not violate UPA with base type's particles
		// UPA violations are checked in validateUPA
		// according to spec, wildcards in extension should union with base wildcards
		// the union is checked at validation time, but we verify here that the structure is valid
		// (UPA violations are the main constraint and are checked separately)
	} else if ct.IsRestriction() {
		// restriction: derived wildcard namespace must be a subset of base wildcard namespace
		// according to spec: "Wildcard Subset" - each derived wildcard must be a subset of at least one base wildcard
		if len(baseWildcards) == 0 && len(derivedWildcards) > 0 {
			// base has no wildcard, but derived does - invalid (can't add wildcard in restriction)
			return fmt.Errorf("wildcard restriction: cannot add wildcard when base type has no wildcard")
		}
		for _, derivedWildcard := range derivedWildcards {
			foundSubset := false
			for _, baseWildcard := range baseWildcards {
				if wildcardIsSubset(derivedWildcard, baseWildcard) {
					foundSubset = true
					break
				}
			}
			if !foundSubset {
				return fmt.Errorf("wildcard restriction: derived wildcard is not a subset of any base wildcard")
			}
		}
	}

	return nil
}

// collectWildcardsFromContent collects all AnyElement wildcards from content model
func collectWildcardsFromContent(content types.Content) []*types.AnyElement {
	var result []*types.AnyElement
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			result = append(result, collectWildcardsInParticle(c.Particle)...)
		}
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			result = append(result, collectWildcardsInParticle(c.Extension.Particle)...)
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			result = append(result, collectWildcardsInParticle(c.Restriction.Particle)...)
		}
	}
	return result
}

// collectWildcardsInParticle collects all AnyElement wildcards in a particle (recursively)
func collectWildcardsInParticle(particle types.Particle) []*types.AnyElement {
	var result []*types.AnyElement
	switch p := particle.(type) {
	case *types.ModelGroup:
		// recursively collect wildcards from all particles in the model group
		for _, child := range p.Particles {
			result = append(result, collectWildcardsInParticle(child)...)
		}
	case *types.AnyElement:
		result = append(result, p)
	}
	return result
}

// wildcardIsSubset checks if wildcard1's namespace constraint is a subset of wildcard2's
// This is used for restriction validation
// w1 is a subset of w2 if every namespace that matches w1 also matches w2
func wildcardIsSubset(w1, w2 *types.AnyElement) bool {
	if !processContentsStrongerOrEqual(w1.ProcessContents, w2.ProcessContents) {
		return false
	}
	return wildcardNamespaceSubset(w1, w2)
}

func wildcardNamespaceSubset(w1, w2 *types.AnyElement) bool {
	// if w2 is ##any, w1 is always a subset (##any matches everything)
	if w2.Namespace == types.NSCAny {
		return true
	}

	// if w1 is ##any, it's only a subset if w2 is also ##any (handled above)
	if w1.Namespace == types.NSCAny {
		return false
	}

	switch w1.Namespace {
	case types.NSCList:
		for _, ns := range w1.NamespaceList {
			if !namespaceMatchesWildcard(ns, w2.Namespace, w2.NamespaceList, w2.TargetNamespace) {
				return false
			}
		}
		return true
	case types.NSCTargetNamespace:
		return namespaceMatchesWildcard(w1.TargetNamespace, w2.Namespace, w2.NamespaceList, w2.TargetNamespace)
	case types.NSCLocal:
		return namespaceMatchesWildcard(types.NamespaceEmpty, w2.Namespace, w2.NamespaceList, w2.TargetNamespace)
	case types.NSCOther:
		if w2.Namespace == types.NSCAny {
			return true
		}
		if w2.Namespace != types.NSCOther {
			return false
		}
		if w2.TargetNamespace.IsEmpty() {
			return true
		}
		return w1.TargetNamespace == w2.TargetNamespace
	default:
		return false
	}
}

// validateAnyAttributeDerivation validates anyAttribute constraints in type derivation
// According to XSD 1.0 spec:
// - For extension: anyAttribute must union with base type's anyAttribute (cos-aw-union)
// - For restriction: anyAttribute namespace constraint must be a subset of base type's anyAttribute (cos-aw-subset)
func validateAnyAttributeDerivation(schema *parser.Schema, ct *types.ComplexType) error {
	baseQName := ct.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil // no derivation
	}

	// check if base type exists
	baseCT, ok := lookupComplexType(schema, baseQName)
	if !ok {
		// base type not found or not complex
		return nil
	}

	baseAnyAttr := collectAnyAttributeFromType(schema, baseCT)
	derivedAnyAttr := collectAnyAttributeFromType(schema, ct)

	if ct.IsExtension() {
		// extension: anyAttribute must union with base anyAttribute
		// according to spec (cos-aw-union): the union must be expressible
		if baseAnyAttr != nil && derivedAnyAttr != nil {
			// both have anyAttribute - union must be expressible
			union := types.UnionAnyAttribute(derivedAnyAttr, baseAnyAttr)
			if union == nil {
				return fmt.Errorf("anyAttribute extension: union of derived and base anyAttribute is not expressible")
			}
		}
		// if only one has anyAttribute, that's fine (union with nil is the non-nil one)
	} else if ct.IsRestriction() {
		// restriction: derived anyAttribute namespace constraint must be a subset of base anyAttribute
		// according to spec (cos-aw-subset): derived namespace constraint must be subset of base
		if baseAnyAttr == nil && derivedAnyAttr != nil {
			// base has no anyAttribute, but derived does - invalid (can't add anyAttribute in restriction)
			return fmt.Errorf("anyAttribute restriction: cannot add anyAttribute when base type has no anyAttribute")
		}
		if derivedAnyAttr != nil && baseAnyAttr != nil {
			// both have anyAttribute - derived must be subset of base
			if !anyAttributeIsSubset(derivedAnyAttr, baseAnyAttr) {
				return fmt.Errorf("anyAttribute restriction: derived anyAttribute is not a valid subset of base anyAttribute")
			}
		}
	}

	return nil
}

// collectAnyAttributeFromType collects anyAttribute from a complex type
// Checks both direct anyAttribute and anyAttribute in extension/restriction
func collectAnyAttributeFromType(schema *parser.Schema, ct *types.ComplexType) *types.AnyAttribute {
	var anyAttrs []*types.AnyAttribute

	if ct.AnyAttribute() != nil {
		anyAttrs = append(anyAttrs, ct.AnyAttribute())
	}
	anyAttrs = append(anyAttrs, collectAnyAttributeFromGroups(schema, ct.AttrGroups, nil)...)

	content := ct.Content()
	if ext := content.ExtensionDef(); ext != nil {
		if ext.AnyAttribute != nil {
			anyAttrs = append(anyAttrs, ext.AnyAttribute)
		}
		anyAttrs = append(anyAttrs, collectAnyAttributeFromGroups(schema, ext.AttrGroups, nil)...)
	}
	if restr := content.RestrictionDef(); restr != nil {
		if restr.AnyAttribute != nil {
			anyAttrs = append(anyAttrs, restr.AnyAttribute)
		}
		anyAttrs = append(anyAttrs, collectAnyAttributeFromGroups(schema, restr.AttrGroups, nil)...)
	}

	if len(anyAttrs) == 0 {
		return nil
	}

	result := anyAttrs[0]
	for i := 1; i < len(anyAttrs); i++ {
		result = types.IntersectAnyAttribute(result, anyAttrs[i])
		if result == nil {
			return nil
		}
	}
	return result
}

// collectAnyAttributeFromGroups collects anyAttribute from attribute groups (recursively)
func collectAnyAttributeFromGroups(schema *parser.Schema, agRefs []types.QName, visited map[types.QName]bool) []*types.AnyAttribute {
	if visited == nil {
		visited = make(map[types.QName]bool)
	}
	var result []*types.AnyAttribute
	for _, ref := range agRefs {
		if visited[ref] {
			continue
		}
		visited[ref] = true
		ag, ok := schema.AttributeGroups[ref]
		if !ok {
			continue
		}
		if ag.AnyAttribute != nil {
			result = append(result, ag.AnyAttribute)
		}
		if len(ag.AttrGroups) > 0 {
			result = append(result, collectAnyAttributeFromGroups(schema, ag.AttrGroups, visited)...)
		}
	}
	return result
}

// anyAttributeIsSubset checks if anyAttribute1's namespace constraint is a subset of anyAttribute2's
func anyAttributeIsSubset(w1, w2 *types.AnyAttribute) bool {
	if !processContentsStrongerOrEqual(w1.ProcessContents, w2.ProcessContents) {
		return false
	}
	// if w2 is ##any, w1 is always a subset (##any matches everything)
	if w2.Namespace == types.NSCAny {
		return true
	}

	// if w1 is ##any, it's only a subset if w2 is also ##any (handled above)
	if w1.Namespace == types.NSCAny {
		return false
	}

	switch w1.Namespace {
	case types.NSCList:
		for _, ns := range w1.NamespaceList {
			if !namespaceMatchesWildcard(ns, w2.Namespace, w2.NamespaceList, w2.TargetNamespace) {
				return false
			}
		}
		return true
	case types.NSCTargetNamespace:
		return namespaceMatchesWildcard(w1.TargetNamespace, w2.Namespace, w2.NamespaceList, w2.TargetNamespace)
	case types.NSCLocal:
		return namespaceMatchesWildcard(types.NamespaceEmpty, w2.Namespace, w2.NamespaceList, w2.TargetNamespace)
	case types.NSCOther:
		if w2.Namespace == types.NSCAny {
			return true
		}
		if w2.Namespace != types.NSCOther {
			return false
		}
		if w2.TargetNamespace.IsEmpty() {
			return true
		}
		return w1.TargetNamespace == w2.TargetNamespace
	default:
		return false
	}
}

func processContentsStrongerOrEqual(derived, base types.ProcessContents) bool {
	switch base {
	case types.Strict:
		return derived == types.Strict
	case types.Lax:
		return derived == types.Lax || derived == types.Strict
	case types.Skip:
		return true
	default:
		return false
	}
}

func processContentsName(pc types.ProcessContents) string {
	switch pc {
	case types.Strict:
		return "strict"
	case types.Lax:
		return "lax"
	case types.Skip:
		return "skip"
	default:
		return "unknown"
	}
}
