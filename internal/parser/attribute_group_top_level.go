package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

// parseTopLevelAttributeGroup parses a top-level <attributeGroup> definition
// Content model: (annotation?, ((attribute | attributeGroup)*, anyAttribute?))
func parseTopLevelAttributeGroup(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	name := getNameAttr(doc, elem)
	if name == "" {
		return fmt.Errorf("attributeGroup missing name attribute")
	}

	if err := validateOptionalID(doc, elem, "attributeGroup", schema); err != nil {
		return err
	}

	attrGroup := &types.AttributeGroup{
		Name: types.QName{
			Namespace: schema.TargetNamespace,
			Local:     name,
		},
		Attributes:      []*types.AttributeDecl{},
		AttrGroups:      []types.QName{},
		SourceNamespace: schema.TargetNamespace,
	}

	hasAnnotation := false
	hasNonAnnotation := false
	hasAnyAttribute := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return fmt.Errorf("attributeGroup '%s': at most one annotation is allowed", name)
			}
			if hasNonAnnotation {
				return fmt.Errorf("attributeGroup '%s': annotation must appear before other elements", name)
			}
			hasAnnotation = true

		case "attribute":
			hasNonAnnotation = true
			attr, err := parseAttribute(doc, child, schema, true)
			if err != nil {
				return fmt.Errorf("attributeGroup: parse attribute: %w", err)
			}
			attrGroup.Attributes = append(attrGroup.Attributes, attr)

		case "attributeGroup":
			hasNonAnnotation = true
			if doc.HasAttribute(child, "name") {
				return fmt.Errorf("attributeGroup reference cannot have 'name' attribute")
			}
			refQName, err := parseAttributeGroupRefQName(doc, child, schema)
			if err != nil {
				return err
			}
			attrGroup.AttrGroups = append(attrGroup.AttrGroups, refQName)

		case "anyAttribute":
			hasNonAnnotation = true
			if hasAnyAttribute {
				return fmt.Errorf("attributeGroup '%s': at most one anyAttribute is allowed", name)
			}
			hasAnyAttribute = true
			anyAttr, err := parseAnyAttribute(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse anyAttribute in attributeGroup: %w", err)
			}
			attrGroup.AnyAttribute = anyAttr

		case "key", "keyref", "unique":
			return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
		default:
			return fmt.Errorf("invalid child element <%s> in <attributeGroup> declaration", doc.LocalName(child))
		}
	}

	qname := types.QName{
		Namespace: schema.TargetNamespace,
		Local:     name,
	}
	if _, exists := schema.AttributeGroups[qname]; exists {
		return fmt.Errorf("attributeGroup %s already defined", qname)
	}
	schema.AttributeGroups[qname] = attrGroup
	schema.addGlobalDecl(GlobalDeclAttributeGroup, qname)
	return nil
}
