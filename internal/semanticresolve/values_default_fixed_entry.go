package semanticresolve

import (
	"errors"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type idValuePolicy int

const (
	idValuesAllowed idValuePolicy = iota
	idValuesDisallowed
)

var errCircularReference = errors.New("circular type reference")

// validateDefaultOrFixedValueWithResolvedType validates a default/fixed value after type resolution.
func validateDefaultOrFixedValueWithResolvedType(schema *parser.Schema, value string, typ types.Type, context map[string]string) error {
	return validateDefaultOrFixedValueWithResolvedTypeVisited(schema, value, typ, context, make(map[types.Type]bool))
}

func validateDefaultOrFixedValueWithResolvedTypeVisited(schema *parser.Schema, value string, typ types.Type, context map[string]string, visited map[types.Type]bool) error {
	return validateDefaultOrFixedValueResolved(schema, value, typ, context, visited, idValuesDisallowed)
}
