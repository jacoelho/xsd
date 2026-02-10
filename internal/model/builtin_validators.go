package model

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/durationlex"
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

// validateLong validates xs:long
func validateLong(lexical string) error {
	_, err := validateSignedInt(lexical, "long")
	return err
}

// validateInt validates xs:int.
func validateInt(lexical string) error {
	n, err := validateSignedInt(lexical, "int")
	if err != nil {
		return err
	}
	if n < math.MinInt32 || n > math.MaxInt32 {
		return fmt.Errorf("int out of range: %s", lexical)
	}
	return nil
}

// validateShort validates xs:short.
func validateShort(lexical string) error {
	n, err := validateSignedInt(lexical, "short")
	if err != nil {
		return err
	}
	if n < math.MinInt16 || n > math.MaxInt16 {
		return fmt.Errorf("short out of range: %s", lexical)
	}
	return nil
}

// validateByte validates xs:byte.
func validateByte(lexical string) error {
	n, err := validateSignedInt(lexical, "byte")
	if err != nil {
		return err
	}
	if n < math.MinInt8 || n > math.MaxInt8 {
		return fmt.Errorf("byte out of range: %s", lexical)
	}
	return nil
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

// validateUnsignedInt validates xs:unsignedInt.
func validateUnsignedInt(lexical string) error {
	n, err := parseUnsignedIntValue(lexical, "unsignedInt")
	if err != nil {
		return err
	}
	if n > math.MaxUint32 {
		return fmt.Errorf("unsignedInt out of range: %s", lexical)
	}
	return nil
}

// validateUnsignedShort validates xs:unsignedShort.
func validateUnsignedShort(lexical string) error {
	n, err := parseUnsignedIntValue(lexical, "unsignedShort")
	if err != nil {
		return err
	}
	if n > math.MaxUint16 {
		return fmt.Errorf("unsignedShort out of range: %s", lexical)
	}
	return nil
}

// validateUnsignedByte validates xs:unsignedByte.
func validateUnsignedByte(lexical string) error {
	n, err := parseUnsignedIntValue(lexical, "unsignedByte")
	if err != nil {
		return err
	}
	if n > math.MaxUint8 {
		return fmt.Errorf("unsignedByte out of range: %s", lexical)
	}
	return nil
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

func validateFromBytes(validate func([]byte) error) TypeValidator {
	return func(lexical string) error {
		return validate([]byte(lexical))
	}
}

var (
	// validateToken validates xs:token.
	validateToken = validateFromBytes(value.ValidateToken)
	// validateName validates xs:Name.
	validateName = validateFromBytes(value.ValidateName)
	// validateNCName validates xs:NCName (Name without colons).
	validateNCName = validateFromBytes(value.ValidateNCName)
)

func validateWhitespaceSeparatedList(typeName string, itemValidator TypeValidator, lexical string) error {
	count := 0
	for part := range FieldsXMLWhitespaceSeq(lexical) {
		count++
		if err := itemValidator(part); err != nil {
			return fmt.Errorf("invalid %s: %w", typeName, err)
		}
	}
	if count == 0 {
		return fmt.Errorf("invalid %s: empty value", typeName)
	}
	return nil
}

// validateIDREFS validates xs:IDREFS (space-separated list of IDREFs)
func validateIDREFS(lexical string) error {
	return validateWhitespaceSeparatedList("IDREFS", validateNCName, lexical)
}

// validateENTITIES validates xs:ENTITIES (space-separated list of ENTITYs)
func validateENTITIES(lexical string) error {
	return validateWhitespaceSeparatedList("ENTITIES", validateNCName, lexical)
}

var (
	// validateNMTOKEN validates xs:NMTOKEN (NameChar+).
	validateNMTOKEN = validateFromBytes(value.ValidateNMTOKEN)
)

// validateNMTOKENS validates xs:NMTOKENS (space-separated list of NMTOKENs)
func validateNMTOKENS(lexical string) error {
	return validateWhitespaceSeparatedList("NMTOKENS", validateNMTOKEN, lexical)
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
	_, err := durationlex.Parse(lexical)
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
	_, err := value.ParseDate([]byte(lexical))
	return err
}

// validateTime validates xs:time
// Format: hh:mm:ss[.sss][Z|(+|-)hh:mm]
func validateTime(lexical string) error {
	_, err := value.ParseTime([]byte(lexical))
	return err
}

// validateGYear validates xs:gYear
// Format: CCYY[Z|(+|-)hh:mm]
// Note: Only years 0001-9999 are supported (Go time.Parse limitation + XSD 1.0 no year 0)
func validateGYear(lexical string) error {
	_, err := value.ParseGYear([]byte(lexical))
	return err
}

// validateGYearMonth validates xs:gYearMonth
// Format: CCYY-MM[Z|(+|-)hh:mm]
// Note: Only years 0001-9999 are supported (Go time.Parse limitation + XSD 1.0 no year 0)
func validateGYearMonth(lexical string) error {
	_, err := value.ParseGYearMonth([]byte(lexical))
	return err
}

// validateGMonth validates xs:gMonth
// Format: --MM[Z|(+|-)hh:mm]
func validateGMonth(lexical string) error {
	_, err := value.ParseGMonth([]byte(lexical))
	return err
}

// validateGMonthDay validates xs:gMonthDay
// Format: --MM-DD[Z|(+|-)hh:mm]
func validateGMonthDay(lexical string) error {
	_, err := value.ParseGMonthDay([]byte(lexical))
	return err
}

// validateGDay validates xs:gDay
// Format: ---DD[Z|(+|-)hh:mm]
func validateGDay(lexical string) error {
	_, err := value.ParseGDay([]byte(lexical))
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

var (
	// validateAnyURI validates xs:anyURI lexical form.
	validateAnyURI = validateFromBytes(value.ValidateAnyURI)
	// validateQName validates xs:QName lexical form.
	validateQName = validateFromBytes(value.ValidateQName)
)
