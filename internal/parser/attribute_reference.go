package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseAttributeReference(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, ref string) (*model.AttributeDecl, error) {
	if doc.HasAttribute(elem, "type") {
		return nil, fmt.Errorf("attribute reference cannot have 'type' attribute")
	}
	if doc.HasAttribute(elem, "form") {
		return nil, fmt.Errorf("attribute reference cannot have 'form' attribute")
	}
	if err := validateOnlyAnnotationChildren(doc, elem, "attribute"); err != nil {
		return nil, err
	}

	refQName, err := resolveQNameWithPolicy(doc, ref, elem, schema, forceEmptyNamespace)
	if err != nil {
		return nil, fmt.Errorf("resolve ref %s: %w", ref, err)
	}

	attr := &model.AttributeDecl{
		Name:        refQName,
		Use:         model.Optional,
		IsReference: true,
	}
	use, err := parseAttributeUse(doc, elem)
	if err != nil {
		return nil, err
	}
	attr.Use = use
	if attr.Use == model.Required && doc.HasAttribute(elem, "default") {
		return nil, fmt.Errorf("attribute with use='required' cannot have default value")
	}
	if attr.Use == model.Prohibited && doc.HasAttribute(elem, "default") {
		return nil, fmt.Errorf("attribute with use='prohibited' cannot have default value")
	}
	if doc.HasAttribute(elem, "default") {
		attr.Default = doc.GetAttribute(elem, "default")
		attr.HasDefault = true
		attr.DefaultContext = namespaceContextForElement(doc, elem, schema)
	}
	if doc.HasAttribute(elem, "fixed") {
		attr.Fixed = doc.GetAttribute(elem, "fixed")
		attr.HasFixed = true
		attr.FixedContext = namespaceContextForElement(doc, elem, schema)
	}
	if attr.HasDefault || attr.HasFixed {
		attr.ValueContext = namespaceContextForElement(doc, elem, schema)
	}
	parsed, err := model.NewAttributeDeclFromParsed(attr)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}
