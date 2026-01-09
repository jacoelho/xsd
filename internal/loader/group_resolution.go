package loader

import (
	"fmt"

	schema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/types"
)

// resolveGroupReferences resolves all GroupRef placeholders in a schema
// This must be called after all schemas (including imports/includes) are loaded
func (l *SchemaLoader) resolveGroupReferences(schema *schema.Schema) error {
	// first, resolve all top-level groups (they may reference each other)
	detector := resolver.NewCycleDetector[types.QName]()

	for qname, group := range schema.Groups {
		if err := detector.WithScope(qname, func() error {
			return l.resolveGroupRefsInModelGroupWithCycleDetection(group, schema, detector)
		}); err != nil {
			return fmt.Errorf("resolve group refs in group %s: %w", qname, err)
		}
	}

	for _, typ := range schema.TypeDefs {
		if ct, ok := typ.(*types.ComplexType); ok {
			if err := l.resolveGroupRefsInContentWithVisited(ct.Content(), schema, detector); err != nil {
				return fmt.Errorf("resolve group refs in type %s: %w", ct.QName, err)
			}
		}
	}

	for _, elem := range schema.ElementDecls {
		if elem.Type != nil {
			if ct, ok := elem.Type.(*types.ComplexType); ok {
				if err := l.resolveGroupRefsInContentWithVisited(ct.Content(), schema, detector); err != nil {
					return fmt.Errorf("resolve group refs in element %s: %w", elem.Name, err)
				}
			}
		}
	}

	return nil
}

// resolveGroupRefsInContentWithVisited resolves GroupRef placeholders in content with cycle detector
func (l *SchemaLoader) resolveGroupRefsInContentWithVisited(content types.Content, schema *schema.Schema, detector *resolver.CycleDetector[types.QName]) error {
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			resolved, err := l.resolveGroupRefsInParticleWithVisited(c.Particle, schema, detector)
			if err != nil {
				return err
			}
			if resolved != nil {
				c.Particle = resolved
			}
		}
	case *types.ComplexContent:
		if c.Restriction != nil && c.Restriction.Particle != nil {
			resolved, err := l.resolveGroupRefsInParticleWithVisited(c.Restriction.Particle, schema, detector)
			if err != nil {
				return err
			}
			if resolved != nil {
				c.Restriction.Particle = resolved
			}
		}
		if c.Extension != nil && c.Extension.Particle != nil {
			resolved, err := l.resolveGroupRefsInParticleWithVisited(c.Extension.Particle, schema, detector)
			if err != nil {
				return err
			}
			if resolved != nil {
				c.Extension.Particle = resolved
			}
		}
	}
	return nil
}

// resolveGroupRefsInParticleWithVisited resolves GroupRef placeholders in a particle with cycle detector
// Returns the resolved particle if it was a GroupRef, nil otherwise
func (l *SchemaLoader) resolveGroupRefsInParticleWithVisited(particle types.Particle, schema *schema.Schema, detector *resolver.CycleDetector[types.QName]) (types.Particle, error) {
	// check if this particle is a GroupRef that needs resolution
	if groupRef, ok := particle.(*types.GroupRef); ok {
		// look up the actual group
		groupDef, ok := schema.Groups[groupRef.RefQName]
		if !ok {
			return nil, fmt.Errorf("group '%s' not found", groupRef.RefQName)
		}
		// create a copy of the group with occurrence constraints from the reference
		// note: Group references with minOccurs > 1 are valid XSD. UPA validation will catch
		// any actual UPA violations that arise from ambiguous content models.
		groupCopy := *groupDef
		groupCopy.MinOccurs = groupRef.MinOccurs
		groupCopy.MaxOccurs = groupRef.MaxOccurs
		// the group is already resolved (it's in schema.Groups and was resolved earlier)
		// just return the copy
		return &groupCopy, nil
	}

	// if it's a ModelGroup, resolve recursively
	if mg, ok := particle.(*types.ModelGroup); ok {
		if err := l.resolveGroupRefsInModelGroupWithCycleDetection(mg, schema, detector); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// resolveGroupRefsInModelGroupWithCycleDetection resolves GroupRef placeholders with cycle detection
func (l *SchemaLoader) resolveGroupRefsInModelGroupWithCycleDetection(mg *types.ModelGroup, schema *schema.Schema, detector *resolver.CycleDetector[types.QName]) error {
	return l.resolveGroupRefsInModelGroupWithPointerCycleDetection(mg, schema, detector, make(map[*types.ModelGroup]bool))
}

// resolveGroupRefsInModelGroupWithPointerCycleDetection resolves GroupRef placeholders with both QName and pointer-based cycle detection
func (l *SchemaLoader) resolveGroupRefsInModelGroupWithPointerCycleDetection(mg *types.ModelGroup, schema *schema.Schema, detector *resolver.CycleDetector[types.QName], visitedMGs map[*types.ModelGroup]bool) error {
	// pointer-based cycle detection for ModelGroup structures
	if visitedMGs[mg] {
		return nil // already processed this ModelGroup
	}
	visitedMGs[mg] = true

	for i, particle := range mg.Particles {
		if groupRef, ok := particle.(*types.GroupRef); ok {
			if err := detector.Enter(groupRef.RefQName); err != nil {
				return fmt.Errorf("circular group reference detected: %s", groupRef.RefQName)
			}
			defer detector.Leave(groupRef.RefQName)

			// look up the actual group
			groupDef, ok := schema.Groups[groupRef.RefQName]
			if !ok {
				return fmt.Errorf("group '%s' not found", groupRef.RefQName)
			}

			// if the group is already resolved (visited), just copy it
			if detector.IsVisited(groupRef.RefQName) {
				// create a deep copy of the already-resolved group with occurrence constraints from the reference
				groupCopy := deepCopyModelGroup(groupDef)
				groupCopy.MinOccurs = groupRef.MinOccurs
				groupCopy.MaxOccurs = groupRef.MaxOccurs
				mg.Particles[i] = groupCopy
				continue
			}

			// create a deep copy of the group with occurrence constraints from the reference
			groupCopy := deepCopyModelGroup(groupDef)
			groupCopy.MinOccurs = groupRef.MinOccurs
			groupCopy.MaxOccurs = groupRef.MaxOccurs

			// recursively resolve any GroupRefs in the copied group
			// use a fresh visitedMGs since this is a new copy
			if err := l.resolveGroupRefsInModelGroupWithPointerCycleDetection(groupCopy, schema, detector, make(map[*types.ModelGroup]bool)); err != nil {
				return err
			}

			// replace the GroupRef with the resolved group
			mg.Particles[i] = groupCopy
		} else if nestedMG, ok := particle.(*types.ModelGroup); ok {
			// recursively resolve nested model groups
			if err := l.resolveGroupRefsInModelGroupWithPointerCycleDetection(nestedMG, schema, detector, visitedMGs); err != nil {
				return err
			}
		}
	}
	return nil
}

// deepCopyModelGroup creates a deep copy of a ModelGroup including its Particles slice
func deepCopyModelGroup(mg *types.ModelGroup) *types.ModelGroup {
	if mg == nil {
		return nil
	}
	copy := *mg
	if mg.Particles != nil {
		copy.Particles = make([]types.Particle, len(mg.Particles))
		for i, p := range mg.Particles {
			copy.Particles[i] = p
		}
	}
	return &copy
}
