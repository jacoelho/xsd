package schemair

type TypeID uint32
type ElementID uint32
type AttributeID uint32
type ModelGroupID uint32
type IdentityID uint32

type Name struct {
	Namespace string
	Local     string
}

type Names struct {
	Values []Name
}

type RuntimeNameOpKind uint8

const (
	RuntimeNameUnknown RuntimeNameOpKind = iota
	RuntimeNameSymbol
	RuntimeNameNamespace
)

type RuntimeNamePlan struct {
	Ops       []RuntimeNameOp
	Notations []Name
}

type RuntimeNameOp struct {
	Kind      RuntimeNameOpKind
	Name      Name
	Namespace string
}

type TypeKind uint8

const (
	TypeUnknown TypeKind = iota
	TypeSimple
	TypeComplex
	TypeBuiltin
)

type Derivation uint8

const (
	DerivationNone        Derivation = 0
	DerivationExtension   Derivation = 1 << 0
	DerivationRestriction Derivation = 1 << 1
	DerivationList        Derivation = 1 << 2
	DerivationUnion       Derivation = 1 << 3
)

type ElementBlock uint8

const (
	ElementBlockSubstitution ElementBlock = 1 << iota
	ElementBlockExtension
	ElementBlockRestriction
)

type BuiltinType struct {
	Name          Name
	Base          TypeRef
	AnyType       bool
	AnySimpleType bool
	Value         SimpleTypeSpec
}

type TypeRef struct {
	ID      TypeID
	Name    Name
	Builtin bool
}

type TypeDecl struct {
	ID         TypeID
	Name       Name
	Kind       TypeKind
	Base       TypeRef
	Derivation Derivation
	Final      Derivation
	Block      Derivation
	Abstract   bool
	Global     bool
	Origin     string
}

type Element struct {
	ID               ElementID
	Name             Name
	TypeDecl         TypeRef
	SubstitutionHead ElementID
	Default          ValueConstraint
	Fixed            ValueConstraint
	Final            Derivation
	Block            ElementBlock
	Nillable         bool
	Abstract         bool
	Global           bool
	Origin           string
}

type Attribute struct {
	ID       AttributeID
	Name     Name
	TypeDecl TypeRef
	Default  ValueConstraint
	Fixed    ValueConstraint
	Global   bool
	Origin   string
}

type Occurs struct {
	Value     uint32
	Unbounded bool
}

type ValueConstraint struct {
	Lexical string
	Context map[string]string
	Present bool
}

type AttributeUseID uint32
type ParticleID uint32
type WildcardID uint32

type AttributeUseKind uint8

const (
	AttributeOptional AttributeUseKind = iota
	AttributeRequired
	AttributeProhibited
)

type AttributeUse struct {
	ID       AttributeUseID
	Name     Name
	TypeDecl TypeRef
	Use      AttributeUseKind
	Decl     AttributeID
	Default  ValueConstraint
	Fixed    ValueConstraint
}

type ContentKind uint8

const (
	ContentEmpty ContentKind = iota
	ContentSimple
	ContentElement
	ContentAll
)

type ComplexTypePlan struct {
	TypeDecl TypeID
	Mixed    bool
	Content  ContentKind
	TextType TypeRef
	TextSpec SimpleTypeSpec
	Attrs    []AttributeUseID
	AnyAttr  WildcardID
	Particle ParticleID
}

type ParticleKind uint8

const (
	ParticleNone ParticleKind = iota
	ParticleElement
	ParticleWildcard
	ParticleGroup
)

type GroupKind uint8

const (
	GroupSequence GroupKind = iota
	GroupChoice
	GroupAll
)

type Particle struct {
	ID                 ParticleID
	Kind               ParticleKind
	Group              GroupKind
	Element            ElementID
	Wildcard           WildcardID
	Children           []ParticleID
	Min                Occurs
	Max                Occurs
	AllowsSubstitution bool
}

type NamespaceConstraintKind uint8

const (
	NamespaceAny NamespaceConstraintKind = iota
	NamespaceOther
	NamespaceTarget
	NamespaceLocal
	NamespaceList
	NamespaceNotAbsent
)

type ProcessContents uint8

const (
	ProcessStrict ProcessContents = iota
	ProcessLax
	ProcessSkip
)

type Wildcard struct {
	ID              WildcardID
	NamespaceKind   NamespaceConstraintKind
	Namespaces      []string
	TargetNamespace string
	ProcessContents ProcessContents
}

type TypeVariety uint8

const (
	TypeVarietyAtomic TypeVariety = iota
	TypeVarietyList
	TypeVarietyUnion
)

type WhitespaceMode uint8

const (
	WhitespacePreserve WhitespaceMode = iota
	WhitespaceReplace
	WhitespaceCollapse
)

type FacetKind uint8

const (
	FacetUnknown FacetKind = iota
	FacetPattern
	FacetEnumeration
	FacetMinInclusive
	FacetMaxInclusive
	FacetMinExclusive
	FacetMaxExclusive
	FacetMinLength
	FacetMaxLength
	FacetLength
	FacetTotalDigits
	FacetFractionDigits
)

type FacetValue struct {
	Lexical string
	Context map[string]string
}

type FacetSpec struct {
	Kind     FacetKind
	Name     string
	Value    string
	IntValue uint32
	Values   []FacetValue
}

type SimpleTypeSpec struct {
	TypeDecl        TypeID
	Name            Name
	Builtin         bool
	Variety         TypeVariety
	Base            TypeRef
	Item            TypeRef
	Members         []TypeRef
	Facets          []FacetSpec
	Primitive       string
	BuiltinBase     string
	Whitespace      WhitespaceMode
	QNameOrNotation bool
	IntegerDerived  bool
}

type IdentityKind uint8

const (
	IdentityUnknown IdentityKind = iota
	IdentityUnique
	IdentityKey
	IdentityKeyRef
)

type IdentityConstraint struct {
	ID               IdentityID
	Element          ElementID
	Name             Name
	Kind             IdentityKind
	Selector         string
	NamespaceContext map[string]string
	Fields           []IdentityField
	Refer            Name
	ReferID          IdentityID
}

type IdentityField struct {
	XPath    string
	TypeDecl TypeRef
}

type ElementReference struct {
	Name    Name
	Element ElementID
}

type AttributeReference struct {
	Name      Name
	Attribute AttributeID
}

type GroupReference struct {
	Name   Name
	Target Name
}

type GlobalIndexes struct {
	Types      []GlobalTypeIndex
	Elements   []GlobalElementIndex
	Attributes []GlobalAttributeIndex
}

type GlobalTypeIndex struct {
	Name     Name
	TypeDecl TypeID
	Builtin  bool
}

type GlobalElementIndex struct {
	Name    Name
	Element ElementID
}

type GlobalAttributeIndex struct {
	Name      Name
	Attribute AttributeID
}

type Schema struct {
	Names               Names
	BuiltinTypes        []BuiltinType
	Types               []TypeDecl
	SimpleTypes         []SimpleTypeSpec
	ComplexTypes        []ComplexTypePlan
	Elements            []Element
	Attributes          []Attribute
	AttributeUses       []AttributeUse
	Particles           []Particle
	Wildcards           []Wildcard
	IdentityConstraints []IdentityConstraint
	ElementRefs         []ElementReference
	AttributeRefs       []AttributeReference
	GroupRefs           []GroupReference
	RuntimeNames        RuntimeNamePlan
	GlobalIndexes       GlobalIndexes
}
