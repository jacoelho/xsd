package runtime

import (
	"errors"
	"testing"
)

func TestValidateRawStringLengthUsesNormalizedCodePointCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      []byte
		whitespace WhitespaceMode
		length     uint32
	}{
		{name: "preserve multibyte", input: []byte("éa"), whitespace: WhitespacePreserve, length: 2},
		{name: "replace", input: []byte("a\tb"), whitespace: WhitespaceReplace, length: 3},
		{name: "collapse", input: []byte(" \té  a\n"), whitespace: WhitespaceCollapse, length: 3},
		{name: "collapse empty", input: []byte(" \t\n"), whitespace: WhitespaceCollapse, length: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facets := LengthFacetValues{Length: FacetCardinalityValue{Value: tc.length, Present: true}}
			if err := validateRawStringLength(tc.input, tc.whitespace, facets); err != nil {
				t.Fatalf("validateRawStringLength() error = %v", err)
			}
		})
	}

	if err := validateRawStringLength([]byte{0xff}, WhitespacePreserve, LengthFacetValues{}); err == nil {
		t.Fatal("validateRawStringLength() accepted invalid UTF-8")
	}
	if err := validateRawStringLength([]byte("ab"), WhitespacePreserve, LengthFacetValues{
		MinLength: FacetCardinalityValue{Value: 3, Present: true},
	}); err == nil || err.Error() != "minLength facet failed" {
		t.Fatalf("validateRawStringLength() error = %v", err)
	}
}

type rawSimpleValueFixture struct {
	routes       map[SimpleTypeID]simpleValueRouteRead
	unions       map[SimpleTypeID][]SimpleTypeID
	enumerations map[SimpleTypeID][]string
	patterns     map[SimpleTypeID]*stringPatternStepRead
}

func (f rawSimpleValueFixture) validate(id SimpleTypeID, raw []byte) (bool, error) {
	resolver := f.resolver()
	return validateResolvedRawSimpleValue(resolver, id, raw)
}

func (f rawSimpleValueFixture) resolver() rawSimpleValueResolver {
	count := 0
	for id := range f.routes {
		count = max(count, int(id)+1)
	}
	routes := make([]simpleValueRouteRead, count)
	cold := simpleValueColdReadTable{index: make([]uint32, count)}
	for i := range cold.index {
		cold.index[i] = invalidID
	}
	for id, route := range f.routes {
		route.present = true
		typ := route.simpleValueType()
		route.rawBypass = SimpleValueBypass(simpleValueAtomicBypassShape(&typ, 0))
		routes[id] = route
		if route.facets == 0 && route.variety != SimpleVarietyUnion {
			continue
		}
		read := simpleValueColdRead{union: f.unions[id]}
		for _, canonical := range f.enumerations[id] {
			read.enumeration = append(read.enumeration, simpleValueLiteralRead{canonical: canonical})
		}
		read.facets = simpleValueFacetRead{patterns: f.patterns[id], present: route.facets}
		cold.index[id] = uint32(len(cold.values)) //nolint:gosec // Test fixture size is bounded by its inline route table.
		cold.values = append(cold.values, read)
	}
	runtime := schemaRuntime{SimpleValueRoutes: routes, SimpleValueCold: cold}
	return rawSimpleValueResolver{runtime: &runtime}
}

func rawSimpleValueTestPatterns(groups [][]StringPattern) *stringPatternStepRead {
	var steps stringPatternSteps
	for _, patterns := range groups {
		steps = appendStringPatternStep(steps, patterns)
	}
	types := []SimpleType{{Facets: FacetSet{patterns: steps}}}
	return newStringPatternReadPoolForSimpleTypes(types)[steps.tail]
}

func TestValidateRawSimpleValueAtomicDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		typ      simpleValueRouteRead
		patterns *stringPatternStepRead
		wantOK   bool
	}{
		{
			name: "accept string",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveString,
			},
			wantOK: true,
		},
		{
			name: "string patterns",
			typ: simpleValueRouteRead{
				facets:    FacetPattern,
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveString,
			},
			patterns: rawSimpleValueTestPatterns([][]StringPattern{{
				NewFastStringPattern(CompileSimpleStringPattern("raw")),
			}}),
			wantOK: true,
		},
		{
			name: "string enumeration",
			typ: simpleValueRouteRead{
				facets:    FacetEnumeration,
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveString,
			},
			wantOK: true,
		},
		{
			name: "integer fast path",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveDecimal,
				builtin:   BuiltinValidationInteger,
				fast:      SimpleFastInt,
			},
			input:  "1",
			wantOK: true,
		},
		{
			name: "decimal validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveDecimal,
			},
			input:  "1.0",
			wantOK: true,
		},
		{
			name: "decimal unsupported facet falls back",
			typ: simpleValueRouteRead{
				facets:    FacetTotalDigits,
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveDecimal,
			},
		},
		{
			name: "boolean validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveBoolean,
			},
			input:  "true",
			wantOK: true,
		},
		{
			name: "date validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveDate,
			},
			input:  "2006-01-02",
			wantOK: true,
		},
		{
			name: "anyURI validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveAnyURI,
			},
			input:  "https://example.test/a%20b",
			wantOK: true,
		},
		{
			name: "hexBinary validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveHexBinary,
			},
			input:  "0a2F",
			wantOK: true,
		},
		{
			name: "base64Binary validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveBase64Binary,
			},
			input:  "AQID",
			wantOK: true,
		},
		{
			name: "float validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveFloat,
			},
			input:  "1.25",
			wantOK: true,
		},
		{
			name: "double validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveDouble,
			},
			input:  "1E9999",
			wantOK: true,
		},
		{
			name: "duration validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveDuration,
			},
			input:  "P1Y2M3DT4H5M6.7S",
			wantOK: true,
		},
		{
			name: "temporal validator",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveDateTime,
			},
			input:  "2026-05-18T24:00:00Z",
			wantOK: true,
		},
		{
			name: "unsupported builtin",
			typ: simpleValueRouteRead{
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveString,
				builtin:   BuiltinValidationName,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := rawSimpleValueFixture{
				routes:       map[SimpleTypeID]simpleValueRouteRead{1: tt.typ},
				enumerations: map[SimpleTypeID][]string{1: {"raw"}},
				patterns:     map[SimpleTypeID]*stringPatternStepRead{1: tt.patterns},
			}
			input := tt.input
			if input == "" {
				input = "raw"
			}
			ok, err := stub.validate(1, []byte(input))
			if err != nil {
				t.Fatalf("ValidateRawSimpleValue() error = %v", err)
			}
			if ok != tt.wantOK {
				t.Fatalf("ValidateRawSimpleValue() handled = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

func TestValidateRawSimpleValueStringExecutors(t *testing.T) {
	t.Parallel()

	pattern := NewFastStringPattern(CompileSimpleStringPattern("ok"))
	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {
				facets:    FacetPattern,
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveString,
			},
			2: {
				facets:    FacetEnumeration,
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveString,
			},
		},
		enumerations: map[SimpleTypeID][]string{2: {"ok"}},
		patterns:     map[SimpleTypeID]*stringPatternStepRead{1: rawSimpleValueTestPatterns([][]StringPattern{{pattern}})},
	}

	ok, err := stub.validate(1, []byte("bad"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "pattern facet failed" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want pattern facet failed", err)
	}

	ok, err = stub.validate(2, []byte("bad"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "enumeration facet failed" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want enumeration facet failed", err)
	}
}

func TestValidateStringPatternStepsGroups(t *testing.T) {
	t.Parallel()

	a := NewFastStringPattern(CompileSimpleStringPattern("a"))
	b := NewFastStringPattern(CompileSimpleStringPattern("b"))
	digit := NewFastStringPattern(CompileSimpleStringPattern(`\d`))
	groups := [][]StringPattern{{a, b}, {digit}}
	steps := newTestStringPatternSteps(groups)
	if err := validateStringPatternSteps(steps, "1"); err == nil || err.Error() != "pattern facet failed" {
		t.Fatalf("validateStringPatternSteps() error = %v, want pattern facet failed", err)
	}
	if err := validateStringPatternSteps(newTestStringPatternSteps(groups[:1]), "b"); err != nil {
		t.Fatalf("validateStringPatternSteps() error = %v", err)
	}

	read := newStringPatternReadPoolForSimpleTypes([]SimpleType{{Facets: FacetSet{patterns: steps}}})[steps.tail]
	if err := validateRawStringPatternStepReads(read, []byte("1")); err == nil || err.Error() != "pattern facet failed" {
		t.Fatalf("validateRawStringPatternStepReads() error = %v, want pattern facet failed", err)
	}
	base := newTestStringPatternSteps(groups[:1])
	read = newStringPatternReadPoolForSimpleTypes([]SimpleType{{Facets: FacetSet{patterns: base}}})[base.tail]
	if err := validateRawStringPatternStepReads(read, []byte("b")); err != nil {
		t.Fatalf("validateRawStringPatternStepReads() error = %v", err)
	}
}

func TestValidateRawSimpleValueListDispatch(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyList, listItem: 2},
			2: {
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveString,
				builtin:   BuiltinValidationNMTOKEN,
			},
		},
	}
	ok, err := stub.validate(1, []byte("a b"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
}

func TestValidateRawSimpleValueNMTOKENListExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyList, listItem: 2},
			2: {
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveString,
				builtin:   BuiltinValidationNMTOKEN,
			},
		},
	}
	ok, err := stub.validate(1, []byte("good bad:name 1"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}

	ok, err = stub.validate(1, []byte("good <bad>"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid NMTOKEN" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid NMTOKEN", err)
	}
}

func TestValidateRawSimpleValueFastIntExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {
				variety:   SimpleVarietyAtomic,
				primitive: PrimitiveDecimal,
				builtin:   BuiltinValidationInteger,
				fast:      SimpleFastInt,
			},
		},
	}
	ok, err := stub.validate(1, []byte("2147483648"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != fastIntErrMaxInclusive {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want maxInclusive failure", err)
	}
}

func TestValidateRawSimpleValueDecimalExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {
				facets:       FacetMinInclusive | FacetMaxInclusive,
				minInclusive: RawDecimalBound{Present: true, Int: "1"},
				maxInclusive: RawDecimalBound{Present: true, Int: "10", Frac: "5"},
				variety:      SimpleVarietyAtomic,
				primitive:    PrimitiveDecimal,
			},
		},
	}
	ok, err := stub.validate(1, []byte("10.50"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}

	ok, err = stub.validate(1, []byte("0.99"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != fastDecimalErrMinInclusive {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want minInclusive failure", err)
	}

	ok, err = stub.validate(1, []byte("10.51"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != fastDecimalErrMaxInclusive {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want maxInclusive failure", err)
	}
}

func TestValidateRawSimpleValueAnyURIExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyAtomic, primitive: PrimitiveAnyURI},
		},
	}
	ok, err := stub.validate(1, []byte("https://example.test/a%20b"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}

	ok, err = stub.validate(1, []byte("a^b"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid anyURI" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid anyURI", err)
	}

	ok, err = stub.validate(1, []byte(" a "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueHexBinaryExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyAtomic, primitive: PrimitiveHexBinary},
		},
	}
	ok, err := stub.validate(1, []byte("0a2F"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}

	ok, err = stub.validate(1, []byte("0g"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid hexBinary" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid hexBinary", err)
	}
}

func TestValidateRawSimpleValueBase64BinaryExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyAtomic, primitive: PrimitiveBase64Binary},
		},
	}
	ok, err := stub.validate(1, []byte("AQID"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}

	ok, err = stub.validate(1, []byte("AQI"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid base64Binary" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid base64Binary", err)
	}

	ok, err = stub.validate(1, []byte("A Q I D"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueFloatExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyAtomic, primitive: PrimitiveFloat},
			2: {variety: SimpleVarietyAtomic, primitive: PrimitiveDouble},
		},
	}
	ok, err := stub.validate(1, []byte("1.25"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}

	ok, err = stub.validate(2, []byte("1E9999"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() overflow error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() overflow handled = false, want true")
	}

	ok, err = stub.validate(1, []byte("+INF"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid float" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid float", err)
	}

	ok, err = stub.validate(1, []byte(" 1 "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueDurationExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyAtomic, primitive: PrimitiveDuration},
		},
	}
	ok, err := stub.validate(1, []byte("P1Y2M3DT4H5M6.7S"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}

	ok, err = stub.validate(1, []byte("P"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid duration" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid duration", err)
	}

	ok, err = stub.validate(1, []byte(" P3D "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueTemporalExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyAtomic, primitive: PrimitiveDateTime},
			2: {variety: SimpleVarietyAtomic, primitive: PrimitiveTime},
			3: {variety: SimpleVarietyAtomic, primitive: PrimitiveGYearMonth},
			4: {variety: SimpleVarietyAtomic, primitive: PrimitiveGYear},
			5: {variety: SimpleVarietyAtomic, primitive: PrimitiveGMonthDay},
			6: {variety: SimpleVarietyAtomic, primitive: PrimitiveGDay},
			7: {variety: SimpleVarietyAtomic, primitive: PrimitiveGMonth},
		},
	}
	for _, tt := range []struct {
		input string
		id    SimpleTypeID
	}{
		{id: 1, input: "2026-05-18T24:00:00Z"},
		{id: 2, input: "23:59:60Z"},
		{id: 3, input: "2000-01Z"},
		{id: 4, input: "10000Z"},
		{id: 5, input: "--02-29"},
		{id: 6, input: "---31-14:00"},
		{id: 7, input: "--12Z"},
	} {
		ok, err := stub.validate(tt.id, []byte(tt.input))
		if err != nil {
			t.Fatalf("ValidateRawSimpleValue(%q) error = %v", tt.input, err)
		}
		if !ok {
			t.Fatalf("ValidateRawSimpleValue(%q) handled = false, want true", tt.input)
		}
	}

	ok, err := stub.validate(1, []byte("2026-05-18T24:00:00.1"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid dateTime" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid dateTime", err)
	}

	ok, err = stub.validate(1, []byte(" 2026-05-18T24:00:00Z "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueBooleanExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyAtomic, primitive: PrimitiveBoolean},
		},
	}
	for _, input := range []string{"true", "false", "1", "0"} {
		ok, err := stub.validate(1, []byte(input))
		if err != nil {
			t.Fatalf("ValidateRawSimpleValue(%q) error = %v", input, err)
		}
		if !ok {
			t.Fatalf("ValidateRawSimpleValue(%q) handled = false, want true", input)
		}
	}
	ok, err := stub.validate(1, []byte("yes"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid boolean" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid boolean", err)
	}
	ok, err = stub.validate(1, []byte(" true "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueDateExecutor(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyAtomic, primitive: PrimitiveDate},
		},
	}
	ok, err := stub.validate(1, []byte("2000-02-29"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}

	ok, err = stub.validate(1, []byte("1900-02-29"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != fastDateErrInvalid {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid date/time", err)
	}

	ok, err = stub.validate(1, []byte("2000-02-29Z"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueMissingType(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{}
	ok, err := stub.validate(1, []byte("raw"))
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want metadata sentinel", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueInvalidVariety(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVariety(99)},
		},
	}
	ok, err := stub.validate(1, []byte("raw"))
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want metadata sentinel", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueListMissingItem(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyList, listItem: 2},
		},
	}
	ok, err := stub.validate(1, []byte("a b"))
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want metadata sentinel", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueUnionBooleanMember(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyUnion},
			2: {variety: SimpleVarietyAtomic, primitive: PrimitiveBoolean},
		},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2}},
	}
	for _, input := range []string{"true", "false", "1", "0"} {
		ok, err := stub.validate(1, []byte(input))
		if err != nil {
			t.Fatalf("ValidateRawSimpleValue(%q) error = %v", input, err)
		}
		if !ok {
			t.Fatalf("ValidateRawSimpleValue(%q) handled = false, want true", input)
		}
	}
	ok, err := stub.validate(1, []byte("yes"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "value does not match any union member" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want union mismatch", err)
	}
}

