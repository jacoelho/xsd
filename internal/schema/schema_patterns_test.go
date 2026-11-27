package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
	"github.com/jacoelho/xsd/xsderrors"
)

// defaultPatternLimits holds the default pattern admission limits, for tests
// that admit patterns without normalized options.
var defaultPatternLimits = Limits{
	MaxSchemaPatternRetainedBytes: defaultMaxSchemaPatternRetainedBytes,
	MaxSchemaPatternAnalysisSteps: defaultMaxSchemaPatternAnalysisSteps,
}

// testSchemaPatterns returns the pattern admission a compiler with limits
// lends to start-tag admission.
func testSchemaPatterns(limits Limits) *schemaPatterns {
	patterns := newSchemaPatterns(limits)
	return &patterns
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

	limits := defaultPatternLimits
	limits.MaxSchemaPatternRetainedBytes = limit
	patterns := testSchemaPatterns(limits)
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

func analysisSteps(t *testing.T, text string) int {
	t.Helper()
	pattern, err := xsdregex.Compile(text)
	if err != nil {
		t.Fatalf("Compile(%q) error = %v", text, err)
	}
	return int(pattern.AnalysisSteps()) //nolint:gosec // A test count far below MaxInt.
}

func TestSchemaPatternsChargeAnalysisStepsOncePerDistinctText(t *testing.T) {
	t.Parallel()

	const built, abandoned, literal = "[a-c]{2}", "(a|b)*a(a|b){12}", "abc"
	builtSteps, abandonedSteps := analysisSteps(t, built), analysisSteps(t, abandoned)
	if builtSteps == 0 || abandonedSteps == 0 || analysisSteps(t, literal) != 0 {
		t.Fatalf("analysis steps = %d built, %d abandoned, %d literal; want two positive counts and zero", builtSteps, abandonedSteps, analysisSteps(t, literal))
	}
	limits := defaultPatternLimits
	limits.MaxSchemaPatternAnalysisSteps = builtSteps + abandonedSteps
	for _, order := range [][]string{{built, abandoned, literal}, {literal, abandoned, built}} {
		patterns := testSchemaPatterns(limits)
		for range 3 {
			for _, text := range order {
				if _, err := patterns.admit(text); err != nil {
					t.Fatalf("admit(%q) in order %q within MaxSchemaPatternAnalysisSteps=%d error = %v", text, order, limits.MaxSchemaPatternAnalysisSteps, err)
				}
			}
		}
	}
}

func TestSchemaPatternsRejectAnAnalysisBreachBeforeKeepingThePattern(t *testing.T) {
	t.Parallel()

	const built, abandoned = "[a-c]{2}", "(a|b)*a(a|b){12}"
	limits := defaultPatternLimits
	limits.MaxSchemaPatternAnalysisSteps = analysisSteps(t, built) + analysisSteps(t, abandoned) - 1
	for _, order := range [][]string{{built, abandoned}, {abandoned, built}} {
		patterns := testSchemaPatterns(limits)
		if _, err := patterns.admit(order[0]); err != nil {
			t.Fatalf("admit(%q) within the limit error = %v", order[0], err)
		}
		for attempt := 1; attempt <= 2; attempt++ {
			_, err := patterns.admit(order[1])
			d, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || d.Code() != xsderrors.CodeSchemaLimit || !strings.Contains(err.Error(), "MaxSchemaPatternAnalysisSteps") {
				t.Fatalf("admission %d of %q beyond MaxSchemaPatternAnalysisSteps error = %v, want schema.limit naming the option", attempt, order[1], err)
			}
		}
	}
}
