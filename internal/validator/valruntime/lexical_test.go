package valruntime

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestValidateAtomic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		kind        runtime.ValidatorKind
		stringKind  runtime.StringKind
		integerKind runtime.IntegerKind
		input       []byte
		wantErr     string
	}{
		{
			name:       "string ncname",
			kind:       runtime.VString,
			stringKind: runtime.StringNCName,
			input:      []byte("bad:name"),
			wantErr:    "NCName cannot contain colons",
		},
		{
			name:        "integer positive",
			kind:        runtime.VInteger,
			integerKind: runtime.IntegerPositive,
			input:       []byte("0"),
			wantErr:     "positiveInteger must be >= 1",
		},
		{
			name:    "invalid float",
			kind:    runtime.VFloat,
			input:   []byte("1e"),
			wantErr: "invalid float",
		},
		{
			name:    "invalid double",
			kind:    runtime.VDouble,
			input:   []byte("1e"),
			wantErr: "invalid double",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAtomic(tc.kind, tc.stringKind, tc.integerKind, tc.input)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidateTemporal(t *testing.T) {
	t.Parallel()

	if err := ValidateTemporal(runtime.VDate, []byte("2024-01-02")); err != nil {
		t.Fatalf("validate date: %v", err)
	}
	if err := ValidateTemporal(runtime.VDate, []byte("bad-date")); err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestParseTemporalUnsupportedKind(t *testing.T) {
	t.Parallel()

	_, err := ParseTemporal(runtime.VString, []byte("x"))
	if err == nil {
		t.Fatal("expected unsupported temporal kind error")
	}
	if err.Error() != "unsupported temporal kind 0" {
		t.Fatalf("error = %q, want %q", err.Error(), "unsupported temporal kind 0")
	}
}

func TestValidateBinaryAndURI(t *testing.T) {
	t.Parallel()

	if err := ValidateAnyURI([]byte("https://example.com/a")); err != nil {
		t.Fatalf("validate anyURI: %v", err)
	}
	if err := ValidateHexBinary([]byte("0G")); err == nil {
		t.Fatal("expected invalid hexBinary error")
	}
	if err := ValidateBase64Binary([]byte("%%%")); err == nil {
		t.Fatal("expected invalid base64Binary error")
	}
}
