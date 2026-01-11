package grammar

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/types"
)

// ParticleAdapter adapts CompiledParticle for automaton construction.
type ParticleAdapter struct {
	Original          types.Particle
	Element           *CompiledElement
	Wildcard          *types.AnyElement
	Children          []*ParticleAdapter
	Kind              ParticleKind
	MinOccurs         int
	MaxOccurs         int
	GroupKind         types.GroupKind
	AllowSubstitution bool
}

// Builder constructs an Automaton from content model particles using
// Glushkov construction followed by subset construction.
type Builder struct {
	root                 node
	groupCounters        map[int]*GroupCounterInfo
	symbolIndexByKey     map[symbolKey]int
	targetNamespace      string
	followPos            []*bitset
	symbolMin            []int
	bitsetPool           []*bitset
	positions            []*Position
	particles            []*ParticleAdapter
	symbols              []Symbol
	posSymbol            []int
	countMapPool         []map[int]int
	symbolMax            []int
	symbolPositionCounts []int
	rangeMapPool         []map[int]occRange
	size                 int
	endPos               int
	elementFormDefault   types.FormChoice
}

type workItem struct {
	set *bitset
	id  int
}

type constructionState struct {
	automaton    *Automaton
	stateIDs     map[string]int
	worklist     []workItem
	nextBySymbol []*bitset
}

func recordSymbolPosition(posRow []int, symbolIndex, pos int) {
	switch posRow[symbolIndex] {
	case symbolPosNone:
		posRow[symbolIndex] = pos
	case symbolPosAmbiguous:
		return
	default:
		if posRow[symbolIndex] != pos {
			posRow[symbolIndex] = symbolPosAmbiguous
		}
	}
}

// NewBuilder creates a builder for the given content model.
func NewBuilder(particles []*ParticleAdapter, targetNamespace string, elementFormDefault types.FormChoice) *Builder {
	return &Builder{
		particles:          particles,
		targetNamespace:    targetNamespace,
		elementFormDefault: elementFormDefault,
		groupCounters:      make(map[int]*GroupCounterInfo),
	}
}

// Build constructs the automaton. Returns an error if construction fails.
func (b *Builder) Build() (*Automaton, error) {
	b.size = b.countLeaves(b.particles) + 1 // +1 for end marker
	b.endPos = b.size - 1
	b.positions = make([]*Position, b.size)
	b.followPos = make([]*bitset, b.size)
	for i := range b.followPos {
		b.followPos[i] = newBitset(b.size)
	}

	var nextPos int
	content := b.buildTree(b.particles, &nextPos)
	if content == nil {
		// empty content model
		return &Automaton{emptyOK: true}, nil
	}

	// append end marker: content · end
	endLeaf := newLeaf(b.endPos, nil, 1, 1, b.size)
	b.root = newSeq(content, endLeaf, b.size)

	// compute followPos (firstPos/lastPos computed lazily)
	b.computeFollowPos(b.root)

	content.lastPos().forEach(func(pos int) {
		b.followPos[pos].or(endLeaf.firstPos())
	})

	b.buildSymbols()
	b.symbolMin, b.symbolMax = b.computeSymbolBounds()
	b.symbolPositionCounts = make([]int, len(b.symbols))
	for pos, symbolIndex := range b.posSymbol {
		if pos < len(b.positions) && b.positions[pos] != nil {
			if symbolIndex >= 0 && symbolIndex < len(b.symbolPositionCounts) {
				b.symbolPositionCounts[symbolIndex]++
			}
		}
	}

	// subset construction
	return b.construct()
}

// BuildAutomaton builds an automaton from compiled particles.
// BuildAutomaton constructs an automaton using the schema's elementFormDefault.
func BuildAutomaton(particles []*CompiledParticle, targetNamespace types.NamespaceURI, elementFormDefault types.FormChoice) (*Automaton, error) {
	adapters := make([]*ParticleAdapter, len(particles))
	for i, p := range particles {
		adapters[i] = convertParticle(p)
	}

	builder := NewBuilder(adapters, string(targetNamespace), elementFormDefault)
	return builder.Build()
}

