package values

import (
	"time"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

// ValidateToken validates xs:token lexical values.
func ValidateToken(v []byte) error {
	return value.ValidateToken(v)
}

// ValidateName validates xs:Name lexical values.
func ValidateName(v []byte) error {
	return value.ValidateName(v)
}

// ValidateNCName validates xs:NCName lexical values.
func ValidateNCName(v []byte) error {
	return value.ValidateNCName(v)
}

// ValidateNMTOKEN validates xs:NMTOKEN lexical values.
func ValidateNMTOKEN(v []byte) error {
	return value.ValidateNMTOKEN(v)
}

// ValidateLanguage validates xs:language lexical values.
func ValidateLanguage(v []byte) error {
	return value.ValidateLanguage(v)
}

// ValidateAnyURI validates xs:anyURI lexical values.
func ValidateAnyURI(v []byte) error {
	return value.ValidateAnyURI(v)
}

// ValidateQName validates xs:QName lexical values.
func ValidateQName(v []byte) error {
	return value.ValidateQName(v)
}

// ParseBoolean parses xs:boolean lexical values.
func ParseBoolean(v []byte) (bool, error) {
	return value.ParseBoolean(v)
}

// ParseDecimal parses xs:decimal lexical values.
func ParseDecimal(v []byte) (num.Dec, error) {
	return value.ParseDecimal(v)
}

// ParseInteger parses xs:integer lexical values.
func ParseInteger(v []byte) (num.Int, error) {
	return value.ParseInteger(v)
}

// ParseFloat parses xs:float lexical values.
func ParseFloat(v []byte) (float32, error) {
	return value.ParseFloat(v)
}

// ParseDouble parses xs:double lexical values.
func ParseDouble(v []byte) (float64, error) {
	return value.ParseDouble(v)
}

// CanonicalFloat canonicalizes float values for value-space comparisons.
func CanonicalFloat(v float64, bits int) string {
	return value.CanonicalFloat(v, bits)
}

// CanonicalDateTimeString canonicalizes dateTime lexical values.
func CanonicalDateTimeString(v time.Time, kind string, tzKind value.TimezoneKind) string {
	return value.CanonicalDateTimeString(v, kind, tzKind)
}

// HasTimezone reports whether a lexical temporal value contains a timezone.
func HasTimezone(v []byte) bool {
	return value.HasTimezone(v)
}

// FormatFraction formats the fractional component for canonical decimal output.
func FormatFraction(nanos int) string {
	return value.FormatFraction(nanos)
}

// UpperHex uppercases hexadecimal digits.
func UpperHex(dst, src []byte) []byte {
	return value.UpperHex(dst, src)
}

// KeyForValidatorKind derives deterministic value-key encoding from canonical lexical bytes.
func KeyForValidatorKind(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForValidatorKind(kind, canonical)
}

// KeyForPrimitiveName derives deterministic value-key encoding from normalized lexical text.
func KeyForPrimitiveName(primitive, normalized string, ctx map[string]string) (runtime.ValueKind, []byte, error) {
	return valuesemantics.KeyForPrimitiveName(primitive, normalized, ctx)
}
