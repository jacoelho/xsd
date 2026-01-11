package grammar

import "github.com/jacoelho/xsd/internal/types"

// Automaton is a compiled deterministic finite automaton for content model schemacheck.
// It validates element sequences in O(n) time with no backtracking.
type Automaton struct {
	groupIndexByID  map[int]int
	groupCounters   map[int]*GroupCounterInfo
	targetNamespace string
	counting        []*Counter
	symbolMin       []int
	symbolMax       []int
	symbols         []Symbol
	accepting       []bool
	transitions     []int
	posElements     []*CompiledElement
	stateSymbolPos  [][]int
	groupCount      int
	emptyOK         bool
}

func (a *Automaton) transitionIndex(state, symbolIndex int) int {
	return state*len(a.symbols) + symbolIndex
}

func (a *Automaton) transition(state, symbolIndex int) int {
	return a.transitions[a.transitionIndex(state, symbolIndex)]
}

func (a *Automaton) setTransition(state, symbolIndex, next int) {
	a.transitions[a.transitionIndex(state, symbolIndex)] = next
}

// GroupCounterInfo tracks information about a group that needs counting
type GroupCounterInfo struct {
	LastPositions     []int
	FirstPositions    []int
	Min               int
	Max               int
	GroupKind         types.GroupKind
	GroupID           int
	FirstPosMaxOccurs int
	UnitSize          int
}

// Symbol represents an input symbol in the automaton's alphabet.
type Symbol struct {
	QName             types.QName
	NS                string
	NSList            []types.NamespaceURI
	Kind              SymbolKind
	AllowSubstitution bool
}

// SymbolKind classifies symbols in the alphabet.
type SymbolKind int

const (
	// KindElement represents a specific element by QName.
	KindElement SymbolKind = iota
	// KindAny represents the ##any wildcard.
	KindAny
	// KindAnyNS represents ##targetNamespace or a single namespace.
	KindAnyNS
	// KindAnyOther represents the ##other wildcard.
	KindAnyOther
	// KindAnyNSList represents an explicit namespace list.
	KindAnyNSList
)

// Counter tracks occurrence constraints for repeating particles.
type Counter struct {
	GroupCompletionSymbols []int
	GroupStartSymbols      []int
	Min                    int
	Max                    int
	SymbolIndex            int
	GroupID                int
	FirstPosMaxOccurs      int
	UnitSize               int
	IsGroupCounter         bool
}

// Position represents a leaf node position in the Glushkov syntax tree.
type Position struct {
	Particle          types.Particle
	Element           *CompiledElement
	Index             int
	Min               int
	Max               int
	AllowSubstitution bool
}

const (
	symbolPosNone      = -1
	symbolPosAmbiguous = -2
)
