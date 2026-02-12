package runtime

// TypeKind defines an exported type.
type TypeKind uint8

const (
	// TypeBuiltin is an exported constant.
	TypeBuiltin TypeKind = iota
	// TypeSimple is an exported constant.
	TypeSimple
	// TypeComplex is an exported constant.
	TypeComplex
)

// Type defines an exported type.
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

// TypeFlags defines an exported type.
type TypeFlags uint32

const (
	// TypeAbstract is an exported constant.
	TypeAbstract TypeFlags = 1 << iota
)

// DerivationMethod defines an exported type.
type DerivationMethod uint8

const (
	// DerNone is an exported constant.
	DerNone DerivationMethod = 0
	// DerExtension is an exported constant.
	DerExtension DerivationMethod = 1 << 0
	// DerRestriction is an exported constant.
	DerRestriction DerivationMethod = 1 << 1
	// DerList is an exported constant.
	DerList DerivationMethod = 1 << 2
	// DerUnion is an exported constant.
	DerUnion DerivationMethod = 1 << 3
)

// TypeAncestors defines an exported type.
type TypeAncestors struct {
	IDs   []TypeID
	Masks []DerivationMethod
}

// ComplexTypeRef defines an exported type.
type ComplexTypeRef struct {
	ID uint32
}

// ComplexType defines an exported type.
type ComplexType struct {
	TextFixed         ValueRef
	TextDefault       ValueRef
	TextFixedMember   ValidatorID
	TextDefaultMember ValidatorID
	Attrs             AttrIndexRef
	Model             ModelRef
	AnyAttr           WildcardID
	TextValidator     ValidatorID
	Content           ContentKind
	Mixed             bool
}

// Element defines an exported type.
type Element struct {
	Name SymbolID

	Type      TypeID
	SubstHead ElemID

	Default       ValueRef
	Fixed         ValueRef
	DefaultKey    ValueKeyRef
	FixedKey      ValueKeyRef
	DefaultMember ValidatorID
	FixedMember   ValidatorID

	Flags ElemFlags
	Block ElemBlock
	Final DerivationMethod

	ICOff uint32
	ICLen uint32
}

// ElemFlags defines an exported type.
type ElemFlags uint32

const (
	// ElemNillable is an exported constant.
	ElemNillable ElemFlags = 1 << iota
	// ElemAbstract is an exported constant.
	ElemAbstract
)

// ElemBlock defines an exported type.
type ElemBlock uint8

const (
	// ElemBlockSubstitution is an exported constant.
	ElemBlockSubstitution ElemBlock = 1 << iota
	// ElemBlockExtension is an exported constant.
	ElemBlockExtension
	// ElemBlockRestriction is an exported constant.
	ElemBlockRestriction
)

// Attribute defines an exported type.
type Attribute struct {
	Name          SymbolID
	Validator     ValidatorID
	Default       ValueRef
	Fixed         ValueRef
	DefaultKey    ValueKeyRef
	FixedKey      ValueKeyRef
	DefaultMember ValidatorID
	FixedMember   ValidatorID
}

// AttrUse defines an exported type.
type AttrUse struct {
	Name          SymbolID
	Validator     ValidatorID
	Use           AttrUseKind
	Default       ValueRef
	Fixed         ValueRef
	DefaultKey    ValueKeyRef
	FixedKey      ValueKeyRef
	DefaultMember ValidatorID
	FixedMember   ValidatorID
}

// AttrUseKind defines an exported type.
type AttrUseKind uint8

const (
	// AttrOptional is an exported constant.
	AttrOptional AttrUseKind = iota
	// AttrRequired is an exported constant.
	AttrRequired
	// AttrProhibited is an exported constant.
	AttrProhibited
)

// AttrIndexRef defines an exported type.
type AttrIndexRef struct {
	Off       uint32
	Len       uint32
	Mode      AttrIndexMode
	HashTable uint32
}

// AttrIndexMode defines an exported type.
type AttrIndexMode uint8

const (
	// AttrIndexSmallLinear is an exported constant.
	AttrIndexSmallLinear AttrIndexMode = iota
	// AttrIndexSortedBinary is an exported constant.
	AttrIndexSortedBinary
	// AttrIndexHash is an exported constant.
	AttrIndexHash
)

// ComplexAttrIndex defines an exported type.
type ComplexAttrIndex struct {
	Uses       []AttrUse
	HashTables []AttrHashTable
}

// AttrHashTable defines an exported type.
type AttrHashTable struct {
	Hash []uint64
	Slot []uint32
}
