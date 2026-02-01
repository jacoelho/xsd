package types

import (
	"fmt"
	"time"

	"github.com/jacoelho/xsd/internal/value"
)

type dateTimeNormalizer struct{}

// Normalize applies whitespace normalization for date/time lexical values.
func (n dateTimeNormalizer) Normalize(lexical string, typ Type) (string, error) {
	if typ == nil {
		return TrimXMLWhitespace(lexical), nil
	}
	normalized := ApplyWhiteSpace(lexical, typ.WhiteSpace())
	return TrimXMLWhitespace(normalized), nil
}

// HasTimezone reports whether a lexical date/time value includes a timezone indicator.
func HasTimezone(lexical string) bool {
	lexical = TrimXMLWhitespace(lexical)
	_, tz := splitTimezone(lexical)
	return tz != ""
}

// ParseDate parses a date string into time.Time (date component only)
// Format: YYYY-MM-DD with optional timezone
func ParseDate(lexical string) (time.Time, error) {
	return value.ParseDate([]byte(lexical))
}

// ParseTime parses a time string into time.Time (time component only)
// Format: HH:MM:SS with optional fractional seconds and timezone
func ParseTime(lexical string) (time.Time, error) {
	return value.ParseTime([]byte(lexical))
}

// ParseGYear parses a gYear string into time.Time
// Format: YYYY with optional timezone
func ParseGYear(lexical string) (time.Time, error) {
	return value.ParseGYear([]byte(lexical))
}

// ParseGYearMonth parses a gYearMonth string into time.Time
// Format: YYYY-MM with optional timezone
func ParseGYearMonth(lexical string) (time.Time, error) {
	return value.ParseGYearMonth([]byte(lexical))
}

// ParseGMonth parses a gMonth string into time.Time
// Format: --MM with optional timezone
func ParseGMonth(lexical string) (time.Time, error) {
	return value.ParseGMonth([]byte(lexical))
}

// ParseGMonthDay parses a gMonthDay string into time.Time
// Format: --MM-DD with optional timezone
func ParseGMonthDay(lexical string) (time.Time, error) {
	return value.ParseGMonthDay([]byte(lexical))
}

// ParseGDay parses a gDay string into time.Time
// Format: ---DD with optional timezone
func ParseGDay(lexical string) (time.Time, error) {
	return value.ParseGDay([]byte(lexical))
}

// ParseDuration validates an XSD duration string and returns the lexical value.
// It does not map to time.Duration.
func ParseDuration(lexical string) (string, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return "", fmt.Errorf("invalid duration: empty string")
	}

	if _, err := ParseXSDDuration(lexical); err != nil {
		return "", err
	}

	return lexical, nil
}
