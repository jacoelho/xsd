package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemaast"
)

func buildRuntimeSchema(schemaXML string) (*runtime.Schema, error) {
	result, err := schemaast.ParseDocumentWithImportsOptions(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	docs := &schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*result.Document}}
	prepared, err := compiler.Prepare(docs)
	if err != nil {
		return nil, err
	}
	return prepared.Build(compiler.BuildConfig{})
}

func mustBuildRuntimeSchema(tb testing.TB, schemaXML string) *runtime.Schema {
	tb.Helper()
	rtSchema, err := buildRuntimeSchema(schemaXML)
	if err != nil {
		tb.Fatalf("runtime build: %v", err)
	}
	return rtSchema
}

func validateRuntimeDoc(t *testing.T, schemaXML, docXML string) error {
	t.Helper()

	rtSchema := mustBuildRuntimeSchema(t, schemaXML)
	sess := NewSession(rtSchema)
	return sess.Validate(strings.NewReader(docXML))
}
