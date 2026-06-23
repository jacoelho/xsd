package runtime

import "testing"

func TestValidateHexBinaryLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "empty"},
		{name: "valid lowercase", input: "0a2f"},
		{name: "valid uppercase", input: "0A2F"},
		{name: "rejects odd length", input: "0af", wantErr: "invalid hexBinary"},
		{name: "rejects non hex", input: "0g", wantErr: "invalid hexBinary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bytesErr := ValidateHexBinaryLexical([]byte(tt.input))
			if got := errorMessage(bytesErr); got != tt.wantErr {
				t.Fatalf("ValidateHexBinaryLexical() error = %q, want %q", got, tt.wantErr)
			}
			stringErr := ValidateHexBinaryLexical(tt.input)
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("ValidateHexBinaryLexical string error for %q = %v, bytes error = %v", tt.input, stringErr, bytesErr)
			}
		})
	}
}

func TestValidateBase64BinaryLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "empty"},
		{name: "no padding", input: "AQID"},
		{name: "xml whitespace", input: "A Q\r\nI\tD"},
		{name: "one padding", input: "AQI="},
		{name: "two padding", input: "AQ=="},
		{name: "invalid length", input: "AQI", wantErr: "invalid base64Binary"},
		{name: "bad character", input: "AQ$D", wantErr: "invalid base64Binary"},
		{name: "data after padding", input: "AQ=I", wantErr: "invalid base64Binary"},
		{name: "too much padding", input: "A===", wantErr: "invalid base64Binary"},
		{name: "non-zero one-pad bits", input: "AQJ=", wantErr: "invalid base64Binary"},
		{name: "non-zero two-pad bits", input: "AB==", wantErr: "invalid base64Binary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bytesErr := ValidateBase64BinaryLexical([]byte(tt.input))
			if got := errorMessage(bytesErr); got != tt.wantErr {
				t.Fatalf("ValidateBase64BinaryLexical() error = %q, want %q", got, tt.wantErr)
			}
			stringErr := ValidateBase64BinaryLexical(tt.input)
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("ValidateBase64BinaryLexical string error for %q = %v, bytes error = %v", tt.input, stringErr, bytesErr)
			}
		})
	}
}

func TestParseBinaryValueCanonicalAndLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		kind      PrimitiveKind
		input     string
		needs     PrimitiveValueNeed
		canonical string
		length    uint32
		wantErr   string
	}{
		{
			name:      "hex canonical uppercase",
			kind:      PrimitiveHexBinary,
			input:     "0aff",
			needs:     PrimitiveNeedCanonical | PrimitiveNeedLength,
			canonical: "0AFF",
			length:    2,
		},
		{
			name:      "base64 canonical strips xml whitespace",
			kind:      PrimitiveBase64Binary,
			input:     "A Q I =",
			needs:     PrimitiveNeedCanonical | PrimitiveNeedLength,
			canonical: "AQI=",
			length:    2,
		},
		{
			name:    "invalid hex",
			kind:    PrimitiveHexBinary,
			input:   "0g",
			needs:   PrimitiveNeedCanonical | PrimitiveNeedLength,
			wantErr: "invalid hexBinary",
		},
		{
			name:    "invalid base64",
			kind:    PrimitiveBase64Binary,
			input:   "AB==",
			needs:   PrimitiveNeedCanonical | PrimitiveNeedLength,
			wantErr: "invalid base64Binary",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseBinaryValue(tt.kind, tt.input, tt.needs)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("ParseBinaryValue() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseBinaryValue() error = %v", err)
			}
			if got.Canonical != tt.canonical || got.Length != tt.length {
				t.Fatalf("ParseBinaryValue() = %+v, want canonical=%q length=%d", got, tt.canonical, tt.length)
			}
		})
	}
}

func TestBinaryLengthMatchesBase64ValueLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		length  uint32
		wantErr string
	}{
		{name: "empty"},
		{name: "no padding", value: "AQID", length: 3},
		{name: "xml whitespace", value: "A Q\r\nI\tD", length: 3},
		{name: "one padding", value: "AQI=", length: 2},
		{name: "two padding", value: "AQ==", length: 1},
		{name: "invalid length", value: "AQI", wantErr: "invalid base64Binary"},
		{name: "bad character", value: "AQ$D", wantErr: "invalid base64Binary"},
		{name: "data after padding", value: "AQ=I", wantErr: "invalid base64Binary"},
		{name: "too much padding", value: "A===", wantErr: "invalid base64Binary"},
		{name: "non-zero one-pad bits", value: "AQJ=", wantErr: "invalid base64Binary"},
		{name: "non-zero two-pad bits", value: "AB==", wantErr: "invalid base64Binary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			length, lengthErr := BinaryLength(PrimitiveBase64Binary, tt.value)
			value, valueErr := ParseBinaryValue(PrimitiveBase64Binary, tt.value, PrimitiveNeedCanonical|PrimitiveNeedLength)
			if tt.wantErr != "" {
				if lengthErr == nil || lengthErr.Error() != tt.wantErr {
					t.Fatalf("BinaryLength() error = %v, want %q", lengthErr, tt.wantErr)
				}
				if valueErr == nil || valueErr.Error() != tt.wantErr {
					t.Fatalf("ParseBinaryValue() error = %v, want %q", valueErr, tt.wantErr)
				}
				return
			}
			if lengthErr != nil {
				t.Fatalf("BinaryLength() error = %v", lengthErr)
			}
			if valueErr != nil {
				t.Fatalf("ParseBinaryValue() error = %v", valueErr)
			}
			if length != tt.length {
				t.Fatalf("BinaryLength() = %d, want %d", length, tt.length)
			}
			if value.Length != tt.length {
				t.Fatalf("ParseBinaryValue().Length = %d, want %d", value.Length, tt.length)
			}
		})
	}
}
