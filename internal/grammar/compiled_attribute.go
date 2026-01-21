package grammar

import "github.com/jacoelho/xsd/internal/types"

// CompiledAttribute is a fully-resolved attribute declaration.
type CompiledAttribute struct {
	Original   *types.AttributeDecl
	Type       *CompiledType
	QName      types.QName
	Default    string
	Fixed      string
	Use        types.AttributeUse
	HasDefault bool
	HasFixed   bool
}
