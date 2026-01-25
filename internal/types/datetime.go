package types

import (
	"fmt"
	"strings"
	"time"
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

func validateYearPrefix(lexical, kind string) error {
	if lexical == "" {
		return fmt.Errorf("invalid %s: empty string", kind)
	}
	if lexical[0] == '+' {
		return fmt.Errorf("invalid %s: leading '+' is not allowed", kind)
	}
	if lexical[0] == '-' {
		return fmt.Errorf("invalid %s: negative year is not supported", kind)
	}
	if len(lexical) < 4 {
		return fmt.Errorf("invalid %s: year must have 4 digits", kind)
	}
	for i := range 4 {
		ch := lexical[i]
		if ch < '0' || ch > '9' {
			return fmt.Errorf("invalid %s: year must have 4 digits", kind)
		}
	}
	if lexical[:4] == "0000" {
		return fmt.Errorf("invalid %s: year 0000 is not allowed", kind)
	}
	if len(lexical) > 4 {
		ch := lexical[4]
		if ch >= '0' && ch <= '9' {
			return fmt.Errorf("invalid %s: year must have 4 digits", kind)
		}
	}
	return nil
}

func validateOptionalTimezone(lexical string) error {
	_, tz := splitTimezone(lexical)
	if tz == "" {
		return nil
	}
	return validateTimezoneOffset(tz)
}

// HasTimezone reports whether a lexical date/time value includes a timezone indicator.
func HasTimezone(lexical string) bool {
	lexical = TrimXMLWhitespace(lexical)
	_, tz := splitTimezone(lexical)
	return tz != ""
}

func is24HourZero(timePart string) bool {
	const prefix = "24:00:00"
	if !strings.HasPrefix(timePart, prefix) {
		return false
	}
	if len(timePart) == len(prefix) {
		return true
	}
	if timePart[len(prefix)] != '.' || len(timePart) == len(prefix)+1 {
		return false
	}
	if len(timePart)-len(prefix)-1 > 9 {
		return false
	}
	for i := len(prefix) + 1; i < len(timePart); i++ {
		if timePart[i] != '0' {
			return false
		}
	}
	return true
}

// ParseDate parses a date string into time.Time (date component only)
// Format: YYYY-MM-DD with optional timezone
func ParseDate(lexical string) (time.Time, error) {
	lexical = TrimXMLWhitespace(lexical)
	if err := validateYearPrefix(lexical, "date"); err != nil {
		return time.Time{}, err
	}
	if err := validateOptionalTimezone(lexical); err != nil {
		return time.Time{}, err
	}

	formats := []string{
		"2006-01-02Z",      // UTC
		"2006-01-02-07:00", // with timezone
		"2006-01-02+07:00", // with timezone
		"2006-01-02",       // no timezone
	}

	for _, format := range formats {
		if t, err := time.Parse(format, lexical); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date: %s", lexical)
}

// ParseTime parses a time string into time.Time (time component only)
// Format: HH:MM:SS with optional fractional seconds and timezone
func ParseTime(lexical string) (time.Time, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return time.Time{}, fmt.Errorf("invalid time: empty string")
	}

	main, tz := splitTimezone(lexical)
	hour, minute, second, fractionLength, ok := parseTimeParts(main)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid time: %s", lexical)
	}

	is24Hour := hour == 24
	if is24Hour {
		if minute != 0 || second != 0 || !is24HourZero(main) {
			return time.Time{}, fmt.Errorf("invalid time: %s", lexical)
		}
	} else if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 60 {
		return time.Time{}, fmt.Errorf("invalid time: %s", lexical)
	}
	leapSecond := second == 60
	if leapSecond {
		main = main[:6] + "59" + main[8:]
	}

	if fractionLength > 9 {
		return time.Time{}, fmt.Errorf("invalid time: %s", lexical)
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return time.Time{}, err
	}

	if is24Hour {
		main = "00:00:00" + main[len("24:00:00"):]
	}

	layout := "2006-01-02T15:04:05" + fractionalLayouts[fractionLength]
	layout = applyTimezoneLayout(layout, tz)
	parseValue := appendTimezoneSuffix("2000-01-01T"+main, tz)
	parsed, err := time.Parse(layout, parseValue)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time: %s", lexical)
	}
	if leapSecond {
		parsed = parsed.Add(time.Second)
	}
	return parsed, nil
}

