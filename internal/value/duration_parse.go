package value

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/num"
)

var (
	// datePattern matches date components in XSD duration format: Y, M, D.
	datePattern = regexp.MustCompile(`(\d+)Y|(\d+)M|(\d+)D`)

	// timePattern matches time components in XSD duration format: H, M, S.
	timePattern = regexp.MustCompile(`(\d+)H|(\d+)M|(\d+(\.\d+)?)S`)

	// durationPattern validates full XSD duration lexical form.
	durationPattern = regexp.MustCompile(`^-?P(\d+Y)?(\d+M)?(\d+D)?(T(\d+H)?(\d+M)?(\d+(\.\d+)?S)?)?$`)
)

// Duration is a parsed XSD duration lexical value.
type Duration struct {
	Seconds  num.Dec
	Years    int
	Months   int
	Days     int
	Hours    int
	Minutes  int
	Negative bool
}

// ParseDuration parses an XSD duration lexical value.
func ParseDuration(s string) (Duration, error) {
	if s == "" {
		return Duration{}, fmt.Errorf("empty duration")
	}

	input := s
	negative, body, err := durationPrefix(s)
	if err != nil {
		return Duration{}, err
	}
	datePart, timePart, sawTimeDesignator, err := durationParts(body)
	if err != nil {
		return Duration{}, err
	}
	if !durationPattern.MatchString(input) {
		return Duration{}, fmt.Errorf("invalid duration format: %s", input)
	}
	parsed, hasDateComponent, err := parseDateDurationPart(datePart)
	if err != nil {
		return Duration{}, err
	}
	hasTimeComponent, err := parseTimeDurationPart(&parsed, timePart)
	if err != nil {
		return Duration{}, err
	}
	if !hasDateComponent && !hasTimeComponent {
		return Duration{}, fmt.Errorf("duration must have at least one component")
	}
	if sawTimeDesignator && !hasTimeComponent {
		return Duration{}, fmt.Errorf("time designator present but no time components specified")
	}
	if isZeroDuration(parsed) {
		negative = false
	}
	parsed.Negative = negative
	return parsed, nil
}

func durationPrefix(s string) (bool, string, error) {
	negative := s[0] == '-'
	if negative {
		s = s[1:]
	}
	if s == "" || s[0] != 'P' {
		return false, "", fmt.Errorf("duration must start with P")
	}
	return negative, s[1:], nil
}

func durationParts(s string) (string, string, bool, error) {
	datePart := s
	timePart := ""
	sawTimeDesignator := false
	if before, after, ok := strings.Cut(s, "T"); ok {
		sawTimeDesignator = true
		datePart = before
		timePart = after
		if strings.IndexByte(timePart, 'T') != -1 {
			return "", "", false, fmt.Errorf("invalid duration format: multiple T separators")
		}
	}
	return datePart, timePart, sawTimeDesignator, nil
}

func parseDateDurationPart(datePart string) (Duration, bool, error) {
	var out Duration
	hasComponent := false
	for _, match := range datePattern.FindAllStringSubmatch(datePart, -1) {
		if match[1] != "" {
			val, err := parseDurationComponent(match[1], "year")
			if err != nil {
				return Duration{}, false, err
			}
			out.Years = val
			hasComponent = true
		}
		if match[2] != "" {
			val, err := parseDurationComponent(match[2], "month")
			if err != nil {
				return Duration{}, false, err
			}
			out.Months = val
			hasComponent = true
		}
		if match[3] != "" {
			val, err := parseDurationComponent(match[3], "day")
			if err != nil {
				return Duration{}, false, err
			}
			out.Days = val
			hasComponent = true
		}
	}
	return out, hasComponent, nil
}

func parseTimeDurationPart(out *Duration, timePart string) (bool, error) {
	hasComponent := false
	for _, match := range timePattern.FindAllStringSubmatch(timePart, -1) {
		if match[1] != "" {
			val, err := parseDurationComponent(match[1], "hour")
			if err != nil {
				return false, err
			}
			out.Hours = val
			hasComponent = true
		}
		if match[2] != "" {
			val, err := parseDurationComponent(match[2], "minute")
			if err != nil {
				return false, err
			}
			out.Minutes = val
			hasComponent = true
		}
		if match[3] != "" {
			seconds, err := parseDurationSeconds(match[3])
			if err != nil {
				return false, err
			}
			out.Seconds = seconds
			hasComponent = true
		}
	}
	return hasComponent, nil
}

func parseDurationComponent(value, label string) (int, error) {
	u, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return 0, fmt.Errorf("%s value too large", label)
		}
		return 0, fmt.Errorf("invalid %s value: %w", label, err)
	}
	maxComponent := uint64(^uint(0) >> 1)
	if u > maxComponent {
		return 0, fmt.Errorf("%s value too large", label)
	}
	return int(u), nil
}

func parseDurationSeconds(value string) (num.Dec, error) {
	dec, err := num.ParseDec([]byte(value))
	if err != nil {
		return num.Dec{}, fmt.Errorf("invalid second value: %w", err)
	}
	if dec.Sign < 0 {
		return num.Dec{}, fmt.Errorf("second value cannot be negative")
	}
	return dec, nil
}

func isZeroDuration(v Duration) bool {
	return v.Years == 0 &&
		v.Months == 0 &&
		v.Days == 0 &&
		v.Hours == 0 &&
		v.Minutes == 0 &&
		v.Seconds.Sign == 0
}
