package xsdregex

import (
	"math"
	"math/bits"
	"unicode/utf8"
)

// Scratch owns the temporary storage used by one matcher. It may be reused
// sequentially, but a Scratch value must not be shared by concurrent calls.
// Pattern instances are immutable and can be matched concurrently when each
// call has its own Scratch value (or no Scratch value).
type Scratch struct {
	current      []thread
	next         []thread
	stack        []thread
	linearRunes  []rune
	linearStates []bool
	// seen holds one generation marker per tracked slot.
	seen []uint32
	// least holds, per state, the smallest count at or above its counter's
	// minimum reached in the generation it records.
	least []leastCount
	mark  uint32
}

type leastCount struct {
	mark, count uint32
}

// thread is one active state with the count of finished iterations of the
// counter enclosing it, zero outside counters.
type thread struct {
	pc    uint32
	count uint32
}

func bitCount(value uint64) uint64 {
	return uint64(bits.OnesCount64(value)) //nolint:gosec // Population counts are in [0, 64].
}

// Reset clears matcher state and retains only bounded scratch capacity. The
// caller owns the retention policy because a validation session may process
// one unusually large lexical value followed by many small values.
func (s *Scratch) Reset(maxRetainedRunes int) {
	if s == nil {
		return
	}
	if maxRetainedRunes < 0 {
		*s = Scratch{}
		return
	}
	s.current = resetRetained(s.current, maxRetainedRunes)
	s.next = resetRetained(s.next, maxRetainedRunes)
	s.stack = resetRetained(s.stack, maxRetainedRunes)
	// seen stores generation markers rather than transient slice contents. Reset
	// restarts mark at one, so retained slots must be zeroed before reuse or a
	// state visited in the previous generation can be mistaken for a current
	// visit.
	s.seen = resetMarkers(s.seen, maxRetainedRunes)
	s.least = resetMarkers(s.least, maxRetainedRunes)
	s.linearRunes = resetRetained(s.linearRunes, maxRetainedRunes)
	s.linearStates = resetRetained(s.linearStates, retainedStateLimit(maxRetainedRunes))
	s.mark = 0
}

func resetMarkers[T any](markers []T, maxCapacity int) []T {
	if cap(markers) > maxCapacity {
		return nil
	}
	clear(markers[:cap(markers)])
	return markers[:0]
}

func resetRetained[T any](values []T, maxCapacity int) []T {
	if cap(values) > maxCapacity {
		return nil
	}
	return values[:0]
}

func retainedStateLimit(maxRetainedRunes int) int {
	if maxRetainedRunes > (math.MaxInt-2)/2 {
		return 0
	}
	return 2 * (maxRetainedRunes + 1)
}

type matchBudget struct {
	limit uint64
	work  uint64
}

func (b *matchBudget) step(units uint64) error {
	if units > b.limit || b.work > b.limit-units {
		return &Error{Kind: ErrorLimit, Offset: -1, What: matchWorkLimitError}
	}
	b.work += units
	return nil
}

type nfaRunner struct {
	pattern  *Pattern
	scratch  *Scratch
	budget   *matchBudget
	slots    []stateSlots
	counters []counter
	// popCost is the work charged per closure pop: countedPopCost in a
	// machine with counters, whose visit and dominance checks take about a
	// third longer than a counter-free pop. The per-value work default is
	// calibrated in time (docs/RESOURCE_LIMITS.md).
	popCost   uint64
	current   uint64
	bitset    bool
	displaced bool
}

func newRunner(pattern *Pattern, budget *matchBudget, scratch *Scratch) (nfaRunner, error) {
	if len(pattern.states) == 0 || pattern.start < 0 || pattern.start >= len(pattern.states) {
		return nfaRunner{}, &Error{Kind: ErrorSyntax, Offset: -1, What: "invalid compiled pattern"}
	}
	if err := prepareRunnerScratch(pattern, scratch); err != nil {
		return nfaRunner{}, err
	}
	runner := nfaRunner{
		pattern: pattern,
		scratch: scratch,
		budget:  budget,
		popCost: 1,
		bitset:  pattern.mode == modeBitset,
	}
	if pattern.counted != nil {
		runner.slots = pattern.counted.slots
		runner.counters = pattern.counted.counters
		runner.popCost = countedPopCost
	}
	return runner, nil
}

