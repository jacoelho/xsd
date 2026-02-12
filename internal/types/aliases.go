package types

import "github.com/jacoelho/xsd/internal/model"

// QName is an alias of model.QName.
type QName = model.QName

// NamespaceURI is an alias of model.NamespaceURI.
type NamespaceURI = model.NamespaceURI

// TypeName is an alias of model.TypeName.
type TypeName = model.TypeName

const (
	// XSDNamespace is an exported constant.
	XSDNamespace = model.XSDNamespace
	// XMLNamespace is an exported constant.
	XMLNamespace = model.XMLNamespace
	// NamespaceEmpty is an exported constant.
	NamespaceEmpty = model.NamespaceEmpty

	// DerivationExtension is an exported constant.
	DerivationExtension = model.DerivationExtension
	// DerivationRestriction is an exported constant.
	DerivationRestriction = model.DerivationRestriction
	// DerivationSubstitution is an exported constant.
	DerivationSubstitution = model.DerivationSubstitution
	// DerivationList is an exported constant.
	DerivationList = model.DerivationList
	// DerivationUnion is an exported constant.
	DerivationUnion = model.DerivationUnion

	// TypeNameAnyType is an exported constant.
	TypeNameAnyType = model.TypeNameAnyType
	// TypeNameAnySimpleType is an exported constant.
	TypeNameAnySimpleType = model.TypeNameAnySimpleType
	// TypeNameString is an exported constant.
	TypeNameString = model.TypeNameString
	// TypeNameQName is an exported constant.
	TypeNameQName = model.TypeNameQName
	// TypeNameNOTATION is an exported constant.
	TypeNameNOTATION = model.TypeNameNOTATION

	// WhiteSpacePreserve is an exported constant.
	WhiteSpacePreserve = model.WhiteSpacePreserve
	// WhiteSpaceReplace is an exported constant.
	WhiteSpaceReplace = model.WhiteSpaceReplace
	// WhiteSpaceCollapse is an exported constant.
	WhiteSpaceCollapse = model.WhiteSpaceCollapse

	// AtomicVariety is an exported constant.
	AtomicVariety = model.AtomicVariety
	// ListVariety is an exported constant.
	ListVariety = model.ListVariety
	// UnionVariety is an exported constant.
	UnionVariety = model.UnionVariety

	// Optional is an exported constant.
	Optional = model.Optional
	// Required is an exported constant.
	Required = model.Required
	// Prohibited is an exported constant.
	Prohibited = model.Prohibited

	// Strict is an exported constant.
	Strict = model.Strict
	// Lax is an exported constant.
	Lax = model.Lax
	// Skip is an exported constant.
	Skip = model.Skip

	// NSCAny is an exported constant.
	NSCAny = model.NSCAny
	// NSCOther is an exported constant.
	NSCOther = model.NSCOther
	// NSCTargetNamespace is an exported constant.
	NSCTargetNamespace = model.NSCTargetNamespace
	// NSCLocal is an exported constant.
	NSCLocal = model.NSCLocal
	// NSCList is an exported constant.
	NSCList = model.NSCList
	// NSCNotAbsent is an exported constant.
	NSCNotAbsent = model.NSCNotAbsent

	// Sequence is an exported constant.
	Sequence = model.Sequence
	// Choice is an exported constant.
	Choice = model.Choice
	// AllGroup is an exported constant.
	AllGroup = model.AllGroup

	// UniqueConstraint is an exported constant.
	UniqueConstraint = model.UniqueConstraint
	// KeyConstraint is an exported constant.
	KeyConstraint = model.KeyConstraint
	// KeyRefConstraint is an exported constant.
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

// AsSimpleType is an exported function.
func AsSimpleType(t Type) (*SimpleType, bool) {
	return model.AsSimpleType(t)
}

// AsComplexType is an exported function.
func AsComplexType(t Type) (*ComplexType, bool) {
	return model.AsComplexType(t)
}

// AsBuiltinType is an exported function.
func AsBuiltinType(t Type) (*BuiltinType, bool) {
	return model.AsBuiltinType(t)
}

// AsDerivedType is an exported function.
func AsDerivedType(t Type) (DerivedType, bool) {
	return model.AsDerivedType(t)
}

// NewComplexType is an exported function.
func NewComplexType(name QName, sourceNamespace NamespaceURI) *ComplexType {
	return model.NewComplexType(name, sourceNamespace)
}

// IsAnyTypeQName is an exported function.
func IsAnyTypeQName(qname QName) bool {
	return model.IsAnyTypeQName(qname)
}

// IsDerivedFrom is an exported function.
func IsDerivedFrom(derived, base Type) bool {
	return model.IsDerivedFrom(derived, base)
}

// IsValidlyDerivedFrom is an exported function.
func IsValidlyDerivedFrom(derived, base Type) bool {
	return model.IsValidlyDerivedFrom(derived, base)
}

// ListItemType is an exported function.
func ListItemType(typ Type) (Type, bool) {
	return model.ListItemType(typ)
}

// NormalizeTypeValue is an exported function.
func NormalizeTypeValue(lexical string, typ Type) (string, error) {
	return model.NormalizeTypeValue(lexical, typ)
}

// NormalizeWhiteSpace is an exported function.
func NormalizeWhiteSpace(lexical string, typ Type) string {
	return model.NormalizeWhiteSpace(lexical, typ)
}

// NewPlaceholderSimpleType is an exported function.
func NewPlaceholderSimpleType(name QName) *SimpleType {
	return model.NewPlaceholderSimpleType(name)
}

// IsPlaceholderSimpleType is an exported function.
func IsPlaceholderSimpleType(simpleType *SimpleType) bool {
	return model.IsPlaceholderSimpleType(simpleType)
}

// NamespaceTargetPlaceholder is an exported constant.
const NamespaceTargetPlaceholder = model.NamespaceTargetPlaceholder
