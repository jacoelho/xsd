package semanticresolve

import "github.com/jacoelho/xsd/internal/parser"

// ResolveTypeReferences resolves all type references after schema parsing.
// This is a compatibility wrapper around the unified resolver.
func ResolveTypeReferences(schema *parser.Schema) error {
	if schema == nil {
		return nil
	}
	res := NewResolver(schema)
	return res.Resolve()
}