func prepareRunnerScratch(pattern *Pattern, scratch *Scratch) error {
	if pattern.mode == modeBitset {
		return nil
	}
	if scratch == nil {
		return &Error{Kind: ErrorSyntax, Offset: -1, What: "missing matcher scratch"}
	}
	// Scratch growth and the generation-wraparound clear are uncharged: the
	// fixed per-pattern state bound caps them, and charging them would make
	// reported work depend on scratch history (regex.go: equal inputs report
	// equal work).
	scratch.seen = sizeMarkers(scratch.seen, pattern.trackedSlots())
	if pattern.counted != nil {
		scratch.least = sizeMarkers(scratch.least, len(pattern.states))
	}
	scratch.current = scratch.current[:0]
	scratch.next = scratch.next[:0]
	scratch.stack = scratch.stack[:0]
	return nil
}

// sizeMarkers reslices markers to n slots, keeping the backing array when it
// is large enough. Slots past the old length hold markers below the current
// generation, because wraparound and Reset clear to capacity.
func sizeMarkers[T any](markers []T, n int) []T {
	if cap(markers) >= n {
		return markers[:n]
	}
	return make([]T, n)
}

func (r *nfaRunner) beginClosure() {
	r.scratch.mark++
	if r.scratch.mark == 0 {
		// Clear to capacity, not length: prepareRunnerScratch reslices markers
		// up to their capacity for a larger pattern, and every marker in the
		// backing array must stay below mark or a stale slot reads as visited.
		clear(r.scratch.seen[:cap(r.scratch.seen)])
		clear(r.scratch.least[:cap(r.scratch.least)])
		r.scratch.mark = 1
	}
	r.scratch.stack = r.scratch.stack[:0]
	r.displaced = false
}

func (r *nfaRunner) addClosure(dst *[]thread, start thread) error {
	if int(start.pc) >= len(r.pattern.states) {
		return &Error{Kind: ErrorSyntax, Offset: -1, What: invalidTransitionError}
	}
	if err := r.budget.step(1); err != nil {
		return err
	}
	r.scratch.stack = append(r.scratch.stack, start)
	return r.drainClosure(dst)
}

func (r *nfaRunner) drainClosure(dst *[]thread) error {
	for len(r.scratch.stack) != 0 {
		last := len(r.scratch.stack) - 1
		current := r.scratch.stack[last]
		r.scratch.stack = r.scratch.stack[:last]
		if err := r.budget.step(r.popCost); err != nil {
			return err
		}
		var expand bool
		if r.slots == nil {
			expand = r.markSeen(current.pc)
		} else {
			expand = r.claimCounted(current)
		}
		if !expand {
			continue
		}
		if err := r.processClosureInstruction(dst, current); err != nil {
			return err
		}
	}
	return nil
}

const invalidTransitionError = "invalid NFA transition"

const countedPopCost = 2

// markSeen marks a state of a counter-free machine visited in this generation
// and reports whether it was unvisited.
func (r *nfaRunner) markSeen(pc uint32) bool {
	seen := &r.scratch.seen[pc]
	if *seen == r.scratch.mark {
		return false
	}
	*seen = r.scratch.mark
	return true
}

// claimCounted marks the thread visited in this generation and reports whether the
// closure walk should expand it: it was not visited before and no smaller
// count dominates it.
func (r *nfaRunner) claimCounted(current thread) bool {
	slots := r.slots[current.pc]
	slot := slots.first + current.count
	if r.scratch.seen[slot] == r.scratch.mark {
		return false
	}
	r.scratch.seen[slot] = r.scratch.mark
	return current.count < slots.dominatedFrom || !r.dominated(current)
}

func (r *nfaRunner) dominated(current thread) bool {
	least := &r.scratch.least[current.pc]
	if least.mark == r.scratch.mark {
		if least.count <= current.count {
			return true
		}
		r.displaced = true
	}
	*least = leastCount{mark: r.scratch.mark, count: current.count}
	return false
}

