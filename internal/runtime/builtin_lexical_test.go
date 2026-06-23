package runtime

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateBuiltinDerived(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		in      BuiltinDerivedInput
	}{
		{
			name: "none",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationNone,
			},
		},
		{
			name: "integer lexical",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationInteger,
				Norm: "1",
			},
		},
		{
			name: "integer rejects decimal lexical",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationInteger,
				Norm: "1.0",
			},
			wantErr: "invalid integer",
		},
		{
			name: "integer preserves decimal lexical diagnostic",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationInteger,
				Norm: "abc",
			},
			wantErr: "invalid decimal",
		},
		{
			name: "Name",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationName,
				Norm: "p:name",
			},
		},
		{
			name: "Name rejects bad name",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationName,
				Norm: "1bad",
			},
			wantErr: "invalid Name",
		},
		{
			name: "NCName",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationNCName,
				Norm: "name",
			},
		},
		{
			name: "NCName rejects colon",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationNCName,
				Norm: "p:name",
			},
			wantErr: "invalid NCName",
		},
		{
			name: "NMTOKEN",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationNMTOKEN,
				Norm: "a.b",
			},
		},
		{
			name: "language",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationLanguage,
				Norm: "en-US",
			},
		},
		{
			name: "xml lang allows empty",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationXMLLang,
			},
		},
		{
			name: "xml space",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationXMLSpace,
				Norm: "preserve",
			},
		},
		{
			name: "xml space rejects other values",
			in: BuiltinDerivedInput{
				Kind: BuiltinValidationXMLSpace,
				Norm: "collapse",
			},
			wantErr: "invalid xml:space",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateBuiltinDerived(tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateBuiltinDerived() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateBuiltinDerived() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateIntegerLexicalStringAndBytesMatch(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"+",
		"-",
		".",
		"0",
		"+1",
		"-0",
		"2147483648",
		"123456789012345678901234567890",
		"1.0",
		"5.",
		".5",
		"-0.0",
		"1.2.3",
		"12a",
		"1e2",
	}
	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			t.Parallel()

			stringErr := ValidateIntegerLexical(test)
			bytesErr := ValidateIntegerLexical([]byte(test))
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("ValidateIntegerLexical string error for %q = %v, bytes error = %v", test, stringErr, bytesErr)
			}
		})
	}
}

func TestValidateFastIntLexicalStringAndBytesMatch(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"+",
		"-",
		"0",
		"+000",
		"2147483647",
		"2147483648",
		"-2147483648",
		"-2147483649",
		"1.0",
		"1..0",
		"x",
	}
	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			t.Parallel()

			stringErr := ValidateFastIntLexical(test)
			bytesErr := ValidateFastIntLexical([]byte(test))
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("ValidateFastIntLexical string error for %q = %v, bytes error = %v", test, stringErr, bytesErr)
			}
		})
	}
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestValidateBuiltinDerivedEntityIsUnsupported(t *testing.T) {
	t.Parallel()

	err := ValidateBuiltinDerived(BuiltinDerivedInput{
		Kind: BuiltinValidationEntity,
		Norm: "entity",
	})
	if err == nil {
		t.Fatal("ValidateBuiltinDerived() error = nil, want unsupported ENTITY")
	}
	if !xsderrors.IsUnsupported(err) {
		t.Fatalf("ValidateBuiltinDerived() unsupported = false for %v", err)
	}
	if !strings.Contains(err.Error(), "ENTITY requires DTD entity declarations") {
		t.Fatalf("ValidateBuiltinDerived() error = %v, want ENTITY diagnostic", err)
	}
}
