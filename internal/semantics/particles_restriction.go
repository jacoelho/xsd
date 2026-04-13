package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateElementPairRestriction(schema *parser.Schema, baseParticle, restrictionParticle model.Particle) (bool, error) {
	baseElem, baseIsElem := baseParticle.(*model.ElementDecl)
	if !baseIsElem {
		return false, nil
	}
	switch restriction := restrictionParticle.(type) {
	case *model.ElementDecl:
		return true, validateElementToElementRestriction(schema, baseElem, restriction)
	case *model.ModelGroup:
		return true, validateElementToChoiceRestriction(schema, baseElem, restriction)
	default:
		return false, nil
	}
}

func validateElementToElementRestriction(schema *parser.Schema, baseElem, restrictionElem *model.ElementDecl) error {
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

func validateElementToChoiceRestriction(schema *parser.Schema, baseElem *model.ElementDecl, restrictionGroup *model.ModelGroup) error {
	if restrictionGroup.Kind != model.Choice {
		return fmt.Errorf("ComplexContent restriction: cannot restrict element %s to model group", baseElem.Name)
	}
	for _, p := range restrictionGroup.Particles {
		if p.MinOcc().IsZero() && p.MaxOcc().IsZero() {
			continue
		}
		childElem, ok := p.(*model.ElementDecl)
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
func validateElementRestriction(schema *parser.Schema, baseElem, restrictionElem *model.ElementDecl) error {
	if !baseElem.Nillable && restrictionElem.Nillable {
		return fmt.Errorf("ComplexContent restriction: element '%s' nillable cannot be true when base element nillable is false", restrictionElem.Name)
	}

	if baseElem.HasFixed {
		if !restrictionElem.HasFixed {
			return fmt.Errorf("ComplexContent restriction: element '%s' must have fixed value matching base fixed value '%s'", restrictionElem.Name, baseElem.Fixed)
		}
		baseType := effectiveElementType(schema, baseElem)
		restrictionType := effectiveElementType(schema, restrictionElem)
		baseFixed := normalizeFixedValue(baseElem.Fixed, baseType)
		restrictionFixed := normalizeFixedValue(restrictionElem.Fixed, restrictionType)
		if baseFixed != restrictionFixed {
			return fmt.Errorf("ComplexContent restriction: element '%s' fixed value '%s' must match base fixed value '%s'", restrictionElem.Name, restrictionElem.Fixed, baseElem.Fixed)
		}
	}

	if !isBlockSuperset(restrictionElem.Block, baseElem.Block) {
		return fmt.Errorf("ComplexContent restriction: element '%s' block constraint must be superset of base block constraint", restrictionElem.Name)
	}

	baseType := effectiveElementType(schema, baseElem)
	restrictionType := effectiveElementType(schema, restrictionElem)
	if baseType == nil || restrictionType == nil {
		return nil
	}
	baseTypeName := baseType.Name()
	restrictionTypeName := restrictionType.Name()

	if baseTypeName.Namespace == model.XSDNamespace && baseTypeName.Local == "anyType" {
		return nil
	}
	if baseTypeName.Namespace == model.XSDNamespace && baseTypeName.Local == "anySimpleType" {
		switch restrictionType.(type) {
		case *model.SimpleType, *model.BuiltinType:
			return nil
		}
	}

	if baseTypeName == restrictionTypeName {
		return nil
	}

	if restrictionTypeName.Local == "" {
		if !isRestrictionDerivedFrom(schema, restrictionType, baseType) {
			if st, ok := restrictionType.(*model.SimpleType); ok {
				if st.Restriction != nil && st.Restriction.Base == baseTypeName {
					return nil
				}
				if st.ResolvedBase != nil && isRestrictionDerivedFrom(schema, st.ResolvedBase, baseType) {
					return nil
				}
			}
			return fmt.Errorf("ComplexContent restriction: element '%s' anonymous type must be derived from base type '%s'", restrictionElem.Name, baseTypeName)
		}
		return nil
	}

	if !isRestrictionDerivedFrom(schema, restrictionType, baseType) {
		return fmt.Errorf("ComplexContent restriction: element '%s' type '%s' must be same as or derived from base type '%s'", restrictionElem.Name, restrictionTypeName, baseTypeName)
	}

	return nil
}

func effectiveElementType(schema *parser.Schema, elem *model.ElementDecl) model.Type {
	if elem == nil {
		return nil
	}
	resolved := parser.ResolveTypeReferenceAllowMissing(schema, elem.Type)
	if resolved != nil {
		return resolved
	}
	return elem.Type
}

func normalizeFixedValue(value string, typ model.Type) string {
	if typ == nil {
		return value
	}
	if st, ok := typ.(*model.SimpleType); ok {
		if st.List != nil || st.Variety() == model.ListVariety {
			return model.ApplyWhiteSpace(value, model.WhiteSpaceCollapse)
		}
		if st.Restriction != nil && !st.Restriction.Base.IsZero() &&
			st.Restriction.Base.Namespace == model.XSDNamespace &&
			model.IsBuiltinListTypeName(st.Restriction.Base.Local) {
			return model.ApplyWhiteSpace(value, model.WhiteSpaceCollapse)
		}
	}
	if bt, ok := typ.(*model.BuiltinType); ok && model.IsBuiltinListTypeName(bt.Name().Local) {
		return model.ApplyWhiteSpace(value, model.WhiteSpaceCollapse)
	}
	return model.NormalizeWhiteSpace(value, typ)
}

// validateModelGroupToElementRestriction validates ModelGroup:Element derivation.
func validateModelGroupToElementRestriction(schema *parser.Schema, baseMG *model.ModelGroup, restrictionElem *model.ElementDecl) (bool, error) {
	baseChildren := derivationChildren(baseMG)
	if len(baseChildren) == 0 {
		return false, nil
	}

	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), model.OccursFromInt(1), model.OccursFromInt(1)); err != nil {
		return false, err
	}

	if baseMG.Kind == model.Choice {
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

func validateGroupChildElementRestriction(schema *parser.Schema, baseMG *model.ModelGroup, baseChildren []model.Particle, baseParticle model.Particle, restrictionElem *model.ElementDecl) (bool, error) {
	switch typed := baseParticle.(type) {
	case *model.ElementDecl:
		return validateElementRestrictionWithGroupOccurrence(schema, baseMG, baseChildren, typed, restrictionElem)
	case *model.AnyElement:
		return validateWildcardRestrictionWithGroupOccurrence(baseMG, baseChildren, typed, restrictionElem)
	default:
		if err := validateParticlePairRestriction(schema, baseParticle, restrictionElem); err != nil {
			return false, nil
		}
		return true, nil
	}
}

func validateElementRestrictionWithGroupOccurrence(schema *parser.Schema, baseMG *model.ModelGroup, baseChildren []model.Particle, baseElem, restrictionElem *model.ElementDecl) (bool, error) {
	if baseElem.Name != restrictionElem.Name {
		if !isSubstitutableElement(schema, baseElem.Name, restrictionElem.Name) {
			return false, nil
		}
	}
	baseMinOcc := baseElem.MinOcc()
	baseMaxOcc := baseElem.MaxOcc()
	if baseMG != nil && baseMG.Kind == model.Choice && !baseMG.MaxOcc().IsOne() {
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

func validateWildcardRestrictionWithGroupOccurrence(baseMG *model.ModelGroup, baseChildren []model.Particle, baseAny *model.AnyElement, restrictionElem *model.ElementDecl) (bool, error) {
	baseMinOcc := baseAny.MinOccurs
	baseMaxOcc := baseAny.MaxOccurs
	if baseMG != nil && baseMG.Kind == model.Choice && !baseMG.MaxOcc().IsOne() {
		baseMinOcc, baseMaxOcc = choiceChildOccurrenceRange(baseMG, baseChildren, baseAny)
	}
	if restrictionElem.MinOcc().IsZero() && restrictionElem.MaxOcc().IsZero() {
		if err := validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionElem.MinOcc(), restrictionElem.MaxOcc()); err != nil {
			return true, err
		}
		return true, nil
	}
	if !model.AllowsNamespace(baseAny.Namespace, baseAny.NamespaceList, baseAny.TargetNamespace, restrictionElem.Name.Namespace) {
		return false, nil
	}
	if err := validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionElem.MinOcc(), restrictionElem.MaxOcc()); err != nil {
		return true, err
	}
	return true, nil
}

func choiceChildOccurrenceRange(baseMG *model.ModelGroup, baseChildren []model.Particle, child model.Particle) (model.Occurs, model.Occurs) {
	childMin := child.MinOcc()
	childMax := child.MaxOcc()
	groupMin := baseMG.MinOcc()
	groupMax := baseMG.MaxOcc()

	minOcc := model.OccursFromInt(0)
	if len(baseChildren) == 1 {
		minOcc = model.MulOccurs(groupMin, childMin)
	}

	if groupMax.IsUnbounded() || childMax.IsUnbounded() {
		return minOcc, model.OccursUnbounded
	}
	return minOcc, model.MulOccurs(groupMax, childMax)
}

func validateParticleRestrictionWithKindChange(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup) error {
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
	baseHasWildcard := modelGroupContainsWildcard(baseMG)

	if baseHasWildcard {
		return validateKindChangeWithWildcard(schema, baseChildren, restrictionMG, restrictionChildren)
	}

	if handled, err := validateAllGroupKindChange(schema, baseMG, restrictionMG, baseChildren, restrictionChildren); handled {
		return err
	}

	if handled, err := validateSequenceToChoiceRestriction(baseMG, restrictionMG); handled {
		return err
	}

	if baseMG.Kind == model.Choice && restrictionMG.Kind == model.Sequence {
		return validateChoiceToSequenceRestriction(schema, baseMG, restrictionMG, baseChildren, restrictionChildren)
	}

	return fmt.Errorf("ComplexContent restriction: invalid model group kind change from %s to %s", groupKindName(baseMG.Kind), groupKindName(restrictionMG.Kind))
}

func validateSequenceToChoiceRestriction(baseMG, restrictionMG *model.ModelGroup) (bool, error) {
	if baseMG.Kind != model.Sequence || restrictionMG.Kind != model.Choice {
		return false, nil
	}
	return true, fmt.Errorf("ComplexContent restriction: cannot restrict sequence to choice")
}

func validateKindChangeWithWildcard(schema *parser.Schema, baseChildren []model.Particle, restrictionMG *model.ModelGroup, restrictionChildren []model.Particle) error {
	for _, baseParticle := range baseChildren {
		if baseWildcard, isWildcard := baseParticle.(*model.AnyElement); isWildcard {
			if err := validateParticlePairRestriction(schema, baseWildcard, restrictionMG); err == nil {
				return nil
			}
		}
	}
	for _, restrictionParticle := range restrictionChildren {
		found := false
		for _, baseParticle := range baseChildren {
			if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
		}
	}
	return nil
}

func validateAllGroupKindChange(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup, baseChildren, restrictionChildren []model.Particle) (bool, error) {
	if restrictionMG.Kind == model.AllGroup && baseMG.Kind != model.AllGroup {
		if len(restrictionChildren) == 1 {
			restrictionParticle := restrictionChildren[0]
			for _, baseParticle := range baseChildren {
				if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
					return true, nil
				}
			}
			return true, fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
		}
		return true, fmt.Errorf("ComplexContent restriction: cannot restrict %s to xs:all", groupKindName(baseMG.Kind))
	}

	if baseMG.Kind == model.AllGroup && restrictionMG.Kind != model.AllGroup {
		for _, restrictionParticle := range restrictionChildren {
			found := false
			for _, baseParticle := range baseChildren {
				if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
					found = true
					break
				}
			}
			if !found {
				return true, fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
			}
		}
		return true, nil
	}

	return false, nil
}

func validateChoiceToSequenceRestriction(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup, baseChildren, restrictionChildren []model.Particle) error {
	derivedCount := len(restrictionChildren)
	countOccurs := model.OccursFromInt(derivedCount)
	derivedMin := model.MulOccurs(restrictionMG.MinOccurs, countOccurs)
	derivedMax := model.MulOccurs(restrictionMG.MaxOccurs, countOccurs)
	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), derivedMin, derivedMax); err != nil {
		return err
	}
	for _, restrictionParticle := range restrictionChildren {
		found := false
		for _, baseParticle := range baseChildren {
			if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle in choice")
		}
	}
	return nil
}