// dropDominated removes threads that a smaller count displaced after they
// were added, so each state carries at most one count at or above its
// minimum into the next rune. It charges no work: each thread it checks was
// charged when added, so the work bound holds.
func (r *nfaRunner) dropDominated(threads []thread) []thread {
	if !r.displaced {
		return threads
	}
	kept := threads[:0]
	for _, current := range threads {
		if current.count >= r.slots[current.pc].dominatedFrom && r.scratch.least[current.pc].count < current.count {
			continue
		}
		kept = append(kept, current)
	}
	return kept
}

func (r *nfaRunner) processClosureInstruction(dst *[]thread, current thread) error {
	state := r.pattern.states[current.pc]
	switch state.kind {
	case instructionSplit:
		return r.processClosureSplit(state, current.count)
	case instructionJump:
		return r.processClosureJump(state, current.count)
	case instructionSet, instructionAccept:
		return r.processClosureTarget(dst, current)
	case instructionCountHead:
		return r.processClosureHead(state, current)
	default:
		return &Error{Kind: ErrorSyntax, Offset: -1, What: "invalid NFA instruction"}
	}
}

func (r *nfaRunner) processClosureSplit(instruction instruction, count uint32) error {
	if instruction.out|instruction.out1 < 0 {
		return &Error{Kind: ErrorSyntax, Offset: -1, What: "incomplete split instruction"}
	}
	r.scratch.stack = append(r.scratch.stack,
		thread{pc: uint32(instruction.out), count: count},  //nolint:gosec // states are bounded by the compile limit.
		thread{pc: uint32(instruction.out1), count: count}) //nolint:gosec // states are bounded by the compile limit.
	return r.budget.step(2)
}

func (r *nfaRunner) processClosureJump(instruction instruction, count uint32) error {
	if instruction.out < 0 {
		return &Error{Kind: ErrorSyntax, Offset: -1, What: "incomplete jump instruction"}
	}
	if err := r.budget.step(1); err != nil {
		return err
	}
	r.scratch.stack = append(r.scratch.stack, thread{pc: uint32(instruction.out), count: count}) //nolint:gosec // states are bounded by the compile limit.
	return nil
}

func (r *nfaRunner) processClosureHead(instruction instruction, current thread) error {
	if instruction.out|instruction.out1 < 0 {
		return &Error{Kind: ErrorSyntax, Offset: -1, What: "incomplete counter head"}
	}
	if err := r.budget.step(2); err != nil {
		return err
	}
	bounds := r.counters[r.slots[current.pc].counter]
	if bounds.unbounded || current.count < bounds.max {
		next := current.count + 1
		if bounds.unbounded && next > bounds.min {
			next = bounds.min
		}
		r.scratch.stack = append(r.scratch.stack, thread{pc: uint32(instruction.out), count: next}) //nolint:gosec // states are bounded by the compile limit.
	}
	if current.count >= bounds.min {
		r.scratch.stack = append(r.scratch.stack, thread{pc: uint32(instruction.out1)}) //nolint:gosec // states are bounded by the compile limit.
	}
	return nil
}

func (r *nfaRunner) processClosureTarget(dst *[]thread, current thread) error {
	if err := r.budget.step(1); err != nil {
		return err
	}
	*dst = append(*dst, current)
	return nil
}

func (r *nfaRunner) start() error {
	if r.bitset {
		r.current = r.pattern.startBits
		return nil
	}
	r.beginClosure()
	if err := r.addClosure(&r.scratch.current, thread{pc: uint32(r.pattern.start)}); err != nil { //nolint:gosec // newRunner checked start against the state count.
		return err
	}
	r.scratch.current = r.dropDominated(r.scratch.current)
	return nil
}

func (r *nfaRunner) rune(value rune) error {
	if r.bitset {
		return r.bitsetRune(value)
	}
	return r.nfaRune(value)
}

func (r *nfaRunner) bitsetRune(value rune) error {
	work := uint64(1)
	previous := r.current
	var next uint64
	for previous != 0 {
		bit := uint(bits.TrailingZeros64(previous))
		previous &^= uint64(1) << bit
		work++
		state := r.pattern.states[bit]
		if state.kind != instructionSet || !matchesRune(state.set, value) {
			continue
		}
		closure := r.pattern.closureBits[bit]
		work += bitCount(closure)
		next |= closure
	}
	if err := r.budget.step(work); err != nil {
		return err
	}
	r.current = next
	return nil
}

