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
