package runtime

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

type BuiltinIDs struct {
	AnyType       TypeID
	AnySimpleType TypeID
}

type PredefinedSymbols struct {
	XsiType                      SymbolID
	XsiNil                       SymbolID
	XsiSchemaLocation            SymbolID
	XsiNoNamespaceSchemaLocation SymbolID

	XmlLang  SymbolID
	XmlSpace SymbolID
}

type PredefinedNamespaces struct {
	Xsi   NamespaceID
	Xml   NamespaceID
	Empty NamespaceID
}

type RootPolicy uint8

const (
	RootStrict RootPolicy = iota
	RootAny
)
