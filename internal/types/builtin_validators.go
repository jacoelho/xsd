package types

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
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

	languagePattern          = regexp.MustCompile(`^[a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*$`)
	durationPattern          = regexp.MustCompile(`^-?P(\d+Y)?(\d+M)?(\d+D)?(T(\d+H)?(\d+M)?(\d+(\.\d+)?S)?)?$`)
	hexBinaryPattern         = regexp.MustCompile(`^[0-9A-Fa-f]+$`)
	uriSchemePattern         = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)
	base64WhitespaceReplacer = strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
)

var fractionalLayouts = [...]string{
	"",
	".0",
	".00",
	".000",
	".0000",
	".00000",
	".000000",
	".0000000",
	".00000000",
	".000000000",
}

func validateDecimal(value string) error {
	if !isValidDecimalLexical(value) {
		return fmt.Errorf("invalid decimal: %s", value)
	}
	return nil
}

// validateFloat validates xs:float
// Per XSD 1.0 spec (3.2.4), the lexical space excludes "+INF", so apply
// an explicit lexical check before ParseFloat (which would otherwise accept it).

func validateFloat(value string) error {
	if value == "INF" || value == "-INF" || value == "NaN" {
		return nil
	}
	if !isFloatLexical(value) {
		return fmt.Errorf("invalid float: %s", value)
	}
	return nil
}

