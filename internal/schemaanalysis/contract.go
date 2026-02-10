package schemaanalysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

// RequireResolved ensures the schema is placeholder-free.
func RequireResolved(schema *parser.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if parser.HasPlaceholders(schema) {
		return fmt.Errorf("schema has unresolved placeholders")
	}
	return nil
}
