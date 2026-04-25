package schemaast

// DocumentSet is the compile boundary for loaded schema documents.
type DocumentSet struct {
	Documents []SchemaDocument
}

// SchemaDocument records one loaded schema document.
type SchemaDocument struct {
	Location          string
	TargetNamespace   NamespaceURI
	NamespaceContexts []NamespaceContext
	Decls             []TopLevelDecl
	Directives        []Directive
	Defaults          SchemaDefaults
	Imports           []ImportInfo
	Includes          []IncludeInfo
}

// NamespaceContextID identifies a captured lexical namespace context.
type NamespaceContextID uint32

// NamespaceBinding records one prefix binding in a lexical namespace context.
type NamespaceBinding struct {
	Prefix string
	URI    NamespaceURI
}

// NamespaceContext records namespace bindings visible at a schema declaration.
type NamespaceContext struct {
	Bindings []NamespaceBinding
}

// SchemaDefaults records root-level schema defaults.
type SchemaDefaults struct {
	ElementFormDefault   Form
	AttributeFormDefault Form
	BlockDefault         DerivationSet
	FinalDefault         DerivationSet
}

// Form represents qualified/unqualified form defaults.
type Form int

const (
	Unqualified Form = iota
	Qualified
)

// FormChoice records explicit local form settings.
type FormChoice int

const (
	FormUnspecified FormChoice = iota
	FormQualified
	FormUnqualified
)

const FormDefault = FormUnspecified

// AttributeUse records lexical attribute use.
type AttributeUse int

const (
	Optional AttributeUse = iota
	Required
	Prohibited
)

// DeclKind identifies a parse-only declaration kind.
type DeclKind uint8

const (
	DeclSimpleType DeclKind = iota + 1
	DeclComplexType
	DeclElement
	DeclAttribute
	DeclGroup
	DeclAttributeGroup
	DeclNotation
)

// TopLevelDecl preserves top-level declaration order.
type TopLevelDecl struct {
	Name           QName
	SimpleType     *SimpleTypeDecl
	ComplexType    *ComplexTypeDecl
	Element        *ElementDecl
	Attribute      *AttributeDecl
	Group          *GroupDecl
	AttributeGroup *AttributeGroupDecl
	Notation       *NotationDecl
	Kind           DeclKind
	Origin         string
}

// TypeUse is a lexical type reference or inline type declaration.
type TypeUse struct {
	Name    QName
	Simple  *SimpleTypeDecl
	Complex *ComplexTypeDecl
}

// ValueConstraintDecl records a lexical default/fixed value with its namespace context.
type ValueConstraintDecl struct {
	Lexical            string
	NamespaceContextID NamespaceContextID
	Present            bool
}

// FacetDecl records a lexical simple-type facet.
type FacetDecl struct {
	Name               string
	Lexical            string
	NamespaceContextID NamespaceContextID
	Fixed              bool
}

// SimpleDerivationKind identifies simple type derivation syntax.
type SimpleDerivationKind uint8

const (
	SimpleDerivationNone SimpleDerivationKind = iota
	SimpleDerivationRestriction
	SimpleDerivationList
	SimpleDerivationUnion
)

// SimpleTypeDecl is a parse-only simple type declaration.
type SimpleTypeDecl struct {
	Name            QName
	Base            QName
	ItemType        QName
	MemberTypes     []QName
	InlineBase      *SimpleTypeDecl
	InlineItem      *SimpleTypeDecl
	InlineMembers   []SimpleTypeDecl
	Facets          []FacetDecl
	Final           DerivationSet
	SourceNamespace NamespaceURI
	Kind            SimpleDerivationKind
	Origin          string
}

// ComplexDerivationKind identifies complex type derivation syntax.
type ComplexDerivationKind uint8

const (
	ComplexDerivationNone ComplexDerivationKind = iota
	ComplexDerivationRestriction
	ComplexDerivationExtension
)

// ComplexContentKind identifies complex type content syntax.
type ComplexContentKind uint8

