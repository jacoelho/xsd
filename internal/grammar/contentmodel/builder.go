package contentmodel

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/jacoelho/xsd/internal/types"
)

// ParticleAdapter adapts grammar.CompiledParticle for use in contentmodel.
// This avoids an import cycle.
type ParticleAdapter struct {
	Kind      int
	MinOccurs int
	MaxOccurs int
	Element   any // *grammar.CompiledElement
	// AllowSubstitution indicates that substitution groups are allowed (element ref).
	AllowSubstitution bool
	Children          []*ParticleAdapter
	GroupKind         types.GroupKind
	Wildcard          *types.AnyElement
	Original          types.Particle // For symbol matching
}

// Particle kind constants - must match grammar.ParticleKind values.
// Duplicated here to avoid import cycle with grammar package.
const (
	ParticleElement  = 0 // grammar.ParticleElement
	ParticleGroup    = 1 // grammar.ParticleGroup
	ParticleWildcard = 2 // grammar.ParticleWildcard
)

// Builder constructs an Automaton from content model particles using
// Glushkov construction followed by subset construction.
type Builder struct {
	particles          []*ParticleAdapter
	subGroups          map[types.QName]any // []*grammar.CompiledElement
	targetNamespace    string              // Schema target namespace for wildcard matching
	elementFormDefault bool                // true if elementFormDefault="qualified"
	symbolIndexByKey   map[symbolKey]int

	// Construction state
	root                 node
	size                 int // total position count (including end marker)
	endPos               int // position index of end-of-content marker
	positions            []*Position
	followPos            []*bitset
	symbols              []Symbol
	posSymbol            []int // position → symbol index
	symbolMin            []int
	symbolMax            []int
	symbolPositionCounts []int
	groupCounters        map[int]*GroupCounterInfo // position index -> group counter info
	rangeMapPool         []map[int]occRange
	countMapPool         []map[int]int
	bitsetPool           []*bitset
}

// NewBuilder creates a builder for the given content model.
func NewBuilder(particles []*ParticleAdapter, subGroups map[types.QName]any, targetNamespace string, elementFormDefault bool) *Builder {
	return &Builder{
		particles:          particles,
		subGroups:          subGroups,
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
	for pos, symIdx := range b.posSymbol {
		if pos < len(b.positions) && b.positions[pos] != nil {
			if symIdx >= 0 && symIdx < len(b.symbolPositionCounts) {
				b.symbolPositionCounts[symIdx]++
			}
		}
	}

	// subset construction
	return b.construct()
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
		var child node
		switch p.GroupKind {
		case types.Sequence:
			child = b.buildTree(p.Children, nextPos)
		case types.Choice:
			child = b.buildChoice(p.Children, nextPos)
		case types.AllGroup:
			// all groups are handled by AllGroupValidator, not the DFA automaton.
			// the compiler skips automaton building for all groups.
			return nil
		}
		if child == nil {
			return nil
		}
		// for groups with non-trivial minOccurs/maxOccurs, track the positions
		// for counting group iterations
		if (p.MinOccurs > 1) || (p.MaxOccurs != 1 && p.MaxOccurs != types.UnboundedOccurs) {
			// collect first positions (start of iteration) and last positions (end of iteration)
			firstPositions := child.firstPos()
			lastPositions := child.lastPos()

			var firstPosList []int
			var lastPosList []int
			groupID := -1
			firstPosMaxOccurs := 1 // default: each start symbol is one iteration

			firstPositions.forEach(func(pos int) {
				firstPosList = append(firstPosList, pos)
				if groupID < 0 || pos < groupID {
					groupID = pos // use minimum first position as GroupID
				}
				// this is used to compute minimum iterations needed
				if pos < len(b.positions) && b.positions[pos] != nil {
					if b.positions[pos].Max == types.UnboundedOccurs {
						firstPosMaxOccurs = types.UnboundedOccurs
						return
					}
					if firstPosMaxOccurs != types.UnboundedOccurs && b.positions[pos].Max > firstPosMaxOccurs {
						firstPosMaxOccurs = b.positions[pos].Max
					}
				}
			})
			lastPositions.forEach(func(pos int) {
				lastPosList = append(lastPosList, pos)
			})

			// assign GroupCounterInfo to all last positions (states that can be "done")
			unitSize := 0
			if len(firstPosList) == 1 && len(lastPosList) == 1 && firstPosList[0] == lastPosList[0] {
				pos := firstPosList[0]
				if pos < len(b.positions) && b.positions[pos] != nil {
					if b.positions[pos].Min == b.positions[pos].Max && b.positions[pos].Min > 1 {
						unitSize = b.positions[pos].Min
					}
				}
			}
			for _, pos := range lastPosList {
				b.groupCounters[pos] = &GroupCounterInfo{
					Min:               p.MinOccurs,
					Max:               p.MaxOccurs,
					LastPositions:     lastPosList,
					FirstPositions:    firstPosList,
					GroupKind:         p.GroupKind,
					GroupID:           groupID,
					FirstPosMaxOccurs: firstPosMaxOccurs,
					UnitSize:          unitSize,
				}
			}
		}
		// wrap the group with its minOccurs/maxOccurs to allow repetition
		return b.wrapOccurs(child, p.MinOccurs, p.MaxOccurs)
	}
	return nil
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
			key = symbolKeyForParticle(p.Particle, p.AllowSubstitution)
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
				b.symbols = append(b.symbols, b.makeSymbol(p.Particle, p.AllowSubstitution))
			}
		}
		b.posSymbol[i] = idx
	}
	b.symbolIndexByKey = seen
}

