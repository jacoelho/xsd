package parser

import (
	"fmt"
)

func (s *complexTypeParseState) handleAttribute(child NodeID) error {
	if err := s.beginAttributeLike(); err != nil {
		return err
	}
	attr, err := parseAttribute(s.doc, child, s.schema, true)
	if err != nil {
		return fmt.Errorf("complexType: parse attribute: %w", err)
	}
	s.ct.SetAttributes(append(s.ct.Attributes(), attr))
	return nil
}

func (s *complexTypeParseState) handleAttributeGroup(child NodeID) error {
	if err := s.beginAttributeLike(); err != nil {
		return err
	}
	refQName, err := parseAttributeGroupRefQName(s.doc, child, s.schema)
	if err != nil {
		return err
	}
	s.ct.AttrGroups = append(s.ct.AttrGroups, refQName)
	return nil
}

func (s *complexTypeParseState) handleAnyAttribute(child NodeID) error {
	s.hasNonAnnotation = true
	s.hasAttributeLike = true
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
	}
	if s.hasAnyAttribute {
		return fmt.Errorf("complexType: at most one anyAttribute is allowed")
	}
	s.hasAnyAttribute = true
	anyAttr, err := parseAnyAttribute(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse anyAttribute: %w", err)
	}
	s.ct.SetAnyAttribute(anyAttr)
	return nil
}
