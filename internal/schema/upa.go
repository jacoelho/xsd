package schema

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateUPA checks Unique Particle Attribution across all complex types.
func ValidateUPA(schema *parser.Schema, registry *Registry) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	if len(schema.GlobalDecls) == 0 && hasGlobalDecls(schema) {
		return fmt.Errorf("schema global declaration order missing")
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
