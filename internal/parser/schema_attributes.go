package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseSchemaAttributes(doc *xsdxml.Document, root xsdxml.NodeID, schema *Schema) error {
	if err := validateSchemaAttributeNamespaces(doc, root); err != nil {
		return err
	}
	targetNSAttr := ""
	targetNSFound := false
	for _, attr := range doc.Attributes(root) {
		if attr.LocalName() == "targetNamespace" {
			switch attr.NamespaceURI() {
			case "":
				targetNSAttr = model.ApplyWhiteSpace(attr.Value(), model.WhiteSpaceCollapse)
				targetNSFound = true
			case xsdxml.XSDNamespace:
				return fmt.Errorf("schema attribute 'targetNamespace' must be unprefixed (found '%s:targetNamespace')", attr.NamespaceURI())
			default:
				continue
			}
		}
	}
	if !targetNSFound {
		schema.TargetNamespace = model.NamespaceEmpty
	} else {
		if targetNSAttr == "" {
			return fmt.Errorf("targetNamespace attribute cannot be empty (must be absent or have a non-empty value)")
		}
		schema.TargetNamespace = model.NamespaceURI(targetNSAttr)
	}

	for _, attr := range doc.Attributes(root) {
		if !isXMLNSDeclaration(attr) {
			continue
		}
		if attr.LocalName() == "xmlns" {
			schema.NamespaceDecls[""] = attr.Value()
			continue
		}
		prefix := attr.LocalName()
		if attr.Value() == "" {
			return fmt.Errorf("namespace prefix %q cannot be bound to empty namespace", prefix)
		}
		schema.NamespaceDecls[prefix] = attr.Value()
	}

	if doc.HasAttribute(root, "elementFormDefault") {
		elemForm := model.ApplyWhiteSpace(doc.GetAttribute(root, "elementFormDefault"), model.WhiteSpaceCollapse)
		if elemForm == "" {
			return fmt.Errorf("elementFormDefault attribute cannot be empty")
		}
		switch elemForm {
		case "qualified":
			schema.ElementFormDefault = Qualified
		case "unqualified":
			schema.ElementFormDefault = Unqualified
		default:
			return fmt.Errorf("invalid elementFormDefault attribute value '%s': must be 'qualified' or 'unqualified'", elemForm)
		}
	}

	if doc.HasAttribute(root, "attributeFormDefault") {
		attrForm := model.ApplyWhiteSpace(doc.GetAttribute(root, "attributeFormDefault"), model.WhiteSpaceCollapse)
		if attrForm == "" {
			return fmt.Errorf("attributeFormDefault attribute cannot be empty")
		}
		switch attrForm {
		case "qualified":
			schema.AttributeFormDefault = Qualified
		case "unqualified":
			schema.AttributeFormDefault = Unqualified
		default:
			return fmt.Errorf("invalid attributeFormDefault attribute value '%s': must be 'qualified' or 'unqualified'", attrForm)
		}
	}

	if doc.HasAttribute(root, "blockDefault") {
		blockDefaultAttr := doc.GetAttribute(root, "blockDefault")
		if model.TrimXMLWhitespace(blockDefaultAttr) == "" {
			return fmt.Errorf("blockDefault attribute cannot be empty")
		}
		block, err := parseDerivationSetWithValidation(blockDefaultAttr, model.DerivationSet(model.DerivationSubstitution|model.DerivationExtension|model.DerivationRestriction))
		if err != nil {
			return fmt.Errorf("invalid blockDefault attribute value '%s': %w", blockDefaultAttr, err)
		}
		schema.BlockDefault = block
	}

	if doc.HasAttribute(root, "finalDefault") {
		finalDefaultAttr := doc.GetAttribute(root, "finalDefault")
		if model.TrimXMLWhitespace(finalDefaultAttr) == "" {
			return fmt.Errorf("finalDefault attribute cannot be empty")
		}
		final, err := parseDerivationSetWithValidation(finalDefaultAttr, model.DerivationSet(model.DerivationExtension|model.DerivationRestriction|model.DerivationList|model.DerivationUnion))
		if err != nil {
			return fmt.Errorf("invalid finalDefault attribute value '%s': %w", finalDefaultAttr, err)
		}
		schema.FinalDefault = final
	}

	return nil
}
