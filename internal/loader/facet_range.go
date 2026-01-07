package loader

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/types"
)

var errDateTimeNotComparable = errors.New("date/time values are not comparable")
var errDurationNotComparable = errors.New("duration values are not comparable")

// validateRangeFacets validates consistency of range facets
func validateRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseTypeName string, bt *types.BuiltinType) error {
	if baseTypeName == "duration" {
		return validateDurationRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive)
	}
	// per XSD spec: maxInclusive and maxExclusive cannot both be present
	if maxInclusive != nil && maxExclusive != nil {
		return fmt.Errorf("maxInclusive and maxExclusive cannot both be specified")
	}

	// per XSD spec: minInclusive and minExclusive cannot both be present
	if minInclusive != nil && minExclusive != nil {
		return fmt.Errorf("minInclusive and minExclusive cannot both be specified")
	}

	// minExclusive > maxInclusive is invalid
	if minExclusive != nil && maxInclusive != nil {
		if err := compareRangeValues(baseTypeName, bt); err == nil {
			// values are comparable, check if minExclusive >= maxInclusive
			if compareNumericOrString(*minExclusive, *maxInclusive, baseTypeName, bt) >= 0 {
				return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
			}
		}
	}

	// minExclusive >= maxExclusive is invalid
	if minExclusive != nil && maxExclusive != nil {
		err := compareRangeValues(baseTypeName, bt)
		if err == nil {
			if compareNumericOrString(*minExclusive, *maxExclusive, baseTypeName, bt) >= 0 {
				return fmt.Errorf("minExclusive (%s) must be < maxExclusive (%s)", *minExclusive, *maxExclusive)
			}
		} else {
			// if compareRangeValues failed (e.g., bt is nil), try comparison anyway for known types
			if isDateTimeTypeName(baseTypeName) || isNumericTypeName(baseTypeName) {
				if compareNumericOrString(*minExclusive, *maxExclusive, baseTypeName, bt) >= 0 {
					return fmt.Errorf("minExclusive (%s) must be < maxExclusive (%s)", *minExclusive, *maxExclusive)
				}
			}
		}
	}

	// minInclusive > maxInclusive is invalid
	if minInclusive != nil && maxInclusive != nil {
		err := compareRangeValues(baseTypeName, bt)
		if err == nil {
			if compareNumericOrString(*minInclusive, *maxInclusive, baseTypeName, bt) > 0 {
				return fmt.Errorf("minInclusive (%s) must be <= maxInclusive (%s)", *minInclusive, *maxInclusive)
			}
		} else {
			// if compareRangeValues failed (e.g., bt is nil), try comparison anyway for known types
			if isDateTimeTypeName(baseTypeName) || isNumericTypeName(baseTypeName) {
				if compareNumericOrString(*minInclusive, *maxInclusive, baseTypeName, bt) > 0 {
					return fmt.Errorf("minInclusive (%s) must be <= maxInclusive (%s)", *minInclusive, *maxInclusive)
				}
			}
		}
	}

	// minInclusive >= maxExclusive is invalid
	if minInclusive != nil && maxExclusive != nil {
		if err := compareRangeValues(baseTypeName, bt); err == nil {
			if compareNumericOrString(*minInclusive, *maxExclusive, baseTypeName, bt) >= 0 {
				return fmt.Errorf("minInclusive (%s) must be < maxExclusive (%s)", *minInclusive, *maxExclusive)
			}
		}
	}

	// minExclusive >= maxInclusive is invalid (already checked above, but also check with inclusive)
	if minExclusive != nil && maxInclusive != nil {
		if err := compareRangeValues(baseTypeName, bt); err == nil {
			if compareNumericOrString(*minExclusive, *maxInclusive, baseTypeName, bt) >= 0 {
				return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
			}
		}
	}

	return nil
}

func validateDurationRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive *string) error {
	compare := func(v1, v2 string) (int, bool, error) {
		cmp, err := compareDurationValues(v1, v2)
		if errors.Is(err, errDurationNotComparable) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return cmp, true, nil
	}

	if maxInclusive != nil && maxExclusive != nil {
		return fmt.Errorf("maxInclusive and maxExclusive cannot both be specified")
	}
	if minInclusive != nil && minExclusive != nil {
		return fmt.Errorf("minInclusive and minExclusive cannot both be specified")
	}

	if minExclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minExclusive/maxInclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
		}
	}
	if minExclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minExclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxExclusive (%s)", *minExclusive, *maxExclusive)
		}
	}
	if minInclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minInclusive/maxInclusive: %w", err)
		} else if ok && cmp > 0 {
			return fmt.Errorf("minInclusive (%s) must be <= maxInclusive (%s)", *minInclusive, *maxInclusive)
		}
	}
	if minInclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minInclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minInclusive (%s) must be < maxExclusive (%s)", *minInclusive, *maxExclusive)
		}
	}
	if minExclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minExclusive/maxInclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
		}
	}

	return nil
}

// validateRangeFacetValues validates that range facet values are within the base type's value space
// Per XSD spec, facet values must be valid for the base type
func validateRangeFacetValues(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseType types.Type, bt *types.BuiltinType) error {
	// try to get a validator and whitespace handling for the base type
	var validator types.TypeValidator
	var whiteSpace types.WhiteSpace

	if bt != nil {
		validator = func(value string) error {
			return bt.Validate(value)
		}
		whiteSpace = bt.WhiteSpace()
	} else if baseType != nil {
		// for user-defined types, try to get the underlying built-in type validator
		switch t := baseType.(type) {
		case *types.BuiltinType:
			validator = func(value string) error {
				return t.Validate(value)
			}
			whiteSpace = t.WhiteSpace()
		case *types.SimpleType:
			// for SimpleType, check if it has a built-in base
			if t.IsBuiltin() || t.QName.Namespace == types.XSDNamespace {
				if builtinType := types.GetBuiltinNS(t.QName.Namespace, t.QName.Local); builtinType != nil {
					validator = func(value string) error {
						return builtinType.Validate(value)
					}
					whiteSpace = builtinType.WhiteSpace()
				}
			} else {
				builtinType := findBuiltinAncestor(baseType)
				if builtinType != nil {
					validator = func(value string) error {
						return builtinType.Validate(value)
					}
					whiteSpace = builtinType.WhiteSpace()
				}
			}
		}
	}

	if validator == nil {
		return nil // can't validate without a validator
	}

	// helper to normalize whitespace before validation
	normalizeValue := func(val string) string {
		switch whiteSpace {
		case types.WhiteSpaceCollapse:
			// collapse: replace sequences of whitespace with single space, trim leading/trailing
			val = strings.TrimSpace(val)
			// replace multiple whitespace with single space
			return joinFields(val)
		case types.WhiteSpaceReplace:
			// replace: replace all whitespace chars with spaces
			return strings.Map(func(r rune) rune {
				if r == '\t' || r == '\n' || r == '\r' {
					return ' '
				}
				return r
			}, val)
		default:
			return val
		}
	}

	if minExclusive != nil {
		normalized := normalizeValue(*minExclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("minExclusive value %q is not valid for base type: %w", *minExclusive, err)
		}
	}
	if maxExclusive != nil {
		normalized := normalizeValue(*maxExclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("maxExclusive value %q is not valid for base type: %w", *maxExclusive, err)
		}
	}
	if minInclusive != nil {
		normalized := normalizeValue(*minInclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("minInclusive value %q is not valid for base type: %w", *minInclusive, err)
		}
	}
	if maxInclusive != nil {
		normalized := normalizeValue(*maxInclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("maxInclusive value %q is not valid for base type: %w", *maxInclusive, err)
		}
	}

	return nil
}

func joinFields(value string) string {
	var b strings.Builder
	first := true
	for field := range strings.FieldsSeq(value) {
		if !first {
			b.WriteByte(' ')
		}
		first = false
		b.WriteString(field)
	}
	return b.String()
}

