package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateWildcardToElementRestriction validates Element:Wildcard derivation
// When base is a wildcard and restriction is an element, this is valid if:
// - Element's namespace is allowed by wildcard's namespace constraint
// - Element's occurrence constraints are within wildcard's constraints
func validateWildcardToElementRestriction(baseAny *types.AnyElement, restrictionElem *types.ElementDecl) error {
	if restrictionElem.MinOcc().IsZero() && restrictionElem.MaxOcc().IsZero() {
		return validateOccurrenceConstraints(baseAny.MinOccurs, baseAny.MaxOccurs, restrictionElem.MinOcc(), restrictionElem.MaxOcc())
	}
	// check namespace constraint: element namespace must be allowed by wildcard
	elemNS := restrictionElem.Name.Namespace
	if !types.AllowsNamespace(baseAny.Namespace, baseAny.NamespaceList, baseAny.TargetNamespace, elemNS) {
		return fmt.Errorf("ComplexContent restriction: element namespace %q not allowed by base wildcard", elemNS)
	}

	return validateOccurrenceConstraints(baseAny.MinOccurs, baseAny.MaxOccurs, restrictionElem.MinOcc(), restrictionElem.MaxOcc())
}

// validateWildcardToModelGroupRestriction validates ModelGroup:Wildcard derivation
// When base is a wildcard and restriction is a model group, we calculate the effective
// occurrence of the model group's content and validate against the wildcard constraints.
func validateWildcardToModelGroupRestriction(schema *parser.Schema, baseAny *types.AnyElement, restrictionMG *types.ModelGroup) error {
	if err := validateWildcardNamespaceRestriction(schema, baseAny, restrictionMG, newModelGroupVisit(), make(map[types.QName]bool)); err != nil {
		return err
	}
	// calculate effective occurrence by recursively finding the total minOccurs/maxOccurs
	// of elements within the model group
	effectiveMinOcc, effectiveMaxOcc := calculateEffectiveOccurrence(restrictionMG)
	return validateOccurrenceConstraints(baseAny.MinOccurs, baseAny.MaxOccurs, effectiveMinOcc, effectiveMaxOcc)
}

func validateWildcardNamespaceRestriction(schema *parser.Schema, baseAny *types.AnyElement, particle types.Particle, visitedMG modelGroupVisit, visitedGroups map[types.QName]bool) error {
	if particle != nil && particle.MinOcc().IsZero() && particle.MaxOcc().IsZero() {
		return nil
	}
	switch p := particle.(type) {
	case *types.ModelGroup:
		if !visitedMG.Enter(p) {
			return nil
		}
		for _, child := range p.Particles {
			if err := validateWildcardNamespaceRestriction(schema, baseAny, child, visitedMG, visitedGroups); err != nil {
				return err
			}
		}
	case *types.GroupRef:
		if schema == nil {
			return nil
		}
		if visitedGroups[p.RefQName] {
			return nil
		}
		visitedGroups[p.RefQName] = true
		if group, ok := schema.Groups[p.RefQName]; ok {
			return validateWildcardNamespaceRestriction(schema, baseAny, group, visitedMG, visitedGroups)
		}
	case *types.ElementDecl:
		if !namespaceMatchesWildcard(p.Name.Namespace, baseAny.Namespace, baseAny.NamespaceList, baseAny.TargetNamespace) {
			return fmt.Errorf("ComplexContent restriction: element namespace %q not allowed by base wildcard", p.Name.Namespace)
		}
	case *types.AnyElement:
		if err := validateWildcardToWildcardRestriction(baseAny, p); err != nil {
			return err
		}
	}
	return nil
}

// validateWildcardToWildcardRestriction validates Wildcard:Wildcard derivation
// Namespace constraint in restriction must be a subset of base, and processContents
// in restriction must be identical or stronger than base.
func validateWildcardToWildcardRestriction(baseAny, restrictionAny *types.AnyElement) error {
	if !processContentsStrongerOrEqual(restrictionAny.ProcessContents, baseAny.ProcessContents) {
		return fmt.Errorf(
			"ComplexContent restriction: wildcard restriction: processContents in restriction must be identical or stronger than base (base is %s, restriction is %s)",
			processContentsName(baseAny.ProcessContents),
			processContentsName(restrictionAny.ProcessContents),
		)
	}
	if !wildcardNamespaceSubset(restrictionAny, baseAny) {
		return fmt.Errorf("ComplexContent restriction: wildcard restriction: wildcard is not a subset of base wildcard")
	}
	return nil
}

func validateWildcardBaseRestriction(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) (bool, error) {
	baseAny, baseIsAny := baseParticle.(*types.AnyElement)
	if !baseIsAny {
		return false, nil
	}
	if restrictionElem, ok := restrictionParticle.(*types.ElementDecl); ok {
		return true, validateWildcardToElementRestriction(baseAny, restrictionElem)
	}
	if restrictionMG, ok := restrictionParticle.(*types.ModelGroup); ok {
		return true, validateWildcardToModelGroupRestriction(schema, baseAny, restrictionMG)
	}
	return false, nil
}

func validateWildcardPairRestriction(baseParticle, restrictionParticle types.Particle) (bool, error) {
	baseAny, baseIsAny := baseParticle.(*types.AnyElement)
	restrictionAny, restrictionIsAny := restrictionParticle.(*types.AnyElement)
	if !baseIsAny && !restrictionIsAny {
		return false, nil
	}
	switch {
	case baseIsAny && restrictionIsAny:
		return true, validateWildcardToWildcardRestriction(baseAny, restrictionAny)
	case baseIsAny && !restrictionIsAny:
		return true, nil
	case !baseIsAny && restrictionIsAny:
		return true, fmt.Errorf("ComplexContent restriction: cannot restrict non-wildcard to wildcard")
	}
	return true, nil
}
