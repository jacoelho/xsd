package schema

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateUPA checks Unique Particle Attribution across all complex types.
func ValidateUPA(schema *parser.Schema, registry *Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	if err := validateSchemaInput(schema); err != nil {
		return err
	}

	for _, entry := range registry.TypeOrder {
		ct, ok := entry.Type.(*types.ComplexType)
		if !ok {
			continue
		}
		if err := schemacheck.ValidateUPA(schema, ct.Content(), schema.TargetNamespace); err != nil {
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
