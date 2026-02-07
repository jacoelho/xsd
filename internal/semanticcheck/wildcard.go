package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
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
		// UPA violations are checked in ValidateUPA
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
	return traversal.CollectFromContent(content, func(p types.Particle) (*types.AnyElement, bool) {
		wildcard, ok := p.(*types.AnyElement)
		return wildcard, ok
	})
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
	return namespaceConstraintSubset(
		w1.Namespace, w1.NamespaceList, w1.TargetNamespace,
		w2.Namespace, w2.NamespaceList, w2.TargetNamespace,
	)
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

	baseAnyAttr, err := collectAnyAttributeFromType(schema, baseCT)
	if err != nil {
		return err
	}
	derivedAnyAttr, err := collectAnyAttributeFromType(schema, ct)
	if err != nil {
		return err
	}

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
func collectAnyAttributeFromType(schema *parser.Schema, ct *types.ComplexType) (*types.AnyAttribute, error) {
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
		return nil, nil
	}

	result := anyAttrs[0]
	for i := 1; i < len(anyAttrs); i++ {
		intersected, expressible, empty := types.IntersectAnyAttributeDetailed(result, anyAttrs[i])
		if !expressible {
			return nil, fmt.Errorf("anyAttribute intersection is not expressible")
		}
		if empty {
			return nil, nil
		}
		result = intersected
	}
	return result, nil
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
	return namespaceConstraintSubset(
		w1.Namespace, w1.NamespaceList, w1.TargetNamespace,
		w2.Namespace, w2.NamespaceList, w2.TargetNamespace,
	)
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

func namespaceMatchesWildcard(ns types.NamespaceURI, constraint types.NamespaceConstraint, list []types.NamespaceURI, target types.NamespaceURI) bool {
	return types.AllowsNamespace(constraint, list, target, ns)
}

func namespaceConstraintSubset(
	ns1 types.NamespaceConstraint,
	list1 []types.NamespaceURI,
	target1 types.NamespaceURI,
	ns2 types.NamespaceConstraint,
	list2 []types.NamespaceURI,
	target2 types.NamespaceURI,
) bool {
	// if ns2 is ##any, ns1 is always a subset (##any matches everything)
	if ns2 == types.NSCAny {
		return true
	}

	// if ns1 is ##any, it's only a subset if ns2 is also ##any (handled above)
	if ns1 == types.NSCAny {
		return false
	}

	switch ns1 {
	case types.NSCList:
		for _, ns := range list1 {
			resolved := ns
			if ns == types.NamespaceTargetPlaceholder {
				resolved = target1
			}
			if !namespaceMatchesWildcard(resolved, ns2, list2, target2) {
				return false
			}
		}
		return true
	case types.NSCTargetNamespace:
		return namespaceMatchesWildcard(target1, ns2, list2, target2)
	case types.NSCLocal:
		return namespaceMatchesWildcard(types.NamespaceEmpty, ns2, list2, target2)
	case types.NSCOther:
		if ns2 == types.NSCAny || ns2 == types.NSCNotAbsent {
			return true
		}
		if ns2 != types.NSCOther {
			return false
		}
		if target2.IsEmpty() {
			return true
		}
		return target1 == target2
	case types.NSCNotAbsent:
		switch ns2 {
		case types.NSCAny, types.NSCNotAbsent:
			return true
		case types.NSCOther:
			return target2.IsEmpty()
		default:
			return false
		}
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
