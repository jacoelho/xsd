package values

import (
	"github.com/jacoelho/xsd/internal/runtime"
	valuelex "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

var ValidateToken = valuelex.ValidateToken
var ValidateName = valuelex.ValidateName
var ValidateNCName = valuelex.ValidateNCName
var ValidateNMTOKEN = valuelex.ValidateNMTOKEN
var ValidateLanguage = valuelex.ValidateLanguage
var ValidateAnyURI = valuelex.ValidateAnyURI
var ValidateQName = valuelex.ValidateQName

var ParseBoolean = valuelex.ParseBoolean
var ParseDecimal = valuelex.ParseDecimal
var ParseInteger = valuelex.ParseInteger
var ParseFloat = valuelex.ParseFloat
var ParseDouble = valuelex.ParseDouble

var CanonicalFloat = valuelex.CanonicalFloat
var CanonicalDateTimeString = valuelex.CanonicalDateTimeString
var HasTimezone = valuelex.HasTimezone
var FormatFraction = valuelex.FormatFraction
var UpperHex = valuelex.UpperHex

// KeyForValidatorKind derives deterministic value-key encoding from canonical lexical bytes.
func KeyForValidatorKind(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForValidatorKind(kind, canonical)
}

// KeyForPrimitiveName derives deterministic value-key encoding from normalized lexical text.
func KeyForPrimitiveName(primitive, normalized string, ctx map[string]string) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForPrimitiveName(primitive, normalized, ctx)
}
