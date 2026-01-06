package loader

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateParticleRestriction validates that particles in a restriction are valid restrictions of base particles
func validateParticleRestriction(schema *schema.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	// If both model groups have maxOccurs=0, the content never occurs, so children
	// constraints are irrelevant. Skip child validation in this case.
	if baseMG.MaxOcc() == 0 && restrictionMG.MaxOcc() == 0 {
		return nil
	}
	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), restrictionMG.MinOcc(), restrictionMG.MaxOcc()); err != nil {
		return err
	}
	// If the base group is a single wildcard particle, validate the restriction group
	// directly against the wildcard using NSRecurseCheckCardinality semantics.
	if len(baseMG.Particles) == 1 {
		if baseAny, ok := baseMG.Particles[0].(*types.AnyElement); ok {
			return validateParticlePairRestriction(schema, baseAny, restrictionMG)
		}
	}
	// Handle model group kind changes: if kinds differ, validate that restriction is valid
	if baseMG.Kind != restrictionMG.Kind {
		// Allow kind changes if restriction particles are valid restrictions of base particles
		// For example: choice -> sequence is valid if sequence elements match choice elements
		// For example: sequence with wildcard -> all with elements is valid
		return validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
	}
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
	// Same kind - validate normally
	if baseMG.Kind == types.Sequence {
		// For sequence, particles must match in order
		// Optional particles (minOccurs=0) can be removed, but required particles must be present
		baseIdx := 0
		matchedBaseParticles := make(map[int]bool) // Track which base particles have been matched
		for _, restrictionParticle := range restrictionChildren {
			if restrictionParticle.MaxOcc() == 0 && restrictionParticle.MinOcc() == 0 {
				continue
			}
			found := false
			for baseIdx < len(baseChildren) {
				baseParticle := baseChildren[baseIdx]
				// Try to match this restriction particle with the current base particle
				err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle)
				if err == nil {
					// Match found - mark this base particle as matched
					matchedBaseParticles[baseIdx] = true
					// If base particle is a wildcard with maxOccurs > 1 or unbounded, we can match multiple restriction particles to it
					// Otherwise, both particles advance
					if baseAny, isWildcard := baseParticle.(*types.AnyElement); isWildcard {
						// Wildcard can match multiple restriction particles
						// Only advance past wildcard if maxOccurs=1
						if baseAny.MaxOccurs == 1 {
							baseIdx++ // Advance past wildcard with maxOccurs=1
						}
						// If maxOccurs > 1 or unbounded, stay on wildcard (don't increment baseIdx)
					} else {
						baseIdx++ // Advance past non-wildcard
					}
					found = true
					break
				}
				// Check if this is a validation error (like maxOccurs, minOccurs) that should be returned immediately
				// These errors mean the particles match but the restriction is invalid
				errMsg := err.Error()
				skippable := baseParticle.MinOcc() == 0
				if !skippable {
					if baseMG, ok := baseParticle.(*types.ModelGroup); ok {
						skippable = isEffectivelyOptional(baseMG)
					}
				}
				if strings.Contains(errMsg, "maxOccurs") || strings.Contains(errMsg, "minOccurs") ||
					strings.Contains(errMsg, "model group kind") || strings.Contains(errMsg, "cannot restrict wildcard") {
					if !skippable {
						// Validation errors should be returned immediately when base particle is required
						return err
					}
				}
				// "Element name mismatch" means these particles don't match - try next base particle
				// No match - check if we can skip this base particle (if it's optional or effectively optional)
				if skippable {
					baseIdx++
					continue
				}
				// Required particle cannot be skipped - return the original error
				return err
			}
			if !found {
				return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle")
			}
		}
		// Check if any remaining required base particles were skipped
		for i := baseIdx; i < len(baseChildren); i++ {
			baseParticle := baseChildren[i]
			// Skip if this particle was already matched
			if matchedBaseParticles[i] {
				continue
			}
			if baseParticle.MinOcc() > 0 {
				// Check if it's effectively optional (contains only optional content)
				if baseMG2, ok := baseParticle.(*types.ModelGroup); ok {
					if isEffectivelyOptional(baseMG2) {
						// Effectively optional - can be skipped
						continue
					}
				}
				return fmt.Errorf("ComplexContent restriction: required particle at position %d is missing", i)
			}
		}
	} else if baseMG.Kind == types.Choice {
		// Choice uses RecurseLax: match restriction particles to base particles in order.
		baseIdx := 0
		for _, restrictionParticle := range restrictionChildren {
			if restrictionParticle.MaxOcc() == 0 && restrictionParticle.MinOcc() == 0 {
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
	} else if baseMG.Kind == types.AllGroup {
		if err := validateAllGroupRestriction(schema, baseMG, restrictionMG); err != nil {
			return err
		}
	}
	return nil
}

func validateAllGroupRestriction(schema *schema.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)

	baseIdx := 0
	for _, restrictionParticle := range restrictionChildren {
		if restrictionParticle.MaxOcc() == 0 && restrictionParticle.MinOcc() == 0 {
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
			skippable := baseParticle.MinOcc() == 0
			if !skippable {
				if baseGroup, ok := baseParticle.(*types.ModelGroup); ok {
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
		if baseParticle.MinOcc() > 0 {
			if baseGroup, ok := baseParticle.(*types.ModelGroup); ok {
				if isEffectivelyOptional(baseGroup) {
					continue
				}
			}
			return fmt.Errorf("ComplexContent restriction: required particle at position %d is missing", i)
		}
	}
	return nil
}

// validateOccurrenceConstraints validates occurrence constraints for particle restrictions
// In a restriction:
// - minOccurs must be >= base minOccurs (can require more)
// - maxOccurs must be <= base maxOccurs (can allow fewer)
// - minOccurs must be <= base maxOccurs (can't require more than base allows)
func validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc int) error {
	if restrictionMinOcc < baseMinOcc {
		return fmt.Errorf("ComplexContent restriction: minOccurs (%d) must be >= base minOccurs (%d)", restrictionMinOcc, baseMinOcc)
	}
	if baseMaxOcc != types.UnboundedOccurs {
		if restrictionMaxOcc == types.UnboundedOccurs {
			return fmt.Errorf("ComplexContent restriction: maxOccurs cannot be unbounded when base maxOccurs is bounded (%d)", baseMaxOcc)
		}
		if restrictionMaxOcc > baseMaxOcc {
			return fmt.Errorf("ComplexContent restriction: maxOccurs (%d) must be <= base maxOccurs (%d)", restrictionMaxOcc, baseMaxOcc)
		}
		if restrictionMinOcc > baseMaxOcc {
			return fmt.Errorf("ComplexContent restriction: minOccurs (%d) must be <= base maxOccurs (%d)", restrictionMinOcc, baseMaxOcc)
		}
	}
	// Base has unbounded maxOccurs, restriction can have any minOccurs >= base minOccurs
	// Both unbounded or restriction bounded with base unbounded are valid
	return nil
}

// validateWildcardToElementRestriction validates Element:Wildcard derivation
// When base is a wildcard and restriction is an element, this is valid if:
// - Element's namespace is allowed by wildcard's namespace constraint
// - Element's occurrence constraints are within wildcard's constraints
func validateWildcardToElementRestriction(schema *schema.Schema, baseAny *types.AnyElement, restrictionElem *types.ElementDecl) error {
	if restrictionElem.MinOcc() == 0 && restrictionElem.MaxOcc() == 0 {
		return validateOccurrenceConstraints(baseAny.MinOccurs, baseAny.MaxOccurs, restrictionElem.MinOcc(), restrictionElem.MaxOcc())
	}
	// Check namespace constraint: element namespace must be allowed by wildcard
	elemNS := restrictionElem.Name.Namespace
	wildcardAllows := false
	switch baseAny.Namespace {
	case types.NSCAny:
		// ##any allows all namespaces
		wildcardAllows = true
	case types.NSCTargetNamespace:
		// ##targetNamespace - check if element is in target namespace
		wildcardAllows = (string(elemNS) == string(schema.TargetNamespace))
	case types.NSCLocal:
		// ##local - element must have no namespace
		wildcardAllows = (elemNS == "")
	case types.NSCOther:
		// ##other - element must NOT be in target namespace
		wildcardAllows = (string(elemNS) != string(schema.TargetNamespace) && elemNS != "")
	case types.NSCList:
		// Element namespace must be in the allowed list
		if slices.Contains(baseAny.NamespaceList, elemNS) {
			wildcardAllows = true
		}
	}

	if !wildcardAllows {
		return fmt.Errorf("ComplexContent restriction: element namespace %q not allowed by base wildcard", elemNS)
	}

	return validateOccurrenceConstraints(baseAny.MinOccurs, baseAny.MaxOccurs, restrictionElem.MinOcc(), restrictionElem.MaxOcc())
}

// validateWildcardToModelGroupRestriction validates ModelGroup:Wildcard derivation
// When base is a wildcard and restriction is a model group, we calculate the effective
// occurrence of the model group's content and validate against the wildcard constraints.
func validateWildcardToModelGroupRestriction(schema *schema.Schema, baseAny *types.AnyElement, restrictionMG *types.ModelGroup) error {
	if err := validateWildcardNamespaceRestriction(schema, baseAny, restrictionMG, make(map[*types.ModelGroup]bool), make(map[types.QName]bool)); err != nil {
		return err
	}
	// Calculate effective occurrence by recursively finding the total minOccurs/maxOccurs
	// of elements within the model group
	effectiveMinOcc, effectiveMaxOcc := calculateEffectiveOccurrence(restrictionMG)
	return validateOccurrenceConstraints(baseAny.MinOccurs, baseAny.MaxOccurs, effectiveMinOcc, effectiveMaxOcc)
}

func validateWildcardNamespaceRestriction(schema *schema.Schema, baseAny *types.AnyElement, particle types.Particle, visitedMG map[*types.ModelGroup]bool, visitedGroups map[types.QName]bool) error {
	if particle != nil && particle.MinOcc() == 0 && particle.MaxOcc() == 0 {
		return nil
	}
	switch p := particle.(type) {
	case *types.ModelGroup:
		if visitedMG[p] {
			return nil
		}
		visitedMG[p] = true
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

// validateModelGroupToElementRestriction validates ModelGroup:Element derivation.
// Per XSD 1.0 spec section 3.9.6 (Particle Derivation OK - RecurseAsIfGroup),
// treat the derived element as if it were wrapped in a group of the same kind
// as the base with minOccurs=maxOccurs=1, then apply recurse mapping.
// Returns false if no matching element is found (to allow fall-through).
func validateModelGroupToElementRestriction(schema *schema.Schema, baseMG *types.ModelGroup, restrictionElem *types.ElementDecl) (bool, error) {
	baseChildren := derivationChildren(baseMG)
	if len(baseChildren) == 0 {
		return false, nil
	}

	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), 1, 1); err != nil {
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
		if baseMG.MinOccurs > 0 && !isEmptiableParticle(baseParticle) {
			break
		}
	}
	if !matched {
		if constraintErr != nil {
			return false, constraintErr
		}
		return false, nil
	}
	if baseMG.MinOccurs > 0 {
		for ; current < len(baseChildren); current++ {
			if !isEmptiableParticle(baseChildren[current]) {
				return false, fmt.Errorf("ComplexContent restriction: required base particle not present in element restriction")
			}
		}
	}
	return true, nil
}

func validateGroupChildElementRestriction(schema *schema.Schema, baseMG *types.ModelGroup, baseChildren []types.Particle, baseParticle types.Particle, restrictionElem *types.ElementDecl) (bool, error) {
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

func validateElementRestrictionWithGroupOccurrence(schema *schema.Schema, baseMG *types.ModelGroup, baseChildren []types.Particle, baseElem, restrictionElem *types.ElementDecl) (bool, error) {
	if baseElem.Name != restrictionElem.Name {
		if !isSubstitutableElement(schema, baseElem.Name, restrictionElem.Name) {
			return false, nil
		}
	}
	if schema != nil && baseMG != nil && baseMG.Kind == types.Choice && baseMG.MaxOcc() != 1 && len(baseChildren) > 1 {
		baseElemMax := baseElem.MaxOcc()
		if baseElemMax >= 0 {
			restrictionMax := restrictionElem.MaxOcc()
			if restrictionMax == types.UnboundedOccurs || restrictionMax > baseElemMax {
				if existing, ok := schema.ParticleRestrictionCaps[restrictionElem]; !ok || baseElemMax < existing {
					schema.ParticleRestrictionCaps[restrictionElem] = baseElemMax
				}
			}
		}
	}
	baseMinOcc := baseElem.MinOcc()
	baseMaxOcc := baseElem.MaxOcc()
	if baseMG != nil && baseMG.Kind == types.Choice && baseMG.MaxOcc() != 1 {
		baseMinOcc, baseMaxOcc = choiceChildOccurrenceRange(baseMG, baseChildren, baseElem)
	}
	if err := validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionElem.MinOcc(), restrictionElem.MaxOcc()); err != nil {
		return true, err
	}
	if restrictionElem.MinOcc() == 0 && restrictionElem.MaxOcc() == 0 {
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
	if baseMG != nil && baseMG.Kind == types.Choice && baseMG.MaxOcc() != 1 {
		baseMinOcc, baseMaxOcc = choiceChildOccurrenceRange(baseMG, baseChildren, baseAny)
	}
	if restrictionElem.MinOcc() == 0 && restrictionElem.MaxOcc() == 0 {
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

func choiceChildOccurrenceRange(baseMG *types.ModelGroup, baseChildren []types.Particle, child types.Particle) (int, int) {
	childMin := child.MinOcc()
	childMax := child.MaxOcc()
	groupMin := baseMG.MinOcc()
	groupMax := baseMG.MaxOcc()

	minOcc := 0
	if len(baseChildren) == 1 {
		minOcc = groupMin * childMin
	}

	if groupMax == types.UnboundedOccurs || childMax == types.UnboundedOccurs {
		return minOcc, types.UnboundedOccurs
	}
	return minOcc, groupMax * childMax
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

// validateParticlePairRestriction validates that a restriction particle is a valid restriction of a base particle
func validateParticlePairRestriction(schema *schema.Schema, baseParticle, restrictionParticle types.Particle) error {
	baseParticle = normalizePointlessParticle(baseParticle)
	restrictionParticle = normalizePointlessParticle(restrictionParticle)

	// Special case: Element:Wildcard derivation (Particle Derivation OK - Element:Any)
	baseAny, baseIsAny := baseParticle.(*types.AnyElement)
	restrictionElem, restrictionIsElem := restrictionParticle.(*types.ElementDecl)
	if baseIsAny && restrictionIsElem {
		return validateWildcardToElementRestriction(schema, baseAny, restrictionElem)
	}

	// Special case: ModelGroup:Wildcard derivation (Particle Derivation OK - NS:RecurseAsIfGroup)
	restrictionMG, restrictionIsMG := restrictionParticle.(*types.ModelGroup)
	if baseIsAny && restrictionIsMG {
		return validateWildcardToModelGroupRestriction(schema, baseAny, restrictionMG)
	}

	// Special case: if base is a ModelGroup and restriction is an ElementDecl,
	// we need to find the matching element inside the group and compare against that
	baseMG, baseIsMG := baseParticle.(*types.ModelGroup)
	if baseIsMG && restrictionIsElem {
		matched, err := validateModelGroupToElementRestriction(schema, baseMG, restrictionElem)
		if err != nil {
			return err
		}
		if matched {
			// Matching element found and constraints are valid
			return nil
		}
		return fmt.Errorf("ComplexContent restriction: element %s does not match any element in base model group", restrictionElem.Name)
	}

	baseMinOcc := baseParticle.MinOcc()
	baseMaxOcc := baseParticle.MaxOcc()
	restrictionMinOcc := restrictionParticle.MinOcc()
	restrictionMaxOcc := restrictionParticle.MaxOcc()
	if baseMG, ok := baseParticle.(*types.ModelGroup); ok {
		if restrictionMG, ok := restrictionParticle.(*types.ModelGroup); ok {
			baseMinOcc, baseMaxOcc = calculateEffectiveOccurrence(baseMG)
			restrictionMinOcc, restrictionMaxOcc = calculateEffectiveOccurrence(restrictionMG)
		}
	}
	if err := validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc); err != nil {
		return err
	}

	// For wildcard (any) elements, validate namespace constraint and processContents
	// Note: baseAny and baseIsAny already declared above for Element:Wildcard check
	baseAny, baseIsAny = baseParticle.(*types.AnyElement)
	restrictionAny, restrictionIsAny := restrictionParticle.(*types.AnyElement)
	if baseIsAny && restrictionIsAny {
		return validateWildcardToWildcardRestriction(baseAny, restrictionAny)
	} else if baseIsAny && !restrictionIsAny {
		// Base is a wildcard, restriction is not - this is valid (restricting wildcard to specific elements)
		// The restriction element must have valid occurrence constraints relative to the wildcard
		return nil
	} else if !baseIsAny && restrictionIsAny {
		// Base is not a wildcard, restriction is - this is invalid (can't restrict element to wildcard)
		return fmt.Errorf("ComplexContent restriction: cannot restrict non-wildcard to wildcard")
	} else {
		// Neither is a wildcard - check if they're the same type
		// For element declarations, they must match
		baseElem, baseIsElem := baseParticle.(*types.ElementDecl)
		restrictionElem, restrictionIsElem := restrictionParticle.(*types.ElementDecl)
		if baseIsElem && restrictionIsElem {
			if err := validateOccurrenceConstraints(baseElem.MinOcc(), baseElem.MaxOcc(), restrictionElem.MinOcc(), restrictionElem.MaxOcc()); err != nil {
				return err
			}
			if restrictionElem.MinOcc() == 0 && restrictionElem.MaxOcc() == 0 {
				return nil
			}
			// Element declarations must match (same name)
			if baseElem.Name != restrictionElem.Name {
				if !isSubstitutableElement(schema, baseElem.Name, restrictionElem.Name) {
					return fmt.Errorf("ComplexContent restriction: element name mismatch (%s vs %s)", baseElem.Name, restrictionElem.Name)
				}
			}
			if err := validateElementRestriction(schema, baseElem, restrictionElem); err != nil {
				return err
			}
		}
		// For model groups, recursively validate
		baseMG, baseIsMG := baseParticle.(*types.ModelGroup)
		restrictionMG, restrictionIsMG := restrictionParticle.(*types.ModelGroup)
		if baseIsMG && restrictionIsMG {
			// Model groups can have different kinds if restriction particles are valid
			// restrictions of base particles. E.g., choice -> sequence is valid if
			// all sequence particles are valid restrictions of some choice particle.
			if baseMG.Kind != restrictionMG.Kind {
				// Delegate to the kind-change validation
				return validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
			}
			// Same kind - recursively validate the model groups
			return validateParticleRestriction(schema, baseMG, restrictionMG)
		}
		if restrictionIsMG && baseIsElem {
			if restrictionMG.Kind != types.Choice {
				return fmt.Errorf("ComplexContent restriction: cannot restrict element %s to model group", baseElem.Name)
			}
			if err := validateOccurrenceConstraints(baseElem.MinOcc(), baseElem.MaxOcc(), restrictionMG.MinOcc(), restrictionMG.MaxOcc()); err != nil {
				return err
			}
			for _, p := range restrictionMG.Particles {
				if p.MinOcc() == 0 && p.MaxOcc() == 0 {
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
		// For other particle types, more validation needed
	}

	return nil
}

// validateElementRestriction validates that a restriction element properly restricts a base element.
// Per XSD 1.0 spec section 3.4.6 Constraints on Particle Schema Components:
// - nillable: If base is not nillable, restriction cannot be nillable
// - fixed: If base has fixed value, restriction must have same fixed value
// - block: Restriction block must be superset of base block (cannot allow more derivations)
// - type: Restriction type must be same as or derived from base type
func validateElementRestriction(schema *schema.Schema, baseElem, restrictionElem *types.ElementDecl) error {
	// Validate nillable: cannot change from false to true
	if !baseElem.Nillable && restrictionElem.Nillable {
		return fmt.Errorf("ComplexContent restriction: element '%s' nillable cannot be true when base element nillable is false", restrictionElem.Name)
	}

	// Validate fixed: if base has fixed value, restriction must have same fixed value
	// Values are compared after whitespace normalization based on the element's type
	if baseElem.HasFixed {
		if !restrictionElem.HasFixed {
			return fmt.Errorf("ComplexContent restriction: element '%s' must have fixed value matching base fixed value '%s'", restrictionElem.Name, baseElem.Fixed)
		}
		// Normalize both fixed values for comparison based on the element's type
		baseFixed := types.NormalizeWhiteSpace(baseElem.Fixed, baseElem.Type)
		restrictionFixed := types.NormalizeWhiteSpace(restrictionElem.Fixed, restrictionElem.Type)
		if baseFixed != restrictionFixed {
			return fmt.Errorf("ComplexContent restriction: element '%s' fixed value '%s' must match base fixed value '%s'", restrictionElem.Name, restrictionElem.Fixed, baseElem.Fixed)
		}
	}

	// Validate block: restriction block must be superset of base block
	// (restriction cannot allow more derivation methods than base)
	if !isBlockSuperset(restrictionElem.Block, baseElem.Block) {
		return fmt.Errorf("ComplexContent restriction: element '%s' block constraint must be superset of base block constraint", restrictionElem.Name)
	}

	// Validate type: restriction type must be same as or derived from base type
	if baseElem.Type != nil && restrictionElem.Type != nil {
		// Types are same if they have the same QName
		baseTypeName := baseElem.Type.Name()
		restrictionTypeName := restrictionElem.Type.Name()

		if baseTypeName.Namespace == types.XSDNamespace && baseTypeName.Local == "anyType" {
			return nil
		}
		if baseTypeName.Namespace == types.XSDNamespace && baseTypeName.Local == "anySimpleType" {
			if _, ok := restrictionElem.Type.(types.SimpleTypeDefinition); ok {
				return nil
			}
		}

		// If types are the same (by name), that's always valid
		if baseTypeName == restrictionTypeName {
			return nil
		}

		// Handle anonymous types (inline simpleType/complexType in restriction)
		// Anonymous types may have empty names but should be derived from base
		if restrictionTypeName.Local == "" {
			// Anonymous type - check if it's derived from base type
			if !isRestrictionDerivedFrom(restrictionElem.Type, baseElem.Type) {
				// For anonymous types, also check if they declare base type explicitly
				// Anonymous simpleTypes with restrictions are valid if their base matches
				if st, ok := restrictionElem.Type.(*types.SimpleType); ok {
					if st.Restriction != nil && st.Restriction.Base == baseTypeName {
						return nil
					}
					// Check if the anonymous type derives from the base through its ResolvedBase
					if st.ResolvedBase != nil && isRestrictionDerivedFrom(st.ResolvedBase, baseElem.Type) {
						return nil
					}
				}
				return fmt.Errorf("ComplexContent restriction: element '%s' anonymous type must be derived from base type '%s'", restrictionElem.Name, baseTypeName)
			}
			return nil
		}

		// If type names are different, restriction type must be derived from base type
		if !isRestrictionDerivedFrom(restrictionElem.Type, baseElem.Type) {
			return fmt.Errorf("ComplexContent restriction: element '%s' type '%s' must be same as or derived from base type '%s'", restrictionElem.Name, restrictionTypeName, baseTypeName)
		}
	}

	return nil
}

// validateParticleRestrictionWithKindChange validates restrictions when model group kinds differ.
// Per XSD 1.0 spec section 3.9.6, compositor changes are valid in these cases:
// - When base contains wildcards: any compositor can restrict wildcards (spec section 7.2.5 "Group restricts wildcard")
// - choice -> sequence: Valid if all sequence particles match some choice particle
// - choice -> all: Generally invalid, but valid if base contains wildcards
// - sequence -> choice: INVALID (cannot loosen order constraint), unless base has wildcards
// - sequence -> all: Generally invalid, but valid if base contains wildcards
// - all -> sequence/choice: INVALID (cannot change all's unique semantics), unless base has wildcards
func validateParticleRestrictionWithKindChange(schema *schema.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
	// Special case: if base contains a wildcard, the restriction can use any compositor
	// Per XSD spec section 7.2.5 (NSRecurseCheckCardinality), a model group can restrict a wildcard
	// with any compositor as long as the model group's particles are valid restrictions of the wildcard
	baseHasWildcard := modelGroupContainsWildcard(baseMG)

	if baseHasWildcard {
		// Find the wildcard in the base and validate the entire restriction group against it
		for _, baseParticle := range baseChildren {
			if baseWildcard, isWildcard := baseParticle.(*types.AnyElement); isWildcard {
				// This uses the ModelGroup:Wildcard derivation rule (NSRecurseCheckCardinality)
				if err := validateParticlePairRestriction(schema, baseWildcard, restrictionMG); err == nil {
					return nil
				}
			}
		}
		// If we found wildcards but couldn't validate against them, try particle-by-particle validation
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

	// No wildcards - apply strict compositor change rules
	// xs:all has unique semantics - non-all cannot restrict to xs:all
	// Exception: xs:all with a single element can restrict sequence/choice
	// (no ordering ambiguity with one element)
	if restrictionMG.Kind == types.AllGroup && baseMG.Kind != types.AllGroup {
		// Allow xs:all with single element to restrict sequence/choice
		if len(restrictionChildren) == 1 {
			restrictionParticle := restrictionChildren[0]
			for _, baseParticle := range baseChildren {
				if err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle); err == nil {
					return nil
				}
			}
			return fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
		}
		return fmt.Errorf("ComplexContent restriction: cannot restrict %s to xs:all", groupKindName(baseMG.Kind))
	}

	// xs:all -> sequence/choice: Valid if restriction particles match base particles
	// Per XSD spec, restricting xs:all to sequence/choice adds ordering constraints
	if baseMG.Kind == types.AllGroup && restrictionMG.Kind != types.AllGroup {
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

	if baseMG.Kind == types.Sequence && restrictionMG.Kind == types.Choice {
		return fmt.Errorf("ComplexContent restriction: cannot restrict sequence to choice")
	}

	// choice -> sequence: Valid if all sequence particles match some choice particle
	if baseMG.Kind == types.Choice && restrictionMG.Kind == types.Sequence {
		derivedCount := len(restrictionChildren)
		derivedMin := restrictionMG.MinOccurs * derivedCount
		derivedMax := restrictionMG.MaxOccurs
		if derivedMax != types.UnboundedOccurs {
			derivedMax *= derivedCount
		}
		if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), derivedMin, derivedMax); err != nil {
			return err
		}
		// Each restriction particle must be a valid restriction of at least one base particle
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

	// Other kind changes should not reach here due to early returns above
	return fmt.Errorf("ComplexContent restriction: invalid model group kind change from %s to %s", groupKindName(baseMG.Kind), groupKindName(restrictionMG.Kind))
}
