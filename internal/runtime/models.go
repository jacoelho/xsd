package runtime

type ContentKind uint8

const (
	ContentEmpty ContentKind = iota
	ContentSimple
	ContentMixed
	ContentElementOnly
	ContentAll
)

type ModelRef struct {
	Kind ModelKind
	ID   uint32
}

type ModelKind uint8

const (
	ModelNone ModelKind = iota
	ModelDFA
	ModelNFA
	ModelAll
)

type ModelsBundle struct {
	DFA      []DFAModel
	NFA      []NFAModel
	All      []AllModel
	AllSubst []ElemID
}

type DFAModel struct {
	States      []DFAState
	Transitions []DFATransition
	Wildcards   []DFAWildcardEdge
	Start       uint32
}

type DFAState struct {
	Accept   bool
	TransOff uint32
	TransLen uint32
	WildOff  uint32
	WildLen  uint32
}

type DFATransition struct {
	Sym  SymbolID
	Next uint32
	Elem ElemID
}

type DFAWildcardEdge struct {
	Rule WildcardID
	Next uint32
}

type AllModel struct {
	Members   []AllMember
	MinOccurs uint32
	Mixed     bool
}

type AllMember struct {
	Elem        ElemID
	Optional    bool
	AllowsSubst bool
	SubstOff    uint32
	SubstLen    uint32
}

type BitsetRef struct {
	Off uint32
	Len uint32
}

type BitsetBlob struct {
	Words []uint64
}

type PosMatchKind uint8

const (
	PosExact PosMatchKind = iota
	PosWildcard
)

type PosMatcher struct {
	Kind PosMatchKind
	Sym  SymbolID
	Elem ElemID
	Rule WildcardID
}

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
