package xsd_test

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func patternSchema(pattern string) string {
	return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:simpleType><xs:restriction base="xs:string">
    <xs:pattern value="` + pattern + `"/>
  </xs:restriction></xs:simpleType></xs:element>
</xs:schema>`
}

func compilePatternSchema(t *testing.T, pattern string) *xsd.Engine {
	t.Helper()
	engine, err := xsd.Compile(xsd.Bytes("pattern.xsd", []byte(patternSchema(pattern))))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return engine
}

func requireDiagnostic(t *testing.T, err error, code xsderrors.Code, mention string) {
	t.Helper()
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Code() != code || !strings.Contains(diagnostic.Error(), mention) {
		t.Fatalf("error = %v, want %s mentioning %q", err, code, mention)
	}
}

// Each W is derived by hand from docs/RESOURCE_LIMITS.md (Pattern matching
// cost): input bytes, plus the clause named beside the row.
func TestMaxInstancePatternWorkFollowsDocumentedUnit(t *testing.T) {
	for _, test := range []struct {
		name, pattern, value string
		work                 uint64
	}{
		// 1000 bytes + one linear atom: 4*1001+1.
		{"single linear atom", "[a-z]+", strings.Repeat("a", 1_000), 5_005},
		// 1001 bytes + 2 linear atoms, literal atom included: 2*(4*1002+1).
		{"literal atom plus set", "x[a-z]+", "x" + strings.Repeat("a", 1_000), 9_019},
		// Atom order is irrelevant.
		{"set plus literal atom", "[a-z]+x", strings.Repeat("a", 1_000) + "x", 9_019},
		// Literal pattern: input bytes only.
		{"literal", "abc", "abc", 3},
		// 4 bytes + 2 atoms of 4*5+1, exact-count atoms charged too.
		{"exact count", "a{3}b", "aaab", 46},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := compilePatternSchema(t, test.pattern)
			instance := "<root>" + test.value + "</root>"
			validate := func(limit uint64) error {
				return engine.ValidateWithOptions(strings.NewReader(instance), xsd.ValidateOptions{
					MaxInstancePatternWork: limit,
					// Far above the lexical charge, so only matching can fire.
					MaxInstanceValueWork: 1 << 40,
				})
			}
			if err := validate(test.work); err != nil {
				t.Fatalf("MaxInstancePatternWork=%d: %v", test.work, err)
			}
			err := validate(test.work - 1)
			requireDiagnostic(t, err, xsderrors.CodeValidationLimit, "MaxInstancePatternWork="+strconv.FormatUint(test.work-1, 10))
		})
	}
}

func TestLongLinearValuePassesUnderDefaults(t *testing.T) {
	engine := compilePatternSchema(t, "[a-z]+")
	for _, n := range []int{100_001, 1_000_000} {
		if err := engine.Validate(strings.NewReader("<root>" + strings.Repeat("a", n) + "</root>")); err != nil {
			t.Fatalf("%d runes: %v", n, err)
		}
	}
}

// A many-state NFA on a maximum-size text value is stopped by the per-value
// budget, not by input length. The limit sits above the input-byte charge so
// only NFA work can exhaust it. Work, not wall time, is the contract: the
// default budget's CPU cost is a measurement in docs/RESOURCE_LIMITS.md, and
// asserting it here would fail on slow or race-instrumented runners.
func TestMaxTextBytesAgainstManyStatePatternIsBoundedByPatternWork(t *testing.T) {
	engine := compilePatternSchema(t, "(a|aa){1,300}(a{1,100}){1,300}b")
	value := strings.Repeat("a", 4<<20-64)
	limit := uint64(len(value)) + 1<<20
	err := engine.ValidateWithOptions(strings.NewReader("<root>"+value+"</root>"), xsd.ValidateOptions{
		MaxInstancePatternWork: limit,
	})
	requireDiagnostic(t, err, xsderrors.CodeValidationLimit, "MaxInstancePatternWork="+strconv.FormatUint(limit, 10))
}

func TestPatternCompileBounds(t *testing.T) {
	compile := func(pattern string) error {
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTokenBytes: 4 << 20}, xsd.Bytes("pattern.xsd", []byte(patternSchema(pattern))))
		return err
	}
	// A character class keeps the node count small so the byte cap is the
	// only bound under test: exactly 1 MiB compiles, one byte more is a limit.
	if err := compile("[" + strings.Repeat("a", 1<<20-2) + "]"); err != nil {
		t.Fatalf("pattern of exactly 1 MiB: %v", err)
	}
	requireDiagnostic(t, compile("["+strings.Repeat("a", 1<<20-1)+"]"), xsderrors.CodeSchemaLimit, "byte limit")
	// 100,001 literal nodes exceed the node cap.
	requireDiagnostic(t, compile(strings.Repeat("a", 100_001)), xsderrors.CodeSchemaLimit, "pattern")
	// Counted repeats expand into NFA states.
	requireDiagnostic(t, compile("((a{100}){100}){100}"), xsderrors.CodeSchemaLimit, "state count")
	requireDiagnostic(t, compile("("), xsderrors.CodeSchemaFacet, "invalid pattern")
	// The retained character-range bound counts distinct range arrays, so a
	// shared category set is cheap while each negated class owns its own.
	requireDiagnostic(t, compile(strings.Repeat(`[^\p{L}]`, 5000)), xsderrors.CodeSchemaLimit, "character-range")
	if err := compile(strings.Repeat(`[^\p{L}]`, 10)); err != nil {
		t.Fatalf("10 negated classes: %v", err)
	}
	if err := compile(strings.Repeat(`\p{L}`, 5000)); err != nil {
		t.Fatalf("5000 shared category sets: %v", err)
	}
	nested := func(depth int) string { return strings.Repeat("(", depth) + "a" + strings.Repeat(")", depth) }
	requireDiagnostic(t, compile(nested(1025)), xsderrors.CodeSchemaLimit, "nesting")
	if err := compile(nested(1024)); err != nil {
		t.Fatalf("1024 nested groups: %v", err)
	}
	// The parse-node, state-count and byte bounds are reached above; the
	// retained-range bound against a tighter value is covered at xsdregex level
	// (TestCompileRetainedRangeLimit) because the fixed bounds are not options.
}

func TestPatternLiteralIsNotCappedByStateCount(t *testing.T) {
	long := strings.Repeat("a", 150_000)
	schema := func(attr string) []byte {
		return []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="xs:string"><xs:pattern value="[a-z]+"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="T" ` + attr + `/>
</xs:schema>`)
	}
	for _, attr := range []string{`fixed="` + long + `"`, `default="` + long + `"`} {
		if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTokenBytes: 1 << 20}, xsd.Bytes("literal.xsd", schema(attr))); err != nil {
			t.Fatalf("%.12s: %v", attr, err)
		}
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTokenBytes: 1 << 20, MaxSchemaPatternWork: 1_000}, xsd.Bytes("literal.xsd", schema(attr)))
		requireDiagnostic(t, err, xsderrors.CodeSchemaLimit, "MaxSchemaPatternWork")
	}
}

