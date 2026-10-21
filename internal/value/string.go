package value

import (
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/uriref"
)

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
		return parseAnyURITextValue(normalized, needs)
	case PrimitiveBoolean, PrimitiveDecimal, PrimitiveFloat, PrimitiveDouble, PrimitiveDuration,
		PrimitiveDateTime, PrimitiveTime, PrimitiveDate,
		PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth,
		PrimitiveHexBinary, PrimitiveBase64Binary, PrimitiveQName, PrimitiveNotation:
		return TextValue{}, ErrMetadata
	default:
		err := ErrMetadata
		return TextValue{}, err
	}
	value := TextValue{Canonical: normalized}
	if needs.Has(PrimitiveNeedLength) {
		length, err := checkedUint32(utf8.RuneCountInString(normalized), msgStringLengthLimit)
		if err != nil {
			return TextValue{}, err
		}
		value.Length = length
	}
	return value, nil
}

func parseAnyURITextValue(normalized string, needs PrimitiveValueNeed) (TextValue, error) {
	characters, err := uriref.Check(normalized)
	if err != nil {
		return TextValue{}, err
	}
	value := TextValue{Canonical: normalized}
	if !needs.Has(PrimitiveNeedLength) {
		return value, nil
	}
	length, err := checkedUint32(characters, msgAnyURILengthLimit)
	if err != nil {
		return TextValue{}, err
	}
	value.Length = length
	return value, nil
}