// construct performs subset construction to build the DFA.
func (b *Builder) construct() (*Automaton, error) {
	initial := b.getWorkBitset()
	initial.or(b.root.firstPos())

	type workItem struct {
		set *bitset
		id  int
	}

	stateIDs := make(map[string]int, b.size)
	stateIDs[initial.key()] = 0
	worklist := make([]workItem, 1, b.size)
	worklist[0] = workItem{set: initial, id: 0}

	trans := make([][]int, 1, b.size)
	trans[0] = b.newTransRow()
	accepting := make([]bool, 1, b.size)
	accepting[0] = initial.test(b.endPos)
	counting := make([]*Counter, 1, b.size)

	a := &Automaton{
		symbols:         b.symbols,
		trans:           trans,
		accepting:       accepting,
		counting:        counting,
		emptyOK:         initial.test(b.endPos), // empty is OK if initial state is accepting
		symbolMin:       b.symbolMin,
		symbolMax:       b.symbolMax,
		targetNamespace: b.targetNamespace,
		groupCounters:   b.groupCounters,
	}
	posElements := make([]any, len(b.positions))
	for i, pos := range b.positions {
		if pos != nil {
			posElements[i] = pos.Element
		}
	}
	a.posElements = posElements
	a.stateSymbolPos = make([][]int, len(a.trans))
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

	nextBySymbol := make([]*bitset, len(b.symbols))
	usedSymbols := make([]int, 0, len(b.symbols))

	for len(worklist) > 0 {
		cur := worklist[0]
		worklist = worklist[1:]
		curSet := cur.set
		curID := cur.id

		usedSymbols = usedSymbols[:0]
		posRow := make([]int, len(b.symbols))
		for i := range posRow {
			posRow[i] = symbolPosNone
		}
		for wordIdx, w := range curSet.words {
			for w != 0 {
				bit := bits.TrailingZeros64(w)
				pos := wordIdx*64 + bit
				if pos < len(b.positions) && b.positions[pos] != nil {
					symIdx := b.posSymbol[pos]
					if symIdx >= 0 && symIdx < len(nextBySymbol) {
						switch posRow[symIdx] {
						case symbolPosNone:
							posRow[symIdx] = pos
						case symbolPosAmbiguous:
						default:
							if posRow[symIdx] != pos {
								posRow[symIdx] = symbolPosAmbiguous
							}
						}
						next := nextBySymbol[symIdx]
						if next == nil {
							next = newBitset(b.size)
							nextBySymbol[symIdx] = next
							usedSymbols = append(usedSymbols, symIdx)
						}
						next.or(b.followPos[pos])
					}
				}
				w &^= 1 << bit
			}
		}

		for _, symIdx := range usedSymbols {
			next := nextBySymbol[symIdx]
			if next.empty() {
				b.putWorkBitset(next)
				nextBySymbol[symIdx] = nil
				continue
			}

			key := next.key()
			nextID, exists := stateIDs[key]
			if !exists {
				nextID = len(a.trans)
				stateIDs[key] = nextID
				a.trans = append(a.trans, b.newTransRow())
				a.accepting = append(a.accepting, next.test(b.endPos))
				a.counting = append(a.counting, nil)
				a.stateSymbolPos = append(a.stateSymbolPos, nil)
				worklist = append(worklist, workItem{set: next, id: nextID})
			} else {
				b.putWorkBitset(next)
			}

			a.trans[curID][symIdx] = nextID
			nextBySymbol[symIdx] = nil
		}

		a.stateSymbolPos[curID] = posRow
		b.setCounter(a, curID, curSet)
		b.putWorkBitset(curSet)
	}

	return a, nil
}

