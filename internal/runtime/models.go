package runtime

// ContentKind defines an exported type.
type ContentKind uint8

const (
	// ContentEmpty is an exported constant.
	ContentEmpty ContentKind = iota
	// ContentSimple is an exported constant.
	ContentSimple
	// ContentMixed is an exported constant.
	ContentMixed
	// ContentElementOnly is an exported constant.
	ContentElementOnly
	// ContentAll is an exported constant.
	ContentAll
)

// ModelRef defines an exported type.
type ModelRef struct {
	Kind ModelKind
	ID   uint32
}

// ModelKind defines an exported type.
type ModelKind uint8

const (
	// ModelNone is an exported constant.
	ModelNone ModelKind = iota
	// ModelDFA is an exported constant.
	ModelDFA
	// ModelNFA is an exported constant.
	ModelNFA
	// ModelAll is an exported constant.
	ModelAll
)

// ModelsBundle defines an exported type.
type ModelsBundle struct {
	DFA      []DFAModel
	NFA      []NFAModel
	All      []AllModel
	AllSubst []ElemID
}

// DFAModel defines an exported type.
type DFAModel struct {
	States      []DFAState
	Transitions []DFATransition
	Wildcards   []DFAWildcardEdge
	Start       uint32
}

// DFAState defines an exported type.
type DFAState struct {
	Accept   bool
	TransOff uint32
	TransLen uint32
	WildOff  uint32
	WildLen  uint32
}

// DFATransition defines an exported type.
type DFATransition struct {
	Sym  SymbolID
	Next uint32
	Elem ElemID
}

// DFAWildcardEdge defines an exported type.
type DFAWildcardEdge struct {
	Rule WildcardID
	Next uint32
}

// AllModel defines an exported type.
type AllModel struct {
	Members   []AllMember
	MinOccurs uint32
	Mixed     bool
}

// AllMember defines an exported type.
type AllMember struct {
	Elem        ElemID
	Optional    bool
	AllowsSubst bool
	SubstOff    uint32
	SubstLen    uint32
}

// BitsetRef defines an exported type.
type BitsetRef struct {
	Off uint32
	Len uint32
}

// BitsetBlob defines an exported type.
type BitsetBlob struct {
	Words []uint64
}

// PosMatchKind defines an exported type.
type PosMatchKind uint8

const (
	// PosExact is an exported constant.
	PosExact PosMatchKind = iota
	// PosWildcard is an exported constant.
	PosWildcard
)

// PosMatcher defines an exported type.
type PosMatcher struct {
	Kind PosMatchKind
	Sym  SymbolID
	Elem ElemID
	Rule WildcardID
}

// NFAModel defines an exported type.
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
