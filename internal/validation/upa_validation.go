package validation

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateUPA validates Unique Particle Attribution for a content model
// UPA requires that no element can be matched by more than one particle
// UPA violations occur when particles in a choice group can both match the same element
func validateUPA(schema *parser.Schema, content types.Content, targetNS types.NamespaceURI) error {
	var particle types.Particle
	var baseParticle types.Particle

	// get particles from the content model (preserving structure)
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			particle = c.Particle
		}
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			particle = c.Extension.Particle
			// for extensions, also get base type particle to check for UPA violations
			if !c.Extension.Base.IsZero() {
				if baseType, ok := schema.TypeDefs[c.Extension.Base]; ok {
					if baseCT, ok := baseType.(*types.ComplexType); ok {
						baseContent := baseCT.Content()
						if baseEC, ok := baseContent.(*types.ElementContent); ok {
							baseParticle = baseEC.Particle
						}
					}
				}
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			particle = c.Restriction.Particle
		}
	}

	// resolve GroupRef if present (should be resolved already, but handle it)
	if groupRef, ok := particle.(*types.GroupRef); ok {
		groupDef, exists := schema.Groups[groupRef.RefQName]
		if !exists {
			return fmt.Errorf("group '%s' not found", groupRef.RefQName)
		}
		// note: Group references with minOccurs > 1 are valid XSD. UPA validation will catch
		// any actual UPA violations that arise from ambiguous content models.
		groupCopy := *groupDef
		groupCopy.MinOccurs = groupRef.MinOccurs
		groupCopy.MaxOccurs = groupRef.MaxOccurs
		particle = &groupCopy
	}

	// note: A sequence group with minOccurs > 1 CAN be used directly as content in a complexType.
	// this is valid XSD - it means the sequence must appear at least minOccurs times.
	// the UPA validation below will catch any actual UPA violations.

	// validate UPA in the particle structure (context-aware)
	if particle != nil {
		if err := validateUPAInParticle(schema, particle, baseParticle, targetNS, nil); err != nil {
			return err
		}
	}

	return nil
}

// validateUPAInParticle validates UPA violations in a particle structure
// parentKind indicates the kind of parent model group (nil if top-level)
func validateUPAInParticle(schema *parser.Schema, particle types.Particle, baseParticle types.Particle, targetNS types.NamespaceURI, parentKind *types.GroupKind) error {
	visited := make(map[*types.ModelGroup]bool)
	return validateUPAInParticleWithVisited(schema, particle, baseParticle, targetNS, parentKind, visited)
}

// validateUPAInParticleWithVisited validates UPA violations with cycle detection
func validateUPAInParticleWithVisited(schema *parser.Schema, particle types.Particle, baseParticle types.Particle, targetNS types.NamespaceURI, parentKind *types.GroupKind, visited map[*types.ModelGroup]bool) error {
	switch p := particle.(type) {
	case *types.GroupRef:
		// GroupRef should be resolved before UPA validation, but handle it just in case
		// if we encounter an unresolved GroupRef, we can't validate UPA properly
		// this should not happen in normal flow, but we'll skip validation for safety
		return nil

	case *types.ModelGroup:
		// cycle detection: skip if already visited
		if visited[p] {
			return nil
		}
		visited[p] = true

		// for choice and all groups, check that no two particles can match the same element
		// (all groups are unordered, so overlaps are always ambiguous)
		if p.Kind == types.Choice || p.Kind == types.AllGroup {
			for i, child1 := range p.Particles {
				for j, child2 := range p.Particles {
					if i >= j {
						continue // avoid duplicate checks
					}
					var err error
					if p.Kind == types.Choice {
						err = checkChoiceUPAViolation(schema, child1, child2, targetNS)
					} else {
						err = checkUPAViolationWithVisited(schema, child1, child2, targetNS, make(map[*types.ModelGroup]bool))
					}
					if err != nil {
						return fmt.Errorf("UPA violation in choice group: %w", err)
					}
				}
			}
		}

		// for sequence groups, check UPA violations considering occurrence constraints
		// UPA violations occur when two particles in a sequence can both match the same element
		// this happens when:
		// 1. Both particles can match the same element name (same QName)
		// 2. The first particle has maxOccurs > 1 (can repeat), OR both particles can be active simultaneously
		if p.Kind == types.Sequence {
			for i, child1 := range p.Particles {
				for j, child2 := range p.Particles {
					if i >= j {
						continue // avoid duplicate checks
					}
					// check if these two particles can both match the same element
					// in a sequence, if child1 has maxOccurs > 1, it can repeat and potentially
					// match the same element that child2 matches
					// pass the parent sequence's particles to check for separators
					if err := checkSequenceUPAViolationWithVisitedAndContext(schema, child1, child2, targetNS, make(map[*types.ModelGroup]bool), p.Particles, i, j); err != nil {
						return fmt.Errorf("UPA violation in sequence group: %w", err)
					}
				}
			}
		}

		// recursively validate particles within this group
		kind := p.Kind
		for _, child := range p.Particles {
			if err := validateUPAInParticleWithVisited(schema, child, nil, targetNS, &kind, visited); err != nil {
				return err
			}
		}

		// for extensions, check extension particles against base particles
		if baseParticle != nil {
			if err := validateExtensionUPA(schema, p, baseParticle, targetNS); err != nil {
				return err
			}
		}

	case *types.ElementDecl:
		// leaf element - no UPA violations at this level
		// but if we're in a choice with base particles, check those
		if baseParticle != nil && parentKind != nil && *parentKind == types.Choice {
			if err := validateExtensionUPA(schema, p, baseParticle, targetNS); err != nil {
				return err
			}
		}

	case *types.AnyElement:
		// leaf wildcard - no UPA violations at this level
		// but if we're in a choice with base particles, check those
		if baseParticle != nil && parentKind != nil && *parentKind == types.Choice {
			if err := validateExtensionUPA(schema, p, baseParticle, targetNS); err != nil {
				return err
			}
		}
	}

	return nil
}
