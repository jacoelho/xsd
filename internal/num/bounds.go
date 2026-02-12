package num

var (
	// IntZero is an exported variable.
	IntZero = Int{Sign: 0, Digits: zeroDigits}

	// MinInt8 is an exported variable.
	MinInt8 = Int{Sign: -1, Digits: []byte("128")}
	// MaxInt8 is an exported variable.
	MaxInt8 = Int{Sign: 1, Digits: []byte("127")}
	// MinInt16 is an exported variable.
	MinInt16 = Int{Sign: -1, Digits: []byte("32768")}
	// MaxInt16 is an exported variable.
	MaxInt16 = Int{Sign: 1, Digits: []byte("32767")}
	// MinInt32 is an exported variable.
	MinInt32 = Int{Sign: -1, Digits: []byte("2147483648")}
	// MaxInt32 is an exported variable.
	MaxInt32 = Int{Sign: 1, Digits: []byte("2147483647")}
	// MinInt64 is an exported variable.
	MinInt64 = Int{Sign: -1, Digits: []byte("9223372036854775808")}
	// MaxInt64 is an exported variable.
	MaxInt64 = Int{Sign: 1, Digits: []byte("9223372036854775807")}

	// MaxUint8 is an exported variable.
	MaxUint8 = Int{Sign: 1, Digits: []byte("255")}
	// MaxUint16 is an exported variable.
	MaxUint16 = Int{Sign: 1, Digits: []byte("65535")}
	// MaxUint32 is an exported variable.
	MaxUint32 = Int{Sign: 1, Digits: []byte("4294967295")}
	// MaxUint64 is an exported variable.
	MaxUint64 = Int{Sign: 1, Digits: []byte("18446744073709551615")}
)
