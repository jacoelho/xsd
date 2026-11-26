package xsdregex

import (
	"math"
	"math/bits"
)

type instructionKind uint8

const (
	instructionSet instructionKind = iota + 1
	instructionSplit
	instructionJump
	instructionAccept
	// instructionCountEnter starts a counted repeat at count zero; out is its
	// head.
	instructionCountEnter
	// instructionCountHead holds the number of finished iterations. It starts
	// another (out, count plus one) while the count is below the maximum and
	// leaves the repeat (out1) once the count reaches the minimum. Body exits
	// lead back to the head, so a body state's count is the iteration it is in.
	instructionCountHead
)

type instruction struct {
	set  rangeSet
	out  int
	out1 int
	kind instructionKind
}

type patch struct {
	pc    int
	which uint8
}

type fragment struct {
	outs  []patch
	start int
}

type nfaCompiler struct {
	counted  map[*node]bool
	lengths  map[*node]matchLength
	states   []instruction
	counters []counter
	regions  []counterRegion
	limit    uint64
	// counting is false while a counted body compiles: counters do not nest.
	counting bool
}

type compiledNFA struct {
	counted      *countedRepeats
	states       []instruction
	closures     [][]int
	startTargets []int
	start        int
}

const stateLimitError = "compiled NFA state count exceeds limit"

// countedRepeats holds the tables of a machine with counters. A thread is a
// state and a count, and slots gives each pair its own visit marker.
type countedRepeats struct {
	counters     []counter
	slots        []stateSlots
	trackedSlots int
}

// counter is one counted repeat r{min,max} whose body is compiled once. A
// thread inside it carries the number of finished iterations. Counts at or
// above min that are larger than another count at the same state are
// dominated: the smaller count admits every continuation the larger one does.
// An unbounded counter stores counts above min as min.
type counter struct {
	min       uint32
	max       uint32
	unbounded bool
}

func (c counter) limit() uint32 {
	if c.unbounded {
		return c.min
	}
	return c.max
}

// stateSlots places one state's visit slots: a state outside every counter
// has one slot, a state inside a counter one per count up to its limit.
// Counts at or above dominatedFrom are dominated by smaller ones; it is
// math.MaxUint32 outside counters and in unbounded counters, whose counts
// above the minimum are already stored as the minimum.
type stateSlots struct {
	first         uint32
	dominatedFrom uint32
	counter       int32
}

type counterRegion struct {
	first, end int
	counter    int32
}

type bitsetNFA struct {
	closureBits []uint64
	startBits   uint64
	acceptBits  uint64
	enabled     bool
}

// compileNFA builds the Thompson graph; Compile assembles the Pattern.
//
// Rejected: returning a fully initialized Pattern from this helper, because
// its frame stays live during bitset allocation and enlarges that allocation
// call chain; reopen only with evidence of non-increasing stack retention,
// which removing intermediate records and preserving field layout do not give.
func compileNFA(root *node, options compileLimits) (compiledNFA, error) {
	plan := newRepeatPlan(root)
	// Admission charges the unrolled size so counted repeats admit exactly the
	// patterns their unrolled form does.
	if plan.unrolled > options.states {
		return compiledNFA{}, &Error{Kind: ErrorLimit, Offset: -1, What: stateLimitError}
	}
	// The scalar and bitset runners hold no counts; a machine that fits them
	// unrolled stays unrolled.
	if plan.unrolled <= 64 {
		plan.counted = nil
	}
	c := nfaCompiler{limit: options.states, counted: plan.counted, lengths: plan.lengths, counting: len(plan.counted) != 0}
	rootFragment, err := c.compile(root)
	if err != nil {
		return compiledNFA{}, err
	}
	accept, err := c.emit(instruction{kind: instructionAccept, out: -1, out1: -1})
	if err != nil {
		return compiledNFA{}, err
	}
	if err := c.patch(rootFragment.outs, accept); err != nil {
		return compiledNFA{}, err
	}
	nfa := compiledNFA{states: c.states, start: rootFragment.start}
	if len(c.counters) != 0 {
		slots, tracked := layoutSlots(len(c.states), c.counters, c.regions)
		nfa.counted = &countedRepeats{counters: c.counters, slots: slots, trackedSlots: tracked}
		return nfa, nil
	}
	// Precomputed closures are only useful to the compact <=64-state scalar and
	// bitset runners, and the Pattern retains tables derived from them rather
	// than the closures. Larger machines use the bounded on-demand walk and avoid carrying
	// a potentially quadratic derived graph through compilation. Those runners
	// hold no counts, so a machine with counters always takes the walk.
	if len(c.states) <= 64 {
		nfa.closures, nfa.startTargets = precomputeClosures(c.states, rootFragment.start)
	}
	return nfa, nil
}

