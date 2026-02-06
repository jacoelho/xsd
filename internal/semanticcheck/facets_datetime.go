package semanticcheck

import (
	"errors"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

var errDateTimeNotComparable = errors.New("date/time values are not comparable")
var errDurationNotComparable = errors.New("duration values are not comparable")

// isDateTimeTypeName checks if a type name represents a date/time type.
func isDateTimeTypeName(typeName string) bool {
	switch typeName {
	case "dateTime", "date", "time", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth":
		return true
	default:
		return false
	}
}

// compareDateTimeValues compares two date/time values, returning -1, 0, or 1.
func compareDateTimeValues(v1, v2, baseTypeName string) (int, error) {
	switch baseTypeName {
	case "date":
		t1, err := parseXSDDate(v1)
		if err != nil {
			return 0, err
		}
		t2, err := parseXSDDate(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "dateTime":
		t1, err := parseXSDDateTime(v1)
		if err != nil {
			return 0, err
		}
		t2, err := parseXSDDateTime(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "time":
		t1, err := parseXSDTime(v1)
		if err != nil {
			return 0, err
		}
		t2, err := parseXSDTime(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gYear":
		t1, err := parseXSDGYear(v1)
		if err != nil {
			return 0, err
		}
		t2, err := parseXSDGYear(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gYearMonth":
		t1, err := parseXSDGYearMonth(v1)
		if err != nil {
			return 0, err
		}
		t2, err := parseXSDGYearMonth(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gMonth":
		t1, err := parseXSDGMonth(v1)
		if err != nil {
			return 0, err
		}
		t2, err := parseXSDGMonth(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gMonthDay":
		t1, err := parseXSDGMonthDay(v1)
		if err != nil {
			return 0, err
		}
		t2, err := parseXSDGMonthDay(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gDay":
		t1, err := parseXSDGDay(v1)
		if err != nil {
			return 0, err
		}
		t2, err := parseXSDGDay(v2)
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
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

func compareDateTimeOrder(t1, t2 temporal.Value) (int, error) {
	cmp, err := temporal.Compare(t1, t2)
	if err != nil {
		return 0, errDateTimeNotComparable
	}
	return cmp, nil
}

func parseXSDDate(lexical string) (temporal.Value, error) {
	return temporal.Parse(temporal.KindDate, []byte(lexical))
}

func parseXSDDateTime(lexical string) (temporal.Value, error) {
	return temporal.Parse(temporal.KindDateTime, []byte(lexical))
}

func parseXSDTime(lexical string) (temporal.Value, error) {
	return temporal.Parse(temporal.KindTime, []byte(lexical))
}

func parseXSDGYear(lexical string) (temporal.Value, error) {
	return temporal.Parse(temporal.KindGYear, []byte(lexical))
}

func parseXSDGYearMonth(lexical string) (temporal.Value, error) {
	return temporal.Parse(temporal.KindGYearMonth, []byte(lexical))
}

func parseXSDGMonth(lexical string) (temporal.Value, error) {
	return temporal.Parse(temporal.KindGMonth, []byte(lexical))
}

func parseXSDGMonthDay(lexical string) (temporal.Value, error) {
	return temporal.Parse(temporal.KindGMonthDay, []byte(lexical))
}

func parseXSDGDay(lexical string) (temporal.Value, error) {
	return temporal.Parse(temporal.KindGDay, []byte(lexical))
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
