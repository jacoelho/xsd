package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimebuild"
)

func mustBuildRuntimeSchema(t *testing.T, schemaXML string) *runtime.Schema {
	t.Helper()

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	schema, err := runtimebuild.BuildSchema(parsed, runtimebuild.BuildConfig{})
	if err != nil {
		t.Fatalf("runtime build: %v", err)
	}
	return schema
}

func validateRuntimeDoc(t *testing.T, schemaXML, docXML string) error {
	t.Helper()

	schema := mustBuildRuntimeSchema(t, schemaXML)
	sess := NewSession(schema)
	return sess.Validate(strings.NewReader(docXML), nil)
}