// layoutSlots gives each state its visit slots in state order and returns the
// total. A counter with limit L over b body states takes (b+1)*(L+1) slots
// where its unrolled form takes at least b*L states, and counters require
// L >= 2, so the total is at most three times the unrolled state count.
func layoutSlots(stateCount int, counters []counter, regions []counterRegion) ([]stateSlots, int) {
	slots := make([]stateSlots, stateCount)
	for pc := range slots {
		slots[pc] = stateSlots{dominatedFrom: math.MaxUint32, counter: -1}
	}
	for _, region := range regions {
		bounds := counters[region.counter]
		dominatedFrom := uint32(math.MaxUint32)
		if !bounds.unbounded {
			dominatedFrom = bounds.min
		}
		for pc := region.first; pc < region.end; pc++ {
			slots[pc] = stateSlots{dominatedFrom: dominatedFrom, counter: region.counter}
		}
	}
	total := 0
	for pc := range slots {
		slots[pc].first = uint32(total)
		if id := slots[pc].counter; id >= 0 {
			total += int(counters[id].limit()) + 1
		} else {
			total++
		}
	}
	return slots, total
}

// precomputeClosures removes repeated epsilon-graph walks from the hot match
// loop for the small machines used by the bitset runner. Larger machines use
// the bounded on-demand walk instead of retaining a derived graph.
func precomputeClosures(states []instruction, start int) ([][]int, []int) {
	if start < 0 || start >= len(states) {
		return nil, nil
	}
	seen := make([]uint32, len(states))
	var mark uint32
	startTargets, ok := epsilonTargets(states, start, seen, &mark)
	if !ok {
		return nil, nil
	}
	closures := make([][]int, len(states))
	for pc, instruction := range states {
		if instruction.kind != instructionSet {
			continue
		}
		targets, ok := epsilonTargets(states, instruction.out, seen, &mark)
		if !ok {
			return nil, nil
		}
		closures[pc] = targets
	}
	return closures, startTargets
}

// scalarNextStates returns, for each set state, the one state its match leads
// to, and -1 for every other state. The slice has exactly one entry per state.
func scalarNextStates(states []instruction, closures [][]int) []int {
	next := make([]int, len(states))
	for pc, instruction := range states {
		next[pc] = -1
		if instruction.kind == instructionSet {
			next[pc] = closures[pc][0]
		}
	}
	return next
}

func isDeterministicNFA(states []instruction, closures [][]int, startTargets []int) bool {
	if len(closures) == 0 || len(startTargets) != 1 {
		return false
	}
	for pc, instruction := range states {
		if instruction.kind == instructionSet && len(closures[pc]) != 1 {
			return false
		}
	}
	return true
}

func buildBitsetNFA(states []instruction, closures [][]int, startTargets []int) bitsetNFA {
	if len(states) == 0 || len(states) > 64 || len(closures) == 0 || len(startTargets) == 0 {
		return bitsetNFA{}
	}
	closureBits, ok := buildClosureBits(states, closures)
	if !ok {
		return bitsetNFA{}
	}
	startBits, ok := buildStartBits(startTargets, len(states))
	if !ok {
		return bitsetNFA{}
	}
	acceptBits := buildAcceptBits(states)
	return bitsetNFA{enabled: true, closureBits: closureBits, startBits: startBits, acceptBits: acceptBits}
}

func buildClosureBits(states []instruction, closures [][]int) ([]uint64, bool) {
	closureBits := make([]uint64, len(states))
	for pc, instruction := range states {
		if instruction.kind != instructionSet {
			continue
		}
		if !appendClosureBits(closureBits, pc, closures[pc], len(states)) {
			return nil, false
		}
	}
	return closureBits, true
}

