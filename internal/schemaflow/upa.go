package schemaflow

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/semanticcheck"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateUPA checks Unique Particle Attribution across all complex types.
func ValidateUPA(schema *parser.Schema, registry *semantic.Registry) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	if err := semantic.RequireResolved(schema); err != nil {
		return err
	}

	for _, entry := range registry.TypeOrder {
		ct, ok := entry.Type.(*types.ComplexType)
		if !ok {
			continue
		}
		if err := semanticcheck.ValidateUPA(schema, ct.Content(), schema.TargetNamespace); err != nil {
			return fmt.Errorf("%s: %w", typeLabel(ct), err)
		}
	}
	return nil
}

func typeLabel(ct *types.ComplexType) string {
	if ct == nil {
		return "complexType"
	}
	if ct.QName.IsZero() {
		return "anonymous complexType"
	}
	return fmt.Sprintf("complexType %s", ct.QName)
}
