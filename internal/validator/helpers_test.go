package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
)

// mustCompile compiles an *xsd.Schema to *grammar.CompiledSchema.
// It fails the test if compilation fails.
func mustCompile(t *testing.T, schema *parser.Schema) *grammar.CompiledSchema {
	t.Helper()

	res := resolver.NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	comp := compiler.NewCompiler(schema)
	compiled, err := comp.Compile()
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	return compiled
}

func mustParseSchema(t *testing.T, schemaXML string) *parser.Schema {
	t.Helper()

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return schema
}

func mustNewValidator(t *testing.T, schemaXML string) *Validator {
	t.Helper()

	return New(mustCompile(t, mustParseSchema(t, schemaXML)))
}

func validateStream(t *testing.T, v *Validator, docXML string) []errors.Validation {
	t.Helper()

	violations, err := v.ValidateStream(strings.NewReader(docXML))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	return violations
}
