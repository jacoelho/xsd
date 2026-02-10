package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseModelGroup(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*model.ModelGroup, error) {
	kind, err := parseModelGroupKind(doc, elem)
	if err != nil {
		return nil, err
	}

	err = validateModelGroupAttributes(doc, elem)
	if err != nil {
		return nil, err
	}
	err = validateOptionalID(doc, elem, doc.LocalName(elem), schema)
	if err != nil {
		return nil, err
	}

	minOccurs, err := parseOccursAttr(doc, elem, "minOccurs")
	if err != nil {
		return nil, err
	}
	maxOccurs, err := parseOccursAttr(doc, elem, "maxOccurs")
	if err != nil {
		return nil, err
	}
	if kind == model.AllGroup {
		if !minOccurs.IsZero() && !minOccurs.IsOne() {
			return nil, fmt.Errorf("xs:all must have minOccurs='0' or '1'")
		}
		if !maxOccurs.IsOne() {
			return nil, fmt.Errorf("xs:all must have maxOccurs='1'")
		}
	}

	mg := &model.ModelGroup{
		Kind:      kind,
		Particles: []model.Particle{},
		MinOccurs: minOccurs,
		MaxOccurs: maxOccurs,
	}

	hasAnnotation := false
	hasNonAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		if doc.LocalName(child) == "annotation" {
			if hasAnnotation {
				return nil, fmt.Errorf("%s: at most one annotation is allowed", doc.LocalName(elem))
			}
			if hasNonAnnotation {
				return nil, fmt.Errorf("%s: annotation must appear before other elements", doc.LocalName(elem))
			}
			hasAnnotation = true
			continue
		}
		hasNonAnnotation = true
		particle, err := parseModelGroupChildParticle(doc, child, schema, kind, doc.LocalName(elem))
		if err != nil {
			return nil, err
		}
		if particle != nil {
			mg.Particles = append(mg.Particles, particle)
		}
	}

	return mg, nil
}

func parseModelGroupKind(doc *xsdxml.Document, elem xsdxml.NodeID) (model.GroupKind, error) {
	switch doc.LocalName(elem) {
	case "sequence":
		return model.Sequence, nil
	case "choice":
		return model.Choice, nil
	case "all":
		return model.AllGroup, nil
	default:
		return 0, fmt.Errorf("unknown model group: %s", doc.LocalName(elem))
	}
}

func validateModelGroupAttributes(doc *xsdxml.Document, elem xsdxml.NodeID) error {
	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() != "" {
			continue
		}
		attrName := attr.LocalName()
		if !validAttributeNames[attrSetModelGroup][attrName] {
			return fmt.Errorf("invalid attribute '%s' on <%s> (only id, minOccurs, maxOccurs allowed)", attrName, doc.LocalName(elem))
		}
		switch attrName {
		case "minOccurs", "maxOccurs":
			if attr.Value() == "" {
				return fmt.Errorf("%s: %s attribute cannot be empty", doc.LocalName(elem), attrName)
			}
		}
	}
	return nil
}
