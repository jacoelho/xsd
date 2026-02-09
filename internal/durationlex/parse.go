package durationlex

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

// Parse parses an XSD duration lexical value.
func Parse(s string) (Duration, error) {
	if s == "" {
		return Duration{}, fmt.Errorf("empty duration")
	}

	input := s
	negative := s[0] == '-'
	if negative {
		s = s[1:]
	}

	if s == "" || s[0] != 'P' {
		return Duration{}, fmt.Errorf("duration must start with P")
	}
	s = s[1:]

	datePart := s
	timePart := ""
	sawTimeDesignator := false
	if before, after, ok := strings.Cut(s, "T"); ok {
		sawTimeDesignator = true
		datePart = before
		timePart = after
		if strings.IndexByte(timePart, 'T') != -1 {
			return Duration{}, fmt.Errorf("invalid duration format: multiple T separators")
		}
	}

	if !durationPattern.MatchString(input) {
		return Duration{}, fmt.Errorf("invalid duration format: %s", input)
	}

	var years, months, days, hours, minutes int
	var seconds num.Dec
	hasDateComponent := false
	hasTimeComponent := false
	maxComponent := uint64(^uint(0) >> 1)
	parseComponent := func(value, label string) (int, error) {
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			if errors.Is(err, strconv.ErrRange) {
				return 0, fmt.Errorf("%s value too large", label)
			}
			return 0, fmt.Errorf("invalid %s value: %w", label, err)
		}
		if u > maxComponent {
			return 0, fmt.Errorf("%s value too large", label)
		}
		return int(u), nil
	}

	if datePart != "" {
		matches := datePattern.FindAllStringSubmatch(datePart, -1)
		for _, match := range matches {
			if match[1] != "" {
				val, err := parseComponent(match[1], "year")
				if err != nil {
					return Duration{}, err
				}
				years = val
				hasDateComponent = true
			}
			if match[2] != "" {
				val, err := parseComponent(match[2], "month")
				if err != nil {
					return Duration{}, err
				}
				months = val
				hasDateComponent = true
			}
			if match[3] != "" {
				val, err := parseComponent(match[3], "day")
				if err != nil {
					return Duration{}, err
				}
				days = val
				hasDateComponent = true
			}
		}
	}

	if timePart != "" {
		matches := timePattern.FindAllStringSubmatch(timePart, -1)
		for _, match := range matches {
			if match[1] != "" {
				val, err := parseComponent(match[1], "hour")
				if err != nil {
					return Duration{}, err
				}
				hours = val
				hasTimeComponent = true
			}
			if match[2] != "" {
				val, err := parseComponent(match[2], "minute")
				if err != nil {
					return Duration{}, err
				}
				minutes = val
				hasTimeComponent = true
			}
			if match[3] != "" {
				dec, perr := num.ParseDec([]byte(match[3]))
				if perr != nil {
					return Duration{}, fmt.Errorf("invalid second value: %w", perr)
				}
				if dec.Sign < 0 {
					return Duration{}, fmt.Errorf("second value cannot be negative")
				}
				seconds = dec
				hasTimeComponent = true
			}
		}
	}

	hasAnyComponent := hasDateComponent || hasTimeComponent
	if !hasAnyComponent {
		return Duration{}, fmt.Errorf("duration must have at least one component")
	}
	if sawTimeDesignator && !hasTimeComponent {
		return Duration{}, fmt.Errorf("time designator present but no time components specified")
	}

	if years == 0 && months == 0 && days == 0 && hours == 0 && minutes == 0 && seconds.Sign == 0 {
		negative = false
	}
	return Duration{
		Negative: negative,
		Years:    years,
		Months:   months,
		Days:     days,
		Hours:    hours,
		Minutes:  minutes,
		Seconds:  seconds,
	}, nil
}
