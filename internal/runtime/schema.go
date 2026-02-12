package runtime

// Schema is the immutable runtime representation used by validation sessions.
type Schema struct {
	Symbols    SymbolsTable
	Namespaces NamespaceTable

	GlobalTypes      []TypeID
	GlobalElements   []ElemID
	GlobalAttributes []AttrID

	Types        []Type
	Ancestors    TypeAncestors
	ComplexTypes []ComplexType
	Elements     []Element
	Attributes   []Attribute
	AttrIndex    ComplexAttrIndex

	Validators ValidatorsBundle
	Facets     []FacetInstr
	Patterns   []Pattern
	Enums      EnumTable
	Values     ValueBlob
	Notations  []SymbolID

	Models     ModelsBundle
	Wildcards  []WildcardRule
	WildcardNS []NamespaceID

	ICs         []IdentityConstraint
	ElemICs     []ICID
	ICSelectors []PathID
	ICFields    []PathID
	Paths       []PathProgram

	Predef   PredefinedSymbols
	PredefNS PredefinedNamespaces
	Builtin  BuiltinIDs

	RootPolicy RootPolicy

	BuildHash uint64
}

// BuiltinIDs caches frequently accessed built-in type IDs.
type BuiltinIDs struct {
	AnyType       TypeID
	AnySimpleType TypeID
}

// PredefinedSymbols caches interned symbols for XML and xsi attributes.
type PredefinedSymbols struct {
	XsiType                      SymbolID
	XsiNil                       SymbolID
	XsiSchemaLocation            SymbolID
	XsiNoNamespaceSchemaLocation SymbolID

	XMLLang  SymbolID
	XMLSpace SymbolID
}

// PredefinedNamespaces caches interned namespace IDs for XML/xsi/empty namespaces.
type PredefinedNamespaces struct {
	Xsi   NamespaceID
	XML   NamespaceID
	Empty NamespaceID
}

// RootPolicy enumerates root policy values.
type RootPolicy uint8

const (
	RootStrict RootPolicy = iota
	RootAny
)
