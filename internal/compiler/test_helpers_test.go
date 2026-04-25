package compiler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemaast"
)

func parseDocumentSet(schemaXML string) (*schemaast.DocumentSet, error) {
	result, err := schemaast.ParseDocumentWithImportsOptions(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	return &schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*result.Document}}, nil
}

func resolveSchema(schemaXML string) (*schemaast.DocumentSet, error) {
	docs, err := parseDocumentSet(schemaXML)
	if err != nil {
		return nil, err
	}
	if _, err := Prepare(docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func buildSchemaForTest(docs *schemaast.DocumentSet, cfg BuildConfig) (*runtime.Schema, error) {
	if docs == nil {
		return nil, fmt.Errorf("runtime build: document set is nil")
	}
	prepared, err := Prepare(docs)
	if err != nil {
		return nil, fmt.Errorf("runtime build: %w", err)
	}
	return prepared.Build(cfg)
}

func mustResolveSchema(tb testing.TB, schemaXML string) *schemaast.DocumentSet {
	tb.Helper()
	docs, err := resolveSchema(schemaXML)
	if err != nil {
		tb.Fatalf("parse schema: %v", err)
	}
	return docs
}
