package schemacheck

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
				if baseCT, ok := lookupComplexType(schema, c.Extension.Base); ok {
					if baseEC, ok := baseCT.Content().(*types.ElementContent); ok {
						baseParticle = baseEC.Particle
					}
				}
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			particle = c.Restriction.Particle
		}
	}

	if particle != nil {
		expanded, err := expandGroupRefs(schema, particle, make(map[types.QName]bool))
		if err != nil {
			return err
		}
		particle = expanded
	}
	if baseParticle != nil {
		expanded, err := expandGroupRefs(schema, baseParticle, make(map[types.QName]bool))
		if err != nil {
			return err
		}
		baseParticle = expanded
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
func validateUPAInParticle(schema *parser.Schema, particle, baseParticle types.Particle, targetNS types.NamespaceURI, parentKind *types.GroupKind) error {
	visited := newModelGroupVisit()
	return validateUPAInParticleWithVisited(schema, particle, baseParticle, targetNS, parentKind, visited)
}

// validateUPAInParticleWithVisited validates UPA violations with cycle detection
func validateUPAInParticleWithVisited(schema *parser.Schema, particle, baseParticle types.Particle, targetNS types.NamespaceURI, parentKind *types.GroupKind, visited modelGroupVisit) error {
	switch p := particle.(type) {
	case *types.GroupRef:
		expanded, err := expandGroupRefs(schema, p, make(map[types.QName]bool))
		if err != nil {
			return err
		}
		return validateUPAInParticleWithVisited(schema, expanded, baseParticle, targetNS, parentKind, visited)

	case *types.ModelGroup:
		// cycle detection: skip if already visited
		if !visited.enter(p) {
			return nil
		}

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
						err = checkUPAViolationWithVisited(schema, child1, child2, targetNS, newModelGroupVisit())
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
					if err := checkSequenceUPAViolationWithVisitedAndContext(schema, child1, child2, targetNS, newModelGroupVisit(), p.Particles, i, j); err != nil {
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

func expandGroupRefs(schema *parser.Schema, particle types.Particle, stack map[types.QName]bool) (types.Particle, error) {
	switch p := particle.(type) {
	case *types.GroupRef:
		if stack[p.RefQName] {
			return nil, fmt.Errorf("circular group reference detected for %s", p.RefQName)
		}
		groupDef, exists := schema.Groups[p.RefQName]
		if !exists {
			return nil, fmt.Errorf("group '%s' not found", p.RefQName)
		}
		stack[p.RefQName] = true
		defer delete(stack, p.RefQName)

		groupCopy := &types.ModelGroup{
			Kind:      groupDef.Kind,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
			Particles: make([]types.Particle, 0, len(groupDef.Particles)),
		}
		for _, child := range groupDef.Particles {
			expanded, err := expandGroupRefs(schema, child, stack)
			if err != nil {
				return nil, err
			}
			groupCopy.Particles = append(groupCopy.Particles, expanded)
		}
		return groupCopy, nil
	case *types.ModelGroup:
		groupCopy := &types.ModelGroup{
			Kind:      p.Kind,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
			Particles: make([]types.Particle, 0, len(p.Particles)),
		}
		for _, child := range p.Particles {
			expanded, err := expandGroupRefs(schema, child, stack)
			if err != nil {
				return nil, err
			}
			groupCopy.Particles = append(groupCopy.Particles, expanded)
		}
		return groupCopy, nil
	default:
		return particle, nil
	}
}

// checkChoiceUPAViolation checks for UPA violations in choice groups by comparing first sets.
func checkChoiceUPAViolation(schema *parser.Schema, p1, p2 types.Particle, targetNS types.NamespaceURI) error {
	first1 := collectPossibleFirstLeafParticles(p1, newModelGroupVisit())
	first2 := collectPossibleFirstLeafParticles(p2, newModelGroupVisit())

	for _, f1 := range first1 {
		for _, f2 := range first2 {
			if err := checkUPAViolationWithVisited(schema, f1, f2, targetNS, newModelGroupVisit()); err != nil {
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
func checkSequenceUPAViolationWithVisitedAndContext(schema *parser.Schema, p1, p2 types.Particle, targetNS types.NamespaceURI, visited modelGroupVisit, parentParticles []types.Particle, i, j int) error {
	// check if p1 and p2 are separated by required particles in the parent sequence
	// a required separator creates a deterministic transition point, eliminating ambiguity
	// between elements in p1 and p2, regardless of whether they overlap.
	// per XSD 1.0 spec, UPA is about whether an incoming element can be uniquely
	// attributed to a particle - a required separator provides this uniqueness.
	if separator := findRequiredSeparator(parentParticles, i, j); separator != nil {
		if sequenceSeparatorDisambiguates(schema, p1, separator, targetNS) {
			return nil
		}
	}

	if err := checkSequenceDuplicateElementName(p1, p2); err != nil {
		return err
	}

	// if p1 is optional, it can be skipped, so p2 may match the same element.
	if p1.MinOcc().IsZero() {
		if err := checkUPAViolationWithVisited(schema, p1, p2, targetNS, visited); err != nil {
			return err
		}
	}

	// if p1 can repeat (maxOccurs > 1 or unbounded), it can match the same element as p2.
	// this handles the case where p1 repeats and overlaps with p2
	maxOcc := p1.MaxOcc()
	if maxOcc.IsUnbounded() || maxOcc.CmpInt(1) > 0 {
		// if p1 has a fixed, bounded occurrence count, the boundary is deterministic.
		if !maxOcc.IsUnbounded() && p1.MinOcc().Equal(maxOcc) {
			if elem1, ok1 := p1.(*types.ElementDecl); ok1 {
				if elem2, ok2 := p2.(*types.ElementDecl); ok2 && elem1.Name == elem2.Name {
					return nil
				}
			}
		}
		// check if nested groups can overlap
		mg1, isMG1 := p1.(*types.ModelGroup)
		mg2, isMG2 := p2.(*types.ModelGroup)
		if isMG1 && isMG2 {
			// check if particles within these groups can overlap
			if err := checkModelGroupUPAWithVisited(schema, mg1, mg2, targetNS, visited); err != nil {
				return err
			}
		}
		// also check direct particle overlap
		return checkUPAViolationWithVisited(schema, p1, p2, targetNS, visited)
	}

	// if p1 can't repeat, check if both are nested groups that can overlap
	// this handles cases like particlesZ037 where both nested sequences contain the same element
	mg1, isMG1 := p1.(*types.ModelGroup)
	mg2, isMG2 := p2.(*types.ModelGroup)
	if isMG1 && isMG2 {
		// for nested sequences, check if they can both match the same elements
		// this is a UPA violation if both sequences can be active and contain overlapping particles
		if mg1.Kind == types.Sequence && mg2.Kind == types.Sequence {
			// check if particles within these sequences can overlap
			// this handles the case where both sequences contain the same element name
			// note: This may have false positives for sequences separated by required particles,
			// but we can't easily check that without context about the parent sequence
			return checkModelGroupUPAWithVisited(schema, mg1, mg2, targetNS, visited)
		}
	}

	if err := checkSequenceTailRepeats(schema, p1, p2, targetNS); err != nil {
		return err
	}

	return nil
}

func findRequiredSeparator(parentParticles []types.Particle, i, j int) types.Particle {
	if parentParticles == nil || i < 0 || j < 0 || i >= j {
		return nil
	}
	for k := i + 1; k < j && k < len(parentParticles); k++ {
		if parentParticles[k].MinOcc().CmpInt(0) > 0 {
			return parentParticles[k]
		}
	}
	return nil
}

func sequenceSeparatorDisambiguates(schema *parser.Schema, p1, separator types.Particle, targetNS types.NamespaceURI) bool {
	p1MaxOcc := p1.MaxOcc()
	if !p1MaxOcc.IsUnbounded() && p1MaxOcc.CmpInt(1) <= 0 {
		return true
	}
	lastParticles := collectPossibleLastLeafParticles(p1, newModelGroupVisit())
	for _, last := range lastParticles {
		if err := checkUPAViolationWithVisited(schema, last, separator, targetNS, newModelGroupVisit()); err != nil {
			return false
		}
	}
	return true
}

func checkSequenceDuplicateElementName(p1, p2 types.Particle) error {
	elem1, ok1 := p1.(*types.ElementDecl)
	if !ok1 {
		return nil
	}
	elem2, ok2 := p2.(*types.ElementDecl)
	if !ok2 || elem1.Name != elem2.Name {
		return nil
	}
	maxOcc := p1.MaxOcc()
	fixed := !maxOcc.IsUnbounded() && p1.MinOcc().Equal(maxOcc)
	if p1.MinOcc().IsZero() || ((maxOcc.IsUnbounded() || maxOcc.CmpInt(1) > 0) && !fixed) {
		return fmt.Errorf("duplicate element name '%s'", elem1.Name)
	}
	return nil
}

func checkSequenceTailRepeats(schema *parser.Schema, p1, p2 types.Particle, targetNS types.NamespaceURI) error {
	// if p1 itself can't repeat, but its possible last particles can repeat, they can
	// still overlap with p2 and cause ambiguity in the sequence.
	if p1.MaxOcc().IsUnbounded() || p1.MaxOcc().CmpInt(1) > 0 {
		return nil
	}
	lastParticles := collectPossibleLastLeafParticles(p1, newModelGroupVisit())
	for _, last := range lastParticles {
		maxOcc := last.MaxOcc()
		if maxOcc.IsUnbounded() || maxOcc.CmpInt(1) > 0 {
			if err := checkUPAViolationWithVisited(schema, last, p2, targetNS, newModelGroupVisit()); err != nil {
				return fmt.Errorf("overlapping repeating particle at sequence end: %w", err)
			}
		}
	}
	return nil
}

// checkUPAViolationWithVisited checks UPA violations with cycle detection
func checkUPAViolationWithVisited(schema *parser.Schema, p1, p2 types.Particle, targetNS types.NamespaceURI, visited modelGroupVisit) error {
	// particles with maxOccurs=0 are effectively absent and can't cause UPA violations
	if p1.MaxOcc().IsZero() || p2.MaxOcc().IsZero() {
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

	// check if both are model groups that could match the same element
	mg1, isMG1 := p1.(*types.ModelGroup)
	mg2, isMG2 := p2.(*types.ModelGroup)
	if isMG1 && isMG2 {
		// check if particles within these groups can overlap
		// this is a simplified check - full UPA would require checking all combinations
		if err := checkModelGroupUPAWithVisited(schema, mg1, mg2, targetNS, visited); err != nil {
			return err
		}
	}
	if isMG1 && !isMG2 {
		// for sequences, only check particles that are in "first" position (not preceded by required elements).
		// particles preceded by required elements are disambiguated by those predecessors.
		if mg1.Kind == types.Sequence {
			hasSeenRequired := false
			for _, child := range mg1.Particles {
				if !hasSeenRequired {
					if err := checkUPAViolationWithVisited(schema, child, p2, targetNS, visited); err != nil {
						return err
					}
				}
				// once we see a required particle, subsequent particles are disambiguated
				if child.MinOcc().CmpInt(0) > 0 {
					hasSeenRequired = true
				}
			}
		} else {
			// for choice groups, check all children
			for _, child := range mg1.Particles {
				if err := checkUPAViolationWithVisited(schema, child, p2, targetNS, visited); err != nil {
					return err
				}
			}
		}
	}
	if isMG2 && !isMG1 {
		// for sequences, only check particles that are in "first" position
		if mg2.Kind == types.Sequence {
			hasSeenRequired := false
			for _, child := range mg2.Particles {
				if !hasSeenRequired {
					if err := checkUPAViolationWithVisited(schema, p1, child, targetNS, visited); err != nil {
						return err
					}
				}
				if child.MinOcc().CmpInt(0) > 0 {
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
func checkModelGroupUPAWithVisited(schema *parser.Schema, mg1, mg2 *types.ModelGroup, targetNS types.NamespaceURI, visited modelGroupVisit) error {
	// cycle detection
	if !visited.enter(mg1) || !visited.enter(mg2) {
		return nil
	}

	// if both are choice groups, check if any particle in mg1 overlaps with any particle in mg2
	if mg1.Kind == types.Choice && mg2.Kind == types.Choice {
		for _, p1 := range mg1.Particles {
			for _, p2 := range mg2.Particles {
				if err := checkUPAViolationWithVisited(schema, p1, p2, targetNS, visited); err != nil {
					return err
				}
			}
		}
	}

	// for sequences, UPA violations can only occur at boundary positions.
	// per XSD 1.0 spec, UPA is about whether an incoming element can be uniquely
	// attributed to a particle. For sequences:
	// - Only the LAST particles of mg1 could potentially overlap with FIRST particles of mg2
	// - If overlapping particles are preceded by required particles in their respective
	//   sequences, the required particles act as disambiguation markers
	//
	// for example: <seq1>[e1, e2]</seq1>, <seq2>[e2, e3]</seq2>
	// the e2 in seq1 is preceded by required e1, so when you see e2 after e1, you know
	// it's from seq1. An e2 without preceding e1 (after seq1 completes) must be from seq2.
	// this is NOT a UPA violation.
	if mg1.Kind == types.Sequence && mg2.Kind == types.Sequence {
		if len(mg1.Particles) == 0 || len(mg2.Particles) == 0 {
			return nil
		}
		lastParticles := collectPossibleLastLeafParticles(mg1, newModelGroupVisit())
		firstParticles := collectPossibleFirstLeafParticles(mg2, newModelGroupVisit())

		for _, last := range lastParticles {
			repeatsOrOptional := last.MinOcc().IsZero() || last.MaxOcc().IsUnbounded() || last.MaxOcc().CmpInt(1) > 0
			if !repeatsOrOptional {
				continue
			}
			if !last.MaxOcc().IsUnbounded() && last.MinOcc().Equal(last.MaxOcc()) {
				continue
			}
			for _, first := range firstParticles {
				if err := checkUPAViolationWithVisited(schema, last, first, targetNS, newModelGroupVisit()); err != nil {
					return err
				}
			}
		}
	}

	// for mixed group kinds (sequence vs choice), check overlaps if the sequence can repeat
	if (mg1.Kind == types.Sequence && mg2.Kind == types.Choice) || (mg1.Kind == types.Choice && mg2.Kind == types.Sequence) {
		seqMG := mg1
		choiceMG := mg2
		if mg2.Kind == types.Sequence {
			seqMG = mg2
			choiceMG = mg1
		}
		// if the sequence can repeat, check for overlaps
		if seqMG.MaxOcc().CmpInt(1) > 0 {
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
func validateExtensionUPA(schema *parser.Schema, extParticle, baseParticle types.Particle, targetNS types.NamespaceURI) error {
	if baseParticle == nil || extParticle == nil {
		return nil
	}
	parent := []types.Particle{baseParticle, extParticle}
	if err := checkSequenceUPAViolationWithVisitedAndContext(schema, baseParticle, extParticle, targetNS, newModelGroupVisit(), parent, 0, 1); err != nil {
		return fmt.Errorf("extension content model is not deterministic: %w", err)
	}
	return nil
}

// wildcardsOverlap checks if two wildcards have overlapping namespace constraints
func wildcardsOverlap(w1, w2 *types.AnyElement) bool {
	// two wildcards overlap if there's at least one namespace that matches both
	// this is a simplified check - for exact UPA validation, we'd need to check
	// if they're in a choice group and can both match the same element

	// if either wildcard is ##any, they overlap (##any matches everything)
	if w1.Namespace == types.NSCAny || w2.Namespace == types.NSCAny {
		return true
	}
	// two ##other wildcards always overlap (intersection excludes both target namespaces).
	if w1.Namespace == types.NSCOther && w2.Namespace == types.NSCOther {
		return true
	}

	// check if the intersection of the two wildcards is non-empty
	// if intersection exists, they overlap
	intersected := types.IntersectAnyElement(w1, w2)
	return intersected != nil
}

// collectPossibleLastLeafParticles collects particles that could be the last leaf particles
// in a particle structure (used for UPA validation in sequences)
func collectPossibleLastLeafParticles(particle types.Particle, visited modelGroupVisit) []types.Particle {
	switch p := particle.(type) {
	case *types.ElementDecl, *types.AnyElement:
		return []types.Particle{p}
	case *types.ModelGroup:
		if !visited.enter(p) {
			return nil
		}
		var out []types.Particle
		switch p.Kind {
		case types.Sequence:
			for i := len(p.Particles) - 1; i >= 0; i-- {
				child := p.Particles[i]
				out = append(out, collectPossibleLastLeafParticles(child, visited)...)
				if child.MinOcc().CmpInt(0) > 0 {
					break
				}
			}
		case types.Choice, types.AllGroup:
			for _, child := range p.Particles {
				out = append(out, collectPossibleLastLeafParticles(child, visited)...)
			}
		}
		return out
	}
	return nil
}

// collectPossibleFirstLeafParticles collects particles that could be the first leaf particles
// in a particle structure (used for UPA validation in sequences).
func collectPossibleFirstLeafParticles(particle types.Particle, visited modelGroupVisit) []types.Particle {
	switch p := particle.(type) {
	case *types.ElementDecl, *types.AnyElement:
		return []types.Particle{p}
	case *types.ModelGroup:
		if !visited.enter(p) {
			return nil
		}
		var out []types.Particle
		switch p.Kind {
		case types.Sequence:
			for _, child := range p.Particles {
				out = append(out, collectPossibleFirstLeafParticles(child, visited)...)
				if child.MinOcc().CmpInt(0) > 0 {
					break
				}
			}
		case types.Choice, types.AllGroup:
			for _, child := range p.Particles {
				out = append(out, collectPossibleFirstLeafParticles(child, visited)...)
			}
		}
		return out
	}
	return nil
}

// wildcardOverlapsElement checks if a wildcard's namespace constraint overlaps with an explicit element's namespace
func wildcardOverlapsElement(wildcard *types.AnyElement, elemDecl *types.ElementDecl) bool {
	elemNS := elemDecl.Name.Namespace

	// check if element's namespace matches wildcard's namespace constraint
	return namespaceMatchesWildcard(elemNS, wildcard.Namespace, wildcard.NamespaceList, wildcard.TargetNamespace)
}

// namespaceMatchesWildcard checks if a namespace matches a wildcard namespace constraint
// This is used for schema validation (UPA checking), not instance validation
func namespaceMatchesWildcard(ns types.NamespaceURI, constraint types.NamespaceConstraint, namespaceList []types.NamespaceURI, targetNS types.NamespaceURI) bool {
	return types.AllowsNamespace(constraint, namespaceList, targetNS, ns)
}
