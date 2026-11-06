package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const defaultPatternRetainedBytes = 64 << 20

// patternsSchema declares one simple type per pattern text, so a text repeated
// in the argument list is repeated in the schema.
func patternsSchema(patterns ...string) []byte {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for i, p := range patterns {
		fmt.Fprintf(&b, `<xs:simpleType name="T%d"><xs:restriction base="xs:string"><xs:pattern value="%s"/></xs:restriction></xs:simpleType>`, i, p)
	}
	b.WriteString(`</xs:schema>`)
	return []byte(b.String())
}

func compileRetained(limit int64, sources ...xsd.SchemaSource) error {
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaPatternRetainedBytes: limit}, sources...)
	return err
}

// retainedThreshold returns the smallest MaxSchemaPatternRetainedBytes under
// which the sources compile, found by binary search so tests state contracts
// (additivity, dedupe) without pinning the byte estimate.
func retainedThreshold(t *testing.T, sources ...xsd.SchemaSource) int64 {
	t.Helper()
	hi := int64(defaultPatternRetainedBytes)
	if err := compileRetained(hi, sources...); err != nil {
		t.Fatalf("compile at the default limit: %v", err)
	}
	lo := int64(1)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if compileRetained(mid, sources...) == nil {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

func requireRetainedLimit(t *testing.T, err error, limit int64) {
	t.Helper()
	requireDiagnostic(t, err, xsderrors.CodeSchemaLimit, fmt.Sprintf("MaxSchemaPatternRetainedBytes=%d exceeded", limit))
}

func TestPatternRetainedBytesAreAdditive(t *testing.T) {
	ranges := func(tail string) string { return strings.Repeat(`[^\p{L}]`, 200) + tail }
	for _, test := range []struct{ name, p1, p2 string }{
		{"states", "(ab|c){1,200}", "(ab|d){1,200}"},
		{"ranges", ranges("a"), ranges("b")},
		// 15 repeats stay within 64 NFA states, so closures are precomputed.
		{"closures", "(a?){1,15}b", "(a?){1,15}c"},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := func(patterns ...string) xsd.SchemaSource {
				return xsd.Bytes("p.xsd", patternsSchema(patterns...))
			}
			s1 := retainedThreshold(t, src(test.p1))
			s2 := retainedThreshold(t, src(test.p2))
			both := src(test.p1, test.p2)
			if err := compileRetained(s1+s2, both); err != nil {
				t.Fatalf("at S1+S2=%d: %v", s1+s2, err)
			}
			requireRetainedLimit(t, compileRetained(s1+s2-1, both), s1+s2-1)
		})
	}
}

