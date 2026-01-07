package types

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

// measureLengthForPrimitive measures length for primitive types.
func measureLengthForPrimitive(value string, primitiveName TypeName) int {
	switch primitiveName {
	case TypeNameHexBinary:
		// hexBinary: each pair of hex characters = 1 octet
		if len(value) == 0 {
			return 0
		}
		if len(value)%2 != 0 {
			// invalid hexBinary - return character count as fallback
			return utf8.RuneCountInString(value)
		}
		return len(value) / 2

	case TypeNameBase64Binary:
		// base64Binary: length is the number of octets it contains
		if len(value) == 0 {
			return 0
		}
		cleaned := strings.ReplaceAll(value, " ", "")
		cleaned = strings.ReplaceAll(cleaned, "\t", "")
		cleaned = strings.ReplaceAll(cleaned, "\n", "")
		cleaned = strings.ReplaceAll(cleaned, "\r", "")

		// decode to get actual byte length
		decoded, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			// try URL encoding variant
			decoded, err = base64.URLEncoding.DecodeString(cleaned)
			if err != nil {
				// invalid base64 - return character count as fallback
				return utf8.RuneCountInString(value)
			}
		}
		return len(decoded)
	}

	// for all other types, length is in characters (Unicode code points)
	return utf8.RuneCountInString(value)
}

// isBuiltinListType checks if a type name is a built-in list type.
func isBuiltinListType(name string) bool {
	return name == string(TypeNameNMTOKENS) ||
		name == string(TypeNameIDREFS) ||
		name == string(TypeNameENTITIES)
}