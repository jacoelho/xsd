package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// checkChoiceUPAViolation checks for UPA violations in choice groups by comparing first sets.
func checkChoiceUPAViolation(schema *schema.Schema, p1, p2 types.Particle, targetNS types.NamespaceURI) error {
	first1 := collectPossibleFirstLeafParticles(p1, make(map[*types.ModelGroup]bool))
	first2 := collectPossibleFirstLeafParticles(p2, make(map[*types.ModelGroup]bool))

	for _, f1 := range first1 {
		for _, f2 := range first2 {
			if err := checkUPAViolationWithVisited(schema, f1, f2, targetNS, make(map[*types.ModelGroup]bool)); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkSequenceUPAViolationWithVisitedAndContext checks if two particles in a sequence can both match the same element
// In a sequence, UPA violations occur when:
// 1. Both particles can match the same element name
// 2. The first particle can repeat (maxOccurs > 1), allowing it to match the same element that the second particle matches
// OR both particles are in nested groups that can both be active and contain overlapping particles
// Note: In a sequence, particles are matched in order, so if p1 and p2 are separated by other particles
// that must be matched, there's no UPA violation. However, if p1 can repeat, it can potentially
// match elements that p2 should match, creating ambiguity.
// parentParticles, i, j provide context about the parent sequence to check for separators
func checkSequenceUPAViolationWithVisitedAndContext(schema *schema.Schema, p1, p2 types.Particle, targetNS types.NamespaceURI, visited map[*types.ModelGroup]bool, parentParticles []types.Particle, i, j int) error {
	// Check if p1 and p2 are separated by required particles in the parent sequence
	// A required separator creates a deterministic transition point, eliminating ambiguity
	// between elements in p1 and p2, regardless of whether they overlap.
	// Per XSD 1.0 spec, UPA is about whether an incoming element can be uniquely
	// attributed to a particle - a required separator provides this uniqueness.
	if parentParticles != nil && i >= 0 && j >= 0 && i < j {
		hasRequiredSeparator := false
		for k := i + 1; k < j; k++ {
			if k < len(parentParticles) {
				separator := parentParticles[k]
				if separator.MinOcc() > 0 {
					hasRequiredSeparator = true
					break
				}
			}
		}
		// If there's a required separator and p1 cannot repeat unboundedly to consume
		// elements past the separator, there's no UPA violation.
		// The separator creates a deterministic boundary: elements before the separator
		// belong to p1, the separator itself is matched, and elements after belong to p2.
		if hasRequiredSeparator {
			// Only check for overlap issues if p1 can repeat past the separator.
			// If p1 has maxOccurs=1 (or finite), it can't "steal" elements from p2.
			p1MaxOcc := p1.MaxOcc()
			if p1MaxOcc != types.UnboundedOccurs && p1MaxOcc <= 1 {
				// p1 cannot repeat - separator prevents any UPA violation
				return nil
			}
			// p1 can repeat - need to check if its last particles could overlap with the separator
			// or with p2's first particles, creating ambiguity
			// For now, be lenient: if there's a separator and p1 doesn't have unbounded
			// repeating elements that could consume the separator, we're safe
			lastParticles := collectPossibleLastLeafParticles(p1, make(map[*types.ModelGroup]bool))
			separatorSafe := true
			for k := i + 1; k < j && separatorSafe; k++ {
				if k < len(parentParticles) {
					sep := parentParticles[k]
					if sep.MinOcc() > 0 {
						// Check if any last particle of p1 could match the separator
						for _, last := range lastParticles {
							if err := checkUPAViolationWithVisited(schema, last, sep, targetNS, make(map[*types.ModelGroup]bool)); err != nil {
								// Last particle overlaps with separator - could be ambiguous
								separatorSafe = false
								break
							}
						}
						break
					}
				}
			}
			if separatorSafe {
				return nil
			}
		}
	}
	if elem1, ok1 := p1.(*types.ElementDecl); ok1 {
		if elem2, ok2 := p2.(*types.ElementDecl); ok2 && elem1.Name == elem2.Name {
			fixed := p1.MaxOcc() != types.UnboundedOccurs && p1.MinOcc() == p1.MaxOcc()
			if p1.MinOcc() == 0 || ((p1.MaxOcc() > 1 || p1.MaxOcc() == types.UnboundedOccurs) && !fixed) {
				return fmt.Errorf("duplicate element name '%s'", elem1.Name)
			}
		}
	}
	// If p1 is optional, it can be skipped, so p2 may match the same element.
	if p1.MinOcc() == 0 {
		if err := checkUPAViolationWithVisited(schema, p1, p2, targetNS, visited); err != nil {
			return err
		}
	}

	// If p1 can repeat (maxOccurs > 1 or unbounded), it can match the same element as p2.
	// This handles the case where p1 repeats and overlaps with p2
	maxOcc := p1.MaxOcc()
	if maxOcc > 1 || maxOcc == types.UnboundedOccurs {
		// If p1 has a fixed, bounded occurrence count, the boundary is deterministic.
		if maxOcc != types.UnboundedOccurs && p1.MinOcc() == maxOcc {
			if elem1, ok1 := p1.(*types.ElementDecl); ok1 {
				if elem2, ok2 := p2.(*types.ElementDecl); ok2 && elem1.Name == elem2.Name {
					return nil
				}
			}
		}
		// Check if nested groups can overlap
		mg1, isMG1 := p1.(*types.ModelGroup)
		mg2, isMG2 := p2.(*types.ModelGroup)
		if isMG1 && isMG2 {
			// Check if particles within these groups can overlap
			if err := checkModelGroupUPAWithVisited(schema, mg1, mg2, targetNS, visited); err != nil {
				return err
			}
		}
		// Also check direct particle overlap
		return checkUPAViolationWithVisited(schema, p1, p2, targetNS, visited)
	}

	// If p1 can't repeat, check if both are nested groups that can overlap
	// This handles cases like particlesZ037 where both nested sequences contain the same element
	mg1, isMG1 := p1.(*types.ModelGroup)
	mg2, isMG2 := p2.(*types.ModelGroup)
	if isMG1 && isMG2 {
		// For nested sequences, check if they can both match the same elements
		// This is a UPA violation if both sequences can be active and contain overlapping particles
		if mg1.Kind == types.Sequence && mg2.Kind == types.Sequence {
			// Check if particles within these sequences can overlap
			// This handles the case where both sequences contain the same element name
			// Note: This may have false positives for sequences separated by required particles,
			// but we can't easily check that without context about the parent sequence
			return checkModelGroupUPAWithVisited(schema, mg1, mg2, targetNS, visited)
		}
	}

	// If p1 itself can't repeat, but its possible last particles can repeat, they can
	// still overlap with p2 and cause ambiguity in the sequence.
	if p1.MaxOcc() <= 1 {
		lastParticles := collectPossibleLastLeafParticles(p1, make(map[*types.ModelGroup]bool))
		for _, last := range lastParticles {
			maxOcc := last.MaxOcc()
			if maxOcc > 1 || maxOcc == types.UnboundedOccurs {
				if err := checkUPAViolationWithVisited(schema, last, p2, targetNS, make(map[*types.ModelGroup]bool)); err != nil {
					return fmt.Errorf("overlapping repeating particle at sequence end: %w", err)
				}
			}
		}
	}

	return nil
}

// checkUPAViolationWithVisited checks UPA violations with cycle detection
func checkUPAViolationWithVisited(schema *schema.Schema, p1, p2 types.Particle, targetNS types.NamespaceURI, visited map[*types.ModelGroup]bool) error {
	// Particles with maxOccurs=0 are effectively absent and can't cause UPA violations
	if p1.MaxOcc() == 0 || p2.MaxOcc() == 0 {
		return nil
	}

	anyElem1, isWildcard1 := p1.(*types.AnyElement)
	elemDecl1, isElement1 := p1.(*types.ElementDecl)

	anyElem2, isWildcard2 := p2.(*types.AnyElement)
	elemDecl2, isElement2 := p2.(*types.ElementDecl)

	if isWildcard1 && isElement2 {
		if wildcardOverlapsElement(anyElem1, elemDecl2) {
			return fmt.Errorf("wildcard namespace constraint overlaps with explicit element '%s'", elemDecl2.Name)
		}
	}
	if isWildcard2 && isElement1 {
		if wildcardOverlapsElement(anyElem2, elemDecl1) {
			return fmt.Errorf("wildcard namespace constraint overlaps with explicit element '%s'", elemDecl1.Name)
		}
	}

	if isWildcard1 && isWildcard2 {
		if wildcardsOverlap(anyElem1, anyElem2) {
			return fmt.Errorf("overlapping wildcard namespace constraints")
		}
	}

	if isElement1 && isElement2 {
		if elemDecl1.Name == elemDecl2.Name {
			return fmt.Errorf("duplicate element name '%s'", elemDecl1.Name)
		}
		if schema != nil {
			if isSubstitutableElement(schema, elemDecl1.Name, elemDecl2.Name) ||
				isSubstitutableElement(schema, elemDecl2.Name, elemDecl1.Name) {
				return fmt.Errorf("elements '%s' and '%s' overlap via substitution groups", elemDecl1.Name, elemDecl2.Name)
			}
		}
	}

	// Check if both are model groups that could match the same element
	mg1, isMG1 := p1.(*types.ModelGroup)
	mg2, isMG2 := p2.(*types.ModelGroup)
	if isMG1 && isMG2 {
		// Check if particles within these groups can overlap
		// This is a simplified check - full UPA would require checking all combinations
		if err := checkModelGroupUPAWithVisited(schema, mg1, mg2, targetNS, visited); err != nil {
			return err
		}
	}
	if isMG1 && !isMG2 {
		// For sequences, only check particles that are in "first" position (not preceded by required elements).
		// Particles preceded by required elements are disambiguated by those predecessors.
		if mg1.Kind == types.Sequence {
			hasSeenRequired := false
			for _, child := range mg1.Particles {
				if !hasSeenRequired {
					if err := checkUPAViolationWithVisited(schema, child, p2, targetNS, visited); err != nil {
						return err
					}
				}
				// Once we see a required particle, subsequent particles are disambiguated
				if child.MinOcc() > 0 {
					hasSeenRequired = true
				}
			}
		} else {
			// For choice groups, check all children
			for _, child := range mg1.Particles {
				if err := checkUPAViolationWithVisited(schema, child, p2, targetNS, visited); err != nil {
					return err
				}
			}
		}
	}
	if isMG2 && !isMG1 {
		// For sequences, only check particles that are in "first" position
		if mg2.Kind == types.Sequence {
			hasSeenRequired := false
			for _, child := range mg2.Particles {
				if !hasSeenRequired {
					if err := checkUPAViolationWithVisited(schema, p1, child, targetNS, visited); err != nil {
						return err
					}
				}
				if child.MinOcc() > 0 {
					hasSeenRequired = true
				}
			}
		} else {
			for _, child := range mg2.Particles {
				if err := checkUPAViolationWithVisited(schema, p1, child, targetNS, visited); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// checkModelGroupUPA checks if two model groups can both match the same element
// checkModelGroupUPAWithVisited checks model group UPA with cycle detection
func checkModelGroupUPAWithVisited(schema *schema.Schema, mg1, mg2 *types.ModelGroup, targetNS types.NamespaceURI, visited map[*types.ModelGroup]bool) error {
	// Cycle detection
	if visited[mg1] || visited[mg2] {
		return nil
	}
	visited[mg1] = true
	visited[mg2] = true

	// If both are choice groups, check if any particle in mg1 overlaps with any particle in mg2
	if mg1.Kind == types.Choice && mg2.Kind == types.Choice {
		for _, p1 := range mg1.Particles {
			for _, p2 := range mg2.Particles {
				if err := checkUPAViolationWithVisited(schema, p1, p2, targetNS, visited); err != nil {
					return err
				}
			}
		}
	}

	// For sequences, UPA violations can only occur at boundary positions.
	// Per XSD 1.0 spec, UPA is about whether an incoming element can be uniquely
	// attributed to a particle. For sequences:
	// - Only the LAST particles of mg1 could potentially overlap with FIRST particles of mg2
	// - If overlapping particles are preceded by required particles in their respective
	//   sequences, the required particles act as disambiguation markers
	//
	// For example: <seq1>[e1, e2]</seq1>, <seq2>[e2, e3]</seq2>
	// The e2 in seq1 is preceded by required e1, so when you see e2 after e1, you know
	// it's from seq1. An e2 without preceding e1 (after seq1 completes) must be from seq2.
	// This is NOT a UPA violation.
	if mg1.Kind == types.Sequence && mg2.Kind == types.Sequence {
		if len(mg1.Particles) == 0 || len(mg2.Particles) == 0 {
			return nil
		}
		lastParticles := collectPossibleLastLeafParticles(mg1, make(map[*types.ModelGroup]bool))
		firstParticles := collectPossibleFirstLeafParticles(mg2, make(map[*types.ModelGroup]bool))

		for _, last := range lastParticles {
			repeatsOrOptional := last.MinOcc() == 0 || last.MaxOcc() > 1 || last.MaxOcc() == types.UnboundedOccurs
			if !repeatsOrOptional {
				continue
			}
			if last.MaxOcc() != types.UnboundedOccurs && last.MinOcc() == last.MaxOcc() {
				continue
			}
			for _, first := range firstParticles {
				if err := checkUPAViolationWithVisited(schema, last, first, targetNS, make(map[*types.ModelGroup]bool)); err != nil {
					return err
				}
			}
		}
	}

	// For mixed group kinds (sequence vs choice), check overlaps if the sequence can repeat
	if (mg1.Kind == types.Sequence && mg2.Kind == types.Choice) || (mg1.Kind == types.Choice && mg2.Kind == types.Sequence) {
		seqMG := mg1
		choiceMG := mg2
		if mg2.Kind == types.Sequence {
			seqMG = mg2
			choiceMG = mg1
		}
		// If the sequence can repeat, check for overlaps
		if seqMG.MaxOcc() > 1 {
			for _, p1 := range seqMG.Particles {
				for _, p2 := range choiceMG.Particles {
					if err := checkUPAViolationWithVisited(schema, p1, p2, targetNS, visited); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// validateExtensionUPA checks UPA violations between extension particles and base particles
func validateExtensionUPA(schema *schema.Schema, extParticle types.Particle, baseParticle types.Particle, targetNS types.NamespaceURI) error {
	if baseParticle == nil || extParticle == nil {
		return nil
	}
	parent := []types.Particle{baseParticle, extParticle}
	if err := checkSequenceUPAViolationWithVisitedAndContext(schema, baseParticle, extParticle, targetNS, make(map[*types.ModelGroup]bool), parent, 0, 1); err != nil {
		return fmt.Errorf("extension content model is not deterministic: %w", err)
	}
	return nil
}
