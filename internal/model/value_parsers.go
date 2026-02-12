package model

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
	// ParseDecimal parses xs:decimal lexical values.
	ParseDecimal = parseFromBytes(value.ParseDecimal)
	// ParseInteger parses xs:integer lexical values.
	ParseInteger = parseFromBytes(value.ParseInteger)
	// ParseBoolean parses xs:boolean lexical values.
	ParseBoolean = parseFromBytes(value.ParseBoolean)
	// ParseFloat parses xs:float lexical values.
	ParseFloat = parseFromBytes(value.ParseFloat)
	// ParseDouble parses xs:double lexical values.
	ParseDouble = parseFromBytes(value.ParseDouble)
	// ParseDateTime parses xs:dateTime lexical values.
	ParseDateTime = parseFromBytes(value.ParseDateTime)
	// ParseLong parses xs:long lexical values.
	ParseLong = parseFromBytes(value.ParseLong)
	// ParseInt parses xs:int lexical values.
	ParseInt = parseFromBytes(value.ParseInt)
	// ParseShort parses xs:short lexical values.
	ParseShort = parseFromBytes(value.ParseShort)
	// ParseByte parses xs:byte lexical values.
	ParseByte = parseFromBytes(value.ParseByte)
	// ParseUnsignedLong parses xs:unsignedLong lexical values.
	ParseUnsignedLong = parseFromBytes(value.ParseUnsignedLong)
	// ParseUnsignedInt parses xs:unsignedInt lexical values.
	ParseUnsignedInt = parseFromBytes(value.ParseUnsignedInt)
	// ParseUnsignedShort parses xs:unsignedShort lexical values.
	ParseUnsignedShort = parseFromBytes(value.ParseUnsignedShort)
	// ParseUnsignedByte parses xs:unsignedByte lexical values.
	ParseUnsignedByte = parseFromBytes(value.ParseUnsignedByte)
	// ParseHexBinary parses xs:hexBinary lexical values.
	ParseHexBinary = parseFromBytes(value.ParseHexBinary)
	// ParseBase64Binary parses xs:base64Binary lexical values.
	ParseBase64Binary = parseFromBytes(value.ParseBase64Binary)
	// ParseAnyURI parses xs:anyURI lexical values.
	ParseAnyURI = parseFromBytes(value.ParseAnyURI)
	// ParseString returns the lexical string unchanged.
	ParseString = func(lexical string) (string, error) {
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