// convertParticle converts a CompiledParticle to a ParticleAdapter.
func convertParticle(p *CompiledParticle) *ParticleAdapter {
	adapter := &ParticleAdapter{
		Kind:              p.Kind,
		MinOccurs:         p.MinOccurs,
		MaxOccurs:         p.MaxOccurs,
		GroupKind:         p.GroupKind,
		Wildcard:          p.Wildcard,
		AllowSubstitution: p.IsReference,
	}

	switch p.Kind {
	case ParticleElement:
		adapter.Element = p.Element
		if p.Element != nil && p.Element.Original != nil {
			adapter.Original = p.Element.Original
		}
	case ParticleWildcard:
		adapter.Original = p.Wildcard
	}

	if len(p.Children) > 0 {
		adapter.Children = make([]*ParticleAdapter, len(p.Children))
		for i, child := range p.Children {
			adapter.Children[i] = convertParticle(child)
		}
	}

	return adapter
}

// buildTree builds the syntax tree from particles.
func (b *Builder) buildTree(particles []*ParticleAdapter, nextPos *int) node {
	if len(particles) == 0 {
		return nil
	}
	if len(particles) == 1 {
		return b.buildNode(particles[0], nextPos)
	}

	// build right-associative sequence: a · (b · c)
	left := b.buildNode(particles[0], nextPos)
	right := b.buildTree(particles[1:], nextPos)
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	return newSeq(left, right, b.size)
}

// buildNode creates a syntax tree node for a single particle.
func (b *Builder) buildNode(p *ParticleAdapter, nextPos *int) node {
	switch p.Kind {
	case ParticleElement, ParticleWildcard:
		pos := *nextPos
		*nextPos++

		particle := p.Original

		b.positions[pos] = &Position{
			Index:             pos,
			Particle:          particle,
			Min:               p.MinOccurs,
			Max:               p.MaxOccurs,
			AllowSubstitution: p.AllowSubstitution,
			Element:           p.Element,
		}

		leaf := newLeaf(pos, particle, p.MinOccurs, p.MaxOccurs, b.size)
		return b.wrapOccurs(leaf, p.MinOccurs, p.MaxOccurs)

	case ParticleGroup:
		child := b.buildGroupChild(p, nextPos)
		if child == nil {
			return nil
		}
		b.attachGroupCounter(child, p)
		return b.wrapOccurs(child, p.MinOccurs, p.MaxOccurs)
	}
	return nil
}

func (b *Builder) buildGroupChild(p *ParticleAdapter, nextPos *int) node {
	switch p.GroupKind {
	case types.Sequence:
		return b.buildTree(p.Children, nextPos)
	case types.Choice:
		return b.buildChoice(p.Children, nextPos)
	case types.AllGroup:
		// all groups are handled by AllGroupValidator, not the DFA automaton.
		// the compiler skips automaton building for all groups.
		return nil
	default:
		return nil
	}
}

func (b *Builder) attachGroupCounter(child node, p *ParticleAdapter) {
	if !b.needsGroupCounter(p) {
		return
	}

	firstPosList := b.collectPositions(child.firstPos())
	lastPosList := b.collectPositions(child.lastPos())
	firstPosMaxOccurs := b.computeFirstPosMaxOccurs(firstPosList)

	info := &GroupCounterInfo{
		Min:               p.MinOccurs,
		Max:               p.MaxOccurs,
		LastPositions:     lastPosList,
		FirstPositions:    firstPosList,
		GroupKind:         p.GroupKind,
		GroupID:           b.computeGroupID(firstPosList),
		FirstPosMaxOccurs: firstPosMaxOccurs,
		UnitSize:          b.computeUnitSize(firstPosList, lastPosList),
	}

	for _, pos := range lastPosList {
		b.groupCounters[pos] = info
	}
}

func (b *Builder) needsGroupCounter(p *ParticleAdapter) bool {
	return (p.MinOccurs > 1) || (p.MaxOccurs != 1 && p.MaxOccurs != types.UnboundedOccurs)
}

func (b *Builder) collectPositions(positions *bitset) []int {
	if positions == nil {
		return nil
	}
	result := make([]int, 0, positions.n)
	positions.forEach(func(pos int) {
		result = append(result, pos)
	})
	return result
}

func (b *Builder) computeGroupID(firstPosList []int) int {
	if len(firstPosList) == 0 {
		return -1
	}
	groupID := firstPosList[0]
	for _, pos := range firstPosList[1:] {
		if pos < groupID {
			groupID = pos
		}
	}
	return groupID
}

