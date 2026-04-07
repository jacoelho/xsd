package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/value"
)

func parseLocalAttribute(doc *Document, elem NodeID, schema *Schema, local bool) (*model.AttributeDecl, error) {
	name := model.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
	if name == "" {
		return nil, fmt.Errorf("attribute missing name and ref")
	}
	if name == "xmlns" {
		return nil, fmt.Errorf("attribute name cannot be 'xmlns'")
	}
	if !model.IsValidNCName(name) {
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
	simpleTypeCount, err := countInlineSimpleTypes(doc, elem)
	if err != nil {
		return nil, err
	}
	if typeName != "" && simpleTypeCount > 0 {
		return nil, fmt.Errorf("attribute cannot have both 'type' attribute and inline simpleType")
	}
	if simpleTypeCount > 1 {
		return nil, fmt.Errorf("attribute cannot have multiple simpleType children")
	}
	attr.Type, err = resolveLocalAttributeType(doc, elem, schema, typeName)
	if err != nil {
		return nil, err
	}
	if attr.Type == nil {
		attr.Type = model.GetBuiltin(model.TypeNameAnySimpleType)
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

	formErr := applyAttributeForm(doc, elem, schema, local, attr)
	if formErr != nil {
		return nil, formErr
	}

	parsed, err := model.NewAttributeDeclFromParsed(attr)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func countInlineSimpleTypes(doc *Document, elem NodeID) (int, error) {
	count := 0
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != value.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "simpleType":
			count++
		case "annotation":
		case "key", "keyref", "unique":
			return 0, fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
		default:
			return 0, fmt.Errorf("invalid child element <%s> in <attribute> declaration", doc.LocalName(child))
		}
	}
	return count, nil
}

func resolveLocalAttributeType(doc *Document, elem NodeID, schema *Schema, typeName string) (model.Type, error) {
	if typeName != "" {
		return resolveLocalAttributeTypeName(doc, elem, schema, typeName)
	}
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != value.XSDNamespace || doc.LocalName(child) != "simpleType" {
			continue
		}
		st, err := parseInlineSimpleType(doc, child, schema)
		if err != nil {
			return nil, fmt.Errorf("parse inline simpleType: %w", err)
		}
		return st, nil
	}
	return nil, nil
}

func resolveLocalAttributeTypeName(doc *Document, elem NodeID, schema *Schema, typeName string) (model.Type, error) {
	typeQName, err := resolveQNameWithPolicy(doc, typeName, elem, schema, useDefaultNamespace)
	if err != nil {
		return nil, fmt.Errorf("resolve type %s: %w", typeName, err)
	}
	if builtinType := model.GetBuiltinNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
		return builtinType, nil
	}
	return model.NewPlaceholderSimpleType(typeQName), nil
}

func applyAttributeForm(doc *Document, elem NodeID, schema *Schema, local bool, attr *model.AttributeDecl) error {
	if doc.HasAttribute(elem, "form") {
		formAttr := model.ApplyWhiteSpace(doc.GetAttribute(elem, "form"), model.WhiteSpaceCollapse)
		switch formAttr {
		case "qualified":
			attr.Form = model.FormQualified
		case "unqualified":
			attr.Form = model.FormUnqualified
		default:
			return fmt.Errorf("invalid form attribute value '%s': must be 'qualified' or 'unqualified'", formAttr)
		}
		return nil
	}
	if !local {
		return nil
	}
	if schema.AttributeFormDefault == Qualified {
		attr.Form = model.FormQualified
		return nil
	}
	attr.Form = model.FormUnqualified
	return nil
}
