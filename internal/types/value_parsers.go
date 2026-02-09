package types

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/value"
)

func parseFromBytes[T any](parse func([]byte) (T, error)) func(string) (T, error) {
	return func(lexical string) (T, error) {
		return parse([]byte(lexical))
	}
}

var (
	ParseDecimal       = parseFromBytes(value.ParseDecimal)
	ParseInteger       = parseFromBytes(value.ParseInteger)
	ParseBoolean       = parseFromBytes(value.ParseBoolean)
	ParseFloat         = parseFromBytes(value.ParseFloat)
	ParseDouble        = parseFromBytes(value.ParseDouble)
	ParseDateTime      = parseFromBytes(value.ParseDateTime)
	ParseLong          = parseFromBytes(value.ParseLong)
	ParseInt           = parseFromBytes(value.ParseInt)
	ParseShort         = parseFromBytes(value.ParseShort)
	ParseByte          = parseFromBytes(value.ParseByte)
	ParseUnsignedLong  = parseFromBytes(value.ParseUnsignedLong)
	ParseUnsignedInt   = parseFromBytes(value.ParseUnsignedInt)
	ParseUnsignedShort = parseFromBytes(value.ParseUnsignedShort)
	ParseUnsignedByte  = parseFromBytes(value.ParseUnsignedByte)
	ParseHexBinary     = parseFromBytes(value.ParseHexBinary)
	ParseBase64Binary  = parseFromBytes(value.ParseBase64Binary)
	ParseAnyURI        = parseFromBytes(value.ParseAnyURI)
	ParseString        = func(lexical string) (string, error) {
		return lexical, nil
	}
)

// measureLengthForPrimitive measures length for primitive types.
func measureLengthForPrimitive(lexical string, primitiveName TypeName) int {
	switch primitiveName {
	case TypeNameHexBinary:
		// hexBinary: each pair of hex characters = 1 octet
		if lexical == "" {
			return 0
		}
		if len(lexical)%2 != 0 {
			// invalid hexBinary - return character count as fallback
			return utf8.RuneCountInString(lexical)
		}
		return len(lexical) / 2

	case TypeNameBase64Binary:
		// base64Binary: length is the number of octets it contains
		if lexical == "" {
			return 0
		}
		cleaned := strings.Map(func(r rune) rune {
			switch r {
			case ' ', '\t', '\n', '\r':
				return -1
			default:
				return r
			}
		}, lexical)

		// decode to get actual byte length
		decoded, err := base64.StdEncoding.Strict().DecodeString(cleaned)
		if err != nil {
			// invalid base64 - return character count as fallback
			return utf8.RuneCountInString(lexical)
		}
		return len(decoded)
	}

	// for all other types, length is in characters (Unicode code points)
	return utf8.RuneCountInString(lexical)
}
