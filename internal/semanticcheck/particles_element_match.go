package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
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

func validateElementPairRestriction(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) (bool, error) {
	baseElem, baseIsElem := baseParticle.(*types.ElementDecl)
	if !baseIsElem {
		return false, nil
	}
	switch restriction := restrictionParticle.(type) {
	case *types.ElementDecl:
		return true, validateElementToElementRestriction(schema, baseElem, restriction)
	case *types.ModelGroup:
		return true, validateElementToChoiceRestriction(schema, baseElem, restriction)
	default:
		return false, nil
	}
}

func validateElementToElementRestriction(schema *parser.Schema, baseElem, restrictionElem *types.ElementDecl) error {
	if restrictionElem.MinOcc().IsZero() && restrictionElem.MaxOcc().IsZero() {
		return nil
	}
	if baseElem.Name != restrictionElem.Name {
		if !isSubstitutableElement(schema, baseElem.Name, restrictionElem.Name) {
			return fmt.Errorf("ComplexContent restriction: element name mismatch (%s vs %s)", baseElem.Name, restrictionElem.Name)
		}
	}
	return validateElementRestriction(schema, baseElem, restrictionElem)
}

func validateElementToChoiceRestriction(schema *parser.Schema, baseElem *types.ElementDecl, restrictionGroup *types.ModelGroup) error {
	if restrictionGroup.Kind != types.Choice {
		return fmt.Errorf("ComplexContent restriction: cannot restrict element %s to model group", baseElem.Name)
	}
	for _, p := range restrictionGroup.Particles {
		if p.MinOcc().IsZero() && p.MaxOcc().IsZero() {
			continue
		}
		childElem, ok := p.(*types.ElementDecl)
		if !ok {
			return fmt.Errorf("ComplexContent restriction: element %s restriction choice must contain only elements", baseElem.Name)
		}
		if err := validateParticlePairRestriction(schema, baseElem, childElem); err != nil {
			return fmt.Errorf("ComplexContent restriction: element %s restriction choice contains invalid particle: %w", baseElem.Name, err)
		}
	}
	return nil
}

