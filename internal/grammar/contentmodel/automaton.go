package contentmodel

import "github.com/jacoelho/xsd/internal/types"

// Automaton is a compiled deterministic finite automaton for content model validation.
// It validates element sequences in O(n) time with no backtracking.
type Automaton struct {
	symbols   []Symbol   // alphabet of the automaton
	trans     [][]int    // transition table: [state][symbol] → next state (-1 = invalid)
	accepting []bool     // accepting[i] = true if state i is a final state
	counting  []*Counter // occurrence constraints per state (nil if not counting)
	emptyOK   bool       // true if empty content is valid
	symbolMin []int      // min occurrences per symbol across the content model
	symbolMax []int      // max occurrences per symbol across the content model
	// targetNamespace is used for resolving ##targetNamespace and ##other wildcards.
	targetNamespace string
	// Group counter info: position index -> group counter info for groups that need counting
	groupCounters map[int]*GroupCounterInfo
	// groupIndexByID maps group IDs to compact indices for counter slices.
	groupIndexByID map[int]int
	// groupCount is the number of unique groups that need counting.
	groupCount int
	// posElements maps position indices to compiled elements for match resolution.
	posElements []any
	// stateSymbolPos maps [state][symbol] to a position index (or negative if none/ambiguous).
	stateSymbolPos [][]int
}

// GroupCounterInfo tracks information about a group that needs counting
type GroupCounterInfo struct {
	Min, Max          int   // minOccurs, maxOccurs of the group
	LastPositions     []int // positions that indicate group completion
	FirstPositions    []int // positions that indicate group start (for counting iterations)
	GroupKind         types.GroupKind
	GroupID           int // unique ID for this group (first position in firstPos)
	FirstPosMaxOccurs int // maxOccurs of the element at firstPos (for computing min iterations)
	UnitSize          int // fixed element occurrences per group iteration (0 if not fixed)
}

// Symbol represents an input symbol in the automaton's alphabet.
type Symbol struct {
	Kind              SymbolKind
	QName             types.QName // for KindElement
	NS                string      // for KindAnyNS, KindAnyOther
	NSList            []types.NamespaceURI
	AllowSubstitution bool // only for KindElement
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
	Min       int // minimum occurrences required
	Max       int // maximum occurrences allowed (-1 = unbounded)
	SymbolIdx int // which symbol is being counted (for element counters)
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
	Min, Max int // occurrence constraints
	// AllowSubstitution indicates if substitution groups apply for this position.
	AllowSubstitution bool
	Element           any // *grammar.CompiledElement
}

const (
	symbolPosNone      = -1
	symbolPosAmbiguous = -2
)
