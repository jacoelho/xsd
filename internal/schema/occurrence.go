package schema

import (
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"

	"github.com/jacoelho/xsd/xsderrors"
)

const (
	// MaxUint32Text is the decimal text form of math.MaxUint32.
	MaxUint32Text              = "4294967295"
	occurrenceUnboundedLexical = "unbounded"
)

// OccurrenceAttrs is the raw occurrence attribute projection from a model
// group or particle node.
type OccurrenceAttrs struct {
	MinOccurs    string
	MaxOccurs    string
	HasMinOccurs bool
	HasMaxOccurs bool
}

// ParseOccurrence parses minOccurs/maxOccurs and applies compile-time finite
// occurrence limits.
func ParseOccurrence(attrs OccurrenceAttrs, limits Limits) (Occurrence, error) {
	minOccurs, err := parseMinOccurrence(attrs)
	if err != nil {
		return Occurrence{}, err
	}
	maximum, err := parseMaxOccurrence(attrs, limits.MaxFiniteOccurs)
	if err != nil {
		return Occurrence{}, err
	}
	return maximum.withMinimum(minOccurs)
}

func (m maxOccurrence) withMinimum(minOccurs uint32) (Occurrence, error) {
	switch m.kind {
	case maxOccurrenceUnbounded:
		return Occurrence{Min: minOccurs, Unbounded: true}, nil
	case maxOccurrenceFinite:
	case maxOccurrenceInvalid:
		return Occurrence{}, xsderrors.InternalInvariant("maxOccurs parser returned an invalid kind")
	default:
		return Occurrence{}, xsderrors.InternalInvariant("maxOccurs parser returned an unknown kind")
	}
	if m.value < minOccurs {
		return Occurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "maxOccurs is less than minOccurs")
	}
	return Occurrence{Min: minOccurs, Max: m.value}, nil
}

func parseMinOccurrence(attrs OccurrenceAttrs) (uint32, error) {
	if !attrs.HasMinOccurs {
		return 1, nil
	}
	number, err := parseOccurrenceInteger(attrs.MinOccurs)
	if err != nil {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "invalid minOccurs "+attrs.MinOccurs)
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

func parseMaxOccurrence(attrs OccurrenceAttrs, limit uint64) (maxOccurrence, error) {
	if !attrs.HasMaxOccurs {
		return maxOccurrence{value: 1, kind: maxOccurrenceFinite}, nil
	}
	if lex.TrimXMLWhitespaceString(attrs.MaxOccurs) == occurrenceUnboundedLexical {
		return maxOccurrence{kind: maxOccurrenceUnbounded}, nil
	}
	number, err := parseOccurrenceInteger(attrs.MaxOccurs)
	if err != nil {
		return maxOccurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "invalid maxOccurs "+attrs.MaxOccurs)
	}
	if maxOccursLimitExceeded(number, limit) {
		return maxOccurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, maxOccursLimitMessage(limit))
	}
	return maxOccurrence{value: uint32(number), kind: maxOccurrenceFinite}, nil // #nosec G115 -- maxOccursLimitExceeded enforces the uint32 cap.
}

// ValidateAllModelOccurrence validates xs:all model group occurrence admission.
func ValidateAllModelOccurrence(occurs Occurrence) error {
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

func parseOccurrenceInteger(v string) (uint64, error) {
	v = lex.TrimXMLWhitespaceString(v)
	if strings.HasPrefix(v, "-") {
		return 0, strconv.ErrSyntax
	}
	return parseSchemaNonNegativeInteger(v)
}
