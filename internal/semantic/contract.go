package semantic

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

// MarkSemantic advances the schema phase to Semantic after structure validation.
func MarkSemantic(schema *parser.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if schema.Phase > parser.PhaseSemantic {
		return fmt.Errorf("schema phase %s cannot be marked semantic", schema.Phase)
	}
	if schema.Phase < parser.PhaseParsed {
		return fmt.Errorf("schema phase %s cannot be marked semantic", schema.Phase)
	}
	schema.Phase = parser.PhaseSemantic
	return nil
}

// MarkResolved advances the schema phase to Resolved once references are validated.
func MarkResolved(schema *parser.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if schema.Phase > parser.PhaseResolved {
		return fmt.Errorf("schema phase %s cannot be marked resolved", schema.Phase)
	}
	if schema.Phase < parser.PhaseSemantic {
		return fmt.Errorf("schema phase %s must be semantic before resolving", schema.Phase)
	}
	if schema.HasPlaceholders {
		return fmt.Errorf("schema has unresolved placeholders")
	}
	schema.Phase = parser.PhaseResolved
	return nil
}

// RequireResolved ensures the schema is resolved and placeholder-free.
func RequireResolved(schema *parser.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if schema.Phase < parser.PhaseResolved {
		return fmt.Errorf("schema phase %s is not resolved", schema.Phase)
	}
	if schema.HasPlaceholders {
		return fmt.Errorf("schema has unresolved placeholders")
	}
	return nil
}

// RequireRuntimeReady ensures the schema is ready for runtime validation.
func RequireRuntimeReady(schema *parser.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if schema.Phase < parser.PhaseRuntimeReady {
		return fmt.Errorf("schema phase %s is not runtime-ready", schema.Phase)
	}
	return nil
}
