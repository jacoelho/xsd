package loader

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/schemacheck"
)

// ValidateSchema validates that a parsed schema conforms to XSD constraints.
// Structural checks are performed first, then reference validation runs after.
func ValidateSchema(schema *parser.Schema) []error {
	errors := schemacheck.ValidateStructure(schema)
	if refErrors := resolver.ValidateReferences(schema); len(refErrors) > 0 {
		errors = append(errors, refErrors...)
	}
	if len(errors) == 0 && schema != nil {
		schema.UPAValidated = true
	}
	return errors
}
