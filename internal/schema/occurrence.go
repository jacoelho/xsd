package schema

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	// maxUint32Text is the decimal text form of math.MaxUint32.
	maxUint32Text              = "4294967295"
	occurrenceUnboundedLexical = "unbounded"
)

// occurrenceScope is what the parent of a particle adds to the occurrence
// range of that particle's start tag (cos-all-limited).
type occurrenceScope uint8

const (
	occurrenceScopeParticle occurrenceScope = iota
	// occurrenceScopeAllGroup is an xs:all model group.
	occurrenceScopeAllGroup
	// occurrenceScopeAllMember is a particle of an xs:all model group.
	occurrenceScopeAllMember
)

const (
	allGroupOccurrenceMessage  = "xs:all occurrence must be zero or one"
	allMemberOccurrenceMessage = "xs:all particles cannot repeat"
)

// occurrenceAdmission is the only owner of the minOccurs and maxOccurs of one
// particle start tag. admitAttribute decides what one attribute decides alone,
// where it is written: its lexical form, the uint32 and MaxFiniteOccurs caps,
// and the xs:all range of its scope. complete decides what needs both
// attributes once every attribute is admitted: the defaults and minOccurs not
// above maxOccurs. The zero value admits a particle outside xs:all under the
// uint32 cap alone.
type occurrenceAdmission struct {
	limit  uint64
	max    maxOccurrence
	min    uint32
	scope  occurrenceScope
	hasMin bool
	hasMax bool
}

// newOccurrenceAdmission returns the admission of the start tag of element,
// a child of parent, whose finite maxOccurs is capped by limit; a zero limit
// caps it at uint32 alone.
func newOccurrenceAdmission(element, parent xml.Name, limit uint64) occurrenceAdmission {
	scope := occurrenceScopeParticle
	switch {
	case isXSDName(element, vocab.XSDElemAll):
		scope = occurrenceScopeAllGroup
	case isXSDName(parent, vocab.XSDElemAll):
		scope = occurrenceScopeAllMember
	}
	return occurrenceAdmission{limit: limit, scope: scope}
}

// admitAttribute admits value, the value of attr, minOccurs or maxOccurs.
func (a *occurrenceAdmission) admitAttribute(attr, value string) error {
	switch attr {
	case vocab.XSDAttrMinOccurs:
		return a.admitMin(value)
	case vocab.XSDAttrMaxOccurs:
		return a.admitMax(value)
	}
	return xsderrors.InternalInvariant("occurrence admission of attribute "+diagnostictext.Quote(attr), nil)
}

func (a *occurrenceAdmission) admitMin(value string) error {
	number, err := parseMinOccursLexical(value)
	if err != nil {
		return err
	}
	if number > maxUint32Value {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "minOccurs exceeds uint32 limit", nil)
	}
	minimum := uint32(number) // #nosec G115 -- checked against the uint32 cap above.
	if a.scope == occurrenceScopeAllGroup && !allGroupMinAdmitted(minimum) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, allGroupOccurrenceMessage, nil)
	}
	a.min, a.hasMin = minimum, true
	return nil
}

func (a *occurrenceAdmission) admitMax(value string) error {
	lexical, err := parseMaxOccursLexical(value)
	if err != nil {
		return err
	}
	maximum := maxOccurrence{kind: maxOccurrenceUnbounded}
	if !lexical.unbounded {
		if maxOccursLimitExceeded(lexical.count, a.limit) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, maxOccursLimitMessage(a.limit), nil)
		}
		maximum = maxOccurrence{value: uint32(lexical.count), kind: maxOccurrenceFinite} // #nosec G115 -- maxOccursLimitExceeded enforces the uint32 cap.
	}
	switch {
	case a.scope == occurrenceScopeAllGroup && !allGroupMaxAdmitted(maximum):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, allGroupOccurrenceMessage, nil)
	case a.scope == occurrenceScopeAllMember && maximum.repeats():
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, allMemberOccurrenceMessage, nil)
	}
	a.max, a.hasMax = maximum, true
	return nil
}

