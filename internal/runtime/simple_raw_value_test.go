package runtime

import (
	"errors"
	"strings"
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

type rawSimpleValueCallbackStub struct {
	types        map[SimpleTypeID]RawSimpleValueType
	unions       map[SimpleTypeID][]SimpleTypeID
	enumerations map[SimpleTypeID][]string
	calls        []string
}

func (s *rawSimpleValueCallbackStub) callbacks() RawSimpleValueCallbacks {
	return RawSimpleValueCallbacks{
		Type:                     s.typ,
		ForEachUnionMember:       s.forEachUnionMember,
		ForEachStringEnumeration: s.forEachStringEnumeration,
	}
}

func (s *rawSimpleValueCallbackStub) typ(id SimpleTypeID) (RawSimpleValueType, bool) {
	typ, ok := s.types[id]
	return typ, ok
}

func (s *rawSimpleValueCallbackStub) forEachUnionMember(id SimpleTypeID, yield func(SimpleTypeID) bool) {
	for _, member := range s.unions[id] {
		if !yield(member) {
			return
		}
	}
}

func (s *rawSimpleValueCallbackStub) forEachStringEnumeration(id SimpleTypeID, yield func(string) bool) {
	for _, canonical := range s.enumerations[id] {
		if !yield(canonical) {
			return
		}
	}
}

func TestValidateRawSimpleValueAtomicDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		typ    RawSimpleValueType
		wantOK bool
	}{
		{
			name: "accept string",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveString,
			},
			wantOK: true,
		},
		{
			name: "string patterns",
			typ: RawSimpleValueType{
				Facets: FacetPattern,
				StringPatterns: []StringPatternGroup{{
					Patterns: []StringPattern{
						NewFastStringPattern("raw", CompileSimpleStringPattern("raw")),
					},
				}},
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveString,
			},
			wantOK: true,
		},
		{
			name: "string enumeration",
			typ: RawSimpleValueType{
				Facets:    FacetEnumeration,
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveString,
			},
			wantOK: true,
		},
		{
			name: "integer fast path",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDecimal,
				Builtin:   BuiltinValidationInteger,
				Fast:      SimpleFastInt,
			},
			input:  "1",
			wantOK: true,
		},
		{
			name: "decimal validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDecimal,
			},
			input:  "1.0",
			wantOK: true,
		},
		{
			name: "decimal unsupported facet falls back",
			typ: RawSimpleValueType{
				Facets:    FacetTotalDigits,
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDecimal,
			},
		},
		{
			name: "boolean validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveBoolean,
			},
			input:  "true",
			wantOK: true,
		},
		{
			name: "date validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDate,
			},
			input:  "2006-01-02",
			wantOK: true,
		},
		{
			name: "anyURI validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveAnyURI,
			},
			input:  "https://example.test/a%20b",
			wantOK: true,
		},
		{
			name: "hexBinary validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveHexBinary,
			},
			input:  "0a2F",
			wantOK: true,
		},
		{
			name: "base64Binary validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveBase64Binary,
			},
			input:  "AQID",
			wantOK: true,
		},
		{
			name: "float validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveFloat,
			},
			input:  "1.25",
			wantOK: true,
		},
		{
			name: "double validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDouble,
			},
			input:  "1E9999",
			wantOK: true,
		},
		{
			name: "duration validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDuration,
			},
			input:  "P1Y2M3DT4H5M6.7S",
			wantOK: true,
		},
		{
			name: "temporal validator",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDateTime,
			},
			input:  "2026-05-18T24:00:00Z",
			wantOK: true,
		},
		{
			name: "unsupported builtin",
			typ: RawSimpleValueType{
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveString,
				Builtin:   BuiltinValidationName,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &rawSimpleValueCallbackStub{
				types:        map[SimpleTypeID]RawSimpleValueType{1: tt.typ},
				enumerations: map[SimpleTypeID][]string{1: {"raw"}},
			}
			input := tt.input
			if input == "" {
				input = "raw"
			}
			ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte(input))
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

	pattern := NewFastStringPattern("ok", CompileSimpleStringPattern("ok"))
	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {
				Facets:         FacetPattern,
				StringPatterns: []StringPatternGroup{{Patterns: []StringPattern{pattern}}},
				Variety:        SimpleVarietyAtomic,
				Primitive:      PrimitiveString,
			},
			2: {
				Facets:    FacetEnumeration,
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveString,
			},
		},
		enumerations: map[SimpleTypeID][]string{2: {"ok"}},
	}

	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("bad"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "pattern facet failed" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want pattern facet failed", err)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 2, []byte("bad"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "enumeration facet failed" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want enumeration facet failed", err)
	}
}

