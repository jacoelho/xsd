package runtime

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	fastIntErrInvalidDecimal = "invalid decimal"
	fastIntErrInvalidInteger = "invalid integer"
	fastIntErrMinInclusive   = "minInclusive facet failed"
	fastIntErrMaxInclusive   = "maxInclusive facet failed"
)

type byteText interface {
	~string | ~[]byte
}

// BuiltinDerivedInput is the projection needed to validate built-in simple
// types whose lexical rules are layered on top of a primitive datatype.
type BuiltinDerivedInput struct {
	Norm string
	Kind BuiltinValidationKind
}

// ValidateFastIntLexical validates the stored xs:int fast path. The fast path
// is admitted only after runtime metadata proves the fixed xs:int facet shape.
func ValidateFastIntLexical[T byteText](s T) error {
	scan, err := scanDecimalText(s)
	if err != nil {
		return errors.New(fastIntErrInvalidDecimal)
	}
	if scan.dot {
		return errors.New(fastIntErrInvalidInteger)
	}
	return validateFastIntBounds(s, scan.start, scan.negative)
}

func validateFastIntBounds[T byteText](s T, start int, negative bool) error {
	digitStart := skipLeadingZeros(s, start, len(s))
	if digitStart == len(s) {
		return nil
	}
	limit := "2147483647"
	if negative {
		limit = "2147483648"
	}
	digitCount := len(s) - digitStart
	if digitCount > len(limit) || digitCount == len(limit) && digitsGreaterThan(s, digitStart, limit) {
		if negative {
			return errors.New(fastIntErrMinInclusive)
		}
		return errors.New(fastIntErrMaxInclusive)
	}
	return nil
}

func skipLeadingZeros[T byteText](s T, start, end int) int {
	for start < end && s[start] == '0' {
		start++
	}
	return start
}

func digitsGreaterThan[T byteText](s T, start int, limit string) bool {
	for i := range limit {
		if s[start+i] != limit[i] {
			return s[start+i] > limit[i]
		}
	}
	return false
}

// ValidateBuiltinDerived validates lexical rules attached to built-in simple
// types. Primitive parsing remains caller-owned until the datatype engine moves
// behind the runtime boundary.
func ValidateBuiltinDerived(in BuiltinDerivedInput) error {
	switch in.Kind {
	case BuiltinValidationInteger:
		return ValidateIntegerLexical(in.Norm)
	case BuiltinValidationName:
		return lexicalValidation(lex.IsXMLName(in.Norm), "invalid Name")
	case BuiltinValidationNCName:
		return lexicalValidation(lex.IsNCName(in.Norm), "invalid NCName")
	case BuiltinValidationEntity:
		return validateEntityLexical(in.Norm)
	case BuiltinValidationNMTOKEN:
		return lexicalValidation(lex.IsNMTOKEN(in.Norm), "invalid NMTOKEN")
	case BuiltinValidationLanguage:
		return lexicalValidation(lex.IsLanguage(in.Norm), "invalid language")
	case BuiltinValidationXMLLang:
		return lexicalValidation(in.Norm == "" || lex.IsLanguage(in.Norm), "invalid language")
	case BuiltinValidationXMLSpace:
		return lexicalValidation(in.Norm == vocab.XMLValueDefault || in.Norm == vocab.XMLValuePreserve, "invalid xml:space")
	default:
		return nil
	}
}

func lexicalValidation(valid bool, message string) error {
	if !valid {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func validateEntityLexical(normalized string) error {
	if err := lexicalValidation(lex.IsNCName(normalized), "invalid NCName"); err != nil {
		return err
	}
	return xsderrors.Unsupported(xsderrors.CodeUnsupportedEntity, "ENTITY requires DTD entity declarations, which are not supported", nil)
}
