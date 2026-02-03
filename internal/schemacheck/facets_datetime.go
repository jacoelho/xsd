package schemacheck

import (
	"errors"
	"time"

	"github.com/jacoelho/xsd/internal/types"
	xsdvalue "github.com/jacoelho/xsd/internal/value"
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

func parseTemporal(value string, parse func([]byte) (time.Time, error)) (time.Time, bool, error) {
	lexical := []byte(value)
	parsed, err := parse(lexical)
	if err != nil {
		return time.Time{}, false, err
	}
	return parsed, xsdvalue.HasTimezone(lexical), nil
}

func parseXSDDate(value string) (time.Time, bool, error) {
	return parseTemporal(value, xsdvalue.ParseDate)
}

func parseXSDDateTime(value string) (time.Time, bool, error) {
	return parseTemporal(value, xsdvalue.ParseDateTime)
}

func parseXSDTime(value string) (time.Time, bool, error) {
	return parseTemporal(value, xsdvalue.ParseTime)
}

func parseXSDGYear(value string) (time.Time, bool, error) {
	return parseTemporal(value, xsdvalue.ParseGYear)
}

func parseXSDGYearMonth(value string) (time.Time, bool, error) {
	return parseTemporal(value, xsdvalue.ParseGYearMonth)
}

func parseXSDGMonth(value string) (time.Time, bool, error) {
	return parseTemporal(value, xsdvalue.ParseGMonth)
}

func parseXSDGMonthDay(value string) (time.Time, bool, error) {
	return parseTemporal(value, xsdvalue.ParseGMonthDay)
}

func parseXSDGDay(value string) (time.Time, bool, error) {
	return parseTemporal(value, xsdvalue.ParseGDay)
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