func (b *Builder) newTransRow() []int {
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
		// check if this is a group completion position
		if groupInfo, isGroupCompletion := b.groupCounters[pos]; isGroupCompletion {
			// collect the symbol indices for completion positions (lastPos)
			var completionSymbols []int
			for _, completionPos := range groupInfo.LastPositions {
				if completionPos < len(b.posSymbol) {
					completionSymbols = append(completionSymbols, b.posSymbol[completionPos])
				}
			}
			// collect the symbol indices for start positions (firstPos)
			var startSymbols []int
			for _, startPos := range groupInfo.FirstPositions {
				if startPos < len(b.posSymbol) {
					startSymbols = append(startSymbols, b.posSymbol[startPos])
				}
			}
			// use GroupID as the counter key so all states share the same counter
			unitSize := 0
			if groupInfo.UnitSize > 0 && len(startSymbols) == 1 && len(completionSymbols) == 1 && startSymbols[0] == completionSymbols[0] {
				if startSymbols[0] >= 0 && startSymbols[0] < len(b.symbolPositionCounts) && b.symbolPositionCounts[startSymbols[0]] == 1 {
					unitSize = groupInfo.UnitSize
				}
			}
			a.counting[stateID] = &Counter{
				Min:                    groupInfo.Min,
				Max:                    groupInfo.Max,
				SymbolIdx:              b.posSymbol[pos],
				IsGroupCounter:         true,
				GroupCompletionSymbols: completionSymbols,
				GroupStartSymbols:      startSymbols,
				GroupID:                groupInfo.GroupID,
				FirstPosMaxOccurs:      groupInfo.FirstPosMaxOccurs,
				UnitSize:               unitSize,
			}
		}
	})
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
	for symIdx, r := range bounds {
		mins[symIdx] = r.min
		maxs[symIdx] = r.max
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
		idx, ok := b.symbolIndexForParticle(p)
		if !ok {
			return nil
		}
		result := b.getRangeMap()
		result[idx] = occRange{min: p.MinOccurs, max: p.MaxOccurs}
		return result
	case ParticleGroup:
		var combined map[int]occRange
		switch p.GroupKind {
		case types.Sequence, types.AllGroup:
			combined = b.getRangeMap()
			for _, child := range p.Children {
				childRanges := b.symbolBoundsForParticle(child)
				if childRanges == nil {
					continue
				}
				mergeSequenceRanges(combined, childRanges)
				b.putRangeMap(childRanges)
			}
		case types.Choice:
			combined = b.getRangeMap()
			counts := b.getCountMap()
			childCount := 0
			for _, child := range p.Children {
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
		default:
			combined = b.getRangeMap()
			for _, child := range p.Children {
				childRanges := b.symbolBoundsForParticle(child)
				if childRanges == nil {
					continue
				}
				mergeSequenceRanges(combined, childRanges)
				b.putRangeMap(childRanges)
			}
		}
		return applyGroupOccursInPlace(combined, p.MinOccurs, p.MaxOccurs)
	default:
		return nil
	}
}

func (b *Builder) symbolIndexForParticle(p *ParticleAdapter) (int, bool) {
	if p.Original == nil {
		return 0, false
	}
	key := symbolKeyForParticle(p.Original, p.AllowSubstitution)
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

type symbolKey struct {
	kind              symbolKeyKind
	allowSubstitution bool
	qname             types.QName
	wildcardNS        types.NamespaceConstraint
	wildcardTarget    types.NamespaceURI
	wildcardList      string
	groupID           int
}

// Symbol helpers
func symbolKeyForParticle(p types.Particle, allowSubstitution bool) symbolKey {
	switch v := p.(type) {
	case *types.ElementDecl:
		return symbolKey{
			kind:              symbolKeyElement,
			allowSubstitution: allowSubstitution,
			qname:             v.Name,
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

func (b *Builder) makeSymbol(p types.Particle, allowSubstitution bool) Symbol {
	switch v := p.(type) {
	case *types.ElementDecl:
		qname := v.Name
		if !b.isElementQualified(v) {
			// unqualified local elements should match elements with no namespace
			qname = types.QName{Namespace: "", Local: v.Name.Local}
		}
		return Symbol{Kind: KindElement, QName: qname, AllowSubstitution: allowSubstitution}
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
	default: // FormDefault - use schema's elementFormDefault
		return b.elementFormDefault
	}
}