// validateParticleRestriction validates that particles in a restriction are valid restrictions of base particles.
func validateParticleRestriction(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup) error {
	if baseMG.MaxOcc().IsZero() && restrictionMG.MaxOcc().IsZero() {
		return nil
	}
	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), restrictionMG.MinOcc(), restrictionMG.MaxOcc()); err != nil {
		return err
	}
	if err := validateSingleWildcardGroupRestriction(schema, baseMG, restrictionMG); err != nil {
		return err
	}
	if baseMG.Kind != restrictionMG.Kind {
		return validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
	}
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
	switch baseMG.Kind {
	case model.Sequence:
		return validateSequenceRestriction(schema, baseChildren, restrictionChildren)
	case model.Choice:
		return validateChoiceRestriction(schema, baseChildren, restrictionChildren)
	case model.AllGroup:
		return validateAllGroupRestriction(schema, baseMG, restrictionMG)
	}
	return nil
}

func validateSingleWildcardGroupRestriction(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup) error {
	if len(baseMG.Particles) != 1 {
		return nil
	}
	baseAny, ok := baseMG.Particles[0].(*model.AnyElement)
	if !ok {
		return nil
	}
	return validateParticlePairRestriction(schema, baseAny, restrictionMG)
}

func validateSequenceRestriction(schema *parser.Schema, baseChildren, restrictionChildren []model.Particle) error {
	baseIdx := 0
	matchedBaseParticles := make(map[int]bool)
	for _, restrictionParticle := range restrictionChildren {
		if restrictionParticle.MaxOcc().IsZero() && restrictionParticle.MinOcc().IsZero() {
			continue
		}
		found := false
		for baseIdx < len(baseChildren) {
			baseParticle := baseChildren[baseIdx]
			err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle)
			if err == nil {
				matchedBaseParticles[baseIdx] = true
				if baseAny, isWildcard := baseParticle.(*model.AnyElement); isWildcard {
					if baseAny.MaxOccurs.IsOne() {
						baseIdx++
					}
				} else {
					baseIdx++
				}
				found = true
				break
			}
			skippable := baseParticle.MinOcc().IsZero()
			if !skippable {
				if baseGroup, ok := baseParticle.(*model.ModelGroup); ok {
					skippable = isEffectivelyOptional(baseGroup)
				}
			}
			if skippable {
				baseIdx++
				continue
			}
			return err
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle")
		}
	}
	for i := baseIdx; i < len(baseChildren); i++ {
		baseParticle := baseChildren[i]
		if matchedBaseParticles[i] {
			continue
		}
		if baseParticle.MinOcc().CmpInt(0) > 0 {
			if baseMG2, ok := baseParticle.(*model.ModelGroup); ok && isEffectivelyOptional(baseMG2) {
				continue
			}
			return fmt.Errorf("ComplexContent restriction: required particle at position %d is missing", i)
		}
	}
	return nil
}