func TestValidateRawStringPatternsGroups(t *testing.T) {
	t.Parallel()

	a := NewFastStringPattern("a", CompileSimpleStringPattern("a"))
	b := NewFastStringPattern("b", CompileSimpleStringPattern("b"))
	digit := NewFastStringPattern(`\d`, CompileSimpleStringPattern(`\d`))
	groups := []StringPatternGroup{
		{Patterns: []StringPattern{a, b}},
		{Patterns: []StringPattern{digit}},
	}

	if err := ValidateRawStringPatterns(groups, []byte("1")); err == nil || err.Error() != "pattern facet failed" {
		t.Fatalf("ValidateRawStringPatterns() error = %v, want pattern facet failed", err)
	}
	if err := ValidateRawStringPatterns(groups[:1], []byte("b")); err != nil {
		t.Fatalf("ValidateRawStringPatterns() error = %v", err)
	}
}

func TestValidateStringPatternsGroups(t *testing.T) {
	t.Parallel()

	a := NewFastStringPattern("a", CompileSimpleStringPattern("a"))
	b := NewFastStringPattern("b", CompileSimpleStringPattern("b"))
	digit := NewFastStringPattern(`\d`, CompileSimpleStringPattern(`\d`))
	groups := []StringPatternGroup{
		{Patterns: []StringPattern{a, b}},
		{Patterns: []StringPattern{digit}},
	}

	if err := ValidateStringPatterns(groups, "1"); err == nil || err.Error() != "pattern facet failed" {
		t.Fatalf("ValidateStringPatterns() error = %v, want pattern facet failed", err)
	}
	if err := ValidateStringPatterns(groups[:1], "b"); err != nil {
		t.Fatalf("ValidateStringPatterns() error = %v", err)
	}
}

func TestValidateStringEnumeration(t *testing.T) {
	t.Parallel()

	forEach := func(id SimpleTypeID, yield func(string) bool) {
		if id != 1 {
			t.Fatalf("id = %d, want 1", id)
		}
		for _, value := range []string{"a", "b"} {
			if !yield(value) {
				return
			}
		}
	}
	if err := ValidateStringEnumeration(1, forEach, "b"); err != nil {
		t.Fatalf("ValidateStringEnumeration() error = %v", err)
	}
	if err := ValidateStringEnumeration(1, forEach, "c"); err == nil || err.Error() != "enumeration facet failed" {
		t.Fatalf("ValidateStringEnumeration() error = %v, want enumeration facet failed", err)
	}
	if err := ValidateStringEnumeration(1, nil, "b"); !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateStringEnumeration() error = %v, want metadata sentinel", err)
	}
}

func TestValidateRawSimpleValueListDispatch(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyList, ListItem: 2},
			2: {
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveString,
				Builtin:   BuiltinValidationNMTOKEN,
			},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("a b"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if got := strings.Join(stub.calls, ","); got != "" {
		t.Fatalf("raw validator calls = %q", got)
	}
}

func TestValidateRawSimpleValueNMTOKENListExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyList, ListItem: 2},
			2: {
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveString,
				Builtin:   BuiltinValidationNMTOKEN,
			},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("good bad:name 1"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("good <bad>"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid NMTOKEN" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid NMTOKEN", err)
	}
}

func TestValidateRawSimpleValueFastIntExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDecimal,
				Builtin:   BuiltinValidationInteger,
				Fast:      SimpleFastInt,
			},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("2147483648"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != fastIntErrMaxInclusive {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want maxInclusive failure", err)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}
}

func TestValidateRawSimpleValueDecimalExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {
				Facets:              FacetMinInclusive | FacetMaxInclusive,
				DecimalMinInclusive: RawDecimalBound{Present: true, Int: "1"},
				DecimalMaxInclusive: RawDecimalBound{Present: true, Int: "10", Frac: "5"},
				Variety:             SimpleVarietyAtomic,
				Primitive:           PrimitiveDecimal,
			},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("10.50"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("0.99"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != fastDecimalErrMinInclusive {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want minInclusive failure", err)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("10.51"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != fastDecimalErrMaxInclusive {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want maxInclusive failure", err)
	}
}

func TestValidateRawSimpleValueAnyURIExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveAnyURI},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("https://example.test/a%20b"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("a^b"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid anyURI" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid anyURI", err)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte(" a "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueHexBinaryExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveHexBinary},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("0a2F"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("0g"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid hexBinary" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid hexBinary", err)
	}
}