const (
	ComplexContentNone ComplexContentKind = iota
	ComplexContentSimple
	ComplexContentComplex
)

// ComplexTypeDecl is a parse-only complex type declaration.
type ComplexTypeDecl struct {
	Name            QName
	Base            QName
	Attributes      []AttributeUseDecl
	AttributeGroups []QName
	AnyAttribute    *WildcardDecl
	Particle        *ParticleDecl
	SimpleType      *SimpleTypeDecl
	SimpleFacets    []FacetDecl
	Final           DerivationSet
	Block           DerivationSet
	SourceNamespace NamespaceURI
	Derivation      ComplexDerivationKind
	Content         ComplexContentKind
	Abstract        bool
	Mixed           bool
	MixedSet        bool
	Origin          string
}

// ElementDecl is a parse-only element declaration.
type ElementDecl struct {
	Name               QName
	Ref                QName
	Type               TypeUse
	Default            ValueConstraintDecl
	Fixed              ValueConstraintDecl
	SubstitutionGroup  QName
	Identity           []IdentityDecl
	MinOccurs          Occurs
	MaxOccurs          Occurs
	Final              DerivationSet
	Block              DerivationSet
	Form               FormChoice
	NamespaceContextID NamespaceContextID
	SourceNamespace    NamespaceURI
	Abstract           bool
	Nillable           bool
	Global             bool
	Origin             string
}

// AttributeDecl is a parse-only attribute declaration.
type AttributeDecl struct {
	Name               QName
	Ref                QName
	Type               TypeUse
	Default            ValueConstraintDecl
	Fixed              ValueConstraintDecl
	Use                AttributeUse
	Form               FormChoice
	NamespaceContextID NamespaceContextID
	SourceNamespace    NamespaceURI
	Global             bool
	Origin             string
}

// AttributeUseDecl is a parse-only attribute use, attributeGroup ref, or anyAttribute.
type AttributeUseDecl struct {
	Attribute      *AttributeDecl
	AttributeGroup QName
}

// GroupDecl is a parse-only model group declaration.
type GroupDecl struct {
	Name            QName
	Ref             QName
	Particle        *ParticleDecl
	MinOccurs       Occurs
	MaxOccurs       Occurs
	SourceNamespace NamespaceURI
	Origin          string
}

// AttributeGroupDecl is a parse-only attribute group declaration.
type AttributeGroupDecl struct {
	Name            QName
	Ref             QName
	Attributes      []AttributeUseDecl
	AttributeGroups []QName
	AnyAttribute    *WildcardDecl
	SourceNamespace NamespaceURI
	Origin          string
}

// NotationDecl is a parse-only notation declaration.
type NotationDecl struct {
	Name            QName
	Public          string
	System          string
	SourceNamespace NamespaceURI
	Origin          string
}

// ParticleKind identifies a parse-only particle.
type ParticleKind uint8

const (
	ParticleElement ParticleKind = iota + 1
	ParticleWildcard
	ParticleGroup
	ParticleSequence
	ParticleChoice
	ParticleAll
)

// ParticleDecl is a parse-only particle declaration.
type ParticleDecl struct {
	Element  *ElementDecl
	Wildcard *WildcardDecl
	GroupRef QName
	Children []ParticleDecl
	Min      Occurs
	Max      Occurs
	Kind     ParticleKind
}

// WildcardDecl is a parse-only wildcard declaration.
type WildcardDecl struct {
	TargetNamespace NamespaceURI
	NamespaceList   []NamespaceURI
	Namespace       NamespaceConstraint
	ProcessContents ProcessContents
	MinOccurs       Occurs
	MaxOccurs       Occurs
}

// IdentityKind identifies a parse-only identity constraint.
type IdentityKind uint8

const (
	IdentityKey IdentityKind = iota + 1
	IdentityUnique
	IdentityKeyref
)

// IdentityDecl is a parse-only identity constraint.
type IdentityDecl struct {
	Name               QName
	Refer              QName
	Selector           string
	Fields             []string
	NamespaceContextID NamespaceContextID
	Kind               IdentityKind
}
