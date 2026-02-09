package types

import "testing"

func TestValidateBytesMatchesString(t *testing.T) {
	tests := []struct {
		validate func(string) error
		bytes    func([]byte) error
		name     string
		values   []string
	}{
		{
			name:     "boolean",
			values:   []string{"true", "false", "1", "0", "yes", "", "True"},
			validate: validateBoolean,
			bytes:    byteValidator(validateBoolean),
		},
		{
			name:     "decimal",
			values:   []string{"0", "-1", "+3.14", "1.0", "1.", ".5", "1e2", "abc"},
			validate: validateDecimal,
			bytes:    byteValidator(validateDecimal),
		},
		{
			name:     "float",
			values:   []string{"0", "123.45", "-123.45", "INF", "-INF", "NaN", "1.23e10", "1.e2", ".5", "+INF", "1e", "abc"},
			validate: validateFloat,
			bytes:    byteValidator(validateFloat),
		},
		{
			name:     "double",
			values:   []string{"0", "123.45", "-123.45", "INF", "-INF", "NaN", "1.23e10", "1.e2", ".5", "+INF", "1e", "abc"},
			validate: validateDouble,
			bytes:    byteValidator(validateDouble),
		},
		{
			name:     "integer",
			values:   []string{"0", "-1", "+1", "1.0", "--1"},
			validate: validateInteger,
			bytes:    byteValidator(validateInteger),
		},
		{
			name:     "long",
			values:   []string{"9223372036854775807", "9223372036854775808"},
			validate: validateLong,
			bytes:    byteValidator(validateLong),
		},
		{
			name:     "int",
			values:   []string{"2147483647", "2147483648"},
			validate: validateInt,
			bytes:    byteValidator(validateInt),
		},
		{
			name:     "short",
			values:   []string{"32767", "32768"},
			validate: validateShort,
			bytes:    byteValidator(validateShort),
		},
		{
			name:     "byte",
			values:   []string{"127", "128"},
			validate: validateByte,
			bytes:    byteValidator(validateByte),
		},
		{
			name:     "nonNegativeInteger",
			values:   []string{"0", "-0", "-00", "-1"},
			validate: validateNonNegativeInteger,
			bytes:    byteValidator(validateNonNegativeInteger),
		},
		{
			name:     "positiveInteger",
			values:   []string{"1", "0", "-1"},
			validate: validatePositiveInteger,
			bytes:    byteValidator(validatePositiveInteger),
		},
		{
			name:     "unsignedLong",
			values:   []string{"0", "-1", "18446744073709551615"},
			validate: validateUnsignedLong,
			bytes:    byteValidator(validateUnsignedLong),
		},
		{
			name:     "unsignedInt",
			values:   []string{"4294967295", "4294967296"},
			validate: validateUnsignedInt,
			bytes:    byteValidator(validateUnsignedInt),
		},
		{
			name:     "unsignedShort",
			values:   []string{"65535", "65536"},
			validate: validateUnsignedShort,
			bytes:    byteValidator(validateUnsignedShort),
		},
		{
			name:     "unsignedByte",
			values:   []string{"255", "256"},
			validate: validateUnsignedByte,
			bytes:    byteValidator(validateUnsignedByte),
		},
		{
			name:     "nonPositiveInteger",
			values:   []string{"0", "-1", "1"},
			validate: validateNonPositiveInteger,
			bytes:    byteValidator(validateNonPositiveInteger),
		},
		{
			name:     "negativeInteger",
			values:   []string{"-1", "0", "1"},
			validate: validateNegativeInteger,
			bytes:    byteValidator(validateNegativeInteger),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, value := range tt.values {
				errString := tt.validate(value)
				errBytes := tt.bytes([]byte(value))
				if (errString == nil) != (errBytes == nil) {
					t.Errorf("%s: string=%v bytes=%v for %q", tt.name, errString, errBytes, value)
				}
			}
		})
	}
}