func appendClosureBits(closureBits []uint64, pc int, targets []int, stateCount int) bool {
	if targets == nil {
		return false
	}
	for _, target := range targets {
		if target < 0 || target >= stateCount {
			return false
		}
		closureBits[pc] |= uint64(1) << uint(target)
	}
	return true
}

func buildStartBits(startTargets []int, stateCount int) (uint64, bool) {
	var startBits uint64
	for _, target := range startTargets {
		if target < 0 || target >= stateCount {
			return 0, false
		}
		startBits |= uint64(1) << uint(target)
	}
	return startBits, true
}

func buildAcceptBits(states []instruction) uint64 {
	var acceptBits uint64
	for pc, instruction := range states {
		if instruction.kind == instructionAccept {
			acceptBits |= uint64(1) << uint(pc)
		}
	}
	return acceptBits
}

func epsilonTargets(states []instruction, start int, seen []uint32, mark *uint32) ([]int, bool) {
	if start < 0 || start >= len(states) {
		return nil, false
	}
	beginEpsilonMark(seen, mark)
	stack := []int{start}
	targets := make([]int, 0, 4)
	for len(stack) != 0 {
		pc, rest := popEpsilonState(stack)
		stack = rest
		if !validEpsilonState(pc, len(states)) {
			return nil, false
		}
		if seen[pc] == *mark {
			continue
		}
		seen[pc] = *mark
		if !walkEpsilonState(states[pc], pc, &stack, &targets) {
			return nil, false
		}
	}
	return targets, true
}

func beginEpsilonMark(seen []uint32, mark *uint32) {
	(*mark)++
	if *mark != 0 {
		return
	}
	clear(seen)
	*mark = 1
}

func popEpsilonState(stack []int) (int, []int) {
	last := len(stack) - 1
	return stack[last], stack[:last]
}

func validEpsilonState(pc, stateCount int) bool {
	return pc >= 0 && pc < stateCount
}

func walkEpsilonState(state instruction, pc int, stack *[]int, targets *[]int) bool {
	switch state.kind {
	case instructionSplit:
		if state.out < 0 || state.out1 < 0 {
			return false
		}
		*stack = append(*stack, state.out, state.out1)
		return true
	case instructionJump:
		if state.out < 0 {
			return false
		}
		*stack = append(*stack, state.out)
		return true
	case instructionSet, instructionAccept:
		*targets = append(*targets, pc)
		return true
	case instructionCountEnter, instructionCountHead:
		// Closures are precomputed only for machines without counters.
	}
	return false
}

func (c *nfaCompiler) emit(instruction instruction) (int, error) {
	if uint64(len(c.states)) >= c.limit {
		return -1, &Error{Kind: ErrorLimit, Offset: -1, What: stateLimitError}
	}
	pc := len(c.states)
	c.states = append(c.states, instruction)
	return pc, nil
}

func (c *nfaCompiler) patch(outs []patch, target int) error {
	if target < 0 || target >= len(c.states) {
		return &Error{Kind: ErrorSyntax, Offset: -1, What: "invalid NFA patch target"}
	}
	for _, out := range outs {
		if out.pc < 0 || out.pc >= len(c.states) {
			return &Error{Kind: ErrorSyntax, Offset: -1, What: "invalid NFA patch source"}
		}
		instruction := &c.states[out.pc]
		switch out.which {
		case 0:
			instruction.out = target
		case 1:
			instruction.out1 = target
		default:
			return &Error{Kind: ErrorSyntax, Offset: -1, What: "invalid NFA patch field"}
		}
	}
	return nil
}

func (c *nfaCompiler) compile(current *node) (fragment, error) {
	if current == nil {
		return fragment{}, &Error{Kind: ErrorSyntax, Offset: -1, What: "nil AST node"}
	}
	switch current.kind {
	case nodeEmpty:
		return c.compileEmpty()
	case nodeSet:
		return c.compileSet(current.set)
	case nodeConcat:
		return c.compileConcat(current.children)
	case nodeAlt:
		return c.compileAlt(current.children)
	case nodeRepeat:
		return c.compileRepeat(current)
	default:
		return fragment{}, &Error{Kind: ErrorSyntax, Offset: -1, What: "invalid AST node"}
	}
}

func (c *nfaCompiler) compileEmpty() (fragment, error) {
	pc, err := c.emit(instruction{kind: instructionJump, out: -1, out1: -1})
	if err != nil {
		return fragment{}, err
	}
	return fragment{start: pc, outs: []patch{{pc: pc, which: 0}}}, nil
}

