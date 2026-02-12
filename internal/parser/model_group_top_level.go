package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

// parseTopLevelGroup parses a top-level <group> definition.
// Content model: (annotation?, (all | choice | sequence))
func parseTopLevelGroup(doc *xmltree.Document, elem xmltree.NodeID, schema *Schema) error {
	name := model.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
	if name == "" {
		return fmt.Errorf("group missing name attribute")
	}

	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() != "" {
			continue
		}
		attrName := attr.LocalName()
		if !validAttributeNames[attrSetTopLevelGroup][attrName] {
			return fmt.Errorf("invalid attribute '%s' on top-level group (only id, name allowed)", attrName)
		}
	}

	if err := validateOptionalID(doc, elem, "group", schema); err != nil {
		return err
	}

	qname := model.QName{Namespace: schema.TargetNamespace, Local: name}
	if _, exists := schema.Groups[qname]; exists {
		return fmt.Errorf("duplicate group definition: '%s'", name)
	}

	hasAnnotation := false
	hasModelGroup := false
	var mg *model.ModelGroup

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xmltree.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return fmt.Errorf("group '%s': at most one annotation is allowed", name)
			}
			if hasModelGroup {
				return fmt.Errorf("group '%s': annotation must appear before model group", name)
			}
			hasAnnotation = true
		case "sequence", "choice", "all":
			if hasModelGroup {
				return fmt.Errorf("group '%s': exactly one model group (all, choice, or sequence) is allowed", name)
			}
			parsed, err := parseModelGroup(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse model group: %w", err)
			}
			mg = parsed
			hasModelGroup = true
		case "key", "keyref", "unique":
			return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
		}
	}

	if mg == nil {
		return fmt.Errorf("group '%s' must contain exactly one model group (all, choice, or sequence)", name)
	}

	mg.SourceNamespace = schema.TargetNamespace
	schema.Groups[qname] = mg
	schema.addGlobalDecl(GlobalDeclGroup, qname)
	return nil
}