func TestValidateRawSimpleValueBase64BinaryExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveBase64Binary},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("AQID"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("AQI"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid base64Binary" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid base64Binary", err)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("A Q I D"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueFloatExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveFloat},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDouble},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("1.25"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 2, []byte("1E9999"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() overflow error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() overflow handled = false, want true")
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("+INF"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid float" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid float", err)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte(" 1 "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueDurationExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDuration},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("P1Y2M3DT4H5M6.7S"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("P"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid duration" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid duration", err)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte(" P3D "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueTemporalExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDateTime},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveTime},
			3: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGYearMonth},
			4: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGYear},
			5: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGMonthDay},
			6: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGDay},
			7: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGMonth},
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
		ok, err := ValidateRawSimpleValue(stub.callbacks(), tt.id, []byte(tt.input))
		if err != nil {
			t.Fatalf("ValidateRawSimpleValue(%q) error = %v", tt.input, err)
		}
		if !ok {
			t.Fatalf("ValidateRawSimpleValue(%q) handled = false, want true", tt.input)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}

	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("2026-05-18T24:00:00.1"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid dateTime" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid dateTime", err)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte(" 2026-05-18T24:00:00Z "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
}

func TestValidateRawSimpleValueBooleanExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveBoolean},
		},
	}
	for _, input := range []string{"true", "false", "1", "0"} {
		ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte(input))
		if err != nil {
			t.Fatalf("ValidateRawSimpleValue(%q) error = %v", input, err)
		}
		if !ok {
			t.Fatalf("ValidateRawSimpleValue(%q) handled = false, want true", input)
		}
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("yes"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "invalid boolean" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid boolean", err)
	}
	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte(" true "))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() whitespace fallback error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want whitespace fallback")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}
}

func TestValidateRawSimpleValueDateExecutor(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDate},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("2000-02-29"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("1900-02-29"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != fastDateErrInvalid {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want invalid date/time", err)
	}

	ok, err = ValidateRawSimpleValue(stub.callbacks(), 1, []byte("2000-02-29Z"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueMissingType(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("raw"))
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want metadata sentinel", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueInvalidVariety(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVariety(99)},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("raw"))
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want metadata sentinel", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueListMissingItem(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyList, ListItem: 2},
		},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("a b"))
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want metadata sentinel", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueUnionBooleanMember(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyUnion},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveBoolean},
		},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2}},
	}
	for _, input := range []string{"true", "false", "1", "0"} {
		ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte(input))
		if err != nil {
			t.Fatalf("ValidateRawSimpleValue(%q) error = %v", input, err)
		}
		if !ok {
			t.Fatalf("ValidateRawSimpleValue(%q) handled = false, want true", input)
		}
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("yes"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "value does not match any union member" {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want union mismatch", err)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("raw validator calls = %v, want none", stub.calls)
	}
}

func TestValidateRawSimpleValueUnionUnhandledMember(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyUnion},
			2: {Facets: FacetTotalDigits, Variety: SimpleVarietyAtomic, Primitive: PrimitiveDecimal},
		},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2}},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("raw"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueUnionMissingMember(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types:  map[SimpleTypeID]RawSimpleValueType{1: {Variety: SimpleVarietyUnion}},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2}},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("raw"))
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateRawSimpleValue() error = %v, want metadata sentinel", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}

func TestValidateRawSimpleValueUnionInvalidMembers(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types: map[SimpleTypeID]RawSimpleValueType{
			1: {Variety: SimpleVarietyUnion},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveBoolean},
			3: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDecimal},
		},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2, 3}},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("raw"))
	if !ok {
		t.Fatal("ValidateRawSimpleValue() handled = false, want true")
	}
	if err == nil || err.Error() != "value does not match any union member" {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if got := strings.Join(stub.calls, ","); got != "" {
		t.Fatalf("raw validator calls = %q", got)
	}
}

func TestValidateRawSimpleValueUnionWhitespaceDisablesRawPath(t *testing.T) {
	t.Parallel()

	stub := &rawSimpleValueCallbackStub{
		types:  map[SimpleTypeID]RawSimpleValueType{1: {Variety: SimpleVarietyUnion}},
		unions: map[SimpleTypeID][]SimpleTypeID{1: {2}},
	}
	ok, err := ValidateRawSimpleValue(stub.callbacks(), 1, []byte("a b"))
	if err != nil {
		t.Fatalf("ValidateRawSimpleValue() error = %v", err)
	}
	if ok {
		t.Fatal("ValidateRawSimpleValue() handled = true, want false")
	}
}
