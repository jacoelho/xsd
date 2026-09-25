package value

import (
	"errors"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	rawDecimalErrInvalidDecimal = "invalid decimal"
	rawDecimalErrInvalidInteger = "invalid integer"
	rawDecimalErrMinInclusive   = "minInclusive facet failed"
	rawDecimalErrMaxInclusive   = "maxInclusive facet failed"
)

// BuiltinDerivedInput is the projection needed to validate built-in simple
// types whose lexical rules are layered on top of a primitive datatype.
type BuiltinDerivedInput struct {
	Norm string
	Kind BuiltinKind
}

func skipLeadingZeros[T byteText](s T, start, end int) int {
	for start < end && s[start] == '0' {
		start++
	}
	return start
}

// ValidateBuiltinDerived validates lexical rules attached to built-in simple
// types. Primitive parsing remains caller-owned until the datatype engine moves
// behind the runtime boundary.
func ValidateBuiltinDerived(in BuiltinDerivedInput) error {
	switch in.Kind {
	case BuiltinInteger:
		return ValidateIntegerLexical(in.Norm)
	case BuiltinName:
		return validateXMLNameLexical(in.Norm)
	case BuiltinNCName:
		return validateNCNameLexical(in.Norm)
	case BuiltinEntity:
		return validateEntityLexical(in.Norm)
	case BuiltinNMTOKEN:
		return validateNMTOKENLexical(in.Norm)
	case BuiltinLanguage:
		return validateLanguageLexical(in.Norm)
	case BuiltinXMLLang:
		return validateXMLLangLexical(in.Norm)
	case BuiltinXMLSpace:
		return validateXMLSpaceLexical(in.Norm)
	case BuiltinNone:
		return nil
	default:
	}
	return nil
}

func validateXMLNameLexical(normalized string) error {
	if !lex.IsXMLName(normalized) {
		return errors.New("invalid Name")
	}
	return nil
}

func validateNCNameLexical(normalized string) error {
	if !lex.IsNCName(normalized) {
		return errors.New("invalid NCName")
	}
	return nil
}

func validateNMTOKENLexical(normalized string) error {
	if !lex.IsNMTOKEN(normalized) {
		return errors.New("invalid NMTOKEN")
	}
	return nil
}

func validateLanguageLexical(normalized string) error {
	if !lex.IsLanguage(normalized) {
		return errors.New("invalid language")
	}
	return nil
}

func validateXMLLangLexical(normalized string) error {
	if normalized != "" {
		return validateLanguageLexical(normalized)
	}
	return nil
}

func validateXMLSpaceLexical(normalized string) error {
	if normalized != vocab.XMLValueDefault && normalized != vocab.XMLValuePreserve {
		return errors.New("invalid xml:space")
	}
	return nil
}

func validateEntityLexical(normalized string) error {
	if err := validateNCNameLexical(normalized); err != nil {
		return err
	}
	return xsderrors.Unsupported(xsderrors.CodeUnsupportedEntity, "ENTITY requires DTD entity declarations, which are not supported", nil)
}
