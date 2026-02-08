package validator

import "github.com/jacoelho/xsd/internal/runtime"

type AttrApplied struct {
	KeyBytes []byte
	Value    runtime.ValueRef
	Name     runtime.SymbolID
	Fixed    bool
	KeyKind  runtime.ValueKind
}

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