// validateElementRestriction validates that a restriction element properly restricts a base element.
// Per XSD 1.0 spec section 3.4.6 Constraints on Particle Schema Components:
// - nillable: If base is not nillable, restriction cannot be nillable
// - fixed: If base has fixed value, restriction must have same fixed value
// - block: Restriction block must be superset of base block (cannot allow more derivations)
// - type: Restriction type must be same as or derived from base type
func validateElementRestriction(schema *parser.Schema, baseElem, restrictionElem *types.ElementDecl) error {
	// validate nillable: cannot change from false to true
	if !baseElem.Nillable && restrictionElem.Nillable {
		return fmt.Errorf("ComplexContent restriction: element '%s' nillable cannot be true when base element nillable is false", restrictionElem.Name)
	}

	// validate fixed: if base has fixed value, restriction must have same fixed value
	// values are compared after whitespace normalization based on the element's type
	if baseElem.HasFixed {
		if !restrictionElem.HasFixed {
			return fmt.Errorf("ComplexContent restriction: element '%s' must have fixed value matching base fixed value '%s'", restrictionElem.Name, baseElem.Fixed)
		}
		baseType := ResolveTypeReference(schema, baseElem.Type, typeops.TypeReferenceAllowMissing)
		restrictionType := ResolveTypeReference(schema, restrictionElem.Type, typeops.TypeReferenceAllowMissing)
		if baseType == nil {
			baseType = baseElem.Type
		}
		if restrictionType == nil {
			restrictionType = restrictionElem.Type
		}
		// normalize fixed values using effective whitespace rules for simple/list types
		baseFixed := normalizeFixedValue(baseElem.Fixed, baseType)
		restrictionFixed := normalizeFixedValue(restrictionElem.Fixed, restrictionType)
		if baseFixed != restrictionFixed {
			return fmt.Errorf("ComplexContent restriction: element '%s' fixed value '%s' must match base fixed value '%s'", restrictionElem.Name, restrictionElem.Fixed, baseElem.Fixed)
		}
	}

	// validate block: restriction block must be superset of base block
	// (restriction cannot allow more derivation methods than base)
	if !isBlockSuperset(restrictionElem.Block, baseElem.Block) {
		return fmt.Errorf("ComplexContent restriction: element '%s' block constraint must be superset of base block constraint", restrictionElem.Name)
	}

	// validate type: restriction type must be same as or derived from base type
	baseType := ResolveTypeReference(schema, baseElem.Type, typeops.TypeReferenceAllowMissing)
	restrictionType := ResolveTypeReference(schema, restrictionElem.Type, typeops.TypeReferenceAllowMissing)
	if baseType == nil {
		baseType = baseElem.Type
	}
	if restrictionType == nil {
		restrictionType = restrictionElem.Type
	}
	if baseType == nil || restrictionType == nil {
		return nil
	}
	// types are same if they have the same QName
	baseTypeName := baseType.Name()
	restrictionTypeName := restrictionType.Name()

	if baseTypeName.Namespace == types.XSDNamespace && baseTypeName.Local == "anyType" {
		return nil
	}
	if baseTypeName.Namespace == types.XSDNamespace && baseTypeName.Local == "anySimpleType" {
		switch restrictionType.(type) {
		case *types.SimpleType, *types.BuiltinType:
			return nil
		}
	}

	// if types are the same (by name), that's always valid
	if baseTypeName == restrictionTypeName {
		return nil
	}

	// handle anonymous types (inline simpleType/complexType in restriction)
	// anonymous types may have empty names but should be derived from base
	if restrictionTypeName.Local == "" {
		// anonymous type - check if it's derived from base type
		if !isRestrictionDerivedFrom(schema, restrictionType, baseType) {
			// for anonymous types, also check if they declare base type explicitly
			// anonymous simpleTypes with restrictions are valid if their base matches
			if st, ok := restrictionType.(*types.SimpleType); ok {
				if st.Restriction != nil && st.Restriction.Base == baseTypeName {
					return nil
				}
				// check if the anonymous type derives from the base through its ResolvedBase
				if st.ResolvedBase != nil && isRestrictionDerivedFrom(schema, st.ResolvedBase, baseType) {
					return nil
				}
			}
			return fmt.Errorf("ComplexContent restriction: element '%s' anonymous type must be derived from base type '%s'", restrictionElem.Name, baseTypeName)
		}
		return nil
	}

	// if type names are different, restriction type must be derived from base type
	if !isRestrictionDerivedFrom(schema, restrictionType, baseType) {
		return fmt.Errorf("ComplexContent restriction: element '%s' type '%s' must be same as or derived from base type '%s'", restrictionElem.Name, restrictionTypeName, baseTypeName)
	}

	return nil
}

func normalizeFixedValue(value string, typ types.Type) string {
	if typ == nil {
		return value
	}
	if st, ok := typ.(*types.SimpleType); ok {
		if st.List != nil || st.Variety() == types.ListVariety {
			return types.ApplyWhiteSpace(value, types.WhiteSpaceCollapse)
		}
		if st.Restriction != nil && !st.Restriction.Base.IsZero() &&
			st.Restriction.Base.Namespace == types.XSDNamespace &&
			types.IsBuiltinListTypeName(st.Restriction.Base.Local) {
			return types.ApplyWhiteSpace(value, types.WhiteSpaceCollapse)
		}
	}
	if bt, ok := typ.(*types.BuiltinType); ok && types.IsBuiltinListTypeName(bt.Name().Local) {
		return types.ApplyWhiteSpace(value, types.WhiteSpaceCollapse)
	}
	return types.NormalizeWhiteSpace(value, typ)
}
