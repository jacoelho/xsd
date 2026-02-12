package analysis

import (
	"fmt"

	parser "github.com/jacoelho/xsd/internal/parser"
)

func validateSchemaInput(schema *parser.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if len(schema.GlobalDecls) == 0 && hasGlobalDecls(schema) {
		return fmt.Errorf("schema global declaration order missing")
	}
	return nil
}