func TestPatternWorkOptionsAcceptFullUnsignedRange(t *testing.T) {
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{
		MaxSchemaPatternWork:      math.MaxUint64,
		MaxSchemaTotalPatternWork: math.MaxUint64,
	}, xsd.Bytes("pattern.xsd", []byte(patternSchema("[a-z]+"))))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

// MaxSchemaPatternWork is a per-literal budget. Each enumeration item below is
// 1,000 ASCII runes matched against [a-z]+, which the documented unit
// definition prices at 5*1000+5 = 5,005 units, so a budget of exactly one
// literal's cost must admit any number of such literals on one type.
func TestMaxSchemaPatternWorkIsPerLiteral(t *testing.T) {
	var enumerations strings.Builder
	for _, r := range "abcde" {
		enumerations.WriteString(`<xs:enumeration value="` + strings.Repeat(string(r), 1_000) + `"/>`)
	}
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Item"><xs:restriction base="xs:string"><xs:pattern value="[a-z]+"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Items"><xs:list itemType="Item"/></xs:simpleType>
  <xs:simpleType name="T"><xs:restriction base="Items">` + enumerations.String() + `</xs:restriction></xs:simpleType>
  <xs:element name="root" type="T"/>
</xs:schema>`)
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaPatternWork: 5_005}, xsd.Bytes("enum.xsd", schema)); err != nil {
		t.Fatalf("budget of one literal: %v", err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaPatternWork: 5_004}, xsd.Bytes("enum.xsd", schema))
	requireDiagnostic(t, err, xsderrors.CodeSchemaLimit, "MaxSchemaPatternWork")
	// The per-literal pair is unchanged under a large aggregate.
	const large = 1 << 40
	if _, err = xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaPatternWork: 5_005, MaxSchemaTotalPatternWork: large}, xsd.Bytes("enum.xsd", schema)); err != nil {
		t.Fatalf("one literal per value under a large total: %v", err)
	}
	_, err = xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaPatternWork: 5_004, MaxSchemaTotalPatternWork: large}, xsd.Bytes("enum.xsd", schema))
	requireDiagnostic(t, err, xsderrors.CodeSchemaLimit, "MaxSchemaPatternWork=5004")
}

// Every compile-time evaluation is charged to the aggregate. Each of the five
// enumeration literals costs T1 = 5,005 per evaluation and is evaluated at
// least once, so 5*T1-1 fails. A facet literal is charged by its base-type
// check and again by facet compilation, so 2*5*T1 is the documented ceiling.
func TestMaxSchemaTotalPatternWorkSumsLiterals(t *testing.T) {
	var enumerations strings.Builder
	for _, r := range "abcde" {
		enumerations.WriteString(`<xs:enumeration value="` + strings.Repeat(string(r), 1_000) + `"/>`)
	}
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Item"><xs:restriction base="xs:string"><xs:pattern value="[a-z]+"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Items"><xs:list itemType="Item"/></xs:simpleType>
  <xs:simpleType name="T"><xs:restriction base="Items">` + enumerations.String() + `</xs:restriction></xs:simpleType>
  <xs:element name="root" type="T"/>
</xs:schema>`)
	const t1, evaluations = 5_005, 5
	compile := func(total uint64) error {
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaPatternWork: t1, MaxSchemaTotalPatternWork: total}, xsd.Bytes("enum.xsd", schema))
		return err
	}
	if err := compile(2 * t1 * evaluations); err != nil {
		t.Fatalf("total of 2*%d*T1: %v", evaluations, err)
	}
	requireDiagnostic(t, compile(t1*evaluations-1), xsderrors.CodeSchemaLimit, "MaxSchemaTotalPatternWork="+strconv.Itoa(t1*evaluations-1))
}

