package runtimeassemble

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
	"github.com/jacoelho/xsd/internal/schemaprep"
)

func resolveSchema(schemaXML string) (*parser.Schema, error) {
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	resolved, err := schemaprep.ResolveAndValidate(sch)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func mustResolveSchema(tb testing.TB, schemaXML string) *parser.Schema {
	tb.Helper()
	sch, err := resolveSchema(schemaXML)
	if err != nil {
		tb.Fatalf("parse schema: %v", err)
	}
	return sch
}

func buildSchemaForTest(sch *parser.Schema, cfg BuildConfig) (*runtime.Schema, error) {
	if sch == nil {
		return nil, fmt.Errorf("runtime build: schema is nil")
	}
	resolvedSchema, err := schemaprep.ResolveAndValidate(sch)
	if err != nil {
		return nil, fmt.Errorf("runtime build: %w", err)
	}
	reg, err := schema.AssignIDs(resolvedSchema)
	if err != nil {
		return nil, fmt.Errorf("runtime build: assign IDs: %w", err)
	}
	refs, err := schema.ResolveReferences(resolvedSchema, reg)
	if err != nil {
		return nil, fmt.Errorf("runtime build: resolve references: %w", err)
	}
	if err := schema.DetectCycles(resolvedSchema); err != nil {
		return nil, fmt.Errorf("runtime build: detect cycles: %w", err)
	}
	if err := schemaprep.ValidateUPA(resolvedSchema, reg); err != nil {
		return nil, fmt.Errorf("runtime build: validate UPA: %w", err)
	}
	return BuildArtifacts(resolvedSchema, reg, refs, cfg)
}