func (b *Builder) computeFirstPosMaxOccurs(firstPosList []int) int {
	maxOccurs := 1
	for _, pos := range firstPosList {
		if pos >= len(b.positions) || b.positions[pos] == nil {
			continue
		}
		posMax := b.positions[pos].Max
		if posMax == types.UnboundedOccurs {
			return types.UnboundedOccurs
		}
		if maxOccurs != types.UnboundedOccurs && posMax > maxOccurs {
			maxOccurs = posMax
		}
	}
	return maxOccurs
}

func (b *Builder) computeUnitSize(firstPosList, lastPosList []int) int {
	if len(firstPosList) != 1 || len(lastPosList) != 1 {
		return 0
	}
	if firstPosList[0] != lastPosList[0] {
		return 0
	}
	pos := firstPosList[0]
	if pos >= len(b.positions) || b.positions[pos] == nil {
		return 0
	}
	position := b.positions[pos]
	if position.Min == position.Max && position.Min > 1 {
		return position.Min
	}
	return 0
}

// buildChoice builds alternation: a | b | c
func (b *Builder) buildChoice(particles []*ParticleAdapter, nextPos *int) node {
	if len(particles) == 0 {
		return nil
	}
	result := b.buildNode(particles[0], nextPos)
	for i := 1; i < len(particles); i++ {
		right := b.buildNode(particles[i], nextPos)
		if right == nil {
			continue
		}
		if result == nil {
			result = right
			continue
		}
		result = newAlt(result, right, b.size)
	}
	return result
}

// wrapOccurs applies occurrence constraints to a node.
func (b *Builder) wrapOccurs(n node, min, max int) node {
	if n == nil {
		return nil
	}
	if max == 0 {
		return nil
	}
	switch {
	case min == 1 && max == 1:
		return n
	case min == 0 && max == 1:
		return newOpt(n, b.size)
	case min == 0 && max == types.UnboundedOccurs:
		return newStar(n, b.size)
	case min == 1 && max == types.UnboundedOccurs:
		return newPlus(n, b.size)
	case min == 0:
		// min=0 with bounded max - treat as optional star
		return newStar(n, b.size)
	case min >= 1:
		// min >= 1 with bounded max - use plus (not nullable)
		// counting constraints handle the actual min/max
		return newPlus(n, b.size)
	default:
		// fallback (shouldn't reach here normally)
		return newPlus(n, b.size)
	}
}

// buildSymbols creates the symbol alphabet.
func (b *Builder) buildSymbols() {
	seen := make(map[symbolKey]int, b.endPos)
	b.posSymbol = make([]int, b.size)
	b.symbols = make([]Symbol, 0, b.endPos)

	for i := 0; i < b.endPos; i++ {
		p := b.positions[i]
		if p == nil {
			continue
		}
		// completion positions (Particle == nil) need a special symbol
		var key symbolKey
		if p.Particle == nil {
			// this is a group completion position - use unique key
			key = symbolKey{kind: symbolKeyGroupCompletion, groupID: i}
		} else {
			key = symbolKeyForParticle(p.Particle, substitutionPolicyFor(p.AllowSubstitution))
		}
		idx, ok := seen[key]
		if !ok {
			idx = len(b.symbols)
			seen[key] = idx
			if p.Particle == nil {
				// use a dummy symbol that won't match any element
				b.symbols = append(b.symbols, Symbol{
					Kind:  KindAny, // use KindAny as a placeholder - it won't match elements
					QName: types.QName{Local: fmt.Sprintf("__group_completion_%d", i)},
				})
			} else {
				b.symbols = append(b.symbols, b.makeSymbol(p.Particle, substitutionPolicyFor(p.AllowSubstitution)))
			}
		}
		b.posSymbol[i] = idx
	}
	b.symbolIndexByKey = seen
}

func (b *Builder) scanPositionsForTransitions(state *bitset, nextBySymbol []*bitset) ([]int, []int) {
	posRow := make([]int, len(b.symbols))
	for i := range posRow {
		posRow[i] = symbolPosNone
	}
	usedSymbols := make([]int, 0, len(b.symbols))

	state.forEach(func(pos int) {
		if pos >= len(b.positions) || b.positions[pos] == nil {
			return
		}
		symbolIndex := b.posSymbol[pos]
		if !inBounds(symbolIndex, len(nextBySymbol)) {
			return
		}

		recordSymbolPosition(posRow, symbolIndex, pos)
		usedSymbols = b.accumulateFollowSet(nextBySymbol, usedSymbols, symbolIndex, pos)
	})

	return posRow, usedSymbols
}

