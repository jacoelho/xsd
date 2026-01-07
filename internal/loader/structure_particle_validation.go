package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateParticleStructure validates structural constraints of particles
// parentKind is the kind of the parent model group (nil if no parent)
func validateParticleStructure(schema *schema.Schema, particle types.Particle, parentKind *types.GroupKind) error {
	visited := make(map[*types.ModelGroup]bool)
	return validateParticleStructureWithVisited(schema, particle, parentKind, visited)
}

// validateParticleStructureWithVisited validates structural constraints with cycle detection
func validateParticleStructureWithVisited(schema *schema.Schema, particle types.Particle, parentKind *types.GroupKind, visited map[*types.ModelGroup]bool) error {
	// per XSD spec "Particle Correct":
	// 1. maxOccurs must be >= 1 (cannot be 0), EXCEPT when minOccurs=0 and maxOccurs=0
	//    (which effectively means the particle cannot appear - used in restrictions)
	// 2. when maxOccurs >= 1, it must be >= minOccurs
	maxOcc := particle.MaxOcc()
	minOcc := particle.MinOcc()

	// maxOccurs can be -1 (unbounded) or a positive integer >= 1, or 0 if minOccurs=0
	// note: W3C test suite includes schemas with maxOccurs=0 and minOccurs=0, which
	// we accept for compatibility even though the spec says maxOccurs >= 1
	if maxOcc == 0 && minOcc != 0 {
		return fmt.Errorf("maxOccurs cannot be 0 when minOccurs > 0")
	}
	if maxOcc > 0 && minOcc > maxOcc {
		return fmt.Errorf("minOccurs (%d) cannot be greater than maxOccurs (%d)", minOcc, maxOcc)
	}

	switch p := particle.(type) {
	case *types.ModelGroup:
		// cycle detection: skip if already visited
		if visited[p] {
			return nil
		}
		visited[p] = true
		// check for duplicate local element declarations within the same model group.
		// per XSD 1.0 "Element Declarations Consistent", declarations with the same
		// QName must have consistent type definitions.
		localElementTypes := make(map[types.QName]types.Type)
		for _, childParticle := range p.Particles {
			if childElem, ok := childParticle.(*types.ElementDecl); ok {
				if childElem.IsReference {
					continue
				}
				if existingType, exists := localElementTypes[childElem.Name]; exists {
					if !elementTypesCompatible(existingType, childElem.Type) {
						return fmt.Errorf("duplicate local element declaration '%s' with different types", childElem.Name)
					}
				} else {
					localElementTypes[childElem.Name] = childElem.Type
				}
			}
		}
		// check if xs:all is nested inside xs:sequence or xs:choice
		if p.Kind == types.AllGroup {
			if parentKind != nil {
				if *parentKind == types.Sequence || *parentKind == types.Choice {
					return fmt.Errorf("xs:all cannot appear as a child of xs:sequence or xs:choice (XSD 1.0)")
				}
			}
			// xs:all requires unique element declarations (no duplicate element names)
			// per XSD 1.0 Structures spec section 3.8.6: all group particles must be distinct.
			seenElements := make(map[types.QName]bool)
			for _, childParticle := range p.Particles {
				if childElem, ok := childParticle.(*types.ElementDecl); ok {
					if seenElements[childElem.Name] {
						return fmt.Errorf("xs:all: duplicate element declaration '%s'", childElem.Name)
					}
					seenElements[childElem.Name] = true
				}
			}
			// xs:all must have minOccurs="0" or "1", and maxOccurs="1" (the defaults)
			// it cannot have any other occurrence values
			if p.MinOccurs != 0 && p.MinOccurs != 1 {
				return fmt.Errorf("xs:all must have minOccurs='0' or '1' (got %d)", p.MinOccurs)
			}
			if p.MaxOccurs != 1 {
				return fmt.Errorf("xs:all must have maxOccurs='1' (got %d)", p.MaxOccurs)
			}
			// all particles in xs:all must have maxOccurs <= 1
			for _, childParticle := range p.Particles {
				if childParticle.MaxOcc() > 1 {
					return fmt.Errorf("xs:all: all particles must have maxOccurs <= 1 (got %d)", childParticle.MaxOcc())
				}
			}
			// no nested xs:all with minOccurs > 0
			for _, childParticle := range p.Particles {
				if childMG, ok := childParticle.(*types.ModelGroup); ok {
					if childMG.Kind == types.AllGroup && childMG.MinOccurs > 0 {
						return fmt.Errorf("xs:all: nested xs:all cannot have minOccurs > 0 (got %d)", childMG.MinOccurs)
					}
				}
			}
		}

		// recursively validate particles with this group as parent
		for _, childParticle := range p.Particles {
			if err := validateParticleStructureWithVisited(schema, childParticle, &p.Kind, visited); err != nil {
				return err
			}
		}
	case *types.GroupRef:
		// group references are particles that reference a named model group
		// the actual group validation happens when the referenced group is defined
		// no additional structural validation needed here beyond occurrence constraints (checked above)
	case *types.AnyElement:
		// wildcard elements don't have additional structural constraints beyond occurrence
	case *types.ElementDecl:
		for _, constraint := range p.Constraints {
			if err := validateIdentityConstraint(schema, constraint, p); err != nil {
				return fmt.Errorf("element '%s' identity constraint '%s': %w", p.Name, constraint.Name, err)
			}
		}

		constraintNames := make(map[string]bool)
		for _, constraint := range p.Constraints {
			if constraintNames[constraint.Name] {
				return fmt.Errorf("element '%s': duplicate identity constraint name '%s'", p.Name, constraint.Name)
			}
			constraintNames[constraint.Name] = true
		}
		// element references don't need their own type - they inherit from the referenced element
		if p.IsReference {
			// this is an element reference - check that the referenced element exists and has a type
			if refDecl, exists := schema.ElementDecls[p.Name]; exists {
				if refDecl.Type == nil {
					return fmt.Errorf("referenced element '%s' must have a type", p.Name)
				}
			}
			// if referenced element doesn't exist, that's a different error (forward reference or missing import)
			// don't validate type here for references
		} else if p.Type == nil {
			// elements without explicit types default to anyType (handled by parser)
			// this should not happen, but if it does, it's not a structural error
			// (the parser should have set anyType as default)
		} else {
			// validate inline types (simpleType or complexType defined inline in the element)
			if st, ok := p.Type.(*types.SimpleType); ok && st.QName.IsZero() {
				if err := validateSimpleTypeStructure(schema, st); err != nil {
					return fmt.Errorf("inline simpleType in element '%s': %w", p.Name, err)
				}
			} else if ct, ok := p.Type.(*types.ComplexType); ok && ct.QName.IsZero() {
				// this is a local element (particle in content model), so isInline=true
				if err := validateComplexTypeStructure(schema, ct, true); err != nil {
					return fmt.Errorf("inline complexType in element '%s': %w", p.Name, err)
				}
			}
		}
	}
	return nil
}

