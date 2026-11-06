package xsdregex

import (
	"bytes"
	"math"
	"strings"
	"unicode/utf8"
)

// linearPattern is the old bounded run-length matcher expressed directly
// from the shared AST. It covers concatenations of character sets and their
// repeats; all other patterns use the Thompson machine.
//
// literal takes priority over fixed. A fixed pattern (every atom has equal
// minimum and maximum counts) is matched by one cursor over the atom sets;
// repetition counts never expand into per-position instructions.
type linearPattern struct {
	literalText  string
	atoms        []linearAtom
	literalBytes []byte
	literal      bool
	fixed        bool
}

type linearAtom struct {
	set       rangeSet
	min       uint64
	max       uint64
	unbounded bool
}

type linearRun struct {
	start     int
	end       int
	minRepeat int
	maxRepeat int
	bounded   bool
}

const (
	linearStackRunes  = 128
	linearStackStates = 2 * (linearStackRunes + 1)
)

func compileLinear(root *node, options compileLimits) (*linearPattern, bool) {
	collector := linearCollector{pattern: &linearPattern{}}
	if !collector.visit(root) || !linearAtomsWithinLimit(collector.pattern.atoms, options.states) {
		return nil, false
	}
	linear := collector.pattern
	linear.fixed = linearFixed(linear.atoms)
	if literal, ok := linearLiteral(linear.atoms); ok {
		linear.literal = true
		linear.literalText = literal
		linear.literalBytes = []byte(literal)
	}
	return linear, true
}

func linearFixed(atoms []linearAtom) bool {
	for _, atom := range atoms {
		if atom.unbounded || atom.min != atom.max {
			return false
		}
	}
	return true
}

type linearCollector struct {
	pattern *linearPattern
}

func (c *linearCollector) visit(current *node) bool {
	if current == nil {
		return false
	}
	switch current.kind {
	case nodeEmpty:
		return true
	case nodeSet:
		c.pattern.atoms = append(c.pattern.atoms, linearAtom{set: current.set, min: 1, max: 1})
		return true
	case nodeConcat:
		return c.visitConcat(current.children)
	case nodeAlt:
		return false
	case nodeRepeat:
		return c.visitRepeat(current)
	}
	return false
}

func (c *linearCollector) visitConcat(children []*node) bool {
	for _, child := range children {
		if !c.visit(child) {
			return false
		}
	}
	return true
}

func (c *linearCollector) visitRepeat(current *node) bool {
	if len(current.children) != 1 || current.children[0] == nil || current.children[0].kind != nodeSet {
		return false
	}
	c.pattern.atoms = append(c.pattern.atoms, linearAtom{
		set:       current.children[0].set,
		min:       current.min,
		max:       current.max,
		unbounded: current.unbounded,
	})
	return true
}

func linearAtomsWithinLimit(atoms []linearAtom, maxStates uint64) bool {
	for _, atom := range atoms {
		if atom.unbounded && atom.min > maxStates {
			return false
		}
		if !atom.unbounded && atom.max > maxStates {
			return false
		}
	}
	return true
}

func linearLiteral(atoms []linearAtom) (string, bool) {
	var literal strings.Builder
	for _, atom := range atoms {
		if atom.min != atom.max || atom.unbounded || atom.min > 1 || len(atom.set.ranges) != 1 || atom.set.ranges[0].lo != atom.set.ranges[0].hi {
			return "", false
		}
		literal.WriteRune(atom.set.ranges[0].lo)
	}
	return literal.String(), true
}

// matchString and matchBytes run with budget already charged for the input
// bytes; they charge the per-row work of the dynamic program on top of it.
func (p *linearPattern) matchString(input string, budget *matchBudget, scratch *Scratch) (bool, error) {
	if p.literal {
		return input == p.literalText, nil
	}
	if len(input) <= linearStackRunes || scratch == nil {
		return p.matchLinearNoScratch(linearInput{stringInput: input}, budget)
	}
	runes := decodeLinearString(input, &scratch.linearRunes)
	states := prepareLinearStates(len(runes), scratch.linearStates)
	scratch.linearStates = states
	return p.matchPreparedRunes(runes, budget, states)
}

func (p *linearPattern) matchBytes(input []byte, budget *matchBudget, scratch *Scratch) (bool, error) {
	if p.literal {
		return bytes.Equal(input, p.literalBytes), nil
	}
	if len(input) <= linearStackRunes || scratch == nil {
		return p.matchLinearNoScratch(linearInput{bytesInput: input, isBytes: true}, budget)
	}
	runes := decodeLinearBytes(input, &scratch.linearRunes)
	states := prepareLinearStates(len(runes), scratch.linearStates)
	scratch.linearStates = states
	return p.matchPreparedRunes(runes, budget, states)
}

