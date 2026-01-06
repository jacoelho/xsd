package grammar

import "github.com/jacoelho/xsd/internal/types"

// CompiledAttribute is a fully-resolved attribute declaration.
type CompiledAttribute struct {
	QName    types.QName
	Original *types.AttributeDecl
	Type     *CompiledType      // Direct pointer to simple type
	Use      types.AttributeUse // Required, Optional, Prohibited
	Default  string
	Fixed    string
	HasFixed bool // true if fixed="" was explicitly present (even if empty)
}
