package runtime

import "testing"

func TestValidateAnyURILexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "empty"},
		{name: "plain", input: "https://example.test/a%20b"},
		{name: "rejects leading colon", input: ":a", wantErr: "invalid anyURI"},
		{name: "rejects trailing colon", input: "a:", wantErr: "invalid anyURI"},
		{name: "rejects incomplete escape", input: "%", wantErr: "invalid anyURI"},
		{name: "rejects bad escape", input: "%xz", wantErr: "invalid anyURI"},
		{name: "rejects caret", input: "a^b", wantErr: "invalid anyURI"},
		{name: "rejects backslash", input: `a\b`, wantErr: "invalid anyURI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bytesErr := ValidateAnyURILexical([]byte(tt.input))
			if got := errorMessage(bytesErr); got != tt.wantErr {
				t.Fatalf("ValidateAnyURILexical() error = %q, want %q", got, tt.wantErr)
			}
			stringErr := ValidateAnyURILexical(tt.input)
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("ValidateAnyURILexical string error for %q = %v, bytes error = %v", tt.input, stringErr, bytesErr)
			}
		})
	}
}

func TestParseTextValueAnyURICanonicalAndLength(t *testing.T) {
	t.Parallel()

	got, err := ParseTextValue(PrimitiveAnyURI, "https://example.test/a\u00e9", PrimitiveNeedCanonical|PrimitiveNeedLength)
	if err != nil {
		t.Fatalf("ParseTextValue() error = %v", err)
	}
	if got.Canonical != "https://example.test/a\u00e9" || got.Length != 23 {
		t.Fatalf("ParseTextValue() = %+v, want canonical=%q length=23", got, "https://example.test/a\u00e9")
	}

	length, err := PrimitiveLength(PrimitiveAnyURI, "https://example.test/a\u00e9")
	if err != nil {
		t.Fatalf("PrimitiveLength() error = %v", err)
	}
	if length != got.Length {
		t.Fatalf("PrimitiveLength() = %d, want %d", length, got.Length)
	}

	for _, input := range []string{":a", "%"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseTextValue(PrimitiveAnyURI, input, PrimitiveNeedCanonical|PrimitiveNeedLength); err == nil || err.Error() != "invalid anyURI" {
				t.Fatalf("ParseTextValue(%q) error = %v, want invalid anyURI", input, err)
			}
			if _, err := PrimitiveLength(PrimitiveAnyURI, input); err == nil || err.Error() != "invalid anyURI" {
				t.Fatalf("PrimitiveLength(%q) error = %v, want invalid anyURI", input, err)
			}
		})
	}
}
