package runtime

import (
	"errors"
	"strings"
	"testing"
)

type simpleValueCallbackStub struct {
	unsupported     error
	unsupportedFunc func(error) bool
	types           map[SimpleTypeID]SimpleValueType
	facets          map[SimpleTypeID]SimpleValueFacets
	enums           map[SimpleTypeID][]string
	qnames          map[string]qnameResolution
	notations       map[ExpandedName]bool
	calls           []string
}

type qnameResolution struct {
	ns, local string
	ok        bool
}

func (s *simpleValueCallbackStub) callbacks() SimpleValueCallbacks {
	cb := SimpleValueCallbacks{
		Type:                     s.typ,
		Facets:                   s.simpleValueFacets,
		ForEachStringEnumeration: s.forEachStringEnumeration,
		Unsupported:              s.isUnsupported,
	}
	if s.qnames != nil {
		cb.ResolveQName = s.resolveQName
	}
	if s.notations != nil {
		cb.Notation = s.notation
	}
	return cb
}

func (s *simpleValueCallbackStub) typ(id SimpleTypeID) (SimpleValueType, bool) {
	typ, ok := s.types[id]
	return typ, ok
}

func (s *simpleValueCallbackStub) forEachStringEnumeration(id SimpleTypeID, yield func(string) bool) {
	for _, canonical := range s.enums[id] {
		if !yield(canonical) {
			return
		}
	}
}

func (s *simpleValueCallbackStub) simpleValueFacets(id SimpleTypeID) (SimpleValueFacets, bool) {
	if s.facets != nil {
		facets, ok := s.facets[id]
		return facets, ok
	}
	typ, ok := s.types[id]
	if !ok {
		return SimpleValueFacets{}, false
	}
	return SimpleValueFacets{
		StringFacets:  typ.StringFacets,
		DecimalFacets: typ.DecimalFacets,
		LengthFacets:  typ.LengthFacets,
		Facets:        typ.Facets,
	}, true
}

func (s *simpleValueCallbackStub) resolveQName(lexical string) (string, string, bool) {
	s.calls = append(s.calls, "qname:"+lexical)
	got, ok := s.qnames[lexical]
	if !ok {
		return "", "", false
	}
	return got.ns, got.local, got.ok
}

func (s *simpleValueCallbackStub) notation(ns, local string) bool {
	s.calls = append(s.calls, "notation:"+FormatExpandedName(ns, local))
	return s.notations[ExpandedName{Namespace: ns, Local: local}]
}

func (s *simpleValueCallbackStub) isUnsupported(err error) bool {
	if s.unsupportedFunc != nil {
		return s.unsupportedFunc(err)
	}
	return s.unsupported != nil && errors.Is(err, s.unsupported)
}

func TestPublishedSimpleValueSharedFallback(t *testing.T) {
	t.Parallel()

	types := []SimpleType{{
		Facets: FacetSet{
			Present:     FacetEnumeration,
			Enumeration: []CompiledLiteral{{Canonical: "allowed"}},
		},
		Variety:    SimpleVarietyAtomic,
		Primitive:  PrimitiveString,
		Whitespace: WhitespacePreserve,
	}}
	types[0].Fast = DeriveSimpleFastPathForSimpleType(types[0])
	schema := &Schema{runtime: schemaRuntime{
		SimpleValueRoutes: newSimpleValueRouteReadsForSimpleTypes(types),
		SimpleValueCold:   newSimpleValueColdReadTable(types),
	}}

	if _, err := schema.validatePublishedSimpleValue(0, "allowed", nil, 0); err != nil {
		t.Fatalf("validatePublishedSimpleValue() error = %v", err)
	}
	if _, err := schema.validatePublishedSimpleValue(0, "rejected", nil, 0); err == nil || err.Error() != "enumeration facet failed" {
		t.Fatalf("validatePublishedSimpleValue() error = %v, want enumeration failure", err)
	}
	if handled, err := schema.validatePublishedRawSimpleValue(0, []byte("allowed")); !handled || err != nil {
		t.Fatalf("validatePublishedRawSimpleValue() = %v, %v; want true, nil", handled, err)
	}
	if handled, err := schema.validatePublishedRawSimpleValue(0, []byte("rejected")); !handled || err == nil || err.Error() != "enumeration facet failed" {
		t.Fatalf("validatePublishedRawSimpleValue() = %v, %v; want handled enumeration failure", handled, err)
	}
}

