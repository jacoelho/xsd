package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// validateParticleStructure validates structural constraints of particles.
func validateParticleStructure(schema *parser.Schema, particle types.Particle) error {
	visited := newModelGroupVisit()
	return validateParticleStructureWithVisited(schema, particle, nil, visited)
}

// validateParticleStructureWithVisited validates structural constraints with cycle detection
func validateParticleStructureWithVisited(schema *parser.Schema, particle types.Particle, parentKind *types.GroupKind, visited modelGroupVisit) error {
	if err := validateParticleOccurs(particle); err != nil {
		return err
	}
	switch p := particle.(type) {
	case *types.ModelGroup:
		return validateModelGroupStructure(schema, p, parentKind, visited)
	case *types.GroupRef:
		// group references are particles that reference a named model group
		// the actual group validation happens when the referenced group is defined
		// no additional structural validation needed here beyond occurrence constraints (checked above)
	case *types.AnyElement:
		// wildcard elements don't have additional structural constraints beyond occurrence
	case *types.ElementDecl:
		return validateElementParticle(schema, p)
	}
	return nil
}

func validateParticleOccurs(particle types.Particle) error {
	// per XSD spec "Particle Correct":
	// 1. maxOccurs must be >= 1 (cannot be 0), EXCEPT when minOccurs=0 and maxOccurs=0
	//    (which effectively means the particle cannot appear - used in restrictions)
	// 2. when maxOccurs >= 1, it must be >= minOccurs
	maxOcc := particle.MaxOcc()
	minOcc := particle.MinOcc()
	if maxOcc.IsOverflow() || minOcc.IsOverflow() {
		return fmt.Errorf("%w: occurrence value exceeds uint32", types.ErrOccursOverflow)
	}

	// maxOccurs can be "unbounded" or a non-negative integer, with 0 only allowed when minOccurs=0
	// note: W3C test suite includes schemas with maxOccurs=0 and minOccurs=0, which
	// we accept for compatibility even though the spec says maxOccurs >= 1
	if maxOcc.IsZero() && !minOcc.IsZero() {
		return fmt.Errorf("maxOccurs cannot be 0 when minOccurs > 0")
	}
	if !maxOcc.IsUnbounded() && !maxOcc.IsZero() && maxOcc.Cmp(minOcc) < 0 {
		return fmt.Errorf("minOccurs (%s) cannot be greater than maxOccurs (%s)", minOcc, maxOcc)
	}
	return nil
}

func validateModelGroupStructure(schema *parser.Schema, group *types.ModelGroup, parentKind *types.GroupKind, visited modelGroupVisit) error {
	// cycle detection: skip if already visited
	if !visited.enter(group) {
		return nil
	}

	if err := validateLocalElementTypes(group.Particles); err != nil {
		return err
	}
	if group.Kind == types.AllGroup {
		if err := validateAllGroupConstraints(group, parentKind); err != nil {
			return err
		}
	}

	// recursively validate particles with this group as parent
	for _, childParticle := range group.Particles {
		if err := validateParticleStructureWithVisited(schema, childParticle, &group.Kind, visited); err != nil {
			return err
		}
	}
	return nil
}

func validateLocalElementTypes(particles []types.Particle) error {
	// check for duplicate local element declarations within the same model group.
	// per XSD 1.0 "Element Declarations Consistent", declarations with the same
	// QName must have consistent type definitions.
	localElementTypes := make(map[types.QName]types.Type)
	for _, childParticle := range particles {
		childElem, ok := childParticle.(*types.ElementDecl)
		if !ok || childElem.IsReference {
			continue
		}
		if existingType, exists := localElementTypes[childElem.Name]; exists {
			if !ElementTypesCompatible(existingType, childElem.Type) {
				return fmt.Errorf("duplicate local element declaration '%s' with different types", childElem.Name)
			}
			continue
		}
		localElementTypes[childElem.Name] = childElem.Type
	}
	return nil
}

func validateAllGroupConstraints(group *types.ModelGroup, parentKind *types.GroupKind) error {
	// check if xs:all is nested inside xs:sequence or xs:choice
	if parentKind != nil {
		if *parentKind == types.Sequence || *parentKind == types.Choice {
			return fmt.Errorf("xs:all cannot appear as a child of xs:sequence or xs:choice (XSD 1.0)")
		}
	}
	// xs:all requires unique element declarations (no duplicate element names)
	// per XSD 1.0 Structures spec section 3.8.6: all group particles must be distinct.
	if err := validateAllGroupUniqueElements(group.Particles); err != nil {
		return err
	}
	// xs:all must have minOccurs="0" or "1", and maxOccurs="1" (the defaults)
	// it cannot have any other occurrence values
	if err := validateAllGroupOccurrence(group); err != nil {
		return err
	}
	// all particles in xs:all must have maxOccurs <= 1
	if err := validateAllGroupParticleOccurs(group.Particles); err != nil {
		return err
	}
	// no nested xs:all with minOccurs > 0
	if err := validateAllGroupNested(group.Particles); err != nil {
		return err
	}
	return nil
}

