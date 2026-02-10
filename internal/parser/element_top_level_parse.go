package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

// parseTopLevelElement parses a top-level element declaration.
func parseTopLevelElement(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) error {
	name, nameErr := validateTopLevelElementStructure(doc, elem, schema)
	if nameErr != nil {
		return nameErr
	}

	decl := newTopLevelElementDecl(name, schema)
	typ, explicit, typeErr := resolveTopLevelElementType(doc, elem, schema)
	if typeErr != nil {
		return typeErr
	}
	decl.Type = typ
	decl.TypeExplicit = explicit

	if attrErr := applyTopLevelElementAttributes(doc, elem, schema, decl); attrErr != nil {
		return attrErr
	}

	decl, declErr := model.NewElementDeclFromParsed(decl)
	if declErr != nil {
		return declErr
	}

	if _, exists := schema.ElementDecls[decl.Name]; exists {
		return fmt.Errorf("duplicate element declaration: '%s'", decl.Name)
	}
	schema.ElementDecls[decl.Name] = decl
	schema.addGlobalDecl(GlobalDeclElement, decl.Name)
	return nil
}

func validateTopLevelElementStructure(doc *schemaxml.Document, elem schemaxml.NodeID, schema *Schema) (string, error) {
	name := model.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
	if name == "" {
		return "", fmt.Errorf("element missing name attribute")
	}

	if err := validateElementAttributes(doc, elem, validTopLevelElementAttributes, "top-level element"); err != nil {
		return "", err
	}
	if err := validateOptionalID(doc, elem, "element", schema); err != nil {
		return "", err
	}
	if doc.HasAttribute(elem, "form") {
		return "", fmt.Errorf("top-level element cannot have 'form' attribute")
	}
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return "", err
	}
	if err := validateElementChildrenOrder(doc, elem); err != nil {
		return "", err
	}

	if err := validateElementChildren(doc, elem); err != nil {
		return "", err
	}
	if err := validateElementDefaultFixedConflict(doc.HasAttribute(elem, "default"), doc.HasAttribute(elem, "fixed")); err != nil {
		return "", err
	}

	return name, nil
}

func newTopLevelElementDecl(name string, schema *Schema) *model.ElementDecl {
	return &model.ElementDecl{
		Name: model.QName{
			Namespace: schema.TargetNamespace,
			Local:     name,
		},
		MinOccurs:       model.OccursFromInt(1),
		MaxOccurs:       model.OccursFromInt(1),
		SourceNamespace: schema.TargetNamespace,
		Form:            model.FormQualified,
	}
}
