package runtime

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

// IsComplexType reports whether the frame type is complex.
func (c ElementChildContent) IsComplexType() bool {
	return c.complex
}

// HasSimpleContent reports whether the complex type has simple content.
func (c ElementChildContent) HasSimpleContent() bool {
	return c.simple
}
