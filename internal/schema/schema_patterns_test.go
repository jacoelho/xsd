package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

// defaultPatternLimits holds the default MaxSchemaPatternRetainedBytes, for
// tests that admit patterns without normalized options.
var defaultPatternLimits = Limits{MaxSchemaPatternRetainedBytes: defaultMaxSchemaPatternRetainedBytes}

// testSchemaPatterns returns the pattern admission a compiler with limits
// lends to start-tag admission.
func testSchemaPatterns(limits Limits) *schemaPatterns {
	patterns := newSchemaPatterns(limits)
	return &patterns
}

// testSchemaAdmission returns what a compiler with limits lends to start-tag
// admission: an empty pattern admission and an empty runtime name table.
func testSchemaAdmission(limits Limits) schemaSetAdmission {
	names, err := newRuntimeNameTable(limits.MaxSchemaNames)
	if err != nil {
		panic(err)
	}
	return schemaSetAdmission{patterns: testSchemaPatterns(limits), names: &names}
}

func TestAdmitFacetValueStoresTheCompiledPattern(t *testing.T) {
	t.Parallel()

	const text = "[a-c]{2}"
	got, err := admitFacetValue("pattern", text, testSchemaPatterns(defaultPatternLimits))
	if err != nil {
		t.Fatalf("admitFacetValue(pattern, %q) error = %v", text, err)
	}
	if got.pattern == nil || got.pattern.Source() != text {
		t.Fatalf("admitFacetValue(pattern, %q) pattern = %v, want the compiled %q", text, got.pattern, text)
	}
}

func TestSchemaPatternsCompileIdenticalTextOnce(t *testing.T) {
	t.Parallel()

	patterns := testSchemaPatterns(defaultPatternLimits)
	first, err := patterns.admit("(a|b){20}")
	if err != nil {
		t.Fatal(err)
	}
	for range 100 {
		again, err := patterns.admit("(a|b){20}")
		if err != nil {
			t.Fatal(err)
		}
		if again != first {
			t.Fatal("identical pattern text compiled to a second *xsdregex.Pattern")
		}
	}
}

func TestSchemaPatternsRejectARetainedBreachBeforeKeepingThePattern(t *testing.T) {
	t.Parallel()

	sizing := testSchemaPatterns(defaultPatternLimits)
	one, err := sizing.admit("x+")
	if err != nil {
		t.Fatal(err)
	}
	limit := int64(one.RetainedBytes()) //nolint:gosec // A test estimate far below MaxInt64.

	patterns := testSchemaPatterns(Limits{MaxSchemaPatternRetainedBytes: limit})
	if _, withinErr := patterns.admit("x+"); withinErr != nil {
		t.Fatalf("admit within the limit error = %v", withinErr)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		_, err = patterns.admit("y+")
		d, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || d.Code() != xsderrors.CodeSchemaLimit {
			t.Fatalf("admission %d of a pattern beyond MaxSchemaPatternRetainedBytes error = %v, want schema.limit", attempt, err)
		}
	}
}
