package parser

import (
	"fmt"

	xsdschema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func parseAttribute(elem xml.Element, schema *xsdschema.Schema) (*types.AttributeDecl, error) {
	if hasIDAttribute(elem) {
		idAttr := elem.GetAttribute("id")
		if err := validateIDAttribute(idAttr, "attribute", schema); err != nil {
			return nil, err
		}
	}

	if err := validateAnnotationOrder(elem); err != nil {
		return nil, err
	}

	ref := elem.GetAttribute("ref")
	nameAttr := elem.GetAttribute("name")
	if ref != "" && nameAttr != "" {
		return nil, fmt.Errorf("attribute cannot have both 'name' and 'ref' attributes")
	}

	validAttributes := map[string]bool{
		"name":    true,
		"ref":     true,
		"type":    true,
		"use":     true,
		"default": true,
		"fixed":   true,
		"form":    true,
		"id":      true,
	}
	for _, attr := range elem.Attributes() {
		if attr.NamespaceURI() == "xmlns" || (attr.NamespaceURI() == "" && attr.LocalName() == "xmlns") {
			continue
		}
		if attr.NamespaceURI() == "" && !validAttributes[attr.LocalName()] {
			return nil, fmt.Errorf("invalid attribute '%s' on <attribute> element", attr.LocalName())
		}
	}

	if elem.HasAttribute("default") && elem.HasAttribute("fixed") {
		return nil, fmt.Errorf("attribute cannot have both 'default' and 'fixed' attributes")
	}

	// check if it's a reference
	if ref != "" {
		if elem.HasAttribute("type") {
			return nil, fmt.Errorf("attribute reference cannot have 'type' attribute")
		}
		if elem.HasAttribute("form") {
			return nil, fmt.Errorf("attribute reference cannot have 'form' attribute")
		}
		if err := validateOnlyAnnotationChildren(elem, "attribute"); err != nil {
			return nil, err
		}
		// for attribute references, use resolveAttributeRefQName
		// per XSD spec, unprefixed attribute refs refer to no namespace
		refQName, err := resolveAttributeRefQName(ref, elem, schema)
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
		use, err := parseAttributeUse(elem)
		if err != nil {
			return nil, err
		}
		attr.Use = use

		if attr.Use == types.Prohibited && elem.HasAttribute("default") {
			return nil, fmt.Errorf("attribute with use='prohibited' cannot have default value")
		}
		if attr.Use == types.Required && elem.HasAttribute("default") {
			return nil, fmt.Errorf("attribute with use='required' cannot have default value")
		}

		if defaultVal := elem.GetAttribute("default"); defaultVal != "" {
			attr.Default = defaultVal
		}

		if elem.HasAttribute("fixed") {
			attr.Fixed = elem.GetAttribute("fixed")
			attr.HasFixed = true
		}

		parsed, err := types.NewAttributeDeclFromParsed(attr)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	}

	// local attribute declaration
	name := getAttr(elem, "name")
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
	typeName := elem.GetAttribute("type")
	simpleTypeCount := 0
	for _, child := range elem.Children() {
		if child.NamespaceURI() == xml.XSDNamespace && child.LocalName() == "simpleType" {
			simpleTypeCount++
		} else if child.NamespaceURI() == xml.XSDNamespace {
			switch child.LocalName() {
			case "key", "keyref", "unique":
				return nil, fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", child.LocalName())
			case "annotation":
				// allowed; ordering handled by validateAnnotationOrder.
			default:
				return nil, fmt.Errorf("invalid child element <%s> in <attribute> declaration", child.LocalName())
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
		typeQName, err := resolveQName(typeName, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		if builtinType := types.GetBuiltinNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			attr.Type = builtinType
		} else {
			attr.Type = &types.SimpleType{
				QName: typeQName,
			}
		}
	} else {
		for _, child := range elem.Children() {
			if child.NamespaceURI() != xml.XSDNamespace {
				continue
			}

			switch child.LocalName() {
			case "simpleType":
				st, err := parseInlineSimpleType(child, schema)
				if err != nil {
					return nil, fmt.Errorf("parse inline simpleType: %w", err)
				}
				attr.Type = st
			}
		}
	}

	use, err := parseAttributeUse(elem)
	if err != nil {
		return nil, err
	}
	attr.Use = use

	if attr.Use == types.Prohibited && elem.HasAttribute("default") {
		return nil, fmt.Errorf("attribute with use='prohibited' cannot have default value")
	}
	if attr.Use == types.Required && elem.HasAttribute("default") {
		return nil, fmt.Errorf("attribute with use='required' cannot have default value")
	}

	if defaultVal := elem.GetAttribute("default"); defaultVal != "" {
		attr.Default = defaultVal
	}

	if elem.HasAttribute("fixed") {
		attr.Fixed = elem.GetAttribute("fixed")
		attr.HasFixed = true
	}

	// parse form attribute - must be exactly "qualified" or "unqualified"
	if elem.HasAttribute("form") {
		formAttr := elem.GetAttribute("form")
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

func parseAttributeUse(elem xml.Element) (types.AttributeUse, error) {
	if elem.HasAttribute("use") {
		useAttr := elem.GetAttribute("use")
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
func parseTopLevelAttribute(elem xml.Element, schema *xsdschema.Schema) error {
	name := getAttr(elem, "name")
	if name == "" {
		return fmt.Errorf("attribute missing name attribute")
	}

	// 'form' attribute only applies to local attribute declarations
	if elem.HasAttribute("form") {
		return fmt.Errorf("top-level attribute cannot have 'form' attribute")
	}
	if elem.HasAttribute("use") {
		return fmt.Errorf("top-level attribute cannot have 'use' attribute")
	}
	if elem.HasAttribute("ref") {
		return fmt.Errorf("top-level attribute cannot have 'ref' attribute")
	}

	attr, err := parseAttribute(elem, schema)
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