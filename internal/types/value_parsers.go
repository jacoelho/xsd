package types

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/value"
)

// ParseDecimal parses a decimal string into num.Dec.
// Handles leading/trailing whitespace and validates decimal format
func ParseDecimal(lexical string) (num.Dec, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return num.Dec{}, fmt.Errorf("invalid decimal: empty string")
	}

	dec, perr := num.ParseDec([]byte(lexical))
	if perr != nil {
		return num.Dec{}, fmt.Errorf("invalid decimal: %s", lexical)
	}
	return dec, nil
}

// ParseInteger parses an integer string into num.Int.
// Handles leading/trailing whitespace and validates integer format
func ParseInteger(lexical string) (num.Int, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return num.Int{}, fmt.Errorf("invalid integer: empty string")
	}

	intVal, perr := num.ParseInt([]byte(lexical))
	if perr != nil {
		return num.Int{}, fmt.Errorf("invalid integer: %s", lexical)
	}
	return intVal, nil
}

// ParseBoolean parses a boolean string into bool
// Accepts "true", "false", "1", "0" (XSD boolean lexical representation)
func ParseBoolean(lexical string) (bool, error) {
	lexical = TrimXMLWhitespace(lexical)
	switch lexical {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean: %s (must be 'true', 'false', '1', or '0')", lexical)
	}
}

// ParseFloat parses a float string into float32 with special value handling
// Handles INF, -INF, and NaN special values
func ParseFloat(lexical string) (float32, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid float: empty string")
	}
	f, _, perr := num.ParseFloat32([]byte(lexical))
	if perr == nil {
		return f, nil
	}
	if perr.Kind == num.ParseEmpty {
		return 0, fmt.Errorf("invalid float: empty string")
	}
	return 0, fmt.Errorf("invalid float: %s", lexical)
}

// ParseDouble parses a double string into float64 with special value handling
// Handles INF, -INF, and NaN special values
func ParseDouble(lexical string) (float64, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid double: empty string")
	}
	f, _, perr := num.ParseFloat64([]byte(lexical))
	if perr == nil {
		return f, nil
	}
	if perr.Kind == num.ParseEmpty {
		return 0, fmt.Errorf("invalid double: empty string")
	}
	return 0, fmt.Errorf("invalid double: %s", lexical)
}

// ParseDateTime parses a dateTime string into time.Time
// Supports various ISO 8601 formats with and without timezone
func ParseDateTime(lexical string) (time.Time, error) {
	return value.ParseDateTime([]byte(lexical))
}

// ParseLong parses a long string into int64
func ParseLong(lexical string) (int64, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid long: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid long: %s", lexical)
	}
	return val, nil
}

// ParseInt parses an int string into int32
func ParseInt(lexical string) (int32, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid int: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid int: %s", lexical)
	}
	return int32(val), nil
}

// ParseShort parses a short string into int16
func ParseShort(lexical string) (int16, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid short: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid short: %s", lexical)
	}
	return int16(val), nil
}

// ParseByte parses a byte string into int8
func ParseByte(lexical string) (int8, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid byte: empty string")
	}

	val, err := strconv.ParseInt(lexical, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid byte: %s", lexical)
	}
	return int8(val), nil
}

// ParseUnsignedLong parses an unsignedLong string into uint64
func ParseUnsignedLong(lexical string) (uint64, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedLong: empty string")
	}

	normalized, err := normalizeUnsignedLexical(lexical)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedLong: %s", lexical)
	}
	val, err := strconv.ParseUint(normalized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedLong: %s", lexical)
	}
	return val, nil
}

// ParseUnsignedInt parses an unsignedInt string into uint32
func ParseUnsignedInt(lexical string) (uint32, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedInt: empty string")
	}

	normalized, err := normalizeUnsignedLexical(lexical)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedInt: %s", lexical)
	}
	val, err := strconv.ParseUint(normalized, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedInt: %s", lexical)
	}
	return uint32(val), nil
}

// ParseUnsignedShort parses an unsignedShort string into uint16
func ParseUnsignedShort(lexical string) (uint16, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedShort: empty string")
	}

	normalized, err := normalizeUnsignedLexical(lexical)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedShort: %s", lexical)
	}
	val, err := strconv.ParseUint(normalized, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedShort: %s", lexical)
	}
	return uint16(val), nil
}

// ParseUnsignedByte parses an unsignedByte string into uint8
func ParseUnsignedByte(lexical string) (uint8, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return 0, fmt.Errorf("invalid unsignedByte: empty string")
	}

	normalized, err := normalizeUnsignedLexical(lexical)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedByte: %s", lexical)
	}
	val, err := strconv.ParseUint(normalized, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedByte: %s", lexical)
	}
	return uint8(val), nil
}

// ParseString parses a string (no-op, returns as-is)
func ParseString(lexical string) (string, error) {
	return lexical, nil
}

// ParseHexBinary parses a hexBinary string into []byte
func ParseHexBinary(lexical string) ([]byte, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return nil, nil
	}
	if len(lexical)%2 != 0 {
		return nil, fmt.Errorf("invalid hexBinary: odd length")
	}
	data := make([]byte, len(lexical)/2)
	for i := 0; i < len(lexical); i += 2 {
		b, err := strconv.ParseUint(lexical[i:i+2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid hexBinary: %s", lexical)
		}
		data[i/2] = byte(b)
	}
	return data, nil
}

// ParseBase64Binary parses a base64Binary string into []byte
func ParseBase64Binary(lexical string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		default:
			return r
		}
	}, lexical)

	if cleaned == "" {
		return nil, nil
	}

	decoded, err := base64.StdEncoding.Strict().DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("invalid base64Binary: %s", lexical)
	}
	return decoded, nil
}

// ParseAnyURI parses an anyURI string (no validation beyond trimming)
func ParseAnyURI(lexical string) (string, error) {
	return TrimXMLWhitespace(lexical), nil
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
