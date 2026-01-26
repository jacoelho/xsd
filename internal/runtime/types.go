package runtime

type TypeKind uint8

const (
	TypeBuiltin TypeKind = iota
	TypeSimple
	TypeComplex
)

type Type struct {
	Name       SymbolID
	Flags      TypeFlags
	Base       TypeID
	AncOff     uint32
	AncLen     uint32
	AncMaskOff uint32
	Validator  ValidatorID
	Complex    ComplexTypeRef
	Kind       TypeKind
	Derivation DerivationMethod
	Final      DerivationMethod
	Block      DerivationMethod
}

type TypeFlags uint32

const (
	TypeAbstract TypeFlags = 1 << iota
)

type DerivationMethod uint8

const (
	DerNone        DerivationMethod = 0
	DerExtension   DerivationMethod = 1 << 0
	DerRestriction DerivationMethod = 1 << 1
	DerList        DerivationMethod = 1 << 2
	DerUnion       DerivationMethod = 1 << 3
)

type TypeAncestors struct {
	IDs   []TypeID
	Masks []DerivationMethod
}

type ComplexTypeRef struct {
	ID uint32
}

type ComplexType struct {
	TextFixed     ValueRef
	TextDefault   ValueRef
	Attrs         AttrIndexRef
	Model         ModelRef
	AnyAttr       WildcardID
	TextValidator ValidatorID
	Content       ContentKind
	Mixed         bool
}

type Element struct {
	Name SymbolID

	Type      TypeID
	SubstHead ElemID

	Default ValueRef
	Fixed   ValueRef

	Flags ElemFlags
	Block ElemBlock
	Final DerivationMethod

	ICOff uint32
	ICLen uint32
}

type ElemFlags uint32

const (
	ElemNillable ElemFlags = 1 << iota
	ElemAbstract
)

type ElemBlock uint8

const (
	ElemBlockSubstitution ElemBlock = 1 << iota
	ElemBlockExtension
	ElemBlockRestriction
)

type Attribute struct {
	Name      SymbolID
	Validator ValidatorID
	Default   ValueRef
	Fixed     ValueRef
}

type AttrUse struct {
	Name      SymbolID
	Validator ValidatorID
	Use       AttrUseKind
	Default   ValueRef
	Fixed     ValueRef
}

type AttrUseKind uint8

const (
	AttrOptional AttrUseKind = iota
	AttrRequired
	AttrProhibited
)

type AttrIndexRef struct {
	Off       uint32
	Len       uint32
	Mode      AttrIndexMode
	HashTable uint32
}

type AttrIndexMode uint8

const (
	AttrIndexSmallLinear AttrIndexMode = iota
	AttrIndexSortedBinary
	AttrIndexHash
)

type ComplexAttrIndex struct {
	Uses       []AttrUse
	HashTables []AttrHashTable
}

type AttrHashTable struct {
	Hash []uint64
	Slot []uint32
}
