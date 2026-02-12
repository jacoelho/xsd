package validator

import "github.com/jacoelho/xsd/internal/runtime"

// AttrApplied defines an exported type.
type AttrApplied struct {
	KeyBytes []byte
	Value    runtime.ValueRef
	Name     runtime.SymbolID
	Fixed    bool
	KeyKind  runtime.ValueKind
}

// AttrResult defines an exported type.
type AttrResult struct {
	Applied []AttrApplied
	Attrs   []StartAttr
}

var (
	xsiLocalType                      = []byte("type")
	xsiLocalNil                       = []byte("nil")
	xsiLocalSchemaLocation            = []byte("schemaLocation")
	xsiLocalNoNamespaceSchemaLocation = []byte("noNamespaceSchemaLocation")
)
