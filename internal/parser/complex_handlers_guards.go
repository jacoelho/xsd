package parser

import "fmt"

func (s *complexTypeParseState) beginElementContent() error {
	s.hasNonAnnotation = true
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: element content cannot appear with simpleContent or complexContent")
	}
	if s.hasAttributeLike {
		return fmt.Errorf("complexType: content model must appear before attributes")
	}
	if s.hasParticle {
		return fmt.Errorf("complexType: only one content model is allowed")
	}
	s.hasParticle = true
	return nil
}

func (s *complexTypeParseState) beginAttributeLike() error {
	s.hasNonAnnotation = true
	s.hasAttributeLike = true
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
	}
	if s.hasAnyAttribute {
		return fmt.Errorf("complexType: anyAttribute must appear after all attributes")
	}
	return nil
}

func (s *complexTypeParseState) beginDerivationContent(kind string) error {
	s.hasNonAnnotation = true
	if s.hasParticle || s.hasAttributeLike {
		return fmt.Errorf("complexType: %s must be the only content model", kind)
	}
	if s.hasSimpleContent || s.hasComplexContent {
		return fmt.Errorf("complexType: only one content model is allowed")
	}
	return nil
}
