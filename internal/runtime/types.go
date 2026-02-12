package runtime

// TypeKind enumerates type kind values.
type TypeKind uint8

const (
	TypeBuiltin TypeKind = iota
	TypeSimple
	TypeComplex
)

// Type stores the compiled runtime descriptor for a schema type.
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

// TypeFlags is a bitset for type flags options.
type TypeFlags uint32

const (
	TypeAbstract TypeFlags = 1 << iota
)

// DerivationMethod enumerates derivation method values.
type DerivationMethod uint8

const (
	DerNone        DerivationMethod = 0
	DerExtension   DerivationMethod = 1 << 0
	DerRestriction DerivationMethod = 1 << 1
	DerList        DerivationMethod = 1 << 2
	DerUnion       DerivationMethod = 1 << 3
)

// TypeAncestors stores precomputed ancestor IDs and derivation masks.
type TypeAncestors struct {
	IDs   []TypeID
	Masks []DerivationMethod
}

// ComplexTypeRef references complex type ref data in packed tables.
type ComplexTypeRef struct {
	ID uint32
}

// ComplexType stores runtime metadata for complex-type validation.
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

// Element stores runtime metadata for an element declaration.
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

// ElemFlags is a bitset for elem flags options.
type ElemFlags uint32

const (
	ElemNillable ElemFlags = 1 << iota
	ElemAbstract
)

// ElemBlock is a bitmask of blocked substitution derivations.
type ElemBlock uint8

const (
	ElemBlockSubstitution ElemBlock = 1 << iota
	ElemBlockExtension
	ElemBlockRestriction
)

// Attribute stores runtime metadata for an attribute declaration.
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

// AttrUse stores runtime metadata for an attribute use.
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

// AttrUseKind enumerates attr use kind values.
type AttrUseKind uint8

const (
	AttrOptional AttrUseKind = iota
	AttrRequired
	AttrProhibited
)

// AttrIndexRef references attr index ref data in packed tables.
type AttrIndexRef struct {
	Off       uint32
	Len       uint32
	Mode      AttrIndexMode
	HashTable uint32
}

// AttrIndexMode enumerates attr index mode values.
type AttrIndexMode uint8

const (
	AttrIndexSmallLinear AttrIndexMode = iota
	AttrIndexSortedBinary
	AttrIndexHash
)

// ComplexAttrIndex stores flattened attribute-use tables for complex types.
type ComplexAttrIndex struct {
	Uses       []AttrUse
	HashTables []AttrHashTable
}

// AttrHashTable stores an open-addressing hash table for attribute lookup.
type AttrHashTable struct {
	Hash []uint64
	Slot []uint32
}
