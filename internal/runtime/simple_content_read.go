package runtime

import "errors"

// SimpleContentTypeReadShape is the runtime-read projection needed to choose
// the simple type used for element text validation.
type SimpleContentTypeReadShape struct {
	Type    SimpleTypeID
	Present bool
}

// SimpleContentTypeRead exposes simple-content type facts without exposing the
// raw complex-type table.
type SimpleContentTypeRead struct {
	typ     SimpleTypeID
	present bool
}

// NewSimpleContentTypeRead returns the immutable simple-content type
// projection for one complex type.
func NewSimpleContentTypeRead(shape SimpleContentTypeReadShape) SimpleContentTypeRead {
	typ := shape.Type
	if !shape.Present {
		typ = NoSimpleType
	}
	return SimpleContentTypeRead{
		typ:     typ,
		present: shape.Present,
	}
}

// NewSimpleContentTypeReadForComplexType returns the immutable simple-content
// type projection for ct.
func NewSimpleContentTypeReadForComplexType(ct ComplexType) SimpleContentTypeRead {
	return NewSimpleContentTypeRead(simpleContentTypeReadShapeForComplexType(ct))
}

// NewSimpleContentTypeReadsForComplexTypes returns simple-content type
// projections for each complex type.
func NewSimpleContentTypeReadsForComplexTypes(complexTypes []ComplexType) []SimpleContentTypeRead {
	out := make([]SimpleContentTypeRead, len(complexTypes))
	for i := range complexTypes {
		out[i] = NewSimpleContentTypeReadForComplexType(complexTypes[i])
	}
	return out
}

// SimpleContentTypeByType returns the simple-content type for t from frozen
// runtime read projections. The booleans report has-simple-content and
// metadata-validity, respectively.
func SimpleContentTypeByType(simpleTypeCount int, reads []SimpleContentTypeRead, t TypeID) (SimpleTypeID, bool, bool) {
	if id, ok := t.Simple(); ok {
		if !ValidSimpleTypeID(id, simpleTypeCount) {
			return NoSimpleType, false, false
		}
		return id, true, true
	}
	id, ok := t.Complex()
	if !ok || !ValidComplexTypeID(id, len(reads)) {
		return NoSimpleType, false, false
	}
	read := reads[id]
	if !read.HasSimpleContent() {
		return NoSimpleType, false, true
	}
	textType := read.TypeID()
	if !ValidSimpleTypeID(textType, simpleTypeCount) {
		return NoSimpleType, false, false
	}
	return textType, true, true
}

// ValidateSimpleContentTypeReadProjection validates that read exposes the
// runtime-owned simple-content projection for shape.
func ValidateSimpleContentTypeReadProjection(read SimpleContentTypeRead, shape SimpleContentTypeReadShape, simpleTypeCount int) error {
	if read.HasSimpleContent() != shape.Present {
		return errors.New("complex simple content read projection does not match complex type")
	}
	if !shape.Present {
		if read.TypeID() != NoSimpleType {
			return errors.New("complex simple content read projection stores text type for non-simple content")
		}
		return nil
	}
	if read.TypeID() != shape.Type {
		return errors.New("complex simple content read projection does not match complex type")
	}
	if !ValidSimpleTypeID(read.TypeID(), simpleTypeCount) {
		return errors.New("complex simple content read projection references invalid text type")
	}
	return nil
}

// ValidateSimpleContentTypeReadForComplexType validates that read exposes the
// runtime-owned simple-content projection for ct.
func ValidateSimpleContentTypeReadForComplexType(read SimpleContentTypeRead, ct ComplexType, simpleTypeCount int) error {
	return ValidateSimpleContentTypeReadProjection(read, simpleContentTypeReadShapeForComplexType(ct), simpleTypeCount)
}

// ValidateSimpleContentTypeReadProjectionTable validates that reads expose the
// runtime-owned simple-content projections for complexTypes.
func ValidateSimpleContentTypeReadProjectionTable(reads []SimpleContentTypeRead, complexTypes []ComplexType, simpleTypeCount int) error {
	if len(reads) != len(complexTypes) {
		return errors.New("complex simple content read projection count does not match types")
	}
	for i, read := range reads {
		if err := ValidateSimpleContentTypeReadForComplexType(read, complexTypes[i], simpleTypeCount); err != nil {
			return err
		}
	}
	return nil
}

func simpleContentTypeReadShapeForComplexType(ct ComplexType) SimpleContentTypeReadShape {
	return SimpleContentTypeReadShape{
		Type:    ct.TextType,
		Present: ct.SimpleContent(),
	}
}

// TypeID returns the text type used for simple-content validation.
func (r SimpleContentTypeRead) TypeID() SimpleTypeID {
	return r.typ
}

// HasSimpleContent reports whether the complex type has simple content.
func (r SimpleContentTypeRead) HasSimpleContent() bool {
	return r.present
}