func TestPublishedSimpleValueFastPathAllocations(t *testing.T) {
	types := []SimpleType{{
		Variety:    SimpleVarietyAtomic,
		Primitive:  PrimitiveDecimal,
		Builtin:    BuiltinValidationInteger,
		Whitespace: WhitespaceCollapse,
		Fast:       SimpleFastInt,
	}}
	schema := &Schema{runtime: schemaRuntime{
		SimpleValueRoutes: newSimpleValueRouteReadsForSimpleTypes(types),
		SimpleValueCold:   newSimpleValueColdReadTable(types),
	}}
	var value SimpleValue
	var err error
	allocs := testing.AllocsPerRun(1_000, func() {
		value, err = schema.validatePublishedSimpleValue(0, "7", nil, 0)
	})
	if err != nil || value.Type != 0 {
		t.Fatalf("validatePublishedSimpleValue() = %+v, %v", value, err)
	}
	if allocs != 0 {
		t.Fatalf("validatePublishedSimpleValue() allocations = %v, want 0", allocs)
	}
}

func TestPublishedNotationFastPathAllocations(t *testing.T) {
	types := []SimpleType{{
		Variety:    SimpleVarietyAtomic,
		Primitive:  PrimitiveNotation,
		Whitespace: WhitespacePreserve,
	}}
	schema := &Schema{runtime: schemaRuntime{
		SimpleValueRoutes: newSimpleValueRouteReadsForSimpleTypes(types),
		SimpleValueCold:   newSimpleValueColdReadTable(types),
		Notations:         map[ExpandedName]bool{{Local: "declared"}: true},
	}}
	var value SimpleValue
	var err error
	allocs := testing.AllocsPerRun(1_000, func() {
		value, err = schema.validatePublishedSimpleValue(0, "declared", nil, 0)
	})
	if err != nil || value.Type != 0 {
		t.Fatalf("validatePublishedSimpleValue() = %+v, %v", value, err)
	}
	if allocs != 0 {
		t.Fatalf("validatePublishedSimpleValue() NOTATION allocations = %v, want 0", allocs)
	}
}

func TestPublishedRawUnionFastPathAllocations(t *testing.T) {
	types := []SimpleType{
		{
			Union:   []SimpleTypeID{1},
			Variety: SimpleVarietyUnion,
		},
		{
			Variety:   SimpleVarietyAtomic,
			Primitive: PrimitiveBoolean,
		},
	}
	schema := &Schema{runtime: schemaRuntime{
		SimpleValueRoutes: newSimpleValueRouteReadsForSimpleTypes(types),
		SimpleValueCold:   newSimpleValueColdReadTable(types),
	}}
	raw := []byte("true")
	var handled bool
	var err error
	allocs := testing.AllocsPerRun(1_000, func() {
		handled, err = schema.validatePublishedRawSimpleValue(0, raw)
	})
	if err != nil || !handled {
		t.Fatalf("validatePublishedRawSimpleValue() = %v, %v; want true, nil", handled, err)
	}
	if allocs != 0 {
		t.Fatalf("validatePublishedRawSimpleValue() allocations = %v, want 0", allocs)
	}
}

func TestValidateSimpleValueRoute(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{}
	value, err := ValidateSimpleValue(stub.callbacks(), NoSimpleType, "raw", 0)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	if value.Canonical != "raw" || value.Type != NoSimpleType {
		t.Fatalf("ValidateSimpleValue() = %#v, want untyped raw value", value)
	}

	_, err = ValidateSimpleValue(stub.callbacks(), 1, "raw", 0)
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateSimpleValue() error = %v, want metadata sentinel", err)
	}

	stub.types = map[SimpleTypeID]SimpleValueType{
		1: {Variety: SimpleVariety(99)},
	}
	_, err = ValidateSimpleValue(stub.callbacks(), 1, "raw", 0)
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateSimpleValue() error = %v, want metadata sentinel", err)
	}
}

func TestValidateSimpleValueAtomicNormalizesBeforeExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveAnyURI, Whitespace: WhitespaceCollapse},
		},
	}
	value, err := ValidateSimpleValue(stub.callbacks(), 1, " a  b ", SimpleNeedCanonical)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	if value.Canonical != "a b" {
		t.Fatalf("ValidateSimpleValue() canonical = %q, want %q", value.Canonical, "a b")
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
	}
}

func TestValidateSimpleValueAtomicStringBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveString, Whitespace: WhitespaceCollapse},
		},
	}
	value, err := ValidateSimpleValue(stub.callbacks(), 1, " a  b ", SimpleNeedCanonical|SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	if value.Canonical != "a b" {
		t.Fatalf("ValidateSimpleValue() canonical = %q, want %q", value.Canonical, "a b")
	}
	if value.Identity != SimpleIdentityKey(PrimitiveString, "a b") {
		t.Fatalf("ValidateSimpleValue() identity = %q, want string identity", value.Identity)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicAnyURIBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveAnyURI, Whitespace: WhitespaceCollapse},
		},
	}
	for _, lexical := range []string{"", "https://example.test/a%20b", " urn:test "} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", lexical, err)
		}
	}
	for _, lexical := range []string{":a", "a:", "%", "a^b", `a\b`} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err == nil || err.Error() != "invalid anyURI" {
			t.Fatalf("ValidateSimpleValue(%q) error = %v, want invalid anyURI", lexical, err)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicHexBinaryBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveHexBinary, Whitespace: WhitespaceCollapse},
		},
	}
	for _, lexical := range []string{"", "0a2F", " 0A2f "} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", lexical, err)
		}
	}
	for _, lexical := range []string{"0af", "0g"} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err == nil || err.Error() != "invalid hexBinary" {
			t.Fatalf("ValidateSimpleValue(%q) error = %v, want invalid hexBinary", lexical, err)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicBase64BinaryBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveBase64Binary, Whitespace: WhitespaceCollapse},
		},
	}
	for _, lexical := range []string{"", "AQID", " A Q I D ", "AQ=="} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", lexical, err)
		}
	}
	for _, lexical := range []string{"AQI", "AQ$D", "AQ=I"} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err == nil || err.Error() != "invalid base64Binary" {
			t.Fatalf("ValidateSimpleValue(%q) error = %v, want invalid base64Binary", lexical, err)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicFloatBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveFloat, Whitespace: WhitespaceCollapse},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDouble, Whitespace: WhitespaceCollapse},
		},
	}
	for _, tt := range []struct {
		lexical string
		id      SimpleTypeID
	}{
		{id: 1, lexical: "1.25"},
		{id: 1, lexical: " INF "},
		{id: 2, lexical: "1E9999"},
		{id: 2, lexical: "NaN"},
	} {
		_, err := ValidateSimpleValue(stub.callbacks(), tt.id, tt.lexical, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", tt.lexical, err)
		}
	}
	for _, tt := range []struct {
		lexical string
		id      SimpleTypeID
	}{
		{id: 1, lexical: "+INF"},
		{id: 2, lexical: "1e"},
	} {
		_, err := ValidateSimpleValue(stub.callbacks(), tt.id, tt.lexical, 0)
		if err == nil || err.Error() != "invalid float" {
			t.Fatalf("ValidateSimpleValue(%q) error = %v, want invalid float", tt.lexical, err)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicDurationBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDuration, Whitespace: WhitespaceCollapse},
		},
	}
	for _, lexical := range []string{"P1Y2M3DT4H5M6.7S", "-PT0S", " P3D "} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", lexical, err)
		}
	}
	for _, lexical := range []string{"P", "PT", "P1.5Y", "PT9223372036854775808S"} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err == nil || err.Error() != "invalid duration" {
			t.Fatalf("ValidateSimpleValue(%q) error = %v, want invalid duration", lexical, err)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicFastIntBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDecimal,
				Builtin:   BuiltinValidationInteger,
				Fast:      SimpleFastInt,
			},
		},
	}
	_, err := ValidateSimpleValue(stub.callbacks(), 1, "2147483647", 0)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	_, err = ValidateSimpleValue(stub.callbacks(), 1, "2147483648", 0)
	if err == nil || err.Error() != "maxInclusive facet failed" {
		t.Fatalf("ValidateSimpleValue() error = %v, want maxInclusive failure", err)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicDecimalBypassUsesRuntimeWhenHandled(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {
				DecimalMinInclusive: RawDecimalBound{Present: true, Int: "1"},
				DecimalMaxInclusive: RawDecimalBound{Present: true, Int: "10", Frac: "5"},
				Facets:              FacetMinInclusive | FacetMaxInclusive,
				Variety:             SimpleVarietyAtomic,
				Primitive:           PrimitiveDecimal,
			},
		},
	}
	for _, lexical := range []string{"1", "10.50", "+0002.500"} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", lexical, err)
		}
	}
	for _, tt := range []struct {
		lexical string
		wantErr string
	}{
		{lexical: "0.99", wantErr: fastDecimalErrMinInclusive},
		{lexical: "10.51", wantErr: fastDecimalErrMaxInclusive},
	} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, tt.lexical, 0)
		if err == nil || err.Error() != tt.wantErr {
			t.Fatalf("ValidateSimpleValue(%q) error = %v, want %q", tt.lexical, err, tt.wantErr)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicDecimalBypassUsesRuntimeFacets(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {
				DecimalFacets: DecimalFacetValues{
					TotalDigits: FacetCardinalityValue{Value: 1, Present: true},
					Facets:      FacetTotalDigits,
				},
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDecimal,
				Facets:    FacetTotalDigits,
			},
		},
	}
	_, err := ValidateSimpleValue(stub.callbacks(), 1, "12", 0)
	if err == nil || err.Error() != "totalDigits facet failed" {
		t.Fatalf("ValidateSimpleValue() error = %v, want totalDigits failure", err)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicLengthFacetsUseRuntime(t *testing.T) {
	t.Parallel()

	cardinality := func(v uint32) FacetCardinalityValue {
		return FacetCardinalityValue{Value: v, Present: true}
	}
	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {
				Facets:       FacetLength,
				LengthFacets: LengthFacetValues{Length: cardinality(2)},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveString,
			},
			2: {
				Facets:       FacetMinLength,
				LengthFacets: LengthFacetValues{MinLength: cardinality(3)},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveAnyURI,
			},
			3: {
				Facets:       FacetLength,
				LengthFacets: LengthFacetValues{Length: cardinality(2)},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveHexBinary,
			},
			4: {
				Facets:       FacetMaxLength,
				LengthFacets: LengthFacetValues{MaxLength: cardinality(3)},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveBase64Binary,
			},
		},
	}
	for _, tt := range []struct {
		input string
		id    SimpleTypeID
	}{
		{id: 1, input: "ab"},
		{id: 2, input: "abc"},
		{id: 3, input: "0AFF"},
		{id: 4, input: "AQID"},
	} {
		_, err := ValidateSimpleValue(stub.callbacks(), tt.id, tt.input, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", tt.input, err)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
	}
}

