package runtime

import (
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

// TemporalSpec defines temporal parse and key-tag metadata for validator kinds.
type TemporalSpec struct {
	Kind   temporal.Kind
	KeyTag byte
}

var temporalSpecs = [...]TemporalSpec{
	VDateTime:   {Kind: temporal.KindDateTime, KeyTag: 0},
	VTime:       {Kind: temporal.KindTime, KeyTag: 2},
	VDate:       {Kind: temporal.KindDate, KeyTag: 1},
	VGYearMonth: {Kind: temporal.KindGYearMonth, KeyTag: 3},
	VGYear:      {Kind: temporal.KindGYear, KeyTag: 4},
	VGMonthDay:  {Kind: temporal.KindGMonthDay, KeyTag: 5},
	VGDay:       {Kind: temporal.KindGDay, KeyTag: 6},
	VGMonth:     {Kind: temporal.KindGMonth, KeyTag: 7},
}

// TemporalSpecForValidatorKind returns temporal metadata for a runtime validator kind.
func TemporalSpecForValidatorKind(kind ValidatorKind) (TemporalSpec, bool) {
	if int(kind) >= len(temporalSpecs) {
		return TemporalSpec{}, false
	}
	spec := temporalSpecs[kind]
	if spec.Kind == temporal.KindInvalid {
		return TemporalSpec{}, false
	}
	return spec, true
}

// ValidateStringKind validates string-family lexical space constraints.
func ValidateStringKind(kind StringKind, normalized []byte) error {
	switch kind {
	case StringToken:
		return value.ValidateToken(normalized)
	case StringLanguage:
		return value.ValidateLanguage(normalized)
	case StringName:
		return value.ValidateName(normalized)
	case StringNCName:
		return value.ValidateNCName(normalized)
	case StringID, StringIDREF, StringEntity:
		return value.ValidateNCName(normalized)
	case StringNMTOKEN:
		return value.ValidateNMTOKEN(normalized)
	default:
		return nil
	}
}
