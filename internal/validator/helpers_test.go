package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/grammar"
	xsdschema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
)

// mustCompile compiles an *xsd.Schema to *grammar.CompiledSchema.
// It fails the test if compilation fails.
func mustCompile(t *testing.T, schema *xsdschema.Schema) *grammar.CompiledSchema {
	t.Helper()

	resolver := resolver.NewResolver(schema)
	if err := resolver.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	compiler := compiler.NewCompiler(schema)
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
