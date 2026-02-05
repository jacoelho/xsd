package types

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
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

	for _, lexical := range valid {
		if err := value.ValidateQName([]byte(lexical)); err != nil {
			t.Fatalf("value.ValidateQName(%q) = %v, want nil", lexical, err)
		}
		if err := validateQName(lexical); err != nil {
			t.Fatalf("types.validateQName(%q) = %v, want nil", lexical, err)
		}
	}
	for _, lexical := range invalid {
		if err := value.ValidateQName([]byte(lexical)); err == nil {
			t.Fatalf("value.ValidateQName(%q) = nil, want error", lexical)
		}
		if err := validateQName(lexical); err == nil {
			t.Fatalf("types.validateQName(%q) = nil, want error", lexical)
		}
	}
}
