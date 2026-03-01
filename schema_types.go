package xsd

import (
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/validator"
)

// Schema wraps a compiled schema with convenience methods.
type Schema struct {
	engine *validator.Engine
}

// QName is a public qualified name with namespace and local part.
type QName = qname.QName
