package types

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/num"
	valuepkg "github.com/jacoelho/xsd/internal/value"
)

// validateAnyType accepts any value (anyType is the base type for all types)
func validateAnyType(value string) error {
	return nil
}

// validateAnySimpleType accepts any simple type value (anySimpleType is the base of all simple types)
func validateAnySimpleType(value string) error {
	return nil
}

// validateString accepts any string
func validateString(value string) error {
	return nil
}

// validateBoolean validates xs:boolean
func validateBoolean(value string) error {
	switch value {
	case "true", "false", "1", "0":
		return nil
	}
	return fmt.Errorf("invalid boolean: %s", value)
}

// validateDecimal validates xs:decimal
var (
	integerPattern = regexp.MustCompile(`^[+-]?\d+$`)

	durationPattern          = regexp.MustCompile(`^-?P(\d+Y)?(\d+M)?(\d+D)?(T(\d+H)?(\d+M)?(\d+(\.\d+)?S)?)?$`)
	hexBinaryPattern         = regexp.MustCompile(`^[0-9A-Fa-f]+$`)
	base64WhitespaceReplacer = strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
)

func validateDecimal(value string) error {
	if _, perr := num.ParseDec([]byte(value)); perr != nil {
		return fmt.Errorf("invalid decimal: %s", value)
	}
	return nil
}

// validateFloat validates xs:float
// Per XSD 1.0 spec (3.2.4), the lexical space excludes "+INF", so apply
// an explicit lexical check before ParseFloat (which would otherwise accept it).

func validateFloat(value string) error {
	if perr := num.ValidateFloatLexical([]byte(value)); perr != nil {
		return fmt.Errorf("invalid float: %s", value)
	}
	return nil
}

// validateDouble validates xs:double
func validateDouble(value string) error {
	if perr := num.ValidateFloatLexical([]byte(value)); perr != nil {
		return fmt.Errorf("invalid double: %s", value)
	}
	return nil
}

func validateInteger(value string) error {
	if !integerPattern.MatchString(value) {
		return fmt.Errorf("invalid integer: %s", value)
	}
	return nil
}

func validateSignedInt(value, label string) (int64, error) {
	if err := validateInteger(value); err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", label, value)
	}
	return n, nil
}

func validateBoundedInt(value, label string, minValue, maxValue int64) error {
	n, err := validateSignedInt(value, label)
	if err != nil {
		return err
	}
	if n < minValue || n > maxValue {
		return fmt.Errorf("%s out of range: %s", label, value)
	}
	return nil
}

func parseUnsignedIntValue(value, label string) (uint64, error) {
	normalized, err := normalizeUnsignedLexical(value)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(normalized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", label, value)
	}
	return n, nil
}

func validateBoundedUint(value, label string, maxValue uint64) error {
	n, err := parseUnsignedIntValue(value, label)
	if err != nil {
		return err
	}
	if n > maxValue {
		return fmt.Errorf("%s out of range: %s", label, value)
	}
	return nil
}

// validateLong validates xs:long
func validateLong(value string) error {
	_, err := validateSignedInt(value, "long")
	return err
}

// validateInt validates xs:int
func validateInt(value string) error {
	return validateBoundedInt(value, "int", math.MinInt32, math.MaxInt32)
}

// validateShort validates xs:short
func validateShort(value string) error {
	return validateBoundedInt(value, "short", math.MinInt16, math.MaxInt16)
}

// validateByte validates xs:byte
func validateByte(value string) error {
	return validateBoundedInt(value, "byte", math.MinInt8, math.MaxInt8)
}

// validateNonNegativeInteger validates xs:nonNegativeInteger
func validateNonNegativeInteger(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	if strings.HasPrefix(value, "-") {
		for i := 1; i < len(value); i++ {
			if value[i] != '0' {
				return fmt.Errorf("nonNegativeInteger must be >= 0: %s", value)
			}
		}
	}
	return nil
}

func normalizeUnsignedLexical(value string) (string, error) {
	if err := validateNonNegativeInteger(value); err != nil {
		return "", err
	}
	if strings.HasPrefix(value, "+") {
		return value[1:], nil
	}
	if strings.HasPrefix(value, "-") {
		return "0", nil
	}
	return value, nil
}

