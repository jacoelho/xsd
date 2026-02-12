package runtime

// ContentKind enumerates content kind values.
type ContentKind uint8

const (
	ContentEmpty ContentKind = iota
	ContentSimple
	ContentMixed
	ContentElementOnly
	ContentAll
)

// ModelRef references model ref data in packed tables.
type ModelRef struct {
	Kind ModelKind
	ID   uint32
}

// ModelKind enumerates model kind values.
type ModelKind uint8

const (
	ModelNone ModelKind = iota
	ModelDFA
	ModelNFA
	ModelAll
)

// ModelsBundle groups all compiled content-model tables.
type ModelsBundle struct {
	DFA      []DFAModel
	NFA      []NFAModel
	All      []AllModel
	AllSubst []ElemID
}

// DFAModel stores a deterministic content model.
type DFAModel struct {
	States      []DFAState
	Transitions []DFATransition
	Wildcards   []DFAWildcardEdge
	Start       uint32
}

// DFAState stores one DFA state and its transition spans.
type DFAState struct {
	Accept   bool
	TransOff uint32
	TransLen uint32
	WildOff  uint32
	WildLen  uint32
}

// DFATransition stores one symbol-driven DFA edge.
type DFATransition struct {
	Sym  SymbolID
	Next uint32
	Elem ElemID
}

// DFAWildcardEdge stores one wildcard DFA edge.
type DFAWildcardEdge struct {
	Rule WildcardID
	Next uint32
}

// AllModel stores the flattened representation of an xs:all model group.
type AllModel struct {
	Members   []AllMember
	MinOccurs uint32
	Mixed     bool
}

// AllMember stores one member constraint in an xs:all model.
type AllMember struct {
	Elem        ElemID
	Optional    bool
	AllowsSubst bool
	SubstOff    uint32
	SubstLen    uint32
}

// BitsetRef references bitset ref data in packed tables.
type BitsetRef struct {
	Off uint32
	Len uint32
}

// BitsetBlob stores packed NFA bitset words.
type BitsetBlob struct {
	Words []uint64
}

// PosMatchKind enumerates pos match kind values.
type PosMatchKind uint8

const (
	PosExact PosMatchKind = iota
	PosWildcard
)

// PosMatcher describes one position matcher in the Glushkov/NFA model.
type PosMatcher struct {
	Kind PosMatchKind
	Sym  SymbolID
	Elem ElemID
	Rule WildcardID
}

// NFAModel stores a nondeterministic content model.
type NFAModel struct {
	Bitsets   BitsetBlob
	Matchers  []PosMatcher
	Follow    []BitsetRef
	Start     BitsetRef
	Accept    BitsetRef
	FollowOff uint32
	FollowLen uint32
	Nullable  bool
}
