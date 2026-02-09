package validatorcompile

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemaflow"
	schema "github.com/jacoelho/xsd/internal/semantic"
)

func resolveSchema(schemaXML string) (*parser.Schema, error) {
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	resolved, err := schemaflow.ResolveAndValidate(sch)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func parseAndAssign(schemaXML string) (*parser.Schema, *schema.Registry, error) {
	sch, err := resolveSchema(schemaXML)
	if err != nil {
		return nil, nil, err
	}
	reg, err := schema.AssignIDs(sch)
	if err != nil {
		return nil, nil, err
	}
	if _, err := schema.ResolveReferences(sch, reg); err != nil {
		return nil, nil, err
	}
	return sch, reg, nil
}

func mustResolveSchema(tb testing.TB, schemaXML string) *parser.Schema {
	tb.Helper()
	sch, err := resolveSchema(schemaXML)
	if err != nil {
		tb.Fatalf("parse schema: %v", err)
	}
	return sch
}