func (r *nfaRunner) nfaRune(value rune) error {
	if err := r.budget.step(1); err != nil {
		return err
	}
	previous := r.scratch.current
	r.scratch.next = r.scratch.next[:0]
	r.beginClosure()
	for _, current := range previous {
		if err := r.advanceNFAState(value, current); err != nil {
			return err
		}
	}
	r.scratch.current = r.dropDominated(r.scratch.next)
	r.scratch.next = previous[:0]
	return nil
}

func (r *nfaRunner) advanceNFAState(value rune, current thread) error {
	if err := r.budget.step(1); err != nil {
		return err
	}
	state := r.pattern.states[current.pc]
	if state.kind != instructionSet || !matchesRune(state.set, value) {
		return nil
	}
	if state.out < 0 {
		return &Error{Kind: ErrorSyntax, Offset: -1, What: invalidTransitionError}
	}
	return r.addClosure(&r.scratch.next, thread{pc: uint32(state.out), count: current.count}) //nolint:gosec // states are bounded by the compile limit.
}

func matchesRune(set rangeSet, value rune) bool {
	if len(set.ranges) == 1 {
		current := set.ranges[0]
		return current.lo <= value && value <= current.hi
	}
	return set.contains(value)
}

func (r *nfaRunner) accepts() (bool, error) {
	if r.bitset {
		if err := r.budget.step(bitCount(r.current)); err != nil {
			return false, err
		}
		return r.current&r.pattern.acceptBits != 0, nil
	}
	for _, current := range r.scratch.current {
		if err := r.budget.step(1); err != nil {
			return false, err
		}
		if r.pattern.states[current.pc].kind == instructionAccept {
			return true, nil
		}
	}
	return false, nil
}

// matchString charges one unit per input byte before decoding or growing
// scratch, so every path pays for the input it reads.
func matchString(pattern *Pattern, input string, options MatchOptions, scratch *Scratch) (bool, uint64, error) {
	budget := matchBudget{limit: options.MaxWork}
	if err := budget.step(uint64(len(input))); err != nil {
		return false, 0, err
	}
	matched, err := matchStringCharged(pattern, input, &budget, scratch)
	return matched, budget.work, err
}

//nolint:dupl // String and []byte inputs keep separate loops so each decodes its own representation without conversion.
func matchStringCharged(pattern *Pattern, input string, budget *matchBudget, scratch *Scratch) (bool, error) {
	if pattern.mode == modeDFA {
		return pattern.dfa.matchString(input, budget)
	}
	if pattern.mode == modeLinear {
		return pattern.linear.matchString(input, budget, scratch)
	}
	var owned Scratch
	if scratch == nil {
		scratch = &owned
	}
	runner, err := newRunner(pattern, budget, scratch)
	if err != nil {
		return false, err
	}
	if err := runner.start(); err != nil {
		return false, err
	}
	offset := 0
	for offset < len(input) {
		value, size := utf8.DecodeRuneInString(input[offset:])
		if err := runner.rune(value); err != nil {
			return false, err
		}
		offset += size
	}
	return runner.accepts()
}

func matchBytes(pattern *Pattern, input []byte, options MatchOptions, scratch *Scratch) (bool, uint64, error) {
	budget := matchBudget{limit: options.MaxWork}
	if err := budget.step(uint64(len(input))); err != nil {
		return false, 0, err
	}
	matched, err := matchBytesCharged(pattern, input, &budget, scratch)
	return matched, budget.work, err
}

//nolint:dupl // String and []byte inputs keep separate loops so each decodes its own representation without conversion.
func matchBytesCharged(pattern *Pattern, input []byte, budget *matchBudget, scratch *Scratch) (bool, error) {
	if pattern.mode == modeDFA {
		return pattern.dfa.matchBytes(input, budget)
	}
	if pattern.mode == modeLinear {
		return pattern.linear.matchBytes(input, budget, scratch)
	}
	var owned Scratch
	if scratch == nil {
		scratch = &owned
	}
	runner, err := newRunner(pattern, budget, scratch)
	if err != nil {
		return false, err
	}
	if err := runner.start(); err != nil {
		return false, err
	}
	offset := 0
	for offset < len(input) {
		value, size := utf8.DecodeRune(input[offset:])
		if err := runner.rune(value); err != nil {
			return false, err
		}
		offset += size
	}
	return runner.accepts()
}