// validatePositiveInteger validates xs:positiveInteger
func validatePositiveInteger(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	n, perr := num.ParseInt([]byte(value))
	if perr != nil || n.Sign <= 0 {
		return fmt.Errorf("positiveInteger must be >= 1: %s", value)
	}
	return nil
}

// validateUnsignedLong validates xs:unsignedLong
func validateUnsignedLong(value string) error {
	_, err := parseUnsignedIntValue(value, "unsignedLong")
	return err
}

// validateUnsignedInt validates xs:unsignedInt
func validateUnsignedInt(value string) error {
	return validateBoundedUint(value, "unsignedInt", math.MaxUint32)
}

// validateUnsignedShort validates xs:unsignedShort
func validateUnsignedShort(value string) error {
	return validateBoundedUint(value, "unsignedShort", math.MaxUint16)
}

// validateUnsignedByte validates xs:unsignedByte
func validateUnsignedByte(value string) error {
	return validateBoundedUint(value, "unsignedByte", math.MaxUint8)
}

// validateNonPositiveInteger validates xs:nonPositiveInteger
func validateNonPositiveInteger(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	n, perr := num.ParseInt([]byte(value))
	if perr != nil || n.Sign > 0 {
		return fmt.Errorf("nonPositiveInteger must be <= 0: %s", value)
	}
	return nil
}

// validateNegativeInteger validates xs:negativeInteger
func validateNegativeInteger(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	n, perr := num.ParseInt([]byte(value))
	if perr != nil || n.Sign >= 0 {
		return fmt.Errorf("negativeInteger must be < 0: %s", value)
	}
	return nil
}

// validateNormalizedString validates xs:normalizedString
func validateNormalizedString(value string) error {
	if strings.ContainsAny(value, "\r\n\t") {
		return fmt.Errorf("normalizedString cannot contain CR, LF, or Tab")
	}
	return nil
}

// validateToken validates xs:token
func validateToken(value string) error {
	return valuepkg.ValidateToken([]byte(value))
}

// validateName validates xs:Name
func validateName(value string) error {
	return valuepkg.ValidateName([]byte(value))
}

// validateNCName validates xs:NCName (Name without colons)
func validateNCName(value string) error {
	return valuepkg.ValidateNCName([]byte(value))
}

// validateID validates xs:ID (same as NCName)
func validateID(value string) error {
	return validateNCName(value)
}

// validateIDREF validates xs:IDREF (same as NCName)
func validateIDREF(value string) error {
	return validateNCName(value)
}

// validateIDREFS validates xs:IDREFS (space-separated list of IDREFs)
func validateIDREFS(value string) error {
	count := 0
	for part := range FieldsXMLWhitespaceSeq(value) {
		count++
		if err := validateIDREF(part); err != nil {
			return fmt.Errorf("invalid IDREFS: %w", err)
		}
	}
	if count == 0 {
		return fmt.Errorf("invalid IDREFS: empty value")
	}
	return nil
}

// validateENTITY validates xs:ENTITY (same as NCName)
func validateENTITY(value string) error {
	return validateNCName(value)
}

// validateENTITIES validates xs:ENTITIES (space-separated list of ENTITYs)
func validateENTITIES(value string) error {
	count := 0
	for part := range FieldsXMLWhitespaceSeq(value) {
		count++
		if err := validateENTITY(part); err != nil {
			return fmt.Errorf("invalid ENTITIES: %w", err)
		}
	}
	if count == 0 {
		return fmt.Errorf("invalid ENTITIES: empty value")
	}
	return nil
}

// validateNMTOKEN validates xs:NMTOKEN
// NMTOKEN is any string matching NameChar+
func validateNMTOKEN(value string) error {
	return valuepkg.ValidateNMTOKEN([]byte(value))
}

// validateNMTOKENS validates xs:NMTOKENS (space-separated list of NMTOKENs)
func validateNMTOKENS(value string) error {
	count := 0
	for part := range FieldsXMLWhitespaceSeq(value) {
		count++
		if err := validateNMTOKEN(part); err != nil {
			return fmt.Errorf("invalid NMTOKENS: %w", err)
		}
	}
	if count == 0 {
		return fmt.Errorf("invalid NMTOKENS: empty value")
	}
	return nil
}