func validateChoiceRestriction(schema *parser.Schema, baseChildren, restrictionChildren []model.Particle) error {
	baseIdx := 0
	for _, restrictionParticle := range restrictionChildren {
		if restrictionParticle.MaxOcc().IsZero() && restrictionParticle.MinOcc().IsZero() {
			continue
		}
		found := false
		for baseIdx < len(baseChildren) {
			baseParticle := baseChildren[baseIdx]
			baseIdx++
			if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle in choice")
		}
	}
	return nil
}

func validateAllGroupRestriction(schema *parser.Schema, baseMG, restrictionMG *model.ModelGroup) error {
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)

	baseIdx := 0
	for _, restrictionParticle := range restrictionChildren {
		if restrictionParticle.MaxOcc().IsZero() && restrictionParticle.MinOcc().IsZero() {
			continue
		}
		found := false
		for baseIdx < len(baseChildren) {
			baseParticle := baseChildren[baseIdx]
			baseIdx++
			err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle)
			if err == nil {
				found = true
				break
			}
			skippable := baseParticle.MinOcc().IsZero()
			if !skippable {
				if baseGroup, ok := baseParticle.(*model.ModelGroup); ok {
					skippable = isEffectivelyOptional(baseGroup)
				}
			}
			if !skippable {
				return err
			}
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle in all group")
		}
	}
	for i := baseIdx; i < len(baseChildren); i++ {
		baseParticle := baseChildren[i]
		if baseParticle.MinOcc().CmpInt(0) > 0 {
			if baseGroup, ok := baseParticle.(*model.ModelGroup); ok && isEffectivelyOptional(baseGroup) {
				continue
			}
			return fmt.Errorf("ComplexContent restriction: required particle at position %d is missing", i)
		}
	}
	return nil
}