// linearInput is the runtime-tagged string/byte carrier for the linear
// matcher's no-scratch path. Rejected: extending it to all regex loops,
// because matched scalar and retained-scratch workloads exceeded the allowed
// 3% median CPU loss; representation-specific rune loops keep compiler
// specialization.
type linearInput struct {
	stringInput string
	bytesInput  []byte
	isBytes     bool
}

func (p *linearPattern) matchLinearNoScratch(input linearInput, budget *matchBudget) (bool, error) {
	var runes [linearStackRunes]rune
	if input.length() > cap(runes) {
		if runeCount := countLinearInput(input); runeCount > cap(runes) {
			decoded := make([]rune, runeCount)
			decodeLinearInput(input, decoded)
			return p.matchRunes(decoded, budget, nil)
		}
	}
	decodedCount := decodeLinearInput(input, runes[:])
	var states [linearStackStates]bool
	return p.matchRunes(runes[:decodedCount], budget, states[:0])
}

func (input linearInput) length() int {
	if input.isBytes {
		return len(input.bytesInput)
	}
	return len(input.stringInput)
}

func countLinearInput(input linearInput) int {
	count := 0
	for offset := 0; offset < input.length(); {
		_, size := input.runeAt(offset)
		count++
		offset += size
	}
	return count
}

func (input linearInput) runeAt(offset int) (rune, int) {
	if input.isBytes {
		return utf8.DecodeRune(input.bytesInput[offset:])
	}
	return utf8.DecodeRuneInString(input.stringInput[offset:])
}

// decodeLinearInput fills dst, which must hold every rune of input.
func decodeLinearInput(input linearInput, dst []rune) int {
	count := 0
	for offset := 0; offset < input.length(); {
		value, size := input.runeAt(offset)
		dst[count] = value
		count++
		offset += size
	}
	return count
}

func decodeLinearString(input string, dst *[]rune) []rune {
	runes := (*dst)[:0]
	for offset := 0; offset < len(input); {
		value, size := utf8.DecodeRuneInString(input[offset:])
		runes = append(runes, value)
		offset += size
	}
	*dst = runes
	return runes
}

func decodeLinearBytes(input []byte, dst *[]rune) []rune {
	runes := (*dst)[:0]
	for offset := 0; offset < len(input); {
		value, size := utf8.DecodeRune(input[offset:])
		runes = append(runes, value)
		offset += size
	}
	*dst = runes
	return runes
}

func (p *linearPattern) matchRunes(runes []rune, budget *matchBudget, states []bool) (bool, error) {
	return p.matchPreparedRunes(runes, budget, prepareLinearStates(len(runes), states))
}

func (p *linearPattern) matchPreparedRunes(runes []rune, budget *matchBudget, states []bool) (bool, error) {
	rowSize := len(runes) + 1
	if p.fixed {
		state := linearMatchState{budget: budget}
		// Preserve the general linear quota even when an exact-count mismatch
		// could finish without visiting every atom.
		for range p.atoms {
			if err := state.chargeLinearAtom(rowSize); err != nil {
				return false, err
			}
		}
		return p.matchFixedRunes(runes), nil
	}
	prev, next := states[:rowSize], states[rowSize:]
	clear(prev)
	prev[0] = true
	state := linearMatchState{prev: prev, next: next, budget: budget}
	for _, atom := range p.atoms {
		if err := state.advance(atom, runes, rowSize); err != nil {
			return false, err
		}
	}
	return state.prev[len(runes)], nil
}

func (p *linearPattern) matchFixedRunes(runes []rune) bool {
	offset := 0
	for _, atom := range p.atoms {
		if atom.min > uint64(len(runes)-offset) {
			return false
		}
		end := offset + int(atom.min) //nolint:gosec // The comparison bounds min by the remaining int-sized slice.
		for _, value := range runes[offset:end] {
			if !atom.set.contains(value) {
				return false
			}
		}
		offset = end
	}
	return offset == len(runes)
}

type linearMatchState struct {
	budget *matchBudget
	prev   []bool
	next   []bool
}

