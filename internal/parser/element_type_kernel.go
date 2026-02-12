package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

type resolvedElementDeclType struct {
	typ          model.Type
	hasInline    bool
	hasTypeValue bool
}

func resolveElementDeclType(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema, typeValue string) (resolvedElementDeclType, error) {
	if typeValue != "" {
		if hasInlineElementTypeChild(doc, elem) {
			return resolvedElementDeclType{}, fmt.Errorf("element cannot have both 'type' attribute and inline type definition")
		}
		typeQName, err := resolveQNameWithPolicy(doc, typeValue, elem, schema, useDefaultNamespace)
		if err != nil {
			return resolvedElementDeclType{}, fmt.Errorf("resolve type %s: %w", typeValue, err)
		}
		if builtinType := builtins.GetNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			return resolvedElementDeclType{typ: builtinType, hasTypeValue: true}, nil
		}
		return resolvedElementDeclType{typ: model.NewPlaceholderSimpleType(typeQName), hasTypeValue: true}, nil
	}

	var typ model.Type
	hasInline := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xmltree.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "complexType":
			if doc.GetAttribute(child, "name") != "" {
				return resolvedElementDeclType{}, fmt.Errorf("inline complexType cannot have 'name' attribute")
			}
			ct, err := parseInlineComplexType(doc, child, schema)
			if err != nil {
				return resolvedElementDeclType{}, fmt.Errorf("parse inline complexType: %w", err)
			}
			typ = ct
			hasInline = true
		case "simpleType":
			if doc.GetAttribute(child, "name") != "" {
				return resolvedElementDeclType{}, fmt.Errorf("inline simpleType cannot have 'name' attribute")
			}
			st, err := parseInlineSimpleType(doc, child, schema)
			if err != nil {
				return resolvedElementDeclType{}, fmt.Errorf("parse inline simpleType: %w", err)
			}
			typ = st
			hasInline = true
		}
	}
	if typ == nil {
		typ = builtins.Get(builtins.TypeNameAnyType)
	}
	return resolvedElementDeclType{typ: typ, hasInline: hasInline}, nil
}

func hasInlineElementTypeChild(doc *xmltree.Document, elem xmltree.NodeID) bool {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xmltree.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "complexType", "simpleType":
			return true
		}
	}
	return false
}
