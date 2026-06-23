package runtime

import "unicode/utf8"

// TextValue is the value-space projection for text primitive values.
type TextValue struct {
	Canonical string
	Length    uint32
}

// ParseTextValue parses normalized as an XML Schema string or anyURI primitive
// value.
func ParseTextValue(kind PrimitiveKind, normalized string, needs PrimitiveValueNeed) (TextValue, error) {
	switch kind {
	case PrimitiveString:
	case PrimitiveAnyURI:
		if err := ValidateAnyURILexical(normalized); err != nil {
			return TextValue{}, err
		}
	default:
		return TextValue{}, ErrSimpleValueMetadata
	}
	value := TextValue{Canonical: normalized}
	if needs.Has(PrimitiveNeedLength) {
		length, err := PrimitiveLength(kind, normalized)
		if err != nil {
			return TextValue{}, err
		}
		value.Length = length
	}
	return value, nil
}

// PrimitiveLength returns the value length for runtime-owned length-capable
// primitive values.
func PrimitiveLength(kind PrimitiveKind, normalized string) (uint32, error) {
	switch kind {
	case PrimitiveString:
		return stringLength(normalized, "string length exceeds uint32 limit")
	case PrimitiveAnyURI:
		return anyURILength(normalized)
	case PrimitiveHexBinary, PrimitiveBase64Binary:
		return BinaryLength(kind, normalized)
	default:
		return 0, ErrSimpleValueMetadata
	}
}

func stringLength(normalized, msg string) (uint32, error) {
	return checkedUint32(utf8.RuneCountInString(normalized), msg)
}
