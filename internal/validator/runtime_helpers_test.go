package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	runtimebuild "github.com/jacoelho/xsd/internal/runtimecompile"
	schema "github.com/jacoelho/xsd/internal/semantic"
	schemacheck "github.com/jacoelho/xsd/internal/semanticcheck"
	resolver "github.com/jacoelho/xsd/internal/semanticresolve"
)

func buildRuntimeSchema(schemaXML string) (*runtime.Schema, error) {
	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	if errs := schemacheck.ValidateStructure(parsed); len(errs) != 0 {
		return nil, errs[0]
	}
	if err := schema.MarkSemantic(parsed); err != nil {
		return nil, err
	}
	if err := resolver.ResolveTypeReferences(parsed); err != nil {
		return nil, err
	}
	if errs := resolver.ValidateReferences(parsed); len(errs) != 0 {
		return nil, errs[0]
	}
	parser.UpdatePlaceholderState(parsed)
	if err := schema.MarkResolved(parsed); err != nil {
		return nil, err
	}
	return runtimebuild.BuildSchema(parsed, runtimebuild.BuildConfig{})
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
