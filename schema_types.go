package xsd

import "github.com/jacoelho/xsd/internal/qname"

// Schema wraps a compiled schema with convenience methods.
type Schema struct {
	engine *engine
}

// QName is a public qualified name with namespace and local part.
type QName = qname.QName