// ParseGYear parses a gYear string into time.Time
// Format: YYYY with optional timezone
func ParseGYear(lexical string) (time.Time, error) {
	lexical = TrimXMLWhitespace(lexical)
	if err := validateYearPrefix(lexical, "gYear"); err != nil {
		return time.Time{}, err
	}
	if err := validateOptionalTimezone(lexical); err != nil {
		return time.Time{}, err
	}

	formats := []string{
		"2006Z",      // UTC
		"2006-07:00", // with timezone
		"2006+07:00", // with timezone
		"2006",       // no timezone
	}

	for _, format := range formats {
		if t, err := time.Parse(format, lexical); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid gYear: %s", lexical)
}

// ParseGYearMonth parses a gYearMonth string into time.Time
// Format: YYYY-MM with optional timezone
func ParseGYearMonth(lexical string) (time.Time, error) {
	lexical = TrimXMLWhitespace(lexical)
	if err := validateYearPrefix(lexical, "gYearMonth"); err != nil {
		return time.Time{}, err
	}
	if err := validateOptionalTimezone(lexical); err != nil {
		return time.Time{}, err
	}

	formats := []string{
		"2006-01Z",      // UTC
		"2006-01-07:00", // with timezone
		"2006-01+07:00", // with timezone
		"2006-01",       // no timezone
	}

	for _, format := range formats {
		if t, err := time.Parse(format, lexical); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid gYearMonth: %s", lexical)
}

// ParseGMonth parses a gMonth string into time.Time
// Format: --MM with optional timezone
func ParseGMonth(lexical string) (time.Time, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return time.Time{}, fmt.Errorf("invalid gMonth: empty string")
	}
	if err := validateOptionalTimezone(lexical); err != nil {
		return time.Time{}, err
	}

	// format strings must include the year placeholder (2006) to match the structure
	formats := []string{
		"2006--01Z",      // format: year--monthZ (UTC)
		"2006--01-07:00", // format: year--month-offset (matches both + and - offsets)
		"2006--01",       // format: year--month (no timezone)
	}

	// prepend reference year to the value
	testValue := "2000" + lexical
	for _, format := range formats {
		if t, err := time.Parse(format, testValue); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid gMonth: %s", lexical)
}

// ParseGMonthDay parses a gMonthDay string into time.Time
// Format: --MM-DD with optional timezone
func ParseGMonthDay(lexical string) (time.Time, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return time.Time{}, fmt.Errorf("invalid gMonthDay: empty string")
	}
	if err := validateOptionalTimezone(lexical); err != nil {
		return time.Time{}, err
	}

	// format strings must include the year placeholder (2006) to match the structure
	formats := []string{
		"2006--01-02Z",      // format: year--month-dayZ (UTC)
		"2006--01-02-07:00", // format: year--month-day-offset (matches both + and - offsets)
		"2006--01-02",       // format: year--month-day (no timezone)
	}

	// prepend reference year to the value
	testValue := "2000" + lexical
	for _, format := range formats {
		if t, err := time.Parse(format, testValue); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid gMonthDay: %s", lexical)
}

// ParseGDay parses a gDay string into time.Time
// Format: ---DD with optional timezone
func ParseGDay(lexical string) (time.Time, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return time.Time{}, fmt.Errorf("invalid gDay: empty string")
	}
	if err := validateOptionalTimezone(lexical); err != nil {
		return time.Time{}, err
	}

	// format strings must include the year and month placeholders (2006-01) to match the structure
	formats := []string{
		"2006-01---02Z",      // format: year-month---dayZ (UTC)
		"2006-01---02-07:00", // format: year-month---day-offset (matches both + and - offsets)
		"2006-01---02",       // format: year-month---day (no timezone)
	}

	// prepend reference year and month to the value
	testValue := "2000-01" + lexical
	for _, format := range formats {
		if t, err := time.Parse(format, testValue); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid gDay: %s", lexical)
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
