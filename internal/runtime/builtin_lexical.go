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
	if len(s) == 0 {
		return errors.New(fastIntErrInvalidDecimal)
	}
	start := 0
	negative := false
	if s[0] == '+' || s[0] == '-' {
		negative = s[0] == '-'
		start = 1
	}
	if start == len(s) {
		return errors.New(fastIntErrInvalidDecimal)
	}
	digits := 0
	dot := false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '.':
			if dot {
				return errors.New(fastIntErrInvalidDecimal)
			}
			dot = true
		default:
			return errors.New(fastIntErrInvalidDecimal)
		}
	}
	if digits == 0 {
		return errors.New(fastIntErrInvalidDecimal)
	}
	if dot {
		return errors.New(fastIntErrInvalidInteger)
	}

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
	case BuiltinValidationNone:
		return nil
	case BuiltinValidationInteger:
		return ValidateIntegerLexical(in.Norm)
	case BuiltinValidationName:
		if !lex.IsXMLName(in.Norm) {
			return fmt.Errorf("invalid Name")
		}
	case BuiltinValidationNCName:
		if !lex.IsNCName(in.Norm) {
			return fmt.Errorf("invalid NCName")
		}
	case BuiltinValidationEntity:
		if !lex.IsNCName(in.Norm) {
			return fmt.Errorf("invalid NCName")
		}
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedEntity, "ENTITY requires DTD entity declarations, which are not supported")
	case BuiltinValidationNMTOKEN:
		if !lex.IsNMTOKEN(in.Norm) {
			return fmt.Errorf("invalid NMTOKEN")
		}
	case BuiltinValidationLanguage:
		if !lex.IsLanguage(in.Norm) {
			return fmt.Errorf("invalid language")
		}
	case BuiltinValidationXMLLang:
		if in.Norm != "" && !lex.IsLanguage(in.Norm) {
			return fmt.Errorf("invalid language")
		}
	case BuiltinValidationXMLSpace:
		if in.Norm != vocab.XMLValueDefault && in.Norm != vocab.XMLValuePreserve {
			return fmt.Errorf("invalid xml:space")
		}
	}
	return nil
}
