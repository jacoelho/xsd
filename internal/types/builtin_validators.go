package types

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
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
var decimalPattern = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)$`)

func validateDecimal(value string) error {
	if !decimalPattern.MatchString(value) {
		return fmt.Errorf("invalid decimal: %s", value)
	}
	return nil
}

// validateFloat validates xs:float
// Per XSD 1.0 spec (3.2.4), the lexical space excludes "+INF", so apply
// an explicit lexical check before ParseFloat (which would otherwise accept it).
var floatPattern = regexp.MustCompile(`^[+-]?((\d+(\.\d*)?)|(\.\d+))([eE][+-]?\d+)?$`)

func validateFloat(value string) error {
	if value == "INF" || value == "-INF" || value == "NaN" {
		return nil
	}
	if !floatPattern.MatchString(value) {
		return fmt.Errorf("invalid float: %s", value)
	}
	if _, err := strconv.ParseFloat(value, 32); err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return nil
		}
		return fmt.Errorf("invalid float: %s", value)
	}
	return nil
}

// validateDouble validates xs:double
func validateDouble(value string) error {
	if value == "INF" || value == "-INF" || value == "NaN" {
		return nil
	}
	if !floatPattern.MatchString(value) {
		return fmt.Errorf("invalid double: %s", value)
	}
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return nil
		}
		return fmt.Errorf("invalid double: %s", value)
	}
	return nil
}

// validateInteger validates xs:integer
var integerPattern = regexp.MustCompile(`^[+-]?\d+$`)

func validateInteger(value string) error {
	if !integerPattern.MatchString(value) {
		return fmt.Errorf("invalid integer: %s", value)
	}
	return nil
}

// validateLong validates xs:long
func validateLong(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	_, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid long: %s", value)
	}
	return nil
}

// validateInt validates xs:int
func validateInt(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid int: %s", value)
	}
	if n < math.MinInt32 || n > math.MaxInt32 {
		return fmt.Errorf("int out of range: %s", value)
	}
	return nil
}

// validateShort validates xs:short
func validateShort(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid short: %s", value)
	}
	if n < math.MinInt16 || n > math.MaxInt16 {
		return fmt.Errorf("short out of range: %s", value)
	}
	return nil
}

// validateByte validates xs:byte
func validateByte(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid byte: %s", value)
	}
	if n < math.MinInt8 || n > math.MaxInt8 {
		return fmt.Errorf("byte out of range: %s", value)
	}
	return nil
}

// validateNonNegativeInteger validates xs:nonNegativeInteger
func validateNonNegativeInteger(value string) error {
	if err := validateInteger(value); err != nil {
		return err
	}
	if strings.HasPrefix(value, "-") && value != "-0" {
		return fmt.Errorf("nonNegativeInteger must be >= 0: %s", value)
	}
	return nil
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
	if err := validateNonNegativeInteger(value); err != nil {
		return err
	}
	_, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid unsignedLong: %s", value)
	}
	return nil
}

// validateUnsignedInt validates xs:unsignedInt
func validateUnsignedInt(value string) error {
	if err := validateNonNegativeInteger(value); err != nil {
		return err
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid unsignedInt: %s", value)
	}
	if n > math.MaxUint32 {
		return fmt.Errorf("unsignedInt out of range: %s", value)
	}
	return nil
}

// validateUnsignedShort validates xs:unsignedShort
func validateUnsignedShort(value string) error {
	if err := validateNonNegativeInteger(value); err != nil {
		return err
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid unsignedShort: %s", value)
	}
	if n > math.MaxUint16 {
		return fmt.Errorf("unsignedShort out of range: %s", value)
	}
	return nil
}

// validateUnsignedByte validates xs:unsignedByte
func validateUnsignedByte(value string) error {
	if err := validateNonNegativeInteger(value); err != nil {
		return err
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid unsignedByte: %s", value)
	}
	if n > math.MaxUint8 {
		return fmt.Errorf("unsignedByte out of range: %s", value)
	}
	return nil
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
	if len(value) == 0 {
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
	if len(value) == 0 {
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

// IsValidNCName returns true if the string is a valid NCName (non-colonized name)
// NCName must not be empty, must not contain colons, must start with a NameStartChar,
// and subsequent characters must be NameChars (XML 1.0 spec)
func IsValidNCName(s string) bool {
	return validateNCName(s) == nil
}

// IsValidQName returns true if the string is a valid QName.
// QName must not be empty, may contain at most one colon, and each part must be a valid NCName.
func IsValidQName(s string) bool {
	return validateQName(s) == nil
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
	if len(value) == 0 {
		return nil // empty is valid
	}

	parts := strings.FieldsSeq(value)
	for part := range parts {
		if err := validateIDREF(part); err != nil {
			return fmt.Errorf("invalid IDREFS: %v", err)
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
	if len(value) == 0 {
		return nil // empty is valid
	}

	parts := strings.FieldsSeq(value)
	for part := range parts {
		if err := validateENTITY(part); err != nil {
			return fmt.Errorf("invalid ENTITIES: %v", err)
		}
	}

	return nil
}

// validateNMTOKEN validates xs:NMTOKEN
// NMTOKEN is any string matching NameChar+
func validateNMTOKEN(value string) error {
	if len(value) == 0 {
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
	if len(value) == 0 {
		return nil // empty is valid
	}

	parts := strings.FieldsSeq(value)
	for part := range parts {
		if err := validateNMTOKEN(part); err != nil {
			return fmt.Errorf("invalid NMTOKENS: %v", err)
		}
	}

	return nil
}

// validateLanguage validates xs:language
// Format: language identifier per RFC 3066
// Pattern: [a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*
var languagePattern = regexp.MustCompile(`^[a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*$`)

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
	// basic format check
	durationPattern := regexp.MustCompile(`^-?P(\d+Y)?(\d+M)?(\d+D)?(T(\d+H)?(\d+M)?(\d+(\.\d+)?S)?)?$`)
	if !durationPattern.MatchString(value) {
		return fmt.Errorf("invalid duration format: %s", value)
	}
	if strings.Contains(value, "T") {
		timeComponentPattern := regexp.MustCompile(`T(\d+H|\d+M|\d+(\.\d+)?S)`)
		if !timeComponentPattern.MatchString(value) {
			return fmt.Errorf("time designator present but no time components specified")
		}
	}

	// try to parse with Go's time.ParseDuration (simplified)
	// for full compliance, we'd need a custom parser
	_, err := parseDuration(value)
	return err
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}

	negative := s[0] == '-'
	if negative {
		s = s[1:]
	}

	if s[0] != 'P' {
		return 0, fmt.Errorf("duration must start with P")
	}
	s = s[1:]
	hasTimeDesignator := strings.Contains(s, "T")

	var days, hours, minutes int
	var seconds float64

	hasDateComponent := false
	hasTimeComponent := false

	parts := strings.Split(s, "T")
	datePart := parts[0]
	timePart := ""
	if len(parts) > 1 {
		timePart = parts[1]
	}

	// parse date part (years, months, days)
	datePattern := regexp.MustCompile(`(\d+)Y|(\d+)M|(\d+)D`)
	matches := datePattern.FindAllStringSubmatch(datePart, -1)
	for _, match := range matches {
		if match[1] != "" {
			// years component found (value may be 0)
			hasDateComponent = true
		}
		if match[2] != "" {
			// months component found (value may be 0)
			hasDateComponent = true
		}
		if match[3] != "" {
			days, _ = strconv.Atoi(match[3])
			hasDateComponent = true
		}
	}

	// parse time part (hours, minutes, seconds)
	if timePart != "" {
		timePattern := regexp.MustCompile(`(\d+)H|(\d+)M|(\d+(\.\d+)?)S`)
		matches := timePattern.FindAllStringSubmatch(timePart, -1)
		for _, match := range matches {
			if match[1] != "" {
				hours, _ = strconv.Atoi(match[1])
				hasTimeComponent = true
			}
			if match[2] != "" {
				minutes, _ = strconv.Atoi(match[2])
				hasTimeComponent = true
			}
			if match[3] != "" {
				seconds, _ = strconv.ParseFloat(match[3], 64)
				hasTimeComponent = true
			}
		}
	}

	// duration must have at least one component specified
	// "P" alone is invalid, but "P0Y" is valid (0 years is a valid component)
	if !hasDateComponent && !hasTimeComponent {
		return 0, fmt.Errorf("duration must have at least one component")
	}

	// if T is present but no time components, that's invalid
	if hasTimeDesignator && !hasTimeComponent {
		return 0, fmt.Errorf("time designator present but no time components specified")
	}

	// in real validation, we'd need to handle years/months specially
	dur := time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second

	if negative {
		dur = -dur
	}

	return dur, nil
}

// validateDateTime validates xs:dateTime
// Format: CCYY-MM-DDThh:mm:ss[.sss][Z|(+|-)hh:mm]
func validateDateTime(value string) error {
	match := regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(\.(\d+))?(Z|([+-]\d{2}:\d{2}))?$`).FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("invalid dateTime format: %s", value)
	}

	year, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	hour, _ := strconv.Atoi(match[4])
	minute, _ := strconv.Atoi(match[5])
	second, _ := strconv.Atoi(match[6])
	fraction := match[8]
	tz := match[9]

	if year < 1 || year > 9999 {
		return fmt.Errorf("invalid dateTime: year %04d is not valid in XSD 1.0", year)
	}
	if month < 1 || month > 12 {
		return fmt.Errorf("invalid dateTime: month out of range")
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return fmt.Errorf("invalid dateTime: time out of range")
	}
	if fraction != "" && len(fraction) > 9 {
		return fmt.Errorf("invalid dateTime: fractional seconds too long")
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return err
	}
	if !isValidDate(year, month, day) {
		return fmt.Errorf("invalid dateTime: date out of range")
	}

	layout := "2006-01-02T15:04:05"
	if fraction != "" {
		layout += "." + strings.Repeat("0", len(fraction))
	}
	if tz == "Z" {
		layout += "Z"
	} else if tz != "" {
		layout += "-07:00"
	}
	if _, err := time.Parse(layout, value); err != nil {
		return fmt.Errorf("invalid dateTime format: %s", value)
	}

	return nil
}

