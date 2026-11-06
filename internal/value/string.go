package value

import (
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/uriref"
)

// textValue is the value-space projection for text primitive values.
type textValue struct {
	Canonical string
	Length    uint32
}

// parseTextValue parses normalized as an XML Schema string or anyURI primitive
// value.
func parseTextValue(kind PrimitiveKind, normalized string, needs primitiveValueNeed) (textValue, error) {
	switch kind {
	case PrimitiveString:
	case PrimitiveAnyURI:
		return parseAnyURITextValue(normalized, needs)
	case PrimitiveBoolean, PrimitiveDecimal, PrimitiveFloat, PrimitiveDouble, PrimitiveDuration,
		PrimitiveDateTime, primitiveTime, primitiveDate,
		primitiveGYearMonth, primitiveGYear, primitiveGMonthDay, primitiveGDay, primitiveGMonth,
		PrimitiveHexBinary, PrimitiveBase64Binary, PrimitiveQName, PrimitiveNotation:
		return textValue{}, ErrMetadata
	default:
		err := ErrMetadata
		return textValue{}, err
	}
	value := textValue{Canonical: normalized}
	if needs.Has(primitiveNeedLength) {
		length, err := checkedUint32(utf8.RuneCountInString(normalized), msgStringLengthLimit)
		if err != nil {
			return textValue{}, err
		}
		value.Length = length
	}
	return value, nil
}

func parseAnyURITextValue(normalized string, needs primitiveValueNeed) (textValue, error) {
	characters, err := uriref.Check(normalized)
	if err != nil {
		return textValue{}, err
	}
	value := textValue{Canonical: normalized}
	if !needs.Has(primitiveNeedLength) {
		return value, nil
	}
	length, err := checkedUint32(characters, msgAnyURILengthLimit)
	if err != nil {
		return textValue{}, err
	}
	value.Length = length
	return value, nil
}
