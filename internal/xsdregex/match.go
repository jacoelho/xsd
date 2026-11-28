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
	linear linearScratch
	nfa    nfaScratch
}

// nfaScratch is the storage of the NFA runner that a Scratch keeps between
// matches.
//
// A marker in seen or least names the closure generation that set it, and
// mark names the current one. No marker in either backing array, up to its
// capacity, exceeds mark, and one equals mark only when the current
// generation set it; a stale slot would otherwise read as visited.
type nfaScratch struct {
	// current holds the threads at the input position, and next collects
	// those of the position after it.
	current []thread
	next    []thread
	stack   []thread
	// seen holds one generation marker per tracked slot.
	seen []uint32
	// least holds, per state, the smallest count at or above its counter's
	// minimum reached in the generation it records.
	least []leastCount
	mark  uint32
	// displaced reports that a count replaced a larger least in this
	// generation, whose thread next may still hold.
	displaced bool
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
	s.nfa.reset(maxRetainedRunes)
	s.linear.reset(maxRetainedRunes)
}

// reset restarts the generations at one, so it zeroes the markers it retains:
// a slot visited in an earlier generation would otherwise read as visited.
func (s *nfaScratch) reset(maxRetained int) {
	s.current = resetRetained(s.current, maxRetained)
	s.next = resetRetained(s.next, maxRetained)
	s.stack = resetRetained(s.stack, maxRetained)
	s.seen = resetMarkers(s.seen, maxRetained)
	s.least = resetMarkers(s.least, maxRetained)
	s.mark = 0
}

// prepare empties the stack and the collected threads, which hold entries
// only after a match that failed. Marker growth and the clear in begin are
// uncharged: the fixed per-pattern state bound caps them, and charging them
// would make reported work depend on scratch history (regex.go: equal inputs
// report equal work).
func (s *nfaScratch) prepare(pattern *Pattern) {
	s.seen = sizeMarkers(s.seen, pattern.trackedSlots())
	if pattern.counted != nil {
		s.least = sizeMarkers(s.least, len(pattern.states))
	}
	s.next = s.next[:0]
	s.stack = s.stack[:0]
}

// begin starts a closure generation, in which no slot is visited.
func (s *nfaScratch) begin() {
	s.mark++
	if s.mark == 0 {
		// Clear to capacity, not length: prepare reslices markers up to their
		// capacity for a larger pattern.
		clear(s.seen[:cap(s.seen)])
		clear(s.least[:cap(s.least)])
		s.mark = 1
	}
	s.displaced = false
}

func (s *nfaScratch) push(pending thread) {
	s.stack = append(s.stack, pending)
}

// pushBoth pushes the two targets of a split in one append. A variadic push
// builds its argument slice on every call, and two pushes keep the split
// handler from inlining: the closure walk is slower either way.
func (s *nfaScratch) pushBoth(first, second thread) {
	s.stack = append(s.stack, first, second)
}

func (s *nfaScratch) pop() (thread, bool) {
	last := len(s.stack) - 1
	if last < 0 {
		return thread{}, false
	}
	pending := s.stack[last]
	s.stack = s.stack[:last]
	return pending, true
}

func (s *nfaScratch) visit(slot uint32) bool {
	seen := &s.seen[slot]
	if *seen == s.mark {
		return false
	}
	*seen = s.mark
	return true
}

// dominated reports whether a count at most current's reached its state in
// this generation. When none did, current's count becomes the state's least.
func (s *nfaScratch) dominated(current thread) bool {
	least := &s.least[current.pc]
	if least.mark == s.mark {
		if least.count <= current.count {
			return true
		}
		s.displaced = true
	}
	*least = leastCount{mark: s.mark, count: current.count}
	return false
}

func (s *nfaScratch) collect(reached thread) {
	s.next = append(s.next, reached)
}

// advance makes the collected threads the current ones. It leaves out those a
// smaller count displaced after they were collected, so each state carries at
// most one count at or above its minimum into the next rune. It charges no
// work: each thread it checks was charged when collected, so the work bound
// holds.
func (s *nfaScratch) advance(slots []stateSlots) {
	collected := s.next
	if s.displaced {
		kept := collected[:0]
		for _, current := range collected {
			if current.count >= slots[current.pc].dominatedFrom && s.least[current.pc].count < current.count {
				continue
			}
			kept = append(kept, current)
		}
		collected = kept
	}
	s.next = s.current[:0]
	s.current = collected
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
	threads  *nfaScratch
	budget   *matchBudget
	slots    []stateSlots
	counters []counter
	// popCost is the work charged per closure pop: countedPopCost in a
	// machine with counters, whose visit and dominance checks take about a
	// third longer than a counter-free pop. The per-value work default is
	// calibrated in time (docs/RESOURCE_LIMITS.md).
	popCost uint64
	current uint64
	bitset  bool
}

