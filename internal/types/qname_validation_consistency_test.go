package types

import (
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func TestQNameValidationConsistency(t *testing.T) {
	valid := []string{
		"local",
		"p:local",
		"_a",
		"a-b",
		"a.b",
	}
	invalid := []string{
		"",
		":a",
		"a:",
		"a::b",
		"xmlns:local",
		"a b",
		"a\tb",
	}

	for _, value := range valid {
		if err := valuepkg.ValidateQName([]byte(value)); err != nil {
			t.Fatalf("value.ValidateQName(%q) = %v, want nil", value, err)
		}
		if err := validateQName(value); err != nil {
			t.Fatalf("types.validateQName(%q) = %v, want nil", value, err)
		}
	}
	for _, value := range invalid {
		if err := valuepkg.ValidateQName([]byte(value)); err == nil {
			t.Fatalf("value.ValidateQName(%q) = nil, want error", value)
		}
		if err := validateQName(value); err == nil {
			t.Fatalf("types.validateQName(%q) = nil, want error", value)
		}
	}
}