func validateAllGroupUniqueElements(particles []types.Particle) error {
	seenElements := make(map[types.QName]bool)
	for _, childParticle := range particles {
		childElem, ok := childParticle.(*types.ElementDecl)
		if !ok {
			continue
		}
		if seenElements[childElem.Name] {
			return fmt.Errorf("xs:all: duplicate element declaration '%s'", childElem.Name)
		}
		seenElements[childElem.Name] = true
	}
	return nil
}

func validateAllGroupOccurrence(group *types.ModelGroup) error {
	if !group.MinOccurs.IsZero() && !group.MinOccurs.IsOne() {
		return fmt.Errorf("xs:all must have minOccurs='0' or '1' (got %s)", group.MinOccurs)
	}
	if !group.MaxOccurs.IsOne() {
		return fmt.Errorf("xs:all must have maxOccurs='1' (got %s)", group.MaxOccurs)
	}
	return nil
}

func validateAllGroupParticleOccurs(particles []types.Particle) error {
	for _, childParticle := range particles {
		if childParticle.MaxOcc().CmpInt(1) > 0 {
			return fmt.Errorf("xs:all: all particles must have maxOccurs <= 1 (got %s)", childParticle.MaxOcc())
		}
	}
	return nil
}

func validateAllGroupNested(particles []types.Particle) error {
	for _, childParticle := range particles {
		childMG, ok := childParticle.(*types.ModelGroup)
		if !ok {
			continue
		}
		if childMG.Kind == types.AllGroup && childMG.MinOccurs.CmpInt(0) > 0 {
			return fmt.Errorf("xs:all: nested xs:all cannot have minOccurs > 0 (got %s)", childMG.MinOccurs)
		}
	}
	return nil
}

func validateElementParticle(schema *parser.Schema, elem *types.ElementDecl) error {
	if err := validateElementConstraints(elem); err != nil {
		return err
	}
	if err := validateElementConstraintNames(elem); err != nil {
		return err
	}
	if elem.IsReference {
		return validateReferencedElementType(schema, elem)
	}
	if elem.Type == nil {
		return nil
	}
	return validateInlineElementType(schema, elem)
}

func validateElementConstraints(elem *types.ElementDecl) error {
	for _, constraint := range elem.Constraints {
		if err := validateIdentityConstraint(constraint); err != nil {
			return fmt.Errorf("element '%s' identity constraint '%s': %w", elem.Name, constraint.Name, err)
		}
	}
	return nil
}

func validateElementConstraintNames(elem *types.ElementDecl) error {
	constraintNames := make(map[string]bool)
	for _, constraint := range elem.Constraints {
		if constraintNames[constraint.Name] {
			return fmt.Errorf("element '%s': duplicate identity constraint name '%s'", elem.Name, constraint.Name)
		}
		constraintNames[constraint.Name] = true
	}
	return nil
}

func validateReferencedElementType(schema *parser.Schema, elem *types.ElementDecl) error {
	// element references don't need their own type - they inherit from the referenced element
	// this is an element reference - check that the referenced element exists and has a type
	if refDecl, exists := schema.ElementDecls[elem.Name]; exists {
		if refDecl.Type == nil {
			return fmt.Errorf("referenced element '%s' must have a type", elem.Name)
		}
	}
	// if referenced element doesn't exist, that's a different error (forward reference or missing import)
	// don't validate type here for references
	return nil
}

func validateInlineElementType(schema *parser.Schema, elem *types.ElementDecl) error {
	// validate inline types (simpleType or complexType defined inline in the element)
	if st, ok := elem.Type.(*types.SimpleType); ok && st.QName.IsZero() {
		if err := validateSimpleTypeStructure(schema, st); err != nil {
			return fmt.Errorf("inline simpleType in element '%s': %w", elem.Name, err)
		}
		return nil
	}
	if complexType, ok := elem.Type.(*types.ComplexType); ok && complexType.QName.IsZero() {
		if err := validateComplexTypeStructure(schema, complexType, typeDefinitionInline); err != nil {
			return fmt.Errorf("inline complexType in element '%s': %w", elem.Name, err)
		}
	}
	return nil
}

// validateElementDeclarationsConsistentInParticle validates "Element Declarations Consistent"
// across a particle tree, including nested model groups and group references.
func validateElementDeclarationsConsistentInParticle(schema *parser.Schema, particle types.Particle) error {
	seen := make(map[types.QName]types.Type)
	visited := newModelGroupVisit()
	return validateElementDeclarationsConsistentWithVisited(schema, particle, seen, visited)
}