func newRunner(pattern *Pattern, budget *matchBudget, threads *nfaScratch) (nfaRunner, error) {
	if len(pattern.states) == 0 || pattern.start < 0 || pattern.start >= len(pattern.states) {
		return nfaRunner{}, &Error{Kind: ErrorSyntax, Offset: -1, What: "invalid compiled pattern"}
	}
	if err := prepareRunnerScratch(pattern, threads); err != nil {
		return nfaRunner{}, err
	}
	runner := nfaRunner{
		pattern: pattern,
		threads: threads,
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

func prepareRunnerScratch(pattern *Pattern, threads *nfaScratch) error {
	if pattern.mode == modeBitset {
		return nil
	}
	if threads == nil {
		return &Error{Kind: ErrorSyntax, Offset: -1, What: "missing matcher scratch"}
	}
	threads.prepare(pattern)
	return nil
}

// sizeMarkers reslices markers to n slots, keeping the backing array when it
// is large enough. Slots past the old length hold markers below the current
// generation, because nfaScratch clears to capacity.
func sizeMarkers[T any](markers []T, n int) []T {
	if cap(markers) >= n {
		return markers[:n]
	}
	return make([]T, n)
}

func (r *nfaRunner) addClosure(start thread) error {
	if int(start.pc) >= len(r.pattern.states) {
		return &Error{Kind: ErrorSyntax, Offset: -1, What: invalidTransitionError}
	}
	if err := r.budget.step(1); err != nil {
		return err
	}
	r.threads.push(start)
	return r.drainClosure()
}

func (r *nfaRunner) drainClosure() error {
	for current, ok := r.threads.pop(); ok; current, ok = r.threads.pop() {
		if err := r.budget.step(r.popCost); err != nil {
			return err
		}
		var expand bool
		if r.slots == nil {
			expand = r.threads.visit(current.pc)
		} else {
			expand = r.claimCounted(current)
		}
		if !expand {
			continue
		}
		if err := r.processClosureInstruction(current); err != nil {
			return err
		}
	}
	return nil
}

const invalidTransitionError = "invalid NFA transition"

const countedPopCost = 2

// claimCounted marks the thread visited in this generation and reports whether the
// closure walk should expand it: it was not visited before and no smaller
// count dominates it.
func (r *nfaRunner) claimCounted(current thread) bool {
	slots := r.slots[current.pc]
	if !r.threads.visit(slots.first + current.count) {
		return false
	}
	return current.count < slots.dominatedFrom || !r.threads.dominated(current)
}

func (r *nfaRunner) processClosureInstruction(current thread) error {
	state := r.pattern.states[current.pc]
	switch state.kind {
	case instructionSplit:
		return r.processClosureSplit(state, current.count)
	case instructionJump:
		return r.processClosureJump(state, current.count)
	case instructionSet, instructionAccept:
		return r.processClosureTarget(current)
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
	r.threads.pushBoth(
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
	r.threads.push(thread{pc: uint32(instruction.out), count: count}) //nolint:gosec // states are bounded by the compile limit.
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
		r.threads.push(thread{pc: uint32(instruction.out), count: next}) //nolint:gosec // states are bounded by the compile limit.
	}
	if current.count >= bounds.min {
		r.threads.push(thread{pc: uint32(instruction.out1)}) //nolint:gosec // states are bounded by the compile limit.
	}
	return nil
}

func (r *nfaRunner) processClosureTarget(current thread) error {
	if err := r.budget.step(1); err != nil {
		return err
	}
	r.threads.collect(current)
	return nil
}

func (r *nfaRunner) start() error {
	if r.bitset {
		r.current = r.pattern.startBits
		return nil
	}
	r.threads.begin()
	if err := r.addClosure(thread{pc: uint32(r.pattern.start)}); err != nil { //nolint:gosec // newRunner checked start against the state count.
		return err
	}
	r.threads.advance(r.slots)
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
	r.threads.begin()
	for _, current := range r.threads.current {
		if err := r.advanceNFAState(value, current); err != nil {
			return err
		}
	}
	r.threads.advance(r.slots)
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
	return r.addClosure(thread{pc: uint32(state.out), count: current.count}) //nolint:gosec // states are bounded by the compile limit.
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
	for _, current := range r.threads.current {
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
	var owned nfaScratch
	threads := &owned
	if scratch != nil {
		threads = &scratch.nfa
	}
	runner, err := newRunner(pattern, budget, threads)
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
	var owned nfaScratch
	threads := &owned
	if scratch != nil {
		threads = &scratch.nfa
	}
	runner, err := newRunner(pattern, budget, threads)
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
