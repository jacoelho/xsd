package schemacheck

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/types"
)

var errDateTimeNotComparable = errors.New("date/time values are not comparable")
var errDurationNotComparable = errors.New("duration values are not comparable")

// isDateTimeTypeName checks if a type name represents a date/time type
func isDateTimeTypeName(typeName string) bool {
	switch typeName {
	case "dateTime", "date", "time", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth":
		return true
	default:
		return false
	}
}

// compareDateTimeValues compares two date/time values, returning -1, 0, or 1
func compareDateTimeValues(v1, v2, baseTypeName string) (int, error) {
	switch baseTypeName {
	case "date":
		t1, tz1, err := parseXSDDate(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDDate(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, tz1, t2, tz2)
	case "dateTime":
		t1, tz1, err := parseXSDDateTime(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDDateTime(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, tz1, t2, tz2)
	case "time":
		t1, tz1, err := parseXSDTime(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDTime(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, tz1, t2, tz2)
	case "gYear":
		t1, tz1, err := parseXSDGYear(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDGYear(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, tz1, t2, tz2)
	case "gYearMonth":
		t1, tz1, err := parseXSDGYearMonth(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDGYearMonth(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, tz1, t2, tz2)
	case "gMonth":
		t1, tz1, err := parseXSDGMonth(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDGMonth(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, tz1, t2, tz2)
	case "gMonthDay":
		t1, tz1, err := parseXSDGMonthDay(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDGMonthDay(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, tz1, t2, tz2)
	case "gDay":
		t1, tz1, err := parseXSDGDay(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDGDay(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, tz1, t2, tz2)
	}

	// fallback: lexicographic comparison for other date/time types.
	if v1 < v2 {
		return -1, nil
	}
	if v1 > v2 {
		return 1, nil
	}
	return 0, nil
}

func compareTimes(t1, t2 time.Time) int {
	if t1.Before(t2) {
		return -1
	}
	if t1.After(t2) {
		return 1
	}
	return 0
}

func compareDateTimeOrder(t1 time.Time, tz1 bool, t2 time.Time, tz2 bool) (int, error) {
	if tz1 == tz2 {
		return compareTimes(t1, t2), nil
	}
	cmp, err := types.ComparableTime{Value: t1, HasTimezone: tz1}.Compare(types.ComparableTime{Value: t2, HasTimezone: tz2})
	if err != nil {
		return 0, errDateTimeNotComparable
	}
	return cmp, nil
}

func splitTimezone(value string) (string, bool, int, error) {
	if before, ok := strings.CutSuffix(value, "Z"); ok {
		return before, true, 0, nil
	}
	if len(value) >= 6 {
		sep := value[len(value)-6]
		if (sep == '+' || sep == '-') && value[len(value)-3] == ':' {
			base := value[:len(value)-6]
			hours, err := strconv.Atoi(value[len(value)-5 : len(value)-3])
			if err != nil {
				return "", false, 0, fmt.Errorf("invalid timezone offset in %q", value)
			}
			mins, err := strconv.Atoi(value[len(value)-2:])
			if err != nil {
				return "", false, 0, fmt.Errorf("invalid timezone offset in %q", value)
			}
			offset := hours*3600 + mins*60
			if sep == '-' {
				offset = -offset
			}
			return base, true, offset, nil
		}
	}
	return value, false, 0, nil
}

func parseXSDDateTimeBase(value string, parse func(string) (time.Time, error), normalize func(time.Time, bool, int) time.Time) (time.Time, bool, error) {
	base, hasTZ, offset, err := splitTimezone(value)
	if err != nil {
		return time.Time{}, false, err
	}
	parsed, err := parse(base)
	if err != nil {
		return time.Time{}, false, err
	}
	if normalize != nil {
		parsed = normalize(parsed, hasTZ, offset)
	}
	return parsed, hasTZ, nil
}

func parseWithLayouts(base string, layouts []string) (time.Time, error) {
	var parsed time.Time
	var parseErr error
	for _, layout := range layouts {
		parsed, parseErr = time.Parse(layout, base)
		if parseErr == nil {
			return parsed, nil
		}
	}
	return time.Time{}, parseErr
}

func normalizeWithTimezone(parsed time.Time, hasTZ bool, offset int) time.Time {
	if !hasTZ {
		return parsed
	}
	loc := time.FixedZone("", offset)
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), loc).UTC()
}

func normalizeTimeOfDay(parsed time.Time, hasTZ bool, offset int) time.Time {
	loc := time.UTC
	if hasTZ {
		loc = time.FixedZone("", offset)
	}
	return time.Date(2000, 1, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), loc).UTC()
}

func parseXSDDate(value string) (time.Time, bool, error) {
	return parseXSDDateTimeBase(value, func(base string) (time.Time, error) {
		return time.Parse("2006-01-02", base)
	}, normalizeWithTimezone)
}

func parseXSDDateTime(value string) (time.Time, bool, error) {
	layouts := []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	return parseXSDDateTimeBase(value, func(base string) (time.Time, error) {
		return parseWithLayouts(base, layouts)
	}, normalizeWithTimezone)
}

func parseXSDTime(value string) (time.Time, bool, error) {
	layouts := []string{
		"15:04:05.999999999",
		"15:04:05",
	}
	return parseXSDDateTimeBase(value, func(base string) (time.Time, error) {
		return parseWithLayouts(base, layouts)
	}, normalizeTimeOfDay)
}

func parseXSDGYear(value string) (time.Time, bool, error) {
	return parseXSDDateTimeBase(value, func(base string) (time.Time, error) {
		return time.Parse("2006", base)
	}, normalizeWithTimezone)
}

func parseXSDGYearMonth(value string) (time.Time, bool, error) {
	return parseXSDDateTimeBase(value, func(base string) (time.Time, error) {
		return time.Parse("2006-01", base)
	}, normalizeWithTimezone)
}

func parseXSDGMonth(value string) (time.Time, bool, error) {
	return parseXSDDateTimeBase(value, func(base string) (time.Time, error) {
		testValue := "2000" + base
		return time.Parse("2006--01", testValue)
	}, normalizeWithTimezone)
}

func parseXSDGMonthDay(value string) (time.Time, bool, error) {
	return parseXSDDateTimeBase(value, func(base string) (time.Time, error) {
		testValue := "2000" + base
		return time.Parse("2006--01-02", testValue)
	}, normalizeWithTimezone)
}

func parseXSDGDay(value string) (time.Time, bool, error) {
	return parseXSDDateTimeBase(value, func(base string) (time.Time, error) {
		testValue := "2000-01" + base
		return time.Parse("2006-01---02", testValue)
	}, normalizeWithTimezone)
}

func compareDurationValues(v1, v2 string) (int, error) {
	left, err := types.ParseXSDDuration(v1)
	if err != nil {
		return 0, err
	}
	right, err := types.ParseXSDDuration(v2)
	if err != nil {
		return 0, err
	}
	cmp, err := types.ComparableXSDDuration{Value: left}.Compare(types.ComparableXSDDuration{Value: right})
	if err != nil {
		return 0, errDurationNotComparable
	}
	return cmp, nil
}
