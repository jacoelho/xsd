package prep

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semanticcheck"
)

// ValidateUPA checks Unique Particle Attribution across all complex model.
func ValidateUPA(schema *parser.Schema, registry *analysis.Registry) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	if err := analysis.RequireResolved(schema); err != nil {
		return err
	}

	for _, entry := range registry.TypeOrder {
		ct, ok := entry.Type.(*model.ComplexType)
		if !ok {
			continue
		}
		if err := semanticcheck.ValidateUPA(schema, ct.Content(), schema.TargetNamespace); err != nil {
			return fmt.Errorf("%s: %w", typeLabel(ct), err)
		}
	}
	return nil
}

func typeLabel(ct *model.ComplexType) string {
	if ct == nil {
		return "complexType"
	}
	if ct.QName.IsZero() {
		return "anonymous complexType"
	}
	return fmt.Sprintf("complexType %s", ct.QName)
}
