package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

var validAttributeAttributes = map[string]bool{
	"name":    true,
	"ref":     true,
	"type":    true,
	"use":     true,
	"default": true,
	"fixed":   true,
	"form":    true,
	"id":      true,
}

func parseAttribute(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.AttributeDecl, error) {
	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "attribute", schema); err != nil {
			return nil, err
		}
	}

	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}

	ref := doc.GetAttribute(elem, "ref")
	nameAttr := doc.GetAttribute(elem, "name")
	if ref != "" && nameAttr != "" {
		return nil, fmt.Errorf("attribute cannot have both 'name' and 'ref' attributes")
	}

	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() == "xmlns" || (attr.NamespaceURI() == "" && attr.LocalName() == "xmlns") {
			continue
		}
		if attr.NamespaceURI() == "" && !validAttributeAttributes[attr.LocalName()] {
			return nil, fmt.Errorf("invalid attribute '%s' on <attribute> element", attr.LocalName())
		}
	}

	if doc.HasAttribute(elem, "default") && doc.HasAttribute(elem, "fixed") {
		return nil, fmt.Errorf("attribute cannot have both 'default' and 'fixed' attributes")
	}

	// check if it's a reference
	if ref != "" {
		if doc.HasAttribute(elem, "type") {
			return nil, fmt.Errorf("attribute reference cannot have 'type' attribute")
		}
		if doc.HasAttribute(elem, "form") {
			return nil, fmt.Errorf("attribute reference cannot have 'form' attribute")
		}
		if err := validateOnlyAnnotationChildren(doc, elem, "attribute"); err != nil {
			return nil, err
		}
		// for attribute references, use resolveAttributeRefQName
		// per XSD spec, unprefixed attribute refs refer to no namespace
		refQName, err := resolveAttributeRefQName(doc, ref, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve ref %s: %w", ref, err)
		}

		attr := &types.AttributeDecl{
			Name: refQName,
			// and will override the referenced attribute's values
			Use:         types.Optional,
			IsReference: true,
		}

		// parse use attribute (can override referenced attribute's use)
		use, err := parseAttributeUse(doc, elem)
		if err != nil {
			return nil, err
		}
		attr.Use = use

		if attr.Use == types.Prohibited && doc.HasAttribute(elem, "default") {
			return nil, fmt.Errorf("attribute with use='prohibited' cannot have default value")
		}
		if attr.Use == types.Required && doc.HasAttribute(elem, "default") {
			return nil, fmt.Errorf("attribute with use='required' cannot have default value")
		}

		if defaultVal := doc.GetAttribute(elem, "default"); defaultVal != "" {
			attr.Default = defaultVal
		}

		if doc.HasAttribute(elem, "fixed") {
			attr.Fixed = doc.GetAttribute(elem, "fixed")
			attr.HasFixed = true
			attr.FixedContext = namespaceContextForElement(doc, elem, schema)
		}

		parsed, err := types.NewAttributeDeclFromParsed(attr)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	}

	// local attribute declaration
	name := getNameAttr(doc, elem)
	if name == "" {
		return nil, fmt.Errorf("attribute missing name and ref")
	}
	if name == "xmlns" {
		return nil, fmt.Errorf("attribute name cannot be 'xmlns'")
	}
	if !types.IsValidNCName(name) {
		return nil, fmt.Errorf("attribute name '%s' must be a valid NCName", name)
	}

	attr := &types.AttributeDecl{
		Name: types.QName{
			// attributes without a namespace prefix are in the empty namespace
			// (unlike elements which are in the target namespace)
			Namespace: "",
			Local:     name,
		},
		Use:             types.Optional,
		SourceNamespace: schema.TargetNamespace,
	}

	// attribute can have either 'type' attribute OR inline simpleType, but not both
	typeName := doc.GetAttribute(elem, "type")
	simpleTypeCount := 0
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) == xsdxml.XSDNamespace && doc.LocalName(child) == "simpleType" {
			simpleTypeCount++
		} else if doc.NamespaceURI(child) == xsdxml.XSDNamespace {
			switch doc.LocalName(child) {
			case "key", "keyref", "unique":
				return nil, fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
			case "annotation":
				// allowed; ordering handled by validateAnnotationOrder.
			default:
				return nil, fmt.Errorf("invalid child element <%s> in <attribute> declaration", doc.LocalName(child))
			}
		}
	}

	if typeName != "" && simpleTypeCount > 0 {
		return nil, fmt.Errorf("attribute cannot have both 'type' attribute and inline simpleType")
	}
	if simpleTypeCount > 1 {
		return nil, fmt.Errorf("attribute cannot have multiple simpleType children")
	}

	if typeName != "" {
		typeQName, err := resolveQName(doc, typeName, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		if builtinType := types.GetBuiltinNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			attr.Type = builtinType
		} else {
			attr.Type = types.NewPlaceholderSimpleType(typeQName)
		}
	} else {
		for _, child := range doc.Children(elem) {
			if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
				continue
			}

			if doc.LocalName(child) == "simpleType" {
				st, err := parseInlineSimpleType(doc, child, schema)
				if err != nil {
					return nil, fmt.Errorf("parse inline simpleType: %w", err)
				}
				attr.Type = st
			}
		}
	}

	use, err := parseAttributeUse(doc, elem)
	if err != nil {
		return nil, err
	}
	attr.Use = use

	if attr.Use == types.Prohibited && doc.HasAttribute(elem, "default") {
		return nil, fmt.Errorf("attribute with use='prohibited' cannot have default value")
	}
	if attr.Use == types.Required && doc.HasAttribute(elem, "default") {
		return nil, fmt.Errorf("attribute with use='required' cannot have default value")
	}

	if defaultVal := doc.GetAttribute(elem, "default"); defaultVal != "" {
		attr.Default = defaultVal
	}

	if doc.HasAttribute(elem, "fixed") {
		attr.Fixed = doc.GetAttribute(elem, "fixed")
		attr.HasFixed = true
		attr.FixedContext = namespaceContextForElement(doc, elem, schema)
	}

	// parse form attribute - must be exactly "qualified" or "unqualified"
	if doc.HasAttribute(elem, "form") {
		formAttr := doc.GetAttribute(elem, "form")
		switch formAttr {
		case "qualified":
			attr.Form = types.FormQualified
		case "unqualified":
			attr.Form = types.FormUnqualified
		default:
			return nil, fmt.Errorf("invalid form attribute value '%s': must be 'qualified' or 'unqualified'", formAttr)
		}
	}

	parsed, err := types.NewAttributeDeclFromParsed(attr)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func parseAttributeUse(doc *xsdxml.Document, elem xsdxml.NodeID) (types.AttributeUse, error) {
	if doc.HasAttribute(elem, "use") {
		useAttr := doc.GetAttribute(elem, "use")
		switch useAttr {
		case "optional":
			return types.Optional, nil
		case "required":
			return types.Required, nil
		case "prohibited":
			return types.Prohibited, nil
		default:
			return types.Optional, fmt.Errorf("invalid use attribute value '%s': must be 'optional', 'prohibited', or 'required'", useAttr)
		}
	}

	return types.Optional, nil
}

