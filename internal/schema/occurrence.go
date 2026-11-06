package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"

	"github.com/jacoelho/xsd/xsderrors"
)

const (
	// maxUint32Text is the decimal text form of math.MaxUint32.
	maxUint32Text              = "4294967295"
	occurrenceUnboundedLexical = "unbounded"
)

// rawOccurrence is the raw occurrence attribute projection from a model
// group or particle node.
type rawOccurrence struct {
	MinOccurs    string
	MaxOccurs    string
	HasMinOccurs bool
	HasMaxOccurs bool
}

// parseOccurrence parses minOccurs/maxOccurs and applies compile-time finite
// occurrence limits.
func parseOccurrence(attrs rawOccurrence, limits Limits) (occurrence, error) {
	minOccurs, err := parseMinOccurrence(attrs)
	if err != nil {
		return occurrence{}, err
	}
	maximum, err := parseMaxOccurrence(attrs, limits.MaxFiniteOccurs)
	if err != nil {
		return occurrence{}, err
	}
	return maximum.withMinimum(minOccurs)
}

func (m maxOccurrence) withMinimum(minOccurs uint32) (occurrence, error) {
	switch m.kind {
	case maxOccurrenceUnbounded:
		return occurrence{Min: minOccurs, Unbounded: true}, nil
	case maxOccurrenceFinite:
	case maxOccurrenceInvalid:
		return occurrence{}, xsderrors.InternalInvariant("maxOccurs parser returned an invalid kind")
	default:
		return occurrence{}, xsderrors.InternalInvariant("maxOccurs parser returned an unknown kind")
	}
	if m.value < minOccurs {
		return occurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "maxOccurs is less than minOccurs")
	}
	return occurrence{Min: minOccurs, Max: m.value}, nil
}

func parseMinOccurrence(attrs rawOccurrence) (uint32, error) {
	if !attrs.HasMinOccurs {
		return 1, nil
	}
	number, err := parseSchemaNonNegativeInteger(lex.TrimXMLWhitespaceString(attrs.MinOccurs))
	if err != nil {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "invalid minOccurs "+diagnostictext.Quote(attrs.MinOccurs))
	}
	if number > maxUint32Value {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "minOccurs exceeds uint32 limit")
	}
	return uint32(number), nil
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

func parseMaxOccurrence(attrs rawOccurrence, limit uint64) (maxOccurrence, error) {
	if !attrs.HasMaxOccurs {
		return maxOccurrence{value: 1, kind: maxOccurrenceFinite}, nil
	}
	if lex.TrimXMLWhitespaceString(attrs.MaxOccurs) == occurrenceUnboundedLexical {
		return maxOccurrence{kind: maxOccurrenceUnbounded}, nil
	}
	number, err := parseSchemaNonNegativeInteger(lex.TrimXMLWhitespaceString(attrs.MaxOccurs))
	if err != nil {
		return maxOccurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "invalid maxOccurs "+diagnostictext.Quote(attrs.MaxOccurs))
	}
	if maxOccursLimitExceeded(number, limit) {
		return maxOccurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, maxOccursLimitMessage(limit))
	}
	return maxOccurrence{value: uint32(number), kind: maxOccurrenceFinite}, nil // #nosec G115 -- maxOccursLimitExceeded enforces the uint32 cap.
}

// validateAllModelOccurrence validates xs:all model group occurrence admission.
func validateAllModelOccurrence(occurs occurrence) error {
	if occurs.Unbounded || occurs.Min > 1 || occurs.Max != 1 {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "xs:all occurrence must be zero or one")
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