// validateElementDeclarationsConsistentInParticle validates "Element Declarations Consistent"
// across a particle tree, including nested model groups and group references.
func validateElementDeclarationsConsistentInParticle(schema *schema.Schema, particle types.Particle) error {
	seen := make(map[types.QName]types.Type)
	visited := make(map[*types.ModelGroup]bool)
	return validateElementDeclarationsConsistentWithVisited(schema, particle, seen, visited)
}

func validateElementDeclarationsConsistentWithVisited(schema *schema.Schema, particle types.Particle, seen map[types.QName]types.Type, visited map[*types.ModelGroup]bool) error {
	switch p := particle.(type) {
	case *types.ModelGroup:
		if visited[p] {
			return nil
		}
		visited[p] = true
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
func validateGroupStructure(schema *schema.Schema, qname types.QName, group *types.ModelGroup) error {
	// this is a structural constraint that is definitely invalid if violated
	if !isValidNCName(qname.Local) {
		return fmt.Errorf("invalid group name '%s': must be a valid NCName", qname.Local)
	}

	// according to XSD spec: groups cannot have minOccurs="0" or maxOccurs="unbounded" directly
	// groups must have minOccurs="1" and maxOccurs="1" (the defaults)
	if group.MinOccurs == 0 {
		return fmt.Errorf("group '%s' cannot have minOccurs='0'", qname.Local)
	}
	if group.MaxOccurs == types.UnboundedOccurs {
		return fmt.Errorf("group '%s' cannot have maxOccurs='unbounded'", qname.Local)
	}
	if group.MinOccurs != 1 || group.MaxOccurs != 1 {
		return fmt.Errorf("group '%s' must have minOccurs='1' and maxOccurs='1' (got minOccurs=%d, maxOccurs=%d)", qname.Local, group.MinOccurs, group.MaxOccurs)
	}

	// don't validate content model references - they might be forward references
	return nil
}