// findBuiltinAncestor walks up the type hierarchy to find the nearest built-in type
func findBuiltinAncestor(t types.Type) *types.BuiltinType {
	visited := make(map[types.Type]bool)
	current := t
	for current != nil && !visited[current] {
		visited[current] = true

		switch ct := current.(type) {
		case *types.BuiltinType:
			return ct
		case *types.SimpleType:
			if ct.IsBuiltin() || ct.QName.Namespace == types.XSDNamespace {
				if bt := types.GetBuiltinNS(ct.QName.Namespace, ct.QName.Local); bt != nil {
					return bt
				}
			}
		}

		current = current.BaseType()
	}
	return nil
}

// compareRangeValues returns an error if range values cannot be compared for the base type.
func compareRangeValues(baseTypeName string, bt *types.BuiltinType) error {
	if baseTypeName == "duration" {
		return nil
	}
	if bt == nil || !bt.Ordered() {
		return fmt.Errorf("cannot compare values for non-ordered type")
	}
	return nil
}

// compareNumericOrString compares two values, returning -1, 0, or 1
func compareNumericOrString(v1, v2, baseTypeName string, bt *types.BuiltinType) int {
	// if bt is nil, try to compare anyway if it's a known date/time or numeric type
	if bt == nil {
		if baseTypeName == "duration" {
			if cmp, err := compareDurationValues(v1, v2); err == nil {
				return cmp
			}
			return 0
		}
		// for date/time types, we can still compare using string comparison
		if isDateTimeTypeName(baseTypeName) {
			if cmp, err := compareDateTimeValues(v1, v2, baseTypeName); err == nil {
				return cmp
			}
			return 0
		}
		// for numeric types, try parsing
		if isNumericTypeName(baseTypeName) {
			val1, err1 := strconv.ParseFloat(v1, 64)
			val2, err2 := strconv.ParseFloat(v2, 64)
			if err1 == nil && err2 == nil {
				if val1 < val2 {
					return -1
				}
				if val1 > val2 {
					return 1
				}
				return 0
			}
		}
		return 0 // can't compare without type info
	}

	if !bt.Ordered() {
		return 0 // can't compare
	}

	// try numeric comparison first
	if isNumericTypeName(baseTypeName) {
		val1, err1 := strconv.ParseFloat(v1, 64)
		val2, err2 := strconv.ParseFloat(v2, 64)
		if err1 == nil && err2 == nil {
			if val1 < val2 {
				return -1
			}
			if val1 > val2 {
				return 1
			}
			return 0
		}
	}

	if baseTypeName == "duration" {
		if cmp, err := compareDurationValues(v1, v2); err == nil {
			return cmp
		}
	}

	// for date/time types, try to parse and compare as dates
	if isDateTimeTypeName(baseTypeName) {
		if result, err := compareDateTimeValues(v1, v2, baseTypeName); err == nil && result != 0 {
			return result
		}
	}

	// fall back to string comparison
	if v1 < v2 {
		return -1
	}
	if v1 > v2 {
		return 1
	}
	return 0
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
		if tz1 != tz2 {
			return 0, errDateTimeNotComparable
		}
		return compareTimes(t1, t2), nil
	case "dateTime":
		t1, tz1, err := parseXSDDateTime(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDDateTime(v2)
		if err != nil {
			return 0, err
		}
		if tz1 != tz2 {
			return 0, errDateTimeNotComparable
		}
		return compareTimes(t1, t2), nil
	case "time":
		t1, tz1, err := parseXSDTime(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDTime(v2)
		if err != nil {
			return 0, err
		}
		if tz1 != tz2 {
			return 0, errDateTimeNotComparable
		}
		return compareTimes(t1, t2), nil
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

func parseXSDDate(value string) (time.Time, bool, error) {
	base, hasTZ, offset, err := splitTimezone(value)
	if err != nil {
		return time.Time{}, false, err
	}
	t, err := time.Parse("2006-01-02", base)
	if err != nil {
		return time.Time{}, false, err
	}
	if hasTZ {
		loc := time.FixedZone("", offset)
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).UTC()
	}
	return t, hasTZ, nil
}

func parseXSDDateTime(value string) (time.Time, bool, error) {
	base, hasTZ, offset, err := splitTimezone(value)
	if err != nil {
		return time.Time{}, false, err
	}
	layouts := []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	var parsed time.Time
	var parseErr error
	for _, layout := range layouts {
		parsed, parseErr = time.Parse(layout, base)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return time.Time{}, false, parseErr
	}
	if hasTZ {
		loc := time.FixedZone("", offset)
		parsed = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), loc).UTC()
	}
	return parsed, hasTZ, nil
}