func TestValidateRawSimpleValueUnionStopsAfterMatch(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyUnion},
			2: {variety: SimpleVarietyAtomic, primitive: PrimitiveBoolean},
		},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2, 3}},
	}
	ok, err := stub.validate(1, []byte("true"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
}

func TestValidateRawSimpleValueUnionUnhandledMember(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyUnion},
			2: {facets: FacetTotalDigits, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal},
		},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2}},
	}
	ok, err := stub.validate(1, []byte("raw"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueUnionMissingMember(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{1: {variety: SimpleVarietyUnion}},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2}},
	}
	ok, err := stub.validate(1, []byte("raw"))
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want metadata sentinel", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueUnionInvalidMembers(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{
			1: {variety: SimpleVarietyUnion},
			2: {variety: SimpleVarietyAtomic, primitive: PrimitiveBoolean},
			3: {variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal},
		},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2, 3}},
	}
	ok, err := stub.validate(1, []byte("raw"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "value does not match any union member" {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
}

func TestValidateRawSimpleValueUnionWhitespaceDisablesRawPath(t *testing.T) {
	t.Parallel()

	stub := rawSimpleValueFixture{
		routes: map[SimpleTypeID]simpleValueRouteRead{1: {variety: SimpleVarietyUnion}},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2}},
	}
	ok, err := stub.validate(1, []byte("a b"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}
