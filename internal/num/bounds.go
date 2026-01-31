package num

var (
	IntZero = Int{Sign: 0, Digits: zeroDigits}

	MinInt8  = Int{Sign: -1, Digits: []byte("128")}
	MaxInt8  = Int{Sign: 1, Digits: []byte("127")}
	MinInt16 = Int{Sign: -1, Digits: []byte("32768")}
	MaxInt16 = Int{Sign: 1, Digits: []byte("32767")}
	MinInt32 = Int{Sign: -1, Digits: []byte("2147483648")}
	MaxInt32 = Int{Sign: 1, Digits: []byte("2147483647")}
	MinInt64 = Int{Sign: -1, Digits: []byte("9223372036854775808")}
	MaxInt64 = Int{Sign: 1, Digits: []byte("9223372036854775807")}

	MaxUint8  = Int{Sign: 1, Digits: []byte("255")}
	MaxUint16 = Int{Sign: 1, Digits: []byte("65535")}
	MaxUint32 = Int{Sign: 1, Digits: []byte("4294967295")}
	MaxUint64 = Int{Sign: 1, Digits: []byte("18446744073709551615")}
)
