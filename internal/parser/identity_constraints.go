package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

// parseIdentityConstraint parses a key, keyref, or unique constraint
func parseIdentityConstraint(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*model.IdentityConstraint, error) {
	name := model.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
	if name == "" {
		return nil, fmt.Errorf("identity constraint missing name attribute")
	}

	if err := validateOptionalID(doc, elem, doc.LocalName(elem), schema); err != nil {
		return nil, err
	}

	nsContext := namespaceContextForElement(doc, elem, schema)

	constraint := &model.IdentityConstraint{
		Name:             name,
		TargetNamespace:  schema.TargetNamespace,
		Fields:           []model.Field{},
		NamespaceContext: nsContext,
	}

	switch doc.LocalName(elem) {
	case "key":
		constraint.Type = model.KeyConstraint
	case "keyref":
		constraint.Type = model.KeyRefConstraint
	case "unique":
		constraint.Type = model.UniqueConstraint
	default:
		return nil, fmt.Errorf("unknown identity constraint type: %s", doc.LocalName(elem))
	}

	refer := doc.GetAttribute(elem, "refer")
	if refer != "" {
		referQName, err := resolveQNameWithPolicy(doc, refer, elem, schema, useDefaultNamespace)
		if err != nil {
			return nil, fmt.Errorf("resolve refer QName %s: %w", refer, err)
		}
		constraint.ReferQName = referQName
	} else if constraint.Type == model.KeyRefConstraint {
		return nil, fmt.Errorf("keyref missing refer attribute")
	}

	annotationCount := 0
	seenSelector := false
	seenField := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if seenSelector || seenField {
				return nil, fmt.Errorf("identity constraint '%s': annotation must appear before selector and field", name)
			}
			annotationCount++
			if annotationCount > 1 {
				return nil, fmt.Errorf("identity constraint '%s': at most one annotation allowed", name)
			}

		case "selector":
			if seenSelector {
				return nil, fmt.Errorf("identity constraint '%s': only one selector allowed", name)
			}
			xpath := doc.GetAttribute(child, "xpath")
			if xpath == "" {
				return nil, fmt.Errorf("selector missing xpath attribute")
			}
			if err := validateAllowedAttributes(doc, child, "selector", validAttributeNames[attrSetIdentityConstraint]); err != nil {
				return nil, err
			}
			if err := validateElementConstraints(doc, child, "selector", schema); err != nil {
				return nil, err
			}
			constraint.Selector = model.Selector{XPath: xpath}
			seenSelector = true

		case "field":
			if !seenSelector {
				return nil, fmt.Errorf("identity constraint '%s': selector must appear before field", name)
			}
			xpath := doc.GetAttribute(child, "xpath")
			if xpath == "" {
				return nil, fmt.Errorf("field missing xpath attribute")
			}
			if err := validateAllowedAttributes(doc, child, "field", validAttributeNames[attrSetIdentityConstraint]); err != nil {
				return nil, err
			}
			if err := validateElementConstraints(doc, child, "field", schema); err != nil {
				return nil, err
			}
			constraint.Fields = append(constraint.Fields, model.Field{XPath: xpath})
			seenField = true
		}
	}

	if constraint.Selector.XPath == "" {
		return nil, fmt.Errorf("identity constraint missing selector")
	}

	if len(constraint.Fields) == 0 {
		return nil, fmt.Errorf("identity constraint missing fields")
	}

	return constraint, nil
}

func validateAllowedAttributes(doc *xsdxml.Document, elem xsdxml.NodeID, elementName string, allowed map[string]bool) error {
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() != "" {
			if attr.NamespaceURI() == xsdxml.XSDNamespace {
				return fmt.Errorf("%s: attribute '%s' must be unprefixed", elementName, attr.LocalName())
			}
			continue
		}
		if !allowed[attr.LocalName()] {
			return fmt.Errorf("%s: unexpected attribute '%s'", elementName, attr.LocalName())
		}
	}
	return nil
}
