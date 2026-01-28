package value

import (
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

// ParseDateTime parses an xs:dateTime lexical value.
func ParseDateTime(lexical []byte) (time.Time, error) {
	trimmed := string(TrimXMLWhitespace(lexical))
	if err := validateYearPrefix(trimmed, "dateTime"); err != nil {
		return time.Time{}, err
	}
	main, tz := splitTimezone(trimmed)
	timeIndex := strings.IndexByte(main, 'T')
	if timeIndex == -1 {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	datePart := main[:timeIndex]
	timePart := main[timeIndex+1:]
	year, month, day, ok := parseDateParts(datePart)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	hour, minute, second, fractionLength, ok := parseTimeParts(timePart)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	if year < 1 || year > 9999 {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	if month < 1 || month > 12 || !isValidDate(year, month, day) {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return time.Time{}, err
	}
	if fractionLength > 9 {
		return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
	}
	needsDayOffset := hour == 24
	if needsDayOffset {
		if minute != 0 || second != 0 || !is24HourZero(timePart) {
			return time.Time{}, fmt.Errorf("invalid dateTime: %s", trimmed)
		}
	} else if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 60 {
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
	if parsed.Year() < 1 || parsed.Year() > 9999 {
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
	main, tz := splitTimezone(trimmed)
	hour, minute, second, fractionLength, ok := parseTimeParts(main)
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
	leapSecond := second == 60
	if leapSecond {
		main = main[:6] + "59" + main[8:]
	}
	if fractionLength > 9 {
		return time.Time{}, fmt.Errorf("invalid time: %s", trimmed)
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
	_, tz := splitTimezone(lexical)
	if tz == "" {
		return nil
	}
	return validateTimezoneOffset(tz)
}

func splitTimezone(value string) (string, string) {
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

func parseDateParts(value string) (int, int, int, bool) {
	if len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return 0, 0, 0, false
	}
	year, ok := parseFixedDigits(value, 0, 4)
	if !ok {
		return 0, 0, 0, false
	}
	month, ok := parseFixedDigits(value, 5, 2)
	if !ok {
		return 0, 0, 0, false
	}
	day, ok := parseFixedDigits(value, 8, 2)
	if !ok {
		return 0, 0, 0, false
	}
	return year, month, day, true
}

func parseTimeParts(value string) (int, int, int, int, bool) {
	if len(value) < 8 || value[2] != ':' || value[5] != ':' {
		return 0, 0, 0, 0, false
	}
	hour, ok := parseFixedDigits(value, 0, 2)
	if !ok {
		return 0, 0, 0, 0, false
	}
	minute, ok := parseFixedDigits(value, 3, 2)
	if !ok {
		return 0, 0, 0, 0, false
	}
	second, ok := parseFixedDigits(value, 6, 2)
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

func parseFixedDigits(value string, start, length int) (int, bool) {
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

func validateTimezoneOffset(tz string) error {
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
	hour, ok := parseFixedDigits(tz, 1, 2)
	if !ok {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	minute, ok := parseFixedDigits(tz, 4, 2)
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

func isValidDate(year, month, day int) bool {
	if day < 1 || day > 31 {
		return false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Year() == year && int(t.Month()) == month && t.Day() == day
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
