package temporal

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/datetime"
)

// Kind identifies an XSD temporal primitive.
type Kind uint8

const (
	KindInvalid Kind = iota
	KindDateTime
	KindDate
	KindTime
	KindGYearMonth
	KindGYear
	KindGMonthDay
	KindGDay
	KindGMonth
)

// KindFromPrimitiveName resolves an XSD primitive name to a temporal kind.
func KindFromPrimitiveName(name string) (Kind, bool) {
	switch name {
	case "dateTime":
		return KindDateTime, true
	case "date":
		return KindDate, true
	case "time":
		return KindTime, true
	case "gYearMonth":
		return KindGYearMonth, true
	case "gYear":
		return KindGYear, true
	case "gMonthDay":
		return KindGMonthDay, true
	case "gDay":
		return KindGDay, true
	case "gMonth":
		return KindGMonth, true
	default:
		return KindInvalid, false
	}
}

func (k Kind) String() string {
	switch k {
	case KindDateTime:
		return "dateTime"
	case KindDate:
		return "date"
	case KindTime:
		return "time"
	case KindGYearMonth:
		return "gYearMonth"
	case KindGYear:
		return "gYear"
	case KindGMonthDay:
		return "gMonthDay"
	case KindGDay:
		return "gDay"
	case KindGMonth:
		return "gMonth"
	default:
		return "invalid"
	}
}

// TimezoneKind mirrors timezone presence for temporal values.
type TimezoneKind uint8

const (
	TZNone TimezoneKind = iota
	TZKnown
)

// Value stores temporal semantics used for equality, ordering, and keying.
type Value struct {
	Kind Kind

	// Time stores the parsed value using the existing parser behavior.
	Time time.Time

	TimezoneKind TimezoneKind

	// LeapSecond preserves lexical second=60 identity for time/dateTime.
	LeapSecond bool
}

// ErrIndeterminateComparison matches XSD indeterminate temporal ordering.
var ErrIndeterminateComparison = errors.New("time comparison indeterminate")

// ParsePrimitive parses an XSD temporal lexical value by primitive name.
func ParsePrimitive(primitiveName string, lexical []byte) (Value, error) {
	kind, ok := KindFromPrimitiveName(primitiveName)
	if !ok {
		return Value{}, fmt.Errorf("unsupported temporal primitive %q", primitiveName)
	}
	return Parse(kind, lexical)
}

// Parse parses an XSD temporal lexical value into semantic representation.
func Parse(kind Kind, lexical []byte) (Value, error) {
	trimmed := value.TrimXMLWhitespace(lexical)
	tzKind := fromValueTimezoneKind(value.TimezoneKindFromLexical(trimmed))

	switch kind {
	case KindDateTime:
		parsed, err := value.ParseDateTime(trimmed)
		if err != nil {
			return Value{}, err
		}
		return Value{
			Kind:         kind,
			Time:         parsed,
			TimezoneKind: tzKind,
			LeapSecond:   hasDateTimeLeapSecond(trimmed),
		}, nil
	case KindDate:
		parsed, err := value.ParseDate(trimmed)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: kind, Time: parsed, TimezoneKind: tzKind}, nil
	case KindTime:
		parsed, err := value.ParseTime(trimmed)
		if err != nil {
			return Value{}, err
		}
		return Value{
			Kind:         kind,
			Time:         parsed,
			TimezoneKind: tzKind,
			LeapSecond:   hasTimeLeapSecond(trimmed),
		}, nil
	case KindGYearMonth:
		parsed, err := value.ParseGYearMonth(trimmed)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: kind, Time: parsed, TimezoneKind: tzKind}, nil
	case KindGYear:
		parsed, err := value.ParseGYear(trimmed)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: kind, Time: parsed, TimezoneKind: tzKind}, nil
	case KindGMonthDay:
		parsed, err := value.ParseGMonthDay(trimmed)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: kind, Time: parsed, TimezoneKind: tzKind}, nil
	case KindGDay:
		parsed, err := value.ParseGDay(trimmed)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: kind, Time: parsed, TimezoneKind: tzKind}, nil
	case KindGMonth:
		parsed, err := value.ParseGMonth(trimmed)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: kind, Time: parsed, TimezoneKind: tzKind}, nil
	default:
		return Value{}, fmt.Errorf("unsupported temporal kind %d", kind)
	}
}

// Equal compares temporal values in XSD value space for the same primitive.
func Equal(left, right Value) bool {
	if left.Kind == KindInvalid || right.Kind == KindInvalid {
		return false
	}
	if left.Kind != right.Kind || left.TimezoneKind != right.TimezoneKind {
		return false
	}

	switch left.Kind {
	case KindTime:
		l := left.Time
		r := right.Time
		if left.TimezoneKind == TZKnown {
			l = l.UTC()
			r = r.UTC()
		}
		if !sameClock(l, r) {
			return false
		}
		return left.LeapSecond == right.LeapSecond
	case KindDateTime:
		l := left.Time
		r := right.Time
		if left.TimezoneKind == TZKnown {
			l = l.UTC()
			r = r.UTC()
		}
		if !l.Equal(r) {
			return false
		}
		return left.LeapSecond == right.LeapSecond
	default:
		l := left.Time
		r := right.Time
		if left.TimezoneKind == TZKnown {
			l = l.UTC()
			r = r.UTC()
		}
		return l.Equal(r)
	}
}

