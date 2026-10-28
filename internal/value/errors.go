package value

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/xsderrors"
)

// ErrMetadata reports a malformed or incomplete sealed value program.
var ErrMetadata = errors.New("value program metadata is invalid")

// ErrLimit reports a bounded value-program construction or evaluation limit.
var ErrLimit = errors.New("value program limit exceeded")

// ErrFacet classifies a value that fails an effective facet. The concrete
// error text remains available for diagnostics.
var ErrFacet = errors.New("value facet failed")

// IsUnsupported reports whether err is an unsupported datatype operation,
// preserving the structured diagnostic supplied by the owning package.
func IsUnsupported(err error) bool { return xsderrors.IsUnsupported(err) }

type facetError string

func (e facetError) Error() string { return string(e) }

func (facetError) Unwrap() error { return ErrFacet }

func facetFailure(message string) error { return facetError(message) }

// Raw numeric rejection can be speculative inside a successful union. Keep its
// marker unwrapped; ErrFacet classification is added at the terminal boundary.
type rawBoundError string

func (e rawBoundError) Error() string { return string(e) }

// Lexical admission and primitive parsing.
const (
	msgInvalidName              = "invalid Name"
	msgInvalidNCName            = "invalid NCName"
	msgInvalidNMTOKEN           = "invalid NMTOKEN"
	msgInvalidLanguage          = "invalid language"
	msgInvalidDecimal           = "invalid decimal"
	msgInvalidInteger           = "invalid integer"
	msgInvalidGPrimitive        = "invalid g value primitive"
	msgInvalidGYearMonth        = "invalid gYearMonth"
	msgInvalidGMonthDay         = "invalid gMonthDay"
	msgInvalidGDay              = "invalid gDay"
	msgInvalidGMonth            = "invalid gMonth"
	msgInvalidDuration          = "invalid duration"
	msgInvalidFloatPrimitive    = "invalid float primitive"
	msgInvalidFloat             = "invalid float"
	msgInvalidBinaryPrimitive   = "invalid binary primitive"
	msgInvalidHexBinary         = "invalid hexBinary"
	msgInvalidBase64Binary      = "invalid base64Binary"
	msgInvalidTemporalPrimitive = "invalid temporal primitive"
	msgInvalidDateTime          = "invalid dateTime"
	msgInvalidDatePart          = "invalid date/time"
	msgInvalidTimezone          = "invalid timezone"
	msgInvalidTime              = "invalid time"
	msgInvalidUTF8              = "invalid UTF-8 value"
	msgInvalidXMLCharacter      = "invalid XML character"
	msgInvalidQName             = "invalid QName syntax; expected a local name or prefix:local name"
	msgUnresolvedQName          = "unresolved QName"
	msgUndeclaredNotation       = "undeclared notation"
	msgNoUnionMember            = "value does not match any union member"
	msgUnsupportedEntity        = "ENTITY requires DTD entity declarations, which are not supported"
)

// Checked representation limits.
const (
	msgStringLengthLimit       = "string length exceeds uint32 limit"
	msgAnyURILengthLimit       = "anyURI length exceeds uint32 limit"
	msgHexBinaryLengthLimit    = "hexBinary length exceeds uint32 limit"
	msgBase64BinaryLengthLimit = "base64Binary length exceeds uint32 limit"
	msgTotalDigitsLimit        = "decimal totalDigits exceeds uint32 limit"
	msgFractionDigitsLimit     = "decimal fractionDigits exceeds uint32 limit"
)

// Facet metadata and literal admission.
const (
	msgBoundKindsConflict    = "inclusive and exclusive bounds cannot both be present"
	msgTotalDigitsPositive   = "totalDigits must be positive"
	msgFacetCardinality      = "facet cardinality relationship is invalid"
	msgOrderedAtomicRequired = "ordered facets require an ordered atomic type"
	msgAtomicDecimalRequired = "digit facets require atomic decimal type"
	msgLengthCapableRequired = "length facets require a length-capable type"
	msgUnionLengthForbidden  = "length facets are not valid on union types"
	msgInvalidOrderedBounds  = "ordered facet bounds are invalid"
	msgTypeMetadata          = "type %d: %w"
	msgBoundValue            = "bound %q: %w"
	msgIncompatibleBound     = "bound %q has an incompatible value type"
	msgEnumerationValue      = "enumeration %q: %w"
)

