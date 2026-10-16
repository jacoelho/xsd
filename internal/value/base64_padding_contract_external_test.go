package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestBase64PaddingAdmissionContract(t *testing.T) {
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	id := value.BuiltinType(value.PrimitiveBase64Binary)
	for _, tc := range []struct {
		name, lexical, canonical, message string
		length                            uint32
	}{
		{name: "one padding low bits", lexical: "YWF=", message: "invalid base64Binary"},
		{name: "excess padding", lexical: "A===", message: "invalid base64Binary"},
		{name: "whitespace between pads", lexical: "YQ= \t=", canonical: "YQ==", length: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, parseErr := value.ParseBinaryValue(value.PrimitiveBase64Binary, tc.lexical, value.PrimitiveNeedCanonical)
			if tc.message != "" {
				if parseErr == nil || parseErr.Error() != tc.message {
					t.Fatalf("ParseBinaryValue error = %v, want %q", parseErr, tc.message)
				}
			} else if parseErr != nil || parsed.Canonical != tc.canonical || parsed.Length != tc.length {
				t.Fatalf("ParseBinaryValue = %#v, %v; want canonical %q, length %d", parsed, parseErr, tc.canonical, tc.length)
			}
			for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
				for _, bytesInput := range []bool{false, true} {
					var got value.Value
					var validationErr error
					if bytesInput {
						got, validationErr = program.ValidateBytes(id, []byte(tc.lexical), value.Resolver{}, needs, 16<<20, nil)
					} else {
						got, validationErr = program.Validate(id, tc.lexical, value.Resolver{}, needs, 16<<20, nil)
					}
					if tc.message != "" {
						if validationErr == nil || validationErr.Error() != tc.message {
							t.Fatalf("bytes=%t needs=%d: error = %v, want %q", bytesInput, needs, validationErr, tc.message)
						}
						continue
					}
					if validationErr != nil {
						t.Fatal(validationErr)
					}
					if needs != 0 && (got.CanonicalText() != tc.canonical || got.IdentityKey() != "\x0f\x1eYQ==") {
						t.Fatalf("projections = %q, %q; want %q and base64 identity", got.CanonicalText(), got.IdentityKey(), tc.canonical)
					}
				}
			}
		})
	}
}
