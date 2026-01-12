package types

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"unsafe"
)

var (
	boolTrueBytes    = []byte("true")
	boolFalseBytes   = []byte("false")
	boolOneBytes     = []byte("1")
	boolZeroBytes    = []byte("0")
	floatInfBytes    = []byte("INF")
	floatNegInfBytes = []byte("-INF")
	floatNaNBytes    = []byte("NaN")
)

func validateBooleanBytes(value []byte) error {
	switch {
	case bytes.Equal(value, boolTrueBytes),
		bytes.Equal(value, boolFalseBytes),
		bytes.Equal(value, boolOneBytes),
		bytes.Equal(value, boolZeroBytes):
		return nil
	}
	return fmt.Errorf("invalid boolean: %s", value)
}

func validateDecimalBytes(value []byte) error {
	if !decimalPattern.Match(value) {
		return fmt.Errorf("invalid decimal: %s", value)
	}
	return nil
}

func validateFloatBytes(value []byte) error {
	if isSpecialFloatBytes(value) {
		return nil
	}
	if !isFloatLexicalBytes(value) {
		return fmt.Errorf("invalid float: %s", value)
	}
	return nil
}

func validateDoubleBytes(value []byte) error {
	if isSpecialFloatBytes(value) {
		return nil
	}
	if !isFloatLexicalBytes(value) {
		return fmt.Errorf("invalid double: %s", value)
	}
	return nil
}

func validateIntegerBytes(value []byte) error {
	if !integerPattern.Match(value) {
		return fmt.Errorf("invalid integer: %s", value)
	}
	return nil
}

func validateLongBytes(value []byte) error {
	if err := validateIntegerBytes(value); err != nil {
		return err
	}
	if _, err := strconv.ParseInt(bytesToStringView(value), 10, 64); err != nil {
		return fmt.Errorf("invalid long: %s", value)
	}
	return nil
}

func validateIntBytes(value []byte) error {
	if err := validateIntegerBytes(value); err != nil {
		return err
	}
	n, err := strconv.ParseInt(bytesToStringView(value), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid int: %s", value)
	}
	if n < math.MinInt32 || n > math.MaxInt32 {
		return fmt.Errorf("int out of range: %s", value)
	}
	return nil
}

func validateShortBytes(value []byte) error {
	if err := validateIntegerBytes(value); err != nil {
		return err
	}
	n, err := strconv.ParseInt(bytesToStringView(value), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid short: %s", value)
	}
	if n < math.MinInt16 || n > math.MaxInt16 {
		return fmt.Errorf("short out of range: %s", value)
	}
	return nil
}

func validateByteBytes(value []byte) error {
	if err := validateIntegerBytes(value); err != nil {
		return err
	}
	n, err := strconv.ParseInt(bytesToStringView(value), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid byte: %s", value)
	}
	if n < math.MinInt8 || n > math.MaxInt8 {
		return fmt.Errorf("byte out of range: %s", value)
	}
	return nil
}

func validateNonNegativeIntegerBytes(value []byte) error {
	if err := validateIntegerBytes(value); err != nil {
		return err
	}
	if len(value) > 0 && value[0] == '-' && !(len(value) == 2 && value[1] == '0') {
		return fmt.Errorf("nonNegativeInteger must be >= 0: %s", value)
	}
	return nil
}

func validatePositiveIntegerBytes(value []byte) error {
	if err := validateIntegerBytes(value); err != nil {
		return err
	}
	n, ok := new(big.Int).SetString(bytesToStringView(value), 10)
	if !ok || n.Sign() <= 0 {
		return fmt.Errorf("positiveInteger must be >= 1: %s", value)
	}
	return nil
}

func validateUnsignedLongBytes(value []byte) error {
	if err := validateNonNegativeIntegerBytes(value); err != nil {
		return err
	}
	if _, err := strconv.ParseUint(bytesToStringView(value), 10, 64); err != nil {
		return fmt.Errorf("invalid unsignedLong: %s", value)
	}
	return nil
}

func validateUnsignedIntBytes(value []byte) error {
	if err := validateNonNegativeIntegerBytes(value); err != nil {
		return err
	}
	n, err := strconv.ParseUint(bytesToStringView(value), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid unsignedInt: %s", value)
	}
	if n > math.MaxUint32 {
		return fmt.Errorf("unsignedInt out of range: %s", value)
	}
	return nil
}

func validateUnsignedShortBytes(value []byte) error {
	if err := validateNonNegativeIntegerBytes(value); err != nil {
		return err
	}
	n, err := strconv.ParseUint(bytesToStringView(value), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid unsignedShort: %s", value)
	}
	if n > math.MaxUint16 {
		return fmt.Errorf("unsignedShort out of range: %s", value)
	}
	return nil
}

func validateUnsignedByteBytes(value []byte) error {
	if err := validateNonNegativeIntegerBytes(value); err != nil {
		return err
	}
	n, err := strconv.ParseUint(bytesToStringView(value), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid unsignedByte: %s", value)
	}
	if n > math.MaxUint8 {
		return fmt.Errorf("unsignedByte out of range: %s", value)
	}
	return nil
}

func validateNonPositiveIntegerBytes(value []byte) error {
	if err := validateIntegerBytes(value); err != nil {
		return err
	}
	n, ok := new(big.Int).SetString(bytesToStringView(value), 10)
	if !ok || n.Sign() > 0 {
		return fmt.Errorf("nonPositiveInteger must be <= 0: %s", value)
	}
	return nil
}

func validateNegativeIntegerBytes(value []byte) error {
	if err := validateIntegerBytes(value); err != nil {
		return err
	}
	n, ok := new(big.Int).SetString(bytesToStringView(value), 10)
	if !ok || n.Sign() >= 0 {
		return fmt.Errorf("negativeInteger must be < 0: %s", value)
	}
	return nil
}

func isSpecialFloatBytes(value []byte) bool {
	return bytes.Equal(value, floatInfBytes) ||
		bytes.Equal(value, floatNegInfBytes) ||
		bytes.Equal(value, floatNaNBytes)
}

func isFloatLexicalBytes(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	i := 0
	if value[i] == '+' || value[i] == '-' {
		i++
		if i == len(value) {
			return false
		}
	}
	startDigits := 0
	for i < len(value) && isDigit(value[i]) {
		i++
		startDigits++
	}
	if i < len(value) && value[i] == '.' {
		i++
		fracDigits := 0
		for i < len(value) && isDigit(value[i]) {
			i++
			fracDigits++
		}
		if startDigits == 0 && fracDigits == 0 {
			return false
		}
	} else if startDigits == 0 {
		return false
	}
	if i < len(value) && (value[i] == 'e' || value[i] == 'E') {
		i++
		if i == len(value) {
			return false
		}
		if value[i] == '+' || value[i] == '-' {
			i++
			if i == len(value) {
				return false
			}
		}
		expDigits := 0
		for i < len(value) && isDigit(value[i]) {
			i++
			expDigits++
		}
		if expDigits == 0 {
			return false
		}
	}
	return i == len(value)
}

func isDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func bytesToStringView(value []byte) string {
	// create a read-only string view for parsing; do not store the result.
	return unsafe.String(unsafe.SliceData(value), len(value))
}
