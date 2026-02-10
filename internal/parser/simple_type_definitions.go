package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

// parseSimpleType parses a top-level simpleType definition
func parseSimpleType(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) error {
	name := model.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
	if name == "" {
		return fmt.Errorf("simpleType missing name attribute")
	}

	if err := validateOptionalID(doc, elem, "simpleType", schema); err != nil {
		return err
	}

	st, err := parseSimpleTypeDefinition(doc, elem, schema)
	if err != nil {
		return err
	}

	st.QName = model.QName{
		Namespace: schema.TargetNamespace,
		Local:     name,
	}
	st.SourceNamespace = schema.TargetNamespace

	if doc.HasAttribute(elem, "final") {
		finalAttr := doc.GetAttribute(elem, "final")
		if model.TrimXMLWhitespace(finalAttr) == "" {
			return fmt.Errorf("final attribute cannot be empty")
		}
		final, err := parseDerivationSetWithValidation(finalAttr, model.DerivationSet(model.DerivationRestriction|model.DerivationList|model.DerivationUnion))
		if err != nil {
			return fmt.Errorf("invalid final attribute value '%s': %w", finalAttr, err)
		}
		st.Final = final
	} else if schema.FinalDefault != 0 {
		st.Final = schema.FinalDefault & model.DerivationSet(model.DerivationRestriction|model.DerivationList|model.DerivationUnion)
	}

	if _, exists := schema.TypeDefs[st.QName]; exists {
		return fmt.Errorf("duplicate type definition: '%s'", st.QName)
	}

	schema.TypeDefs[st.QName] = st
	schema.addGlobalDecl(GlobalDeclType, st.QName)
	return nil
}

// parseInlineSimpleType parses an inline simpleType definition.
func parseInlineSimpleType(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) (*model.SimpleType, error) {
	if doc.GetAttribute(elem, "name") != "" {
		return nil, fmt.Errorf("inline simpleType cannot have 'name' attribute")
	}
	if err := validateOptionalID(doc, elem, "simpleType", schema); err != nil {
		return nil, err
	}
	return parseSimpleTypeDefinition(doc, elem, schema)
}

// parseSimpleTypeDefinition parses the derivation content of a simpleType element.
func parseSimpleTypeDefinition(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) (*model.SimpleType, error) {
	var parsed *model.SimpleType
	seenDerivation := false

	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			continue
		case "restriction":
			if seenDerivation {
				return nil, fmt.Errorf("simpleType must have exactly one derivation child (restriction, list, or union)")
			}
			seenDerivation = true
			var err error
			parsed, err = parseRestrictionDerivation(doc, child, schema)
			if err != nil {
				return nil, err
			}
		case "list":
			if seenDerivation {
				return nil, fmt.Errorf("simpleType must have exactly one derivation child (restriction, list, or union)")
			}
			seenDerivation = true
			var err error
			parsed, err = parseListDerivation(doc, child, schema)
			if err != nil {
				return nil, err
			}
		case "union":
			if seenDerivation {
				return nil, fmt.Errorf("simpleType must have exactly one derivation child (restriction, list, or union)")
			}
			seenDerivation = true
			var err error
			parsed, err = parseUnionDerivation(doc, child, schema)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("simpleType: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	if parsed == nil {
		return nil, fmt.Errorf("simpleType must have a derivation")
	}

	return parsed, nil
}
