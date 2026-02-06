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
	"github.com/jacoelho/xsd/internal/value"
)

// validateAnyType accepts any value (anyType is the base type for all types)
func validateAnyType(lexical string) error {
	return nil
}

// validateAnySimpleType accepts any simple type value (anySimpleType is the base of all simple types)
func validateAnySimpleType(lexical string) error {
	return nil
}

// validateString accepts any string
func validateString(lexical string) error {
	return nil
}

// validateBoolean validates xs:boolean
func validateBoolean(lexical string) error {
	switch lexical {
	case "true", "false", "1", "0":
		return nil
	}
	return fmt.Errorf("invalid boolean: %s", lexical)
}

// validateDecimal validates xs:decimal
var (
	integerPattern = regexp.MustCompile(`^[+-]?\d+$`)

	durationPattern          = regexp.MustCompile(`^-?P(\d+Y)?(\d+M)?(\d+D)?(T(\d+H)?(\d+M)?(\d+(\.\d+)?S)?)?$`)
	hexBinaryPattern         = regexp.MustCompile(`^[0-9A-Fa-f]+$`)
	base64WhitespaceReplacer = strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
)

func validateDecimal(lexical string) error {
	if _, perr := num.ParseDec([]byte(lexical)); perr != nil {
		return fmt.Errorf("invalid decimal: %s", lexical)
	}
	return nil
}

// validateFloat validates xs:float
// Per XSD 1.0 spec (3.2.4), the lexical space excludes "+INF", so apply
// an explicit lexical check before ParseFloat (which would otherwise accept it).

func validateFloat(lexical string) error {
	if perr := num.ValidateFloatLexical([]byte(lexical)); perr != nil {
		return fmt.Errorf("invalid float: %s", lexical)
	}
	return nil
}

// validateDouble validates xs:double
func validateDouble(lexical string) error {
	if perr := num.ValidateFloatLexical([]byte(lexical)); perr != nil {
		return fmt.Errorf("invalid double: %s", lexical)
	}
	return nil
}

func validateInteger(lexical string) error {
	if !integerPattern.MatchString(lexical) {
		return fmt.Errorf("invalid integer: %s", lexical)
	}
	return nil
}

func validateSignedInt(lexical, label string) (int64, error) {
	if err := validateInteger(lexical); err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(lexical, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", label, lexical)
	}
	return n, nil
}

func validateBoundedInt(lexical, label string, minValue, maxValue int64) error {
	n, err := validateSignedInt(lexical, label)
	if err != nil {
		return err
	}
	if n < minValue || n > maxValue {
		return fmt.Errorf("%s out of range: %s", label, lexical)
	}
	return nil
}

func parseUnsignedIntValue(lexical, label string) (uint64, error) {
	normalized, err := normalizeUnsignedLexical(lexical, label)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(normalized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", label, lexical)
	}
	return n, nil
}

func validateBoundedUint(lexical, label string, maxValue uint64) error {
	n, err := parseUnsignedIntValue(lexical, label)
	if err != nil {
		return err
	}
	if n > maxValue {
		return fmt.Errorf("%s out of range: %s", label, lexical)
	}
	return nil
}

// validateLong validates xs:long
func validateLong(lexical string) error {
	_, err := validateSignedInt(lexical, "long")
	return err
}

// validateInt validates xs:int
func validateInt(lexical string) error {
	return validateBoundedInt(lexical, "int", math.MinInt32, math.MaxInt32)
}

// validateShort validates xs:short
func validateShort(lexical string) error {
	return validateBoundedInt(lexical, "short", math.MinInt16, math.MaxInt16)
}

// validateByte validates xs:byte
func validateByte(lexical string) error {
	return validateBoundedInt(lexical, "byte", math.MinInt8, math.MaxInt8)
}

// validateNonNegativeInteger validates xs:nonNegativeInteger
func validateNonNegativeInteger(lexical string) error {
	if err := validateInteger(lexical); err != nil {
		return err
	}
	if strings.HasPrefix(lexical, "-") {
		for i := 1; i < len(lexical); i++ {
			if lexical[i] != '0' {
				return fmt.Errorf("nonNegativeInteger must be >= 0: %s", lexical)
			}
		}
	}
	return nil
}

