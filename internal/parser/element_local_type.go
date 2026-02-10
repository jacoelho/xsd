package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func elementHasInlineType(doc *schemaxml.Document, elem schemaxml.NodeID) bool {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) == schemaxml.XSDNamespace {
			name := doc.LocalName(child)
			if name == "complexType" || name == "simpleType" {
				return true
			}
		}
	}
	return false
}

func resolveElementType(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, attrs *elementAttrScan) (model.Type, error) {
	if typeName := attrs.typ; typeName != "" {
		typeQName, err := resolveQNameWithPolicy(doc, typeName, elem, schema, useDefaultNamespace)
		if err != nil {
			return nil, fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		if builtinType := builtins.GetNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			return builtinType, nil
		}
		return model.NewPlaceholderSimpleType(typeQName), nil
	}

	var typ model.Type
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != schemaxml.XSDNamespace {
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
		typ = builtins.Get(builtins.TypeNameAnyType)
	}

	return typ, nil
}