// complete returns the admitted occurrence, with minOccurs and maxOccurs
// defaulting to one.
func (a *occurrenceAdmission) complete() (occurrence, error) {
	minimum := uint32(1)
	if a.hasMin {
		minimum = a.min
	}
	maximum := maxOccurrence{value: 1, kind: maxOccurrenceFinite}
	if a.hasMax {
		maximum = a.max
	}
	return maximum.withMinimum(minimum)
}

func (m maxOccurrence) withMinimum(minOccurs uint32) (occurrence, error) {
	switch m.kind {
	case maxOccurrenceUnbounded:
		return occurrence{Min: minOccurs, Unbounded: true}, nil
	case maxOccurrenceFinite:
	case maxOccurrenceInvalid:
		return occurrence{}, xsderrors.InternalInvariant("maxOccurs parser returned an invalid kind", nil)
	default:
		return occurrence{}, xsderrors.InternalInvariant("maxOccurs parser returned an unknown kind", nil)
	}
	if m.value < minOccurs {
		return occurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "maxOccurs is less than minOccurs", nil)
	}
	return occurrence{Min: minOccurs, Max: m.value}, nil
}

// parseMinOccursLexical parses the lexical form of minOccurs. The result is
// exact through uint32; see parseSchemaNonNegativeInteger.
func parseMinOccursLexical(value string) (uint64, error) {
	number, err := parseSchemaNonNegativeInteger(lex.TrimXMLWhitespaceString(value))
	if err != nil {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "invalid minOccurs "+diagnostictext.Quote(value), nil)
	}
	return number, nil
}

// maxOccursLexical is a maxOccurs value as written: unbounded, or a count
// that is exact through uint32.
type maxOccursLexical struct {
	count     uint64
	unbounded bool
}

// parseMaxOccursLexical parses the lexical form of maxOccurs.
func parseMaxOccursLexical(value string) (maxOccursLexical, error) {
	trimmed := lex.TrimXMLWhitespaceString(value)
	if trimmed == occurrenceUnboundedLexical {
		return maxOccursLexical{unbounded: true}, nil
	}
	count, err := parseSchemaNonNegativeInteger(trimmed)
	if err != nil {
		return maxOccursLexical{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "invalid maxOccurs "+diagnostictext.Quote(value), nil)
	}
	return maxOccursLexical{count: count}, nil
}

type maxOccurrence struct {
	value uint32
	kind  maxOccurrenceKind
}

type maxOccurrenceKind uint8

const (
	maxOccurrenceInvalid maxOccurrenceKind = iota
	maxOccurrenceFinite
	maxOccurrenceUnbounded
)

func (m maxOccurrence) repeats() bool {
	return m.kind == maxOccurrenceUnbounded || m.value > 1
}

func allGroupMinAdmitted(minimum uint32) bool {
	return minimum <= 1
}

func allGroupMaxAdmitted(maximum maxOccurrence) bool {
	return maximum.kind == maxOccurrenceFinite && maximum.value == 1
}

// validateAllModelOccurrence checks occurs, the occurrence a model group
// reference gives an xs:all group, against the range of an xs:all.
func validateAllModelOccurrence(occurs occurrence) error {
	maximum := maxOccurrence{value: occurs.Max, kind: maxOccurrenceFinite}
	if occurs.Unbounded {
		maximum = maxOccurrence{kind: maxOccurrenceUnbounded}
	}
	if !allGroupMinAdmitted(occurs.Min) || !allGroupMaxAdmitted(maximum) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, allGroupOccurrenceMessage, nil)
	}
	return nil
}

func maxOccursLimitExceeded(number, limit uint64) bool {
	limitCap := maxUint32Value
	if limit != 0 && limit < limitCap {
		limitCap = limit
	}
	return number > limitCap
}

func maxOccursLimitMessage(limit uint64) string {
	if limit != 0 && limit < maxUint32Value {
		return "maxOccurs exceeds configured limit"
	}
	return "maxOccurs exceeds uint32 limit"
}
