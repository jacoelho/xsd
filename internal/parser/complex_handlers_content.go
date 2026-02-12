package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func (s *complexTypeParseState) handleModelGroup(child xmltree.NodeID) error {
	if err := s.beginElementContent(); err != nil {
		return err
	}
	mg, err := parseModelGroup(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse model group: %w", err)
	}
	s.ct.SetContent(&model.ElementContent{Particle: mg})
	return nil
}

func (s *complexTypeParseState) handleAny(child xmltree.NodeID) error {
	if err := s.beginElementContent(); err != nil {
		return err
	}
	anyElem, err := parseAnyElement(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse any element: %w", err)
	}
	s.ct.SetContent(&model.ElementContent{Particle: anyElem})
	return nil
}

func (s *complexTypeParseState) handleGroupRef(child xmltree.NodeID) error {
	if err := s.beginElementContent(); err != nil {
		return err
	}
	if err := validateElementConstraints(s.doc, child, "group", s.schema); err != nil {
		return err
	}
	ref := s.doc.GetAttribute(child, "ref")
	if ref == "" {
		return fmt.Errorf("group reference missing ref attribute")
	}
	refQName, err := resolveQNameWithPolicy(s.doc, ref, child, s.schema, useDefaultNamespace)
	if err != nil {
		return fmt.Errorf("resolve group ref %s: %w", ref, err)
	}
	minOccurs, err := parseOccursAttr(s.doc, child, "minOccurs")
	if err != nil {
		return err
	}
	maxOccurs, err := parseOccursAttr(s.doc, child, "maxOccurs")
	if err != nil {
		return err
	}
	groupRef := &model.GroupRef{
		RefQName:  refQName,
		MinOccurs: minOccurs,
		MaxOccurs: maxOccurs,
	}
	s.ct.SetContent(&model.ElementContent{Particle: groupRef})
	return nil
}

func (s *complexTypeParseState) handleSimpleContent(child xmltree.NodeID) error {
	if err := s.beginDerivationContent("simpleContent"); err != nil {
		return err
	}
	s.hasSimpleContent = true
	sc, err := parseSimpleContent(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse simpleContent: %w", err)
	}
	s.ct.SetContent(sc)
	if sc.Extension != nil {
		s.ct.DerivationMethod = model.DerivationExtension
	} else if sc.Restriction != nil {
		s.ct.DerivationMethod = model.DerivationRestriction
	}
	return nil
}

func (s *complexTypeParseState) handleComplexContent(child xmltree.NodeID) error {
	if err := s.beginDerivationContent("complexContent"); err != nil {
		return err
	}
	s.hasComplexContent = true
	cc, err := parseComplexContent(s.doc, child, s.schema)
	if err != nil {
		return fmt.Errorf("parse complexContent: %w", err)
	}
	s.ct.SetContent(cc)
	if cc.Extension != nil {
		s.ct.DerivationMethod = model.DerivationExtension
	} else if cc.Restriction != nil {
		s.ct.DerivationMethod = model.DerivationRestriction
	}
	return nil
}