func (b *Builder) accumulateFollowSet(nextBySymbol []*bitset, usedSymbols []int, symbolIndex, pos int) []int {
	next := nextBySymbol[symbolIndex]
	if next == nil {
		next = newBitset(b.size)
		nextBySymbol[symbolIndex] = next
		usedSymbols = append(usedSymbols, symbolIndex)
	}
	next.or(b.followPos[pos])
	return usedSymbols
}

func (b *Builder) initializeConstruction() *constructionState {
	initial := b.getWorkBitset()
	initial.or(b.root.firstPos())

	stateIDs := make(map[string]int, b.size)
	stateIDs[initial.key()] = 0
	worklist := make([]workItem, 1, b.size)
	worklist[0] = workItem{set: initial, id: 0}

	a := &Automaton{
		symbols:         b.symbols,
		transitions:     append([]int(nil), b.newTransitionRow()...),
		accepting:       []bool{initial.test(b.endPos)},
		counting:        make([]*Counter, 1, b.size),
		emptyOK:         initial.test(b.endPos),
		symbolMin:       b.symbolMin,
		symbolMax:       b.symbolMax,
		targetNamespace: b.targetNamespace,
		groupCounters:   b.groupCounters,
		stateSymbolPos:  make([][]int, 1, b.size),
	}
	b.initializeAutomatonMaps(a)

	return &constructionState{
		automaton:    a,
		stateIDs:     stateIDs,
		worklist:     worklist,
		nextBySymbol: make([]*bitset, len(b.symbols)),
	}
}

func (b *Builder) initializeAutomatonMaps(a *Automaton) {
	posElements := make([]*CompiledElement, len(b.positions))
	for i, pos := range b.positions {
		if pos != nil {
			posElements[i] = pos.Element
		}
	}
	a.posElements = posElements
	if len(a.groupCounters) > 0 {
		a.groupIndexByID = make(map[int]int)
		for _, info := range a.groupCounters {
			if _, ok := a.groupIndexByID[info.GroupID]; ok {
				continue
			}
			a.groupIndexByID[info.GroupID] = len(a.groupIndexByID)
		}
		a.groupCount = len(a.groupIndexByID)
	}
}

func (b *Builder) processSymbolTransitions(a *Automaton, currentStateID int, usedSymbols []int, nextBySymbol []*bitset, stateIDs map[string]int, worklist []workItem) []workItem {
	var nextID int
	for _, symbolIndex := range usedSymbols {
		next := nextBySymbol[symbolIndex]
		if next.empty() {
			b.putWorkBitset(next)
			nextBySymbol[symbolIndex] = nil
			continue
		}

		nextID, worklist = b.getOrCreateState(a, next, stateIDs, worklist)
		a.setTransition(currentStateID, symbolIndex, nextID)
		nextBySymbol[symbolIndex] = nil
	}
	return worklist
}

func (b *Builder) getOrCreateState(a *Automaton, stateSet *bitset, stateIDs map[string]int, worklist []workItem) (int, []workItem) {
	key := stateSet.key()
	stateID, exists := stateIDs[key]
	if exists {
		b.putWorkBitset(stateSet)
		return stateID, worklist
	}

	stateID = len(a.accepting)
	stateIDs[key] = stateID
	a.transitions = append(a.transitions, b.newTransitionRow()...)
	a.accepting = append(a.accepting, stateSet.test(b.endPos))
	a.counting = append(a.counting, nil)
	a.stateSymbolPos = append(a.stateSymbolPos, nil)
	worklist = append(worklist, workItem{set: stateSet, id: stateID})
	return stateID, worklist
}

// construct performs subset construction to build the DFA.
func (b *Builder) construct() (*Automaton, error) {
	cs := b.initializeConstruction()

	for len(cs.worklist) > 0 {
		cur := cs.worklist[0]
		cs.worklist = cs.worklist[1:]

		posRow, usedSymbols := b.scanPositionsForTransitions(cur.set, cs.nextBySymbol)

		cs.worklist = b.processSymbolTransitions(
			cs.automaton,
			cur.id,
			usedSymbols,
			cs.nextBySymbol,
			cs.stateIDs,
			cs.worklist,
		)

		cs.automaton.stateSymbolPos[cur.id] = posRow
		b.setCounter(cs.automaton, cur.id, cur.set)
		b.putWorkBitset(cur.set)
	}

	return cs.automaton, nil
}

