package runtime

// SymbolID identifies an interned symbol in Schema.Symbols.
type SymbolID uint32

// NamespaceID identifies an interned namespace URI in Schema.Namespaces.
type NamespaceID uint32

// TypeID identifies an entry in Schema.Types.
type TypeID uint32

// ElemID identifies an entry in Schema.Elements.
type ElemID uint32

// AttrID identifies an entry in Schema.Attributes.
type AttrID uint32

// ModelID identifies a compiled content-model entry.
type ModelID uint32

// WildcardID identifies an entry in Schema.Wildcards.
type WildcardID uint32

// ICID identifies an identity-constraint program entry.
type ICID uint32

// PathID identifies a compiled selector/field path program.
type PathID uint32

// ValidatorID identifies a compiled value validator.
type ValidatorID uint32

// PatternID identifies a compiled regular expression.
type PatternID uint32

// EnumID identifies an enumeration table entry.
type EnumID uint32
