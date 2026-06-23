package runtime

import (
	"errors"
	"testing"
)

func TestParseTextValueStringCanonicalAndLength(t *testing.T) {
	t.Parallel()

	got, err := ParseTextValue(PrimitiveString, "a\u00e9", PrimitiveNeedCanonical|PrimitiveNeedLength)
	if err != nil {
		t.Fatalf("ParseTextValue() error = %v", err)
	}
	if got.Canonical != "a\u00e9" || got.Length != 2 {
		t.Fatalf("ParseTextValue() = %+v, want canonical=%q length=2", got, "a\u00e9")
	}

	length, err := PrimitiveLength(PrimitiveString, "a\u00e9")
	if err != nil {
		t.Fatalf("PrimitiveLength() error = %v", err)
	}
	if length != got.Length {
		t.Fatalf("PrimitiveLength() = %d, want %d", length, got.Length)
	}
}

func TestParseTextValueRejectsUnsupportedPrimitive(t *testing.T) {
	t.Parallel()

	if _, err := ParseTextValue(PrimitiveBoolean, "true", PrimitiveNeedCanonical); !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("ParseTextValue() error = %v, want metadata sentinel", err)
	}
	if _, err := PrimitiveLength(PrimitiveBoolean, "true"); !errors.Is(err, ErrSimpleValueMetadata) {
		t.Fatalf("PrimitiveLength() error = %v, want metadata sentinel", err)
	}
}
