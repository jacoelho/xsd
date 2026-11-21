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

// builtinXSIAttributes is the only source of XSI attribute declarations:
// source admission rejects them, and publication requires exactly these four.
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

// builtinAnyTypeLocalName returns the fixed local name for xs:anyType.
func builtinAnyTypeLocalName() string {
	return vocab.XSDValueAnyType
}

// builtinAnyTypeWildcard returns the lax ##any wildcard that xs:anyType uses
// for both its attribute wildcard and its content particle.
func builtinAnyTypeWildcard() wildcardRuntime {
	return wildcardRuntime{Mode: WildcardAny, Process: ProcessLax}
}

// builtinAnyTypeWildcardShape reports whether w admits the elements or
// attributes builtinAnyTypeWildcard admits, processed the same way.
func builtinAnyTypeWildcardShape(w wildcardRuntime) bool {
	want := builtinAnyTypeWildcard()
	return w.Mode == want.Mode && w.Process == want.Process
}

// builtinAnyTypeAttributeUseSet returns the attribute-use set used by
// xs:anyType.
func builtinAnyTypeAttributeUseSet(wildcard WildcardID) attributeUseSet {
	return attributeUseSet{
		Wildcard:         wildcard,
		WildcardBase:     NoWildcard,
		WildcardDeclared: wildcard,
	}
}

// builtinAnyTypeContentModel returns the 1..1 sequence that is the
// {content type} particle of xs:anyType (Structures §3.4.7); its one member is
// builtinAnyTypeContentParticle.
func builtinAnyTypeContentModel() contentModelNode {
	return contentModelNode{Kind: modelSequence, Occurs: occurrence{Min: 1, Max: 1}}
}

// builtinAnyTypeContentParticle returns the lax ##any wildcard particle,
// 0..unbounded, that xs:anyType's content sequence holds.
func builtinAnyTypeContentParticle(wildcard WildcardID) contentParticle {
	return wildcardParticle(wildcard, occurrence{Unbounded: true})
}

// builtinAnyTypeContentParticleShape reports whether p, whose wildcard is in
// wildcards, is the particle builtinAnyTypeContentParticle returns for a
// wildcard of builtinAnyTypeWildcardShape.
func builtinAnyTypeContentParticleShape(wildcards []wildcardRuntime, p contentParticle) bool {
	if p != builtinAnyTypeContentParticle(p.Wildcard) {
		return false
	}
	w, ok := wildcardByID(wildcards, p.Wildcard)
	return ok && builtinAnyTypeWildcardShape(w)
}

// builtinAnyTypeComplexType returns the fixed xs:anyType complex-type
// declaration using already allocated child component IDs.
func builtinAnyTypeComplexType(name QName, content contentModelID, attrs attributeUseSetID) complexTypeMetadata {
	return complexTypeMetadata{
		Name:        name,
		Content:     content,
		Attrs:       attrs,
		TextType:    NoSimpleType,
		ContentKind: contentMixed,
	}
}

// builtinDeclarationCounts is the schema projection needed to verify that
// fixed declaration slots were seeded.
type builtinDeclarationCounts struct {
	SimpleTypes      int
	Attributes       int
	ComplexTypes     int
	Wildcards        int
	AttributeUseSets int
	Models           int
}

const builtinComplexTypeDeclarationCount = 1

// builtinSimpleTypeCount returns the number of fixed value-program type IDs
// represented by schema declaration records.
func builtinSimpleTypeCount() int {
	return int(builtinSchemaSimpleTypeCount)
}

// builtinComplexTypeCount returns the number of fixed complex declarations.
func builtinComplexTypeCount() int {
	return builtinComplexTypeDeclarationCount
}

// builtinGlobalTypeCount returns the number of fixed global type bindings.
func builtinGlobalTypeCount() int {
	return int(value.BuiltinTypeCount) + builtinComplexTypeDeclarationCount
}

// validateBuiltinDeclarationCounts validates the required fixed declaration
// cardinalities seeded into every schema build.
func validateBuiltinDeclarationCounts(counts builtinDeclarationCounts) error {
	if counts.SimpleTypes < builtinSimpleTypeCount() ||
		counts.Attributes < len(builtinXSIAttributes) ||
		counts.ComplexTypes < builtinComplexTypeDeclarationCount ||
		counts.Wildcards == 0 ||
		counts.AttributeUseSets == 0 ||
		counts.Models == 0 {
		return errors.New("runtime is missing builtin declarations")
	}
	return nil
}

func builtinAnyTypeQName(names *nameTable) (QName, bool) {
	if names == nil {
		return QName{}, false
	}
	return names.LookupQName(vocab.XSDNamespaceURI, vocab.XSDValueAnyType)
}
