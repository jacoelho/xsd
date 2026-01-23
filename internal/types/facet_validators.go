package types

import (
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// getXSDTypeName returns a user-friendly XSD type name for error messages
func getXSDTypeName(value TypedValue) string {
	if value == nil {
		return "unknown"
	}
	typ := value.Type()
	if typ == nil {
		return "unknown"
	}
	return typ.Name().Local
}

// parseTemporalValue parses a lexical value according to its primitive type name.
func parseTemporalValue(primitiveName, lexical string) (time.Time, error) {
	switch primitiveName {
	case "dateTime":
		return ParseDateTime(lexical)
	case "date":
		return ParseDate(lexical)
	case "time":
		return ParseTime(lexical)
	case "gYear":
		return ParseGYear(lexical)
	case "gYearMonth":
		return ParseGYearMonth(lexical)
	case "gMonth":
		return ParseGMonth(lexical)
	case "gMonthDay":
		return ParseGMonthDay(lexical)
	case "gDay":
		return ParseGDay(lexical)
	default:
		return time.Time{}, fmt.Errorf("unsupported date/time type: %s", primitiveName)
	}
}

// durationToXSD converts a time.Duration to XSDDuration.
func durationToXSD(d time.Duration) XSDDuration {
	negative := d < 0
	if negative {
		d = -d
	}
	hours := int(d / time.Hour)
	d %= time.Hour
	minutes := int(d / time.Minute)
	d %= time.Minute
	seconds := float64(d) / float64(time.Second)
	return XSDDuration{
		Negative: negative,
		Years:    0,
		Months:   0,
		Days:     0,
		Hours:    hours,
		Minutes:  minutes,
		Seconds:  seconds,
	}
}

// integerDerivedTypeNames is a lookup table for types derived from xs:integer.
// Package-level var avoids repeated allocation.
var integerDerivedTypeNames = map[string]bool{
	"integer":            true,
	"long":               true,
	"int":                true,
	"short":              true,
	"byte":               true,
	"unsignedLong":       true,
	"unsignedInt":        true,
	"unsignedShort":      true,
	"unsignedByte":       true,
	"nonNegativeInteger": true,
	"positiveInteger":    true,
	"negativeInteger":    true,
	"nonPositiveInteger": true,
}

// isIntegerDerivedType checks if t derives from xs:integer by walking the derivation chain.
func isIntegerDerivedType(t Type) bool {
	if t == nil {
		return false
	}

	typeName := t.Name().Local

	// check if the type name itself is integer-derived
	if integerDerivedTypeNames[typeName] {
		return true
	}

	// for SimpleType, walk the derivation chain
	if st, ok := t.(*SimpleType); ok {
		current := st.ResolvedBase
		for current != nil {
			// use Name() interface method instead of type assertions
			currentName := current.Name().Local
			if integerDerivedTypeNames[currentName] {
				return true
			}
			// continue walking the chain if it's a SimpleType
			if currentST, ok := current.(*SimpleType); ok {
				current = currentST.ResolvedBase
			} else {
				// BuiltinType or other type - stop here
				break
			}
		}
	}

	return false
}

// extractComparableValue extracts a ComparableValue from a TypedValue.
// This is the shared logic used by all range facet validators.
func extractComparableValue(value TypedValue, baseType Type) (ComparableValue, error) {
	if value == nil {
		return nil, fmt.Errorf("cannot compare nil value")
	}

	native := value.Native()
	typ := value.Type()
	if typ == nil {
		typ = baseType
	}

	// try to convert native to ComparableValue directly
	if compVal, ok := native.(ComparableValue); ok {
		return compVal, nil
	}
	if unwrappable, ok := native.(Unwrappable); ok {
		native = unwrappable.Unwrap()
	}

	switch v := native.(type) {
	case *big.Rat:
		return ComparableBigRat{Value: v, Typ: typ}, nil
	case *big.Int:
		return ComparableBigInt{Value: v, Typ: typ}, nil
	case time.Time:
		hasTZ := HasTimezone(value.Lexical())
		return ComparableTime{Value: v, Typ: typ, HasTimezone: hasTZ}, nil
	case time.Duration:
		xsdDur := durationToXSD(v)
		return ComparableXSDDuration{Value: xsdDur, Typ: typ}, nil
	case float64:
		return ComparableFloat64{Value: v, Typ: typ}, nil
	case float32:
		return ComparableFloat32{Value: v, Typ: typ}, nil
	case string:
		return parseStringToComparableValue(value, v, typ)
	}

	// all conversion attempts failed
	xsdTypeName := getXSDTypeName(value)
	return nil, fmt.Errorf("value type %s cannot be compared with facet value", xsdTypeName)
}

// parseStringToComparableValue parses a string value according to the TypedValue's type
// and converts it to the appropriate ComparableValue.
func parseStringToComparableValue(value TypedValue, lexical string, typ Type) (ComparableValue, error) {
	if typ == nil {
		typ = value.Type()
	}
	if typ == nil {
		return nil, fmt.Errorf("cannot parse string: value has no type")
	}

	typeName := typ.Name().Local

	// check if the actual type is integer (even though primitive is decimal)
	if typeName == "integer" {
		intVal, err := ParseInteger(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse integer: %w", err)
		}
		return ComparableBigInt{Value: intVal, Typ: typ}, nil
	}

	var primitiveType Type
	switch t := typ.(type) {
	case *SimpleType:
		primitiveType = t.PrimitiveType()
	case *BuiltinType:
		primitiveType = t.PrimitiveType()
	default:
		return nil, fmt.Errorf("cannot parse string: unsupported type %T", typ)
	}

	if primitiveType == nil {
		return nil, fmt.Errorf("cannot parse string: cannot determine primitive type")
	}

	primitiveName := primitiveType.Name().Local

	// check if type is integer-derived
	isIntegerDerived := isIntegerDerivedType(typ)

	switch primitiveName {
	case "decimal":
		// if type is integer-derived, parse as integer
		if isIntegerDerived {
			intVal, err := ParseInteger(lexical)
			if err != nil {
				return nil, fmt.Errorf("cannot parse integer: %w", err)
			}
			return ComparableBigInt{Value: intVal, Typ: typ}, nil
		}
		rat, err := ParseDecimal(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse decimal: %w", err)
		}
		return ComparableBigRat{Value: rat, Typ: typ}, nil

	case "dateTime", "date", "time", "gYear", "gYearMonth", "gMonth", "gMonthDay", "gDay":
		timeVal, err := parseTemporalValue(primitiveName, lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse date/time: %w", err)
		}
		return ComparableTime{Value: timeVal, Typ: typ, HasTimezone: HasTimezone(lexical)}, nil

	case "float":
		floatVal, err := ParseFloat(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse float: %w", err)
		}
		return ComparableFloat32{Value: floatVal, Typ: typ}, nil

	case "double":
		doubleVal, err := ParseDouble(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse double: %w", err)
		}
		return ComparableFloat64{Value: doubleVal, Typ: typ}, nil

	case "duration":
		xsdDur, err := ParseXSDDuration(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse duration: %w", err)
		}
		return ComparableXSDDuration{Value: xsdDur, Typ: typ}, nil

	default:
		return nil, fmt.Errorf("cannot parse string: unsupported primitive type %s for Comparable conversion", primitiveName)
	}
}

// RangeFacet is a unified implementation for all range facets.
type RangeFacet struct {
	// Facet name (minInclusive, maxInclusive, etc.)
	name string
	// Keep lexical for schema/error messages
	lexical string
	// Comparable value
	value ComparableValue
	// Comparison function: returns true if validation passes
	cmpFunc func(cmp int) bool
	// Error operator string (">=", "<=", ">", "<")
	errOp string
}

// Name returns the facet name
func (r *RangeFacet) Name() string {
	return r.name
}

// GetLexical returns the lexical value (implements LexicalFacet)
func (r *RangeFacet) GetLexical() string {
	return r.lexical
}

// Validate validates a TypedValue using ComparableValue comparison
func (r *RangeFacet) Validate(value TypedValue, baseType Type) error {
	compVal, err := extractComparableValue(value, baseType)
	if err != nil {
		return fmt.Errorf("%s: %w", r.name, err)
	}

	// compare using ComparableValue interface
	cmp, err := compVal.Compare(r.value)
	if err != nil {
		if errors.Is(err, errIndeterminateDurationComparison) || errors.Is(err, errIndeterminateTimeComparison) {
			return fmt.Errorf("value %s must be %s %s", value.String(), r.errOp, r.lexical)
		}
		return fmt.Errorf("%s: cannot compare values: %w", r.name, err)
	}

	if !r.cmpFunc(cmp) {
		return fmt.Errorf("value %s must be %s %s", value.String(), r.errOp, r.lexical)
	}

	return nil
}

// isQNameOrNotationType checks if a type is QName, NOTATION, or restricts either.
// Per XSD 1.0 errata, length facets should be ignored for QName and NOTATION types
// because their value space length depends on namespace context, not lexical form.
// This is a wrapper around IsQNameOrNotationType for consistency with existing code.
func isQNameOrNotationType(t Type) bool {
	return IsQNameOrNotationType(t)
}

// Length represents a length facet
type Length struct {
	Value int
}

// Name returns the facet name
func (l *Length) Name() string {
	return "length"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (l *Length) GetIntValue() int {
	return l.Value
}

// Validate checks if the value has the exact length (unified Facet interface)
func (l *Length) Validate(value TypedValue, baseType Type) error {
	return l.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value has the exact length.
func (l *Length) ValidateLexical(lexical string, baseType Type) error {
	// per XSD 1.0 errata, length facets are ignored for QName and NOTATION types
	if isQNameOrNotationType(baseType) {
		return nil
	}
	length := getLength(lexical, baseType)
	if length != l.Value {
		return fmt.Errorf("length must be %d, got %d", l.Value, length)
	}
	return nil
}

// MinLength represents a minLength facet
type MinLength struct {
	Value int
}

// Name returns the facet name
func (m *MinLength) Name() string {
	return "minLength"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (m *MinLength) GetIntValue() int {
	return m.Value
}

// Validate checks if the value meets minimum length (unified Facet interface)
func (m *MinLength) Validate(value TypedValue, baseType Type) error {
	return m.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value meets minimum length.
func (m *MinLength) ValidateLexical(lexical string, baseType Type) error {
	// per XSD 1.0 errata, length facets are ignored for QName and NOTATION types
	if isQNameOrNotationType(baseType) {
		return nil
	}
	length := getLength(lexical, baseType)
	if length < m.Value {
		return fmt.Errorf("length must be at least %d, got %d", m.Value, length)
	}
	return nil
}

// MaxLength represents a maxLength facet
type MaxLength struct {
	Value int
}

// Name returns the facet name
func (m *MaxLength) Name() string {
	return "maxLength"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (m *MaxLength) GetIntValue() int {
	return m.Value
}

// Validate checks if the value meets maximum length (unified Facet interface)
func (m *MaxLength) Validate(value TypedValue, baseType Type) error {
	return m.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value meets maximum length.
func (m *MaxLength) ValidateLexical(lexical string, baseType Type) error {
	// per XSD 1.0 errata, length facets are ignored for QName and NOTATION types
	if isQNameOrNotationType(baseType) {
		return nil
	}
	length := getLength(lexical, baseType)
	if length > m.Value {
		return fmt.Errorf("length must be at most %d, got %d", m.Value, length)
	}
	return nil
}

// getLength calculates the length of a value according to XSD 1.0 specification.
// The unit of length varies by type:
//   - hexBinary/base64Binary: octets (bytes) - XSD 1.0 Part 2, sections 3.2.1.1-3.2.1.3
//   - list types: number of list items - XSD 1.0 Part 2, section 3.2.1
//   - string types: characters (Unicode code points) - XSD 1.0 Part 2, sections 3.2.1.1-3.2.1.3
func getLength(value string, baseType Type) int {
	if baseType == nil {
		// no type information - use character count as default
		return utf8.RuneCountInString(value)
	}

	// use LengthMeasurable interface if available
	if lm, ok := baseType.(LengthMeasurable); ok {
		return lm.MeasureLength(value)
	}

	// fallback: character count for types that don't implement LengthMeasurable
	return utf8.RuneCountInString(value)
}

// Enumeration represents an enumeration facet
type Enumeration struct {
	Values []string
	// ValueContexts holds namespace contexts aligned with Values.
	ValueContexts []map[string]string
	// QNameValues holds resolved QName values for QName/NOTATION enumerations.
	QNameValues []QName
	cachedBase  Type
	// cachedAtomicValues holds parsed values for atomic enumerations.
	cachedAtomicValues []TypedValue
	// cachedUnionValues holds parsed values for union enumerations (flattened across member types).
	cachedUnionValues []TypedValue
	// cachedListValues holds parsed list values for list enumerations.
	cachedListValues [][][]TypedValue
}

// Name returns the facet name
func (e *Enumeration) Name() string {
	return "enumeration"
}

// Validate checks if the value is in the enumeration (unified Facet interface)
func (e *Enumeration) Validate(value TypedValue, baseType Type) error {
	return e.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value is in the enumeration.
func (e *Enumeration) ValidateLexical(lexical string, baseType Type) error {
	if e == nil {
		return nil
	}
	if baseType == nil {
		return fmt.Errorf("enumeration: missing base type")
	}

	normalized := NormalizeWhiteSpace(lexical, baseType)

	if isQNameOrNotationType(baseType) {
		if slices.Contains(e.Values, normalized) {
			return nil
		}
		return fmt.Errorf("value %s not in enumeration: %s", lexical, FormatEnumerationValues(e.Values))
	}

	if itemType, ok := ListItemType(baseType); ok {
		match, err := e.matchesListEnumeration(normalized, baseType, itemType)
		if err != nil {
			return err
		}
		if match {
			return nil
		}
		return fmt.Errorf("value %s not in enumeration: %s", lexical, FormatEnumerationValues(e.Values))
	}

	if memberTypes := unionMemberTypes(baseType); len(memberTypes) > 0 {
		match, err := e.matchesUnionEnumeration(normalized, baseType, memberTypes)
		if err != nil {
			return err
		}
		if match {
			return nil
		}
		return fmt.Errorf("value %s not in enumeration: %s", lexical, FormatEnumerationValues(e.Values))
	}

	match, err := e.matchesAtomicEnumeration(normalized, baseType)
	if err != nil {
		return err
	}
	if match {
		return nil
	}
	return fmt.Errorf("value %s not in enumeration: %s", lexical, FormatEnumerationValues(e.Values))
}

func (e *Enumeration) matchesAtomicEnumeration(lexical string, baseType Type) (bool, error) {
	actual, err := parseTypedValue(lexical, baseType)
	if err != nil {
		return false, err
	}
	allowed, err := e.atomicEnumerationValues(baseType)
	if err != nil {
		return false, err
	}
	for _, candidate := range allowed {
		if ValuesEqual(actual, candidate) {
			return true, nil
		}
	}
	return false, nil
}

func (e *Enumeration) matchesUnionEnumeration(lexical string, baseType Type, memberTypes []Type) (bool, error) {
	actualValues, err := parseUnionValueVariants(lexical, memberTypes)
	if err != nil {
		return false, err
	}
	allowed, err := e.unionEnumerationValues(baseType, memberTypes)
	if err != nil {
		return false, err
	}
	for _, actual := range actualValues {
		for _, candidate := range allowed {
			if ValuesEqual(actual, candidate) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *Enumeration) matchesListEnumeration(lexical string, baseType, itemType Type) (bool, error) {
	actualItems, err := parseListValueVariants(lexical, itemType)
	if err != nil {
		return false, err
	}
	allowed, err := e.listEnumerationValues(baseType, itemType)
	if err != nil {
		return false, err
	}
	for _, candidate := range allowed {
		if listValuesEqual(actualItems, candidate) {
			return true, nil
		}
	}
	return false, nil
}

func (e *Enumeration) resetCacheIfNeeded(baseType Type) {
	if e.cachedBase == baseType {
		return
	}
	e.cachedBase = baseType
	e.cachedAtomicValues = nil
	e.cachedUnionValues = nil
	e.cachedListValues = nil
}

func (e *Enumeration) atomicEnumerationValues(baseType Type) ([]TypedValue, error) {
	e.resetCacheIfNeeded(baseType)
	if e.cachedAtomicValues != nil {
		return e.cachedAtomicValues, nil
	}
	values := make([]TypedValue, 0, len(e.Values))
	for _, val := range e.Values {
		normalized := NormalizeWhiteSpace(val, baseType)
		typed, err := parseTypedValue(normalized, baseType)
		if err != nil {
			return nil, fmt.Errorf("enumeration value %q: %w", val, err)
		}
		values = append(values, typed)
	}
	e.cachedAtomicValues = values
	return values, nil
}

func (e *Enumeration) unionEnumerationValues(baseType Type, memberTypes []Type) ([]TypedValue, error) {
	e.resetCacheIfNeeded(baseType)
	if e.cachedUnionValues != nil {
		return e.cachedUnionValues, nil
	}
	values := make([]TypedValue, 0, len(e.Values))
	for _, val := range e.Values {
		normalized := NormalizeWhiteSpace(val, baseType)
		typed, err := parseUnionValueVariants(normalized, memberTypes)
		if err != nil {
			return nil, fmt.Errorf("enumeration value %q: %w", val, err)
		}
		values = append(values, typed...)
	}
	e.cachedUnionValues = values
	return values, nil
}

func (e *Enumeration) listEnumerationValues(baseType, itemType Type) ([][][]TypedValue, error) {
	e.resetCacheIfNeeded(baseType)
	if e.cachedListValues != nil {
		return e.cachedListValues, nil
	}
	values := make([][][]TypedValue, len(e.Values))
	for i, val := range e.Values {
		normalized := NormalizeWhiteSpace(val, baseType)
		parsed, err := parseListValueVariants(normalized, itemType)
		if err != nil {
			return nil, fmt.Errorf("enumeration value %q: %w", val, err)
		}
		values[i] = parsed
	}
	e.cachedListValues = values
	return values, nil
}

func parseTypedValue(lexical string, typ Type) (TypedValue, error) {
	switch t := typ.(type) {
	case *SimpleType:
		return t.ParseValue(lexical)
	case *BuiltinType:
		return t.ParseValue(lexical)
	default:
		return nil, fmt.Errorf("unsupported type %T", typ)
	}
}

func parseUnionValueVariants(lexical string, memberTypes []Type) ([]TypedValue, error) {
	if len(memberTypes) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	values := make([]TypedValue, 0, len(memberTypes))
	var firstErr error
	for _, memberType := range memberTypes {
		typed, err := parseTypedValue(lexical, memberType)
		if err == nil {
			values = append(values, typed)
			continue
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if len(values) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("value %q does not match any union member type", lexical)
	}
	return values, nil
}

func parseListValueVariants(lexical string, itemType Type) ([][]TypedValue, error) {
	if itemType == nil {
		return nil, fmt.Errorf("list item type is nil")
	}
	items := splitXMLWhitespaceFields(lexical)
	if len(items) == 0 {
		return nil, nil
	}
	parsed := make([][]TypedValue, len(items))
	for i, item := range items {
		values, err := parseValueVariants(item, itemType)
		if err != nil {
			return nil, fmt.Errorf("invalid list item %q: %w", item, err)
		}
		parsed[i] = values
	}
	return parsed, nil
}

func parseValueVariants(lexical string, typ Type) ([]TypedValue, error) {
	if members := unionMemberTypes(typ); len(members) > 0 {
		return parseUnionValueVariants(lexical, members)
	}
	typed, err := parseTypedValue(lexical, typ)
	if err != nil {
		return nil, err
	}
	return []TypedValue{typed}, nil
}

func listValuesEqual(left, right [][]TypedValue) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !anyValueEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

func anyValueEqual(left, right []TypedValue) bool {
	for _, l := range left {
		for _, r := range right {
			if ValuesEqual(l, r) {
				return true
			}
		}
	}
	return false
}

// ResolveQNameValues parses enumeration values as QNames using ValueContexts.
// It returns a QName for each entry in Values or an error if any value cannot be resolved.
func (e *Enumeration) ResolveQNameValues() ([]QName, error) {
	if e == nil || len(e.Values) == 0 {
		return nil, nil
	}
	if len(e.ValueContexts) != len(e.Values) {
		return nil, fmt.Errorf("enumeration contexts %d do not match values %d", len(e.ValueContexts), len(e.Values))
	}
	qnames := make([]QName, len(e.Values))
	for i, value := range e.Values {
		context := e.ValueContexts[i]
		if context == nil {
			return nil, fmt.Errorf("missing namespace context for enumeration value %q", value)
		}
		qname, err := ParseQNameValue(value, context)
		if err != nil {
			return nil, fmt.Errorf("invalid QName enumeration value %q: %w", value, err)
		}
		qnames[i] = qname
	}
	return qnames, nil
}

// FormatEnumerationValues returns a quoted list for enumeration errors.
func FormatEnumerationValues(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = strconv.Quote(value)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// TotalDigits represents a totalDigits facet
type TotalDigits struct {
	Value int
}

// Name returns the facet name
func (t *TotalDigits) Name() string {
	return "totalDigits"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (t *TotalDigits) GetIntValue() int {
	return t.Value
}

// Validate checks if the total number of digits doesn't exceed the limit (unified Facet interface)
func (t *TotalDigits) Validate(value TypedValue, baseType Type) error {
	return t.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value respects totalDigits.
func (t *TotalDigits) ValidateLexical(lexical string, _ Type) error {
	lexical = TrimXMLWhitespace(lexical)
	digitCount := countDigits(lexical)
	if digitCount > t.Value {
		return fmt.Errorf("total number of digits (%d) exceeds limit (%d)", digitCount, t.Value)
	}
	return nil
}

// FractionDigits represents a fractionDigits facet
type FractionDigits struct {
	Value int
}

// Name returns the facet name
func (f *FractionDigits) Name() string {
	return "fractionDigits"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (f *FractionDigits) GetIntValue() int {
	return f.Value
}

// Validate checks if the number of fractional digits doesn't exceed the limit (unified Facet interface)
func (f *FractionDigits) Validate(value TypedValue, baseType Type) error {
	return f.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value respects fractionDigits.
func (f *FractionDigits) ValidateLexical(lexical string, _ Type) error {
	lexical = TrimXMLWhitespace(lexical)
	fractionDigits := countFractionDigits(lexical)
	if fractionDigits > f.Value {
		return fmt.Errorf("number of fraction digits (%d) exceeds limit (%d)", fractionDigits, f.Value)
	}
	return nil
}

// countDigits counts the total number of digits in a string
func countDigits(value string) int {
	count := 0
	for _, r := range value {
		if r >= '0' && r <= '9' {
			count++
		}
	}
	return count
}

// countFractionDigits counts digits after the decimal point
func countFractionDigits(value string) int {
	_, after, ok := strings.Cut(value, ".")
	if !ok {
		return 0 // no decimal point, so no fraction digits
	}

	fractionPart := after

	// remove exponent if present (e.g., "1.23E4" -> "1.23")
	if eIdx := strings.IndexAny(fractionPart, "Ee"); eIdx >= 0 {
		fractionPart = fractionPart[:eIdx]
	}

	return countDigits(fractionPart)
}
