package runtime

// Schema defines an exported type.
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

// BuiltinIDs defines an exported type.
type BuiltinIDs struct {
	AnyType       TypeID
	AnySimpleType TypeID
}

// PredefinedSymbols defines an exported type.
type PredefinedSymbols struct {
	XsiType                      SymbolID
	XsiNil                       SymbolID
	XsiSchemaLocation            SymbolID
	XsiNoNamespaceSchemaLocation SymbolID

	XMLLang  SymbolID
	XMLSpace SymbolID
}

// PredefinedNamespaces defines an exported type.
type PredefinedNamespaces struct {
	Xsi   NamespaceID
	XML   NamespaceID
	Empty NamespaceID
}

// RootPolicy defines an exported type.
type RootPolicy uint8

const (
	// RootStrict is an exported constant.
	RootStrict RootPolicy = iota
	// RootAny is an exported constant.
	RootAny
)
