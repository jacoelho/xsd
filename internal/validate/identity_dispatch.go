package validate

import (
	"cmp"
	"slices"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
)

type identitySelectionCandidate struct {
	selection  int
	constraint xsdSchema.IdentityConstraintID
}

// Scratch is rebuilt from pending selections for each value. It never becomes
// another writer of selection membership or participates in start rollback.
type identityFieldDispatchScratch struct {
	selections []identitySelectionCandidate
	matched    []uint64 // Zero between collections; unique outputs own cleanup.
}

func (s *identityFieldDispatchScratch) prepare(selections []identitySelection) {
	s.selections = slices.Grow(s.selections[:0], len(selections))[:len(selections)]
	ordered := true
	for index := range selections {
		constraint := selections[index].constraint
		s.selections[index] = identitySelectionCandidate{selection: index, constraint: constraint}
		if index != 0 && selections[index-1].constraint > constraint {
			ordered = false
		}
	}
	if !ordered {
		slices.SortFunc(s.selections, func(a, b identitySelectionCandidate) int {
			if order := cmp.Compare(a.constraint, b.constraint); order != 0 {
				return order
			}
			return cmp.Compare(a.selection, b.selection)
		})
	}
}

func (s *identityFieldDispatchScratch) prepareMatches(fields int) {
	words := fields / 64
	if fields%64 != 0 {
		words++
	}
	s.matched = slices.Grow(s.matched[:0], words)[:words]
}

func (s *identityFieldDispatchScratch) forConstraint(constraint xsdSchema.IdentityConstraintID) []identitySelectionCandidate {
	start, _ := slices.BinarySearchFunc(s.selections, constraint, func(candidate identitySelectionCandidate, target xsdSchema.IdentityConstraintID) int {
		return cmp.Compare(candidate.constraint, target)
	})
	end := start
	for end < len(s.selections) && s.selections[end].constraint == constraint {
		end++
	}
	return s.selections[start:end]
}

func (e *identityEvaluation) appendIdentityFieldMatch(matches []identityFieldMatch, selection, field int) []identityFieldMatch {
	if len(matches) == 0 {
		e.fieldDispatch.prepareMatches(len(e.fieldValues))
	}
	index := e.selections[selection].fieldStart + field
	word, mask := index/64, uint64(1)<<(index%64)
	if e.fieldDispatch.matched[word]&mask != 0 {
		return matches
	}
	e.fieldDispatch.matched[word] |= mask
	return append(matches, identityFieldMatch{Selection: selection, Field: field})
}

// Every marked slot has one unique output match. Clearing those words after
// collection avoids work proportional to all pending fields on every value.
func (e *identityEvaluation) clearIdentityFieldMatches(matches []identityFieldMatch) {
	for _, match := range matches {
		index := e.selections[match.Selection].fieldStart + match.Field
		e.fieldDispatch.matched[index/64] = 0
	}
}

func (s *identityFieldDispatchScratch) reset(maxRetainedSlices int) {
	s.selections = resetRetainedValues(s.selections, maxRetainedSlices)
	// Reset also runs while unwinding a panic that interrupted collection.
	clear(s.matched)
	s.matched = resetRetainedValues(s.matched, (maxRetainedSlices+63)/64)
}
