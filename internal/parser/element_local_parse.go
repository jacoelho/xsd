package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func parseLocalElement(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema, attrs *elementAttrScan) (*model.ElementDecl, error) {
	if attrs.name == "" {
		return nil, fmt.Errorf("element missing name and ref")
	}
	if attrs.invalidLocalAttr != "" {
		return nil, fmt.Errorf("invalid attribute '%s' on local element", attrs.invalidLocalAttr)
	}
	if err := validateLocalElementChildren(doc, elem); err != nil {
		return nil, err
	}
	if err := validateLocalElementAttributes(attrs); err != nil {
		return nil, err
	}

	hasInlineType := elementHasInlineType(doc, elem)
	if attrs.typ != "" && hasInlineType {
		return nil, fmt.Errorf("element cannot have both 'type' attribute and inline type definition")
	}

	effectiveForm, elementNamespace, err := resolveLocalElementForm(attrs, schema)
	if err != nil {
		return nil, err
	}

	minOccurs, maxOccurs, err := parseElementOccurs(attrs)
	if err != nil {
		return nil, err
	}

	decl := &model.ElementDecl{
		Name: model.QName{
			Namespace: elementNamespace,
			Local:     attrs.name,
		},
		SourceNamespace: schema.TargetNamespace,
		MinOccurs:       minOccurs,
		MaxOccurs:       maxOccurs,
	}
	decl.TypeExplicit = attrs.hasType || hasInlineType
	if effectiveForm == Qualified {
		decl.Form = model.FormQualified
	} else {
		decl.Form = model.FormUnqualified
	}

	typ, err := resolveElementType(doc, elem, schema, attrs)
	if err != nil {
		return nil, err
	}
	decl.Type = typ
	decl.TypeExplicit = attrs.hasType || hasInlineType

	err = applyElementConstraints(doc, elem, schema, attrs, decl)
	if err != nil {
		return nil, err
	}

	parsed, err := model.NewElementDeclFromParsed(decl)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}