// Effective facets and restrictions retain ErrFacet classification.
const (
	errLengthFacet                    facetError    = "length facet failed"
	errPatternFacet                   facetError    = "pattern facet failed"
	errDigitFacet                     facetError    = "digit facet failed"
	errLowerBoundFacet                facetError    = "lower bound facet failed"
	errUpperBoundFacet                facetError    = "upper bound facet failed"
	errMinInclusiveFacet              rawBoundError = "minInclusive facet failed"
	errMaxInclusiveFacet              rawBoundError = "maxInclusive facet failed"
	errEnumerationFacet               facetError    = "enumeration facet failed"
	errFacetNotAllowed                facetError    = "facet is not allowed for simple type"
	errFixedPatternEnumeration        facetError    = "pattern and enumeration facets cannot be fixed"
	errRestrictionSemantics           facetError    = "simple type semantic fields do not match base restriction"
	errRestrictionListItem            facetError    = "simple type list item does not match base restriction"
	errRestrictionUnionMembers        facetError    = "simple type union members do not match base restriction"
	errRestrictionAtomicShape         facetError    = "atomic type shape does not match base restriction"
	errWhitespaceLoosensBase          facetError    = "whiteSpace cannot loosen base restriction"
	errLengthBelowMinLength           facetError    = "length cannot be less than minLength"
	errLengthAboveMaxLength           facetError    = "length cannot exceed maxLength"
	errMinLengthAboveMaxLength        facetError    = "minLength cannot exceed maxLength"
	errFractionDigitsAboveTotalDigits facetError    = "fractionDigits cannot exceed totalDigits"
	errLengthChangesBase              facetError    = "length must equal base length"
	errMinLengthLoosensBase           facetError    = "minLength cannot be less than base minLength"
	errMaxLengthLoosensBase           facetError    = "maxLength cannot exceed base maxLength"
	errTotalDigitsLoosensBase         facetError    = "totalDigits cannot exceed base totalDigits"
	errFractionDigitsLoosensBase      facetError    = "fractionDigits cannot exceed base fractionDigits"
	errLengthRequiresBaseMinLength    facetError    = "length requires an inherited minLength with the same value"
	errLengthRequiresBaseMaxLength    facetError    = "length requires an inherited maxLength with the same value"
	errLowerBoundLoosensBase          facetError    = "lower ordered facet cannot loosen base restriction"
	errUpperBoundLoosensBase          facetError    = "upper ordered facet cannot loosen base restriction"
	errEnumerationLoosensBase         facetError    = "enumeration facet cannot loosen base restriction"
	errFixedWhitespaceChanges         facetError    = "fixed whiteSpace facet cannot change"
	errFixedCardinalityChanges        facetError    = "fixed cardinality facet cannot change"
	errFixedOrderedChanges            facetError    = "fixed ordered facet cannot change"
)

var errBoolean = errors.New("invalid boolean")

func invalidTypeError(label string) error {
	return errors.New("invalid " + label)
}

func enumerationDiagnostic[T byteText](lexical T, group []parsedValue) string {
	var message strings.Builder
	message.WriteString("value ")
	message.WriteString(diagnostictext.Quote(lexical))
	message.WriteString(" is not one of [")
	for i := range min(len(group), diagnostictext.MaxAlternatives) {
		if i != 0 {
			message.WriteString(", ")
		}
		message.WriteString(diagnostictext.Quote(group[i].canonical))
	}
	if len(group) > diagnostictext.MaxAlternatives {
		message.WriteString(", ...")
	}
	message.WriteByte(']')
	return message.String()
}

type valueDiagnosticError struct {
	cause   error
	message string
}

func (e *valueDiagnosticError) Error() string { return e.message }
func (e *valueDiagnosticError) Unwrap() error { return e.cause }

func (e *valueDiagnosticError) Is(target error) bool {
	_, rawBound := e.cause.(rawBoundError)
	return target == ErrFacet && rawBound
}

