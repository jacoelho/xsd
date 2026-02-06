package source

import (
	"github.com/jacoelho/xsd/internal/parser"
	semanticcheck "github.com/jacoelho/xsd/internal/semanticcheck"
	semanticresolve "github.com/jacoelho/xsd/internal/semanticresolve"
)

func ValidateSchema(schema *parser.Schema) []error {
	errors := semanticcheck.ValidateStructure(schema)
	if refErrors := semanticresolve.ValidateReferences(schema); len(refErrors) > 0 {
		errors = append(errors, refErrors...)
	}
	return errors
}
