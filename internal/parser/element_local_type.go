package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func elementHasInlineType(doc *xsdxml.Document, elem xsdxml.NodeID) bool {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) == xsdxml.XSDNamespace {
			name := doc.LocalName(child)
			if name == "complexType" || name == "simpleType" {
				return true
			}
		}
	}
	return false
}

func resolveElementType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, attrs *elementAttrScan) (types.Type, error) {
	if typeName := attrs.typ; typeName != "" {
		typeQName, err := resolveQNameWithPolicy(doc, typeName, elem, schema, useDefaultNamespace)
		if err != nil {
			return nil, fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		if builtinType := types.GetBuiltinNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			return builtinType, nil
		}
		return types.NewPlaceholderSimpleType(typeQName), nil
	}

	var typ types.Type
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "complexType":
			if doc.GetAttribute(child, "name") != "" {
				return nil, fmt.Errorf("inline complexType cannot have 'name' attribute")
			}
			ct, err := parseInlineComplexType(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse inline complexType: %w", err)
			}
			typ = ct
		case "simpleType":
			if doc.GetAttribute(child, "name") != "" {
				return nil, fmt.Errorf("inline simpleType cannot have 'name' attribute")
			}
			st, err := parseInlineSimpleType(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse inline simpleType: %w", err)
			}
			typ = st
		}
	}

	if typ == nil {
		typ = types.GetBuiltin(types.TypeNameAnyType)
	}

	return typ, nil
}