func (c *nfaCompiler) compileSet(set rangeSet) (fragment, error) {
	pc, err := c.emit(instruction{kind: instructionSet, set: set, out: -1, out1: -1})
	if err != nil {
		return fragment{}, err
	}
	return fragment{start: pc, outs: []patch{{pc: pc, which: 0}}}, nil
}

func (c *nfaCompiler) compileConcat(children []*node) (fragment, error) {
	if len(children) == 0 {
		return c.compileEmpty()
	}
	result, err := c.compile(children[0])
	if err != nil {
		return fragment{}, err
	}
	for _, child := range children[1:] {
		next, err := c.compile(child)
		if err != nil {
			return fragment{}, err
		}
		if err := c.patch(result.outs, next.start); err != nil {
			return fragment{}, err
		}
		result.outs = next.outs
	}
	return result, nil
}

func (c *nfaCompiler) compileAlt(children []*node) (fragment, error) {
	if len(children) == 0 {
		return c.compileEmpty()
	}
	result, err := c.compile(children[0])
	if err != nil {
		return fragment{}, err
	}
	for _, child := range children[1:] {
		next, err := c.compile(child)
		if err != nil {
			return fragment{}, err
		}
		split, err := c.emit(instruction{kind: instructionSplit, out: result.start, out1: next.start})
		if err != nil {
			return fragment{}, err
		}
		result = fragment{start: split, outs: append(result.outs, next.outs...)}
	}
	return result, nil
}

func (c *nfaCompiler) compileRepeat(current *node) (fragment, error) {
	child := current.children
	if len(child) != 1 || child[0] == nil {
		return fragment{}, &Error{Kind: ErrorSyntax, Offset: -1, What: "invalid repeat node"}
	}
	if c.counting && c.counted[current] {
		return c.compileCounter(current, child[0])
	}
	result, err := c.compileRequired(child[0], current.min)
	if err != nil {
		return fragment{}, err
	}
	if current.unbounded {
		return c.compileUnboundedRepeat(result, child[0])
	}
	return c.compileOptionalRepeats(result, child[0], current.max-current.min)
}

func (c *nfaCompiler) compileRequired(child *node, minimum uint64) (fragment, error) {
	if minimum == 0 {
		return c.compileEmpty()
	}
	result, err := c.compile(child)
	if err != nil {
		return fragment{}, err
	}
	// Repeat bounds are uint64 so an oversized bound fails through c.emit rather
	// than being truncated before the state limit is applied.
	for count := uint64(1); count < minimum; count++ {
		next, err := c.compile(child)
		if err != nil {
			return fragment{}, err
		}
		if err := c.patch(result.outs, next.start); err != nil {
			return fragment{}, err
		}
		result.outs = next.outs
	}
	return result, nil
}

func (c *nfaCompiler) compileUnboundedRepeat(result fragment, child *node) (fragment, error) {
	body, err := c.compile(child)
	if err != nil {
		return fragment{}, err
	}
	split, err := c.emit(instruction{kind: instructionSplit, out: body.start, out1: -1})
	if err != nil {
		return fragment{}, err
	}
	if err := c.patch(result.outs, split); err != nil {
		return fragment{}, err
	}
	if err := c.patch(body.outs, split); err != nil {
		return fragment{}, err
	}
	result.outs = []patch{{pc: split, which: 1}}
	return result, nil
}

// compileOptionalRepeats nests the optional copies, x(x(x)?)?, so each copy
// is reachable only through the copy before it and every skip edge leaves the
// repeat. Chaining them as x?x?x? lets every later copy start from any
// earlier one, so all optional copies stay active even when the input admits
// one iteration count.
func (c *nfaCompiler) compileOptionalRepeats(result fragment, child *node, optional uint64) (fragment, error) {
	var skips []patch
	for count := uint64(0); count < optional; count++ { //nolint:intrange,modernize // preserve uint64 repeat bounds
		body, err := c.compile(child)
		if err != nil {
			return fragment{}, err
		}
		split, err := c.emit(instruction{kind: instructionSplit, out: body.start, out1: -1})
		if err != nil {
			return fragment{}, err
		}
		if err := c.patch(result.outs, split); err != nil {
			return fragment{}, err
		}
		result.outs = body.outs
		skips = append(skips, patch{pc: split, which: 1})
	}
	result.outs = append(skips, result.outs...)
	return result, nil
}

