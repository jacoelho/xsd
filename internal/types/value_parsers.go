package types

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/value"
)

// ParseDecimal parses a decimal string into num.Dec.
// Handles leading/trailing whitespace and validates decimal format
func ParseDecimal(lexical string) (num.Dec, error) {
	return value.ParseDecimal([]byte(lexical))
}

// ParseInteger parses an integer string into num.Int.
// Handles leading/trailing whitespace and validates integer format
func ParseInteger(lexical string) (num.Int, error) {
	return value.ParseInteger([]byte(lexical))
}

// ParseBoolean parses a boolean string into bool
// Accepts "true", "false", "1", "0" (XSD boolean lexical representation)
func ParseBoolean(lexical string) (bool, error) {
	return value.ParseBoolean([]byte(lexical))
}

// ParseFloat parses a float string into float32 with special value handling
// Handles INF, -INF, and NaN special values
func ParseFloat(lexical string) (float32, error) {
	return value.ParseFloat([]byte(lexical))
}

// ParseDouble parses a double string into float64 with special value handling
// Handles INF, -INF, and NaN special values
func ParseDouble(lexical string) (float64, error) {
	return value.ParseDouble([]byte(lexical))
}

// ParseDateTime parses a dateTime string into time.Time
// Supports various ISO 8601 formats with and without timezone
func ParseDateTime(lexical string) (time.Time, error) {
	return value.ParseDateTime([]byte(lexical))
}

// ParseLong parses a long string into int64
func ParseLong(lexical string) (int64, error) {
	return value.ParseLong([]byte(lexical))
}

// ParseInt parses an int string into int32
func ParseInt(lexical string) (int32, error) {
	return value.ParseInt([]byte(lexical))
}

// ParseShort parses a short string into int16
func ParseShort(lexical string) (int16, error) {
	return value.ParseShort([]byte(lexical))
}

// ParseByte parses a byte string into int8
func ParseByte(lexical string) (int8, error) {
	return value.ParseByte([]byte(lexical))
}

// ParseUnsignedLong parses an unsignedLong string into uint64
func ParseUnsignedLong(lexical string) (uint64, error) {
	return value.ParseUnsignedLong([]byte(lexical))
}

// ParseUnsignedInt parses an unsignedInt string into uint32
func ParseUnsignedInt(lexical string) (uint32, error) {
	return value.ParseUnsignedInt([]byte(lexical))
}

// ParseUnsignedShort parses an unsignedShort string into uint16
func ParseUnsignedShort(lexical string) (uint16, error) {
	return value.ParseUnsignedShort([]byte(lexical))
}

// ParseUnsignedByte parses an unsignedByte string into uint8
func ParseUnsignedByte(lexical string) (uint8, error) {
	return value.ParseUnsignedByte([]byte(lexical))
}

// ParseString parses a string (no-op, returns as-is)
func ParseString(lexical string) (string, error) {
	return lexical, nil
}

// ParseHexBinary parses a hexBinary string into []byte
func ParseHexBinary(lexical string) ([]byte, error) {
	return value.ParseHexBinary([]byte(lexical))
}

// ParseBase64Binary parses a base64Binary string into []byte
func ParseBase64Binary(lexical string) ([]byte, error) {
	return value.ParseBase64Binary([]byte(lexical))
}

// ParseAnyURI parses an anyURI string (no validation beyond trimming)
func ParseAnyURI(lexical string) (string, error) {
	return value.ParseAnyURI([]byte(lexical))
}

// ParseQNameValue parses a QName value (lexical string) into a QName with namespace resolution.
func ParseQNameValue(lexical string, nsContext map[string]string) (QName, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if trimmed == "" {
		return QName{}, fmt.Errorf("invalid QName: empty string")
	}

	prefix, local, hasPrefix, err := ParseQName(trimmed)
	if err != nil {
		return QName{}, err
	}

	var ns NamespaceURI
	if hasPrefix {
		var ok bool
		ns, ok = ResolveNamespace(prefix, nsContext)
		if !ok {
			return QName{}, fmt.Errorf("prefix %s not found in namespace context", prefix)
		}
	} else {
		if defaultNS, ok := ResolveNamespace("", nsContext); ok {
			ns = defaultNS
		}
	}

	return QName{Namespace: ns, Local: local}, nil
}

// ParseNOTATION parses a NOTATION value (lexical string) into a QName with namespace resolution.
func ParseNOTATION(lexical string, nsContext map[string]string) (QName, error) {
	return ParseQNameValue(lexical, nsContext)
}

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

// isBuiltinListType checks if a type name is a built-in list type.
func isBuiltinListType(name string) bool {
	return name == string(TypeNameNMTOKENS) ||
		name == string(TypeNameIDREFS) ||
		name == string(TypeNameENTITIES)
}

func builtinListItemTypeName(name string) (TypeName, bool) {
	switch name {
	case string(TypeNameNMTOKENS):
		return TypeNameNMTOKEN, true
	case string(TypeNameIDREFS):
		return TypeNameIDREF, true
	case string(TypeNameENTITIES):
		return TypeNameENTITY, true
	default:
		return "", false
	}
}
