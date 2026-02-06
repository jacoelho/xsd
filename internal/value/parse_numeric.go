package value

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/runtime"
)

// ParseLong parses an xs:long lexical value into int64.
func ParseLong(lexical []byte) (int64, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid long: empty string")
	}
	val, err := strconv.ParseInt(string(trimmed), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid long: %s", string(trimmed))
	}
	return val, nil
}

// ParseInt parses an xs:int lexical value into int32.
func ParseInt(lexical []byte) (int32, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid int: empty string")
	}
	val, err := strconv.ParseInt(string(trimmed), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid int: %s", string(trimmed))
	}
	return int32(val), nil
}

// ParseShort parses an xs:short lexical value into int16.
func ParseShort(lexical []byte) (int16, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid short: empty string")
	}
	val, err := strconv.ParseInt(string(trimmed), 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid short: %s", string(trimmed))
	}
	return int16(val), nil
}

// ParseByte parses an xs:byte lexical value into int8.
func ParseByte(lexical []byte) (int8, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid byte: empty string")
	}
	val, err := strconv.ParseInt(string(trimmed), 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid byte: %s", string(trimmed))
	}
	return int8(val), nil
}

// ParseUnsignedLong parses an xs:unsignedLong lexical value into uint64.
func ParseUnsignedLong(lexical []byte) (uint64, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid unsignedLong: empty string")
	}
	normalized, err := normalizeUnsignedLexical(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedLong: %s", string(trimmed))
	}
	val, err := strconv.ParseUint(normalized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedLong: %s", string(trimmed))
	}
	return val, nil
}

// ParseUnsignedInt parses an xs:unsignedInt lexical value into uint32.
func ParseUnsignedInt(lexical []byte) (uint32, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid unsignedInt: empty string")
	}
	normalized, err := normalizeUnsignedLexical(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedInt: %s", string(trimmed))
	}
	val, err := strconv.ParseUint(normalized, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedInt: %s", string(trimmed))
	}
	return uint32(val), nil
}

// ParseUnsignedShort parses an xs:unsignedShort lexical value into uint16.
func ParseUnsignedShort(lexical []byte) (uint16, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid unsignedShort: empty string")
	}
	normalized, err := normalizeUnsignedLexical(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedShort: %s", string(trimmed))
	}
	val, err := strconv.ParseUint(normalized, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedShort: %s", string(trimmed))
	}
	return uint16(val), nil
}

// ParseUnsignedByte parses an xs:unsignedByte lexical value into uint8.
func ParseUnsignedByte(lexical []byte) (uint8, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid unsignedByte: empty string")
	}
	normalized, err := normalizeUnsignedLexical(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedByte: %s", string(trimmed))
	}
	val, err := strconv.ParseUint(normalized, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid unsignedByte: %s", string(trimmed))
	}
	return uint8(val), nil
}

// ParseHexBinary parses an xs:hexBinary lexical value into bytes.
func ParseHexBinary(lexical []byte) ([]byte, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if len(trimmed)%2 != 0 {
		return nil, fmt.Errorf("invalid hexBinary: odd length")
	}
	data := make([]byte, len(trimmed)/2)
	for i := 0; i < len(trimmed); i += 2 {
		b, err := strconv.ParseUint(string(trimmed[i:i+2]), 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid hexBinary: %s", string(trimmed))
		}
		data[i/2] = byte(b)
	}
	return data, nil
}

// ParseBase64Binary parses an xs:base64Binary lexical value into bytes.
func ParseBase64Binary(lexical []byte) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		default:
			return r
		}
	}, string(lexical))

	if cleaned == "" {
		return nil, nil
	}

	decoded, err := base64.StdEncoding.Strict().DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("invalid base64Binary: %s", string(lexical))
	}
	return decoded, nil
}

// ParseAnyURI parses an xs:anyURI lexical value and validates its syntax.
func ParseAnyURI(lexical []byte) (string, error) {
	normalized := NormalizeWhitespace(runtime.WS_Collapse, lexical, nil)
	if err := ValidateAnyURI(normalized); err != nil {
		return "", fmt.Errorf("invalid anyURI: %s", string(normalized))
	}
	return string(normalized), nil
}

func normalizeUnsignedLexical(trimmed []byte) (string, error) {
	for _, b := range trimmed {
		if b < '0' || b > '9' {
			return "", fmt.Errorf("unsigned integer must contain only digits")
		}
	}
	return string(trimmed), nil
}