// prepareLinearStates sizes the two dynamic-programming rows. The row size is
// bounded by the input length, which the caller has already charged.
func prepareLinearStates(runeCount int, states []bool) []bool {
	size := 2 * (runeCount + 1)
	if cap(states) < size {
		return make([]bool, size)
	}
	return states[:size]
}

func (s *linearMatchState) advance(atom linearAtom, runes []rune, rowSize int) error {
	// The run scan and sliding-window updates each visit at most one input
	// offset per direction. Charge that bound before doing the work.
	if err := s.chargeLinearAtom(rowSize); err != nil {
		return err
	}
	clear(s.next)
	if atom.min == 0 {
		copy(s.next, s.prev)
	}
	if minCount, maxCount, ok := linearRepeatBounds(atom, uint64(len(runes)), len(runes)); ok {
		markLinearRuns(runes, s.prev, s.next, atom.set, minCount, maxCount)
	}
	s.prev, s.next = s.next, s.prev
	return nil
}

func (s *linearMatchState) chargeLinearAtom(rowSize int) error {
	if uint64(rowSize) > (math.MaxUint64-1)/4 { //nolint:gosec // rowSize is a non-negative slice-derived bound
		return &Error{Kind: ErrorLimit, Offset: -1, What: matchWorkLimitError}
	}
	return s.budget.step(uint64(rowSize)*4 + 1) //nolint:gosec // rowSize is a non-negative slice-derived bound
}

func linearRepeatBounds(atom linearAtom, runeCount uint64, runeCountInt int) (minCount, maxCount int, ok bool) {
	if atom.max == 0 || atom.min > runeCount {
		return 0, 0, false
	}
	minRepeat := atom.min
	if minRepeat == 0 {
		minRepeat = 1
	}
	if minRepeat > runeCount {
		return 0, 0, false
	}
	minCount = int(minRepeat)
	maxCount = runeCountInt
	if !atom.unbounded && atom.max < runeCount {
		// The comparison above proves atom.max is representable as an int.
		maxCount = int(atom.max) //nolint:gosec // bounded by maxCount
	}
	return minCount, maxCount, true
}

func markLinearRuns(runes []rune, prev, next []bool, set rangeSet, minRepeat, maxRepeat int) {
	if len(set.ranges) == 1 {
		markSingleRangeRuns(runes, prev, next, set.ranges[0], minRepeat, maxRepeat)
		return
	}
	markMultiRangeRuns(runes, prev, next, set, minRepeat, maxRepeat)
}

func markSingleRangeRuns(runes []rune, prev, next []bool, match runeRange, minRepeat, maxRepeat int) {
	start := 0
	for start < len(runes) {
		for start < len(runes) && (runes[start] < match.lo || runes[start] > match.hi) {
			start++
		}
		runStart := start
		for start < len(runes) && match.lo <= runes[start] && runes[start] <= match.hi {
			start++
		}
		markLinearRun(prev, next, linearRun{start: runStart, end: start, minRepeat: minRepeat, maxRepeat: maxRepeat, bounded: maxRepeat < len(runes)})
	}
}

func markMultiRangeRuns(runes []rune, prev, next []bool, set rangeSet, minRepeat, maxRepeat int) {
	start := 0
	for start < len(runes) {
		for start < len(runes) && !set.contains(runes[start]) {
			start++
		}
		runStart := start
		for start < len(runes) && set.contains(runes[start]) {
			start++
		}
		markLinearRun(prev, next, linearRun{start: runStart, end: start, minRepeat: minRepeat, maxRepeat: maxRepeat, bounded: maxRepeat < len(runes)})
	}
}

func markLinearRun(prev, next []bool, run linearRun) {
	if run.end-run.start < run.minRepeat {
		return
	}
	if run.bounded {
		markBoundedLinearRun(prev, next, run)
		return
	}
	markUnboundedLinearRun(prev, next, run)
}

func markUnboundedLinearRun(prev, next []bool, run linearRun) {
	active := 0
	for pos := run.start + run.minRepeat; pos <= run.end; pos++ {
		if prev[pos-run.minRepeat] {
			active++
		}
		if active > 0 {
			next[pos] = true
		}
	}
}

func markBoundedLinearRun(prev, next []bool, run linearRun) {
	active := 0
	for pos := run.start + run.minRepeat; pos <= run.end; pos++ {
		if prev[pos-run.minRepeat] {
			active++
		}
		remove := pos - run.maxRepeat - 1
		if remove >= run.start && prev[remove] {
			active--
		}
		if active > 0 {
			next[pos] = true
		}
	}
}
