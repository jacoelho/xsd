package xsdregex

import (
	"math/bits"
	"slices"
	"unicode/utf8"
)

// dfa is a deterministic automaton over the classes of its alphabet. A state
// is the offset of its row in next, so a step is one lookup; row zero is the
// dead state, whose entries all lead back to it. States at or above
// acceptFrom accept.
type dfa struct {
	next       []uint16
	alphabet   alphabet
	start      int
	acceptFrom int
}

// matchBytes reports whether the automaton accepts input. It charges one unit
// per rune it steps over: all of them, or those up to the one that enters the
// dead state. The charge follows the loop, which the caller's charge for the
// input bytes already bounds.
//
//nolint:dupl // String and []byte inputs keep separate loops so each decodes its own representation without conversion.
func (d *dfa) matchBytes(input []byte, budget *matchBudget) (bool, error) {
	next := d.next
	state := d.start
	offset, continuation := 0, 0
	for offset < len(input) && state != 0 {
		var class uint8
		if first := input[offset]; first < utf8.RuneSelf {
			class = d.alphabet.ascii[first]
			offset++
		} else {
			value, size := utf8.DecodeRune(input[offset:])
			class = d.alphabet.class(value)
			offset += size
			continuation += size - 1
		}
		state = int(next[state+int(class)])
	}
	if err := budget.step(uint64(offset - continuation)); err != nil {
		return false, err
	}
	return state >= d.acceptFrom, nil
}

//nolint:dupl // String and []byte inputs keep separate loops so each decodes its own representation without conversion.
func (d *dfa) matchString(input string, budget *matchBudget) (bool, error) {
	next := d.next
	state := d.start
	offset, continuation := 0, 0
	for offset < len(input) && state != 0 {
		var class uint8
		if first := input[offset]; first < utf8.RuneSelf {
			class = d.alphabet.ascii[first]
			offset++
		} else {
			value, size := utf8.DecodeRuneInString(input[offset:])
			class = d.alphabet.class(value)
			offset += size
			continuation += size - 1
		}
		state = int(next[state+int(class)])
	}
	if err := budget.step(uint64(offset - continuation)); err != nil {
		return false, err
	}
	return state >= d.acceptFrom, nil
}

// maxDFAEntries is the largest table whose row offsets fit an entry.
const maxDFAEntries = 1 << 16

// compileDFA determinises the pattern's unrolled NFA and returns the steps it
// spent. It returns no table, and the caller compiles another matcher, when
// the NFA has more than limits.dfaStates states, which spends no step, or when
// it abandons the attempt: the table would exceed limits.dfaEntries entries,
// construction would take more than limits.dfaSteps steps, newClasses or
// alphabet.index refuses the pattern's sets, or two rows hash alike. The error
// is the NFA compiler's for a tree it rejects for any reason but its state
// limit.
func compileDFA(root *node, unrolled uint64, limits compileLimits) (*dfa, uint64, error) {
	if unrolled > limits.dfaStates || limits.dfaEntries > maxDFAEntries {
		return nil, 0, nil
	}
	c := nfaCompiler{limit: limits.dfaStates, states: make([]instruction, 0, unrolled)}
	start, err := c.compileRoot(root)
	if IsLimit(err) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	b := dfaBuilder{states: c.states, budget: buildBudget{remaining: limits.dfaSteps}, entries: limits.dfaEntries}
	return b.build(start), limits.dfaSteps - b.budget.remaining, nil
}

type dfaBuilder struct {
	// rows maps the hash of a row's members to the row.
	rows   map[uint64]uint16
	states []instruction
	// setOf maps a set state to its index in the alphabet's set list.
	setOf []int32
	// targetFirst and targets hold, per set state, the set and accept states
	// its match leads to through empty transitions.
	targetFirst []uint32
	targets     []uint32
	// memberFirst and members hold the NFA states of each row.
	memberFirst []uint32
	members     []uint32
	table       []uint16
	// seen marks the states one closure walk visited with its generation.
	seen    []uint32
	stack   []int
	classes classes
	budget  buildBudget
	entries uint64
	mark    uint32
}