func TestValidateSimpleValueAtomicLengthFacetsRejectBeforeExecutor(t *testing.T) {
	t.Parallel()

	cardinality := func(v uint32) FacetCardinalityValue {
		return FacetCardinalityValue{Value: v, Present: true}
	}
	tests := []struct {
		name    string
		input   string
		wantErr string
		typ     SimpleValueType
	}{
		{
			name: "string length mismatch",
			typ: SimpleValueType{
				Facets:       FacetLength,
				LengthFacets: LengthFacetValues{Length: cardinality(2)},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveString,
			},
			input:   "abc",
			wantErr: "length facet failed",
		},
		{
			name: "anyURI invalid lexical",
			typ: SimpleValueType{
				Facets:       FacetMinLength,
				LengthFacets: LengthFacetValues{MinLength: cardinality(1)},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveAnyURI,
			},
			input:   ":bad",
			wantErr: "invalid anyURI",
		},
		{
			name: "hexBinary length mismatch",
			typ: SimpleValueType{
				Facets:       FacetLength,
				LengthFacets: LengthFacetValues{Length: cardinality(2)},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveHexBinary,
			},
			input:   "0A",
			wantErr: "length facet failed",
		},
		{
			name: "base64Binary length mismatch",
			typ: SimpleValueType{
				Facets:       FacetMinLength,
				LengthFacets: LengthFacetValues{MinLength: cardinality(2)},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveBase64Binary,
			},
			input:   "AQ==",
			wantErr: "minLength facet failed",
		},
		{
			name: "base64Binary invalid lexical",
			typ: SimpleValueType{
				Facets:       FacetMinLength,
				LengthFacets: LengthFacetValues{MinLength: cardinality(1)},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveBase64Binary,
			},
			input:   "AB==",
			wantErr: "invalid base64Binary",
		},
		{
			name: "missing length metadata",
			typ: SimpleValueType{
				Facets:    FacetLength,
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveString,
			},
			input:   "a",
			wantErr: ErrSimpleValueMetadata.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &simpleValueCallbackStub{
				types: map[SimpleTypeID]SimpleValueType{1: tt.typ},
			}
			_, err := ValidateSimpleValue(stub.callbacks(), 1, tt.input, 0)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleValue() error = %v, want %q", err, tt.wantErr)
			}
			if len(stub.calls) != 0 {
				t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
			}
		})
	}
}

func TestValidateSimpleValueAtomicBuiltinDerivedUsesRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
		builtin BuiltinValidationKind
		prim    PrimitiveKind
	}{
		{name: "integer rejects non-decimal text", builtin: BuiltinValidationInteger, prim: PrimitiveDecimal, input: "abc", wantErr: "invalid decimal"},
		{name: "integer rejects decimal lexical", builtin: BuiltinValidationInteger, prim: PrimitiveDecimal, input: "1.0", wantErr: "invalid integer"},
		{name: "Name", builtin: BuiltinValidationName, prim: PrimitiveString, input: "a b", wantErr: "invalid Name"},
		{name: "NCName", builtin: BuiltinValidationNCName, prim: PrimitiveString, input: "a:b", wantErr: "invalid NCName"},
		{name: "NMTOKEN", builtin: BuiltinValidationNMTOKEN, prim: PrimitiveString, input: "a b", wantErr: "invalid NMTOKEN"},
		{name: "language", builtin: BuiltinValidationLanguage, prim: PrimitiveString, input: "en_us", wantErr: "invalid language"},
		{name: "xml lang", builtin: BuiltinValidationXMLLang, prim: PrimitiveString, input: "en_us", wantErr: "invalid language"},
		{name: "xml space", builtin: BuiltinValidationXMLSpace, prim: PrimitiveString, input: "collapse", wantErr: "invalid xml:space"},
		{name: "ENTITY", builtin: BuiltinValidationEntity, prim: PrimitiveString, input: "entity", wantErr: "unsupported.entity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &simpleValueCallbackStub{
				types: map[SimpleTypeID]SimpleValueType{
					1: {
						Variety:   SimpleVarietyAtomic,
						Primitive: tt.prim,
						Builtin:   tt.builtin,
					},
				},
			}
			_, err := ValidateSimpleValue(stub.callbacks(), 1, tt.input, 0)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSimpleValue() error = %v, want containing %q", err, tt.wantErr)
			}
			if len(stub.calls) != 0 {
				t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
			}
		})
	}
}