func parseXSDTime(value string) (time.Time, bool, error) {
	base, hasTZ, offset, err := splitTimezone(value)
	if err != nil {
		return time.Time{}, false, err
	}
	layouts := []string{
		"15:04:05.999999999",
		"15:04:05",
	}
	var parsed time.Time
	var parseErr error
	for _, layout := range layouts {
		parsed, parseErr = time.Parse(layout, base)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return time.Time{}, false, parseErr
	}
	loc := time.UTC
	if hasTZ {
		loc = time.FixedZone("", offset)
	}
	parsed = time.Date(2000, 1, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), loc).UTC()
	return parsed, hasTZ, nil
}

func compareDurationValues(v1, v2 string) (int, error) {
	months1, seconds1, err := parseDurationParts(v1)
	if err != nil {
		return 0, err
	}
	months2, seconds2, err := parseDurationParts(v2)
	if err != nil {
		return 0, err
	}

	if months1 == months2 && seconds1 == seconds2 {
		return 0, nil
	}
	if months1 <= months2 && seconds1 <= seconds2 {
		return -1, nil
	}
	if months1 >= months2 && seconds1 >= seconds2 {
		return 1, nil
	}
	return 0, errDurationNotComparable
}

func parseDurationParts(value string) (int, float64, error) {
	if value == "" {
		return 0, 0, fmt.Errorf("empty duration")
	}

	negative := value[0] == '-'
	if negative {
		value = value[1:]
	}
	if len(value) == 0 || value[0] != 'P' {
		return 0, 0, fmt.Errorf("duration must start with P")
	}
	value = value[1:]

	var years, months, days, hours, minutes int
	var seconds float64

	datePart := value
	timePart := ""
	if before, after, ok := strings.Cut(value, "T"); ok {
		datePart = before
		timePart = after
		if extra := strings.IndexByte(timePart, 'T'); extra != -1 {
			timePart = timePart[:extra]
		}
	}

	datePattern := regexp.MustCompile(`([0-9]+)Y|([0-9]+)M|([0-9]+)D`)
	matches := datePattern.FindAllStringSubmatch(datePart, -1)
	for _, match := range matches {
		if match[1] != "" {
			years, _ = strconv.Atoi(match[1])
		}
		if match[2] != "" {
			months, _ = strconv.Atoi(match[2])
		}
		if match[3] != "" {
			days, _ = strconv.Atoi(match[3])
		}
	}

	if timePart != "" {
		timePattern := regexp.MustCompile(`([0-9]+)H|([0-9]+)M|([0-9]+(?:\.[0-9]+)?)S`)
		matches = timePattern.FindAllStringSubmatch(timePart, -1)
		for _, match := range matches {
			if match[1] != "" {
				hours, _ = strconv.Atoi(match[1])
			}
			if match[2] != "" {
				minutes, _ = strconv.Atoi(match[2])
			}
			if match[3] != "" {
				seconds, _ = strconv.ParseFloat(match[3], 64)
			}
		}
	}

	totalMonths := years*12 + months
	totalSeconds := float64(days*24*60*60+hours*60*60+minutes*60) + seconds

	if negative {
		totalMonths = -totalMonths
		totalSeconds = -totalSeconds
	}

	return totalMonths, totalSeconds, nil
}
