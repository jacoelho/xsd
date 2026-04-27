package runtime

// Schema is the immutable runtime representation used by validation sessions.
type Schema struct {
	symbols    SymbolsTable
	namespaces NamespaceTable

	globalTypes      []TypeID
	globalElements   []ElemID
	globalAttributes []AttrID

	types        []Type
	ancestors    TypeAncestors
	complexTypes []ComplexType
	elements     []Element
	attributes   []Attribute
	attrIndex    ComplexAttrIndex

	validators ValidatorsBundle
	facets     []FacetInstr
	patterns   []Pattern
	enums      EnumTable
	values     ValueBlob
	notations  []SymbolID

	models     ModelsBundle
	wildcards  []WildcardRule
	wildcardNS []NamespaceID

	identityConstraints        []IdentityConstraint
	elementIdentityConstraints []ICID
	identitySelectors          []PathID
	identityFields             []PathID
	paths                      []PathProgram

	predef   PredefinedSymbols
	predefNS PredefinedNamespaces
	builtin  BuiltinIDs

	rootPolicy RootPolicy

	buildHash uint64
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
