package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateElementDeclarationsConsistent validates that element declarations are consistent
// in extensions. According to XSD spec "Element Declarations Consistent": when extending
// a complex type, elements in the extension cannot have the same name as elements in the
// base type with different types.
func validateElementDeclarationsConsistent(schema *schema.Schema, ct *types.ComplexType) error {
	if !ct.IsExtension() {
		return nil
	}

	content := ct.Content()
	ext := content.ExtensionDef()
	if ext == nil {
		return nil
	}

	baseQName := content.BaseTypeQName()
	baseType, ok := schema.TypeDefs[baseQName]
	if !ok {
		return nil // Base type not found - might be builtin or forward reference
	}

	baseCT, ok := baseType.(*types.ComplexType)
	if !ok {
		return nil // Base type is not complex - no elements to check
	}

	baseElements := collectAllElementDeclarationsFromType(schema, baseCT)

	// SimpleContent extensions don't have particles
	if ext.Particle == nil {
		return nil
	}
	extElements := collectElementDeclarationsFromParticle(ext.Particle)

	for _, extElem := range extElements {
		for _, baseElem := range baseElements {
			// Check if names match (same local name and namespace)
			if extElem.Name == baseElem.Name {
				// Names match - types must also match
				// Compare types by checking if they're the same object or have the same QName
				extTypeQName := getTypeQName(extElem.Type)
				baseTypeQName := getTypeQName(baseElem.Type)
				if extTypeQName != baseTypeQName {
					return fmt.Errorf("element '%s' in extension has type '%s' but base type has type '%s' (Element Declarations Consistent violation)", extElem.Name.Local, extTypeQName, baseTypeQName)
				}
			}
		}
	}

	return nil
}

// collectAllElementDeclarationsFromType collects all element declarations from a complex type
// This recursively collects from the type's content model and its base types
func collectAllElementDeclarationsFromType(schema *schema.Schema, ct *types.ComplexType) []*types.ElementDecl {
	visited := make(map[types.QName]bool)
	return collectElementDeclarationsRecursive(schema, ct, visited)
}

// collectElementDeclarationsRecursive recursively collects element declarations from a type and its base types
func collectElementDeclarationsRecursive(schema *schema.Schema, ct *types.ComplexType, visited map[types.QName]bool) []*types.ElementDecl {
	// Avoid infinite loops
	if visited[ct.QName] {
		return nil
	}
	visited[ct.QName] = true

	var result []*types.ElementDecl

	// Collect from this type's content
	content := ct.Content()
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			result = append(result, collectElementDeclarationsFromParticle(c.Particle)...)
		}
	case *types.ComplexContent:
		// For extensions, collect from extension particles
		if c.Extension != nil && c.Extension.Particle != nil {
			result = append(result, collectElementDeclarationsFromParticle(c.Extension.Particle)...)
		}
		// For restrictions, collect from restriction particles (which restrict base)
		if c.Restriction != nil && c.Restriction.Particle != nil {
			result = append(result, collectElementDeclarationsFromParticle(c.Restriction.Particle)...)
		}
		// Also collect from base type recursively
		var baseQName types.QName
		if c.Extension != nil {
			baseQName = c.Extension.Base
		} else if c.Restriction != nil {
			baseQName = c.Restriction.Base
		}
		if !baseQName.IsZero() {
			if baseType, ok := schema.TypeDefs[baseQName]; ok {
				if baseCT, ok := baseType.(*types.ComplexType); ok {
					result = append(result, collectElementDeclarationsRecursive(schema, baseCT, visited)...)
				}
			}
		}
	}
	return result
}

// collectElementDeclarationsFromParticle collects all element declarations from a particle (recursively)
func collectElementDeclarationsFromParticle(particle types.Particle) []*types.ElementDecl {
	var result []*types.ElementDecl
	switch p := particle.(type) {
	case *types.ModelGroup:
		// Recursively collect from all particles in the group
		for _, child := range p.Particles {
			result = append(result, collectElementDeclarationsFromParticle(child)...)
		}
	case *types.ElementDecl:
		result = append(result, p)
	case *types.AnyElement:
		// Wildcards don't have element declarations
	}
	return result
}