func TestPatternRetainedBytesChargeIdenticalTextOnce(t *testing.T) {
	const p1, p2 = "(ab|c){1,200}", "(ab|d){1,200}"
	src := func(patterns ...string) xsd.SchemaSource { return xsd.Bytes("p.xsd", patternsSchema(patterns...)) }
	s1 := retainedThreshold(t, src(p1))
	s2 := retainedThreshold(t, src(p2))

	t.Run("two types", func(t *testing.T) {
		same := src(p1, p1)
		if err := compileRetained(s1, same); err != nil {
			t.Fatalf("at S1=%d: %v", s1, err)
		}
		requireRetainedLimit(t, compileRetained(s1-1, same), s1-1)
	})
	t.Run("chameleon include", func(t *testing.T) {
		included := []byte(string(patternsSchema(p1)))
		root := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t">
<xs:include schemaLocation="inc.xsd"/>
<xs:simpleType name="Own"><xs:restriction base="xs:string"><xs:pattern value="` + p1 + `"/></xs:restriction></xs:simpleType>
</xs:schema>`)
		source := xsd.Bytes("root.xsd", root).WithResolver(xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
			return xsd.Bytes(location, included), nil
		}))
		if err := compileRetained(s1, source); err != nil {
			t.Fatalf("at S1=%d: %v", s1, err)
		}
		requireRetainedLimit(t, compileRetained(s1-1, source), s1-1)
	})
	t.Run("distinct text", func(t *testing.T) {
		distinct := src(p1, p2)
		if err := compileRetained(s1+s2, distinct); err != nil {
			t.Fatalf("at S1+S2: %v", err)
		}
		requireRetainedLimit(t, compileRetained(s1+s2-1, distinct), s1+s2-1)
	})
}

func TestInheritedPatternsAreNotRecharged(t *testing.T) {
	const p1, p2 = "(ab|c){1,200}", "(ab|d){1,200}"
	schema := func(extra string) xsd.SchemaSource {
		var b strings.Builder
		b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
		fmt.Fprintf(&b, `<xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:pattern value="%s"/></xs:restriction></xs:simpleType>`, p1)
		for i := range 50 {
			fmt.Fprintf(&b, `<xs:simpleType name="D%d"><xs:restriction base="Base"><xs:maxLength value="%d"/></xs:restriction></xs:simpleType>`, i, 100+i)
		}
		if extra != "" {
			fmt.Fprintf(&b, `<xs:simpleType name="Extra"><xs:restriction base="Base"><xs:pattern value="%s"/></xs:restriction></xs:simpleType>`, extra)
		}
		b.WriteString(`</xs:schema>`)
		return xsd.Bytes("p.xsd", []byte(b.String()))
	}
	s1 := retainedThreshold(t, xsd.Bytes("p.xsd", patternsSchema(p1)))
	s2 := retainedThreshold(t, xsd.Bytes("p.xsd", patternsSchema(p2)))
	if got := retainedThreshold(t, schema("")); got != s1 {
		t.Fatalf("50 derived types need %d, want S1=%d", got, s1)
	}
	if got := retainedThreshold(t, schema(p2)); got != s1+s2 {
		t.Fatalf("one extra distinct pattern needs %d, want S1+S2=%d", got, s1+s2)
	}
}

// Each pattern uses a two-byte alternative so every pattern has the same
// source length and therefore the same estimate.
func distinctPattern(i int) string {
	return fmt.Sprintf("(ab|%c){1,2000}", rune(0x100+i)) //nolint:gosec // i is small and non-negative.
}

func TestPatternRetainedBytesDefaultRejects(t *testing.T) {
	src := func(from, to int) xsd.SchemaSource {
		patterns := make([]string, 0, to-from)
		for i := from; i < to; i++ {
			patterns = append(patterns, distinctPattern(i))
		}
		return xsd.Bytes("p.xsd", patternsSchema(patterns...))
	}
	s1 := retainedThreshold(t, src(0, 1))
	n := int(defaultPatternRetainedBytes / s1)
	if n < 2 || n > 1000 {
		t.Fatalf("n = %d patterns fit under the default; adjust the pattern size", n)
	}
	if _, err := xsd.Compile(src(0, n-1)); err != nil {
		t.Fatalf("%d patterns: %v", n-1, err)
	}
	_, err := xsd.Compile(src(0, n+1))
	requireRetainedLimit(t, err, defaultPatternRetainedBytes)
}

func TestPatternRetainedBytesDefaultAdmitsUnicodeCategorySchemas(t *testing.T) {
	patterns := make([]string, 3000)
	for i := range patterns {
		patterns[i] = fmt.Sprintf(`[\p{L}\p{N} ]{1,%d}`, i+1)
	}
	if _, err := xsd.Compile(xsd.Bytes("p.xsd", patternsSchema(patterns...))); err != nil {
		t.Fatalf("3000 Unicode-category patterns under default options: %v", err)
	}
}

func TestNegativePatternRetainedBytesIsRejected(t *testing.T) {
	err := compileRetained(-1, xsd.Bytes("p.xsd", patternsSchema("a")))
	requireDiagnostic(t, err, xsderrors.CodeSchemaLimit, "MaxSchemaPatternRetainedBytes")
}
