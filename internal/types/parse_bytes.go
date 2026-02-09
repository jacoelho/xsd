package types

import "github.com/jacoelho/xsd/internal/value"

// ParseHexBinaryBytes parses a hexBinary lexical value from bytes.
func ParseHexBinaryBytes(lexical []byte) ([]byte, error) {
	return value.ParseHexBinary(lexical)
}

// ParseBase64BinaryBytes parses a base64Binary lexical value from bytes.
func ParseBase64BinaryBytes(lexical []byte) ([]byte, error) {
	return value.ParseBase64Binary(lexical)
}

// ParseXSDDurationBytes parses an XSD duration lexical value from bytes.
func ParseXSDDurationBytes(lexical []byte) (XSDDuration, error) {
	return ParseXSDDuration(bytesToStringView(lexical))
}
