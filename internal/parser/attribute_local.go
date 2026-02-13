package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func parseLocalAttribute(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema, local bool) (*model.AttributeDecl, error) {
	name := model.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
	if name == "" {
		return nil, fmt.Errorf("attribute missing name and ref")
	}
	if name == "xmlns" {
		return nil, fmt.Errorf("attribute name cannot be 'xmlns'")
	}
	if !qname.IsValidNCName(name) {
		return nil, fmt.Errorf("attribute name '%s' must be a valid NCName", name)
	}

	attr := &model.AttributeDecl{
		Name: model.QName{
			Namespace: "",
			Local:     name,
		},
		Use:             model.Optional,
		SourceNamespace: schema.TargetNamespace,
	}

	typeName := doc.GetAttribute(elem, "type")
	simpleTypeCount := 0
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) == xmltree.XSDNamespace && doc.LocalName(child) == "simpleType" {
			simpleTypeCount++
		} else if doc.NamespaceURI(child) == xmltree.XSDNamespace {
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
		typeQName, err := resolveQNameWithPolicy(doc, typeName, elem, schema, useDefaultNamespace)
		if err != nil {
			return nil, fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		if builtinType := builtins.GetNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			attr.Type = builtinType
		} else {
			attr.Type = model.NewPlaceholderSimpleType(typeQName)
		}
	} else {
		for _, child := range doc.Children(elem) {
			if doc.NamespaceURI(child) != xmltree.XSDNamespace {
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
		attr.Type = builtins.Get(builtins.TypeNameAnySimpleType)
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

	formExplicit := doc.HasAttribute(elem, "form")
	if formExplicit {
		formAttr := model.ApplyWhiteSpace(doc.GetAttribute(elem, "form"), model.WhiteSpaceCollapse)
		switch formAttr {
		case "qualified":
			attr.Form = model.FormQualified
		case "unqualified":
			attr.Form = model.FormUnqualified
		default:
			return nil, fmt.Errorf("invalid form attribute value '%s': must be 'qualified' or 'unqualified'", formAttr)
		}
	} else if local {
		if schema.AttributeFormDefault == Qualified {
			attr.Form = model.FormQualified
		} else {
			attr.Form = model.FormUnqualified
		}
	}

	parsed, err := model.NewAttributeDeclFromParsed(attr)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}
