package runtimebuild

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/schemacheck"
)

func resolveSchema(schemaXML string) (*parser.Schema, error) {
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	if errs := schemacheck.ValidateStructure(sch); len(errs) != 0 {
		return nil, errs[0]
	}
	if err := schema.MarkSemantic(sch); err != nil {
		return nil, err
	}
	if err := resolver.ResolveTypeReferences(sch); err != nil {
		return nil, err
	}
	if errs := resolver.ValidateReferences(sch); len(errs) != 0 {
		return nil, errs[0]
	}
	parser.UpdatePlaceholderState(sch)
	if err := schema.MarkResolved(sch); err != nil {
		return nil, err
	}
	return sch, nil
}

func mustResolveSchema(tb testing.TB, schemaXML string) *parser.Schema {
	tb.Helper()
	sch, err := resolveSchema(schemaXML)
	if err != nil {
		tb.Fatalf("parse schema: %v", err)
	}
	return sch
}
