package schema

import (
	"errors"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
)

const (
	builtinSchemaLocationTypeID  = value.BuiltinTypeCount
	builtinSchemaSimpleTypeCount = value.BuiltinTypeCount + 1
)

type builtinAttributeDefinition struct {
	local string
	typ   SimpleTypeID
}

var builtinXSIAttributes = [...]builtinAttributeDefinition{
	{local: vocab.XSIAttrType, typ: value.BuiltinType(value.PrimitiveQName)},
	{local: vocab.XSIAttrNil, typ: value.BuiltinType(value.PrimitiveBoolean)},
	{local: vocab.XSIAttrSchemaLocation, typ: builtinSchemaLocationTypeID},
	{local: vocab.XSIAttrNoNamespaceSchemaLocation, typ: value.BuiltinType(value.PrimitiveAnyURI)},
}

func builtinXSIAttributeNames() []ExpandedName {
	names := make([]ExpandedName, len(builtinXSIAttributes))
	for i, attribute := range builtinXSIAttributes {
		names[i] = ExpandedName{Namespace: vocab.XSINamespaceURI, Local: attribute.local}
	}
	return names
}

func builtinXSIAttributeType(local string) (SimpleTypeID, bool) {
	for _, attribute := range builtinXSIAttributes {
		if attribute.local == local {
			return attribute.typ, true
		}
	}
	return NoSimpleType, false
}

func builtinSchemaLocationTypeSpec() value.TypeSpec {
	return value.TypeSpec{
		Variety: value.List, Primitive: value.PrimitiveString,
		Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
		Base: value.AnySimpleTypeID, ListItem: value.BuiltinType(value.PrimitiveAnyURI),
	}
}

// BuiltinAnyTypeLocalName returns the fixed local name for xs:anyType.
func BuiltinAnyTypeLocalName() string {
	return vocab.XSDValueAnyType
}

// BuiltinAnyTypeWildcard returns the attribute wildcard used by xs:anyType.
func BuiltinAnyTypeWildcard() Wildcard {
	return Wildcard{Mode: WildcardAny, Process: ProcessLax}
}

// BuiltinAnyTypeAttributeUseSet returns the attribute-use set used by
// xs:anyType.
func BuiltinAnyTypeAttributeUseSet(wildcard WildcardID) AttributeUseSet {
	return AttributeUseSet{
		Wildcard:         wildcard,
		WildcardBase:     NoWildcard,
		WildcardDeclared: wildcard,
	}
}

// BuiltinAnyTypeContentModel returns the content model used by xs:anyType.
func BuiltinAnyTypeContentModel() ContentModel {
	return ContentModel{Kind: ModelAny, Mixed: true}
}

// BuiltinAnyTypeComplexType returns the fixed xs:anyType complex-type
// declaration using already allocated child component IDs.
func BuiltinAnyTypeComplexType(name QName, content ContentModelID, attrs AttributeUseSetID) ComplexType {
	return ComplexType{
		Name:        name,
		Content:     content,
		Attrs:       attrs,
		TextType:    NoSimpleType,
		ContentKind: ContentMixed,
	}
}

// BuiltinDeclarationCounts is the schema projection needed to verify that
// fixed declaration slots were seeded.
type BuiltinDeclarationCounts struct {
	SimpleTypes      int
	Attributes       int
	ComplexTypes     int
	Wildcards        int
	AttributeUseSets int
	Models           int
}

const builtinComplexTypeDeclarationCount = 1

// BuiltinSimpleTypeCount returns the number of fixed value-program type IDs
// represented by schema declaration records.
func BuiltinSimpleTypeCount() int {
	return int(builtinSchemaSimpleTypeCount)
}

// BuiltinComplexTypeCount returns the number of fixed complex declarations.
func BuiltinComplexTypeCount() int {
	return builtinComplexTypeDeclarationCount
}

// BuiltinGlobalTypeCount returns the number of fixed global type bindings.
func BuiltinGlobalTypeCount() int {
	return int(value.BuiltinTypeCount) + builtinComplexTypeDeclarationCount
}

// ValidateBuiltinDeclarationCounts validates the required fixed declaration
// cardinalities seeded into every schema build.
func ValidateBuiltinDeclarationCounts(counts BuiltinDeclarationCounts) error {
	if counts.SimpleTypes < BuiltinSimpleTypeCount() ||
		counts.Attributes < len(builtinXSIAttributes) ||
		counts.ComplexTypes < builtinComplexTypeDeclarationCount ||
		counts.Wildcards == 0 ||
		counts.AttributeUseSets == 0 ||
		counts.Models == 0 {
		return errors.New("runtime is missing builtin declarations")
	}
	return nil
}

func builtinAnyTypeQName(names *NameTable) (QName, bool) {
	if names == nil {
		return QName{}, false
	}
	return names.LookupQName(vocab.XSDNamespaceURI, vocab.XSDValueAnyType)
}
