package types

import (
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/value"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"time with negative offset", "13:20:00-05:00", false},
		{"time with positive offset", "13:20:00+05:00", false},
		{"time UTC", "13:20:00Z", false},
		{"time no timezone", "13:20:00", false},
		{"time 24:00:00", "24:00:00", false},
		{"time 24:00:00 with zeros", "24:00:00.000", false},
		{"time 24:00:00 with fraction", "24:00:00.001", true},
		{"time with milliseconds and offset", "13:20:00.123-05:00", false},
		{"time with 1 fractional digit", "13:20:00.1", false},
		{"time with 2 fractional digits UTC", "13:20:00.12Z", false},
		{"time with 4 fractional digits and offset", "13:20:00.1234+05:00", false},
		{"time with 8 fractional digits", "13:20:00.12345678", false},
		{"time with too many fractional digits", "13:20:00.1234567890", true},
		{"time with nanoseconds and offset", "13:20:00.123456789-05:00", false},
		{"time with milliseconds UTC", "13:20:00.123Z", false},
		{"time with milliseconds no timezone", "13:20:00.123", false},
		{"time leap second UTC", "23:59:60Z", false},
		{"empty", "", true},
		{"invalid format", "25:00:00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseGMonth(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"gMonth basic", "--03", false},
		{"gMonth UTC", "--03Z", false},
		{"gMonth negative offset", "--03-05:00", false},
		{"gMonth positive offset", "--03+05:00", false},
		{"gMonth January", "--01", false},
		{"gMonth December", "--12", false},
		{"empty", "", true},
		{"invalid format", "03", true},
		{"invalid month", "--13", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGMonth(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGMonth(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseGMonthDay(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"gMonthDay basic", "--03-15", false},
		{"gMonthDay UTC", "--03-15Z", false},
		{"gMonthDay negative offset", "--03-15-05:00", false},
		{"gMonthDay positive offset", "--03-15+05:00", false},
		{"gMonthDay January 1", "--01-01", false},
		{"gMonthDay December 31", "--12-31", false},
		{"empty", "", true},
		{"invalid format", "03-15", true},
		{"invalid day", "--03-32", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGMonthDay(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGMonthDay(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseGDay(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"gDay basic", "---15", false},
		{"gDay UTC", "---15Z", false},
		{"gDay negative offset", "---15-05:00", false},
		{"gDay positive offset", "---15+05:00", false},
		{"gDay day 1", "---01", false},
		{"gDay day 31", "---31", false},
		{"empty", "", true},
		{"invalid format", "15", true},
		{"invalid day", "---32", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGDay(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGDay(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseDateYearConstraints(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"date basic", "2001-01-01", false},
		{"date year zero", "0000-01-01", true},
		{"date leading plus", "+2001-01-01", true},
		{"date negative year", "-0001-01-01", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseGYearConstraints(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"gYear basic", "2001", false},
		{"gYear UTC", "2001Z", false},
		{"gYear year zero", "0000", true},
		{"gYear leading plus", "+2001", true},
		{"gYear negative year", "-0001", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGYear(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGYear(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseGYearMonthConstraints(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"gYearMonth basic", "2001-10", false},
		{"gYearMonth UTC", "2001-10Z", false},
		{"gYearMonth year zero", "0000-10", true},
		{"gYearMonth leading plus", "+2001-10", true},
		{"gYearMonth negative year", "-0001-10", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGYearMonth(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGYearMonth(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// Test that parsed times have correct structure
func TestParseTimeStructure(t *testing.T) {
	tm, err := ParseTime("13:20:00-05:00")
	if err != nil {
		t.Fatalf("ParseTime failed: %v", err)
	}
	// should parse to a valid time
	if tm.IsZero() {
		t.Error("ParseTime returned zero time")
	}
	// time should be normalized to UTC reference date 2000-01-01
	expectedDate := time.Date(2000, 1, 1, 18, 20, 0, 0, time.UTC)
	if !tm.Equal(expectedDate) {
		t.Errorf("ParseTime returned wrong time: got %v, want %v", tm, expectedDate)
	}
}

func TestNormalizeValue_DateTime(t *testing.T) {
	dateTimeType := GetBuiltin(TypeNameDateTime)
	if dateTimeType == nil {
		t.Fatal("GetBuiltin(TypeNameDateTime) returned nil")
	}

	normalized, err := NormalizeValue(" 2001-10-26T21:32:52 ", dateTimeType)
	if err != nil {
		t.Fatalf("NormalizeValue() error = %v", err)
	}
	if normalized != "2001-10-26T21:32:52" {
		t.Errorf("NormalizeValue() = %q, want %q", normalized, "2001-10-26T21:32:52")
	}
}

func TestTemporalParsingConsistency(t *testing.T) {
	compare := func(t *testing.T, input string, typesFn func(string) (time.Time, error), valueFn func([]byte) (time.Time, error)) {
		t.Helper()
		tTypes, errTypes := typesFn(input)
		tValue, errValue := valueFn([]byte(input))
		if (errTypes != nil) != (errValue != nil) {
			t.Fatalf("parse mismatch for %q: types err=%v value err=%v", input, errTypes, errValue)
		}
		if errTypes == nil && !tTypes.Equal(tValue) {
			t.Fatalf("parse value mismatch for %q: types=%v value=%v", input, tTypes, tValue)
		}
	}

	dateTimeCases := []string{
		"2001-10-26T21:32:52",
		"2001-10-26T21:32:52Z",
		"2001-10-26T21:32:52+02:00",
		"2001-10-26T23:59:60Z",
		"0000-01-01T00:00:00",
	}
	for _, input := range dateTimeCases {
		compare(t, input, ParseDateTime, value.ParseDateTime)
	}

	timeCases := []string{
		"13:20:00",
		"13:20:00Z",
		"23:59:60",
		"12:00:60",
	}
	for _, input := range timeCases {
		compare(t, input, ParseTime, value.ParseTime)
	}

	dateCases := []string{
		"2001-10-26",
		"2001-10-26Z",
		"0000-10-26",
	}
	for _, input := range dateCases {
		compare(t, input, ParseDate, value.ParseDate)
	}

	gYearCases := []string{
		"2001",
		"2001Z",
		"0000",
	}
	for _, input := range gYearCases {
		compare(t, input, ParseGYear, value.ParseGYear)
	}

	gYearMonthCases := []string{
		"2001-10",
		"2001-10Z",
		"0000-10",
	}
	for _, input := range gYearMonthCases {
		compare(t, input, ParseGYearMonth, value.ParseGYearMonth)
	}

	gMonthCases := []string{
		"--10",
		"--10Z",
		"--13",
	}
	for _, input := range gMonthCases {
		compare(t, input, ParseGMonth, value.ParseGMonth)
	}

	gMonthDayCases := []string{
		"--10-26",
		"--10-26Z",
		"--10-32",
	}
	for _, input := range gMonthDayCases {
		compare(t, input, ParseGMonthDay, value.ParseGMonthDay)
	}

	gDayCases := []string{
		"---26",
		"---26Z",
		"---32",
	}
	for _, input := range gDayCases {
		compare(t, input, ParseGDay, value.ParseGDay)
	}
}
