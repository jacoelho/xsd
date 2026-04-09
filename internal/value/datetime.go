package value

import (
	"errors"
	"fmt"
	"strings"
	"time"
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

type parsedDateTime struct {
	trimmed        string
	main           string
	tz             string
	datePart       string
	timePart       string
	year           int
	month          int
	day            int
	hour           int
	minute         int
	second         int
	fractionLength int
	tzKind         TimezoneKind
}

// ParseDateTime parses an xs:dateTime lexical value.
func ParseDateTime(lexical []byte) (time.Time, error) {
	parsed, err := parseDateTimeLexical(lexical)
	if err != nil {
		return time.Time{}, err
	}
	needsDayOffset, leapSecond, err := validateParsedDateTime(parsed)
	if err != nil {
		return time.Time{}, err
	}
	value, layout := dateTimeParseInput(parsed, needsDayOffset, leapSecond)
	ts, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, invalidDateTimeValue(parsed.trimmed)
	}
	ts = normalizeParsedDateTime(ts, needsDayOffset, leapSecond)
	if err := validateParsedDateTimeRange(parsed, ts); err != nil {
		return time.Time{}, err
	}
	return ts, nil
}

func parseDateTimeLexical(lexical []byte) (parsedDateTime, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if err := validateYearPrefix(trimmed, "dateTime"); err != nil {
		return parsedDateTime{}, err
	}
	main, tz := SplitTimezone(trimmed)
	before, after, ok := strings.Cut(main, "T")
	if !ok {
		return parsedDateTime{}, invalidDateTimeValue(trimmed)
	}
	datePart := before
	timePart := after
	year, month, day, ok := ParseDateParts(datePart)
	if !ok {
		return parsedDateTime{}, invalidDateTimeValue(trimmed)
	}
	hour, minute, second, fractionLength, ok := ParseTimeParts(timePart)
	if !ok {
		return parsedDateTime{}, invalidDateTimeValue(trimmed)
	}
	return parsedDateTime{
		trimmed:        trimmed,
		main:           main,
		tz:             tz,
		tzKind:         timezoneKindFromTZ(tz),
		datePart:       datePart,
		timePart:       timePart,
		year:           year,
		month:          month,
		day:            day,
		hour:           hour,
		minute:         minute,
		second:         second,
		fractionLength: fractionLength,
	}, nil
}

func validateParsedDateTime(parsed parsedDateTime) (bool, bool, error) {
	if parsed.year < 1 || parsed.year > 9999 {
		return false, false, invalidDateTimeValue(parsed.trimmed)
	}
	if parsed.month < 1 || parsed.month > 12 || !IsValidDate(parsed.year, parsed.month, parsed.day) {
		return false, false, invalidDateTimeValue(parsed.trimmed)
	}
	if err := ValidateTimezoneOffset(parsed.tz); err != nil {
		return false, false, err
	}
	if parsed.fractionLength > 9 {
		return false, false, fmt.Errorf("invalid dateTime: %w", errFractionalSecondsTooLong)
	}
	needsDayOffset := parsed.hour == 24
	if needsDayOffset {
		if parsed.minute != 0 || parsed.second != 0 || !is24HourZero(parsed.timePart) {
			return false, false, invalidDateTimeValue(parsed.trimmed)
		}
	} else if parsed.hour < 0 || parsed.hour > 23 || parsed.minute < 0 || parsed.minute > 59 || parsed.second < 0 || parsed.second > 60 {
		return false, false, invalidDateTimeValue(parsed.trimmed)
	}
	leapSecond := parsed.second == 60
	if leapSecond && (parsed.hour != 23 || parsed.minute != 59) {
		return false, false, invalidDateTimeValue(parsed.trimmed)
	}
	return needsDayOffset, leapSecond, nil
}