// describeValueFailure runs only after a complete value has failed. A rejected
// union member can still lead to successful validation, so speculative checks
// return their cheap error identities without formatting diagnostic context.
func describeValueFailure[T byteText](program *Program, id TypeID, lexical T, parsed *parsedValue, cause error) error {
	if errors.Is(cause, ErrLimit) || errors.Is(cause, ErrMetadata) || IsUnsupported(cause) {
		return cause
	}
	t, ok := program.typeDef(id)
	if !ok {
		return cause
	}
	var message string
	switch {
	case t.variety == Union && (parsed == nil || parsed.typeID != id):
		message = unionDiagnostic(program, t, lexical)
	case failedListItem(t, parsed, cause):
		message = fmt.Sprintf("value %s is not a valid list of %s: %s", diagnostictext.Quote(lexical), diagnosticTypeDescription(program, t.listItem, 0), cause.Error())
	default:
		message = facetDiagnostic(t, lexical, parsed, cause)
	}
	if message == "" {
		message = lexicalValueDiagnostic(program, id, lexical, cause)
	}
	return &valueDiagnosticError{message: message, cause: cause}
}

func failedListItem(t *typeDef, parsed *parsedValue, cause error) bool {
	if t.variety != List || parsed != nil && parsed.isList {
		return false
	}
	return parsed != nil || t.facets.raw.kind != rawPlanNMTokens || !errors.Is(cause, errLengthFacet)
}

func lexicalValueDiagnostic[T byteText](program *Program, id TypeID, lexical T, cause error) string {
	return fmt.Sprintf("value %s is not valid for %s: %s", diagnostictext.Quote(lexical), diagnosticTypeName(program, id), cause.Error())
}

func diagnosticTypeName(program *Program, id TypeID) string {
	if name, ok := BuiltinTypeName(id); ok {
		return "xs:" + name
	}
	t, ok := program.typeDef(id)
	if !ok {
		return "the declared type"
	}
	if t.variety == Union {
		return "a union member type"
	}
	if t.variety == List {
		return "a list type"
	}
	// An atomic restriction names its nearest built-in ancestor (xs:int, not
	// the xs:integer lexical family); user QNames are not program vocabulary.
	// Admission completes bases first, so the walk ends within len(types).
	for range len(program.types) + 1 {
		if name, ok := BuiltinTypeName(t.base); ok {
			return "xs:" + name
		}
		next, ok := program.typeDef(t.base)
		if !ok {
			break
		}
		t = next
	}
	// A base-less atomic type names its lexical family.
	builtin := BuiltinType(t.primitive)
	switch t.builtin {
	case BuiltinInteger:
		builtin = builtinInteger
	case BuiltinName:
		builtin = builtinName
	case BuiltinNCName:
		builtin = builtinNCName
	case BuiltinNMTOKEN:
		builtin = builtinNMTOKEN
	case BuiltinLanguage:
		builtin = builtinLanguage
	case BuiltinEntity:
		builtin = builtinENTITY
	case BuiltinNone:
	}
	name, _ := BuiltinTypeName(builtin)
	return "xs:" + name
}

func unionDiagnostic[T byteText](program *Program, t *typeDef, lexical T) string {
	var message strings.Builder
	message.WriteString("value ")
	message.WriteString(diagnostictext.Quote(lexical))
	message.WriteString(" does not match any union member; expected one of [")
	for i := range min(len(t.union), diagnostictext.MaxAlternatives) {
		if i != 0 {
			message.WriteString(", ")
		}
		message.WriteString(diagnosticTypeDescription(program, t.union[i], 0))
	}
	if len(t.union) > diagnostictext.MaxAlternatives {
		message.WriteString(", ...")
	}
	message.WriteByte(']')
	return message.String()
}

func facetDiagnostic[T byteText](t *typeDef, lexical T, parsed *parsedValue, cause error) string {
	switch {
	case errors.Is(cause, errEnumerationFacet):
		return failedEnumerationDiagnostic(t.facets.enumGroups, lexical, parsed)
	case errors.Is(cause, errLengthFacet):
		return lengthDiagnostic(t, lexical, parsed)
	case errors.Is(cause, errDigitFacet):
		return digitDiagnostic(&t.facets, lexical, parsed)
	case errors.Is(cause, errPatternFacet):
		return "value " + diagnostictext.Quote(lexical) + " does not satisfy pattern groups " + diagnosticPatternGroups(t.facets.patterns)
	case errors.Is(cause, errMinInclusiveFacet):
		if len(t.facets.lower) != 0 {
			return boundDiagnostic(t, lexical, "minInclusive", &t.facets.lower[0])
		}
	case errors.Is(cause, errMaxInclusiveFacet):
		if len(t.facets.upper) != 0 {
			return boundDiagnostic(t, lexical, "maxInclusive", &t.facets.upper[0])
		}
	case errors.Is(cause, errLowerBoundFacet), errors.Is(cause, errUpperBoundFacet):
		if parsed != nil {
			return failedOrderedBoundDiagnostic(t, lexical, parsed)
		}
	}
	return ""
}