// validateLanguage validates xs:language
// Format: language identifier per RFC 3066
// Pattern: [a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*

func validateLanguage(value string) error {
	if err := valuepkg.ValidateLanguage([]byte(value)); err != nil {
		return fmt.Errorf("invalid language format: %s", value)
	}
	return nil
}

// validateDuration validates xs:duration
// Format: PnYnMnDTnHnMnS or -PnYnMnDTnHnMnS
func validateDuration(value string) error {
	_, err := ParseXSDDuration(value)
	return err
}

// validateDateTime validates xs:dateTime
// Format: CCYY-MM-DDThh:mm:ss[.sss][Z|(+|-)hh:mm]
func validateDateTime(value string) error {
	_, err := ParseDateTime(value)
	return err
}

// validateDate validates xs:date
// Format: CCYY-MM-DD[Z|(+|-)hh:mm]
func validateDate(value string) error {
	_, err := ParseDate(value)
	return err
}

// validateTime validates xs:time
// Format: hh:mm:ss[.sss][Z|(+|-)hh:mm]
func validateTime(value string) error {
	_, err := ParseTime(value)
	return err
}

// validateGYear validates xs:gYear
// Format: CCYY[Z|(+|-)hh:mm]
// Note: Only years 0001-9999 are supported (Go time.Parse limitation + XSD 1.0 no year 0)
func validateGYear(value string) error {
	_, err := ParseGYear(value)
	return err
}

// validateGYearMonth validates xs:gYearMonth
// Format: CCYY-MM[Z|(+|-)hh:mm]
// Note: Only years 0001-9999 are supported (Go time.Parse limitation + XSD 1.0 no year 0)
func validateGYearMonth(value string) error {
	_, err := ParseGYearMonth(value)
	return err
}

// validateGMonth validates xs:gMonth
// Format: --MM[Z|(+|-)hh:mm]
func validateGMonth(value string) error {
	_, err := ParseGMonth(value)
	return err
}

// validateGMonthDay validates xs:gMonthDay
// Format: --MM-DD[Z|(+|-)hh:mm]
func validateGMonthDay(value string) error {
	_, err := ParseGMonthDay(value)
	return err
}

// validateGDay validates xs:gDay
// Format: ---DD[Z|(+|-)hh:mm]
func validateGDay(value string) error {
	_, err := ParseGDay(value)
	return err
}

// validateHexBinary validates xs:hexBinary
// Format: hexadecimal digits (0-9, a-f, A-F) in pairs
func validateHexBinary(value string) error {
	if value == "" {
		return nil // empty is valid
	}

	// must be even number of characters
	if len(value)%2 != 0 {
		return fmt.Errorf("hexBinary must have even number of characters")
	}

	// must contain only hex digits
	if !hexBinaryPattern.MatchString(value) {
		return fmt.Errorf("hexBinary must contain only hexadecimal digits (0-9, A-F, a-f)")
	}

	// try to decode to verify it's valid hex
	_, err := hex.DecodeString(value)
	if err != nil {
		return fmt.Errorf("invalid hexBinary: %w", err)
	}

	return nil
}

// validateBase64Binary validates xs:base64Binary
// Format: base64 encoded string
func validateBase64Binary(value string) error {
	if value == "" {
		return nil // empty is valid
	}

	// remove whitespace (base64 can contain whitespace in XML)
	value = base64WhitespaceReplacer.Replace(value)

	// try to decode to verify it's valid base64 (strict padding/charset).
	if _, err := base64.StdEncoding.Strict().DecodeString(value); err != nil {
		return fmt.Errorf("invalid base64Binary: %w", err)
	}

	return nil
}

// validateAnyURI validates xs:anyURI
// Format: URI/IRI reference (RFC 2396 and RFC 2732)
func validateAnyURI(value string) error {
	return valuepkg.ValidateAnyURI([]byte(value))
}

// validateQName validates xs:QName
// Format: NCName (possibly qualified with a prefix)
func validateQName(value string) error {
	return valuepkg.ValidateQName([]byte(value))
}

// validateNOTATION validates xs:NOTATION
// Format: QName, but must reference a notation declared in the schema
// We can only validate the QName format here; notation reference validation
// must be done at the schema level
func validateNOTATION(value string) error {
	// format is the same as QName
	return validateQName(value)
}
