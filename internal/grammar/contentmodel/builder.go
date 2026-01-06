package contentmodel

import (
	"fmt"
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
	symbolIndexByKey   map[string]int

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
		// Empty content model
		return &Automaton{emptyOK: true}, nil
	}

	// Append end marker: content · end
	endLeaf := newLeaf(b.endPos, nil, 1, 1, b.size)
	b.root = newSeq(content, endLeaf, b.size)

	// Compute followPos (firstPos/lastPos computed lazily)
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

	// Subset construction
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

	// Build right-associative sequence: a · (b · c)
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
			// All groups are handled by AllGroupValidator, not the DFA automaton.
			// The compiler skips automaton building for all groups.
			return nil
		}
		if child == nil {
			return nil
		}
		// For groups with non-trivial minOccurs/maxOccurs, track the positions
		// for counting group iterations
		if (p.MinOccurs > 1) || (p.MaxOccurs != 1 && p.MaxOccurs != types.UnboundedOccurs) {
			// Collect first positions (start of iteration) and last positions (end of iteration)
			firstPositions := child.firstPos()
			lastPositions := child.lastPos()

			var firstPosList []int
			var lastPosList []int
			groupID := -1
			firstPosMaxOccurs := 1 // Default: each start symbol is one iteration

			firstPositions.forEach(func(pos int) {
				firstPosList = append(firstPosList, pos)
				if groupID < 0 || pos < groupID {
					groupID = pos // Use minimum first position as GroupID
				}
				// This is used to compute minimum iterations needed
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

			// Assign GroupCounterInfo to all last positions (states that can be "done")
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
		// Wrap the group with its minOccurs/maxOccurs to allow repetition
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
		// Counting constraints handle the actual min/max
		return newPlus(n, b.size)
	default:
		// Fallback (shouldn't reach here normally)
		return newPlus(n, b.size)
	}
}

// buildSymbols creates the symbol alphabet.
func (b *Builder) buildSymbols() {
	seen := make(map[string]int)
	b.posSymbol = make([]int, b.size)

	for i := 0; i < b.endPos; i++ {
		p := b.positions[i]
		if p == nil {
			continue
		}
		// Completion positions (Particle == nil) need a special symbol
		var key string
		if p.Particle == nil {
			// This is a group completion position - use unique key
			key = fmt.Sprintf("__group_completion_%d", i)
		} else {
			key = symbolKey(p.Particle, p.AllowSubstitution)
		}
		idx, ok := seen[key]
		if !ok {
			idx = len(b.symbols)
			seen[key] = idx
			if p.Particle == nil {
				// Use a dummy symbol that won't match any element
				b.symbols = append(b.symbols, Symbol{
					Kind:  KindAny, // Use KindAny as a placeholder - it won't match elements
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
	initial := b.root.firstPos()

	stateIDs := make(map[string]int)
	stateIDs[initial.String()] = 0
	worklist := []*bitset{initial}

	a := &Automaton{
		symbols:         b.symbols,
		trans:           [][]int{b.newTransRow()},
		accepting:       []bool{initial.test(b.endPos)},
		counting:        []*Counter{nil},
		emptyOK:         initial.test(b.endPos), // Empty is OK if initial state is accepting
		symbolMin:       b.symbolMin,
		symbolMax:       b.symbolMax,
		targetNamespace: b.targetNamespace,
		groupCounters:   b.groupCounters,
	}

	for len(worklist) > 0 {
		cur := worklist[0]
		worklist = worklist[1:]
		curID := stateIDs[cur.String()]

		for symIdx := range b.symbols {
			next := newBitset(b.size)
			cur.forEach(func(pos int) {
				if pos < len(b.posSymbol) && b.posSymbol[pos] == symIdx {
					next.or(b.followPos[pos])
				}
			})

			if next.empty() {
				continue
			}

			key := next.String()
			nextID, exists := stateIDs[key]
			if !exists {
				nextID = len(a.trans)
				stateIDs[key] = nextID
				a.trans = append(a.trans, b.newTransRow())
				a.accepting = append(a.accepting, next.test(b.endPos))
				a.counting = append(a.counting, nil)
				worklist = append(worklist, next)
			}

			a.trans[curID][symIdx] = nextID
		}

		b.setCounter(a, curID, cur)
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
		// Check if this is a group completion position
		if groupInfo, isGroupCompletion := b.groupCounters[pos]; isGroupCompletion {
			// Collect the symbol indices for completion positions (lastPos)
			var completionSymbols []int
			for _, completionPos := range groupInfo.LastPositions {
				if completionPos < len(b.posSymbol) {
					completionSymbols = append(completionSymbols, b.posSymbol[completionPos])
				}
			}
			// Collect the symbol indices for start positions (firstPos)
			var startSymbols []int
			for _, startPos := range groupInfo.FirstPositions {
				if startPos < len(b.posSymbol) {
					startSymbols = append(startSymbols, b.posSymbol[startPos])
				}
			}
			// Use GroupID as the counter key so all states share the same counter
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
	return mins, maxs
}

func (b *Builder) symbolBoundsForParticles(particles []*ParticleAdapter) map[int]occRange {
	result := make(map[int]occRange)
	for _, p := range particles {
		child := b.symbolBoundsForParticle(p)
		for key, r := range child {
			cur := result[key]
			result[key] = occRange{
				min: cur.min + r.min,
				max: sumMax(cur.max, r.max),
			}
		}
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
		return map[int]occRange{
			idx: {min: p.MinOccurs, max: p.MaxOccurs},
		}
	case ParticleGroup:
		var childRanges []map[int]occRange
		for _, child := range p.Children {
			childRanges = append(childRanges, b.symbolBoundsForParticle(child))
		}
		var combined map[int]occRange
		switch p.GroupKind {
		case types.Sequence, types.AllGroup:
			combined = combineSequenceRanges(childRanges)
		case types.Choice:
			combined = combineChoiceRanges(childRanges)
		default:
			combined = combineSequenceRanges(childRanges)
		}
		return applyGroupOccurs(combined, p.MinOccurs, p.MaxOccurs)
	default:
		return nil
	}
}

func (b *Builder) symbolIndexForParticle(p *ParticleAdapter) (int, bool) {
	if p.Original == nil {
		return 0, false
	}
	key := symbolKey(p.Original, p.AllowSubstitution)
	idx, ok := b.symbolIndexByKey[key]
	return idx, ok
}

func combineSequenceRanges(ranges []map[int]occRange) map[int]occRange {
	result := make(map[int]occRange)
	for _, r := range ranges {
		for key, val := range r {
			cur := result[key]
			result[key] = occRange{
				min: cur.min + val.min,
				max: sumMax(cur.max, val.max),
			}
		}
	}
	return result
}

func combineChoiceRanges(ranges []map[int]occRange) map[int]occRange {
	keys := make(map[int]struct{})
	for _, r := range ranges {
		for key := range r {
			keys[key] = struct{}{}
		}
	}
	result := make(map[int]occRange, len(keys))
	for key := range keys {
		min := -1
		max := 0
		for _, r := range ranges {
			val, ok := r[key]
			if !ok {
				val = occRange{min: 0, max: 0}
			}
			if min == -1 || val.min < min {
				min = val.min
			}
			if max == types.UnboundedOccurs || val.max == types.UnboundedOccurs {
				max = types.UnboundedOccurs
			} else if val.max > max {
				max = val.max
			}
		}
		result[key] = occRange{min: min, max: max}
	}
	return result
}

func applyGroupOccurs(ranges map[int]occRange, groupMin, groupMax int) map[int]occRange {
	result := make(map[int]occRange, len(ranges))
	for key, r := range ranges {
		result[key] = occRange{
			min: r.min * groupMin,
			max: multiplyMax(r.max, groupMax),
		}
	}
	return result
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

// Symbol helpers
func symbolKey(p types.Particle, allowSubstitution bool) string {
	switch v := p.(type) {
	case *types.ElementDecl:
		return fmt.Sprintf("e:%t:%s", allowSubstitution, v.Name.String())
	case *types.AnyElement:
		if v.Namespace == types.NSCList {
			parts := make([]string, len(v.NamespaceList))
			for i, ns := range v.NamespaceList {
				parts[i] = string(ns)
			}
			return "a:" + fmt.Sprintf("%d:%s", int(v.Namespace), strings.Join(parts, ","))
		}
		if v.Namespace == types.NSCOther || v.Namespace == types.NSCTargetNamespace {
			return "a:" + fmt.Sprintf("%d:%s", int(v.Namespace), v.TargetNamespace)
		}
		return "a:" + fmt.Sprintf("%d", int(v.Namespace))
	default:
		return "?"
	}
}

func (b *Builder) makeSymbol(p types.Particle, allowSubstitution bool) Symbol {
	switch v := p.(type) {
	case *types.ElementDecl:
		qname := v.Name
		if !b.isElementQualified(v) {
			// Unqualified local elements should match elements with no namespace
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
			// Explicit namespace list - only elements from listed namespaces match
			nsList := make([]string, len(v.NamespaceList))
			for i, ns := range v.NamespaceList {
				nsList[i] = string(ns)
			}
			return Symbol{Kind: KindAnyNSList, NSList: nsList}
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
