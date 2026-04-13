package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateAllGroupConstraints(group *model.ModelGroup, parentKind *model.GroupKind) error {
	if parentKind != nil {
		if *parentKind == model.Sequence || *parentKind == model.Choice {
			return fmt.Errorf("xs:all cannot appear as a child of xs:sequence or xs:choice (XSD 1.0)")
		}
	}
	if err := validateAllGroupUniqueElements(group.Particles); err != nil {
		return err
	}
	if err := validateAllGroupOccurrence(group); err != nil {
		return err
	}
	if err := validateAllGroupParticleOccurs(group.Particles); err != nil {
		return err
	}
	return validateAllGroupNested(group.Particles)
}

func validateAllGroupUniqueElements(particles []model.Particle) error {
	seenElements := make(map[model.QName]bool)
	for _, childParticle := range particles {
		childElem, ok := childParticle.(*model.ElementDecl)
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

func validateAllGroupOccurrence(group *model.ModelGroup) error {
	switch model.CheckAllGroupBounds(group.MinOccurs, group.MaxOccurs) {
	case model.AllGroupMinNotZeroOrOne:
		return fmt.Errorf("xs:all must have minOccurs='0' or '1' (got %s)", group.MinOccurs)
	case model.AllGroupMaxNotOne:
		return fmt.Errorf("xs:all must have maxOccurs='1' (got %s)", group.MaxOccurs)
	}
	return nil
}

func validateAllGroupParticleOccurs(particles []model.Particle) error {
	for _, childParticle := range particles {
		if !model.IsAllGroupChildMaxValid(childParticle.MaxOcc()) {
			return fmt.Errorf("xs:all: all particles must have maxOccurs <= 1 (got %s)", childParticle.MaxOcc())
		}
	}
	return nil
}

func validateAllGroupNested(particles []model.Particle) error {
	for _, childParticle := range particles {
		childMG, ok := childParticle.(*model.ModelGroup)
		if !ok {
			continue
		}
		if childMG.Kind == model.AllGroup && childMG.MinOccurs.CmpInt(0) > 0 {
			return fmt.Errorf("xs:all: nested xs:all cannot have minOccurs > 0 (got %s)", childMG.MinOccurs)
		}
	}
	return nil
}

// validateElementDeclarationsConsistentInParticle validates "Element Declarations Consistent"
// across a particle tree, including nested model groups and group references.
func validateElementDeclarationsConsistentInParticle(schema *parser.Schema, particle model.Particle) error {
	seen := make(map[model.QName]model.Type)
	visited := newModelGroupVisit()
	return validateElementDeclarationsConsistentWithVisited(schema, particle, seen, visited)
}

func validateElementDeclarationsConsistentWithVisited(schema *parser.Schema, particle model.Particle, seen map[model.QName]model.Type, visited modelGroupVisitTracker) error {
	switch p := particle.(type) {
	case *model.ModelGroup:
		if !visited.Enter(p) {
			return nil
		}
		for _, child := range p.Particles {
			if err := validateElementDeclarationsConsistentWithVisited(schema, child, seen, visited); err != nil {
				return err
			}
		}
	case *model.GroupRef:
		if schema == nil {
			return nil
		}
		if group, ok := schema.Groups[p.RefQName]; ok {
			return validateElementDeclarationsConsistentWithVisited(schema, group, seen, visited)
		}
	case *model.ElementDecl:
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
			if !model.ElementTypesCompatible(existing, elemType) {
				return fmt.Errorf("element declarations consistent violation for element '%s'", p.Name)
			}
			return nil
		}
		seen[p.Name] = elemType
	}
	return nil
}

func validateElementParticle(schema *parser.Schema, elem *model.ElementDecl) error {
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

func validateElementConstraints(elem *model.ElementDecl) error {
	for _, constraint := range elem.Constraints {
		if err := validateIdentityConstraint(constraint); err != nil {
			return fmt.Errorf("element '%s' identity constraint '%s': %w", elem.Name, constraint.Name, err)
		}
	}
	return nil
}

func validateElementConstraintNames(elem *model.ElementDecl) error {
	constraintNames := make(map[string]bool)
	for _, constraint := range elem.Constraints {
		if constraintNames[constraint.Name] {
			return fmt.Errorf("element '%s': duplicate identity constraint name '%s'", elem.Name, constraint.Name)
		}
		constraintNames[constraint.Name] = true
	}
	return nil
}

func validateReferencedElementType(schema *parser.Schema, elem *model.ElementDecl) error {
	if refDecl, exists := schema.ElementDecls[elem.Name]; exists && refDecl.Type == nil {
		return fmt.Errorf("referenced element '%s' must have a type", elem.Name)
	}
	return nil
}

func validateInlineElementType(schema *parser.Schema, elem *model.ElementDecl) error {
	if st, ok := elem.Type.(*model.SimpleType); ok && st.QName.IsZero() {
		if err := validateSimpleTypeStructure(schema, st); err != nil {
			return fmt.Errorf("inline simpleType in element '%s': %w", elem.Name, err)
		}
		return nil
	}
	if complexType, ok := elem.Type.(*model.ComplexType); ok && complexType.QName.IsZero() {
		if err := validateComplexTypeStructure(schema, complexType, typeDefinitionInline); err != nil {
			return fmt.Errorf("inline complexType in element '%s': %w", elem.Name, err)
		}
	}
	return nil
}

// validateGroupStructure validates structural constraints of a group definition.
// Does not validate references, which might be forward references or imports.
func validateGroupStructure(groupQName model.QName, group *model.ModelGroup) error {
	if !model.IsValidNCName(groupQName.Local) {
		return fmt.Errorf("invalid group name '%s': must be a valid NCName", groupQName.Local)
	}

	if group.MinOccurs.IsZero() {
		return fmt.Errorf("group '%s' cannot have minOccurs='0'", groupQName.Local)
	}
	if group.MaxOccurs.IsUnbounded() {
		return fmt.Errorf("group '%s' cannot have maxOccurs='unbounded'", groupQName.Local)
	}
	if !group.MinOccurs.IsOne() || !group.MaxOccurs.IsOne() {
		return fmt.Errorf("group '%s' must have minOccurs='1' and maxOccurs='1' (got minOccurs=%s, maxOccurs=%s)", groupQName.Local, group.MinOccurs, group.MaxOccurs)
	}

	return nil
}

// validateParticleStructure validates structural constraints of particles.
func validateParticleStructure(schema *parser.Schema, particle model.Particle) error {
	visited := newModelGroupVisit()
	return validateParticleStructureWithVisited(schema, particle, nil, visited)
}

// validateParticleStructureWithVisited validates structural constraints with cycle detection
func validateParticleStructureWithVisited(schema *parser.Schema, particle model.Particle, parentKind *model.GroupKind, visited modelGroupVisitTracker) error {
	if err := validateParticleOccurs(particle); err != nil {
		return err
	}
	switch p := particle.(type) {
	case *model.ModelGroup:
		return validateModelGroupStructure(schema, p, parentKind, visited)
	case *model.GroupRef:
	case *model.AnyElement:
	case *model.ElementDecl:
		return validateElementParticle(schema, p)
	}
	return nil
}

func validateParticleOccurs(particle model.Particle) error {
	maxOcc := particle.MaxOcc()
	minOcc := particle.MinOcc()
	switch model.CheckBounds(minOcc, maxOcc) {
	case model.BoundsOverflow:
		return fmt.Errorf("%w: occurrence value exceeds uint32", model.ErrOccursOverflow)
	case model.BoundsMaxZeroWithMinNonZero:
		return fmt.Errorf("maxOccurs cannot be 0 when minOccurs > 0")
	case model.BoundsMinGreaterThanMax:
		return fmt.Errorf("minOccurs (%s) cannot be greater than maxOccurs (%s)", minOcc, maxOcc)
	}
	return nil
}

func validateModelGroupStructure(schema *parser.Schema, group *model.ModelGroup, parentKind *model.GroupKind, visited modelGroupVisitTracker) error {
	if !visited.Enter(group) {
		return nil
	}

	if err := validateLocalElementTypes(group.Particles); err != nil {
		return err
	}
	if group.Kind == model.AllGroup {
		if err := validateAllGroupConstraints(group, parentKind); err != nil {
			return err
		}
	}

	for _, childParticle := range group.Particles {
		if err := validateParticleStructureWithVisited(schema, childParticle, &group.Kind, visited); err != nil {
			return err
		}
	}
	return nil
}

func validateLocalElementTypes(particles []model.Particle) error {
	localElementTypes := make(map[model.QName]model.Type)
	for _, childParticle := range particles {
		childElem, ok := childParticle.(*model.ElementDecl)
		if !ok || childElem.IsReference {
			continue
		}
		if existingType, exists := localElementTypes[childElem.Name]; exists {
			if !model.ElementTypesCompatible(existingType, childElem.Type) {
				return fmt.Errorf("duplicate local element declaration '%s' with different types", childElem.Name)
			}
			continue
		}
		localElementTypes[childElem.Name] = childElem.Type
	}
	return nil
}
