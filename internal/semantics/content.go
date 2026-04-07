package semantics

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
		return validateModelGroupParticleReferences(schema, p, visited, originLocation)
	case *model.ElementDecl:
		return validateElementDeclParticleReferences(schema, p, visited, originLocation)
	case *model.AnyElement:
		// wildcards don't have references.
	}
	return nil
}

func validateModelGroupParticleReferences(schema *parser.Schema, group *model.ModelGroup, visited map[*model.ModelGroup]bool, originLocation string) error {
	// skip if already visited (prevents infinite recursion in cyclic groups).
	if visited[group] {
		return nil
	}
	visited[group] = true

	// recursively validate particles in the group.
	for _, childParticle := range group.Particles {
		if err := validateParticleReferencesWithVisited(schema, childParticle, visited, originLocation); err != nil {
			return err
		}
	}
	return nil
}

func validateElementDeclParticleReferences(schema *parser.Schema, elem *model.ElementDecl, visited map[*model.ModelGroup]bool, originLocation string) error {
	if elem.IsReference {
		if err := validateImportForNamespaceAtLocation(schema, originLocation, elem.Name.Namespace); err != nil {
			return fmt.Errorf("element reference %s: %w", elem.Name, err)
		}
		return nil
	}
	if elem.Type == nil {
		return nil
	}

	contextNS := elementDeclContextNamespace(elem)
	if err := validateTypeReferenceFromTypeWithVisited(schema, elem.Type, visited, contextNS, originLocation); err != nil {
		return fmt.Errorf("element %s: %w", elem.Name, err)
	}
	if err := validateAttributeValueConstraintsForType(schema, elem.Type); err != nil {
		return fmt.Errorf("element %s: %w", elem.Name, err)
	}
	return nil
}

func elementDeclContextNamespace(elem *model.ElementDecl) model.NamespaceURI {
	// use SourceNamespace (the declaring schema's namespace) as context,
	// not Name.Namespace which may be empty for unqualified local elements.
	if elem.SourceNamespace != "" {
		return elem.SourceNamespace
	}
	return elem.Name.Namespace
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
