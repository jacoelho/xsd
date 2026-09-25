package schema

// ElementFrameRead is the immutable schema projection needed to initialize
// one validation element frame. The fields are values copied from published
// schema tables; changing a returned projection cannot mutate the Schema.
//
// SimpleContent is NoSimpleType when the effective type has no simple
// content. TextContent carries the element declaration's default/fixed
// constraint flags, and Content is the initial child content-model state.
type ElementFrameRead struct {
	SimpleContent SimpleTypeID
	TextContent   ElementTextContent
	Content       ContentFrame
}
