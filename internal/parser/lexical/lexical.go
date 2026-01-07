package lexical

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// ParseDecimal parses a decimal string into *big.Rat
// Handles leading/trailing whitespace and validates decimal format
func ParseDecimal(lexical string) (*big.Rat, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return nil, fmt.Errorf("invalid decimal: empty string")
	}

	rat := new(big.Rat)
	if _, ok := rat.SetString(lexical); !ok {
		return nil, fmt.Errorf("invalid decimal: %s", lexical)
	}
	return rat, nil
}

// ParseInteger parses an integer string into *big.Int
// Handles leading/trailing whitespace and validates integer format
func ParseInteger(lexical string) (*big.Int, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return nil, fmt.Errorf("invalid integer: empty string")
	}

	intVal := new(big.Int)
	if _, ok := intVal.SetString(lexical, 10); !ok {
		return nil, fmt.Errorf("invalid integer: %s", lexical)
	}
	return intVal, nil
}

// ParseBoolean parses a boolean string into bool
// Accepts "true", "false", "1", "0" (XSD boolean lexical representation)
func ParseBoolean(lexical string) (bool, error) {
	lexical = strings.TrimSpace(lexical)
	switch lexical {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean: %s (must be 'true', 'false', '1', or '0')", lexical)
	}
}

// ParseFloat parses a float string into float32 with special value handling
// Handles INF, -INF, and NaN special values
func ParseFloat(lexical string) (float32, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid float: empty string")
	}

	switch lexical {
	case "INF", "+INF":
		return float32(math.Inf(1)), nil
	case "-INF":
		return float32(math.Inf(-1)), nil
	case "NaN":
		return float32(math.NaN()), nil
	default:
		f, err := strconv.ParseFloat(lexical, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid float: %s", lexical)
		}
		return float32(f), nil
	}
}

// ParseDouble parses a double string into float64 with special value handling
// Handles INF, -INF, and NaN special values
func ParseDouble(lexical string) (float64, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid double: empty string")
	}

	switch lexical {
	case "INF", "+INF":
		return math.Inf(1), nil
	case "-INF":
		return math.Inf(-1), nil
	case "NaN":
		return math.NaN(), nil
	default:
		f, err := strconv.ParseFloat(lexical, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid double: %s", lexical)
		}
		return f, nil
	}
}

// ParseDateTime parses a dateTime string into time.Time
// Supports various ISO 8601 formats with and without timezone
func ParseDateTime(lexical string) (time.Time, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return time.Time{}, fmt.Errorf("invalid dateTime: empty string")
	}

	// try various ISO 8601 formats
	formats := []string{
		time.RFC3339Nano,                // 2006-01-02T15:04:05.999999999Z07:00
		time.RFC3339,                    // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05.999999999", // with nanoseconds, no timezone
		"2006-01-02T15:04:05.999999",    // with microseconds, no timezone
		"2006-01-02T15:04:05.999",       // with milliseconds, no timezone
		"2006-01-02T15:04:05",           // no fractional seconds, no timezone
		"2006-01-02T15:04:05Z",          // no fractional seconds, UTC
		"2006-01-02T15:04:05-07:00",     // no fractional seconds, with timezone
		"2006-01-02T15:04:05+07:00",     // no fractional seconds, with timezone
		"2006-01-02T15:04:05.999Z",      // with milliseconds, UTC
		"2006-01-02T15:04:05.999-07:00", // with milliseconds, with timezone
		"2006-01-02T15:04:05.999+07:00", // with milliseconds, with timezone
	}

	for _, format := range formats {
		if t, err := time.Parse(format, lexical); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid dateTime: %s", lexical)
}

// ParseLong parses a long string into int64
func ParseLong(lexical string) (int64, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid long: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid long: %s", lexical)
	}
	return val, nil
}

// ParseInt parses an int string into int32
func ParseInt(lexical string) (int32, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid int: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid int: %s", lexical)
	}
	return int32(val), nil
}

// ParseShort parses a short string into int16
func ParseShort(lexical string) (int16, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid short: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid short: %s", lexical)
	}
	return int16(val), nil
}

// ParseByte parses a byte string into int8
func ParseByte(lexical string) (int8, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid byte: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid byte: %s", lexical)
	}
	return int8(val), nil
}

// ParseUnsignedLong parses an unsignedLong string into uint64
func ParseUnsignedLong(lexical string) (uint64, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedLong: empty string")
	}

	val, err := strconv.ParseUint(lexical, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedLong: %s", lexical)
	}
	return val, nil
}

// ParseUnsignedInt parses an unsignedInt string into uint32
func ParseUnsignedInt(lexical string) (uint32, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedInt: empty string")
	}

	val, err := strconv.ParseUint(lexical, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedInt: %s", lexical)
	}
	return uint32(val), nil
}

// ParseUnsignedShort parses an unsignedShort string into uint16
func ParseUnsignedShort(lexical string) (uint16, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedShort: empty string")
	}

	val, err := strconv.ParseUint(lexical, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedShort: %s", lexical)
	}
	return uint16(val), nil
}

// ParseUnsignedByte parses an unsignedByte string into uint8
func ParseUnsignedByte(lexical string) (uint8, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedByte: empty string")
	}

	val, err := strconv.ParseUint(lexical, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedByte: %s", lexical)
	}
	return uint8(val), nil
}

// ParseString parses a string (no-op, returns as-is)
func ParseString(lexical string) (string, error) {
	return lexical, nil
}