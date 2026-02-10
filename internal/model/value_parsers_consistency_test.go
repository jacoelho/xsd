package model

import (
	"bytes"
	"math"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestParseDecimalMatchesValue(t *testing.T) {
	cases := []string{"1", " 1.0 ", "-0.5", "", "1.2.3"}
	for _, input := range cases {
		got, err := ParseDecimal(input)
		want, werr := value.ParseDecimal([]byte(input))
		if (err != nil) != (werr != nil) {
			t.Fatalf("ParseDecimal(%q) error mismatch: %v vs %v", input, err, werr)
		}
		if err != nil {
			continue
		}
		if !bytes.Equal(got.RenderCanonical(nil), want.RenderCanonical(nil)) {
			t.Fatalf("ParseDecimal(%q) = %q, want %q", input, got.RenderCanonical(nil), want.RenderCanonical(nil))
		}
	}
}

func TestParseIntegerMatchesValue(t *testing.T) {
	cases := []string{"0", "-1", " 10 ", "", "1.0"}
	for _, input := range cases {
		got, err := ParseInteger(input)
		want, werr := value.ParseInteger([]byte(input))
		if (err != nil) != (werr != nil) {
			t.Fatalf("ParseInteger(%q) error mismatch: %v vs %v", input, err, werr)
		}
		if err != nil {
			continue
		}
		if !bytes.Equal(got.RenderCanonical(nil), want.RenderCanonical(nil)) {
			t.Fatalf("ParseInteger(%q) = %q, want %q", input, got.RenderCanonical(nil), want.RenderCanonical(nil))
		}
	}
}

func TestParseFloatMatchesValue(t *testing.T) {
	cases := []string{"INF", "-INF", "NaN", "+INF", "1.25"}
	for _, input := range cases {
		got, err := ParseFloat(input)
		want, werr := value.ParseFloat([]byte(input))
		if (err != nil) != (werr != nil) {
			t.Fatalf("ParseFloat(%q) error mismatch: %v vs %v", input, err, werr)
		}
		if err != nil {
			continue
		}
		if !float32Equal(got, want) {
			t.Fatalf("ParseFloat(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParseDoubleMatchesValue(t *testing.T) {
	cases := []string{"INF", "-INF", "NaN", "+INF", "1.25"}
	for _, input := range cases {
		got, err := ParseDouble(input)
		want, werr := value.ParseDouble([]byte(input))
		if (err != nil) != (werr != nil) {
			t.Fatalf("ParseDouble(%q) error mismatch: %v vs %v", input, err, werr)
		}
		if err != nil {
			continue
		}
		if !float64Equal(got, want) {
			t.Fatalf("ParseDouble(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParseDateTimeMatchesValue(t *testing.T) {
	cases := []string{
		"2001-10-26T23:59:60Z",
		"2001-10-26T24:00:00Z",
		"2001-10-26T24:00:01Z",
	}
	for _, input := range cases {
		got, err := ParseDateTime(input)
		want, werr := value.ParseDateTime([]byte(input))
		if (err != nil) != (werr != nil) {
			t.Fatalf("ParseDateTime(%q) error mismatch: %v vs %v", input, err, werr)
		}
		if err != nil {
			continue
		}
		if !got.Equal(want) {
			t.Fatalf("ParseDateTime(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestValidateAnyURIMatchesValue(t *testing.T) {
	cases := []string{"http://example.com/%G1", "http://exa mple.com"}
	for _, input := range cases {
		err := validateAnyURI(input)
		werr := value.ValidateAnyURI([]byte(input))
		if (err != nil) != (werr != nil) {
			t.Fatalf("validateAnyURI(%q) error mismatch: %v vs %v", input, err, werr)
		}
	}
}

func TestParseAnyURIMatchesValue(t *testing.T) {
	cases := []string{"http://example.com", "http://exa mple.com"}
	for _, input := range cases {
		got, err := ParseAnyURI(input)
		want, werr := value.ParseAnyURI([]byte(input))
		if (err != nil) != (werr != nil) {
			t.Fatalf("ParseAnyURI(%q) error mismatch: %v vs %v", input, err, werr)
		}
		if err != nil {
			continue
		}
		if got != want {
			t.Fatalf("ParseAnyURI(%q) = %q, want %q", input, got, want)
		}
	}
}

func float32Equal(left, right float32) bool {
	if math.IsNaN(float64(left)) {
		return math.IsNaN(float64(right))
	}
	if math.IsInf(float64(left), 0) {
		return math.IsInf(float64(right), int(math.Copysign(1, float64(left))))
	}
	return left == right
}

func float64Equal(left, right float64) bool {
	if math.IsNaN(left) {
		return math.IsNaN(right)
	}
	if math.IsInf(left, 0) {
		return math.IsInf(right, int(math.Copysign(1, left)))
	}
	return left == right
}
