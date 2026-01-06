package types

import (
	"testing"
	"time"
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
		{"time with milliseconds and offset", "13:20:00.123-05:00", false},
		{"time with nanoseconds and offset", "13:20:00.123456789-05:00", false},
		{"time with milliseconds UTC", "13:20:00.123Z", false},
		{"time with milliseconds no timezone", "13:20:00.123", false},
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

// Test that parsed times have correct structure
func TestParseTimeStructure(t *testing.T) {
	tm, err := ParseTime("13:20:00-05:00")
	if err != nil {
		t.Fatalf("ParseTime failed: %v", err)
	}
	// Should parse to a valid time
	if tm.IsZero() {
		t.Error("ParseTime returned zero time")
	}
	// Time should be on reference date 2000-01-01
	expectedDate := time.Date(2000, 1, 1, 13, 20, 0, 0, time.FixedZone("", -5*3600))
	if tm.Format("2006-01-02T15:04:05-07:00") != expectedDate.Format("2006-01-02T15:04:05-07:00") {
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
