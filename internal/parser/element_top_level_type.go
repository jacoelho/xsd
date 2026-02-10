package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func hasInlineTypeChild(doc *xsdxml.Document, elem xsdxml.NodeID) bool {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "complexType", "simpleType":
			return true
		}
	}
	return false
}

func resolveTopLevelElementType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (model.Type, bool, error) {
	if typeName := doc.GetAttribute(elem, "type"); typeName != "" {
		if hasInlineTypeChild(doc, elem) {
			return nil, false, fmt.Errorf("element cannot have both 'type' attribute and inline type definition")
		}
		typeQName, err := resolveQNameWithPolicy(doc, typeName, elem, schema, useDefaultNamespace)
		if err != nil {
			return nil, false, fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		if builtinType := builtins.GetNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			return builtinType, true, nil
		}
		return model.NewPlaceholderSimpleType(typeQName), true, nil
	}

	var resolved model.Type
	var explicit bool
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "complexType":
			if doc.GetAttribute(child, "name") != "" {
				return nil, false, fmt.Errorf("inline complexType cannot have 'name' attribute")
			}
			ct, err := parseInlineComplexType(doc, child, schema)
			if err != nil {
				return nil, false, fmt.Errorf("parse inline complexType: %w", err)
			}
			resolved = ct
			explicit = true
		case "simpleType":
			if doc.GetAttribute(child, "name") != "" {
				return nil, false, fmt.Errorf("inline simpleType cannot have 'name' attribute")
			}
			st, err := parseInlineSimpleType(doc, child, schema)
			if err != nil {
				return nil, false, fmt.Errorf("parse inline simpleType: %w", err)
			}
			resolved = st
			explicit = true
		}
	}
	if resolved == nil {
		return builtins.Get(builtins.TypeNameAnyType), false, nil
	}
	return resolved, explicit, nil
}
