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
	case *types.AnyElement:
	case *types.ElementDecl:
		return validateElementParticle(schema, p)
	}
	return nil
}

func validateParticleOccurs(particle types.Particle) error {
	maxOcc := particle.MaxOcc()
	minOcc := particle.MinOcc()
	if maxOcc.IsOverflow() || minOcc.IsOverflow() {
		return fmt.Errorf("%w: occurrence value exceeds uint32", types.ErrOccursOverflow)
	}

	if maxOcc.IsZero() && !minOcc.IsZero() {
		return fmt.Errorf("maxOccurs cannot be 0 when minOccurs > 0")
	}
	if !maxOcc.IsUnbounded() && !maxOcc.IsZero() && maxOcc.Cmp(minOcc) < 0 {
		return fmt.Errorf("minOccurs (%s) cannot be greater than maxOccurs (%s)", minOcc, maxOcc)
	}
	return nil
}

func validateModelGroupStructure(schema *parser.Schema, group *types.ModelGroup, parentKind *types.GroupKind, visited modelGroupVisit) error {
	if !visited.Enter(group) {
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

	for _, childParticle := range group.Particles {
		if err := validateParticleStructureWithVisited(schema, childParticle, &group.Kind, visited); err != nil {
			return err
		}
	}
	return nil
}

func validateLocalElementTypes(particles []types.Particle) error {
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
