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

// NewElementTextContentsForComplexTypes returns character-data content
// projections for each complex type.
func NewElementTextContentsForComplexTypes(complexTypes []ComplexType, fixed bool) []ElementTextContent {
	out := make([]ElementTextContent, len(complexTypes))
	for i := range complexTypes {
		out[i] = NewElementTextContentForComplexType(complexTypes[i], fixed)
	}
	return out
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

// EqualElementTextContentForComplexType reports whether read exposes the
// runtime-owned character-data content projection for ct.
func EqualElementTextContentForComplexType(read ElementTextContent, ct ComplexType, fixed bool) bool {
	return EqualElementTextContent(read, NewElementTextContentForComplexType(ct, fixed))
}

// EqualElementTextContentProjection reports whether reads expose the
// runtime-owned character-data content projections for complexTypes.
func EqualElementTextContentProjection(reads []ElementTextContent, complexTypes []ComplexType, fixed bool) bool {
	if len(reads) != len(complexTypes) {
		return false
	}
	for i, read := range reads {
		if !EqualElementTextContentForComplexType(read, complexTypes[i], fixed) {
			return false
		}
	}
	return true
}

// ValidateElementTextContentProjection validates a complex-type text content
// read projection table.
func ValidateElementTextContentProjection(reads []ElementTextContent, complexTypes []ComplexType, fixed bool) error {
	label := "complex text content"
	if fixed {
		label = "fixed complex text content"
	}
	if len(reads) != len(complexTypes) {
		return errors.New(label + " read projection count does not match types")
	}
	if !EqualElementTextContentProjection(reads, complexTypes, fixed) {
		return errors.New(label + " read projection does not match type")
	}
	return nil
}

// ElementTextContentByType returns the character-data content projection for
// one validation frame from frozen runtime read projections.
func ElementTextContentByType(
	simpleTypeCount int,
	complexReads []ElementTextContent,
	fixedComplexReads []ElementTextContent,
	elementValues []ElementValueConstraints,
	simpleRead ElementTextContent,
	t TypeID,
	elem ElementID,
) (ElementTextContent, bool) {
	if !ValidTypeID(t, simpleTypeCount, len(complexReads)) {
		return ElementTextContent{}, false
	}
	if elem != NoElement && !ValidElementID(elem, len(elementValues)) {
		return ElementTextContent{}, false
	}
	if id, ok := t.Complex(); ok {
		if !ValidComplexTypeID(id, len(complexReads)) {
			return ElementTextContent{}, false
		}
		if elem != NoElement {
			if _, fixed := elementValues[elem].FixedValue(); fixed {
				if !ValidComplexTypeID(id, len(fixedComplexReads)) {
					return ElementTextContent{}, false
				}
				return fixedComplexReads[id], true
			}
		}
		return complexReads[id], true
	}
	if !simpleRead.HasSimpleContent() || simpleRead.IsComplexType() {
		return ElementTextContent{}, false
	}
	return simpleRead, true
}

// ElementHasSimpleContentByType reports whether the frame selected by t and
// elem accepts simple content.
func ElementHasSimpleContentByType(
	simpleTypeCount int,
	complexReads []ElementTextContent,
	fixedComplexReads []ElementTextContent,
	elementValues []ElementValueConstraints,
	simpleRead ElementTextContent,
	t TypeID,
	elem ElementID,
) (bool, bool) {
	content, ok := ElementTextContentByType(simpleTypeCount, complexReads, fixedComplexReads, elementValues, simpleRead, t, elem)
	if !ok {
		return false, false
	}
	return content.HasSimpleContent(), true
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

// NewElementChildContentsForComplexTypes returns child-content projections for
// each complex type.
func NewElementChildContentsForComplexTypes(complexTypes []ComplexType) []ElementChildContent {
	out := make([]ElementChildContent, len(complexTypes))
	for i := range complexTypes {
		out[i] = NewElementChildContentForComplexType(complexTypes[i])
	}
	return out
}

// EqualElementChildContent reports whether two child-content projections expose
// the same validation-facing content facts.
func EqualElementChildContent(a, b ElementChildContent) bool {
	return a == b
}

// EqualElementChildContentForComplexType reports whether read exposes the
// runtime-owned child-content projection for ct.
func EqualElementChildContentForComplexType(read ElementChildContent, ct ComplexType) bool {
	return EqualElementChildContent(read, NewElementChildContentForComplexType(ct))
}

// EqualElementChildContentProjection reports whether reads expose the
// runtime-owned child-content projections for complexTypes.
func EqualElementChildContentProjection(reads []ElementChildContent, complexTypes []ComplexType) bool {
	if len(reads) != len(complexTypes) {
		return false
	}
	for i, read := range reads {
		if !EqualElementChildContentForComplexType(read, complexTypes[i]) {
			return false
		}
	}
	return true
}

// ValidateElementChildContentProjection validates a complex-type child content
// read projection table.
func ValidateElementChildContentProjection(reads []ElementChildContent, complexTypes []ComplexType) error {
	if len(reads) != len(complexTypes) {
		return errors.New("complex child content read projection count does not match types")
	}
	if !EqualElementChildContentProjection(reads, complexTypes) {
		return errors.New("complex child content read projection does not match type")
	}
	return nil
}

// ElementChildContentByType returns the child-content projection for t from
// frozen runtime read projections.
func ElementChildContentByType(simpleTypeCount int, reads []ElementChildContent, t TypeID) (ElementChildContent, bool) {
	if !ValidTypeID(t, simpleTypeCount, len(reads)) {
		return ElementChildContent{}, false
	}
	if _, ok := t.Simple(); ok {
		return ElementChildContent{}, true
	}
	id, ok := t.Complex()
	if !ok || !ValidComplexTypeID(id, len(reads)) {
		return ElementChildContent{}, false
	}
	return reads[id], true
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
