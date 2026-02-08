package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseLocalAttribute(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, local bool) (*types.AttributeDecl, error) {
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
			Namespace: "",
			Local:     name,
		},
		Use:             types.Optional,
		SourceNamespace: schema.TargetNamespace,
	}

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
	if attr.Type == nil {
		attr.Type = types.GetBuiltin(types.TypeNameAnySimpleType)
	}

	use, err := parseAttributeUse(doc, elem)
	if err != nil {
		return nil, err
	}
	attr.Use = use

	if attr.Use == types.Required && doc.HasAttribute(elem, "default") {
		return nil, fmt.Errorf("attribute with use='required' cannot have default value")
	}
	if attr.Use == types.Prohibited && doc.HasAttribute(elem, "default") {
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

	formExplicit := doc.HasAttribute(elem, "form")
	if formExplicit {
		formAttr := types.ApplyWhiteSpace(doc.GetAttribute(elem, "form"), types.WhiteSpaceCollapse)
		switch formAttr {
		case "qualified":
			attr.Form = types.FormQualified
		case "unqualified":
			attr.Form = types.FormUnqualified
		default:
			return nil, fmt.Errorf("invalid form attribute value '%s': must be 'qualified' or 'unqualified'", formAttr)
		}
	} else if local {
		if schema.AttributeFormDefault == Qualified {
			attr.Form = types.FormQualified
		} else {
			attr.Form = types.FormUnqualified
		}
	}

	parsed, err := types.NewAttributeDeclFromParsed(attr)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}
