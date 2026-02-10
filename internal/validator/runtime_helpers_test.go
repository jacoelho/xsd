package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/pipeline"
	"github.com/jacoelho/xsd/internal/runtime"
)

func buildRuntimeSchema(schemaXML string) (*runtime.Schema, error) {
	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	prepared, err := pipeline.Prepare(parsed)
	if err != nil {
		return nil, err
	}
	return prepared.BuildRuntime(pipeline.CompileConfig{})
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
