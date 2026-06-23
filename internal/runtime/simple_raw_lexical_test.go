package runtime

import "testing"

func TestValidateFastDateLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantErr     string
		wantHandled bool
	}{
		{
			name:        "valid common year",
			input:       "2001-02-28",
			wantHandled: true,
		},
		{
			name:        "valid leap day",
			input:       "2000-02-29",
			wantHandled: true,
		},
		{
			name:        "rejects non leap day",
			input:       "1900-02-29",
			wantHandled: true,
			wantErr:     fastDateErrInvalid,
		},
		{
			name:        "rejects zero year",
			input:       "0000-01-01",
			wantHandled: true,
			wantErr:     fastDateErrInvalid,
		},
		{
			name:        "rejects bad month",
			input:       "2001-13-01",
			wantHandled: true,
			wantErr:     fastDateErrInvalid,
		},
		{
			name:        "rejects bad digit",
			input:       "200x-01-01",
			wantHandled: true,
			wantErr:     fastDateErrInvalid,
		},
		{
			name:  "timezone form falls back",
			input: "2000-02-29Z",
		},
		{
			name:  "extended year falls back",
			input: "12026-05-18",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handled, err := ValidateFastDateLexical([]byte(tt.input))
			if handled != tt.wantHandled {
				t.Fatalf("ValidateFastDateLexical() handled = %v, want %v", handled, tt.wantHandled)
			}
			if got := errorMessage(err); got != tt.wantErr {
				t.Fatalf("ValidateFastDateLexical() error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestValidateDateLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:  "valid common year",
			input: "2001-02-28",
		},
		{
			name:  "valid leap day",
			input: "2000-02-29",
		},
		{
			name:  "valid expanded year",
			input: "10000-01-01",
		},
		{
			name:  "valid negative year",
			input: "-0001-01-01",
		},
		{
			name:  "valid negative leap year",
			input: "-0001-02-29",
		},
		{
			name:  "valid z timezone",
			input: "2026-05-18Z",
		},
		{
			name:  "valid positive max timezone",
			input: "2026-05-18+14:00",
		},
		{
			name:  "valid negative max timezone",
			input: "2026-05-18-14:00",
		},
		{
			name:    "rejects zero year",
			input:   "0000-01-01",
			wantErr: dateErrInvalidDateTime,
		},
		{
			name:    "rejects signed positive year",
			input:   "+2001-01-01",
			wantErr: dateErrInvalidDateTime,
		},
		{
			name:    "rejects expanded leading zero year",
			input:   "02026-05-18",
			wantErr: dateErrInvalidDateTime,
		},
		{
			name:    "rejects negative zero year",
			input:   "-0000-01-01",
			wantErr: dateErrInvalidDateTime,
		},
		{
			name:    "rejects non leap day",
			input:   "1900-02-29",
			wantErr: dateErrInvalidDateTime,
		},
		{
			name:    "rejects negative non leap day",
			input:   "-0004-02-29",
			wantErr: dateErrInvalidDateTime,
		},
		{
			name:    "rejects one digit month",
			input:   "2026-5-18",
			wantErr: dateErrInvalidDateTime,
		},
		{
			name:    "rejects one digit day",
			input:   "2026-05-8",
			wantErr: dateErrInvalidDateTime,
		},
		{
			name:    "rejects timezone minute above max at hour fourteen",
			input:   "2026-05-18+14:01",
			wantErr: dateErrInvalidTimezone,
		},
		{
			name:    "rejects timezone hour above max",
			input:   "2026-05-18+15:00",
			wantErr: dateErrInvalidTimezone,
		},
		{
			name:    "rejects trailing data after z timezone",
			input:   "2026-05-18Zx",
			wantErr: dateErrInvalidDate,
		},
		{
			name:    "rejects bad timezone introducer",
			input:   "2026-05-18T",
			wantErr: dateErrInvalidTimezone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bytesErr := ValidateDateLexical([]byte(tt.input))
			if got := errorMessage(bytesErr); got != tt.wantErr {
				t.Fatalf("ValidateDateLexical() error = %q, want %q", got, tt.wantErr)
			}
			stringErr := validateDateLexical(tt.input)
			if errorMessage(stringErr) != errorMessage(bytesErr) {
				t.Fatalf("validateDateLexical string error for %q = %v, bytes error = %v", tt.input, stringErr, bytesErr)
			}
		})
	}
}