func dateTimeParseInput(parsed parsedDateTime, needsDayOffset, leapSecond bool) (string, string) {
	timePart := parsed.timePart
	main := parsed.main
	if leapSecond {
		timePart = timePart[:6] + "59" + timePart[8:]
		main = parsed.datePart + "T" + timePart
	}
	parseValue := main
	if needsDayOffset {
		parseValue = parsed.datePart + "T00:00:00" + timePart[len("24:00:00"):]
	}
	layout := "2006-01-02T15:04:05" + fractionalLayouts[parsed.fractionLength]
	layout = applyTimezoneLayout(layout, parsed.tz)
	parseValue = appendTimezoneSuffix(parseValue, parsed.tz)
	return parseValue, layout
}

func normalizeParsedDateTime(ts time.Time, needsDayOffset, leapSecond bool) time.Time {
	if leapSecond {
		ts = ts.Add(time.Second)
	}
	if needsDayOffset {
		ts = ts.Add(24 * time.Hour)
	}
	return ts
}

func validateParsedDateTimeRange(parsed parsedDateTime, ts time.Time) error {
	if parsed.tzKind == TZKnown {
		utc := ts.UTC()
		if utc.Year() < 1 || utc.Year() > 9999 {
			return invalidDateTimeValue(parsed.trimmed)
		}
		return nil
	}
	if ts.Year() < 1 || ts.Year() > 9999 {
		return invalidDateTimeValue(parsed.trimmed)
	}
	return nil
}

func invalidDateTimeValue(trimmed string) error {
	return fmt.Errorf("invalid dateTime: %s", trimmed)
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
	layouts := []string{
		"2006-01-02Z",
		"2006-01-02-07:00",
		"2006-01-02+07:00",
		"2006-01-02",
	}
	return parseTemporalByLayouts("date", trimmed, trimmed, layouts, tzKind)
}

// ParseTime parses an xs:time lexical value.
func ParseTime(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("invalid time: empty string")
	}
	main, tz := SplitTimezone(trimmed)
	hour, minute, second, fractionLength, ok := ParseTimeParts(main)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid time: %s", trimmed)
	}
	is24Hour := hour == 24
	if is24Hour {
		if minute != 0 || second != 0 || !is24HourZero(main) {
			return time.Time{}, fmt.Errorf("invalid time: %s", trimmed)
		}
	} else if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 60 {
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
	if err := ValidateTimezoneOffset(tz); err != nil {
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
	layouts := []string{
		"2006Z",
		"2006-07:00",
		"2006+07:00",
		"2006",
	}
	return parseTemporalByLayouts("gYear", trimmed, trimmed, layouts, tzKind)
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
	layouts := []string{
		"2006-01Z",
		"2006-01-07:00",
		"2006-01+07:00",
		"2006-01",
	}
	return parseTemporalByLayouts("gYearMonth", trimmed, trimmed, layouts, tzKind)
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
	layouts := []string{
		"2006--01Z",
		"2006--01-07:00",
		"2006--01",
	}
	return parseTemporalByLayouts("gMonth", trimmed, "2000"+trimmed, layouts, TZNone)
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
	layouts := []string{
		"2006--01-02Z",
		"2006--01-02-07:00",
		"2006--01-02",
	}
	return parseTemporalByLayouts("gMonthDay", trimmed, "2000"+trimmed, layouts, TZNone)
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
	layouts := []string{
		"2006-01---02Z",
		"2006-01---02-07:00",
		"2006-01---02",
	}
	return parseTemporalByLayouts("gDay", trimmed, "2000-01"+trimmed, layouts, TZNone)
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
	_, tz := SplitTimezone(lexical)
	if tz == "" {
		return nil
	}
	return ValidateTimezoneOffset(tz)
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

func parseTemporalByLayouts(kind, lexical, parseValue string, layouts []string, tzKind TimezoneKind) (time.Time, error) {
	for _, layout := range layouts {
		t, err := time.Parse(layout, parseValue)
		if err != nil {
			continue
		}
		if tzKind == TZKnown {
			utc := t.UTC()
			if utc.Year() < 1 || utc.Year() > 9999 {
				return time.Time{}, fmt.Errorf("invalid %s: %s", kind, lexical)
			}
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid %s: %s", kind, lexical)
}