func normalizeUnsignedLexical(lexical, label string) (string, error) {
	parsed, perr := num.ParseInt([]byte(lexical))
	if perr != nil {
		return "", fmt.Errorf("invalid %s: %s", label, lexical)
	}
	if parsed.Sign < 0 {
		return "", fmt.Errorf("invalid %s: %s", label, lexical)
	}
	return string(parsed.Digits), nil
}

// validatePositiveInteger validates xs:positiveInteger
func validatePositiveInteger(lexical string) error {
	if err := validateInteger(lexical); err != nil {
		return err
	}
	n, perr := num.ParseInt([]byte(lexical))
	if perr != nil || n.Sign <= 0 {
		return fmt.Errorf("positiveInteger must be >= 1: %s", lexical)
	}
	return nil
}

// validateUnsignedLong validates xs:unsignedLong
func validateUnsignedLong(lexical string) error {
	_, err := parseUnsignedIntValue(lexical, "unsignedLong")
	return err
}

// validateUnsignedInt validates xs:unsignedInt
func validateUnsignedInt(lexical string) error {
	return validateBoundedUint(lexical, "unsignedInt", math.MaxUint32)
}

// validateUnsignedShort validates xs:unsignedShort
func validateUnsignedShort(lexical string) error {
	return validateBoundedUint(lexical, "unsignedShort", math.MaxUint16)
}

// validateUnsignedByte validates xs:unsignedByte
func validateUnsignedByte(lexical string) error {
	return validateBoundedUint(lexical, "unsignedByte", math.MaxUint8)
}

// validateNonPositiveInteger validates xs:nonPositiveInteger
func validateNonPositiveInteger(lexical string) error {
	if err := validateInteger(lexical); err != nil {
		return err
	}
	n, perr := num.ParseInt([]byte(lexical))
	if perr != nil || n.Sign > 0 {
		return fmt.Errorf("nonPositiveInteger must be <= 0: %s", lexical)
	}
	return nil
}

// validateNegativeInteger validates xs:negativeInteger
func validateNegativeInteger(lexical string) error {
	if err := validateInteger(lexical); err != nil {
		return err
	}
	n, perr := num.ParseInt([]byte(lexical))
	if perr != nil || n.Sign >= 0 {
		return fmt.Errorf("negativeInteger must be < 0: %s", lexical)
	}
	return nil
}

// validateNormalizedString validates xs:normalizedString
func validateNormalizedString(lexical string) error {
	if strings.ContainsAny(lexical, "\r\n\t") {
		return fmt.Errorf("normalizedString cannot contain CR, LF, or Tab")
	}
	return nil
}

// validateToken validates xs:token
func validateToken(lexical string) error {
	return value.ValidateToken([]byte(lexical))
}

// validateName validates xs:Name
func validateName(lexical string) error {
	return value.ValidateName([]byte(lexical))
}

// validateNCName validates xs:NCName (Name without colons)
func validateNCName(lexical string) error {
	return value.ValidateNCName([]byte(lexical))
}

// validateID validates xs:ID (same as NCName)
func validateID(lexical string) error {
	return validateNCName(lexical)
}

// validateIDREF validates xs:IDREF (same as NCName)
func validateIDREF(lexical string) error {
	return validateNCName(lexical)
}

