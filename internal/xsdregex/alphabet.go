package xsdregex

import (
	"math/bits"
	"slices"
	"unicode/utf8"
)

// alphabet partitions the code points into classes: two code points share a
// class exactly when every character set of one pattern contains both or
// neither. A matcher that looks a rune's class up once needs no set test.
type alphabet struct {
	// runs holds one entry per maximal run of one class above ASCII, as
	// first<<8|class in ascending order; runs[0] starts at utf8.RuneSelf.
	runs []uint32
	// pages holds, per page of 1<<pageShift code points below pagedRunes, the
	// index of the run holding the page's first code point, and last the
	// index of the run holding pagedRunes. It is empty when runs is short
	// enough to search whole.
	pages []uint16
	ascii [utf8.RuneSelf]uint8
}

const (
	// maxClasses is the number of classes a class id can name.
	maxClasses = 1 << 8
	pageShift  = 6
	pagedRunes = 1 << 16
	// pagedRunsFrom is the number of runs from which an alphabet has pages. A
	// shorter list is searched whole in at most six probes, without retaining
	// pages.
	pagedRunsFrom = 65
)

// class returns the class of a code point at or above utf8.RuneSelf.
func (a *alphabet) class(r rune) uint8 {
	runs := a.runs
	lo, hi := 0, len(runs)
	if pages := a.pages; len(pages) != 0 {
		if page := int(r >> pageShift); page < len(pages)-1 {
			lo, hi = int(pages[page]), int(pages[page+1])+1
		} else {
			lo = int(pages[len(pages)-1])
		}
	}
	key := uint32(r)<<8 | (maxClasses - 1) //nolint:gosec // r is a decoded rune, at most utf8.MaxRune.
	for hi-lo > 1 {
		mid := int(uint(lo+hi) >> 1)
		if runs[mid] <= key {
			lo = mid
		} else {
			hi = mid
		}
	}
	return uint8(runs[lo]) //nolint:gosec // The low byte is the class.
}

// index builds pages when runs is long enough to gain from them. It reports
// false when a run index does not fit a page entry.
func (a *alphabet) index() bool {
	if len(a.runs) < pagedRunsFrom {
		return true
	}
	if len(a.runs) > 1<<16 {
		return false
	}
	a.pages = make([]uint16, pagedRunes>>pageShift+1)
	run := 0
	for page := range a.pages {
		first := uint32(max(page<<pageShift, utf8.RuneSelf))
		for run+1 < len(a.runs) && a.runs[run+1]>>8 <= first {
			run++
		}
		a.pages[page] = uint16(run)
	}
	return true
}

// setKey identifies a character set by its backing array and length; sets
// that share both hold the same ranges.
type setKey struct {
	first *runeRange
	count int
}

// classes is the alphabet of a list of distinct sets, with each set's classes.
type classes struct {
	// bySet lists, for each set, the classes whose code points it contains.
	bySet    [][]uint8
	alphabet alphabet
	count    int
}

// buildBudget meters DFA construction in steps of bounded cost, so compiling a
// pattern costs at most a fixed amount whatever the DFA attempt finds.
type buildBudget struct {
	remaining uint64
}

func (b *buildBudget) spend(steps int) bool {
	if steps < 0 || uint64(steps) > b.remaining {
		return false
	}
	b.remaining -= uint64(steps)
	return true
}

// newClasses sweeps the range boundaries of sets in code-point order. It
// reports false when the budget runs out or the sets induce more than
// maxClasses classes.
func newClasses(sets []rangeSet, budget *buildBudget) (classes, bool) {
	changes, ok := membershipChanges(sets, budget)
	if !ok {
		return classes{}, false
	}
	sweep := classSweep{member: make([]uint64, len(sets)/64+1), bySet: make([][]uint8, len(sets))}
	from := rune(0)
	for from <= utf8.MaxRune {
		for len(changes) != 0 && rune(changes[0]>>32) == from { //nolint:gosec // The high half is a code point.
			set := uint32(changes[0]) //nolint:gosec // The low half is the set.
			sweep.member[set/64] ^= 1 << (set % 64)
			changes = changes[1:]
		}
		to := rune(utf8.MaxRune + 1)
		if len(changes) != 0 {
			to = rune(changes[0] >> 32) //nolint:gosec // The high half is a code point.
		}
		if !sweep.assign(from, to, budget) {
			return classes{}, false
		}
		from = to
	}
	if !sweep.alphabet.index() {
		return classes{}, false
	}
	return classes{alphabet: sweep.alphabet, bySet: sweep.bySet, count: len(sweep.signatures) / len(sweep.member)}, true
}

// membershipChanges returns, in code-point order, each code point at which
// membership in one set flips, as at<<32|set: a range's first code point and
// the one after its last.
func membershipChanges(sets []rangeSet, budget *buildBudget) ([]uint64, bool) {
	total := 0
	for _, set := range sets {
		total += 2 * len(set.ranges)
	}
	if !budget.spend(total * (1 + bits.Len(uint(total)))) {
		return nil, false
	}
	changes := make([]uint64, 0, total)
	for index, set := range sets {
		for _, current := range set.ranges {
			changes = append(changes, uint64(current.lo)<<32|uint64(index)) //nolint:gosec // Code points and slice indexes are non-negative.
			if current.hi < utf8.MaxRune {
				changes = append(changes, uint64(current.hi+1)<<32|uint64(index)) //nolint:gosec // Code points and slice indexes are non-negative.
			}
		}
	}
	slices.Sort(changes)
	return changes, true
}

type classSweep struct {
	// signatures holds the membership words of each class, class after class.
	signatures []uint64
	member     []uint64
	bySet      [][]uint8
	alphabet   alphabet
}

// assign gives the code points in [from, to) the class of the current
// membership.
func (s *classSweep) assign(from, to rune, budget *buildBudget) bool {
	class, ok := s.membershipClass(budget)
	if !ok {
		return false
	}
	for r := from; r < min(to, utf8.RuneSelf); r++ {
		s.alphabet.ascii[r] = class
	}
	if to <= utf8.RuneSelf {
		return true
	}
	runs := s.alphabet.runs
	if len(runs) != 0 && uint8(runs[len(runs)-1]) == class { //nolint:gosec // The low byte is the class.
		return true
	}
	s.alphabet.runs = append(s.alphabet.runs, uint32(max(from, utf8.RuneSelf))<<8|uint32(class))
	return true
}

func (s *classSweep) membershipClass(budget *buildBudget) (uint8, bool) {
	words := len(s.member)
	count := len(s.signatures) / words
	if !budget.spend(words * (count + 1)) {
		return 0, false
	}
	for class := range count {
		if slices.Equal(s.signatures[class*words:(class+1)*words], s.member) {
			return uint8(class), true
		}
	}
	if count == maxClasses || !budget.spend(64*words) {
		return 0, false
	}
	s.signatures = append(s.signatures, s.member...)
	for index, word := range s.member {
		for ; word != 0; word &= word - 1 {
			set := index*64 + bits.TrailingZeros64(word)
			s.bySet[set] = append(s.bySet[set], uint8(count)) //nolint:gosec // count is below maxClasses.
		}
	}
	return uint8(count), true //nolint:gosec // count is below maxClasses.
}
