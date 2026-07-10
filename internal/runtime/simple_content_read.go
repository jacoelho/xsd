package runtime

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
