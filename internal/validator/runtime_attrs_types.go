package validator

import "github.com/jacoelho/xsd/internal/runtime"

// AttrApplied records a defaulted/fixed attribute applied by the validator.
type AttrApplied struct {
	KeyBytes []byte
	Value    runtime.ValueRef
	Name     runtime.SymbolID
	Fixed    bool
	KeyKind  runtime.ValueKind
}

// AttrResult holds validated input attributes and applied default/fixed attributes.
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