// compileCounter emits enter, head and the body once. Every state from the
// head through the body belongs to the counter.
func (c *nfaCompiler) compileCounter(current, child *node) (fragment, error) {
	enter, err := c.emit(instruction{kind: instructionCountEnter, out: -1, out1: -1})
	if err != nil {
		return fragment{}, err
	}
	head, err := c.emit(instruction{kind: instructionCountHead, out: -1, out1: -1})
	if err != nil {
		return fragment{}, err
	}
	c.states[enter].out = head
	c.counting = false
	body, err := c.compile(child)
	c.counting = true
	if err != nil {
		return fragment{}, err
	}
	if err := c.patch(body.outs, head); err != nil {
		return fragment{}, err
	}
	c.states[head].out = body.start
	// A body that matches the empty string can pad any run to the minimum,
	// so r{m,n} accepts what r{0,n} does, and a zero minimum lets every count
	// be dominated.
	minimum := current.min
	if c.lengths[child].min == 0 {
		minimum = 0
	}
	id := int32(len(c.counters)) //nolint:gosec // counters are bounded by the state limit.
	c.counters = append(c.counters, counter{
		min:       uint32(minimum),
		max:       uint32(min(current.max, uint64(math.MaxUint32))),
		unbounded: current.unbounded,
	})
	c.regions = append(c.regions, counterRegion{first: head, end: len(c.states), counter: id})
	return fragment{start: enter, outs: []patch{{pc: head, which: 1}}}, nil
}

// repeatPlan chooses which repeats compile to a counter. unrolled is the state
// count of the machine with every repeat unrolled, accept state included.
//
// A repeat whose body has one non-zero length stays unrolled when its entry
// position is fixed or its count is exact: input position then fixes the
// iteration count, or no count is ever dominated, so unrolled copies keep one
// copy active per entry and skip the counter's step per iteration.
//
// Unrolling every repeat was rejected: a body that can match more than one
// length, such as a|aa or a?, keeps a copy active per reachable iteration, so
// matching costs input length times bound. Rewriting a nullable body's
// repeat as an unbounded one was rejected because a non-nullable body of
// several lengths stays quadratic. Counting every repeat was rejected because
// the counter's step per iteration made fixed-length repeats, which unrolling
// already matches in linear time, 30 to 60 percent slower. Counters are
// linear only where dominance applies: a count below the minimum is never
// dominated, so (a|aa){400,512} can track up to 400 counts per state
// (Holík et al., "Fast Matching of Regular Patterns with Synchronizing
// Counting", FoSSaCS 2023, arXiv:2301.12851).
type repeatPlan struct {
	counted  map[*node]bool
	lengths  map[*node]matchLength
	unrolled uint64
}

type matchLength struct {
	min, max  uint64
	unbounded bool
}

func (l matchLength) fixed() bool {
	return !l.unbounded && l.min == l.max && l.min != 0
}

func (p *repeatPlan) measure(current *node) matchLength {
	lengths := make([]matchLength, len(current.children))
	for i, child := range current.children {
		lengths[i] = p.measure(child)
	}
	length := combineLengths(current, lengths)
	p.lengths[current] = length
	return length
}

func combineLengths(current *node, children []matchLength) matchLength {
	switch current.kind {
	case nodeSet:
		return matchLength{min: 1, max: 1}
	case nodeConcat:
		return sumLengths(children)
	case nodeAlt:
		return unionLengths(children)
	case nodeRepeat:
		if len(children) != 1 {
			return matchLength{}
		}
		l := children[0]
		return matchLength{
			min:       saturatingMul(current.min, l.min),
			max:       saturatingMul(current.max, l.max),
			unbounded: current.unbounded && l.max != 0 || l.unbounded && current.max != 0,
		}
	case nodeEmpty:
	}
	return matchLength{}
}

func sumLengths(children []matchLength) matchLength {
	var total matchLength
	for _, l := range children {
		total = matchLength{min: saturatingAdd(total.min, l.min), max: saturatingAdd(total.max, l.max), unbounded: total.unbounded || l.unbounded}
	}
	return total
}

