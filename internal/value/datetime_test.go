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

func TestParseTimeLeapSecond(t *testing.T) {
	if _, err := ParseTime([]byte("23:59:60")); err != nil {
		t.Fatalf("ParseTime(23:59:60) error = %v", err)
	}
	if _, err := ParseTime([]byte("12:00:60")); err == nil {
		t.Fatalf("expected error for leap second outside 23:59")
	}
}

func TestParseGYearMonth(t *testing.T) {
	if _, err := ParseGYearMonth([]byte("2001-10")); err != nil {
		t.Fatalf("ParseGYearMonth() error = %v", err)
	}
}

func TestParseDateTimeLeapSecond(t *testing.T) {
	if _, err := ParseDateTime([]byte("2001-10-26T23:59:60Z")); err != nil {
		t.Fatalf("ParseDateTime leap second error = %v", err)
	}
	if _, err := ParseDateTime([]byte("2001-10-26T12:00:60Z")); err == nil {
		t.Fatalf("expected error for leap second outside 23:59")
	}
}

func TestCanonicalDateTimeUTC(t *testing.T) {
	lexical := []byte("2001-10-26T21:32:52+02:00")
	ts, err := ParseDateTime(lexical)
	if err != nil {
		t.Fatalf("ParseDateTime() error = %v", err)
	}
	got := CanonicalDateTimeString(ts, "dateTime", HasTimezone(lexical))
	if got != "2001-10-26T19:32:52Z" {
		t.Fatalf("CanonicalDateTimeString() = %q, want %q", got, "2001-10-26T19:32:52Z")
	}
}

func TestCanonicalTimeUTC(t *testing.T) {
	lexical := []byte("23:00:00-01:00")
	ts, err := ParseTime(lexical)
	if err != nil {
		t.Fatalf("ParseTime() error = %v", err)
	}
	got := CanonicalDateTimeString(ts, "time", HasTimezone(lexical))
	if got != "00:00:00Z" {
		t.Fatalf("CanonicalDateTimeString() = %q, want %q", got, "00:00:00Z")
	}
}
