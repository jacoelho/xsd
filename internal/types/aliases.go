package types

import "github.com/jacoelho/xsd/internal/model"

type QName = model.QName
type NamespaceURI = model.NamespaceURI
type TypeName = model.TypeName

const (
	XSDNamespace   = model.XSDNamespace
	XMLNamespace   = model.XMLNamespace
	NamespaceEmpty = model.NamespaceEmpty

	DerivationExtension    = model.DerivationExtension
	DerivationRestriction  = model.DerivationRestriction
	DerivationSubstitution = model.DerivationSubstitution
	DerivationList         = model.DerivationList
	DerivationUnion        = model.DerivationUnion

	TypeNameAnyType       = model.TypeNameAnyType
	TypeNameAnySimpleType = model.TypeNameAnySimpleType
	TypeNameString        = model.TypeNameString
	TypeNameQName         = model.TypeNameQName
	TypeNameNOTATION      = model.TypeNameNOTATION

	WhiteSpacePreserve = model.WhiteSpacePreserve
	WhiteSpaceReplace  = model.WhiteSpaceReplace
	WhiteSpaceCollapse = model.WhiteSpaceCollapse

	AtomicVariety = model.AtomicVariety
	ListVariety   = model.ListVariety
	UnionVariety  = model.UnionVariety

	Optional   = model.Optional
	Required   = model.Required
	Prohibited = model.Prohibited

	Strict = model.Strict
	Lax    = model.Lax
	Skip   = model.Skip

	NSCAny             = model.NSCAny
	NSCOther           = model.NSCOther
	NSCTargetNamespace = model.NSCTargetNamespace
	NSCLocal           = model.NSCLocal
	NSCList            = model.NSCList
	NSCNotAbsent       = model.NSCNotAbsent

	Sequence = model.Sequence
	Choice   = model.Choice
	AllGroup = model.AllGroup

	UniqueConstraint = model.UniqueConstraint
	KeyConstraint    = model.KeyConstraint
	KeyRefConstraint = model.KeyRefConstraint
)

type Type = model.Type
type DerivedType = model.DerivedType
type SimpleType = model.SimpleType
type ComplexType = model.ComplexType
type BuiltinType = model.BuiltinType
type WhiteSpace = model.WhiteSpace
type SimpleTypeVariety = model.SimpleTypeVariety
type ListType = model.ListType
type UnionType = model.UnionType
type TypedValue = model.TypedValue
type Enumeration = model.Enumeration

type AttributeDecl = model.AttributeDecl
type AttributeGroup = model.AttributeGroup
type AttributeUse = model.AttributeUse
type ConstraintType = model.ConstraintType
type ElementDecl = model.ElementDecl
type IdentityConstraint = model.IdentityConstraint
type Selector = model.Selector
type Field = model.Field
type ModelGroup = model.ModelGroup
type GroupRef = model.GroupRef
type NotationDecl = model.NotationDecl
type AnyElement = model.AnyElement
type AnyAttribute = model.AnyAttribute
type Particle = model.Particle
type Content = model.Content

type NamespaceConstraint = model.NamespaceConstraint
type ProcessContents = model.ProcessContents

type ElementContent = model.ElementContent
type ComplexContent = model.ComplexContent
type EmptyContent = model.EmptyContent
type SimpleContent = model.SimpleContent
type Restriction = model.Restriction

type DerivationMethod = model.DerivationMethod
type DerivationSet = model.DerivationSet

func AsSimpleType(t Type) (*SimpleType, bool) {
	return model.AsSimpleType(t)
}

func AsComplexType(t Type) (*ComplexType, bool) {
	return model.AsComplexType(t)
}

func AsBuiltinType(t Type) (*BuiltinType, bool) {
	return model.AsBuiltinType(t)
}

func AsDerivedType(t Type) (DerivedType, bool) {
	return model.AsDerivedType(t)
}

func NewComplexType(name QName, sourceNamespace NamespaceURI) *ComplexType {
	return model.NewComplexType(name, sourceNamespace)
}

func IsAnyTypeQName(qname QName) bool {
	return model.IsAnyTypeQName(qname)
}

func IsDerivedFrom(derived, base Type) bool {
	return model.IsDerivedFrom(derived, base)
}

func IsValidlyDerivedFrom(derived, base Type) bool {
	return model.IsValidlyDerivedFrom(derived, base)
}

func ListItemType(typ Type) (Type, bool) {
	return model.ListItemType(typ)
}

func NormalizeTypeValue(lexical string, typ Type) (string, error) {
	return model.NormalizeTypeValue(lexical, typ)
}

func NormalizeWhiteSpace(lexical string, typ Type) string {
	return model.NormalizeWhiteSpace(lexical, typ)
}

func NewPlaceholderSimpleType(name QName) *SimpleType {
	return model.NewPlaceholderSimpleType(name)
}

func IsPlaceholderSimpleType(simpleType *SimpleType) bool {
	return model.IsPlaceholderSimpleType(simpleType)
}

const NamespaceTargetPlaceholder = model.NamespaceTargetPlaceholder
