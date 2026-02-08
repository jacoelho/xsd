package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

// makeAnyType returns the canonical anyType instance.
// anyType is treated as a built-in type throughout the system.
func makeAnyType() types.Type {
	return types.GetBuiltin(types.TypeNameAnyType)
}

// parseTopLevelElement parses a top-level element declaration.
func parseTopLevelElement(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
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

	decl, declErr := types.NewElementDeclFromParsed(decl)
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

func validateTopLevelElementStructure(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (string, error) {
	name := getNameAttr(doc, elem)
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

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation", "complexType", "simpleType", "key", "keyref", "unique":
			// allowed.
		default:
			return "", fmt.Errorf("invalid child element <%s> in <element> declaration", doc.LocalName(child))
		}
	}

	if doc.HasAttribute(elem, "default") && doc.HasAttribute(elem, "fixed") {
		return "", fmt.Errorf("element cannot have both 'default' and 'fixed' attributes")
	}

	return name, nil
}

func newTopLevelElementDecl(name string, schema *Schema) *types.ElementDecl {
	return &types.ElementDecl{
		Name: types.QName{
			Namespace: schema.TargetNamespace,
			Local:     name,
		},
		MinOccurs:       types.OccursFromInt(1),
		MaxOccurs:       types.OccursFromInt(1),
		SourceNamespace: schema.TargetNamespace,
		Form:            types.FormQualified,
	}
}
