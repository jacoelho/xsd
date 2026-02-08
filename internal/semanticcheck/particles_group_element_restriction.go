package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateModelGroupToElementRestriction validates ModelGroup:Element derivation.
// Per XSD 1.0 spec section 3.9.6 (Particle Derivation OK - RecurseAsIfGroup),
// treat the derived element as if it were wrapped in a group of the same kind
// as the base with minOccurs=maxOccurs=1, then apply recurse mapping.
// Returns false if no matching element is found (to allow fall-through).
func validateModelGroupToElementRestriction(schema *parser.Schema, baseMG *types.ModelGroup, restrictionElem *types.ElementDecl) (bool, error) {
	baseChildren := derivationChildren(baseMG)
	if len(baseChildren) == 0 {
		return false, nil
	}

	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), types.OccursFromInt(1), types.OccursFromInt(1)); err != nil {
		return false, err
	}

	if baseMG.Kind == types.Choice {
		var constraintErr error
		for _, baseParticle := range baseChildren {
			matched, err := validateGroupChildElementRestriction(schema, baseMG, baseChildren, baseParticle, restrictionElem)
			if matched && err == nil {
				return true, nil
			}
			if matched && err != nil && constraintErr == nil {
				constraintErr = err
			}
		}
		if constraintErr != nil {
			return false, constraintErr
		}
		return false, nil
	}

	current := 0
	matched := false
	var constraintErr error
	for current < len(baseChildren) {
		baseParticle := baseChildren[current]
		current++
		childMatched, err := validateGroupChildElementRestriction(schema, baseMG, baseChildren, baseParticle, restrictionElem)
		if childMatched && err == nil {
			matched = true
			break
		}
		if childMatched && err != nil && constraintErr == nil {
			constraintErr = err
		}
		if baseMG.MinOccurs.CmpInt(0) > 0 && !isEmptiableParticle(baseParticle) {
			break
		}
	}
	if !matched {
		if constraintErr != nil {
			return false, constraintErr
		}
		return false, nil
	}
	if baseMG.MinOccurs.CmpInt(0) > 0 {
		for ; current < len(baseChildren); current++ {
			if !isEmptiableParticle(baseChildren[current]) {
				return false, fmt.Errorf("ComplexContent restriction: required base particle not present in element restriction")
			}
		}
	}
	return true, nil
}

func validateGroupChildElementRestriction(schema *parser.Schema, baseMG *types.ModelGroup, baseChildren []types.Particle, baseParticle types.Particle, restrictionElem *types.ElementDecl) (bool, error) {
	switch typed := baseParticle.(type) {
	case *types.ElementDecl:
		return validateElementRestrictionWithGroupOccurrence(schema, baseMG, baseChildren, typed, restrictionElem)
	case *types.AnyElement:
		return validateWildcardRestrictionWithGroupOccurrence(baseMG, baseChildren, typed, restrictionElem)
	default:
		if err := validateParticlePairRestriction(schema, baseParticle, restrictionElem); err != nil {
			return false, nil
		}
		return true, nil
	}
}

func validateElementRestrictionWithGroupOccurrence(schema *parser.Schema, baseMG *types.ModelGroup, baseChildren []types.Particle, baseElem, restrictionElem *types.ElementDecl) (bool, error) {
	if baseElem.Name != restrictionElem.Name {
		if !isSubstitutableElement(schema, baseElem.Name, restrictionElem.Name) {
			return false, nil
		}
	}
	baseMinOcc := baseElem.MinOcc()
	baseMaxOcc := baseElem.MaxOcc()
	if baseMG != nil && baseMG.Kind == types.Choice && !baseMG.MaxOcc().IsOne() {
		baseMinOcc, baseMaxOcc = choiceChildOccurrenceRange(baseMG, baseChildren, baseElem)
	}
	if err := validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionElem.MinOcc(), restrictionElem.MaxOcc()); err != nil {
		return true, err
	}
	if restrictionElem.MinOcc().IsZero() && restrictionElem.MaxOcc().IsZero() {
		return true, nil
	}
	if err := validateElementRestriction(schema, baseElem, restrictionElem); err != nil {
		return true, err
	}
	return true, nil
}

func validateWildcardRestrictionWithGroupOccurrence(baseMG *types.ModelGroup, baseChildren []types.Particle, baseAny *types.AnyElement, restrictionElem *types.ElementDecl) (bool, error) {
	baseMinOcc := baseAny.MinOccurs
	baseMaxOcc := baseAny.MaxOccurs
	if baseMG != nil && baseMG.Kind == types.Choice && !baseMG.MaxOcc().IsOne() {
		baseMinOcc, baseMaxOcc = choiceChildOccurrenceRange(baseMG, baseChildren, baseAny)
	}
	if restrictionElem.MinOcc().IsZero() && restrictionElem.MaxOcc().IsZero() {
		if err := validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionElem.MinOcc(), restrictionElem.MaxOcc()); err != nil {
			return true, err
		}
		return true, nil
	}
	if !namespaceMatchesWildcard(restrictionElem.Name.Namespace, baseAny.Namespace, baseAny.NamespaceList, baseAny.TargetNamespace) {
		return false, nil
	}
	if err := validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionElem.MinOcc(), restrictionElem.MaxOcc()); err != nil {
		return true, err
	}
	return true, nil
}

func choiceChildOccurrenceRange(baseMG *types.ModelGroup, baseChildren []types.Particle, child types.Particle) (types.Occurs, types.Occurs) {
	childMin := child.MinOcc()
	childMax := child.MaxOcc()
	groupMin := baseMG.MinOcc()
	groupMax := baseMG.MaxOcc()

	minOcc := types.OccursFromInt(0)
	if len(baseChildren) == 1 {
		minOcc = types.MulOccurs(groupMin, childMin)
	}

	if groupMax.IsUnbounded() || childMax.IsUnbounded() {
		return minOcc, types.OccursUnbounded
	}
	return minOcc, types.MulOccurs(groupMax, childMax)
}