// validateOccurrenceConstraints validates occurrence constraints for particle restrictions.
func validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc model.Occurs) error {
	if baseMinOcc.IsOverflow() || baseMaxOcc.IsOverflow() || restrictionMinOcc.IsOverflow() || restrictionMaxOcc.IsOverflow() {
		return fmt.Errorf("%w: occurrence value exceeds uint32", model.ErrOccursOverflow)
	}
	if restrictionMinOcc.Cmp(baseMinOcc) < 0 {
		return fmt.Errorf("ComplexContent restriction: minOccurs (%s) must be >= base minOccurs (%s)", restrictionMinOcc, baseMinOcc)
	}
	if !baseMaxOcc.IsUnbounded() {
		if restrictionMaxOcc.IsUnbounded() {
			return fmt.Errorf("ComplexContent restriction: maxOccurs cannot be unbounded when base maxOccurs is bounded (%s)", baseMaxOcc)
		}
		if restrictionMaxOcc.Cmp(baseMaxOcc) > 0 {
			return fmt.Errorf("ComplexContent restriction: maxOccurs (%s) must be <= base maxOccurs (%s)", restrictionMaxOcc, baseMaxOcc)
		}
		if restrictionMinOcc.Cmp(baseMaxOcc) > 0 {
			return fmt.Errorf("ComplexContent restriction: minOccurs (%s) must be <= base maxOccurs (%s)", restrictionMinOcc, baseMaxOcc)
		}
	}
	return nil
}

