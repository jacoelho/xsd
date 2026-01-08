package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/loader"
	xsdschema "github.com/jacoelho/xsd/internal/schema"
)

// mustCompile compiles an *xsd.Schema to *grammar.CompiledSchema.
// It fails the test if compilation fails.
func mustCompile(t *testing.T, schema *xsdschema.Schema) *grammar.CompiledSchema {
	t.Helper()

	resolver := loader.NewResolver(schema)
	if err := resolver.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	compiler := loader.NewCompiler(schema)
	compiled, err := compiler.Compile()
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	return compiled
}

func validateStream(t *testing.T, v *Validator, docXML string) []errors.Validation {
	t.Helper()

	violations, err := v.ValidateStream(strings.NewReader(docXML))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	return violations
}
