package semanticcheck

import (
	"fmt"

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