func effectiveParticleOccurrence(baseParticle, restrictionParticle model.Particle) (model.Occurs, model.Occurs, model.Occurs, model.Occurs) {
	baseMinOcc := baseParticle.MinOcc()
	baseMaxOcc := baseParticle.MaxOcc()
	restrictionMinOcc := restrictionParticle.MinOcc()
	restrictionMaxOcc := restrictionParticle.MaxOcc()
	if baseMG, ok := baseParticle.(*model.ModelGroup); ok {
		if restrictionMG, ok := restrictionParticle.(*model.ModelGroup); ok {
			baseMinOcc, baseMaxOcc = calculateEffectiveOccurrence(baseMG)
			restrictionMinOcc, restrictionMaxOcc = calculateEffectiveOccurrence(restrictionMG)
		}
	}
	return baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc
}

func validateParticlePairOccurrence(baseParticle, restrictionParticle model.Particle) error {
	baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc := effectiveParticleOccurrence(baseParticle, restrictionParticle)
	return validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc)
}

// validateParticlePairRestriction validates that a restriction particle is a valid restriction of a base particle.
func validateParticlePairRestriction(schema *parser.Schema, baseParticle, restrictionParticle model.Particle) error {
	baseParticle = normalizePointlessParticle(baseParticle)
	restrictionParticle = normalizePointlessParticle(restrictionParticle)

	if handled, err := validateWildcardBaseRestriction(schema, baseParticle, restrictionParticle); handled {
		return err
	}
	if handled, err := validateModelGroupElementRestriction(schema, baseParticle, restrictionParticle); handled {
		return err
	}
	if err := validateParticlePairOccurrence(baseParticle, restrictionParticle); err != nil {
		return err
	}
	if handled, err := validateWildcardPairRestriction(baseParticle, restrictionParticle); handled {
		return err
	}
	if handled, err := validateElementPairRestriction(schema, baseParticle, restrictionParticle); handled {
		return err
	}
	if handled, err := validateModelGroupPairRestriction(schema, baseParticle, restrictionParticle); handled {
		return err
	}
	return nil
}

func validateModelGroupElementRestriction(schema *parser.Schema, baseParticle, restrictionParticle model.Particle) (bool, error) {
	baseMG, baseIsMG := baseParticle.(*model.ModelGroup)
	restrictionElem, restrictionIsElem := restrictionParticle.(*model.ElementDecl)
	if !baseIsMG || !restrictionIsElem {
		return false, nil
	}
	matched, err := validateModelGroupToElementRestriction(schema, baseMG, restrictionElem)
	if err != nil {
		return true, err
	}
	if matched {
		return true, nil
	}
	return true, fmt.Errorf("ComplexContent restriction: element %s does not match any element in base model group", restrictionElem.Name)
}

func validateModelGroupPairRestriction(schema *parser.Schema, baseParticle, restrictionParticle model.Particle) (bool, error) {
	baseMG, baseIsMG := baseParticle.(*model.ModelGroup)
	restrictionMG, restrictionIsMG := restrictionParticle.(*model.ModelGroup)
	if !baseIsMG || !restrictionIsMG {
		return false, nil
	}
	if baseMG.Kind != restrictionMG.Kind {
		return true, validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
	}
	return true, validateParticleRestriction(schema, baseMG, restrictionMG)
}