func (b *Builder) newTransitionRow() []int {
	row := make([]int, len(b.symbols))
	for i := range row {
		row[i] = -1 // invalid transition
	}
	return row
}

func (b *Builder) setCounter(a *Automaton, stateID int, state *bitset) {
	state.forEach(func(pos int) {
		if pos >= len(b.positions) || b.positions[pos] == nil {
			return
		}
		groupInfo, ok := b.groupCounters[pos]
		if !ok {
			return
		}
		a.counting[stateID] = b.groupCounterForPosition(pos, groupInfo)
	})
}

func (b *Builder) groupCounterForPosition(pos int, groupInfo *GroupCounterInfo) *Counter {
	completionSymbols := b.symbolsForPositions(groupInfo.LastPositions)
	startSymbols := b.symbolsForPositions(groupInfo.FirstPositions)
	unitSize := b.groupUnitSize(groupInfo, startSymbols, completionSymbols)
	return &Counter{
		Min:                    groupInfo.Min,
		Max:                    groupInfo.Max,
		SymbolIndex:            b.posSymbol[pos],
		IsGroupCounter:         true,
		GroupCompletionSymbols: completionSymbols,
		GroupStartSymbols:      startSymbols,
		GroupID:                groupInfo.GroupID,
		FirstPosMaxOccurs:      groupInfo.FirstPosMaxOccurs,
		UnitSize:               unitSize,
	}
}

func (b *Builder) symbolsForPositions(positions []int) []int {
	if len(positions) == 0 {
		return nil
	}
	symbols := make([]int, 0, len(positions))
	for _, pos := range positions {
		if pos < len(b.posSymbol) {
			symbols = append(symbols, b.posSymbol[pos])
		}
	}
	return symbols
}

func (b *Builder) groupUnitSize(groupInfo *GroupCounterInfo, startSymbols, completionSymbols []int) int {
	if groupInfo.UnitSize <= 0 {
		return 0
	}
	if len(startSymbols) != 1 || len(completionSymbols) != 1 {
		return 0
	}
	if startSymbols[0] != completionSymbols[0] {
		return 0
	}
	symbolIndex := startSymbols[0]
	if !inBounds(symbolIndex, len(b.symbolPositionCounts)) || b.symbolPositionCounts[symbolIndex] != 1 {
		return 0
	}
	return groupInfo.UnitSize
}

func (b *Builder) countLeaves(particles []*ParticleAdapter) int {
	n := 0
	for _, p := range particles {
		switch p.Kind {
		case ParticleElement, ParticleWildcard:
			n++
		case ParticleGroup:
			n += b.countLeaves(p.Children)
		}
	}
	return n
}

type occRange struct {
	min int
	max int
}

func (b *Builder) getRangeMap() map[int]occRange {
	n := len(b.rangeMapPool)
	if n == 0 {
		return make(map[int]occRange)
	}
	m := b.rangeMapPool[n-1]
	b.rangeMapPool = b.rangeMapPool[:n-1]
	return m
}

func (b *Builder) putRangeMap(m map[int]occRange) {
	if m == nil {
		return
	}
	clear(m)
	b.rangeMapPool = append(b.rangeMapPool, m)
}

func (b *Builder) getCountMap() map[int]int {
	n := len(b.countMapPool)
	if n == 0 {
		return make(map[int]int)
	}
	m := b.countMapPool[n-1]
	b.countMapPool = b.countMapPool[:n-1]
	return m
}

func (b *Builder) putCountMap(m map[int]int) {
	if m == nil {
		return
	}
	clear(m)
	b.countMapPool = append(b.countMapPool, m)
}

func (b *Builder) getWorkBitset() *bitset {
	n := len(b.bitsetPool)
	if n == 0 {
		return newBitset(b.size)
	}
	bs := b.bitsetPool[n-1]
	b.bitsetPool = b.bitsetPool[:n-1]
	bs.clear()
	return bs
}

