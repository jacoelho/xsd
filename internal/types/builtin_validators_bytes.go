package types

import "unsafe"

func validateBooleanBytes(value []byte) error {
	return validateBytesWithString(value, validateBoolean)
}

func validateDecimalBytes(value []byte) error {
	return validateBytesWithString(value, validateDecimal)
}

func validateFloatBytes(value []byte) error {
	return validateBytesWithString(value, validateFloat)
}

func validateDoubleBytes(value []byte) error {
	return validateBytesWithString(value, validateDouble)
}

func validateIntegerBytes(value []byte) error {
	return validateBytesWithString(value, validateInteger)
}

func validateLongBytes(value []byte) error {
	return validateBytesWithString(value, validateLong)
}

func validateIntBytes(value []byte) error {
	return validateBytesWithString(value, validateInt)
}

func validateShortBytes(value []byte) error {
	return validateBytesWithString(value, validateShort)
}

func validateByteBytes(value []byte) error {
	return validateBytesWithString(value, validateByte)
}

func validateNonNegativeIntegerBytes(value []byte) error {
	return validateBytesWithString(value, validateNonNegativeInteger)
}

func validatePositiveIntegerBytes(value []byte) error {
	return validateBytesWithString(value, validatePositiveInteger)
}

func validateUnsignedLongBytes(value []byte) error {
	return validateBytesWithString(value, validateUnsignedLong)
}

func validateUnsignedIntBytes(value []byte) error {
	return validateBytesWithString(value, validateUnsignedInt)
}

func validateUnsignedShortBytes(value []byte) error {
	return validateBytesWithString(value, validateUnsignedShort)
}

func validateUnsignedByteBytes(value []byte) error {
	return validateBytesWithString(value, validateUnsignedByte)
}

func validateNonPositiveIntegerBytes(value []byte) error {
	return validateBytesWithString(value, validateNonPositiveInteger)
}

func validateNegativeIntegerBytes(value []byte) error {
	return validateBytesWithString(value, validateNegativeInteger)
}

func bytesToStringView(value []byte) string {
	// create a read-only string view for immediate parsing only.
	// callers must not retain the string or mutate the backing slice.
	return unsafe.String(unsafe.SliceData(value), len(value))
}

func validateBytesWithString(value []byte, validator func(string) error) error {
	return validator(bytesToStringView(value))
}
