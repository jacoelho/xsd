package value

import "testing"

func TestParseDateTime(t *testing.T) {
	if _, err := ParseDateTime([]byte("2001-10-26T21:32:52")); err != nil {
		t.Fatalf("ParseDateTime() error = %v", err)
	}
	if _, err := ParseDateTime([]byte("0000-01-01T00:00:00")); err == nil {
		t.Fatalf("expected error for year 0000")
	}
}

func TestParseDate(t *testing.T) {
	if _, err := ParseDate([]byte("2001-10-26")); err != nil {
		t.Fatalf("ParseDate() error = %v", err)
	}
}

func TestParseTime(t *testing.T) {
	if _, err := ParseTime([]byte("24:00:00")); err != nil {
		t.Fatalf("ParseTime() error = %v", err)
	}
	if _, err := ParseTime([]byte("24:01:00")); err == nil {
		t.Fatalf("expected error for invalid 24-hour time")
	}
}

func TestParseGYearMonth(t *testing.T) {
	if _, err := ParseGYearMonth([]byte("2001-10")); err != nil {
		t.Fatalf("ParseGYearMonth() error = %v", err)
	}
}
