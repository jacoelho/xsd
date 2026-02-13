package values

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

// ValidateToken validates xs:token lexical values.
var ValidateToken = value.ValidateToken

// ValidateName validates xs:Name lexical values.
var ValidateName = value.ValidateName

// ValidateNCName validates xs:NCName lexical values.
var ValidateNCName = value.ValidateNCName

// ValidateNMTOKEN validates xs:NMTOKEN lexical values.
var ValidateNMTOKEN = value.ValidateNMTOKEN

// ValidateLanguage validates xs:language lexical values.
var ValidateLanguage = value.ValidateLanguage

// ValidateAnyURI validates xs:anyURI lexical values.
var ValidateAnyURI = value.ValidateAnyURI

// ValidateQName validates xs:QName lexical values.
var ValidateQName = value.ValidateQName

// ParseBoolean parses xs:boolean lexical values.
var ParseBoolean = value.ParseBoolean

// ParseDecimal parses xs:decimal lexical values.
var ParseDecimal = value.ParseDecimal

// ParseInteger parses xs:integer lexical values.
var ParseInteger = value.ParseInteger

// ParseFloat parses xs:float lexical values.
var ParseFloat = value.ParseFloat

// ParseDouble parses xs:double lexical values.
var ParseDouble = value.ParseDouble

// CanonicalFloat canonicalizes float values for value-space comparisons.
var CanonicalFloat = value.CanonicalFloat

// CanonicalDateTimeString canonicalizes dateTime lexical values.
var CanonicalDateTimeString = value.CanonicalDateTimeString

// HasTimezone reports whether a lexical temporal value contains a timezone.
var HasTimezone = value.HasTimezone

// FormatFraction formats the fractional component for canonical decimal output.
var FormatFraction = value.FormatFraction

// UpperHex uppercases hexadecimal digits.
var UpperHex = value.UpperHex

// KeyForValidatorKind derives deterministic value-key encoding from canonical lexical bytes.
func KeyForValidatorKind(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForValidatorKind(kind, canonical)
}

// KeyForPrimitiveName derives deterministic value-key encoding from normalized lexical text.
func KeyForPrimitiveName(primitive, normalized string, ctx map[string]string) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForPrimitiveName(primitive, normalized, ctx)
}
