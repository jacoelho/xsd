package runtime

import "testing"

func TestParseTextValueAnyURICanonicalAndLength(t *testing.T) {
	t.Parallel()

	got, err := ParseTextValue(PrimitiveAnyURI, "https://example.test/a\u00e9", PrimitiveNeedCanonical|PrimitiveNeedLength)
	if err != nil {
		t.Fatalf("ParseTextValue() error = %v", err)
	}
	if got.Canonical != "https://example.test/a\u00e9" || got.Length != 23 {
		t.Fatalf("ParseTextValue() = %+v, want canonical=%q length=23", got, "https://example.test/a\u00e9")
	}

	length, err := PrimitiveLength(PrimitiveAnyURI, "https://example.test/a\u00e9")
	if err != nil {
		t.Fatalf("PrimitiveLength() error = %v", err)
	}
	if length != got.Length {
		t.Fatalf("PrimitiveLength() = %d, want %d", length, got.Length)
	}

	for _, input := range []string{":a", "%"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseTextValue(PrimitiveAnyURI, input, PrimitiveNeedCanonical|PrimitiveNeedLength); err == nil || err.Error() != "invalid anyURI" {
				t.Fatalf("ParseTextValue(%q) error = %v, want invalid anyURI", input, err)
			}
			if _, err := PrimitiveLength(PrimitiveAnyURI, input); err == nil || err.Error() != "invalid anyURI" {
				t.Fatalf("PrimitiveLength(%q) error = %v, want invalid anyURI", input, err)
			}
		})
	}
}

func TestAnyURIRawSpellingDefinesValueIdentityAndEnumeration(t *testing.T) {
	t.Parallel()
	plain := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveAnyURI, Whitespace: WhitespaceCollapse},
		},
	}
	space, err := ValidateSimpleValue(plain.callbacks(), 1, "a b", SimpleNeedCanonical|SimpleNeedIdentity)
	if err != nil {
		t.Fatal(err)
	}
	escaped, err := ValidateSimpleValue(plain.callbacks(), 1, "a%20b", SimpleNeedCanonical|SimpleNeedIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if space.Canonical != "a b" || escaped.Canonical != "a%20b" || space.Identity == escaped.Identity {
		t.Fatalf("anyURI values = %+v and %+v, want distinct raw canonical identities", space, escaped)
	}

	restricted := &simpleValueCallbackStub{
		types: map[SimpleTypeID]SimpleValueType{
			1: {
				StringFacets: StringFacetValues{HasEnumeration: true},
				Facets:       FacetEnumeration, Variety: SimpleVarietyAtomic,
				Primitive: PrimitiveAnyURI, Whitespace: WhitespaceCollapse,
			},
		},
		facets: map[SimpleTypeID]SimpleValueFacets{
			1: {
				Facets:       FacetEnumeration,
				StringFacets: StringFacetValues{HasEnumeration: true},
				Enumeration:  []SimpleValueFacetLiteral{{Canonical: "a b"}},
			},
		},
	}
	if _, err := ValidateSimpleValue(restricted.callbacks(), 1, "a b", 0); err != nil {
		t.Fatalf("ValidateSimpleValue(enumeration match) error = %v", err)
	}
	if _, err := ValidateSimpleValue(restricted.callbacks(), 1, "a%20b", 0); err == nil || err.Error() != "enumeration facet failed" {
		t.Fatalf("ValidateSimpleValue(escaped spelling) error = %v, want enumeration failure", err)
	}
}
