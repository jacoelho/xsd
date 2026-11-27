package xsd_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const defaultPatternAnalysisSteps = 1 << 28

func compileAnalysis(limit int, sources ...xsd.SchemaSource) error {
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaPatternAnalysisSteps: limit}, sources...)
	return err
}

// analysisThreshold returns the smallest MaxSchemaPatternAnalysisSteps under
// which the sources compile.
func analysisThreshold(t *testing.T, sources ...xsd.SchemaSource) int {
	t.Helper()
	return leastCompilingLimit(t, defaultPatternAnalysisSteps, func(limit int) error {
		return compileAnalysis(limit, sources...)
	})
}

func requireAnalysisLimit(t *testing.T, err error, limit int) {
	t.Helper()
	requireDiagnostic(t, err, xsderrors.CodeSchemaLimit, fmt.Sprintf("MaxSchemaPatternAnalysisSteps (limit %d)", limit))
}

func TestPatternAnalysisStepsAreAdditiveInEitherOrder(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, p1, p2 string }{
		{"two kept tables", `[A-Z]{2}[0-9]{2}[a-zA-Z0-9]{1,30}`, `[\p{L}\p{N} ]{1,70}`},
		{"two abandoned tables", `(a|b)*a(a|b){12}`, `(a?){1,2000}`},
		{"a kept and an abandoned table", `[a-c]{2}`, `[a-c]{8190}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s1 := analysisThreshold(t, patternsSource(test.p1))
			s2 := analysisThreshold(t, patternsSource(test.p2))
			if s1 < 2 || s2 < 2 {
				t.Fatalf("thresholds %d and %d: both patterns must spend steps", s1, s2)
			}
			for _, both := range []xsd.SchemaSource{patternsSource(test.p1, test.p2), patternsSource(test.p2, test.p1)} {
				if err := compileAnalysis(s1+s2, both); err != nil {
					t.Fatalf("at S1+S2=%d: %v", s1+s2, err)
				}
				requireAnalysisLimit(t, compileAnalysis(s1+s2-1, both), s1+s2-1)
			}
		})
	}
}

func TestPatternAnalysisStepsChargeIdenticalTextOnce(t *testing.T) {
	t.Parallel()
	const p1 = `(a|b)*a(a|b){12}`
	s1 := analysisThreshold(t, patternsSource(p1))
	same := patternsSource(p1, p1, p1)
	if err := compileAnalysis(s1, same); err != nil {
		t.Fatalf("three types of one pattern at S1=%d: %v", s1, err)
	}
	requireAnalysisLimit(t, compileAnalysis(s1-1, same), s1-1)
}

func TestPatternsWithoutATableAttemptSpendNoAnalysisSteps(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{`abc`, ``, `[a-z]{60000}[a-z]{60000}`, `(a|b){9000}`} {
		if err := compileAnalysis(1, patternsSource(pattern, pattern+"x")); err != nil {
			t.Errorf("%q under MaxSchemaPatternAnalysisSteps=1, the smallest limit: %v", pattern, err)
		}
	}
}

func TestPatternAnalysisBreachIsReportedAtTheFacetStartTag(t *testing.T) {
	t.Parallel()
	const p1, p2 = `(a|b)*a(a|b){12}`, `(a|c)*a(a|c){12}`
	s1 := analysisThreshold(t, patternsSource(p1))
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:element name="first" type="missing"/>
<xs:simpleType name="A"><xs:restriction base="xs:string"><xs:pattern value="` + p1 + `"/></xs:restriction></xs:simpleType>
<xs:simpleType name="B"><xs:restriction base="xs:string"><xs:pattern value="` + p2 + `"/></xs:restriction></xs:simpleType>
</xs:schema>`)
	err := compileAnalysis(s1, xsd.Bytes("p.xsd", schema))
	requireAnalysisLimit(t, err, s1)
	if d, ok := errors.AsType[*xsderrors.Error](err); !ok || d.Line() != 4 || d.Column() != 58 {
		t.Fatalf("error = %v, want the second pattern's start tag at 4:58, before the unresolved type at 2:1", err)
	}
}

// The patterns differ in one character, so each spends the same steps.
func TestPatternAnalysisStepsDefaultStopsTableDefeatingSchemas(t *testing.T) {
	t.Parallel()
	s1 := analysisThreshold(t, patternsSource(`(a?){1,2000}`))
	n := defaultPatternAnalysisSteps / s1
	if n < 2 || n > 4096 {
		t.Fatalf("n = %d patterns fit under the default; adjust the pattern", n)
	}
	patterns := make([]string, n+1)
	for i := range patterns {
		patterns[i] = fmt.Sprintf(`(%c?){1,2000}`, rune(0x100+i))
	}
	_, err := xsd.Compile(patternsSource(patterns...))
	requireAnalysisLimit(t, err, defaultPatternAnalysisSteps)
}

func TestNegativePatternAnalysisStepsIsRejected(t *testing.T) {
	t.Parallel()
	err := compileAnalysis(-1, patternsSource("a"))
	requireDiagnostic(t, err, xsderrors.CodeSchemaLimit, "MaxSchemaPatternAnalysisSteps")
}
