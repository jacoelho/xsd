package schema

import (
	"github.com/jacoelho/xsd/internal/lex"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// LexicalAttribute keeps an XML attribute's lexical value and presence state
// together after raw-node lookup.
type LexicalAttribute struct {
	Value   string
	Present bool
}

// BooleanAttr is the raw attribute state needed to parse an XML Schema boolean
// attribute.
type BooleanAttr struct {
	Name     string
	Value    string
	HasValue bool
	Default  bool
}

// ParseBooleanAttr parses an XML Schema boolean attribute value.
func ParseBooleanAttr(attr BooleanAttr) (bool, error) {
	if !attr.HasValue {
		return attr.Default, nil
	}
	switch lex.TrimXMLWhitespaceString(attr.Value) {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid boolean attribute "+attr.Name)
	}
}

// FormAttr is the raw attribute state needed to parse an XML Schema form
// attribute.
type FormAttr struct {
	Name             string
	Value            string
	HasValue         bool
	DefaultQualified bool
}

// SchemaDefaults is the parsed xs:schema default state used by compilation.
type SchemaDefaults struct {
	TargetNamespace    string
	BlockDefault       DerivationMask
	FinalDefault       DerivationMask
	ElementQualified   bool
	AttributeQualified bool
}

// ParseSchemaDefaults validates and parses xs:schema target/default attributes.
func ParseSchemaDefaults(attrs schemaDocumentSource) (SchemaDefaults, error) {
	if err := ValidateSchemaTargetNamespace(attrs.TargetNamespace); err != nil {
		return SchemaDefaults{}, err
	}
	blockDefault, err := ParseDerivationSet(attrs.BlockDefault.Value, "schema blockDefault", DerivationBlockDefaultMask)
	if err != nil {
		return SchemaDefaults{}, err
	}
	finalDefault, err := ParseDerivationSet(attrs.FinalDefault.Value, "schema finalDefault", DerivationFinalDefaultMask)
	if err != nil {
		return SchemaDefaults{}, err
	}
	elementQualified, err := ParseFormDefaultAttr(FormAttr{
		Name:     vocab.XSDAttrElementFormDefault,
		Value:    attrs.ElementFormDefault.Value,
		HasValue: attrs.ElementFormDefault.Present,
	})
	if err != nil {
		return SchemaDefaults{}, err
	}
	attributeQualified, err := ParseFormDefaultAttr(FormAttr{
		Name:     vocab.XSDAttrAttributeFormDefault,
		Value:    attrs.AttributeFormDefault.Value,
		HasValue: attrs.AttributeFormDefault.Present,
	})
	if err != nil {
		return SchemaDefaults{}, err
	}
	return SchemaDefaults{
		TargetNamespace:    attrs.TargetNamespace.Value,
		BlockDefault:       blockDefault,
		FinalDefault:       finalDefault,
		ElementQualified:   elementQualified,
		AttributeQualified: attributeQualified,
	}, nil
}

// ParseFormDefaultAttr parses elementFormDefault/attributeFormDefault.
func ParseFormDefaultAttr(attr FormAttr) (bool, error) {
	return parseFormAttr(attr, func(value string) string {
		return "invalid " + attr.Name + " value " + value
	})
}

// ParseElementFormAttr parses a local xs:element form attribute.
func ParseElementFormAttr(attr FormAttr) (bool, error) {
	return parseFormAttr(attr, func(value string) string {
		return "invalid element form value " + value
	})
}

// ParseAttributeFormAttr parses a local xs:attribute form attribute.
func ParseAttributeFormAttr(attr FormAttr) (bool, error) {
	return parseFormAttr(attr, func(value string) string {
		return "invalid attribute form " + value
	})
}

func parseFormAttr(attr FormAttr, invalidMessage func(string) string) (bool, error) {
	if !attr.HasValue {
		return attr.DefaultQualified, nil
	}
	switch attr.Value {
	case vocab.XSDValueQualified:
		return true, nil
	case vocab.XSDValueUnqualified:
		return false, nil
	default:
		return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, invalidMessage(attr.Value))
	}
}
