package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// lexicalAttribute keeps an XML attribute's lexical value and presence state
// together after raw-node lookup.
type lexicalAttribute struct {
	Value   string
	Present bool
}

// booleanAttr is the raw attribute state needed to parse an XML Schema boolean
// attribute.
type booleanAttr struct {
	Name     string
	Value    string
	HasValue bool
	Default  bool
}

// parseBooleanAttr parses an XML Schema boolean attribute value.
func parseBooleanAttr(attr booleanAttr) (bool, error) {
	if !attr.HasValue {
		return attr.Default, nil
	}
	parsed, err := valuepkg.ParseBooleanValue(lex.TrimXMLWhitespaceString(attr.Value))
	if err != nil {
		return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid boolean attribute "+diagnostictext.Quote(attr.Name))
	}
	return parsed, nil
}

// formAttr is the raw attribute state needed to parse an XML Schema form
// attribute.
type formAttr struct {
	Name             string
	Value            string
	HasValue         bool
	DefaultQualified bool
}

// schemaDefaults is the parsed xs:schema default state used by compilation.
type schemaDefaults struct {
	TargetNamespace    string
	BlockDefault       DerivationMask
	FinalDefault       DerivationMask
	ElementQualified   bool
	AttributeQualified bool
}

// parseSchemaDefaults validates and parses xs:schema target/default attributes.
func parseSchemaDefaults(attrs schemaDocumentSource) (schemaDefaults, error) {
	if err := validateSchemaTargetNamespace(attrs.TargetNamespace); err != nil {
		return schemaDefaults{}, err
	}
	blockDefault, err := parseDerivationSet(attrs.BlockDefault.Value, "schema blockDefault", derivationBlockDefaultMask)
	if err != nil {
		return schemaDefaults{}, err
	}
	finalDefault, err := parseDerivationSet(attrs.FinalDefault.Value, "schema finalDefault", derivationFinalDefaultMask)
	if err != nil {
		return schemaDefaults{}, err
	}
	elementQualified, err := parseFormDefaultAttr(formAttr{
		Name:     vocab.XSDAttrElementFormDefault,
		Value:    attrs.ElementFormDefault.Value,
		HasValue: attrs.ElementFormDefault.Present,
	})
	if err != nil {
		return schemaDefaults{}, err
	}
	attributeQualified, err := parseFormDefaultAttr(formAttr{
		Name:     vocab.XSDAttrAttributeFormDefault,
		Value:    attrs.AttributeFormDefault.Value,
		HasValue: attrs.AttributeFormDefault.Present,
	})
	if err != nil {
		return schemaDefaults{}, err
	}
	return schemaDefaults{
		TargetNamespace:    attrs.TargetNamespace.Value,
		BlockDefault:       blockDefault,
		FinalDefault:       finalDefault,
		ElementQualified:   elementQualified,
		AttributeQualified: attributeQualified,
	}, nil
}

// parseFormDefaultAttr parses elementFormDefault/attributeFormDefault.
func parseFormDefaultAttr(attr formAttr) (bool, error) {
	return parseFormAttr(attr, "invalid "+diagnostictext.Quote(attr.Name)+" value ")
}

// parseElementFormAttr parses a local xs:element form attribute.
func parseElementFormAttr(attr formAttr) (bool, error) {
	return parseFormAttr(attr, "invalid element form value ")
}

// parseAttributeFormAttr parses a local xs:attribute form attribute.
func parseAttributeFormAttr(attr formAttr) (bool, error) {
	return parseFormAttr(attr, "invalid attribute form ")
}

func parseFormAttr(attr formAttr, invalidPrefix string) (bool, error) {
	if !attr.HasValue {
		return attr.DefaultQualified, nil
	}
	switch attr.Value {
	case vocab.XSDValueQualified:
		return true, nil
	case vocab.XSDValueUnqualified:
		return false, nil
	default:
		return false, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, invalidPrefix+diagnostictext.Quote(attr.Value))
	}
}