func unionLengths(children []matchLength) matchLength {
	if len(children) == 0 {
		return matchLength{}
	}
	total := children[0]
	for _, l := range children[1:] {
		total = matchLength{min: min(total.min, l.min), max: max(total.max, l.max), unbounded: total.unbounded || l.unbounded}
	}
	return total
}

type planCost struct {
	unrolled uint64
	planned  uint64
}

func newRepeatPlan(root *node) repeatPlan {
	plan := repeatPlan{counted: make(map[*node]bool), lengths: make(map[*node]matchLength)}
	plan.measure(root)
	cost := plan.visit(root, entryFixed)
	plan.unrolled = saturatingAdd(cost.unrolled, 1)
	return plan
}

// entryPosition says whether a node always starts at the same input position.
type entryPosition uint8

const (
	entryVaries entryPosition = iota
	entryFixed
)

// Counters do not nest, so a counted repeat's body is charged unrolled.
func (p *repeatPlan) visit(current *node, entry entryPosition) planCost {
	switch current.kind {
	case nodeConcat:
		return p.visitConcat(current.children, entry)
	case nodeAlt:
		return p.visitAlt(current.children, entry)
	case nodeRepeat:
		return p.visitRepeat(current, entry)
	case nodeEmpty, nodeSet:
	}
	return planCost{unrolled: 1, planned: 1}
}

func (p *repeatPlan) visitConcat(children []*node, entry entryPosition) planCost {
	if len(children) == 0 {
		return planCost{unrolled: 1, planned: 1}
	}
	var total planCost
	for _, child := range children {
		cost := p.visit(child, entry)
		if length := p.lengths[child]; length.unbounded || length.min != length.max {
			entry = entryVaries
		}
		total = planCost{unrolled: saturatingAdd(total.unrolled, cost.unrolled), planned: saturatingAdd(total.planned, cost.planned)}
	}
	return total
}

func (p *repeatPlan) visitAlt(children []*node, entry entryPosition) planCost {
	if len(children) == 0 {
		return planCost{unrolled: 1, planned: 1}
	}
	splits := uint64(len(children)) - 1
	total := planCost{unrolled: splits, planned: splits}
	for _, child := range children {
		cost := p.visit(child, entry)
		total = planCost{unrolled: saturatingAdd(total.unrolled, cost.unrolled), planned: saturatingAdd(total.planned, cost.planned)}
	}
	return total
}

func (p *repeatPlan) visitRepeat(current *node, entry entryPosition) planCost {
	if len(current.children) != 1 || current.children[0] == nil {
		return planCost{unrolled: 1, planned: 1}
	}
	child := current.children[0]
	// Each unrolled copy of a fixed-length body starts at a fixed offset; a
	// loop or a body of varying length is entered at many positions.
	fixedBody := p.lengths[child].fixed()
	bodyEntry := entry
	if current.unbounded || !fixedBody {
		bodyEntry = entryVaries
	}
	body := p.visit(child, bodyEntry)
	cost := planCost{unrolled: repeatStates(current, body.unrolled), planned: repeatStates(current, body.planned)}
	if !countable(current) || fixedBody && (entry == entryFixed || !current.unbounded && current.min == current.max) {
		return cost
	}
	if counted := saturatingAdd(body.unrolled, 2); counted < cost.planned {
		p.counted[current] = true
		cost.planned = counted
	}
	return cost
}

// countable reports whether a repeat iterates more than once, so a counter
// can replace copies of its body.
func countable(current *node) bool {
	if current.unbounded {
		return current.min >= 2
	}
	return current.max >= 2
}

// repeatStates mirrors compileRepeat's unrolled emission for a body of body
// states.
func repeatStates(current *node, body uint64) uint64 {
	required := uint64(1)
	if current.min != 0 {
		required = saturatingMul(current.min, body)
	}
	if current.unbounded {
		return saturatingAdd(required, saturatingAdd(body, 1))
	}
	return saturatingAdd(required, saturatingMul(current.max-current.min, saturatingAdd(body, 1)))
}

func saturatingAdd(a, b uint64) uint64 {
	if sum := a + b; sum >= a {
		return sum
	}
	return math.MaxUint64
}

func saturatingMul(a, b uint64) uint64 {
	high, low := bits.Mul64(a, b)
	if high != 0 {
		return math.MaxUint64
	}
	return low
}
