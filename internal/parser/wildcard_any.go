package parser

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

// parseAnyElement parses an <any> wildcard element
// Content model: (annotation?)
func parseAnyElement(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*model.AnyElement, error) {
	nsConstraint, nsList, processContents, parseErr := parseWildcardConstraints(
		doc,
		elem,
		"any",
		"namespace, processContents, minOccurs, maxOccurs",
		validAttributeNames[attrSetAnyElement],
	)
	if parseErr != nil {
		return nil, parseErr
	}

	if err := validateOptionalID(doc, elem, "any", schema); err != nil {
		return nil, err
	}

	hasAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return nil, fmt.Errorf("any: at most one annotation is allowed")
			}
			hasAnnotation = true
		default:
			return nil, fmt.Errorf("any: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	minOccursAttr := doc.GetAttribute(elem, "minOccurs")
	maxOccursAttr := doc.GetAttribute(elem, "maxOccurs")
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "minOccurs" && attr.NamespaceURI() == "" && minOccursAttr == "" {
			return nil, fmt.Errorf("minOccurs attribute cannot be empty")
		}
		if attr.LocalName() == "maxOccurs" && attr.NamespaceURI() == "" && maxOccursAttr == "" {
			return nil, fmt.Errorf("maxOccurs attribute cannot be empty")
		}
	}
	if err := validateOccursValue(minOccursAttr); err != nil {
		return nil, fmt.Errorf("invalid minOccurs value '%s': %w", minOccursAttr, err)
	}
	if maxOccursAttr != "" && maxOccursAttr != "unbounded" {
		if err := validateOccursInteger(maxOccursAttr); err != nil {
			return nil, fmt.Errorf("invalid maxOccurs value '%s': %w", maxOccursAttr, err)
		}
	}

	minOccurs, err := parseOccursAttr(doc, elem, "minOccurs")
	if err != nil {
		return nil, err
	}
	maxOccurs, err := parseOccursAttr(doc, elem, "maxOccurs")
	if err != nil {
		return nil, err
	}
	anyElem := &model.AnyElement{
		MinOccurs:       minOccurs,
		MaxOccurs:       maxOccurs,
		ProcessContents: processContents,
		TargetNamespace: schema.TargetNamespace,
	}
	anyElem.Namespace = nsConstraint
	anyElem.NamespaceList = nsList

	return anyElem, nil
}

func validateOccursValue(value string) error {
	if value == "" {
		return nil
	}
	if value == "unbounded" {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	return validateOccursInteger(value)
}

func validateOccursInteger(value string) error {
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	if value == "" {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	intVal, perr := num.ParseInt([]byte(value))
	if perr != nil || intVal.Sign < 0 {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	return nil
}

// parseNamespaceConstraint parses a namespace constraint value
func parseNamespaceConstraint(value string) (model.NamespaceConstraint, []model.NamespaceURI, error) {
	switch value {
	case "##any":
		return model.NSCAny, nil, nil
	case "##other":
		return model.NSCOther, nil, nil
	case "##targetNamespace":
		return model.NSCTargetNamespace, nil, nil
	case "##local":
		return model.NSCLocal, nil, nil
	}

	var resultList []model.NamespaceURI
	seen := false
	for ns := range model.FieldsXMLWhitespaceSeq(value) {
		seen = true
		if strings.HasPrefix(ns, "##") && !validNamespaceConstraintTokens[ns] {
			if ns == "##any" || ns == "##other" {
				return 0, nil, fmt.Errorf("invalid namespace constraint: %s cannot appear in a namespace list (must be used alone)", ns)
			}
			return 0, nil, fmt.Errorf("invalid namespace constraint: unknown special token %s (must be one of: ##any, ##other, ##targetNamespace, ##local)", ns)
		}

		switch ns {
		case "##targetNamespace":
			resultList = append(resultList, model.NamespaceTargetPlaceholder)
		case "##local":
			resultList = append(resultList, model.NamespaceEmpty)
		default:
			resultList = append(resultList, model.NamespaceURI(ns))
		}
	}
	if !seen {
		return 0, nil, fmt.Errorf("invalid namespace constraint: empty namespace list")
	}

	return model.NSCList, resultList, nil
}
