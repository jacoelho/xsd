package runtime

import (
	"errors"
	"testing"
)

func TestValidateFastDecimalLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantErr     string
		shape       RawDecimalFastPathShape
		wantHandled bool
		wantMetaErr bool
	}{
		{
			name:        "valid unbounded decimal",
			input:       "+000.0100",
			wantHandled: true,
		},
		{
			name:        "invalid lexical",
			input:       ".",
			wantHandled: true,
			wantErr:     fastDecimalErrInvalid,
		},
		{
			name: "inclusive bounds accept equal normalized value",
			shape: RawDecimalFastPathShape{
				MinInclusive: RawDecimalBound{Present: true, Int: "1"},
				MaxInclusive: RawDecimalBound{Present: true, Int: "10", Frac: "5"},
				Facets:       FacetMinInclusive | FacetMaxInclusive,
			},
			input:       "10.50",
			wantHandled: true,
		},
		{
			name: "minInclusive failure",
			shape: RawDecimalFastPathShape{
				MinInclusive: RawDecimalBound{Present: true, Int: "0", Frac: "01"},
				Facets:       FacetMinInclusive,
			},
			input:       "0.009",
			wantHandled: true,
			wantErr:     fastDecimalErrMinInclusive,
		},
		{
			name: "maxInclusive failure",
			shape: RawDecimalFastPathShape{
				MaxInclusive: RawDecimalBound{Present: true, Int: "10", Frac: "5"},
				Facets:       FacetMaxInclusive,
			},
			input:       "10.51",
			wantHandled: true,
			wantErr:     fastDecimalErrMaxInclusive,
		},
		{
			name: "negative non-zero with non-negative minInclusive fails",
			shape: RawDecimalFastPathShape{
				MinInclusive: RawDecimalBound{Present: true, Int: "0"},
				Facets:       FacetMinInclusive,
			},
			input:       "-0.1",
			wantHandled: true,
			wantErr:     fastDecimalErrMinInclusive,
		},
		{
			name: "negative zero is non-negative",
			shape: RawDecimalFastPathShape{
				MinInclusive: RawDecimalBound{Present: true, Int: "0"},
				Facets:       FacetMinInclusive,
			},
			input:       "-0.0",
			wantHandled: true,
		},
		{
			name: "unsupported totalDigits falls back",
			shape: RawDecimalFastPathShape{
				Facets: FacetTotalDigits,
			},
			input: "1",
		},
		{
			name: "negative minInclusive falls back",
			shape: RawDecimalFastPathShape{
				MinInclusive: RawDecimalBound{Present: true, Negative: true},
				Facets:       FacetMinInclusive,
			},
			input: "1",
		},
		{
			name: "missing projected bound is metadata error",
			shape: RawDecimalFastPathShape{
				Facets: FacetMinInclusive,
			},
			input:       "1",
			wantMetaErr: true,
		},
		{
			name: "invalid projected bound is metadata error",
			shape: RawDecimalFastPathShape{
				MinInclusive: RawDecimalBound{Present: true, Int: "x"},
				Facets:       FacetMinInclusive,
			},
			input:       "1",
			wantMetaErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handled, err := ValidateFastDecimalLexical(tt.shape, []byte(tt.input))
			if handled != tt.wantHandled {
				t.Fatalf("ValidateFastDecimalLexical() handled = %v, want %v", handled, tt.wantHandled)
			}
			stringHandled, stringErr := ValidateFastDecimalLexical(tt.shape, tt.input)
			if stringHandled != handled || errorMessage(stringErr) != errorMessage(err) {
				t.Fatalf("ValidateFastDecimalLexical string = (%v, %v), bytes = (%v, %v)", stringHandled, stringErr, handled, err)
			}
			if tt.wantMetaErr {
				if !errors.Is(err, ErrSimpleValueMetadata) {
					t.Fatalf("ValidateFastDecimalLexical() error = %v, want metadata sentinel", err)
				}
				return
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateFastDecimalLexical() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateFastDecimalLexical() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
