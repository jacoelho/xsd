package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateValueAgainstTypeWithFacets(schema *parser.Schema, value string, typ model.Type, context map[string]string) error {
	return ValidateWithFacets(schema, value, typ, context, convertDeferredFacet)
}