// Default and fixed value constraints are evaluated once, and publication
// never re-evaluates them. Each literal is 1,000
// ASCII runes matched against [a-z]+, priced at 5*1000+5 = 5,005 units.
func TestMaxSchemaTotalPatternWorkChargesValueConstraintsOnce(t *testing.T) {
	const cost = 5_005
	literal := strings.Repeat("a", 1_000)
	tests := []struct {
		name     string
		literals uint64
		decls    string
	}{
		{name: "element default", literals: 1, decls: `<xs:element name="root" type="T" default="` + literal + `"/>`},
		{name: "attribute fixed", literals: 1, decls: `<xs:element name="root"><xs:complexType><xs:attribute name="a" type="T" fixed="` + literal + `"/></xs:complexType></xs:element>`},
		{name: "element default and attribute fixed", literals: 2, decls: `<xs:element name="e" type="T" default="` + literal + `"/>` +
			`<xs:element name="root"><xs:complexType><xs:attribute name="a" type="T" fixed="` + literal + `"/></xs:complexType></xs:element>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="xs:string"><xs:pattern value="[a-z]+"/></xs:restriction></xs:simpleType>
  ` + tt.decls + `
</xs:schema>`)
			compile := func(total uint64) error {
				_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalPatternWork: total}, xsd.Bytes("vc.xsd", schema))
				return err
			}
			w := tt.literals * cost
			if err := compile(w); err != nil {
				t.Fatalf("total W=%d: %v", w, err)
			}
			requireDiagnostic(t, compile(w-1), xsderrors.CodeSchemaLimit, "MaxSchemaTotalPatternWork="+strconv.FormatUint(w-1, 10))
		})
	}
}

