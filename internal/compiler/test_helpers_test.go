package compiler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/semantics"
)

func resolveSchema(schemaXML string) (*parser.Schema, error) {
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	if err := resolveAndValidateOwned(sch); err != nil {
		return nil, err
	}
	return sch, nil
}

func buildSchemaForTest(sch *parser.Schema, cfg BuildConfig) (*runtime.Schema, error) {
	if sch == nil {
		return nil, fmt.Errorf("runtime build: schema is nil")
	}
	resolvedSchema := parser.CloneSchema(sch)
	if err := resolveAndValidateOwned(resolvedSchema); err != nil {
		return nil, fmt.Errorf("runtime build: %w", err)
	}
	registry, err := analysis.AssignIDs(resolvedSchema)
	if err != nil {
		return nil, fmt.Errorf("runtime build: assign IDs: %w", err)
	}
	if err := analysis.DetectCycles(resolvedSchema); err != nil {
		return nil, fmt.Errorf("runtime build: detect cycles: %w", err)
	}
	if err := validateUPA(resolvedSchema, registry); err != nil {
		return nil, fmt.Errorf("runtime build: validate UPA: %w", err)
	}
	refs, err := analysis.ResolveReferences(resolvedSchema, registry)
	if err != nil {
		return nil, fmt.Errorf("runtime build: resolve references: %w", err)
	}
	sem, err := semantics.Build(resolvedSchema, registry, refs)
	if err != nil {
		return nil, fmt.Errorf("runtime build: semantics: %w", err)
	}
	if err := sem.Particles().ValidateUPA(); err != nil {
		return nil, fmt.Errorf("runtime build: validate UPA: %w", err)
	}
	validators, err := sem.CompiledValidators()
	if err != nil {
		return nil, fmt.Errorf("runtime build: compile validators: %w", err)
	}
	return BuildArtifacts(resolvedSchema, registry, refs, validators, cfg)
}

func mustResolveSchema(tb testing.TB, schemaXML string) *parser.Schema {
	tb.Helper()
	sch, err := resolveSchema(schemaXML)
	if err != nil {
		tb.Fatalf("parse schema: %v", err)
	}
	return sch
}
