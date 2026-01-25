package types

import "testing"

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// expected decimal value as string (for comparison)
		want    string
		wantErr bool
	}{
		{"positive integer", "123", "123", false},
		{"negative integer", "-456", "-456", false},
		{"positive decimal", "123.456", "123.456", false},
		{"negative decimal", "-123.456", "-123.456", false},
		{"zero", "0", "0", false},
		{"leading plus", "+123", "123", false},
		{"with whitespace", "  123.456  ", "123.456", false},
		{"exponent", "1e2", "", true},
		{"fraction", "1/3", "", true},
		{"invalid", "abc", "", true},
		{"empty", "", "", true},
		{"only dot", ".", "", true},
		{"double dot", "12.34.56", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDecimal(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDecimal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// compare as float64 for decimal values
				gotFloat, _ := got.Float64()
				wantRat, _ := ParseDecimal(tt.want)
				wantFloat, _ := wantRat.Float64()
				if gotFloat != wantFloat {
					t.Errorf("ParseDecimal() = %v (%v), want %v (%v)", got, gotFloat, tt.want, wantFloat)
				}
			}
		})
	}
}

func TestParseInteger(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// expected string representation
		want    string
		wantErr bool
	}{
		{"positive", "123", "123", false},
		{"negative", "-456", "-456", false},
		{"zero", "0", "0", false},
		{"leading plus", "+123", "123", false},
		{"with whitespace", "  123  ", "123", false},
		{"large number", "12345678901234567890", "12345678901234567890", false},
		{"invalid", "abc", "", true},
		{"decimal", "123.456", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInteger(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseInteger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.String() != tt.want {
					t.Errorf("ParseInteger() = %v, want %v", got.String(), tt.want)
				}
			}
		})
	}
}

func TestParseBoolean(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{"true", "true", true, false},
		{"false", "false", false, false},
		{"one", "1", true, false},
		{"zero", "0", false, false},
		{"with whitespace", "  true  ", true, false},
		{"invalid", "yes", false, true},
		{"empty", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBoolean(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBoolean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseBoolean() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTimeLeapSecond(t *testing.T) {
	t1, err := ParseTime("23:59:59Z")
	if err != nil {
		t.Fatalf("ParseTime() error = %v", err)
	}
	t2, err := ParseTime("23:59:60Z")
	if err != nil {
		t.Fatalf("ParseTime() error = %v", err)
	}
	if t1.Equal(t2) {
		t.Fatalf("expected leap second to differ from 23:59:59")
	}
	if !t2.After(t1) {
		t.Fatalf("expected leap second to be after 23:59:59")
	}
}

func TestParseUnsignedAcceptsSignedZero(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) (uint64, error)
	}{
		{
			name: "unsignedLong",
			fn:   ParseUnsignedLong,
		},
		{
			name: "unsignedInt",
			fn: func(value string) (uint64, error) {
				v, err := ParseUnsignedInt(value)
				return uint64(v), err
			},
		},
		{
			name: "unsignedShort",
			fn: func(value string) (uint64, error) {
				v, err := ParseUnsignedShort(value)
				return uint64(v), err
			},
		},
		{
			name: "unsignedByte",
			fn: func(value string) (uint64, error) {
				v, err := ParseUnsignedByte(value)
				return uint64(v), err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tests := map[string]uint64{
				"+0": 0,
				"-0": 0,
				"+1": 1,
			}
			for value, want := range tests {
				got, err := tt.fn(value)
				if err != nil {
					t.Fatalf("Parse(%q) error = %v", value, err)
				}
				if got != want {
					t.Fatalf("Parse(%q) = %d, want %d", value, got, want)
				}
			}
			if _, err := tt.fn("-1"); err == nil {
				t.Fatalf("expected Parse(-1) to error")
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"positive", "123.456", false},
		{"negative", "-123.456", false},
		{"zero", "0", false},
		{"plus INF", "+INF", true},
		{"INF", "INF", false},
		{"negative INF", "-INF", false},
		{"NaN", "NaN", false},
		{"with whitespace", "  123.456  ", false},
		{"exponent", "1.2e3", false},
		{"hex", "0x1.2p3", true},
		{"underscore", "1_2.0", true},
		{"lower inf", "inf", true},
		{"lower nan", "nan", true},
		{"invalid", "abc", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFloat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				switch tt.input {
				case "INF":
					if !isInf32(got) {
						t.Errorf("ParseFloat() = %v, want INF", got)
					}
				case "-INF":
					if !isInf32(-got) {
						t.Errorf("ParseFloat() = %v, want -INF", got)
					}
				case "NaN":
					if !isNaN32(got) {
						t.Errorf("ParseFloat() = %v, want NaN", got)
					}
				}
			}
		})
	}
}

func TestParseDouble(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"positive", "123.456", false},
		{"negative", "-123.456", false},
		{"zero", "0", false},
		{"plus INF", "+INF", true},
		{"INF", "INF", false},
		{"negative INF", "-INF", false},
		{"NaN", "NaN", false},
		{"with whitespace", "  123.456  ", false},
		{"exponent", "1.2e3", false},
		{"hex", "0x1.2p3", true},
		{"underscore", "1_2.0", true},
		{"lower inf", "inf", true},
		{"lower nan", "nan", true},
		{"invalid", "abc", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDouble(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDouble() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				switch tt.input {
				case "INF":
					if !isInf64(got) {
						t.Errorf("ParseDouble() = %v, want INF", got)
					}
				case "-INF":
					if !isInf64(-got) {
						t.Errorf("ParseDouble() = %v, want -INF", got)
					}
				case "NaN":
					if !isNaN64(got) {
						t.Errorf("ParseDouble() = %v, want NaN", got)
					}
				}
			}
		})
	}
}

func TestParseDateTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"with 24:00:00", "2001-10-26T24:00:00", false},
		{"max year overflow 24:00:00", "9999-12-31T24:00:00", true},
		{"leap second", "1999-12-31T23:59:60Z", false},
		{"valid datetime", "2001-10-26T21:32:52", false},
		{"with timezone", "2001-10-26T21:32:52+02:00", false},
		{"with Z", "2001-10-26T21:32:52Z", false},
		{"with milliseconds", "2001-10-26T21:32:52.12679", false},
		{"leading plus year", "+2001-10-26T21:32:52", true},
		{"negative year", "-0001-10-26T21:32:52", true},
		{"year zero", "0000-10-26T21:32:52", true},
		{"invalid", "not-a-date", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDateTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.IsZero() {
				t.Errorf("ParseDateTime() returned zero time")
			}
		})
	}
}

func TestParseDateTime24HourRolloverBoundaries(t *testing.T) {
	got, err := ParseDateTime("9999-12-30T24:00:00")
	if err != nil {
		t.Fatalf("ParseDateTime() error = %v", err)
	}
	if got.Year() != 9999 || got.Month() != 12 || got.Day() != 31 || got.Hour() != 0 {
		t.Fatalf("unexpected rollover result: %s", got.Format("2006-01-02T15:04:05"))
	}

	got, err = ParseDateTime("0001-01-01T24:00:00")
	if err != nil {
		t.Fatalf("ParseDateTime() error = %v", err)
	}
	if got.Year() != 1 || got.Month() != 1 || got.Day() != 2 || got.Hour() != 0 {
		t.Fatalf("unexpected rollover result: %s", got.Format("2006-01-02T15:04:05"))
	}
}

// Helper functions to check for INF and NaN
func isInf32(f float32) bool {
	return f > 1e38 || f < -1e38
}

func isNaN32(f float32) bool {
	return f != f
}

func isInf64(f float64) bool {
	return f > 1e308 || f < -1e308
}

func isNaN64(f float64) bool {
	return f != f
}
