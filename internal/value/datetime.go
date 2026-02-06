package value

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/value/datetime"
)

var fractionalLayouts = [...]string{
	"",
	".0",
	".00",
	".000",
	".0000",
	".00000",
	".000000",
	".0000000",
	".00000000",
	".000000000",
}

var errFractionalSecondsTooLong = errors.New("fractional seconds exceed 9 digits (implementation limit)")

// ParseDateTime parses an xs:dateTime lexical value.
func ParseDateTime(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if err := validateYearPrefix(trimmed, "dateTime"); err != nil {
		return time.Time{}, err
	}
	main, tz := datetime.SplitTimezone(trimmed)
	tzKind := timezoneKindFromTZ(tz)
	timeIndex := strings.IndexByte(main, 'T')
	if timeIndex == -1 {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	datePart := main[:timeIndex]
	timePart := main[timeIndex+1:]
	year, month, day, ok := datetime.ParseDateParts(datePart)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	hour, minute, second, fractionLength, ok := datetime.ParseTimeParts(timePart)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	if year < 1 || year > 9999 {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	if month < 1 || month > 12 || !datetime.IsValidDate(year, month, day) {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	if err := datetime.ValidateTimezoneOffset(tz); err != nil {
		return time.Time{}, err
	}
	if fractionLength > 9 {
		return time.Time{}, fmt.Errorf("invalid dateTime: %w", errFractionalSecondsTooLong)
	}
	needsDayOffset := hour == 24
	if needsDayOffset {
		if minute != 0 || second != 0 || !is24HourZero(timePart) {
			return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
		}
	} else if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 60 {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	if second == 60 && (hour != 23 || minute != 59) {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	leapSecond := second == 60
	if leapSecond {
		timePart = timePart[:6] + "59" + timePart[8:]
		main = datePart + "T" + timePart
	}
	layout := "2006-01-02T15:04:05" + fractionalLayouts[fractionLength]
	parseValue := main
	if needsDayOffset {
		parseValue = datePart + "T00:00:00" + timePart[len("24:00:00"):]
	}
	layout = applyTimezoneLayout(layout, tz)
	parseValue = appendTimezoneSuffix(parseValue, tz)
	parsed, err := time.Parse(layout, parseValue)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	if leapSecond {
		parsed = parsed.Add(time.Second)
	}
	if needsDayOffset {
		parsed = parsed.Add(24 * time.Hour)
	}
	if tzKind == TZKnown {
		utc := parsed.UTC()
		if utc.Year() < 1 || utc.Year() > 9999 {
			return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
		}
	} else if parsed.Year() < 1 || parsed.Year() > 9999 {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	return parsed, nil
}

// ParseDate parses an xs:date lexical value.
func ParseDate(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if err := validateYearPrefix(trimmed, "date"); err != nil {
		return time.Time{}, err
	}
	tzKind := TimezoneKindFromLexical([]byte(trimmed))
	if err := validateOptionalTimezone(trimmed); err != nil {
		return time.Time{}, err
	}
	formats := []string{
		"2006-01-02Z",
		"2006-01-02-07:00",
		"2006-01-02+07:00",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, trimmed); err == nil {
			if tzKind == TZKnown {
				utc := t.UTC()
				if utc.Year() < 1 || utc.Year() > 9999 {
					return time.Time{}, fmt.Errorf("invalid date: %s", trimmed)
				}
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date: %s", trimmed)
}

// ParseTime parses an xs:time lexical value.
func ParseTime(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("invalid time: empty string")
	}
	main, tz := datetime.SplitTimezone(trimmed)
	hour, minute, second, fractionLength, ok := datetime.ParseTimeParts(main)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid time: %s", trimmed)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 60 {
		return time.Time{}, fmt.Errorf("invalid time: %s", trimmed)
	}
	if second == 60 && (hour != 23 || minute != 59) {
		return time.Time{}, fmt.Errorf("invalid time: %s", trimmed)
	}
	leapSecond := second == 60
	if leapSecond {
		main = main[:6] + "59" + main[8:]
	}
	if fractionLength > 9 {
		return time.Time{}, fmt.Errorf("invalid time: %w", errFractionalSecondsTooLong)
	}
	if err := datetime.ValidateTimezoneOffset(tz); err != nil {
		return time.Time{}, err
	}
	layout := "2006-01-02T15:04:05" + fractionalLayouts[fractionLength]
	layout = applyTimezoneLayout(layout, tz)
	parseValue := appendTimezoneSuffix("2000-01-01T"+main, tz)
	parsed, err := time.Parse(layout, parseValue)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time: %s", trimmed)
	}
	if leapSecond {
		parsed = parsed.Add(time.Second)
	}
	return parsed, nil
}

// ParseGYear parses an xs:gYear lexical value.
func ParseGYear(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if err := validateYearPrefix(trimmed, "gYear"); err != nil {
		return time.Time{}, err
	}
	tzKind := TimezoneKindFromLexical([]byte(trimmed))
	if err := validateOptionalTimezone(trimmed); err != nil {
		return time.Time{}, err
	}
	formats := []string{
		"2006Z",
		"2006-07:00",
		"2006+07:00",
		"2006",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, trimmed); err == nil {
			if tzKind == TZKnown {
				utc := t.UTC()
				if utc.Year() < 1 || utc.Year() > 9999 {
					return time.Time{}, fmt.Errorf("invalid gYear: %s", trimmed)
				}
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid gYear: %s", trimmed)
}

// ParseGYearMonth parses an xs:gYearMonth lexical value.
func ParseGYearMonth(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if err := validateYearPrefix(trimmed, "gYearMonth"); err != nil {
		return time.Time{}, err
	}
	tzKind := TimezoneKindFromLexical([]byte(trimmed))
	if err := validateOptionalTimezone(trimmed); err != nil {
		return time.Time{}, err
	}
	formats := []string{
		"2006-01Z",
		"2006-01-07:00",
		"2006-01+07:00",
		"2006-01",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, trimmed); err == nil {
			if tzKind == TZKnown {
				utc := t.UTC()
				if utc.Year() < 1 || utc.Year() > 9999 {
					return time.Time{}, fmt.Errorf("invalid gYearMonth: %s", trimmed)
				}
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid gYearMonth: %s", trimmed)
}

// ParseGMonth parses an xs:gMonth lexical value.
func ParseGMonth(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("invalid gMonth: empty string")
	}
	if err := validateOptionalTimezone(trimmed); err != nil {
		return time.Time{}, err
	}
	formats := []string{
		"2006--01Z",
		"2006--01-07:00",
		"2006--01",
	}
	testValue := "2000" + trimmed
	for _, format := range formats {
		if t, err := time.Parse(format, testValue); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid gMonth: %s", trimmed)
}

// ParseGMonthDay parses an xs:gMonthDay lexical value.
func ParseGMonthDay(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("invalid gMonthDay: empty string")
	}
	if err := validateOptionalTimezone(trimmed); err != nil {
		return time.Time{}, err
	}
	formats := []string{
		"2006--01-02Z",
		"2006--01-02-07:00",
		"2006--01-02",
	}
	testValue := "2000" + trimmed
	for _, format := range formats {
		if t, err := time.Parse(format, testValue); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid gMonthDay: %s", trimmed)
}

// ParseGDay parses an xs:gDay lexical value.
func ParseGDay(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("invalid gDay: empty string")
	}
	if err := validateOptionalTimezone(trimmed); err != nil {
		return time.Time{}, err
	}
	formats := []string{
		"2006-01---02Z",
		"2006-01---02-07:00",
		"2006-01---02",
	}
	testValue := "2000-01" + trimmed
	for _, format := range formats {
		if t, err := time.Parse(format, testValue); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid gDay: %s", trimmed)
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
	_, tz := datetime.SplitTimezone(lexical)
	if tz == "" {
		return nil
	}
	return datetime.ValidateTimezoneOffset(tz)
}

func appendTimezoneSuffix(value, tz string) string {
	switch tz {
	case "Z":
		return value + "Z"
	case "":
		return value
	default:
		return value + tz
	}
}

func applyTimezoneLayout(layout, tz string) string {
	switch tz {
	case "Z":
		return layout + "Z"
	case "":
		return layout
	default:
		return layout + "-07:00"
	}
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

func timezoneKindFromTZ(tz string) TimezoneKind {
	switch tz {
	case "":
		return TZNone
	default:
		return TZKnown
	}
}
