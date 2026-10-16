package value_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestDecimalNormalizedFractionOrdering(t *testing.T) {
	prefix := strings.Repeat("7", 1024)
	for _, test := range []struct {
		name, left, right string
		want              int
	}{
		{"negative prefix", "-0.12", "-0.120001", 1},
		{"long late difference", "0." + prefix + "8", "0." + prefix + "9", -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			left, err := value.ParseDecimalValue(test.left)
			if err != nil {
				t.Fatal(err)
			}
			right, err := value.ParseDecimalValue(test.right)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.CompareDecimalValues(left, right); got != test.want {
				t.Fatalf("CompareDecimalValues(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
			}
		})
	}
}

func TestTemporalNormalizedFractionOrdering(t *testing.T) {
	for _, test := range []struct {
		name, left, right string
		want              value.OrderedFacetRelation
	}{
		{"trailing zeros", ".1200", ".12", value.OrderedFacetEqual},
		{"strict prefix", ".12", ".120001", value.OrderedFacetLess},
		{"zero fraction", ".0000", "", value.OrderedFacetEqual},
	} {
		t.Run(test.name, func(t *testing.T) {
			leftTime, err := value.ParseTimeValue("12:00:00" + test.left)
			if err != nil {
				t.Fatal(err)
			}
			rightTime, err := value.ParseTimeValue("12:00:00" + test.right)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.CompareTimePartial(leftTime, rightTime); got != test.want {
				t.Fatalf("CompareTimePartial() = %v, want %v", got, test.want)
			}
			leftDateTime, err := value.ParseDateTimeValue("2000-01-01T12:00:00" + test.left)
			if err != nil {
				t.Fatal(err)
			}
			rightDateTime, err := value.ParseDateTimeValue("2000-01-01T12:00:00" + test.right)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.CompareTemporalValues(leftDateTime.Temporal(), rightDateTime.Temporal()); got != test.want {
				t.Fatalf("CompareTemporalValues() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNegativeDurationFractionReferenceOrder(t *testing.T) {
	left, err := value.ParseDurationValue("-P1MT0.10S")
	if err != nil {
		t.Fatal(err)
	}
	right, err := value.ParseDurationValue("-P28DT0.1001S")
	if err != nil {
		t.Fatal(err)
	}
	if got := value.CompareDurationValues(left, right); got != value.OrderedFacetIncomparable {
		t.Fatalf("CompareDurationValues() = %v, want Incomparable", got)
	}
}
