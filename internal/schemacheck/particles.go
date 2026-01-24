package schemacheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
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
			if !elementTypesCompatible(existingType, childElem.Type) {
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
			if !elementTypesCompatible(existing, elemType) {
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

func isSubstitutableElement(schema *parser.Schema, head, member types.QName) bool {
	if schema == nil || head == member {
		return true
	}
	headDecl := schema.ElementDecls[head]
	if headDecl == nil {
		return false
	}
	if headDecl.Block.Has(types.DerivationSubstitution) {
		return false
	}
	if !isSubstitutionGroupMember(schema, head, member) {
		return false
	}
	memberDecl := schema.ElementDecls[member]
	if memberDecl == nil {
		return false
	}
	headType := resolveTypeForFinalValidation(schema, headDecl.Type)
	memberType := resolveTypeForFinalValidation(schema, memberDecl.Type)
	if headType == nil || memberType == nil {
		return true
	}
	combinedBlock := headDecl.Block
	if headCT, ok := headType.(*types.ComplexType); ok {
		combinedBlock = combinedBlock.Add(types.DerivationMethod(headCT.Block))
	}
	if isDerivationBlocked(memberType, headType, combinedBlock) {
		return false
	}
	return true
}

func isSubstitutionGroupMember(schema *parser.Schema, head, member types.QName) bool {
	if schema == nil {
		return false
	}
	visited := make(map[types.QName]bool)
	var walk func(types.QName) bool
	walk = func(current types.QName) bool {
		if visited[current] {
			return false
		}
		visited[current] = true
		for _, sub := range schema.SubstitutionGroups[current] {
			if sub == member {
				return true
			}
			if walk(sub) {
				return true
			}
		}
		return false
	}
	return walk(head)
}

func isDerivationBlocked(memberType, headType types.Type, block types.DerivationSet) bool {
	if memberType == nil || headType == nil || block == 0 {
		return false
	}
	current := memberType
	for current != nil && current != headType {
		method := derivationMethodForType(current)
		if method != 0 && block.Has(method) {
			return true
		}
		derived, ok := types.AsDerivedType(current)
		if !ok {
			return false
		}
		current = derived.ResolvedBaseType()
	}
	return false
}

func derivationMethodForType(typ types.Type) types.DerivationMethod {
	switch typed := typ.(type) {
	case *types.ComplexType:
		return typed.DerivationMethod
	case *types.SimpleType:
		if typed.List != nil || typed.Variety() == types.ListVariety {
			return types.DerivationList
		}
		if typed.Union != nil || typed.Variety() == types.UnionVariety {
			return types.DerivationUnion
		}
		if typed.Restriction != nil || typed.ResolvedBase != nil {
			return types.DerivationRestriction
		}
	case *types.BuiltinType:
		return types.DerivationRestriction
	}
	return 0
}

func isRestrictionDerivedFrom(derived, base types.Type) bool {
	if derived == nil || base == nil {
		return false
	}
	baseCT, ok := base.(*types.ComplexType)
	if ok {
		return isRestrictionDerivedFromComplex(derived, baseCT)
	}
	return types.IsValidlyDerivedFrom(derived, base)
}

func isRestrictionDerivedFromComplex(derived types.Type, base *types.ComplexType) bool {
	derivedCT, ok := derived.(*types.ComplexType)
	if !ok {
		return false
	}
	if derivedCT == base {
		return true
	}
	current := derivedCT
	for current != nil && current != base {
		if current.DerivationMethod != types.DerivationRestriction {
			return false
		}
		nextDT, ok := types.AsDerivedType(current)
		if !ok {
			return false
		}
		next := nextDT.ResolvedBaseType()
		if next == nil {
			return false
		}
		if next == base {
			return true
		}
		current, _ = next.(*types.ComplexType)
	}
	return current == base
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
	// if the base group is a single wildcard particle, validate the restriction group
	// directly against the wildcard using NSRecurseCheckCardinality semantics.
	if len(baseMG.Particles) == 1 {
		if baseAny, ok := baseMG.Particles[0].(*types.AnyElement); ok {
			return validateParticlePairRestriction(schema, baseAny, restrictionMG)
		}
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
	case types.Choice:
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
	case types.AllGroup:
		if err := validateAllGroupRestriction(schema, baseMG, restrictionMG); err != nil {
			return err
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
	if schema != nil && baseMG != nil && baseMG.Kind == types.Choice && !baseMG.MaxOcc().IsOne() && len(baseChildren) > 1 {
		baseElemMax := baseElem.MaxOcc()
		if !baseElemMax.IsUnbounded() {
			restrictionMax := restrictionElem.MaxOcc()
			if restrictionMax.IsUnbounded() || restrictionMax.Cmp(baseElemMax) > 0 {
				if existing, ok := schema.ParticleRestrictionCaps[restrictionElem]; !ok || baseElemMax.Cmp(existing) < 0 {
					schema.ParticleRestrictionCaps[restrictionElem] = baseElemMax
				}
			}
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
	if err := validateElementRestriction(baseElem, restrictionElem); err != nil {
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

	baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc := effectiveParticleOccurrence(baseParticle, restrictionParticle)
	if err := validateOccurrenceConstraints(baseMinOcc, baseMaxOcc, restrictionMinOcc, restrictionMaxOcc); err != nil {
		return err
	}

	if handled, err := validateWildcardPairRestriction(baseParticle, restrictionParticle); handled {
		return err
	}

	if handled, err := validateElementPairRestrictions(schema, baseParticle, restrictionParticle); handled {
		return err
	}

	if handled, err := validateModelGroupPairRestrictions(schema, baseParticle, restrictionParticle); handled {
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

func validateElementPairRestrictions(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) (bool, error) {
	baseElem, baseIsElem := baseParticle.(*types.ElementDecl)
	if !baseIsElem {
		return false, nil
	}
	switch restriction := restrictionParticle.(type) {
	case *types.ElementDecl:
		if restriction.MinOcc().IsZero() && restriction.MaxOcc().IsZero() {
			return true, nil
		}
		if baseElem.Name != restriction.Name {
			if !isSubstitutableElement(schema, baseElem.Name, restriction.Name) {
				return true, fmt.Errorf("ComplexContent restriction: element name mismatch (%s vs %s)", baseElem.Name, restriction.Name)
			}
		}
		if err := validateElementRestriction(baseElem, restriction); err != nil {
			return true, err
		}
		return true, nil
	case *types.ModelGroup:
		if restriction.Kind != types.Choice {
			return true, fmt.Errorf("ComplexContent restriction: cannot restrict element %s to model group", baseElem.Name)
		}
		for _, p := range restriction.Particles {
			if p.MinOcc().IsZero() && p.MaxOcc().IsZero() {
				continue
			}
			childElem, ok := p.(*types.ElementDecl)
			if !ok {
				return true, fmt.Errorf("ComplexContent restriction: element %s restriction choice must contain only elements", baseElem.Name)
			}
			if err := validateParticlePairRestriction(schema, baseElem, childElem); err != nil {
				return true, fmt.Errorf("ComplexContent restriction: element %s restriction choice contains invalid particle: %w", baseElem.Name, err)
			}
		}
		return true, nil
	default:
		return false, nil
	}
}

func validateModelGroupPairRestrictions(schema *parser.Schema, baseParticle, restrictionParticle types.Particle) (bool, error) {
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
func validateElementRestriction(baseElem, restrictionElem *types.ElementDecl) error {
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
		// normalize both fixed values for comparison based on the element's type
		baseFixed := types.NormalizeWhiteSpace(baseElem.Fixed, baseElem.Type)
		restrictionFixed := types.NormalizeWhiteSpace(restrictionElem.Fixed, restrictionElem.Type)
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
	if baseElem.Type != nil && restrictionElem.Type != nil {
		// types are same if they have the same QName
		baseTypeName := baseElem.Type.Name()
		restrictionTypeName := restrictionElem.Type.Name()

		if baseTypeName.Namespace == types.XSDNamespace && baseTypeName.Local == "anyType" {
			return nil
		}
		if baseTypeName.Namespace == types.XSDNamespace && baseTypeName.Local == "anySimpleType" {
			switch restrictionElem.Type.(type) {
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
			if !isRestrictionDerivedFrom(restrictionElem.Type, baseElem.Type) {
				// for anonymous types, also check if they declare base type explicitly
				// anonymous simpleTypes with restrictions are valid if their base matches
				if st, ok := restrictionElem.Type.(*types.SimpleType); ok {
					if st.Restriction != nil && st.Restriction.Base == baseTypeName {
						return nil
					}
					// check if the anonymous type derives from the base through its ResolvedBase
					if st.ResolvedBase != nil && isRestrictionDerivedFrom(st.ResolvedBase, baseElem.Type) {
						return nil
					}
				}
				return fmt.Errorf("ComplexContent restriction: element '%s' anonymous type must be derived from base type '%s'", restrictionElem.Name, baseTypeName)
			}
			return nil
		}

		// if type names are different, restriction type must be derived from base type
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
func validateParticleRestrictionWithKindChange(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup) error {
	baseChildren := derivationChildren(baseMG)
	restrictionChildren := derivationChildren(restrictionMG)
	// special case: if base contains a wildcard, the restriction can use any compositor
	// per XSD spec section 7.2.5 (NSRecurseCheckCardinality), a model group can restrict a wildcard
	// with any compositor as long as the model group's particles are valid restrictions of the wildcard
	baseHasWildcard := modelGroupContainsWildcard(baseMG)

	if baseHasWildcard {
		return validateKindChangeWithWildcard(schema, baseChildren, restrictionMG, restrictionChildren)
	}

	if handled, err := validateAllGroupKindChange(schema, baseMG, restrictionMG, baseChildren, restrictionChildren); handled {
		return err
	}

	if baseMG.Kind == types.Sequence && restrictionMG.Kind == types.Choice {
		return fmt.Errorf("ComplexContent restriction: cannot restrict sequence to choice")
	}

	// choice -> sequence: Valid if all sequence particles match some choice particle
	if baseMG.Kind == types.Choice && restrictionMG.Kind == types.Sequence {
		return validateChoiceToSequenceRestriction(schema, baseMG, restrictionMG, baseChildren, restrictionChildren)
	}

	// other kind changes should not reach here due to early returns above
	return fmt.Errorf("ComplexContent restriction: invalid model group kind change from %s to %s", groupKindName(baseMG.Kind), groupKindName(restrictionMG.Kind))
}

func validateKindChangeWithWildcard(schema *parser.Schema, baseChildren []types.Particle, restrictionMG *types.ModelGroup, restrictionChildren []types.Particle) error {
	// find the wildcard in the base and validate the entire restriction group against it
	for _, baseParticle := range baseChildren {
		if baseWildcard, isWildcard := baseParticle.(*types.AnyElement); isWildcard {
			// this uses the ModelGroup:Wildcard derivation rule (NSRecurseCheckCardinality)
			if err := validateParticlePairRestriction(schema, baseWildcard, restrictionMG); err == nil {
				return nil
			}
		}
	}
	// if we found wildcards but couldn't validate against them, try particle-by-particle validation
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

func validateAllGroupKindChange(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup, baseChildren, restrictionChildren []types.Particle) (bool, error) {
	// no wildcards - apply strict compositor change rules
	// xs:all has unique semantics - non-all cannot restrict to xs:all
	// exception: xs:all with a single element can restrict sequence/choice
	// (no ordering ambiguity with one element)
	if restrictionMG.Kind == types.AllGroup && baseMG.Kind != types.AllGroup {
		// allow xs:all with single element to restrict sequence/choice
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

	// xs:all -> sequence/choice: Valid if restriction particles match base particles
	// per XSD spec, restricting xs:all to sequence/choice adds ordering constraints
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
				return true, fmt.Errorf("ComplexContent restriction: restriction particle is not a valid restriction of any base particle")
			}
		}
		return true, nil
	}

	return false, nil
}

func validateChoiceToSequenceRestriction(schema *parser.Schema, baseMG, restrictionMG *types.ModelGroup, baseChildren, restrictionChildren []types.Particle) error {
	derivedCount := len(restrictionChildren)
	countOccurs := types.OccursFromInt(derivedCount)
	derivedMin := types.MulOccurs(restrictionMG.MinOccurs, countOccurs)
	derivedMax := types.MulOccurs(restrictionMG.MaxOccurs, countOccurs)
	if err := validateOccurrenceConstraints(baseMG.MinOcc(), baseMG.MaxOcc(), derivedMin, derivedMax); err != nil {
		return err
	}
	// each restriction particle must be a valid restriction of at least one base particle
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

func normalizePointlessParticle(p types.Particle) types.Particle {
	for {
		mg, ok := p.(*types.ModelGroup)
		if !ok || mg == nil {
			return p
		}
		if !mg.MinOccurs.IsOne() || !mg.MaxOccurs.IsOne() {
			return p
		}
		children := derivationChildren(mg)
		if len(children) != 1 {
			return p
		}
		p = children[0]
	}
}

func derivationChildren(mg *types.ModelGroup) []types.Particle {
	if mg == nil {
		return nil
	}
	children := make([]types.Particle, 0, len(mg.Particles))
	for _, child := range mg.Particles {
		children = append(children, gatherPointlessChildren(mg.Kind, child)...)
	}
	return children
}

func gatherPointlessChildren(parentKind types.GroupKind, particle types.Particle) []types.Particle {
	switch p := particle.(type) {
	case *types.ElementDecl, *types.AnyElement:
		return []types.Particle{p}
	case *types.ModelGroup:
		if !p.MinOccurs.IsOne() || !p.MaxOccurs.IsOne() {
			return []types.Particle{p}
		}
		if len(p.Particles) == 1 {
			return gatherPointlessChildren(parentKind, p.Particles[0])
		}
		if p.Kind == parentKind {
			var out []types.Particle
			for _, child := range p.Particles {
				out = append(out, gatherPointlessChildren(parentKind, child)...)
			}
			return out
		}
		return []types.Particle{p}
	default:
		return []types.Particle{p}
	}
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
func calculateEffectiveOccurrence(mg *types.ModelGroup) (minOcc, maxOcc types.Occurs) {
	groupMinOcc := mg.MinOcc()
	groupMaxOcc := mg.MaxOcc()

	if len(mg.Particles) == 0 {
		return types.OccursFromInt(0), types.OccursFromInt(0)
	}

	switch mg.Kind {
	case types.Sequence:
		// for sequences, sum all children's occurrences
		sumMinOcc := types.OccursFromInt(0)
		sumMaxOcc := types.OccursFromInt(0)
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			sumMinOcc = types.AddOccurs(sumMinOcc, childMin)
			sumMaxOcc = types.AddOccurs(sumMaxOcc, childMax)
		}
		minOcc = types.MulOccurs(groupMinOcc, sumMinOcc)
		maxOcc = types.MulOccurs(groupMaxOcc, sumMaxOcc)
	case types.Choice:
		// for choices, take the min of children's minOccurs (since only one branch is taken)
		// and max of children's maxOccurs
		childMinOcc := types.OccursFromInt(0)
		childMaxOcc := types.OccursFromInt(0)
		childMinOccSet := false
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			if childMax.IsZero() {
				continue
			}
			if !childMinOccSet || childMin.Cmp(childMinOcc) < 0 {
				childMinOcc = childMin
				childMinOccSet = true
			}
			childMaxOcc = types.MaxOccurs(childMaxOcc, childMax)
		}
		if !childMinOccSet {
			childMinOcc = types.OccursFromInt(0)
		}
		minOcc = types.MulOccurs(groupMinOcc, childMinOcc)
		maxOcc = types.MulOccurs(groupMaxOcc, childMaxOcc)
	case types.AllGroup:
		// for all groups, sum all children (like sequence, all must appear)
		sumMinOcc := types.OccursFromInt(0)
		sumMaxOcc := types.OccursFromInt(0)
		for _, p := range mg.Particles {
			childMin, childMax := getParticleEffectiveOccurrence(p)
			sumMinOcc = types.AddOccurs(sumMinOcc, childMin)
			sumMaxOcc = types.AddOccurs(sumMaxOcc, childMax)
		}
		minOcc = types.MulOccurs(groupMinOcc, sumMinOcc)
		maxOcc = types.MulOccurs(groupMaxOcc, sumMaxOcc)
	default:
		minOcc = groupMinOcc
		maxOcc = groupMaxOcc
	}
	return
}

// getParticleEffectiveOccurrence gets the effective occurrence of a single particle
func getParticleEffectiveOccurrence(p types.Particle) (minOcc, maxOcc types.Occurs) {
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
		if particle.MinOcc().CmpInt(0) > 0 {
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
	if p.MaxOcc().IsZero() {
		return true
	}
	// minOccurs=0 means we can choose zero occurrences.
	if p.MinOcc().IsZero() {
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
		default:
			return false
		}
	case *types.ElementDecl, *types.AnyElement:
		return false
	default:
		return p.MinOcc().IsZero()
	}
}