// build returns the table, or nil when it abandons the attempt.
func (b *dfaBuilder) build(start int) *dfa {
	if !b.buildClasses() || !b.buildTargets() {
		return nil
	}
	// Row zero is the dead state: no members, every entry zero.
	b.rows = make(map[uint64]uint16, len(b.states))
	b.memberFirst = append(make([]uint32, 0, len(b.states)+2), 0, 0)
	b.members = make([]uint32, 0, 2*len(b.states))
	initial, ok := b.closure(nil, start)
	if !ok {
		return nil
	}
	slices.Sort(initial)
	first, ok := b.row(initial)
	if !ok || !b.fill() {
		return nil
	}
	return b.finish(first)
}

func (b *dfaBuilder) buildClasses() bool {
	if !b.budget.spend(len(b.states)) {
		return false
	}
	b.setOf = make([]int32, len(b.states))
	b.seen = make([]uint32, len(b.states))
	indexes := make(map[setKey]int32)
	var sets []rangeSet
	for pc, state := range b.states {
		b.setOf[pc] = -1
		if state.kind != instructionSet || len(state.set.ranges) == 0 {
			continue
		}
		key := setKey{first: &state.set.ranges[0], count: len(state.set.ranges)}
		index, ok := indexes[key]
		if !ok {
			index = int32(len(sets)) //nolint:gosec // Sets are bounded by the state count.
			indexes[key] = index
			sets = append(sets, state.set)
		}
		b.setOf[pc] = index
	}
	var ok bool
	b.classes, ok = newClasses(sets, &b.budget)
	return ok
}

func (b *dfaBuilder) buildTargets() bool {
	b.targetFirst = make([]uint32, len(b.states)+1)
	b.targets = make([]uint32, 0, len(b.states))
	for pc, state := range b.states {
		b.targetFirst[pc] = uint32(len(b.targets)) //nolint:gosec // Targets are bounded by the construction budget.
		if state.kind != instructionSet {
			continue
		}
		var ok bool
		if b.targets, ok = b.closure(b.targets, state.out); !ok {
			return false
		}
	}
	b.targetFirst[len(b.states)] = uint32(len(b.targets)) //nolint:gosec // Targets are bounded by the construction budget.
	return true
}

// closure appends the set and accept states reachable from pc through empty
// transitions, each once. The budget bounds the walks, so mark cannot wrap.
func (b *dfaBuilder) closure(dst []uint32, pc int) ([]uint32, bool) {
	b.mark++
	b.stack = append(b.stack[:0], pc)
	for len(b.stack) != 0 {
		current := b.stack[len(b.stack)-1]
		b.stack = b.stack[:len(b.stack)-1]
		if current < 0 || current >= len(b.states) || !b.budget.spend(1) {
			return nil, false
		}
		if b.seen[current] == b.mark {
			continue
		}
		b.seen[current] = b.mark
		if !walkEpsilonState(b.states[current], current, &b.stack, &dst) {
			return nil, false
		}
	}
	return dst, true
}

// FNV-1a parameters (http://www.isthe.com/chongo/tech/comp/fnv/).
const (
	fnvOffset = 14695981039346656037
	fnvPrime  = 1099511628211
)

func hashMembers(members []uint32) uint64 {
	hash := uint64(fnvOffset)
	for _, member := range members {
		hash = (hash ^ uint64(member)) * fnvPrime
	}
	return hash
}

// rowCount returns the rows added so far, the dead row among them.
func (b *dfaBuilder) rowCount() int {
	return len(b.memberFirst) - 1
}

func (b *dfaBuilder) accepts(index int) bool {
	for _, member := range b.members[b.memberFirst[index]:b.memberFirst[index+1]] {
		if b.states[member].kind == instructionAccept {
			return true
		}
	}
	return false
}