// k elements of cost C = 9,019 (x[a-z]+ on "x" and 1,000 "a") are charged to
// one document total.
func TestMaxInstanceTotalPatternWorkSumsValues(t *testing.T) {
	const cost, k = 9_019, 4
	engine := compileRepeatedPatternSchema(t, "x[a-z]+")
	document := repeatedValues(k, "x"+strings.Repeat("a", 1_000))
	validate := func(total uint64) error {
		return engine.ValidateWithOptions(strings.NewReader(document), xsd.ValidateOptions{MaxInstanceTotalPatternWork: total, MaxInstanceValueWork: 1 << 40})
	}
	if err := validate(k * cost); err != nil {
		t.Fatalf("total k*C: %v", err)
	}
	err := validate(k*cost - 1)
	requireDiagnostic(t, err, xsderrors.CodeValidationLimit, "MaxInstanceTotalPatternWork="+strconv.Itoa(k*cost-1))
	if strings.Contains(err.Error(), "MaxInstancePatternWork=") {
		t.Fatalf("error names the per-value option: %v", err)
	}
}

func TestMaxInstanceTotalPatternWorkCountsLiteralPatterns(t *testing.T) {
	const cost, k = 3, 4
	engine := compileRepeatedPatternSchema(t, "abc")
	document := repeatedValues(k, "abc")
	validate := func(total uint64) error {
		return engine.ValidateWithOptions(strings.NewReader(document), xsd.ValidateOptions{MaxInstanceTotalPatternWork: total})
	}
	if err := validate(k * cost); err != nil {
		t.Fatalf("total k*C: %v", err)
	}
	requireDiagnostic(t, validate(k*cost-1), xsderrors.CodeValidationLimit, "MaxInstanceTotalPatternWork=")
}

