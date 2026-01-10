package grammar

import "github.com/jacoelho/xsd/internal/types"

// Automaton is a compiled deterministic finite automaton for content model schemacheck.
// It validates element sequences in O(n) time with no backtracking.
type Automaton struct {
	// alphabet of the automaton
	symbols []Symbol
	// transition table: [state*symbolCount + symbol] → next state (-1 = invalid)
	transitions []int
	// accepting[i] = true if state i is a final state
	accepting []bool
	// occurrence constraints per state (nil if not counting)
	counting []*Counter
	// true if empty content is valid
	emptyOK bool
	// min occurrences per symbol across the content model
	symbolMin []int
	// max occurrences per symbol across the content model
	symbolMax []int
	// targetNamespace is used for resolving ##targetNamespace and ##other wildcards.
	targetNamespace string
	// Group counter info: position index -> group counter info for groups that need counting
	groupCounters map[int]*GroupCounterInfo
	// groupIndexByID maps group IDs to compact indices for counter slices.
	groupIndexByID map[int]int
	// groupCount is the number of unique groups that need counting.
	groupCount int
	// posElements maps position indices to compiled elements for match resolution.
	posElements []*CompiledElement
	// stateSymbolPos maps [state][symbol] to a position index (or negative if none/ambiguous).
	stateSymbolPos [][]int
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
	// minOccurs, maxOccurs of the group
	Min, Max int
	// positions that indicate group completion
	LastPositions []int
	// positions that indicate group start (for counting iterations)
	FirstPositions []int
	GroupKind      types.GroupKind
	// unique ID for this group (first position in firstPos)
	GroupID int
	// maxOccurs of the element at firstPos (for computing min iterations)
	FirstPosMaxOccurs int
	// fixed element occurrences per group iteration (0 if not fixed)
	UnitSize int
}

// Symbol represents an input symbol in the automaton's alphabet.
type Symbol struct {
	Kind SymbolKind
	// for KindElement
	QName types.QName
	// for KindAnyNS, KindAnyOther
	NS     string
	NSList []types.NamespaceURI
	// only for KindElement
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
	// minimum occurrences required
	Min int
	// maximum occurrences allowed (-1 = unbounded)
	Max int
	// which symbol is being counted (for element counters)
	SymbolIndex int
	// For groups: tracks if this counter is for a group completion (not individual element)
	IsGroupCounter bool
	// For groups: symbol indices that indicate group completion (symbols for last positions of the group)
	GroupCompletionSymbols []int
	// For groups: symbol indices that indicate group start (symbols for first positions of the group)
	GroupStartSymbols []int
	// For groups: unique ID shared by all counters tracking the same group
	GroupID int
	// For groups: maxOccurs of the element at firstPos (for computing minimum iterations)
	FirstPosMaxOccurs int
	// For groups: fixed element occurrences per group iteration (0 if not fixed)
	UnitSize int
}

// Position represents a leaf node position in the Glushkov syntax tree.
type Position struct {
	Index    int
	Particle types.Particle
	// occurrence constraints
	Min, Max int
	// AllowSubstitution indicates if substitution groups apply for this position.
	AllowSubstitution bool
	Element           *CompiledElement
}

const (
	symbolPosNone      = -1
	symbolPosAmbiguous = -2
)