func validateElementDeclarationsConsistentWithVisited(schema *parser.Schema, particle types.Particle, seen map[types.QName]types.Type, visited modelGroupVisit) error {
	switch p := particle.(type) {
	case *types.ModelGroup:
		if !visited.enter(p) {
			return nil
		}
		for _, child := range p.Particles {
			if err := validateElementDeclarationsConsistentWithVisited(schema, child, seen, visited); err != nil {
				return err
			}
		}
	case *types.GroupRef:
		if schema == nil {
			return nil
		}
		if group, ok := schema.Groups[p.RefQName]; ok {
			return validateElementDeclarationsConsistentWithVisited(schema, group, seen, visited)
		}
	case *types.ElementDecl:
		elemType := p.Type
		if p.IsReference && schema != nil {
			if refDecl, ok := schema.ElementDecls[p.Name]; ok {
				elemType = refDecl.Type
			}
		}
		if elemType == nil {
			return nil
		}
		if existing, ok := seen[p.Name]; ok {
			if !ElementTypesCompatible(existing, elemType) {
				return fmt.Errorf("element declarations consistent violation for element '%s'", p.Name)
			}
			return nil
		}
		seen[p.Name] = elemType
	}
	return nil
}

// validateGroupStructure validates structural constraints of a group definition
// Does not validate references (which might be forward references or imports)
func validateGroupStructure(qname types.QName, group *types.ModelGroup) error {
	// this is a structural constraint that is definitely invalid if violated
	if !isValidNCName(qname.Local) {
		return fmt.Errorf("invalid group name '%s': must be a valid NCName", qname.Local)
	}

	// according to XSD spec: groups cannot have minOccurs="0" or maxOccurs="unbounded" directly
	// groups must have minOccurs="1" and maxOccurs="1" (the defaults)
	if group.MinOccurs.IsZero() {
		return fmt.Errorf("group '%s' cannot have minOccurs='0'", qname.Local)
	}
	if group.MaxOccurs.IsUnbounded() {
		return fmt.Errorf("group '%s' cannot have maxOccurs='unbounded'", qname.Local)
	}
	if !group.MinOccurs.IsOne() || !group.MaxOccurs.IsOne() {
		return fmt.Errorf("group '%s' must have minOccurs='1' and maxOccurs='1' (got minOccurs=%s, maxOccurs=%s)", qname.Local, group.MinOccurs, group.MaxOccurs)
	}

	// don't validate content model references - they might be forward references
	return nil
}

// validateParticleRestriction validates that particles in a restriction are valid restrictions of base particles
func validateParticleRestriction(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	// if both model groups have maxOccurs=0, the content never occurs, so children
	// constraints are irrelevant. Skip child validation in this case.
	if baseMG.MaxOcc().IsZero() && restrictionMG.MaxOcc().IsZero() {
		return nil
	}
	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), restrictionMG.MinOcc(), restrictionMG.MaxOcc()); err != nil {
		return err
	}
	if err := validateSingleWildcardGroupRestriction(schema, baseMG, restrictionMG); err != nil {
		return err
	}
	// handle model group kind changes: if kinds differ, validate that restriction is valid
	if baseMG.Kind != restrictionMG.Kind {
		// allow kind changes if restriction particles are valid restrictions of base particles
		// for example: choice -> sequence is valid if sequence elements match choice elements
		// for example: sequence with wildcard -> all with elements is valid
		return validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
	}
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
	// same kind - validate normally
	switch baseMG.Kind {
	case types.Sequence:
		return validateSequenceRestriction(schema, baseChildren, restrictionChildren)
	case types.Choice:
		return validateChoiceRestriction(schema, baseChildren, restrictionChildren)
	case types.AllGroup:
		return validateAllGroupRestriction(schema, baseMG, restrictionMG)
	}
	return nil
}

func validateSingleWildcardGroupRestriction(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	// if the base group is a single wildcard particle, validate the restriction group
	// directly against the wildcard using NSRecurseCheckCardinality semantics.
	if len(baseMG.Particles) != 1 {
		return nil
	}
	baseAny, ok := baseMG.Particles[0].(*types.AnyElement)
	if !ok {
		return nil
	}
	return validateParticlePairRestriction(schema, baseAny, restrictionMG)
}

