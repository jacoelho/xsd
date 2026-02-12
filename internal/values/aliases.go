package values

import (
	"github.com/jacoelho/xsd/internal/runtime"
	valuelex "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

// ValidateToken validates xs:token lexical values.
var ValidateToken = valuelex.ValidateToken

// ValidateName validates xs:Name lexical values.
var ValidateName = valuelex.ValidateName

// ValidateNCName validates xs:NCName lexical values.
var ValidateNCName = valuelex.ValidateNCName

// ValidateNMTOKEN validates xs:NMTOKEN lexical values.
var ValidateNMTOKEN = valuelex.ValidateNMTOKEN

// ValidateLanguage validates xs:language lexical values.
var ValidateLanguage = valuelex.ValidateLanguage

// ValidateAnyURI validates xs:anyURI lexical values.
var ValidateAnyURI = valuelex.ValidateAnyURI

// ValidateQName validates xs:QName lexical values.
var ValidateQName = valuelex.ValidateQName

// ParseBoolean parses xs:boolean lexical values.
var ParseBoolean = valuelex.ParseBoolean

// ParseDecimal parses xs:decimal lexical values.
var ParseDecimal = valuelex.ParseDecimal

// ParseInteger parses xs:integer lexical values.
var ParseInteger = valuelex.ParseInteger

// ParseFloat parses xs:float lexical values.
var ParseFloat = valuelex.ParseFloat

// ParseDouble parses xs:double lexical values.
var ParseDouble = valuelex.ParseDouble

// CanonicalFloat canonicalizes float values for value-space comparisons.
var CanonicalFloat = valuelex.CanonicalFloat

// CanonicalDateTimeString canonicalizes dateTime lexical values.
var CanonicalDateTimeString = valuelex.CanonicalDateTimeString

// HasTimezone reports whether a lexical temporal value contains a timezone.
var HasTimezone = valuelex.HasTimezone

// FormatFraction formats the fractional component for canonical decimal output.
var FormatFraction = valuelex.FormatFraction

// UpperHex uppercases hexadecimal digits.
var UpperHex = valuelex.UpperHex

// KeyForValidatorKind derives deterministic value-key encoding from canonical lexical bytes.
func KeyForValidatorKind(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForValidatorKind(kind, canonical)
}

// KeyForPrimitiveName derives deterministic value-key encoding from normalized lexical text.
func KeyForPrimitiveName(primitive, normalized string, ctx map[string]string) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForPrimitiveName(primitive, normalized, ctx)
}
