package types

import "github.com/jacoelho/xsd/internal/model"

// QName is an alias of model.QName.
type QName = model.QName

// NamespaceURI is an alias of model.NamespaceURI.
type NamespaceURI = model.NamespaceURI

// TypeName is an alias of model.TypeName.
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

// Type is an alias of model.Type.
type Type = model.Type

// DerivedType is an alias of model.DerivedType.
type DerivedType = model.DerivedType

// SimpleType is an alias of model.SimpleType.
type SimpleType = model.SimpleType

// ComplexType is an alias of model.ComplexType.
type ComplexType = model.ComplexType

// BuiltinType is an alias of model.BuiltinType.
type BuiltinType = model.BuiltinType

// WhiteSpace is an alias of model.WhiteSpace.
type WhiteSpace = model.WhiteSpace

// SimpleTypeVariety is an alias of model.SimpleTypeVariety.
type SimpleTypeVariety = model.SimpleTypeVariety

// ListType is an alias of model.ListType.
type ListType = model.ListType

// UnionType is an alias of model.UnionType.
type UnionType = model.UnionType

// TypedValue is an alias of model.TypedValue.
type TypedValue = model.TypedValue

// Enumeration is an alias of model.Enumeration.
type Enumeration = model.Enumeration

// AttributeDecl is an alias of model.AttributeDecl.
type AttributeDecl = model.AttributeDecl

// AttributeGroup is an alias of model.AttributeGroup.
type AttributeGroup = model.AttributeGroup

// AttributeUse is an alias of model.AttributeUse.
type AttributeUse = model.AttributeUse

// ConstraintType is an alias of model.ConstraintType.
type ConstraintType = model.ConstraintType

// ElementDecl is an alias of model.ElementDecl.
type ElementDecl = model.ElementDecl

// IdentityConstraint is an alias of model.IdentityConstraint.
type IdentityConstraint = model.IdentityConstraint

// Selector is an alias of model.Selector.
type Selector = model.Selector

// Field is an alias of model.Field.
type Field = model.Field

// ModelGroup is an alias of model.ModelGroup.
type ModelGroup = model.ModelGroup

// GroupRef is an alias of model.GroupRef.
type GroupRef = model.GroupRef

// NotationDecl is an alias of model.NotationDecl.
type NotationDecl = model.NotationDecl

// AnyElement is an alias of model.AnyElement.
type AnyElement = model.AnyElement

// AnyAttribute is an alias of model.AnyAttribute.
type AnyAttribute = model.AnyAttribute

// Particle is an alias of model.Particle.
type Particle = model.Particle

// Content is an alias of model.Content.
type Content = model.Content

// NamespaceConstraint is an alias of model.NamespaceConstraint.
type NamespaceConstraint = model.NamespaceConstraint

// ProcessContents is an alias of model.ProcessContents.
type ProcessContents = model.ProcessContents

// ElementContent is an alias of model.ElementContent.
type ElementContent = model.ElementContent

// ComplexContent is an alias of model.ComplexContent.
type ComplexContent = model.ComplexContent

// EmptyContent is an alias of model.EmptyContent.
type EmptyContent = model.EmptyContent

// SimpleContent is an alias of model.SimpleContent.
type SimpleContent = model.SimpleContent

// Restriction is an alias of model.Restriction.
type Restriction = model.Restriction

// DerivationMethod is an alias of model.DerivationMethod.
type DerivationMethod = model.DerivationMethod

// DerivationSet is an alias of model.DerivationSet.
type DerivationSet = model.DerivationSet

// AsSimpleType returns simple type when the type assertion succeeds.
func AsSimpleType(t Type) (*SimpleType, bool) {
	return model.AsSimpleType(t)
}

// AsComplexType returns complex type when the type assertion succeeds.
func AsComplexType(t Type) (*ComplexType, bool) {
	return model.AsComplexType(t)
}

// AsBuiltinType returns builtin type when the type assertion succeeds.
func AsBuiltinType(t Type) (*BuiltinType, bool) {
	return model.AsBuiltinType(t)
}

// AsDerivedType returns derived type when the type assertion succeeds.
func AsDerivedType(t Type) (DerivedType, bool) {
	return model.AsDerivedType(t)
}

// NewComplexType constructs a complex type.
func NewComplexType(name QName, sourceNamespace NamespaceURI) *ComplexType {
	return model.NewComplexType(name, sourceNamespace)
}

// IsAnyTypeQName reports whether qname is xs:anyType.
func IsAnyTypeQName(qname QName) bool {
	return model.IsAnyTypeQName(qname)
}

// IsDerivedFrom reports whether derived is in the derivation chain of base.
func IsDerivedFrom(derived, base Type) bool {
	return model.IsDerivedFrom(derived, base)
}

// IsValidlyDerivedFrom reports whether derived is validly derived from base.
func IsValidlyDerivedFrom(derived, base Type) bool {
	return model.IsValidlyDerivedFrom(derived, base)
}

// ListItemType returns the member type for list simple types.
func ListItemType(typ Type) (Type, bool) {
	return model.ListItemType(typ)
}

// NormalizeTypeValue applies whitespace normalization and type-specific value normalization.
func NormalizeTypeValue(lexical string, typ Type) (string, error) {
	return model.NormalizeTypeValue(lexical, typ)
}

// NormalizeWhiteSpace applies XML Schema whitespace normalization for typ.
func NormalizeWhiteSpace(lexical string, typ Type) string {
	return model.NormalizeWhiteSpace(lexical, typ)
}

// NewPlaceholderSimpleType constructs a placeholder simple type.
func NewPlaceholderSimpleType(name QName) *SimpleType {
	return model.NewPlaceholderSimpleType(name)
}

// IsPlaceholderSimpleType reports whether simpleType is a placeholder forward reference.
func IsPlaceholderSimpleType(simpleType *SimpleType) bool {
	return model.IsPlaceholderSimpleType(simpleType)
}

const NamespaceTargetPlaceholder = model.NamespaceTargetPlaceholder
