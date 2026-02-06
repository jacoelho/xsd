package source

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/pipeline"
)

func ValidateSchema(schema *parser.Schema) []error {
	if schema == nil {
		return []error{fmt.Errorf("schema is nil")}
	}
	if _, err := pipeline.Prepare(schema); err != nil {
		return []error{err}
	}
	return nil
}
