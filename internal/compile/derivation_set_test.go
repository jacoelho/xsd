package compile

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestParseDerivationSet(t *testing.T) {
	t.Parallel()

	allowed := runtime.DerivationExtension | runtime.DerivationRestriction
	tests := []struct {
		name        string
		value       string
		label       string
		allowed     runtime.DerivationMask
		want        runtime.DerivationMask
		wantMessage string
	}{
		{
			name:    "empty",
			label:   "complexType final",
			allowed: allowed,
		},
		{
			name:    "explicit tokens",
			value:   "extension restriction",
			label:   "complexType final",
			allowed: allowed,
			want:    allowed,
		},
		{
			name:    "all",
			value:   "#all",
			label:   "complexType final",
			allowed: allowed,
			want:    allowed,
		},
		{
			name:        "all combination",
			value:       "#all extension",
			label:       "complexType final",
			allowed:     allowed,
			wantMessage: "complexType final cannot combine #all with other values",
		},
		{
			name:        "disallowed token",
			value:       "list",
			label:       "complexType final",
			allowed:     allowed,
			wantMessage: "complexType final cannot contain list",
		},
		{
			name:        "invalid token",
			value:       "bad",
			label:       "complexType final",
			allowed:     allowed,
			wantMessage: "invalid complexType final value bad",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseDerivationSet(tt.value, tt.label, tt.allowed)
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ParseDerivationSet() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseDerivationSet() = %08b, want %08b", got, tt.want)
			}
		})
	}
}

func TestParseDerivationAttrWithDefault(t *testing.T) {
	t.Parallel()

	def := runtime.DerivationBlockDefaultMask
	got, err := ParseDerivationAttrWithDefault("", false, def, ComplexTypeBlockDerivation)
	if err != nil {
		t.Fatalf("ParseDerivationAttrWithDefault(absent) error = %v", err)
	}
	want := runtime.DerivationExtension | runtime.DerivationRestriction
	if got != want {
		t.Fatalf("ParseDerivationAttrWithDefault(absent) = %08b, want %08b", got, want)
	}

	got, err = ParseDerivationAttrWithDefault("extension", true, def, ComplexTypeBlockDerivation)
	if err != nil {
		t.Fatalf("ParseDerivationAttrWithDefault(present) error = %v", err)
	}
	if got != runtime.DerivationExtension {
		t.Fatalf("ParseDerivationAttrWithDefault(present) = %08b, want extension", got)
	}

	_, err = ParseDerivationAttrWithDefault("list", true, def, ComplexTypeBlockDerivation)
	expectInvalidAttributeMessage(t, err, "complexType block cannot contain list")
}