func (b *Builder) putWorkBitset(bs *bitset) {
	if bs == nil {
		return
	}
	bs.clear()
	b.bitsetPool = append(b.bitsetPool, bs)
}

func (b *Builder) computeSymbolBounds() ([]int, []int) {
	bounds := b.symbolBoundsForParticles(b.particles)
	mins := make([]int, len(b.symbols))
	maxs := make([]int, len(b.symbols))
	for i := range maxs {
		maxs[i] = types.UnboundedOccurs
	}
	for symbolIndex, r := range bounds {
		mins[symbolIndex] = r.min
		maxs[symbolIndex] = r.max
	}
	b.putRangeMap(bounds)
	return mins, maxs
}

func (b *Builder) symbolBoundsForParticles(particles []*ParticleAdapter) map[int]occRange {
	result := b.getRangeMap()
	for _, p := range particles {
		child := b.symbolBoundsForParticle(p)
		if child == nil {
			continue
		}
		mergeSequenceRanges(result, child)
		b.putRangeMap(child)
	}
	return result
}

func (b *Builder) symbolBoundsForParticle(p *ParticleAdapter) map[int]occRange {
	switch p.Kind {
	case ParticleElement, ParticleWildcard:
		return b.boundsForLeafParticle(p)
	case ParticleGroup:
		return b.boundsForGroupParticle(p)
	default:
		return nil
	}
}

func (b *Builder) boundsForLeafParticle(p *ParticleAdapter) map[int]occRange {
	idx, ok := b.symbolIndexForParticle(p)
	if !ok {
		return nil
	}
	result := b.getRangeMap()
	result[idx] = occRange{min: p.MinOccurs, max: p.MaxOccurs}
	return result
}

func (b *Builder) boundsForGroupParticle(p *ParticleAdapter) map[int]occRange {
	var combined map[int]occRange
	switch p.GroupKind {
	case types.Sequence, types.AllGroup:
		combined = b.mergeChildRangesSequentially(p.Children)
	case types.Choice:
		combined = b.mergeChildRangesAsChoice(p.Children)
	default:
		combined = b.mergeChildRangesSequentially(p.Children)
	}
	return applyGroupOccursInPlace(combined, p.MinOccurs, p.MaxOccurs)
}

func (b *Builder) mergeChildRangesSequentially(children []*ParticleAdapter) map[int]occRange {
	combined := b.getRangeMap()
	for _, child := range children {
		childRanges := b.symbolBoundsForParticle(child)
		if childRanges == nil {
			continue
		}
		mergeSequenceRanges(combined, childRanges)
		b.putRangeMap(childRanges)
	}
	return combined
}

func (b *Builder) mergeChildRangesAsChoice(children []*ParticleAdapter) map[int]occRange {
	combined := b.getRangeMap()
	counts := b.getCountMap()
	childCount := 0
	for _, child := range children {
		childCount++
		childRanges := b.symbolBoundsForParticle(child)
		if childRanges == nil {
			continue
		}
		mergeChoiceRanges(combined, counts, childRanges)
		b.putRangeMap(childRanges)
	}
	finalizeChoiceRanges(combined, counts, childCount)
	b.putCountMap(counts)
	return combined
}

func (b *Builder) symbolIndexForParticle(p *ParticleAdapter) (int, bool) {
	if p.Original == nil {
		return 0, false
	}
	key := symbolKeyForParticle(p.Original, substitutionPolicyFor(p.AllowSubstitution))
	idx, ok := b.symbolIndexByKey[key]
	return idx, ok
}

func mergeSequenceRanges(dst, src map[int]occRange) {
	for key, val := range src {
		cur := dst[key]
		dst[key] = occRange{
			min: cur.min + val.min,
			max: sumMax(cur.max, val.max),
		}
	}
}

func mergeChoiceRanges(dst map[int]occRange, counts map[int]int, src map[int]occRange) {
	for key, val := range src {
		cur, ok := dst[key]
		if !ok {
			dst[key] = occRange{min: val.min, max: val.max}
		} else {
			if val.min < cur.min {
				cur.min = val.min
			}
			if cur.max == types.UnboundedOccurs || val.max == types.UnboundedOccurs {
				cur.max = types.UnboundedOccurs
			} else if val.max > cur.max {
				cur.max = val.max
			}
			dst[key] = cur
		}
		counts[key]++
	}
}

