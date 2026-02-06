package semantics

import (
	"encoding/base64"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

// TemporalSpec defines temporal parse and key-tag metadata for validator kinds.
type TemporalSpec struct {
	Kind   temporal.Kind
	KeyTag byte
}

var temporalSpecs = [...]TemporalSpec{
	runtime.VDateTime:   {Kind: temporal.KindDateTime, KeyTag: 0},
	runtime.VTime:       {Kind: temporal.KindTime, KeyTag: 2},
	runtime.VDate:       {Kind: temporal.KindDate, KeyTag: 1},
	runtime.VGYearMonth: {Kind: temporal.KindGYearMonth, KeyTag: 3},
	runtime.VGYear:      {Kind: temporal.KindGYear, KeyTag: 4},
	runtime.VGMonthDay:  {Kind: temporal.KindGMonthDay, KeyTag: 5},
	runtime.VGDay:       {Kind: temporal.KindGDay, KeyTag: 6},
	runtime.VGMonth:     {Kind: temporal.KindGMonth, KeyTag: 7},
}

// TemporalSpecForValidatorKind returns temporal metadata for a runtime validator kind.
func TemporalSpecForValidatorKind(kind runtime.ValidatorKind) (TemporalSpec, bool) {
	if int(kind) >= len(temporalSpecs) {
		return TemporalSpec{}, false
	}
	spec := temporalSpecs[kind]
	if spec.Kind == temporal.KindInvalid {
		return TemporalSpec{}, false
	}
	return spec, true
}

// TemporalKindForPrimitive returns the temporal kind for a primitive XSD name.
func TemporalKindForPrimitive(primName string) (temporal.Kind, bool) {
	switch primName {
	case "dateTime":
		return temporal.KindDateTime, true
	case "date":
		return temporal.KindDate, true
	case "time":
		return temporal.KindTime, true
	case "gYearMonth":
		return temporal.KindGYearMonth, true
	case "gYear":
		return temporal.KindGYear, true
	case "gMonthDay":
		return temporal.KindGMonthDay, true
	case "gDay":
		return temporal.KindGDay, true
	case "gMonth":
		return temporal.KindGMonth, true
	default:
		return temporal.KindInvalid, false
	}
}

// ValidateStringKind validates string-family lexical space constraints.
func ValidateStringKind(kind runtime.StringKind, normalized []byte) error {
	switch kind {
	case runtime.StringToken:
		return value.ValidateToken(normalized)
	case runtime.StringLanguage:
		return value.ValidateLanguage(normalized)
	case runtime.StringName:
		return value.ValidateName(normalized)
	case runtime.StringNCName:
		return value.ValidateNCName(normalized)
	case runtime.StringID, runtime.StringIDREF, runtime.StringEntity:
		return value.ValidateNCName(normalized)
	case runtime.StringNMTOKEN:
		return value.ValidateNMTOKEN(normalized)
	default:
		return nil
	}
}

// EncodeBase64 returns canonical base64 text for data.
func EncodeBase64(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}
