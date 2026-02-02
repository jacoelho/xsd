package datetime

import "testing"

func TestDateTimeHelpers(t *testing.T) {
	main, tz := SplitTimezone("2024-01-01Z")
	if main != "2024-01-01" || tz != "Z" {
		t.Fatalf("SplitTimezone mismatch: %q %q", main, tz)
	}
	main, tz = SplitTimezone("2024-01-01+02:00")
	if main != "2024-01-01" || tz != "+02:00" {
		t.Fatalf("SplitTimezone mismatch: %q %q", main, tz)
	}

	if _, ok := ParseFixedDigits("2024", 0, 4); !ok {
		t.Fatalf("expected ParseFixedDigits to succeed")
	}
	year, month, day, ok := ParseDateParts("2024-02-29")
	if !ok {
		t.Fatalf("expected ParseDateParts to succeed")
	}
	if year != 2024 || month != 2 || day != 29 {
		t.Fatalf("unexpected ParseDateParts values: %d-%02d-%02d", year, month, day)
	}
	if _, _, _, ok := ParseDateParts("2024-2-29"); ok {
		t.Fatalf("expected ParseDateParts to fail")
	}
	hour, minute, second, fractionLength, ok := ParseTimeParts("12:34:56.789")
	if !ok {
		t.Fatalf("expected ParseTimeParts to succeed")
	}
	if hour != 12 || minute != 34 || second != 56 || fractionLength != 3 {
		t.Fatalf("unexpected ParseTimeParts values: %02d:%02d:%02d (%d)", hour, minute, second, fractionLength)
	}
	if _, _, _, _, ok := ParseTimeParts("12:34"); ok {
		t.Fatalf("expected ParseTimeParts to fail")
	}

	if err := ValidateTimezoneOffset("Z"); err != nil {
		t.Fatalf("unexpected timezone error: %v", err)
	}
	if err := ValidateTimezoneOffset("+14:00"); err != nil {
		t.Fatalf("unexpected timezone error: %v", err)
	}
	if err := ValidateTimezoneOffset("+14:01"); err == nil {
		t.Fatalf("expected timezone range error")
	}

	if !IsValidDate(2024, 2, 29) {
		t.Fatalf("expected leap day to be valid")
	}
	if IsValidDate(2023, 2, 29) {
		t.Fatalf("expected non-leap day to be invalid")
	}
}
