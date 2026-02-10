package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

// parseInlineComplexType parses a complexType definition (inline or named).
func parseInlineComplexType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*model.ComplexType, error) {
	ct := &model.ComplexType{}

	if doc.GetAttribute(elem, "name") == "" {
		if err := validateOptionalID(doc, elem, "complexType", schema); err != nil {
			return nil, err
		}
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "mixed"); err != nil {
		return nil, err
	} else if ok {
		ct.SetMixed(value)
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "abstract"); err != nil {
		return nil, err
	} else if ok {
		ct.Abstract = value
	}

	if doc.HasAttribute(elem, "block") {
		blockAttr := doc.GetAttribute(elem, "block")
		if model.TrimXMLWhitespace(blockAttr) == "" {
			return nil, fmt.Errorf("block attribute cannot be empty")
		}
		block, err := parseDerivationSetWithValidation(blockAttr, model.DerivationSet(model.DerivationExtension|model.DerivationRestriction))
		if err != nil {
			return nil, fmt.Errorf("invalid block attribute value '%s': %w", blockAttr, err)
		}
		ct.Block = block
	} else {
		ct.Block = schema.BlockDefault & model.DerivationSet(model.DerivationExtension|model.DerivationRestriction)
	}

	if doc.HasAttribute(elem, "final") {
		finalAttr := doc.GetAttribute(elem, "final")
		if model.TrimXMLWhitespace(finalAttr) == "" {
			return nil, fmt.Errorf("final attribute cannot be empty")
		}
		final, err := parseDerivationSetWithValidation(finalAttr, model.DerivationSet(model.DerivationExtension|model.DerivationRestriction))
		if err != nil {
			return nil, fmt.Errorf("invalid final attribute value '%s': %w", finalAttr, err)
		}
		ct.Final = final
	} else if schema.FinalDefault != 0 {
		ct.Final = schema.FinalDefault & model.DerivationSet(model.DerivationExtension|model.DerivationRestriction)
	}

	state := complexTypeParseState{doc: doc, schema: schema, ct: ct}
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		if err := state.handleChild(child); err != nil {
			return nil, err
		}
	}

	if ct.Content() == nil {
		ct.SetContent(&model.EmptyContent{})
	}

	parsed, err := model.NewComplexTypeFromParsed(ct)
	if err != nil {
		return nil, fmt.Errorf("complexType: %w", err)
	}
	return parsed, nil
}