func TestValidateSimpleValueAtomicStringIdentityBuiltinFastPath(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {
				Variety:    SimpleVarietyAtomic,
				Primitive:  PrimitiveString,
				Builtin:    BuiltinValidationNCName,
				Whitespace: WhitespaceCollapse,
				Identity:   SimpleIdentityID,
			},
		},
	}
	value, err := ValidateSimpleValue(stub.callbacks(), 1, "  id1  ", SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	if value.Canonical != "id1" || value.IDs != "id1" || value.Identity != SimpleIdentityKey(PrimitiveString, "id1") {
		t.Fatalf("ValidateSimpleValue() = %#v, want ID identity payload for id1", value)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
	}

	_, err = ValidateSimpleValue(stub.callbacks(), 1, "a:b", SimpleNeedIdentity)
	if err == nil || err.Error() != "invalid NCName" {
		t.Fatalf("ValidateSimpleValue(invalid) error = %v, want invalid NCName", err)
	}
}

func TestValidateSimpleValueAtomicIntegerUsesFallbackAfterLexicalValidation(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {
				Variety:   SimpleVarietyAtomic,
				Primitive: PrimitiveDecimal,
				Builtin:   BuiltinValidationInteger,
			},
		},
	}
	value, err := ValidateSimpleValue(stub.callbacks(), 1, "+0005", SimpleNeedCanonical)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	if value.Canonical != "5" {
		t.Fatalf("ValidateSimpleValue() canonical = %q, want integer canonical", value.Canonical)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
	}
}

func TestValidateSimpleValueAtomicBuiltinDerivedPrecedesRuntimeLength(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {
				Facets:       FacetLength,
				LengthFacets: LengthFacetValues{Length: FacetCardinalityValue{Value: 1, Present: true}},
				Variety:      SimpleVarietyAtomic,
				Primitive:    PrimitiveString,
				Builtin:      BuiltinValidationName,
			},
		},
	}
	_, err := ValidateSimpleValue(stub.callbacks(), 1, "a b", 0)
	if err == nil || err.Error() != "invalid Name" {
		t.Fatalf("ValidateSimpleValue() error = %v, want invalid Name", err)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicBooleanBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveBoolean, Whitespace: WhitespaceCollapse},
		},
	}
	for _, lexical := range []string{"true", "false", "1", "0", " true "} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", lexical, err)
		}
	}
	_, err := ValidateSimpleValue(stub.callbacks(), 1, "yes", 0)
	if err == nil || err.Error() != "invalid boolean" {
		t.Fatalf("ValidateSimpleValue() error = %v, want invalid boolean", err)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicDateBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDate, Whitespace: WhitespaceCollapse},
		},
	}
	for _, lexical := range []string{
		"2000-02-29",
		"10000-01-01",
		"-0001-01-01",
		" 2026-05-18Z ",
		"2026-05-18+14:00",
		"2026-05-18-14:00",
	} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, lexical, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", lexical, err)
		}
	}
	for _, tt := range []struct {
		lexical string
		wantErr string
	}{
		{lexical: "1900-02-29", wantErr: dateErrInvalidDateTime},
		{lexical: "2026-05-18+14:01", wantErr: dateErrInvalidTimezone},
	} {
		_, err := ValidateSimpleValue(stub.callbacks(), 1, tt.lexical, 0)
		if err == nil || err.Error() != tt.wantErr {
			t.Fatalf("ValidateSimpleValue(%q) error = %v, want %q", tt.lexical, err, tt.wantErr)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicTemporalBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDateTime, Whitespace: WhitespaceCollapse},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveTime, Whitespace: WhitespaceCollapse},
			3: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGYearMonth, Whitespace: WhitespaceCollapse},
			4: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGYear, Whitespace: WhitespaceCollapse},
			5: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGMonthDay, Whitespace: WhitespaceCollapse},
			6: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGDay, Whitespace: WhitespaceCollapse},
			7: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveGMonth, Whitespace: WhitespaceCollapse},
		},
	}
	for _, tt := range []struct {
		lexical string
		id      SimpleTypeID
	}{
		{id: 1, lexical: " 2026-05-18T24:00:00Z "},
		{id: 2, lexical: "23:59:60Z"},
		{id: 3, lexical: "2000-01Z"},
		{id: 4, lexical: "10000Z"},
		{id: 5, lexical: "--02-29"},
		{id: 6, lexical: "---31-14:00"},
		{id: 7, lexical: "--12Z"},
	} {
		_, err := ValidateSimpleValue(stub.callbacks(), tt.id, tt.lexical, 0)
		if err != nil {
			t.Fatalf("ValidateSimpleValue(%q) error = %v", tt.lexical, err)
		}
	}
	for _, tt := range []struct {
		lexical string
		wantErr string
		id      SimpleTypeID
	}{
		{id: 1, lexical: "2026-05-18T24:00:00.1", wantErr: "invalid dateTime"},
		{id: 2, lexical: "23:58:60", wantErr: "invalid time"},
		{id: 3, lexical: "2000-13", wantErr: "invalid gYearMonth"},
		{id: 4, lexical: "02000", wantErr: dateErrInvalidDateTime},
		{id: 5, lexical: "--02-30", wantErr: "invalid gMonthDay"},
		{id: 6, lexical: "---32", wantErr: "invalid gDay"},
		{id: 7, lexical: "--13", wantErr: "invalid gMonth"},
	} {
		_, err := ValidateSimpleValue(stub.callbacks(), tt.id, tt.lexical, 0)
		if err == nil || err.Error() != tt.wantErr {
			t.Fatalf("ValidateSimpleValue(%q) error = %v, want %q", tt.lexical, err, tt.wantErr)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
	}
}

