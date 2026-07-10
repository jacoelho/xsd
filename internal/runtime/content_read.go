package runtime

import "errors"

// ChildContentInfo summarizes whether a type can accept child elements.
type ChildContentInfo struct {
	Complex bool
	Simple  bool
}

// NewChildContentInfoForElementChildContent projects child-content facts into
// the validation-facing summary used by content frames.
func NewChildContentInfoForElementChildContent(content ElementChildContent) ChildContentInfo {
	return ChildContentInfo{
		Complex: content.IsComplexType(),
		Simple:  content.HasSimpleContent(),
	}
}

// ElementTextContentShape is the runtime-read projection needed to classify
// character data for one element frame.
type ElementTextContentShape struct {
	Simple  bool
	Complex bool
	Mixed   bool
	Fixed   bool
}

// ElementTextContent summarizes whether character data is allowed in one
// element frame.
type ElementTextContent struct {
	simple  bool
	complex bool
	mixed   bool
	fixed   bool
}

// NewElementTextContent returns an immutable character-data content projection.
func NewElementTextContent(shape ElementTextContentShape) ElementTextContent {
	return ElementTextContent{
		simple:  shape.Simple,
		complex: shape.Complex,
		mixed:   shape.Mixed,
		fixed:   shape.Fixed,
	}
}

// NewElementTextContentForSimpleType returns the character-data content
// projection for a simple type.
func NewElementTextContentForSimpleType() ElementTextContent {
	return NewElementTextContent(ElementTextContentShape{Simple: true})
}

// NewElementTextContentForComplexType returns the character-data content
// projection for a complex type.
func NewElementTextContentForComplexType(ct ComplexType, fixed bool) ElementTextContent {
	return NewElementTextContent(elementTextContentShapeForComplexType(ct, fixed))
}

// EqualElementTextContent reports whether two character-data content
// projections expose the same validation-facing content facts.
func EqualElementTextContent(a, b ElementTextContent) bool {
	return a == b
}

// EqualElementTextContentForSimpleType reports whether read exposes the
// runtime-owned character-data content projection for a simple type.
func EqualElementTextContentForSimpleType(read ElementTextContent) bool {
	return EqualElementTextContent(read, NewElementTextContentForSimpleType())
}

// ValidateElementTextContentForSimpleType validates the simple-type text
// content read projection.
func ValidateElementTextContentForSimpleType(read ElementTextContent) error {
	if !EqualElementTextContentForSimpleType(read) {
		return errors.New("simple text content read projection does not match simple type")
	}
	return nil
}

func elementTextContentShapeForComplexType(ct ComplexType, fixed bool) ElementTextContentShape {
	return ElementTextContentShape{
		Simple:  ct.SimpleContent(),
		Complex: true,
		Mixed:   ct.Mixed(),
		Fixed:   fixed,
	}
}

// HasSimpleContent reports whether the frame accepts simple character content.
func (c ElementTextContent) HasSimpleContent() bool {
	return c.simple
}

// IsComplexType reports whether the frame type is complex.
func (c ElementTextContent) IsComplexType() bool {
	return c.complex
}

// AllowsMixedContent reports whether non-whitespace character data is allowed
// alongside child elements.
func (c ElementTextContent) AllowsMixedContent() bool {
	return c.mixed
}

// HasFixedElementValue reports whether a fixed element constraint requires
// retaining mixed character data for later comparison.
func (c ElementTextContent) HasFixedElementValue() bool {
	return c.fixed
}

// ElementChildContentShape is the runtime-read projection needed before
// accepting child elements.
type ElementChildContentShape struct {
	Complex bool
	Simple  bool
}

// ElementChildContent summarizes whether a type can accept child elements.
type ElementChildContent struct {
	complex bool
	simple  bool
}

// NewElementChildContent returns an immutable child-content projection.
func NewElementChildContent(shape ElementChildContentShape) ElementChildContent {
	return ElementChildContent{
		complex: shape.Complex,
		simple:  shape.Simple,
	}
}

// NewElementChildContentForComplexType returns the child-content projection for
// a complex type.
func NewElementChildContentForComplexType(ct ComplexType) ElementChildContent {
	return NewElementChildContent(elementChildContentShapeForComplexType(ct))
}

func elementChildContentShapeForComplexType(ct ComplexType) ElementChildContentShape {
	return ElementChildContentShape{
		Complex: true,
		Simple:  ct.SimpleContent(),
	}
}

// IsComplexType reports whether the frame type is complex.
func (c ElementChildContent) IsComplexType() bool {
	return c.complex
}

// HasSimpleContent reports whether the complex type has simple content.
func (c ElementChildContent) HasSimpleContent() bool {
	return c.simple
}
