package model

import "unsafe"

func byteValidator(validator TypeValidator) TypeValidatorBytes {
	if validator == nil {
		return nil
	}
	return func(value []byte) error {
		// create a read-only string view for immediate parsing only.
		// callers must not retain the string or mutate the backing slice.
		return validator(unsafe.String(unsafe.SliceData(value), len(value)))
	}
}