func TestManyValuesWithinPerValueLimitPassUnderLargeTotal(t *testing.T) {
	const cost = 9_019
	engine := compileRepeatedPatternSchema(t, "x[a-z]+")
	document := repeatedValues(50, "x"+strings.Repeat("a", 1_000))
	err := engine.ValidateWithOptions(strings.NewReader(document), xsd.ValidateOptions{MaxInstancePatternWork: cost, MaxInstanceTotalPatternWork: 1 << 40, MaxInstanceValueWork: 1 << 40})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestMaxInstanceTotalPatternWorkSessionReuse(t *testing.T) {
	const cost, k = 9_019, 4
	engine := compileRepeatedPatternSchema(t, "x[a-z]+")
	value := "x" + strings.Repeat("a", 1_000)
	full, one := repeatedValues(k, value), repeatedValues(1, value)
	options := func(total uint64) xsd.ValidateOptions {
		return xsd.ValidateOptions{MaxInstanceTotalPatternWork: total, MaxInstanceValueWork: 1 << 40}
	}
	session, err := engine.NewSession(options(k*cost - 1))
	if err != nil {
		t.Fatal(err)
	}
	// (a) exhausts the total; (b) a small document then succeeds.
	requireDiagnostic(t, session.Validate(strings.NewReader(full)), xsderrors.CodeValidationLimit, "MaxInstanceTotalPatternWork=")
	if err = session.Validate(strings.NewReader(one)); err != nil {
		t.Fatalf("small document after exhaustion: %v", err)
	}
	// (c) per-call totals decide the first document independently of history.
	if err = engine.ValidateWithOptions(strings.NewReader(full), options(k*cost)); err != nil {
		t.Fatalf("larger per-call total: %v", err)
	}
	requireDiagnostic(t, engine.ValidateWithOptions(strings.NewReader(full), options(k*cost-1)), xsderrors.CodeValidationLimit, "MaxInstanceTotalPatternWork=")
	// (d) a document that consumes most of the total and then fails
	// mid-stream must not leak its usage into the next document.
	reuse, err := engine.NewSession(options(k * cost))
	if err != nil {
		t.Fatal(err)
	}
	truncated := strings.TrimSuffix(repeatedValues(k-1, value), "</root>") + "</wrong>"
	requireDiagnostic(t, reuse.Validate(strings.NewReader(truncated)), xsderrors.CodeValidationXML, "")
	if err = reuse.Validate(strings.NewReader(full)); err != nil {
		t.Fatalf("full document after mid-stream failure: %v", err)
	}
}

func TestMaxInstanceTotalPatternWorkIsPerDocumentAcrossGoroutines(t *testing.T) {
	t.Parallel()
	const cost, k = 9_019, 4
	engine := compileRepeatedPatternSchema(t, "x[a-z]+")
	document := repeatedValues(k, "x"+strings.Repeat("a", 1_000))
	for i := range 8 {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			err := engine.ValidateWithOptions(strings.NewReader(document), xsd.ValidateOptions{MaxInstanceTotalPatternWork: k * cost, MaxInstanceValueWork: 1 << 40})
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func compileRepeatedPatternSchema(t *testing.T, pattern string) *xsd.Engine {
	t.Helper()
	engine, err := xsd.Compile(xsd.Bytes("repeated.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="xs:string"><xs:pattern value="`+pattern+`"/></xs:restriction></xs:simpleType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="v" type="T" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return engine
}

func repeatedValues(n int, value string) string {
	return "<root>" + strings.Repeat("<v>"+value+"</v>", n) + "</root>"
}

// x[a-z]+ on "x"+1000 "a": 1001 bytes + 2 atoms of 4*1002+1 = 9019.
func TestMaxSchemaPatternWorkFollowsDocumentedUnit(t *testing.T) {
	var enumerations strings.Builder
	for _, r := range "abcde" {
		enumerations.WriteString(`<xs:enumeration value="x` + strings.Repeat(string(r), 1_000) + `"/>`)
	}
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Item"><xs:restriction base="xs:string"><xs:pattern value="x[a-z]+"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="T"><xs:restriction base="Item">` + enumerations.String() + `</xs:restriction></xs:simpleType>
  <xs:element name="root" type="T"/>
</xs:schema>`)
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaPatternWork: 9_019}, xsd.Bytes("enum.xsd", schema)); err != nil {
		t.Fatalf("budget of one literal: %v", err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaPatternWork: 9_018}, xsd.Bytes("enum.xsd", schema))
	requireDiagnostic(t, err, xsderrors.CodeSchemaLimit, "MaxSchemaPatternWork")
}

func TestPatternWorkLimitIndependentOfSessionHistory(t *testing.T) {
	engine := compilePatternSchema(t, "(a|ab){1,60}x")
	instance := "<root>" + strings.Repeat("a", 30) + "x</root>"
	newSession := func(limit uint64) *xsd.Session {
		s, err := engine.NewSession(xsd.ValidateOptions{MaxInstancePatternWork: limit, MaxInstanceValueWork: 1 << 40})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	validate := func(s *xsd.Session) error { return s.Validate(strings.NewReader(instance)) }
	// minimal returns the smallest passing limit; with warm, the probe is the
	// second run on a session that already ran the document once.
	minimal := func(warm bool) uint64 {
		lo, hi := uint64(1), uint64(1<<30)
		for lo < hi {
			mid := lo + (hi-lo)/2
			s := newSession(mid)
			if warm {
				_ = validate(s) //nolint:errcheck // Only warms the session.
			}
			if validate(s) == nil {
				hi = mid
			} else {
				lo = mid + 1
			}
		}
		return lo
	}
	cold, warm := minimal(false), minimal(true)
	if cold != warm {
		t.Fatalf("minimal MaxInstancePatternWork: cold %d, warm %d", cold, warm)
	}
	below := newSession(cold - 1)
	at := newSession(cold)
	for range 2 {
		requireDiagnostic(t, validate(below), xsderrors.CodeValidationLimit, "MaxInstancePatternWork=")
		if err := validate(at); err != nil {
			t.Fatalf("at limit %d: %v", cold, err)
		}
	}
}

func compileTotal(t *testing.T, total uint64, schema string) error {
	t.Helper()
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTotalPatternWork: total}, xsd.Bytes("total.xsd", []byte(schema)))
	return err
}

// This test pins the N-independence of charged work. It does not by itself
// prove that publication replay was removed; that is the deletion of
// Builder.ValidateUncharged.
func TestMaxSchemaTotalPatternWorkSharedFixedAcrossUseSets(t *testing.T) {
	const cost = 5_005
	for _, n := range []int{1, 2000} {
		var types strings.Builder
		for i := range n {
			types.WriteString(`<xs:element name="e` + strconv.Itoa(i) + `"><xs:complexType><xs:attribute name="b` + strconv.Itoa(i) + `"/><xs:attributeGroup ref="g"/></xs:complexType></xs:element>`)
		}
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="xs:string"><xs:pattern value="[a-z]+"/></xs:restriction></xs:simpleType>
  <xs:attribute name="a" type="T" fixed="` + strings.Repeat("a", 1_000) + `"/>
  <xs:attributeGroup name="g"><xs:attribute ref="a"/></xs:attributeGroup>` + types.String() + `
</xs:schema>`
		if err := compileTotal(t, cost, schema); err != nil {
			t.Fatalf("N=%d total %d: %v", n, cost, err)
		}
		requireDiagnostic(t, compileTotal(t, cost-1, schema), xsderrors.CodeSchemaLimit, "MaxSchemaTotalPatternWork=5004")
	}
}

// The facet-literal base-type check is charged, not exempt.
func TestMaxSchemaTotalPatternWorkChargesFacetLiteralBaseCheck(t *testing.T) {
	const cost = 5_005
	literal := strings.Repeat("a", 1_000)
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="xs:string"><xs:pattern value="[a-z]+"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="U"><xs:restriction base="T"><xs:enumeration value="` + literal + `"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="U"/>
</xs:schema>`
	if err := compileTotal(t, cost, schema); err != nil {
		t.Fatalf("total %d: %v", cost, err)
	}
	requireDiagnostic(t, compileTotal(t, cost-1, schema), xsderrors.CodeSchemaLimit, "MaxSchemaTotalPatternWork=5004")

	// No pattern anywhere: nothing to charge.
	noPattern := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U"><xs:restriction base="xs:string"><xs:enumeration value="` + literal + `"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="U"/>
</xs:schema>`
	if err := compileTotal(t, 1, noPattern); err != nil {
		t.Fatalf("pattern-free enumeration under total 1: %v", err)
	}

	// Bound facet path.
	bound := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="D"><xs:restriction base="xs:integer"><xs:pattern value="[0-9]+"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="E"><xs:restriction base="D"><xs:minInclusive value="` + strings.Repeat("1", 1_000) + `"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="E"/>
</xs:schema>`
	const c = 5_005 // one evaluation of the 1,000-digit literal, measured
	if err := compileTotal(t, c, bound); err != nil {
		t.Fatalf("total C=%d: %v", c, err)
	}
	requireDiagnostic(t, compileTotal(t, c-1, bound), xsderrors.CodeSchemaLimit, "MaxSchemaTotalPatternWork="+strconv.Itoa(c-1))
}