// parseTopLevelAttribute parses a top-level attribute declaration
func parseTopLevelAttribute(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	name := getNameAttr(doc, elem)
	if name == "" {
		return fmt.Errorf("attribute missing name attribute")
	}

	// 'form' attribute only applies to local attribute declarations
	if doc.HasAttribute(elem, "form") {
		return fmt.Errorf("top-level attribute cannot have 'form' attribute")
	}
	if doc.HasAttribute(elem, "use") {
		return fmt.Errorf("top-level attribute cannot have 'use' attribute")
	}
	if doc.HasAttribute(elem, "ref") {
		return fmt.Errorf("top-level attribute cannot have 'ref' attribute")
	}

	attr, err := parseAttribute(doc, elem, schema)
	if err != nil {
		return err
	}

	// top-level attributes are always in the target namespace
	attrQName := types.QName{
		Local:     name,
		Namespace: schema.TargetNamespace,
	}

	// update the attribute's name to reflect the correct namespace
	attr.Name = attrQName

	attr.SourceNamespace = schema.TargetNamespace

	// store in schema's global attribute declarations
	if _, exists := schema.AttributeDecls[attrQName]; exists {
		return fmt.Errorf("attribute %s already defined", attrQName)
	}
	schema.AttributeDecls[attrQName] = attr

	return nil
}
