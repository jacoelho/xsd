package grammar

import "github.com/jacoelho/xsd/internal/types"

// CompiledAttribute is a fully-resolved attribute declaration.
type CompiledAttribute struct {
	QName    types.QName
	Original *types.AttributeDecl
	Type     *CompiledType
	Use      types.AttributeUse
	Default  string
	Fixed    string
	// true if fixed="" was explicitly present (even if empty)
	HasFixed bool
}