func finalizeChoiceRanges(dst map[int]occRange, counts map[int]int, childCount int) {
	if childCount <= 1 {
		return
	}
	for key, r := range dst {
		if counts[key] < childCount {
			r.min = 0
			dst[key] = r
		}
	}
}

func applyGroupOccursInPlace(ranges map[int]occRange, groupMin, groupMax int) map[int]occRange {
	if ranges == nil || (groupMin == 1 && groupMax == 1) {
		return ranges
	}
	for key, r := range ranges {
		r.min = r.min * groupMin
		r.max = multiplyMax(r.max, groupMax)
		ranges[key] = r
	}
	return ranges
}

func sumMax(a, b int) int {
	if a == types.UnboundedOccurs || b == types.UnboundedOccurs {
		return types.UnboundedOccurs
	}
	return a + b
}

func multiplyMax(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	if a == types.UnboundedOccurs || b == types.UnboundedOccurs {
		return types.UnboundedOccurs
	}
	return a * b
}

type symbolKeyKind uint8

const (
	symbolKeyElement symbolKeyKind = iota
	symbolKeyAny
	symbolKeyGroupCompletion
)

type substitutionPolicy int

const (
	substitutionDisallowed substitutionPolicy = iota
	substitutionAllowed
)

type symbolKey struct {
	qname          types.QName
	wildcardTarget types.NamespaceURI
	wildcardList   string
	wildcardNS     types.NamespaceConstraint
	groupID        int
	kind           symbolKeyKind
	substitution   substitutionPolicy
}

// Symbol helpers
func symbolKeyForParticle(p types.Particle, substitution substitutionPolicy) symbolKey {
	switch v := p.(type) {
	case *types.ElementDecl:
		return symbolKey{
			kind:         symbolKeyElement,
			substitution: substitution,
			qname:        v.Name,
		}
	case *types.AnyElement:
		key := symbolKey{
			kind:           symbolKeyAny,
			wildcardNS:     v.Namespace,
			wildcardTarget: v.TargetNamespace,
		}
		if v.Namespace == types.NSCList {
			key.wildcardList = namespaceListKey(v.NamespaceList)
		}
		return key
	default:
		return symbolKey{kind: symbolKeyAny}
	}
}

func substitutionPolicyFor(allow bool) substitutionPolicy {
	if allow {
		return substitutionAllowed
	}
	return substitutionDisallowed
}

func namespaceListKey(list []types.NamespaceURI) string {
	if len(list) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, ns := range list {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(string(ns))
	}
	return sb.String()
}

func (b *Builder) makeSymbol(p types.Particle, substitution substitutionPolicy) Symbol {
	switch v := p.(type) {
	case *types.ElementDecl:
		qname := v.Name
		if !b.isElementQualified(v) {
			// unqualified local elements should match elements with no namespace
			qname = types.QName{Namespace: "", Local: v.Name.Local}
		}
		return Symbol{Kind: KindElement, QName: qname, AllowSubstitution: substitution == substitutionAllowed}
	case *types.AnyElement:
		switch v.Namespace {
		case types.NSCAny:
			return Symbol{Kind: KindAny}
		case types.NSCOther:
			// ##other - matches any namespace except target namespace
			return Symbol{Kind: KindAnyOther, NS: v.TargetNamespace.String()}
		case types.NSCTargetNamespace:
			// ##targetNamespace - matches only target namespace
			return Symbol{Kind: KindAnyNS, NS: v.TargetNamespace.String()}
		case types.NSCLocal:
			// ##local - matches empty namespace
			return Symbol{Kind: KindAnyNS, NS: ""}
		case types.NSCList:
			// explicit namespace list - only elements from listed namespaces match
			return Symbol{Kind: KindAnyNSList, NSList: v.NamespaceList}
		default:
			return Symbol{Kind: KindAny}
		}
	default:
		return Symbol{Kind: KindAny}
	}
}

// isElementQualified determines if an element should be namespace-qualified in instances.
// Global elements are always qualified. Local elements depend on their form attribute
// or the schema's elementFormDefault.
func (b *Builder) isElementQualified(elem *types.ElementDecl) bool {
	switch elem.Form {
	case types.FormQualified:
		return true
	case types.FormUnqualified:
		return false
	default: // formDefault uses schema's elementFormDefault
		return b.elementFormDefault == types.FormQualified
	}
}