func failedEnumerationDiagnostic[T byteText](groups [][]parsedValue, lexical T, parsed *parsedValue) string {
	for _, group := range groups {
		if !diagnosticEnumerationMatches(group, lexical, parsed) {
			return enumerationDiagnostic(lexical, group)
		}
	}
	return ""
}

func diagnosticEnumerationMatches[T byteText](group []parsedValue, lexical T, parsed *parsedValue) bool {
	if parsed == nil {
		return stringEnumGroupMatches(group, []byte(lexical))
	}
	for i := range group {
		if equalParsed(parsed, &group[i]) {
			return true
		}
	}
	return false
}

func digitDiagnostic[T byteText](f *facetProgram, lexical T, parsed *parsedValue) string {
	if parsed == nil {
		return ""
	}
	decimal := &parsed.atom.decimal
	if f.totalDigits.Present && decimal.TotalDigits > f.totalDigits.Value {
		return fmt.Sprintf("value %s has %d total digits; totalDigits permits at most %d", diagnostictext.Quote(lexical), decimal.TotalDigits, f.totalDigits.Value)
	}
	return fmt.Sprintf("value %s has %d fractional digits; fractionDigits permits at most %d", diagnostictext.Quote(lexical), decimal.FractionDigits, f.fractionDigits.Value)
}

func lengthDiagnostic[T byteText](t *typeDef, lexical T, parsed *parsedValue) string {
	count, unit := diagnosticLength(t, lexical, parsed)
	if count == 1 {
		unit = strings.TrimSuffix(unit, "s")
	}
	f := &t.facets
	relation, expected := "exactly", f.length.Value
	if !f.length.Present {
		if f.minLength.Present && count < uint64(f.minLength.Value) {
			relation, expected = "at least", f.minLength.Value
		} else {
			relation, expected = "at most", f.maxLength.Value
		}
	}
	return fmt.Sprintf("value %s has length %d %s; expected %s %d", diagnostictext.Quote(lexical), count, unit, relation, expected)
}

const (
	diagnosticLengthItems      = "items"
	diagnosticLengthCharacters = "characters"
)

func diagnosticLength[T byteText](t *typeDef, lexical T, parsed *parsedValue) (uint64, string) {
	if t.variety == List {
		if parsed != nil {
			return uint64(parsed.count), diagnosticLengthItems
		}
		return listFieldCount(string(lexical)), diagnosticLengthItems
	}
	if parsed != nil {
		unit := diagnosticLengthCharacters
		if t.primitive == PrimitiveHexBinary || t.primitive == PrimitiveBase64Binary {
			unit = "bytes"
		}
		return uint64(parsed.atom.lexical.Length), unit
	}
	var count uint64
	for range string(lexical) {
		count++
	}
	return count, diagnosticLengthCharacters
}

func diagnosticPatternGroups(groups [][]*Pattern) string {
	var message strings.Builder
	remaining := diagnostictext.MaxAlternatives
	for i, group := range groups {
		if remaining == 0 {
			message.WriteString(" and ...")
			break
		}
		if i != 0 {
			message.WriteString(" and ")
		}
		remaining = appendDiagnosticPatternGroup(&message, group, remaining)
	}
	return message.String()
}

func appendDiagnosticPatternGroup(message *strings.Builder, group []*Pattern, remaining int) int {
	message.WriteByte('[')
	for i, pattern := range group {
		if remaining == 0 {
			message.WriteString(", ...")
			break
		}
		if i != 0 {
			message.WriteString(" or ")
		}
		message.WriteString(diagnostictext.Quote(pattern.Source()))
		remaining--
	}
	message.WriteByte(']')
	return remaining
}

