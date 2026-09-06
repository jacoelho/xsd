package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
)

func TestParseDerivationSet(t *testing.T) {
	t.Parallel()

	allowed := DerivationExtension | DerivationRestriction
	tests := []struct {
		name        string
		value       string
		label       string
		allowed     DerivationMask
		want        DerivationMask
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
			name:        "non XML whitespace is not a separator",
			value:       "extension\u00a0restriction",
			label:       "complexType final",
			allowed:     allowed,
			wantMessage: "invalid complexType final value extension\u00a0restriction",
		},
		{
			name:        "repeated all",
			value:       "#all #all",
			label:       "complexType final",
			allowed:     allowed,
			wantMessage: "complexType final cannot combine #all with other values",
		},
		{
			name:    "duplicate token is idempotent",
			value:   vocab.XSDElemExtension + " " + vocab.XSDElemExtension,
			label:   "complexType final",
			allowed: allowed,
			want:    DerivationExtension,
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

	def := DerivationBlockDefaultMask
	got, err := ParseDerivationAttrWithDefault(LexicalAttribute{}, def, complexTypeBlockDerivation())
	if err != nil {
		t.Fatalf("ParseDerivationAttrWithDefault(absent) error = %v", err)
	}
	want := DerivationExtension | DerivationRestriction
	if got != want {
		t.Fatalf("ParseDerivationAttrWithDefault(absent) = %08b, want %08b", got, want)
	}

	got, err = ParseDerivationAttrWithDefault(LexicalAttribute{Value: "extension", Present: true}, def, complexTypeBlockDerivation())
	if err != nil {
		t.Fatalf("ParseDerivationAttrWithDefault(present) error = %v", err)
	}
	if got != DerivationExtension {
		t.Fatalf("ParseDerivationAttrWithDefault(present) = %08b, want extension", got)
	}

	_, err = ParseDerivationAttrWithDefault(LexicalAttribute{Value: "list", Present: true}, def, complexTypeBlockDerivation())
	expectInvalidAttributeMessage(t, err, "complexType block cannot contain list")
}

func TestDerivationRulesReturnIndependentValues(t *testing.T) {
	first := complexTypeBlockDerivation()
	want := first
	first.Name = "poison"
	first.Label = "poison"
	first.Allowed = 0

	if got := complexTypeBlockDerivation(); got != want {
		t.Fatalf("complexTypeBlockDerivation() = %#v, want %#v", got, want)
	}
}
