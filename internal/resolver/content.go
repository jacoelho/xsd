package resolver

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validation"
)

// validateContentReferences validates references within content models.
func validateContentReferences(schema *parser.Schema, typeQName types.QName, content types.Content, originLocation string) error {
	return validation.WalkContentParticles(content, func(particle types.Particle) error {
		return validateParticleReferences(schema, particle, originLocation)
	})
}

// validateParticleReferences validates references within particles.
func validateParticleReferences(schema *parser.Schema, particle types.Particle, originLocation string) error {
	visited := make(map[*types.ModelGroup]bool)
	return validateParticleReferencesWithVisited(schema, particle, visited, originLocation)
}

// validateParticleReferencesWithVisited validates references with cycle detection.
func validateParticleReferencesWithVisited(schema *parser.Schema, particle types.Particle, visited map[*types.ModelGroup]bool, originLocation string) error {
	switch p := particle.(type) {
	case *types.ModelGroup:
		// skip if already visited (prevents infinite recursion in cyclic groups).
		if visited[p] {
			return nil
		}
		visited[p] = true
		// recursively validate particles in the group.
		for _, childParticle := range p.Particles {
			if err := validateParticleReferencesWithVisited(schema, childParticle, visited, originLocation); err != nil {
				return err
			}
		}
	case *types.ElementDecl:
		if p.IsReference {
			if err := validateImportForNamespaceAtLocation(schema, originLocation, p.Name.Namespace); err != nil {
				return fmt.Errorf("element reference %s: %w", p.Name, err)
			}
			return nil
		}
		if p.Type != nil {
			// use SourceNamespace (the declaring schema's namespace) as context,
			// not p.Name.Namespace which may be empty for unqualified local elements.
			contextNS := p.SourceNamespace
			if contextNS.IsEmpty() {
				contextNS = p.Name.Namespace
			}
			if err := validateTypeReferenceFromTypeWithVisited(schema, p.Type, visited, typeReferenceAllowMissing, contextNS, originLocation); err != nil {
				return fmt.Errorf("element %s: %w", p.Name, err)
			}
			if err := validateAttributeValueConstraintsForType(schema, p.Type); err != nil {
				return fmt.Errorf("element %s: %w", p.Name, err)
			}
		}
	case *types.AnyElement:
		// wildcards don't have references.
	}
	return nil
}

// validateGroupReferences validates references within group definitions.
func validateGroupReferences(schema *parser.Schema, qname types.QName, group *types.ModelGroup) error {
	visited := make(map[*types.ModelGroup]bool)
	origin := schema.GroupOrigins[qname]
	// recursively validate particles in the group.
	for _, particle := range group.Particles {
		if err := validateParticleReferencesWithVisited(schema, particle, visited, origin); err != nil {
			return err
		}
	}
	return nil
}
