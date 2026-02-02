package datetime

import (
	"fmt"
	"time"
)

// SplitTimezone separates a lexical date/time value into the main portion and timezone suffix.
func SplitTimezone(value string) (string, string) {
	if value == "" {
		return value, ""
	}
	last := value[len(value)-1]
	if last == 'Z' {
		return value[:len(value)-1], "Z"
	}
	if len(value) >= 6 {
		tz := value[len(value)-6:]
		if (tz[0] == '+' || tz[0] == '-') && tz[3] == ':' {
			return value[:len(value)-6], tz
		}
	}
	return value, ""
}

// ParseFixedDigits parses a fixed-width digit sequence from value.
func ParseFixedDigits(value string, start, length int) (int, bool) {
	if start < 0 || length <= 0 || start+length > len(value) {
		return 0, false
	}
	n := 0
	for i := range length {
		ch := value[start+i]
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int(ch-'0')
	}
	return n, true
}

// ParseDateParts parses YYYY-MM-DD into year, month, day.
func ParseDateParts(value string) (int, int, int, bool) {
	if len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return 0, 0, 0, false
	}
	year, ok := ParseFixedDigits(value, 0, 4)
	if !ok {
		return 0, 0, 0, false
	}
	month, ok := ParseFixedDigits(value, 5, 2)
	if !ok {
		return 0, 0, 0, false
	}
	day, ok := ParseFixedDigits(value, 8, 2)
	if !ok {
		return 0, 0, 0, false
	}
	return year, month, day, true
}

// ParseTimeParts parses hh:mm:ss[.fff] into time parts and fractional length.
func ParseTimeParts(value string) (int, int, int, int, bool) {
	if len(value) < 8 || value[2] != ':' || value[5] != ':' {
		return 0, 0, 0, 0, false
	}
	hour, ok := ParseFixedDigits(value, 0, 2)
	if !ok {
		return 0, 0, 0, 0, false
	}
	minute, ok := ParseFixedDigits(value, 3, 2)
	if !ok {
		return 0, 0, 0, 0, false
	}
	second, ok := ParseFixedDigits(value, 6, 2)
	if !ok {
		return 0, 0, 0, 0, false
	}
	if len(value) == 8 {
		return hour, minute, second, 0, true
	}
	if value[8] != '.' || len(value) == 9 {
		return 0, 0, 0, 0, false
	}
	for i := 9; i < len(value); i++ {
		ch := value[i]
		if ch < '0' || ch > '9' {
			return 0, 0, 0, 0, false
		}
	}
	fractionLength := len(value) - 9
	return hour, minute, second, fractionLength, true
}

// ValidateTimezoneOffset validates a timezone offset suffix.
func ValidateTimezoneOffset(tz string) error {
	if tz == "" || tz == "Z" {
		return nil
	}
	if len(tz) != 6 {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	if tz[0] != '+' && tz[0] != '-' {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	if tz[3] != ':' {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	hour, ok := ParseFixedDigits(tz, 1, 2)
	if !ok {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	minute, ok := ParseFixedDigits(tz, 4, 2)
	if !ok {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	if hour < 0 || hour > 14 || minute < 0 || minute > 59 {
		return fmt.Errorf("invalid timezone offset: %s", tz)
	}
	if hour == 14 && minute != 0 {
		return fmt.Errorf("invalid timezone offset: %s", tz)
	}
	return nil
}

// IsValidDate reports whether the date is valid in the Gregorian calendar.
func IsValidDate(year, month, day int) bool {
	if day < 1 || day > 31 {
		return false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Year() == year && int(t.Month()) == month && t.Day() == day
}
