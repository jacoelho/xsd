package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/valuevalidate"
)

func validateValueAgainstTypeWithFacets(schema *parser.Schema, value string, typ model.Type, context map[string]string) error {
	return valuevalidate.ValidateWithFacets(schema, value, typ, context, convertDeferredFacet)
}
