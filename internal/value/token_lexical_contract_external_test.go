package value_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

type tokenString string
type tokenBytes []byte

func TestPrimitiveTokenLexicalContracts(t *testing.T) {
	t.Run("string", func(t *testing.T) { checkPrimitiveTokens[string](t) })
	t.Run("bytes", func(t *testing.T) { checkPrimitiveTokens[[]byte](t) })
	t.Run("named string", func(t *testing.T) { checkPrimitiveTokens[tokenString](t) })
	t.Run("named bytes", func(t *testing.T) { checkPrimitiveTokens[tokenBytes](t) })
}

func checkPrimitiveTokens[T ~string | ~[]byte](t *testing.T) {
	t.Helper()
	var empty T
	if value.BooleanLexicalOK(empty) || value.ValidateBooleanLexical(empty) == nil || value.ValidateFloatLexical(empty, 64) == nil {
		t.Fatal("accepted zero-value input")
	}
	for _, tc := range []struct {
		text string
		want bool
	}{{"true", true}, {"false", false}, {"1", true}, {"0", false}} {
		raw := T(tc.text)
		got, err := value.ParseBooleanValue(raw)
		if err != nil || got != tc.want || !value.BooleanLexicalOK(raw) || value.ValidateBooleanLexical(raw) != nil {
			t.Fatalf("boolean %q = %t, %v; want %t", tc.text, got, err, tc.want)
		}
	}
	for _, text := range []string{"", "trux", "falsx", "True", "true\x00", "tru\xff", "trué", strings.Repeat("x", 4096)} {
		raw := T(text)
		got, err := value.ParseBooleanValue(raw)
		validation := value.ValidateBooleanLexical(raw)
		if got || err == nil || err.Error() != "invalid boolean" || value.BooleanLexicalOK(raw) || validation == nil || validation.Error() != "invalid boolean" {
			t.Fatalf("invalid boolean %q = %t, %v, %v", text, got, err, validation)
		}
	}
	for _, bits := range []int{32, 64} {
		for _, text := range []string{"INF", "-INF", "NaN", ".1", "1.", "1e+2", "1e400", "1e-400"} {
			if err := value.ValidateFloatLexical(T(text), bits); err != nil {
				t.Fatalf("float%d %q: %v", bits, text, err)
			}
		}
		for _, text := range []string{"", "+INF", "nan", "INX", "-INX", "INF\x00", "Na\xff", "Naé", "1e", "1e+", strings.Repeat("x", 4096)} {
			if err := value.ValidateFloatLexical(T(text), bits); err == nil || err.Error() != "invalid float" {
				t.Fatalf("invalid float%d %q: %v", bits, text, err)
			}
		}
	}
}

func TestRawPrimitiveTokenValidationReuse(t *testing.T) {
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	var scratch value.Scratch
	for _, tc := range []struct {
		kind    value.PrimitiveKind
		valid   []string
		invalid string
	}{{value.PrimitiveBoolean, []string{"true", "false", "1", "0"}, "trux"},
		{value.PrimitiveFloat, []string{"INF", "-INF", "NaN"}, "INX"},
		{value.PrimitiveDouble, []string{"INF", "-INF", "NaN"}, "INX"}} {
		id := value.BuiltinType(tc.kind)
		if _, err := program.ValidateBytes(id, []byte(tc.invalid), value.Resolver{}, 0, 16<<20, &scratch); err == nil {
			t.Fatalf("accepted invalid primitive %d", tc.kind)
		}
		for _, text := range tc.valid {
			got, err := program.ValidateBytes(id, []byte(text), value.Resolver{}, 0, 16<<20, &scratch)
			if err != nil || got.Type() != id || got.SelectedType() != value.NoType || got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
				t.Fatalf("primitive %d %q: type/selected/canonical %d/%d/%q, %v", tc.kind, text, got.Type(), got.SelectedType(), got.CanonicalText(), err)
			}
		}
	}
}
