package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/value"
)

// parseTopLevelGroup parses a top-level <group> definition.
// Content model: (annotation?, (all | choice | sequence))
func parseTopLevelGroup(doc *Document, elem NodeID, schema *Schema) error {
	name := model.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
	if name == "" {
		return fmt.Errorf("group missing name attribute")
	}

	if err := validateElementAttributes(
		doc,
		elem,
		validAttributeNames[attrSetTopLevelGroup],
		"top-level group (only id, name allowed)",
	); err != nil {
		return err
	}

	if err := validateOptionalID(doc, elem, "group", schema); err != nil {
		return err
	}

	qname := model.QName{Namespace: schema.TargetNamespace, Local: name}
	if _, exists := schema.Groups[qname]; exists {
		return fmt.Errorf("duplicate group definition: '%s'", name)
	}

	hasAnnotation := false
	var mg *model.ModelGroup

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != value.XSDNamespace {
			continue
		}
		childName := doc.LocalName(child)

		switch childName {
		case "annotation":
			if hasAnnotation {
				return fmt.Errorf("group '%s': at most one annotation is allowed", name)
			}
			if mg != nil {
				return fmt.Errorf("group '%s': annotation must appear before model group", name)
			}
			hasAnnotation = true
		case "sequence", "choice", "all":
			if mg != nil {
				return fmt.Errorf("group '%s': exactly one model group (all, choice, or sequence) is allowed", name)
			}
			parsed, err := parseModelGroup(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse model group: %w", err)
			}
			mg = parsed
		case "key", "keyref", "unique":
			return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", childName)
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
