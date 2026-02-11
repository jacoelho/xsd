package durationconv

import (
	"errors"
	"fmt"
	"time"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
)

var (
	// ErrIndeterminate reports that duration contains years or months.
	ErrIndeterminate = errors.New("duration conversion indeterminate")
	// ErrOverflow reports that duration cannot fit in time.Duration.
	ErrOverflow = errors.New("duration conversion overflow")
	// ErrComponentRange reports that one duration component is out of range.
	ErrComponentRange = errors.New("duration component out of range")
)

// ParseToStdDuration parses an XSD duration and converts it to time.Duration.
func ParseToStdDuration(text string) (time.Duration, error) {
	parsed, err := durationlex.Parse(text)
	if err != nil {
		return 0, err
	}
	return ToStdDuration(parsed)
}

// ToStdDuration converts a parsed XSD duration to time.Duration.
func ToStdDuration(parsed durationlex.Duration) (time.Duration, error) {
	if parsed.Years != 0 || parsed.Months != 0 {
		return 0, ErrIndeterminate
	}

	const maxDuration = time.Duration(^uint64(0) >> 1)

	componentDuration := func(value int, unit time.Duration) (time.Duration, error) {
		if value == 0 {
			return 0, nil
		}
		if value < 0 {
			return 0, ErrComponentRange
		}
		limit := int64(maxDuration / unit)
		if int64(value) > limit {
			return 0, ErrOverflow
		}
		return time.Duration(value) * unit, nil
	}

	addDuration := func(total, delta time.Duration) (time.Duration, error) {
		if delta < 0 {
			return 0, ErrComponentRange
		}
		if total > maxDuration-delta {
			return 0, ErrOverflow
		}
		return total + delta, nil
	}

	dur := time.Duration(0)
	var err error

	delta, err := componentDuration(parsed.Days, 24*time.Hour)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, delta)
	if err != nil {
		return 0, err
	}

	delta, err = componentDuration(parsed.Hours, time.Hour)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, delta)
	if err != nil {
		return 0, err
	}

	delta, err = componentDuration(parsed.Minutes, time.Minute)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, delta)
	if err != nil {
		return 0, err
	}

	secondsDuration, err := secondsToDuration(parsed.Seconds)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, secondsDuration)
	if err != nil {
		return 0, err
	}

	if parsed.Negative {
		dur = -dur
	}
	return dur, nil
}

func secondsToDuration(sec num.Dec) (time.Duration, error) {
	if sec.Sign < 0 {
		return 0, fmt.Errorf("second value cannot be negative")
	}
	scaled, err := num.DecToScaledIntExact(sec, 9)
	if err != nil {
		return 0, err
	}
	const maxDuration = time.Duration(^uint64(0) >> 1)
	maxSeconds := num.FromInt64(int64(maxDuration))
	if scaled.Compare(maxSeconds) > 0 {
		return 0, fmt.Errorf("second value too large")
	}
	val, ok := int64FromDigits(scaled.Digits)
	if !ok {
		return 0, fmt.Errorf("second value too large")
	}
	if scaled.Sign < 0 {
		val = -val
	}
	return time.Duration(val), nil
}

func int64FromDigits(digits []byte) (int64, bool) {
	if len(digits) == 0 {
		return 0, true
	}
	var n int64
	for _, d := range digits {
		if n > (int64(^uint64(0)>>1)-int64(d-'0'))/10 {
			return 0, false
		}
		n = n*10 + int64(d-'0')
	}
	return n, true
}
