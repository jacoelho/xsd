package grammar

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestScanPositionsForTransitions(t *testing.T) {
	builder := &Builder{
		size:      4,
		symbols:   []Symbol{{}, {}},
		posSymbol: []int{0, 0, 1, 0},
		positions: []*Position{
			{Index: 0},
			{Index: 1},
			{Index: 2},
			nil,
		},
		followPos: make([]*bitset, 4),
	}
	for i := range builder.followPos {
		builder.followPos[i] = newBitset(builder.size)
	}
	builder.followPos[0].set(2)
	builder.followPos[1].set(2)

	state := newBitset(builder.size)
	state.set(0)
	state.set(1)
	state.set(3) // nil position should be ignored

	nextBySymbol := make([]*bitset, len(builder.symbols))
	posRow, usedSymbols := builder.scanPositionsForTransitions(state, nextBySymbol)

	if len(usedSymbols) != 1 || usedSymbols[0] != 0 {
		t.Fatalf("unexpected used symbols: %v", usedSymbols)
	}
	if got := posRow[0]; got != symbolPosAmbiguous {
		t.Fatalf("expected symbol 0 to be ambiguous, got %d", got)
	}
	if got := posRow[1]; got != symbolPosNone {
		t.Fatalf("expected symbol 1 to be empty, got %d", got)
	}
	if nextBySymbol[0] == nil || !nextBySymbol[0].test(2) {
		t.Fatalf("expected follow set for symbol 0 to include position 2")
	}
	if nextBySymbol[1] != nil {
		t.Fatalf("expected no follow set for symbol 1")
	}
}

func TestGetOrCreateState(t *testing.T) {
	builder := &Builder{
		size:    2,
		endPos:  1,
		symbols: []Symbol{{}},
	}
	automaton := &Automaton{
		symbols:        builder.symbols,
		transitions:    append([]int(nil), builder.newTransitionRow()...),
		accepting:      []bool{false},
		counting:       []*Counter{nil},
		stateSymbolPos: [][]int{nil},
	}
	stateIDs := map[string]int{}
	worklist := []workItem{}

	stateSet := newBitset(builder.size)
	stateSet.set(builder.endPos)

	stateID, worklist := builder.getOrCreateState(automaton, stateSet, stateIDs, worklist)
	if stateID != 1 {
		t.Fatalf("expected new state ID 1, got %d", stateID)
	}
	if len(automaton.accepting) != 2 || !automaton.accepting[1] {
		t.Fatalf("expected accepting state to be appended")
	}
	if len(worklist) != 1 {
		t.Fatalf("expected worklist to include new state")
	}

	duplicate := newBitset(builder.size)
	duplicate.set(builder.endPos)
	dupID, worklist := builder.getOrCreateState(automaton, duplicate, stateIDs, worklist)
	if dupID != stateID {
		t.Fatalf("expected duplicate state ID %d, got %d", stateID, dupID)
	}
	if len(automaton.accepting) != 2 {
		t.Fatalf("expected no new state to be added")
	}
	if len(worklist) != 1 {
		t.Fatalf("expected worklist size to remain the same")
	}
}

func TestAttachGroupCounter(t *testing.T) {
	builder := &Builder{
		size:          1,
		positions:     []*Position{{Index: 0, Min: 2, Max: 2}},
		groupCounters: make(map[int]*GroupCounterInfo),
	}
	child := newLeaf(0, nil, 2, 2, builder.size)
	group := &ParticleAdapter{
		Kind:      ParticleGroup,
		GroupKind: types.Sequence,
		MinOccurs: 2,
		MaxOccurs: 5,
	}

	builder.attachGroupCounter(child, group)

	info, ok := builder.groupCounters[0]
	if !ok {
		t.Fatalf("expected group counter to be attached")
	}
	if info.Min != 2 || info.Max != 5 {
		t.Fatalf("unexpected group bounds: min=%d max=%d", info.Min, info.Max)
	}
	if info.GroupKind != types.Sequence || info.GroupID != 0 {
		t.Fatalf("unexpected group identifiers: kind=%v id=%d", info.GroupKind, info.GroupID)
	}
	if info.FirstPosMaxOccurs != 2 || info.UnitSize != 2 {
		t.Fatalf("unexpected group sizing: maxOccurs=%d unitSize=%d", info.FirstPosMaxOccurs, info.UnitSize)
	}
	if len(info.FirstPositions) != 1 || len(info.LastPositions) != 1 {
		t.Fatalf("expected single first/last position")
	}
}

func TestBoundsForGroupParticleChoice(t *testing.T) {
	elemA := &types.ElementDecl{Name: types.QName{Local: "a"}}
	elemB := &types.ElementDecl{Name: types.QName{Local: "b"}}

	builder := &Builder{
		symbolIndexByKey: map[symbolKey]int{
			symbolKeyForParticle(elemA, substitutionDisallowed): 0,
			symbolKeyForParticle(elemB, substitutionDisallowed): 1,
		},
	}

	group := &ParticleAdapter{
		Kind:      ParticleGroup,
		GroupKind: types.Choice,
		MinOccurs: 1,
		MaxOccurs: 1,
		Children: []*ParticleAdapter{
			{Kind: ParticleElement, MinOccurs: 1, MaxOccurs: 1, Original: elemA},
			{Kind: ParticleElement, MinOccurs: 1, MaxOccurs: 1, Original: elemB},
		},
	}

	bounds := builder.symbolBoundsForParticle(group)
	if bounds == nil {
		t.Fatalf("expected bounds to be computed")
	}
	if got := bounds[0]; got.min != 0 || got.max != 1 {
		t.Fatalf("unexpected bounds for symbol 0: min=%d max=%d", got.min, got.max)
	}
	if got := bounds[1]; got.min != 0 || got.max != 1 {
		t.Fatalf("unexpected bounds for symbol 1: min=%d max=%d", got.min, got.max)
	}
	builder.putRangeMap(bounds)
}