func diagnosticTypeDescription(program *Program, id TypeID, depth int) string {
	name := diagnosticTypeName(program, id)
	if id < BuiltinTypeCount {
		return name
	}
	if depth == 3 {
		return name + " (...)"
	}
	t, ok := program.typeDef(id)
	if !ok {
		return name
	}
	if t.variety == List {
		name = "list of " + diagnosticTypeDescription(program, t.listItem, depth+1)
	}
	rules := diagnosticFacetRules(t.builtin, &t.facets)
	if len(rules) != 0 {
		return name + " (" + strings.Join(rules, "; ") + ")"
	}
	return name
}

func diagnosticCardinalityRules(f *facetProgram) []string {
	var rules []string
	if f.length.Present {
		rules = append(rules, fmt.Sprintf("length exactly %d", f.length.Value))
	}
	if f.minLength.Present {
		rules = append(rules, fmt.Sprintf("length at least %d", f.minLength.Value))
	}
	if f.maxLength.Present {
		rules = append(rules, fmt.Sprintf("length at most %d", f.maxLength.Value))
	}
	if f.totalDigits.Present {
		rules = append(rules, fmt.Sprintf("totalDigits at most %d", f.totalDigits.Value))
	}
	if f.fractionDigits.Present {
		rules = append(rules, fmt.Sprintf("fractionDigits at most %d", f.fractionDigits.Value))
	}
	return rules
}

func diagnosticFacetRules(builtin BuiltinKind, f *facetProgram) []string {
	rules := diagnosticCardinalityRules(f)
	if len(f.enumGroups) != 0 {
		rules = append(rules, "enumeration groups "+diagnosticEnumerationGroups(f.enumGroups))
	}
	if len(f.patterns) != 0 {
		rules = append(rules, "pattern groups "+diagnosticPatternGroups(f.patterns))
	}
	for _, side := range []struct {
		prefix string
		bounds []boundValue
	}{{"min", f.lower}, {"max", f.upper}} {
		var truncated bool
		rules, truncated = appendDiagnosticBounds(rules, builtin, side.prefix, side.bounds)
		if truncated {
			return append(rules, "...")
		}
	}
	return rules
}

func diagnosticBoundName(prefix string, bound *boundValue) string {
	if bound.exclusive {
		return prefix + "Exclusive"
	}
	return prefix + "Inclusive"
}

func appendDiagnosticBounds(rules []string, builtin BuiltinKind, prefix string, bounds []boundValue) ([]string, bool) {
	for i := range bounds {
		if len(rules) == diagnostictext.MaxAlternatives {
			return rules, true
		}
		bound := &bounds[i]
		rules = append(rules, diagnosticBoundName(prefix, bound)+" "+diagnosticBoundValue(builtin, &bound.value))
	}
	return rules, false
}

func diagnosticEnumerationGroups(groups [][]parsedValue) string {
	var text strings.Builder
	remaining := diagnostictext.MaxAlternatives
	for i, group := range groups {
		if remaining == 0 {
			text.WriteString(" and ...")
			break
		}
		if i != 0 {
			text.WriteString(" and ")
		}
		remaining = appendDiagnosticEnumerationGroup(&text, group, remaining)
	}
	return text.String()
}

func appendDiagnosticEnumerationGroup(text *strings.Builder, group []parsedValue, remaining int) int {
	text.WriteByte('[')
	for i, literal := range group {
		if remaining == 0 {
			text.WriteString(", ...")
			break
		}
		if i != 0 {
			text.WriteString(", ")
		}
		text.WriteString(diagnostictext.Quote(literal.canonical))
		remaining--
	}
	text.WriteByte(']')
	return remaining
}

func failedLowerBoundDiagnostic[T byteText](t *typeDef, lexical T, parsed *parsedValue) string {
	for i := range t.facets.lower {
		bound := &t.facets.lower[i]
		relation, ok := compareAtomic(&parsed.atom, &bound.value)
		if !ok || !OrderedFacetLowerBoundAccepts(bound.exclusive, relation) {
			name := diagnosticBoundName("min", bound)
			return boundDiagnostic(t, lexical, name, bound)
		}
	}
	return ""
}

