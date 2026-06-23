package compile

import (
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
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
func ParseOccurrence(attrs OccurrenceAttrs, limits Limits) (runtime.Occurrence, error) {
	minOccurs := uint32(1)
	minDigits := "1"
	if attrs.HasMinOccurs {
		digits, err := parseOccurrenceDigits(attrs.MinOccurs)
		if err != nil {
			return runtime.Occurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "invalid minOccurs "+attrs.MinOccurs)
		}
		minDigits = digits
		if occurrenceUint32LimitExceeded(digits) {
			return runtime.Occurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "minOccurs exceeds uint32 limit")
		}
		minOccurs = occurrenceUint32(digits)
	}
	maxOccurs := uint32(1)
	maxDigits := "1"
	if attrs.HasMaxOccurs {
		if lex.TrimXMLWhitespaceString(attrs.MaxOccurs) == occurrenceUnboundedLexical {
			return runtime.Occurrence{Min: minOccurs, Unbounded: true}, nil
		}
		digits, err := parseOccurrenceDigits(attrs.MaxOccurs)
		if err != nil {
			return runtime.Occurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "invalid maxOccurs "+attrs.MaxOccurs)
		}
		if maxOccursLimitExceeded(digits, limits.MaxFiniteOccurs) {
			return runtime.Occurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, maxOccursLimitMessage(limits.MaxFiniteOccurs))
		}
		maxDigits = digits
		maxOccurs = occurrenceUint32(digits)
	}
	if compareUnsignedDecimalText(maxDigits, minDigits) < 0 {
		return runtime.Occurrence{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "maxOccurs is less than minOccurs")
	}
	return runtime.Occurrence{Min: minOccurs, Max: maxOccurs}, nil
}

// ValidateAllModelOccurrence validates xs:all model group occurrence admission.
func ValidateAllModelOccurrence(occurs runtime.Occurrence) error {
	if occurs.Unbounded || occurs.Min > 1 || occurs.Max != 1 {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaOccurrence, "xs:all occurrence must be zero or one")
	}
	return nil
}

func maxOccursLimitExceeded(digits string, limit uint64) bool {
	limitCap := maxUint32Value
	if limit != 0 && limit < limitCap {
		limitCap = limit
	}
	return compareUnsignedDecimalText(digits, strconv.FormatUint(limitCap, 10)) > 0
}

func maxOccursLimitMessage(limit uint64) string {
	if limit != 0 && limit < maxUint32Value {
		return "maxOccurs exceeds configured limit"
	}
	return "maxOccurs exceeds uint32 limit"
}

// occurrenceUint32LimitExceeded compares textually so huge values cannot overflow.
func occurrenceUint32LimitExceeded(digits string) bool {
	return compareUnsignedDecimalText(digits, MaxUint32Text) > 0
}

func parseOccurrenceDigits(v string) (string, error) {
	v = lex.TrimXMLWhitespaceString(v)
	v = strings.TrimPrefix(v, "+")
	if v == "" {
		return "", strconv.ErrSyntax
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return "", strconv.ErrSyntax
		}
	}
	v = strings.TrimLeft(v, "0")
	if v == "" {
		return "0", nil
	}
	return v, nil
}

func occurrenceUint32(digits string) uint32 {
	if compareUnsignedDecimalText(digits, MaxUint32Text) > 0 {
		return uint32(maxUint32Value)
	}
	v, err := strconv.ParseUint(digits, 10, 32)
	if err != nil {
		return uint32(maxUint32Value)
	}
	return uint32(v)
}

func compareUnsignedDecimalText(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
