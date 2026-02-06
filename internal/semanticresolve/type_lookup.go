package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// lookupTypeInSchema finds a type by QName in schema or builtins.
// Returns an error if type is not found.
func lookupTypeInSchema(schema *parser.Schema, qname types.QName) (types.Type, error) {
	return typeops.ResolveTypeQName(schema, qname, typeops.TypeReferenceMustExist)
}