func failedOrderedBoundDiagnostic[T byteText](t *typeDef, lexical T, parsed *parsedValue) string {
	if message := failedLowerBoundDiagnostic(t, lexical, parsed); message != "" {
		return message
	}
	for i := range t.facets.upper {
		bound := &t.facets.upper[i]
		relation, ok := compareAtomic(&parsed.atom, &bound.value)
		if !ok || !OrderedFacetUpperBoundAccepts(bound.exclusive, relation) {
			name := diagnosticBoundName("max", bound)
			return boundDiagnostic(t, lexical, name, bound)
		}
	}
	return ""
}

func boundDiagnostic[T byteText](t *typeDef, lexical T, name string, bound *boundValue) string {
	return fmt.Sprintf("value %s does not satisfy %s %s", diagnostictext.Quote(lexical), name, diagnosticBoundValue(t.builtin, &bound.value))
}

// diagnosticBoundValue renders a bound in the canonical form of the
// restricted type, so integer-family bounds carry no fractional part.
func diagnosticBoundValue(builtin BuiltinKind, atom *atomicValue) string {
	switch atom.kind {
	case PrimitiveFloat, PrimitiveDouble:
		bits, _ := floatBits(atom.kind)
		return diagnostictext.Quote(formatFloatCanonical(atom.float, bits))
	case PrimitiveDate:
		return diagnostictext.Quote((DateValue{point: atom.temporal.instant, hasTZ: atom.temporal.hasTZ}).CanonicalText())
	case PrimitiveDateTime:
		return diagnostictext.Quote(DateTimeValue(atom.temporal).CanonicalText())
	case PrimitiveDuration:
		return diagnosticDuration(atom.duration)
	case PrimitiveString, PrimitiveBoolean, PrimitiveDecimal, PrimitiveTime,
		PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay,
		PrimitiveGMonth, PrimitiveHexBinary, PrimitiveBase64Binary,
		PrimitiveAnyURI, PrimitiveQName, PrimitiveNotation:
		return diagnostictext.Quote(atomicCanonical(builtin, atom, ""))
	}
	return "the declared bound"
}

func diagnosticDuration(duration DurationValue) string {
	months, seconds := duration.coordinates()
	negative := duration.negativeFrac
	if strings.HasPrefix(months.text(), "-") {
		months, negative = months.neg(), true
	}
	if strings.HasPrefix(seconds.text(), "-") {
		seconds, negative = seconds.neg(), true
	}
	parts := make([]string, 0, 24)
	if negative {
		parts = append(parts, "-")
	}
	parts = append(parts, "P")
	parts, remainingSeconds := appendDiagnosticDurationDate(parts, months, seconds)
	if remainingSeconds != 0 || duration.frac != "" || len(parts) == 1 || negative && len(parts) == 2 {
		parts = appendDiagnosticDurationTime(parts, remainingSeconds, duration.frac)
	}
	return diagnostictext.QuoteParts(parts...)
}

func appendDiagnosticDurationDate(parts []string, months, seconds durationInteger) ([]string, int64) {
	years, remainingMonths := months.divFloor(12)
	days, remainingSeconds := seconds.divFloor(86400)
	if years.small != 0 || years.digits != "" {
		parts = append(parts, years.text(), "Y")
	}
	if remainingMonths != 0 {
		parts = append(parts, strconv.FormatInt(remainingMonths, 10), "M")
	}
	if days.small != 0 || days.digits != "" {
		parts = append(parts, days.text(), "D")
	}
	return parts, remainingSeconds
}

func appendDiagnosticDurationTime(parts []string, remainingSeconds int64, fraction string) []string {
	parts = append(parts, "T")
	if hours := remainingSeconds / 3600; hours != 0 {
		parts = append(parts, strconv.FormatInt(hours, 10), "H")
	}
	if minutes := remainingSeconds % 3600 / 60; minutes != 0 {
		parts = append(parts, strconv.FormatInt(minutes, 10), "M")
	}
	seconds := remainingSeconds % 60
	if seconds != 0 || fraction != "" || parts[len(parts)-1] == "T" {
		parts = append(parts, strconv.FormatInt(seconds, 10))
		if fraction != "" {
			parts = append(parts, ".", fraction)
		}
		parts = append(parts, "S")
	}
	return parts
}