func TestValidateSimpleValueAtomicFallbackBuildsProjection(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveDecimal},
		},
	}
	value, err := ValidateSimpleValue(stub.callbacks(), 1, "5.0", SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	if value.Identity != SimpleIdentityKey(PrimitiveDecimal, "5.0") {
		t.Fatalf("ValidateSimpleValue() identity = %q, want decimal identity canonical", value.Identity)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
	}
}

func TestValidateSimpleValueAtomicStringFacetBypassDoesNotCallExecutor(t *testing.T) {
	t.Parallel()

	pattern := StringFacetValues{
		Patterns: []StringPatternGroup{{
			Patterns: []StringPattern{
				NewFastStringPattern("ok", CompileSimpleStringPattern("ok")),
			},
		}},
	}
	tests := []struct {
		enums   map[SimpleTypeID][]string
		name    string
		lexical string
		wantErr string
		typ     SimpleValueType
	}{
		{
			name:    "pattern match",
			typ:     SimpleValueType{StringFacets: pattern, Facets: FacetPattern, Variety: SimpleVarietyAtomic, Primitive: PrimitiveString},
			lexical: "ok",
		},
		{
			name:    "pattern mismatch",
			typ:     SimpleValueType{StringFacets: pattern, Facets: FacetPattern, Variety: SimpleVarietyAtomic, Primitive: PrimitiveString},
			lexical: "bad",
			wantErr: "pattern facet failed",
		},
		{
			name:    "enumeration match",
			typ:     SimpleValueType{StringFacets: StringFacetValues{HasEnumeration: true}, Facets: FacetEnumeration, Variety: SimpleVarietyAtomic, Primitive: PrimitiveString},
			enums:   map[SimpleTypeID][]string{1: {"ok"}},
			lexical: "ok",
		},
		{
			name:    "enumeration mismatch",
			typ:     SimpleValueType{StringFacets: StringFacetValues{HasEnumeration: true}, Facets: FacetEnumeration, Variety: SimpleVarietyAtomic, Primitive: PrimitiveString},
			enums:   map[SimpleTypeID][]string{1: {"other"}},
			lexical: "ok",
			wantErr: "enumeration facet failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &simpleValueCallbackStub{
				types: map[SimpleTypeID]SimpleValueType{1: tt.typ},
				enums: tt.enums,
			}
			_, err := ValidateSimpleValue(stub.callbacks(), 1, tt.lexical, 0)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleValue() error = %v", err)
				}
			} else if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleValue() error = %v, want %q", err, tt.wantErr)
			}
			if len(stub.calls) != 0 {
				t.Fatalf("calls = %v, want no atomic executor calls", stub.calls)
			}
		})
	}
}

func TestValidateSimpleValueListBuildsCanonicalAndIDRefs(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyList, ListItem: 2, Identity: SimpleIdentityIDREFList},
			2: {Variety: SimpleVarietyAtomic, Identity: SimpleIdentityIDREF},
		},
	}
	value, err := ValidateSimpleValue(stub.callbacks(), 1, "a b", SimpleNeedCanonical|SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	if value.Canonical != "a b" {
		t.Fatalf("ValidateSimpleValue() canonical = %q", value.Canonical)
	}
	if value.IDRefs != "a b" {
		t.Fatalf("ValidateSimpleValue() IDRefs = %q", value.IDRefs)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
	}
}