// row returns the row whose members are the sorted, distinct NFA states in
// members, adding it when it is new. Two rows whose members hash alike abandon
// the attempt.
func (b *dfaBuilder) row(members []uint32) (uint16, bool) {
	if !b.budget.spend(1 + len(members)) {
		return 0, false
	}
	hash := hashMembers(members)
	if existing, ok := b.rows[hash]; ok {
		return existing, slices.Equal(b.members[b.memberFirst[existing]:b.memberFirst[existing+1]], members)
	}
	index := b.rowCount()
	if uint64(index+1)*uint64(b.classes.count) > b.entries { //nolint:gosec // Row and class counts are non-negative.
		return 0, false
	}
	b.rows[hash] = uint16(index) //nolint:gosec // The entry limit bounds the row count below 1<<16.
	b.members = append(b.members, members...)
	b.memberFirst = append(b.memberFirst, uint32(len(b.members))) //nolint:gosec // Members are bounded by the construction budget.
	return uint16(index), true                                    //nolint:gosec // The entry limit bounds the row count below 1<<16.
}

// fill computes the entries of every row. Computing a row's entries adds the
// rows they lead to, which the loop reaches in turn.
func (b *dfaBuilder) fill() bool {
	width := b.classes.count
	b.table = make([]uint16, width, min(uint64(len(b.states)+1)*uint64(width), b.entries)) //nolint:gosec // State and class counts are non-negative.
	reached := make([][]uint32, width)
	for index := 1; index < b.rowCount(); index++ {
		if !b.reach(index, reached) {
			return false
		}
		for class := range reached {
			entry, ok := b.entry(reached[class])
			if !ok {
				return false
			}
			reached[class] = reached[class][:0]
			b.table = append(b.table, entry)
		}
	}
	return true
}

// reach appends to reached, per class, the states the members of row index
// lead to on a character of that class.
func (b *dfaBuilder) reach(index int, reached [][]uint32) bool {
	for _, member := range b.members[b.memberFirst[index]:b.memberFirst[index+1]] {
		set := b.setOf[member]
		if set < 0 {
			continue
		}
		targets := b.targets[b.targetFirst[member]:b.targetFirst[member+1]]
		if !b.budget.spend(len(targets) * len(b.classes.bySet[set])) {
			return false
		}
		for _, class := range b.classes.bySet[set] {
			reached[class] = append(reached[class], targets...)
		}
	}
	return true
}

// entry returns the row of the states in reached, which it sorts and
// deduplicates in place. No states lead to the dead row.
func (b *dfaBuilder) entry(reached []uint32) (uint16, bool) {
	count := len(reached)
	if count == 0 {
		return 0, true
	}
	if !b.budget.spend(count * bits.Len(uint(count))) {
		return 0, false
	}
	slices.Sort(reached)
	return b.row(slices.Compact(reached))
}

// finish numbers accepting rows last and rewrites entries as row offsets.
// The dead row is not accepting and is the first row, so it keeps offset zero.
func (b *dfaBuilder) finish(first uint16) *dfa {
	width := b.classes.count
	offsets := make([]int, b.rowCount())
	accepting := make([]bool, len(offsets))
	position := 0
	for index := range offsets {
		accepting[index] = b.accepts(index)
		if !accepting[index] {
			offsets[index] = position * width
			position++
		}
	}
	acceptFrom := position * width
	for index, accepts := range accepting {
		if accepts {
			offsets[index] = position * width
			position++
		}
	}
	next := make([]uint16, len(b.table))
	for index, offset := range offsets {
		for class := range width {
			next[offset+class] = uint16(offsets[b.table[index*width+class]]) //nolint:gosec // The entry limit bounds offsets below 1<<16.
		}
	}
	return &dfa{next: next, alphabet: b.classes.alphabet, start: offsets[first], acceptFrom: acceptFrom}
}
