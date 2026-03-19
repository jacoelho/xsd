package compiler

import (
	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/parser"
)

func resolveAndValidateOwned(sch *parser.Schema) error {
	return ResolveAndValidateOwned(sch)
}

func validateUPA(schema *parser.Schema, registry *analysis.Registry) error {
	return ValidateUPA(schema, registry)
}