// validateIDREFS validates xs:IDREFS (space-separated list of IDREFs)
func validateIDREFS(lexical string) error {
	count := 0
	for part := range FieldsXMLWhitespaceSeq(lexical) {
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
func validateENTITY(lexical string) error {
	return validateNCName(lexical)
}

// validateENTITIES validates xs:ENTITIES (space-separated list of ENTITYs)
func validateENTITIES(lexical string) error {
	count := 0
	for part := range FieldsXMLWhitespaceSeq(lexical) {
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
func validateNMTOKEN(lexical string) error {
	return value.ValidateNMTOKEN([]byte(lexical))
}

// validateNMTOKENS validates xs:NMTOKENS (space-separated list of NMTOKENs)
func validateNMTOKENS(lexical string) error {
	count := 0
	for part := range FieldsXMLWhitespaceSeq(lexical) {
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

func validateLanguage(lexical string) error {
	if err := value.ValidateLanguage([]byte(lexical)); err != nil {
		return fmt.Errorf("invalid language format: %s", lexical)
	}
	return nil
}

// validateDuration validates xs:duration
// Format: PnYnMnDTnHnMnS or -PnYnMnDTnHnMnS
func validateDuration(lexical string) error {
	_, err := ParseXSDDuration(lexical)
	return err
}

// validateDateTime validates xs:dateTime
// Format: CCYY-MM-DDThh:mm:ss[.sss][Z|(+|-)hh:mm]
func validateDateTime(lexical string) error {
	_, err := ParseDateTime(lexical)
	return err
}

// validateDate validates xs:date
// Format: CCYY-MM-DD[Z|(+|-)hh:mm]
func validateDate(lexical string) error {
	_, err := ParseDate(lexical)
	return err
}

// validateTime validates xs:time
// Format: hh:mm:ss[.sss][Z|(+|-)hh:mm]
func validateTime(lexical string) error {
	_, err := ParseTime(lexical)
	return err
}

// validateGYear validates xs:gYear
// Format: CCYY[Z|(+|-)hh:mm]
// Note: Only years 0001-9999 are supported (Go time.Parse limitation + XSD 1.0 no year 0)
func validateGYear(lexical string) error {
	_, err := ParseGYear(lexical)
	return err
}

// validateGYearMonth validates xs:gYearMonth
// Format: CCYY-MM[Z|(+|-)hh:mm]
// Note: Only years 0001-9999 are supported (Go time.Parse limitation + XSD 1.0 no year 0)
func validateGYearMonth(lexical string) error {
	_, err := ParseGYearMonth(lexical)
	return err
}

// validateGMonth validates xs:gMonth
// Format: --MM[Z|(+|-)hh:mm]
func validateGMonth(lexical string) error {
	_, err := ParseGMonth(lexical)
	return err
}

// validateGMonthDay validates xs:gMonthDay
// Format: --MM-DD[Z|(+|-)hh:mm]
func validateGMonthDay(lexical string) error {
	_, err := ParseGMonthDay(lexical)
	return err
}

// validateGDay validates xs:gDay
// Format: ---DD[Z|(+|-)hh:mm]
func validateGDay(lexical string) error {
	_, err := ParseGDay(lexical)
	return err
}

// validateHexBinary validates xs:hexBinary
// Format: hexadecimal digits (0-9, a-f, A-F) in pairs
func validateHexBinary(lexical string) error {
	if lexical == "" {
		return nil // empty is valid
	}

	// must be even number of characters
	if len(lexical)%2 != 0 {
		return fmt.Errorf("hexBinary must have even number of characters")
	}

	// must contain only hex digits
	if !hexBinaryPattern.MatchString(lexical) {
		return fmt.Errorf("hexBinary must contain only hexadecimal digits (0-9, A-F, a-f)")
	}

	// try to decode to verify it's valid hex
	_, err := hex.DecodeString(lexical)
	if err != nil {
		return fmt.Errorf("invalid hexBinary: %w", err)
	}

	return nil
}

// validateBase64Binary validates xs:base64Binary
// Format: base64 encoded string
func validateBase64Binary(lexical string) error {
	if lexical == "" {
		return nil // empty is valid
	}

	// remove whitespace (base64 can contain whitespace in XML)
	lexical = base64WhitespaceReplacer.Replace(lexical)

	// try to decode to verify it's valid base64 (strict padding/charset).
	if _, err := base64.StdEncoding.Strict().DecodeString(lexical); err != nil {
		return fmt.Errorf("invalid base64Binary: %w", err)
	}

	return nil
}

// validateAnyURI validates xs:anyURI
// Format: URI/IRI reference (RFC 2396 and RFC 2732)
func validateAnyURI(lexical string) error {
	return value.ValidateAnyURI([]byte(lexical))
}

// validateQName validates xs:QName
// Format: NCName (possibly qualified with a prefix)
func validateQName(lexical string) error {
	return value.ValidateQName([]byte(lexical))
}

// validateNOTATION validates xs:NOTATION
// Format: QName, but must reference a notation declared in the schema
// We can only validate the QName format here; notation reference validation
// must be done at the schema level
func validateNOTATION(lexical string) error {
	// format is the same as QName
	return validateQName(lexical)
}
