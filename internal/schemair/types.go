package schemair

import (
	"maps"
	"slices"
)

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

type TypeRefKind uint8

const (
	TypeRefNone TypeRefKind = iota
	TypeRefBuiltin
	TypeRefUser
	TypeRefUnresolved
)

type TypeRef struct {
	id      TypeID
	name    Name
	builtin bool
}

func BuiltinTypeRef(id TypeID, name Name) TypeRef {
	return TypeRef{id: id, name: name, builtin: true}
}

func UserTypeRef(id TypeID, name Name) TypeRef {
	if id == 0 {
		return UnresolvedTypeRef(name)
	}
	return TypeRef{id: id, name: name}
}

func UnresolvedTypeRef(name Name) TypeRef {
	if name == (Name{}) {
		return NoTypeRef()
	}
	return TypeRef{name: name}
}

func NoTypeRef() TypeRef {
	return TypeRef{}
}

func (r TypeRef) Kind() TypeRefKind {
	switch {
	case r.builtin:
		return TypeRefBuiltin
	case r.id != 0:
		return TypeRefUser
	case r.name != (Name{}):
		return TypeRefUnresolved
	default:
		return TypeRefNone
	}
}

func (r TypeRef) IsZero() bool {
	return r.Kind() == TypeRefNone
}

func (r TypeRef) IsBuiltin() bool {
	return r.Kind() == TypeRefBuiltin
}

func (r TypeRef) IsUser() bool {
	return r.Kind() == TypeRefUser
}

func (r TypeRef) IsUnresolved() bool {
	return r.Kind() == TypeRefUnresolved
}

func (r TypeRef) TypeID() TypeID {
	return r.id
}

func (r TypeRef) TypeName() Name {
	return r.name
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

type ValueConstraintKind uint8

const (
	ValueConstraintNone ValueConstraintKind = iota
	ValueConstraintDefault
	ValueConstraintFixed
)

type ValueConstraint struct {
	lexical string
	context map[string]string
	present bool
	kind    ValueConstraintKind
}

func NoValueConstraint() ValueConstraint {
	return ValueConstraint{}
}

func DefaultValueConstraint(lexical string, context map[string]string) ValueConstraint {
	return valueConstraint(ValueConstraintDefault, lexical, context)
}

func FixedValueConstraint(lexical string, context map[string]string) ValueConstraint {
	return valueConstraint(ValueConstraintFixed, lexical, context)
}

func valueConstraint(kind ValueConstraintKind, lexical string, context map[string]string) ValueConstraint {
	if kind == ValueConstraintNone {
		return NoValueConstraint()
	}
	return ValueConstraint{
		lexical: lexical,
		context: maps.Clone(context),
		present: true,
		kind:    kind,
	}
}

func (v ValueConstraint) Kind() ValueConstraintKind {
	if !v.present {
		return ValueConstraintNone
	}
	if v.kind != ValueConstraintNone {
		return v.kind
	}
	return ValueConstraintDefault
}

func (v ValueConstraint) IsPresent() bool {
	return v.Kind() != ValueConstraintNone
}

func (v ValueConstraint) LexicalValue() string {
	if !v.IsPresent() {
		return ""
	}
	return v.lexical
}

func (v ValueConstraint) NamespaceContext() map[string]string {
	if !v.IsPresent() {
		return nil
	}
	return maps.Clone(v.context)
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
	id                 ParticleID
	kind               ParticleKind
	group              GroupKind
	element            ElementID
	wildcard           WildcardID
	children           []ParticleID
	min                Occurs
	max                Occurs
	allowsSubstitution bool
}

func NoParticle(id ParticleID) Particle {
	return Particle{id: id, kind: ParticleNone}
}

func ElementParticle(id ParticleID, element ElementID, minOccurs Occurs, maxOccurs Occurs, allowsSubstitution bool) Particle {
	if element == 0 {
		return NoParticle(id)
	}
	return Particle{
		id:                 id,
		kind:               ParticleElement,
		element:            element,
		min:                minOccurs,
		max:                maxOccurs,
		allowsSubstitution: allowsSubstitution,
	}
}

func WildcardParticle(id ParticleID, wildcard WildcardID, minOccurs Occurs, maxOccurs Occurs) Particle {
	if wildcard == 0 {
		return NoParticle(id)
	}
	return Particle{
		id:       id,
		kind:     ParticleWildcard,
		wildcard: wildcard,
		min:      minOccurs,
		max:      maxOccurs,
	}
}

func GroupParticle(id ParticleID, group GroupKind, children []ParticleID, minOccurs Occurs, maxOccurs Occurs) Particle {
	return Particle{
		id:       id,
		kind:     ParticleGroup,
		group:    group,
		children: slices.Clone(children),
		min:      minOccurs,
		max:      maxOccurs,
	}
}

func (p Particle) ParticleKind() ParticleKind {
	return p.kind
}

func (p Particle) ID() ParticleID {
	return p.id
}

func (p Particle) ElementID() (ElementID, bool) {
	if p.kind != ParticleElement || p.element == 0 {
		return 0, false
	}
	return p.element, true
}

func (p Particle) WildcardID() (WildcardID, bool) {
	if p.kind != ParticleWildcard || p.wildcard == 0 {
		return 0, false
	}
	return p.wildcard, true
}

func (p Particle) GroupKindValue() (GroupKind, bool) {
	if p.kind != ParticleGroup {
		return 0, false
	}
	return p.group, true
}

func (p Particle) GroupKind() GroupKind {
	if p.kind != ParticleGroup {
		return 0
	}
	return p.group
}

func (p Particle) ChildParticles() []ParticleID {
	if p.kind != ParticleGroup {
		return nil
	}
	return slices.Clone(p.children)
}

func (p Particle) ElementRef() ElementID {
	id, _ := p.ElementID()
	return id
}

func (p Particle) WildcardRef() WildcardID {
	id, _ := p.WildcardID()
	return id
}

func (p Particle) OccursRange() (Occurs, Occurs) {
	return p.min, p.max
}

func (p Particle) MinOccurs() Occurs {
	return p.min
}

func (p Particle) MaxOccurs() Occurs {
	return p.max
}

func (p Particle) WithOccurs(minOccurs Occurs, maxOccurs Occurs) Particle {
	p.min = minOccurs
	p.max = maxOccurs
	return p
}

func (p Particle) AllowsSubstitutionGroup() bool {
	return p.kind == ParticleElement && p.allowsSubstitution
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