// Compare compares temporal values in XSD order.
func Compare(left, right Value) (int, error) {
	if left.Kind == KindInvalid || right.Kind == KindInvalid {
		return 0, fmt.Errorf("cannot compare invalid temporal values")
	}
	if left.Kind != right.Kind {
		return 0, fmt.Errorf("cannot compare %s with %s", left.Kind, right.Kind)
	}
	if left.TimezoneKind == right.TimezoneKind {
		return compareSameTimezone(left, right), nil
	}
	if left.TimezoneKind == TZKnown {
		return compareKnownToLocal(left, right)
	}
	cmp, err := compareKnownToLocal(right, left)
	if err != nil {
		return 0, err
	}
	return -cmp, nil
}

// Canonical returns the canonical lexical representation for temporal values.
func Canonical(v Value) string {
	if v.Kind == KindInvalid {
		return ""
	}
	if v.LeapSecond && (v.Kind == KindDateTime || v.Kind == KindTime) {
		return canonicalLeap(v)
	}
	return value.CanonicalDateTimeString(v.Time, v.Kind.String(), toValueTimezoneKind(v.TimezoneKind))
}

// ValueTimezoneKind converts temporal timezone kind to value.TimezoneKind.
func ValueTimezoneKind(kind TimezoneKind) value.TimezoneKind {
	return toValueTimezoneKind(kind)
}

func compareSameTimezone(left, right Value) int {
	switch left.Kind {
	case KindTime:
		l := left.Time
		r := right.Time
		if left.TimezoneKind == TZKnown {
			l = l.UTC()
			r = r.UTC()
		}
		if l.Before(r) {
			return -1
		}
		if l.After(r) {
			return 1
		}
		return compareLeapTie(left, right)
	case KindDateTime:
		l := left.Time
		r := right.Time
		if left.TimezoneKind == TZKnown {
			l = l.UTC()
			r = r.UTC()
		}
		if l.Before(r) {
			return -1
		}
		if l.After(r) {
			return 1
		}
		return compareLeapTie(left, right)
	default:
		l := left.Time
		r := right.Time
		if left.TimezoneKind == TZKnown {
			l = l.UTC()
			r = r.UTC()
		}
		if l.Before(r) {
			return -1
		}
		if l.After(r) {
			return 1
		}
		return 0
	}
}

func compareKnownToLocal(known, local Value) (int, error) {
	knownUTC := known.Time.UTC()
	localUTC := local.Time.UTC()
	localPlus14 := localUTC.Add(-14 * time.Hour)
	localMinus14 := localUTC.Add(14 * time.Hour)
	if knownUTC.Before(localPlus14) {
		return -1, nil
	}
	if knownUTC.After(localMinus14) {
		return 1, nil
	}
	return 0, ErrIndeterminateComparison
}

func compareLeapTie(left, right Value) int {
	if left.LeapSecond == right.LeapSecond {
		return 0
	}
	if left.LeapSecond {
		return -1
	}
	return 1
}

func sameClock(left, right time.Time) bool {
	return left.Hour() == right.Hour() &&
		left.Minute() == right.Minute() &&
		left.Second() == right.Second() &&
		left.Nanosecond() == right.Nanosecond()
}

func hasDateTimeLeapSecond(lexical []byte) bool {
	main, _ := datetime.SplitTimezone(string(lexical))
	t := strings.IndexByte(main, 'T')
	if t < 0 {
		return false
	}
	timePart := main[t+1:]
	hour, minute, second, _, ok := datetime.ParseTimeParts(timePart)
	if !ok {
		return false
	}
	return hour == 23 && minute == 59 && second == 60
}

func hasTimeLeapSecond(lexical []byte) bool {
	main, _ := datetime.SplitTimezone(string(lexical))
	hour, minute, second, _, ok := datetime.ParseTimeParts(main)
	if !ok {
		return false
	}
	return hour == 23 && minute == 59 && second == 60
}

func canonicalLeap(v Value) string {
	t := v.Time
	if v.TimezoneKind == TZKnown {
		t = t.UTC()
	}
	adjusted := t.Add(-time.Second)
	hour, minute, _ := adjusted.Clock()
	if hour != 23 || minute != 59 {
		return value.CanonicalDateTimeString(v.Time, v.Kind.String(), toValueTimezoneKind(v.TimezoneKind))
	}
	fraction := formatFraction(adjusted.Nanosecond())
	tz := ""
	if v.TimezoneKind == TZKnown {
		tz = "Z"
	}
	if v.Kind == KindTime {
		return fmt.Sprintf("%02d:%02d:60%s%s", hour, minute, fraction, tz)
	}
	year, month, day := adjusted.Date()
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:60%s%s", year, int(month), day, hour, minute, fraction, tz)
}

func formatFraction(nanos int) string {
	if nanos == 0 {
		return ""
	}
	frac := fmt.Sprintf("%09d", nanos)
	frac = strings.TrimRight(frac, "0")
	return "." + frac
}

func fromValueTimezoneKind(kind value.TimezoneKind) TimezoneKind {
	if kind == value.TZKnown {
		return TZKnown
	}
	return TZNone
}

func toValueTimezoneKind(kind TimezoneKind) value.TimezoneKind {
	if kind == TZKnown {
		return value.TZKnown
	}
	return value.TZNone
}