func validateSequenceRestriction(schema *parser.Schema, baseChildren, restrictionChildren []types.Particle) error {
	// for sequence, particles must match in order
	// optional particles (minOccurs=0) can be removed, but required particles must be present
	baseIdx := 0
	matchedBaseParticles := make(map[int]bool) // track which base particles have been matched
	for _, restrictionParticle := range restrictionChildren {
		if restrictionParticle.MaxOcc().IsZero() && restrictionParticle.MinOcc().IsZero() {
			continue
		}
		found := false
		for baseIdx < len(baseChildren) {
			baseParticle := baseChildren[baseIdx]
			// try to match this restriction particle with the current base particle
			err := validateParticlePairRestriction(schema, baseParticle, restrictionParticle)
			if err == nil {
				// match found - mark this base particle as matched
				matchedBaseParticles[baseIdx] = true
				// if base particle is a wildcard with maxOccurs > 1 or unbounded, we can match multiple restriction particles to it
				// otherwise, both particles advance
				if baseAny, isWildcard := baseParticle.(*types.AnyElement); isWildcard {
					// wildcard can match multiple restriction particles
					// only advance past wildcard if maxOccurs=1
					if baseAny.MaxOccurs.IsOne() {
						baseIdx++ // advance past wildcard with maxOccurs=1
					}
					// if maxOccurs > 1 or unbounded, stay on wildcard (don't increment baseIdx)
				} else {
					baseIdx++ // advance past non-wildcard
				}
				found = true
				break
			}
			skippable := baseParticle.MinOcc().IsZero()
			if !skippable {
				if baseGroup, ok := baseParticle.(*types.ModelGroup); ok {
					skippable = isEffectivelyOptional(baseGroup)
				}
			}
			if skippable {
				baseIdx++
				continue
			}
			// required particle cannot be skipped - return the original error
			return err
		}
		if !found {
			return fmt.Errorf("ComplexContent restriction: restriction particle does not match any base particle")
		}
	}
	// check if any remaining required base particles were skipped
	for i := baseIdx; i < len(baseChildren); i++ {
		baseParticle := baseChildren[i]
		// skip if this particle was already matched
		if matchedBaseParticles[i] {
			continue
		}
		if baseParticle.MinOcc().CmpInt(0) > 0 {
			// check if it's effectively optional (contains only optional content)
			if baseMG2, ok := baseParticle.(*types.ModelGroup); ok {
				if isEffectivelyOptional(baseMG2) {
					// effectively optional - can be skipped
					continue
				}
			}
			return fmt.Errorf("ComplexContent restriction: required particle at position %d is missing", i)
		}
	}
	return nil
}

func validateChoiceRestriction(schema *parser.Schema, baseChildren, restrictionChildren []types.Particle) error {
	// choice uses RecurseLax: match restriction particles to base particles in order.
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

func validateAllGroupRestriction(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup) error {
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
		if baseParticle.MinOcc().CmpInt(0) > 0 {
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
func validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc types.Occurs) error {
	if baseMinOcc.IsOverflow() || baseMaxOcc.IsOverflow() || restrictionMinOcc.IsOverflow() || restrictionMaxOcc.IsOverflow() {
		return fmt.Errorf("%w: occurrence value exceeds uint32", types.ErrOccursOverflow)
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
	// base has unbounded maxOccurs, restriction can have any minOccurs >= base minOccurs
	// both unbounded or restriction bounded with base unbounded are valid
	return nil
}

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
		if !visitedMG.enter(p) {
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
func validateParticlePairRestriction(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) error {
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

func validateModelGroupElementRestriction(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) (bool, error) {
	baseMG, baseIsMG := baseParticle.(*types.ModelGroup)
	restrictionElem, restrictionIsElem := restrictionParticle.(*types.ElementDecl)
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

func effectiveParticleOccurrence(baseParticle, restrictionParticle types.Particle) (types.Occurs, types.Occurs, types.Occurs, types.Occurs) {
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
	return baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc
}

func validateParticlePairOccurrence(baseParticle, restrictionParticle types.Particle) error {
	baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc := effectiveParticleOccurrence(baseParticle, restrictionParticle)
	return validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc)
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

func validateModelGroupPairRestriction(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) (bool, error) {
	baseMG, baseIsMG := baseParticle.(*types.ModelGroup)
	restrictionMG, restrictionIsMG := restrictionParticle.(*types.ModelGroup)
	if !baseIsMG || !restrictionIsMG {
		return false, nil
	}
	if baseMG.Kind != restrictionMG.Kind {
		return true, validateParticleRestrictionWithKindChange(schema, baseMG, restrictionMG)
	}
	return true, validateParticleRestriction(schema, baseMG, restrictionMG)
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

// validateParticleRestrictionWithKindChange validates restrictions when model group kinds differ.
// Per XSD 1.0 spec section 3.9.6, compositor changes are valid in these cases:
// - When base contains wildcards: any compositor can restrict wildcards (spec section 7.2.5 "Group restricts wildcard")
// - choice -> sequence: Valid if all sequence particles match some choice particle
// - choice -> all: Generally invalid, but valid if base contains wildcards
// - sequence -> choice: INVALID (cannot loosen order constraint), unless base has wildcards
// - sequence -> all: Generally invalid, but valid if base contains wildcards
// - all -> sequence/choice: INVALID (cannot change all's unique semantics), unless base has wildcards
