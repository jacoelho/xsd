package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

func resolveAndValidateOwned(sch *parser.Schema) error {
	return ResolveAndValidateOwned(sch)
}

func validateUPA(schema *parser.Schema, registry *analysis.Registry) error {
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
		if err := semantics.ValidateUPA(schema, ct.Content(), schema.TargetNamespace); err != nil {
			return fmt.Errorf("%s: %w", complexTypeLabel(ct), err)
		}
	}
	return nil
}
