package values

import (
	"github.com/jacoelho/xsd/internal/runtime"
	valuelex "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

// ValidateToken is an exported variable.
var ValidateToken = valuelex.ValidateToken

// ValidateName is an exported variable.
var ValidateName = valuelex.ValidateName

// ValidateNCName is an exported variable.
var ValidateNCName = valuelex.ValidateNCName

// ValidateNMTOKEN is an exported variable.
var ValidateNMTOKEN = valuelex.ValidateNMTOKEN

// ValidateLanguage is an exported variable.
var ValidateLanguage = valuelex.ValidateLanguage

// ValidateAnyURI is an exported variable.
var ValidateAnyURI = valuelex.ValidateAnyURI

// ValidateQName is an exported variable.
var ValidateQName = valuelex.ValidateQName

// ParseBoolean is an exported variable.
var ParseBoolean = valuelex.ParseBoolean

// ParseDecimal is an exported variable.
var ParseDecimal = valuelex.ParseDecimal

// ParseInteger is an exported variable.
var ParseInteger = valuelex.ParseInteger

// ParseFloat is an exported variable.
var ParseFloat = valuelex.ParseFloat

// ParseDouble is an exported variable.
var ParseDouble = valuelex.ParseDouble

// CanonicalFloat is an exported variable.
var CanonicalFloat = valuelex.CanonicalFloat

// CanonicalDateTimeString is an exported variable.
var CanonicalDateTimeString = valuelex.CanonicalDateTimeString

// HasTimezone is an exported variable.
var HasTimezone = valuelex.HasTimezone

// FormatFraction is an exported variable.
var FormatFraction = valuelex.FormatFraction

// UpperHex is an exported variable.
var UpperHex = valuelex.UpperHex

// KeyForValidatorKind derives deterministic value-key encoding from canonical lexical bytes.
func KeyForValidatorKind(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForValidatorKind(kind, canonical)
}

// KeyForPrimitiveName derives deterministic value-key encoding from normalized lexical text.
func KeyForPrimitiveName(primitive, normalized string, ctx map[string]string) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForPrimitiveName(primitive, normalized, ctx)
}