// validateDate validates xs:date
// Format: CCYY-MM-DD[Z|(+|-)hh:mm]
func validateDate(value string) error {
	match := regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})(Z|([+-]\d{2}:\d{2}))?$`).FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("invalid date format: %s", value)
	}

	year, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	tz := match[4]

	if year < 1 || year > 9999 {
		return fmt.Errorf("invalid date: year %04d is not valid in XSD 1.0", year)
	}
	if month < 1 || month > 12 || !isValidDate(year, month, day) {
		return fmt.Errorf("invalid date: date out of range")
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return err
	}

	layout := "2006-01-02"
	if tz == "Z" {
		layout += "Z"
	} else if tz != "" {
		layout += "-07:00"
	}
	if _, err := time.Parse(layout, value); err != nil {
		return fmt.Errorf("invalid date format: %s", value)
	}

	return nil
}

// validateTime validates xs:time
// Format: hh:mm:ss[.sss][Z|(+|-)hh:mm]
func validateTime(value string) error {
	match := regexp.MustCompile(`^(\d{2}):(\d{2}):(\d{2})(\.(\d+))?(Z|([+-]\d{2}:\d{2}))?$`).FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("invalid time format: %s", value)
	}

	hour, _ := strconv.Atoi(match[1])
	minute, _ := strconv.Atoi(match[2])
	second, _ := strconv.Atoi(match[3])
	fraction := match[5]
	tz := match[6]

	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return fmt.Errorf("invalid time: time out of range")
	}
	if fraction != "" && len(fraction) > 9 {
		return fmt.Errorf("invalid time: fractional seconds too long")
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return err
	}

	layout := "15:04:05"
	if fraction != "" {
		layout += "." + strings.Repeat("0", len(fraction))
	}
	if tz == "Z" {
		layout += "Z"
	} else if tz != "" {
		layout += "-07:00"
	}
	if _, err := time.Parse(layout, value); err != nil {
		return fmt.Errorf("invalid time format: %s", value)
	}

	return nil
}

// validateGYear validates xs:gYear
// Format: CCYY[Z|(+|-)hh:mm]
// Note: Only years 0001-9999 are supported (Go time.Parse limitation + XSD 1.0 no year 0)
func validateGYear(value string) error {
	match := regexp.MustCompile(`^(\d{4})(Z|([+-]\d{2}:\d{2}))?$`).FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("invalid gYear format: %s", value)
	}

	year, _ := strconv.Atoi(match[1])
	tz := match[2]
	if year < 1 || year > 9999 {
		return fmt.Errorf("invalid gYear: year %04d is not valid in XSD 1.0", year)
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return err
	}

	layout := "2006"
	if tz == "Z" {
		layout += "Z"
	} else if tz != "" {
		layout += "-07:00"
	}
	if _, err := time.Parse(layout, value); err != nil {
		return fmt.Errorf("invalid gYear format: %s", value)
	}

	return nil
}

// validateGYearMonth validates xs:gYearMonth
// Format: CCYY-MM[Z|(+|-)hh:mm]
// Note: Only years 0001-9999 are supported (Go time.Parse limitation + XSD 1.0 no year 0)
func validateGYearMonth(value string) error {
	match := regexp.MustCompile(`^(\d{4})-(\d{2})(Z|([+-]\d{2}:\d{2}))?$`).FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("invalid gYearMonth format: %s", value)
	}

	year, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	tz := match[3]
	if year < 1 || year > 9999 {
		return fmt.Errorf("invalid gYearMonth: year %04d is not valid in XSD 1.0", year)
	}
	if month < 1 || month > 12 {
		return fmt.Errorf("invalid gYearMonth: month out of range")
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return err
	}

	layout := "2006-01"
	if tz == "Z" {
		layout += "Z"
	} else if tz != "" {
		layout += "-07:00"
	}
	if _, err := time.Parse(layout, value); err != nil {
		return fmt.Errorf("invalid gYearMonth format: %s", value)
	}

	return nil
}

// validateGMonth validates xs:gMonth
// Format: --MM[Z|(+|-)hh:mm]
func validateGMonth(value string) error {
	match := regexp.MustCompile(`^--(\d{2})(Z|([+-]\d{2}:\d{2}))?$`).FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("invalid gMonth format: %s", value)
	}
	month, _ := strconv.Atoi(match[1])
	tz := match[2]
	if month < 1 || month > 12 {
		return fmt.Errorf("invalid month value: %s", match[1])
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return err
	}

	layout := "2006-01"
	testValue := "2000-" + value[2:]
	if tz == "Z" {
		layout += "Z"
	} else if tz != "" {
		layout += "-07:00"
	}
	if _, err := time.Parse(layout, testValue); err != nil {
		return fmt.Errorf("invalid gMonth format: %s", value)
	}

	return nil
}

// validateGMonthDay validates xs:gMonthDay
// Format: --MM-DD[Z|(+|-)hh:mm]
func validateGMonthDay(value string) error {
	match := regexp.MustCompile(`^--(\d{2})-(\d{2})(Z|([+-]\d{2}:\d{2}))?$`).FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("invalid gMonthDay format: %s", value)
	}
	month, _ := strconv.Atoi(match[1])
	day, _ := strconv.Atoi(match[2])
	tz := match[3]
	if month < 1 || month > 12 {
		return fmt.Errorf("invalid gMonthDay: month out of range")
	}
	if !isValidDate(2000, month, day) {
		return fmt.Errorf("invalid gMonthDay: day out of range")
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return err
	}

	layout := "2006-01-02"
	testValue := fmt.Sprintf("2000-%02d-%02d", month, day)
	if tz == "Z" {
		layout += "Z"
		testValue += "Z"
	} else if tz != "" {
		layout += "-07:00"
		testValue += tz
	}
	if _, err := time.Parse(layout, testValue); err != nil {
		return fmt.Errorf("invalid gMonthDay format: %s", value)
	}

	return nil
}

// validateGDay validates xs:gDay
// Format: ---DD[Z|(+|-)hh:mm]
func validateGDay(value string) error {
	match := regexp.MustCompile(`^---(\d{2})(Z|([+-]\d{2}:\d{2}))?$`).FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("invalid gDay format: %s", value)
	}
	day, _ := strconv.Atoi(match[1])
	tz := match[2]
	if day < 1 || day > 31 {
		return fmt.Errorf("invalid gDay: day out of range")
	}
	if err := validateTimezoneOffset(tz); err != nil {
		return err
	}

	layout := "2006-01-02"
	testValue := fmt.Sprintf("2000-01-%02d", day)
	if tz == "Z" {
		layout += "Z"
		testValue += "Z"
	} else if tz != "" {
		layout += "-07:00"
		testValue += tz
	}
	if _, err := time.Parse(layout, testValue); err != nil {
		return fmt.Errorf("invalid gDay format: %s", value)
	}

	return nil
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
	if len(value) == 0 {
		return nil // empty is valid
	}

	// must be even number of characters
	if len(value)%2 != 0 {
		return fmt.Errorf("hexBinary must have even number of characters")
	}

	// must contain only hex digits
	hexPattern := regexp.MustCompile(`^[0-9A-Fa-f]+$`)
	if !hexPattern.MatchString(value) {
		return fmt.Errorf("hexBinary must contain only hexadecimal digits (0-9, A-F, a-f)")
	}

	// try to decode to verify it's valid hex
	_, err := hex.DecodeString(value)
	if err != nil {
		return fmt.Errorf("invalid hexBinary: %v", err)
	}

	return nil
}

// validateBase64Binary validates xs:base64Binary
// Format: base64 encoded string
func validateBase64Binary(value string) error {
	if len(value) == 0 {
		return nil // empty is valid
	}

	// remove whitespace (base64 can contain whitespace in XML)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\t", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")

	// try to decode to verify it's valid base64 (strict padding/charset).
	if _, err := base64.StdEncoding.Strict().DecodeString(value); err != nil {
		return fmt.Errorf("invalid base64Binary: %v", err)
	}

	return nil
}

// validateAnyURI validates xs:anyURI
// Format: URI/IRI reference (RFC 2396 and RFC 2732)
func validateAnyURI(value string) error {
	if len(value) == 0 {
		return nil // empty URI is valid
	}

	// reject control characters and whitespace.
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("anyURI contains control characters")
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return fmt.Errorf("anyURI contains whitespace")
		}
	}

	// backslashes and a few prohibited delimiters are not valid in URI references.
	if strings.ContainsAny(value, "\\{}|^`") {
		return fmt.Errorf("anyURI contains invalid characters")
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
			schemePattern := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)
			if !schemePattern.MatchString(scheme) {
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
	if len(value) == 0 {
		return fmt.Errorf("QName cannot be empty")
	}

	// check for prefix (local:name format)
	parts := strings.Split(value, ":")
	if len(parts) > 2 {
		return fmt.Errorf("QName can have at most one colon")
	}
	if len(parts) == 2 && parts[0] == "xmlns" {
		return fmt.Errorf("QName cannot use reserved prefix 'xmlns'")
	}

	for _, part := range parts {
		if len(part) == 0 {
			return fmt.Errorf("QName part cannot be empty")
		}
		if err := validateNCName(part); err != nil {
			return fmt.Errorf("invalid QName part '%s': %v", part, err)
		}
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