// validateWildcardToElementRestriction validates Element:Wildcard derivation.
func validateWildcardToElementRestriction(baseAny *model.AnyElement, restrictionElem *model.ElementDecl) error {
	if restrictionElem.MinOcc().IsZero() && restrictionElem.MaxOcc().IsZero() {
		return validateOccurrenceConstraints(baseAny.MinOccurs, baseAny.MaxOccurs, restrictionElem.MinOcc(), restrictionElem.MaxOcc())
	}
	elemNS := restrictionElem.Name.Namespace
	if !model.AllowsNamespace(baseAny.Namespace, baseAny.NamespaceList, baseAny.TargetNamespace, elemNS) {
		return fmt.Errorf("ComplexContent restriction: element namespace %q not allowed by base wildcard", elemNS)
	}
	return validateOccurrenceConstraints(baseAny.MinOccurs, baseAny.MaxOccurs, restrictionElem.MinOcc(), restrictionElem.MaxOcc())
}

// validateWildcardToModelGroupRestriction validates ModelGroup:Wildcard derivation.
func validateWildcardToModelGroupRestriction(schema *parser.Schema, baseAny *model.AnyElement, restrictionMG *model.ModelGroup) error {
	if err := validateWildcardNamespaceRestriction(schema, baseAny, restrictionMG, newModelGroupVisit(), make(map[model.QName]bool)); err != nil {
		return err
	}
	effectiveMinOcc, effectiveMaxOcc := calculateEffectiveOccurrence(restrictionMG)
	return validateOccurrenceConstraints(baseAny.MinOccurs, baseAny.MaxOccurs, effectiveMinOcc, effectiveMaxOcc)
}

func validateWildcardNamespaceRestriction(schema *parser.Schema, baseAny *model.AnyElement, particle model.Particle, visitedMG modelGroupVisitTracker, visitedGroups map[model.QName]bool) error {
	if particle != nil && particle.MinOcc().IsZero() && particle.MaxOcc().IsZero() {
		return nil
	}
	switch p := particle.(type) {
	case *model.ModelGroup:
		if !visitedMG.Enter(p) {
			return nil
		}
		for _, child := range p.Particles {
			if err := validateWildcardNamespaceRestriction(schema, baseAny, child, visitedMG, visitedGroups); err != nil {
				return err
			}
		}
	case *model.GroupRef:
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
	case *model.ElementDecl:
		if !model.AllowsNamespace(baseAny.Namespace, baseAny.NamespaceList, baseAny.TargetNamespace, p.Name.Namespace) {
			return fmt.Errorf("ComplexContent restriction: element namespace %q not allowed by base wildcard", p.Name.Namespace)
		}
	case *model.AnyElement:
		if err := validateWildcardToWildcardRestriction(baseAny, p); err != nil {
			return err
		}
	}
	return nil
}

// validateWildcardToWildcardRestriction validates Wildcard:Wildcard derivation.
func validateWildcardToWildcardRestriction(baseAny, restrictionAny *model.AnyElement) error {
	if !model.ProcessContentsStrongerOrEqual(restrictionAny.ProcessContents, baseAny.ProcessContents) {
		return fmt.Errorf(
			"ComplexContent restriction: wildcard restriction: processContents in restriction must be identical or stronger than base (base is %s, restriction is %s)",
			processContentsName(baseAny.ProcessContents),
			processContentsName(restrictionAny.ProcessContents),
		)
	}
	if !model.NamespaceConstraintSubset(
		restrictionAny.Namespace, restrictionAny.NamespaceList, restrictionAny.TargetNamespace,
		baseAny.Namespace, baseAny.NamespaceList, baseAny.TargetNamespace,
	) {
		return fmt.Errorf("ComplexContent restriction: wildcard restriction: wildcard is not a subset of base wildcard")
	}
	return nil
}

func validateWildcardBaseRestriction(schema *parser.Schema, baseParticle, restrictionParticle model.Particle) (bool, error) {
	baseAny, baseIsAny := baseParticle.(*model.AnyElement)
	if !baseIsAny {
		return false, nil
	}
	if restrictionElem, ok := restrictionParticle.(*model.ElementDecl); ok {
		return true, validateWildcardToElementRestriction(baseAny, restrictionElem)
	}
	if restrictionMG, ok := restrictionParticle.(*model.ModelGroup); ok {
		return true, validateWildcardToModelGroupRestriction(schema, baseAny, restrictionMG)
	}
	return false, nil
}

func validateWildcardPairRestriction(baseParticle, restrictionParticle model.Particle) (bool, error) {
	baseAny, baseIsAny := baseParticle.(*model.AnyElement)
	restrictionAny, restrictionIsAny := restrictionParticle.(*model.AnyElement)
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