func TestValidateSimpleValueListSplitsXMLWhitespace(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyList, ListItem: 2, Identity: SimpleIdentityIDREFList},
			2: {Variety: SimpleVarietyAtomic, Identity: SimpleIdentityIDREF},
		},
	}
	value, err := ValidateSimpleValue(stub.callbacks(), 1, "\ta\n b\r\n", SimpleNeedCanonical|SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	if value.Canonical != "a b" {
		t.Fatalf("ValidateSimpleValue() canonical = %q", value.Canonical)
	}
	if value.IDRefs != "a b" {
		t.Fatalf("ValidateSimpleValue() IDRefs = %q", value.IDRefs)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
	}
}

func TestValidateSimpleValueListLengthFacets(t *testing.T) {
	t.Parallel()

	value := func(v uint32) FacetCardinalityValue {
		return FacetCardinalityValue{Value: v, Present: true}
	}
	tests := []struct {
		name    string
		wantErr string
		typ     SimpleValueType
	}{
		{
			name: "length match",
			typ: SimpleValueType{
				Facets:       FacetLength,
				LengthFacets: LengthFacetValues{Length: value(2)},
			},
		},
		{
			name: "length mismatch",
			typ: SimpleValueType{
				Facets:       FacetLength,
				LengthFacets: LengthFacetValues{Length: value(1)},
			},
			wantErr: "length facet failed",
		},
		{
			name: "minLength mismatch",
			typ: SimpleValueType{
				Facets:       FacetMinLength,
				LengthFacets: LengthFacetValues{MinLength: value(3)},
			},
			wantErr: "minLength facet failed",
		},
		{
			name: "maxLength mismatch",
			typ: SimpleValueType{
				Facets:       FacetMaxLength,
				LengthFacets: LengthFacetValues{MaxLength: value(1)},
			},
			wantErr: "maxLength facet failed",
		},
		{
			name: "mask without value",
			typ: SimpleValueType{
				Facets: FacetLength,
			},
			wantErr: ErrSimpleValueMetadata.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			typ := tt.typ
			typ.Variety = SimpleVarietyList
			typ.ListItem = 2
			stub := &simpleValueCallbackStub{
				types: map[SimpleTypeID]SimpleValueType{
					1: typ,
					2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveQName},
				},
			}
			_, err := ValidateSimpleValue(stub.callbacks(), 1, "a b", 0)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleValue() error = %v", err)
				}
				if len(stub.calls) != 0 {
					t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleValue() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleValueListStringFacets(t *testing.T) {
	t.Parallel()

	matchingPattern := StringFacetValues{
		Patterns: []StringPatternGroup{{
			Patterns: []StringPattern{
				NewFastStringPattern("a b", CompileSimpleStringPattern("a b")),
			},
		}},
	}
	failingPattern := StringFacetValues{
		Patterns: []StringPatternGroup{{
			Patterns: []StringPattern{
				NewFastStringPattern("z", CompileSimpleStringPattern("z")),
			},
		}},
	}
	tests := []struct {
		enums   map[SimpleTypeID][]string
		name    string
		wantErr string
		strings StringFacetValues
		facets  FacetMask
	}{
		{name: "pattern match", facets: FacetPattern, strings: matchingPattern},
		{name: "pattern mismatch", facets: FacetPattern, strings: failingPattern, wantErr: "pattern facet failed"},
		{name: "pattern mask without patterns", facets: FacetPattern, wantErr: ErrSimpleValueMetadata.Error()},
		{name: "enumeration match", facets: FacetEnumeration, strings: StringFacetValues{HasEnumeration: true}, enums: map[SimpleTypeID][]string{1: {"a b"}}},
		{name: "enumeration mismatch", facets: FacetEnumeration, strings: StringFacetValues{HasEnumeration: true}, enums: map[SimpleTypeID][]string{1: {"c"}}, wantErr: "enumeration facet failed"},
		{name: "enumeration mask without values", facets: FacetEnumeration, wantErr: ErrSimpleValueMetadata.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &simpleValueCallbackStub{
				types: map[SimpleTypeID]SimpleValueType{
					1: {StringFacets: tt.strings, ListItem: 2, Facets: tt.facets, Variety: SimpleVarietyList},
					2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveQName},
				},
				enums: tt.enums,
			}
			_, err := ValidateSimpleValue(stub.callbacks(), 1, "a b", 0)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleValue() error = %v", err)
				}
				if len(stub.calls) != 0 {
					t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleValue() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleValueListMissingItemIsMetadataError(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyList, ListItem: 2},
		},
	}
	_, err := ValidateSimpleValue(stub.callbacks(), 1, "a", 0)
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateSimpleValue() error = %v, want metadata sentinel", err)
	}
}

