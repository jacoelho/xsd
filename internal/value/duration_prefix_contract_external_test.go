package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestDurationPrefixMalformedInputs(t *testing.T) {
	for _, input := range []string{"-", "-P", "-PT"} {
		t.Run(input, func(t *testing.T) {
			if err := value.ValidateDurationLexical(input); err == nil || err.Error() != "invalid duration" {
				t.Errorf("string %q: expected invalid duration, got %v", input, err)
			}
			if err := value.ValidateDurationLexical([]byte(input)); err == nil || err.Error() != "invalid duration" {
				t.Errorf("bytes %q: expected invalid duration, got %v", input, err)
			}
		})
	}
}

func TestDurationCrossSignFractionOrder(t *testing.T) {
	negative, err := value.ParseDurationValue("-PT0.5S")
	if err != nil {
		t.Fatal(err)
	}
	positive, err := value.ParseDurationValue("PT0.5S")
	if err != nil {
		t.Fatal(err)
	}
	if got := value.CompareDurationValues(negative, positive); got != value.OrderedFacetLess {
		t.Errorf("negative versus positive half-second: got %v, want Less", got)
	}
	if got := value.CompareDurationValues(positive, negative); got != value.OrderedFacetGreater {
		t.Errorf("positive versus negative half-second: got %v, want Greater", got)
	}
	if value.EqualDurationValues(negative, positive) {
		t.Error("negative and positive half-second durations must differ")
	}
}
