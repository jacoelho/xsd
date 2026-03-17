package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateParticleReferences validates references within particles.
func validateParticleReferences(schema *parser.Schema, particle model.Particle, originLocation string) error {
	visited := make(map[*model.ModelGroup]bool)
	return validateParticleReferencesWithVisited(schema, particle, visited, originLocation)
}

// validateParticleReferencesWithVisited validates references with cycle detection.
func validateParticleReferencesWithVisited(schema *parser.Schema, particle model.Particle, visited map[*model.ModelGroup]bool, originLocation string) error {
	switch p := particle.(type) {
	case *model.ModelGroup:
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
	case *model.ElementDecl:
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
			if contextNS == "" {
				contextNS = p.Name.Namespace
			}
			if err := validateTypeReferenceFromTypeWithVisited(schema, p.Type, visited, contextNS, originLocation); err != nil {
				return fmt.Errorf("element %s: %w", p.Name, err)
			}
			if err := validateAttributeValueConstraintsForType(schema, p.Type); err != nil {
				return fmt.Errorf("element %s: %w", p.Name, err)
			}
		}
	case *model.AnyElement:
		// wildcards don't have references.
	}
	return nil
}

// validateGroupReferences validates references within group definitions.
func validateGroupReferences(schema *parser.Schema, qname model.QName, group *model.ModelGroup) error {
	visited := make(map[*model.ModelGroup]bool)
	origin := schema.GroupOrigins[qname]
	// recursively validate particles in the group.
	for _, particle := range group.Particles {
		if err := validateParticleReferencesWithVisited(schema, particle, visited, origin); err != nil {
			return err
		}
	}
	return nil
}