func TestValidateSimpleValueUnionKeepsUnsupportedOnlyIfNoMemberMatches(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {UnionMembers: []SimpleTypeID{2, 3}, Variety: SimpleVarietyUnion, Whitespace: WhitespaceCollapse},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveString, Builtin: BuiltinValidationEntity},
			3: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveQName},
		},
		unsupportedFunc: func(err error) bool {
			return strings.Contains(err.Error(), "unsupported.entity")
		},
	}
	value, err := ValidateSimpleValue(stub.callbacks(), 1, " raw ", SimpleNeedCanonical)
	if err != nil {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
	if value.Canonical != "raw" {
		t.Fatalf("ValidateSimpleValue() canonical = %q, want raw", value.Canonical)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
	}
}

func TestValidateSimpleValueUnionStringFacets(t *testing.T) {
	t.Parallel()

	pattern := StringFacetValues{
		Patterns: []StringPatternGroup{{
			Patterns: []StringPattern{
				NewFastStringPattern("raw", CompileSimpleStringPattern("raw")),
			},
		}},
	}
	tests := []struct {
		enums   map[SimpleTypeID][]string
		name    string
		wantErr string
		strings StringFacetValues
		facets  FacetMask
	}{
		{name: "pattern match", facets: FacetPattern, strings: pattern},
		{name: "pattern mask without patterns", facets: FacetPattern, wantErr: ErrSimpleValueMetadata.Error()},
		{name: "enumeration match", facets: FacetEnumeration, strings: StringFacetValues{HasEnumeration: true}, enums: map[SimpleTypeID][]string{1: {"raw"}}},
		{name: "enumeration mismatch", facets: FacetEnumeration, strings: StringFacetValues{HasEnumeration: true}, enums: map[SimpleTypeID][]string{1: {"other"}}, wantErr: "enumeration facet failed"},
		{name: "enumeration mask without values", facets: FacetEnumeration, wantErr: ErrSimpleValueMetadata.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &simpleValueCallbackStub{
				types: map[SimpleTypeID]SimpleValueType{
					1: {StringFacets: tt.strings, UnionMembers: []SimpleTypeID{2}, Facets: tt.facets, Variety: SimpleVarietyUnion, Whitespace: WhitespaceCollapse},
					2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveQName},
				},
				enums: tt.enums,
			}
			_, err := ValidateSimpleValue(stub.callbacks(), 1, " raw ", 0)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleValue() error = %v", err)
				}
				if len(stub.calls) != 0 {
					t.Fatalf("calls = %v, want no edge callbacks", stub.calls)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleValue() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleValueStringEnumerationNeedsCallback(t *testing.T) {
	t.Parallel()

	cb := SimpleValueCallbacks{
		Type: func(id SimpleTypeID) (SimpleValueType, bool) {
			if id == 2 {
				return SimpleValueType{Variety: SimpleVarietyAtomic}, true
			}
			return SimpleValueType{
				StringFacets: StringFacetValues{HasEnumeration: true},
				ListItem:     2,
				Facets:       FacetEnumeration,
				Variety:      SimpleVarietyList,
			}, true
		},
		Facets: func(id SimpleTypeID) (SimpleValueFacets, bool) {
			return SimpleValueFacets{}, true
		},
	}
	_, err := ValidateSimpleValue(cb, 1, "a b", 0)
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateSimpleValue() error = %v, want metadata sentinel", err)
	}
}

func TestValidateSimpleValueUnionReturnsUnsupportedWhenNoMemberMatches(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {UnionMembers: []SimpleTypeID{2, 3}, Variety: SimpleVarietyUnion},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveString, Builtin: BuiltinValidationEntity},
			3: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveBoolean},
		},
		unsupportedFunc: func(err error) bool {
			return strings.Contains(err.Error(), "unsupported.entity")
		},
	}
	_, err := ValidateSimpleValue(stub.callbacks(), 1, "raw", 0)
	if err == nil || !strings.Contains(err.Error(), "unsupported.entity") {
		t.Fatalf("ValidateSimpleValue() error = %v, want unsupported ENTITY", err)
	}
}

func TestValidateSimpleValueUnionMismatch(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {UnionMembers: []SimpleTypeID{2}, Variety: SimpleVarietyUnion},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveBoolean},
		},
	}
	_, err := ValidateSimpleValue(stub.callbacks(), 1, "raw", 0)
	if err == nil || err.Error() != "value does not match any union member" {
		t.Fatalf("ValidateSimpleValue() error = %v", err)
	}
}

func TestValidateSimpleValueUnionWithoutMembersIsMetadataError(t *testing.T) {
	t.Parallel()

	stub := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyUnion},
		},
	}
	_, err := ValidateSimpleValue(stub.callbacks(), 1, "raw", 0)
	if !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ValidateSimpleValue() error = %v, want metadata sentinel", err)
	}
}
