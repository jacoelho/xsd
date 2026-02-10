package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

type complexTypeParseState struct {
	doc    *schemaxml.Document
	schema *Schema
	ct     *model.ComplexType

	hasAnnotation     bool
	hasNonAnnotation  bool
	hasAnyAttribute   bool
	hasParticle       bool
	hasSimpleContent  bool
	hasComplexContent bool
	hasAttributeLike  bool
}

func (s *complexTypeParseState) handleChild(child schemaxml.NodeID) error {
	switch s.doc.LocalName(child) {
	case "annotation":
		return s.handleAnnotation()
	case "sequence", "choice", "all":
		return s.handleModelGroup(child)
	case "any":
		return s.handleAny(child)
	case "group":
		return s.handleGroupRef(child)
	case "attribute":
		return s.handleAttribute(child)
	case "attributeGroup":
		return s.handleAttributeGroup(child)
	case "anyAttribute":
		return s.handleAnyAttribute(child)
	case "simpleContent":
		return s.handleSimpleContent(child)
	case "complexContent":
		return s.handleComplexContent(child)
	case "key", "keyref", "unique":
		return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", s.doc.LocalName(child))
	default:
		return fmt.Errorf("complexType: unexpected child element '%s'", s.doc.LocalName(child))
	}
}

func (s *complexTypeParseState) handleAnnotation() error {
	if s.hasAnnotation {
		return fmt.Errorf("complexType: at most one annotation is allowed")
	}
	if s.hasNonAnnotation {
		return fmt.Errorf("complexType: annotation must appear before other elements")
	}
	s.hasAnnotation = true
	return nil
}