// validateDouble validates xs:double
func validateDouble(value string) error {
	if value == "INF" || value == "-INF" || value == "NaN" {
		return nil
	}
	if !isFloatLexical(value) {
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
	n, ok := new(big.Int).SetString(value, 10)
	if !ok || n.Sign() <= 0 {
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
	n, ok := new(big.Int).SetString(value, 10)
	if !ok || n.Sign() > 0 {
		return fmt.Errorf("nonPositiveInteger must be <= 0: %s", value)
	}
	return nil
}

// validateNegativeInteger validates xs:negativeInteger
func validateNegativeInteger(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	n, ok := new(big.Int).SetString(value, 10)
	if !ok || n.Sign() >= 0 {
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
	if strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		return fmt.Errorf("token cannot have leading or trailing whitespace")
	}
	if strings.Contains(value, "  ") {
		return fmt.Errorf("token cannot have consecutive spaces")
	}
	if strings.ContainsAny(value, "\r\n\t") {
		return fmt.Errorf("token cannot contain CR, LF, or Tab")
	}
	return nil
}

// validateName validates xs:Name
func validateName(value string) error {
	if value == "" {
		return fmt.Errorf("Name cannot be empty")
	}

	// iterate through runes (not bytes) to handle UTF-8 properly
	runes := []rune(value)
	if !isNameStartChar(runes[0]) {
		return fmt.Errorf("invalid Name start character: %c", runes[0])
	}

	for _, r := range runes[1:] {
		if !isNameChar(r) {
			return fmt.Errorf("invalid Name character: %c", r)
		}
	}

	return nil
}

// validateNCName validates xs:NCName (Name without colons)
func validateNCName(value string) error {
	if value == "" {
		return fmt.Errorf("NCName cannot be empty")
	}

	if strings.Contains(value, ":") {
		return fmt.Errorf("NCName cannot contain colons")
	}

	// iterate through runes (not bytes) to handle UTF-8 properly
	runes := []rune(value)
	if !isNameStartChar(runes[0]) {
		return fmt.Errorf("invalid NCName start character: %c", runes[0])
	}

	for _, r := range runes[1:] {
		if !isNameChar(r) {
			return fmt.Errorf("invalid NCName character: %c", r)
		}
	}

	return nil
}

// isNameStartChar checks if a rune is a valid Name start character
func isNameStartChar(r rune) bool {
	return r == ':' || r == '_' ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 0xC0 && r <= 0xD6) ||
		(r >= 0xD8 && r <= 0xF6) ||
		(r >= 0xF8 && r <= 0x2FF) ||
		(r >= 0x370 && r <= 0x37D) ||
		(r >= 0x37F && r <= 0x1FFF) ||
		(r >= 0x200C && r <= 0x200D) ||
		(r >= 0x2070 && r <= 0x218F) ||
		(r >= 0x2C00 && r <= 0x2FEF) ||
		(r >= 0x3001 && r <= 0xD7FF) ||
		(r >= 0xF900 && r <= 0xFDCF) ||
		(r >= 0xFDF0 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0xEFFFF)
}

// isNameChar checks if a rune is a valid Name character
func isNameChar(r rune) bool {
	return isNameStartChar(r) ||
		r == '-' || r == '.' ||
		(r >= '0' && r <= '9') ||
		r == 0xB7 ||
		(r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x203F && r <= 0x2040)
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
	parts := splitXMLWhitespaceFields(value)
	if len(parts) == 0 {
		return fmt.Errorf("IDREFS must have at least one item")
	}

	for _, part := range parts {
		if err := validateIDREF(part); err != nil {
			return fmt.Errorf("invalid IDREFS: %w", err)
		}
	}

	return nil
}

// validateENTITY validates xs:ENTITY (same as NCName)
func validateENTITY(value string) error {
	return validateNCName(value)
}

// validateENTITIES validates xs:ENTITIES (space-separated list of ENTITYs)
func validateENTITIES(value string) error {
	parts := splitXMLWhitespaceFields(value)
	if len(parts) == 0 {
		return fmt.Errorf("ENTITIES must have at least one item")
	}

	for _, part := range parts {
		if err := validateENTITY(part); err != nil {
			return fmt.Errorf("invalid ENTITIES: %w", err)
		}
	}

	return nil
}

// validateNMTOKEN validates xs:NMTOKEN
// NMTOKEN is any string matching NameChar+
func validateNMTOKEN(value string) error {
	if value == "" {
		return fmt.Errorf("NMTOKEN cannot be empty")
	}

	for _, r := range value {
		if !isNameChar(r) {
			return fmt.Errorf("invalid NMTOKEN character: %c", r)
		}
	}

	return nil
}

// validateNMTOKENS validates xs:NMTOKENS (space-separated list of NMTOKENs)
func validateNMTOKENS(value string) error {
	parts := splitXMLWhitespaceFields(value)
	if len(parts) == 0 {
		return fmt.Errorf("NMTOKENS must have at least one item")
	}

	for _, part := range parts {
		if err := validateNMTOKEN(part); err != nil {
			return fmt.Errorf("invalid NMTOKENS: %w", err)
		}
	}

	return nil
}

// validateLanguage validates xs:language
// Format: language identifier per RFC 3066
// Pattern: [a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*

func validateLanguage(value string) error {
	// per XSD spec, language pattern is [a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*
	// empty string is not valid (pattern requires at least one character)
	if !languagePattern.MatchString(value) {
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

func splitTimezone(value string) (string, string) {
	if value == "" {
		return value, ""
	}
	last := value[len(value)-1]
	if last == 'Z' {
		return value[:len(value)-1], "Z"
	}
	if len(value) >= 6 {
		tz := value[len(value)-6:]
		if (tz[0] == '+' || tz[0] == '-') && tz[3] == ':' {
			return value[:len(value)-6], tz
		}
	}
	return value, ""
}

func parseFixedDigits(value string, start, length int) (int, bool) {
	if start < 0 || length <= 0 || start+length > len(value) {
		return 0, false
	}
	n := 0
	for i := range length {
		ch := value[start+i]
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int(ch-'0')
	}
	return n, true
}

func parseDateParts(value string) (int, int, int, bool) {
	if len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return 0, 0, 0, false
	}
	year, ok := parseFixedDigits(value, 0, 4)
	if !ok {
		return 0, 0, 0, false
	}
	month, ok := parseFixedDigits(value, 5, 2)
	if !ok {
		return 0, 0, 0, false
	}
	day, ok := parseFixedDigits(value, 8, 2)
	if !ok {
		return 0, 0, 0, false
	}
	return year, month, day, true
}

func parseTimeParts(value string) (int, int, int, int, bool) {
	if len(value) < 8 || value[2] != ':' || value[5] != ':' {
		return 0, 0, 0, 0, false
	}
	hour, ok := parseFixedDigits(value, 0, 2)
	if !ok {
		return 0, 0, 0, 0, false
	}
	minute, ok := parseFixedDigits(value, 3, 2)
	if !ok {
		return 0, 0, 0, 0, false
	}
	second, ok := parseFixedDigits(value, 6, 2)
	if !ok {
		return 0, 0, 0, 0, false
	}
	if len(value) == 8 {
		return hour, minute, second, 0, true
	}
	if value[8] != '.' || len(value) == 9 {
		return 0, 0, 0, 0, false
	}
	for i := 9; i < len(value); i++ {
		ch := value[i]
		if ch < '0' || ch > '9' {
			return 0, 0, 0, 0, false
		}
	}
	fractionLength := len(value) - 9
	return hour, minute, second, fractionLength, true
}

func appendTimezoneSuffix(value, tz string) string {
	switch tz {
	case "Z":
		return value + "Z"
	case "":
		return value
	default:
		return value + tz
	}
}

func applyTimezoneLayout(layout, tz string) string {
	switch tz {
	case "Z":
		return layout + "Z"
	case "":
		return layout
	default:
		return layout + "-07:00"
	}
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

func validateTimezoneOffset(tz string) error {
	if tz == "" || tz == "Z" {
		return nil
	}
	if len(tz) != 6 {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	if tz[0] != '+' && tz[0] != '-' {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	if tz[3] != ':' {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	hour, err := strconv.Atoi(tz[1:3])
	if err != nil {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	minute, err := strconv.Atoi(tz[4:6])
	if err != nil {
		return fmt.Errorf("invalid timezone format: %s", tz)
	}
	if hour < 0 || hour > 14 || minute < 0 || minute > 59 {
		return fmt.Errorf("invalid timezone offset: %s", tz)
	}
	if hour == 14 && minute != 0 {
		return fmt.Errorf("invalid timezone offset: %s", tz)
	}
	return nil
}

func isValidDate(year, month, day int) bool {
	if day < 1 || day > 31 {
		return false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Year() == year && int(t.Month()) == month && t.Day() == day
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
	if value == "" {
		return nil // empty URI is valid
	}

	// reject control characters and disallowed ASCII characters.
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("anyURI contains control characters")
		}
		switch r {
		case '\t', '\n', '\r', '\\', '{', '}', '|', '^', '`':
			return fmt.Errorf("anyURI contains invalid characters")
		}
	}

	// validate percent-encoding: every '%' must be followed by two hex digits.
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			continue
		}
		if i+2 >= len(value) || !isHexDigit(value[i+1]) || !isHexDigit(value[i+2]) {
			return fmt.Errorf("anyURI contains invalid percent-encoding")
		}
		i += 2
	}

	// validate scheme if present (scheme must precede any '/', '?', or '#').
	if idx := strings.Index(value, ":"); idx != -1 {
		delimiter := strings.IndexAny(value, "/?#")
		if delimiter == -1 || idx < delimiter {
			if idx == 0 {
				return fmt.Errorf("anyURI scheme cannot be empty")
			}
			scheme := value[:idx]
			if !uriSchemePattern.MatchString(scheme) {
				return fmt.Errorf("anyURI has invalid scheme: %s", scheme)
			}
		}
	}

	return nil
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')
}

// validateQName validates xs:QName
// Format: NCName (possibly qualified with a prefix)
func validateQName(value string) error {
	if value == "" {
		return fmt.Errorf("QName cannot be empty")
	}

	// check for prefix (local:name format)
	colon := strings.IndexByte(value, ':')
	if colon == -1 {
		if err := validateNCName(value); err != nil {
			return fmt.Errorf("invalid QName part '%s': %w", value, err)
		}
		return nil
	}
	if colon == 0 || colon == len(value)-1 {
		return fmt.Errorf("QName part cannot be empty")
	}
	if strings.IndexByte(value[colon+1:], ':') != -1 {
		return fmt.Errorf("QName can have at most one colon")
	}
	prefix := value[:colon]
	local := value[colon+1:]
	if prefix == "xmlns" {
		return fmt.Errorf("QName cannot use reserved prefix 'xmlns'")
	}
	if err := validateNCName(prefix); err != nil {
		return fmt.Errorf("invalid QName part '%s': %w", prefix, err)
	}
	if err := validateNCName(local); err != nil {
		return fmt.Errorf("invalid QName part '%s': %w", local, err)
	}

	return nil
}

// validateNOTATION validates xs:NOTATION
// Format: QName, but must reference a notation declared in the schema
// We can only validate the QName format here; notation reference validation
// must be done at the schema level
func validateNOTATION(value string) error {
	// format is the same as QName
	return validateQName(value)
}
