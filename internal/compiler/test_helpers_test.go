package compiler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/semantics"
)

func resolveSchema(schemaXML string) (*parser.Schema, error) {
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	if _, err := semantics.PrepareOwned(sch); err != nil {
		return nil, err
	}
	return sch, nil
}

func buildSchemaForTest(sch *parser.Schema, cfg BuildConfig) (*runtime.Schema, error) {
	if sch == nil {
		return nil, fmt.Errorf("runtime build: schema is nil")
	}
	prepared, err := Prepare(sch)
	if err != nil {
		return nil, fmt.Errorf("runtime build: %w", err)
	}
	return prepared.Build(cfg)
}

func mustResolveSchema(tb testing.TB, schemaXML string) *parser.Schema {
	tb.Helper()
	sch, err := resolveSchema(schemaXML)
	if err != nil {
		tb.Fatalf("parse schema: %v", err)
	}
	return sch
}
