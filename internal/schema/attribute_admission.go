package schema

import (
	"errors"

	"github.com/jacoelho/xsd/internal/diagnostictext"
)

// attributeUseMode identifies the lexical xs:attribute use mode.
type attributeUseMode uint8

const (
	// attributeUseOptional is the default attribute-use mode.
	attributeUseOptional attributeUseMode = iota
	// attributeUseRequired records use="required".
	attributeUseRequired
	// attributeUseProhibited records use="prohibited".
	attributeUseProhibited
)

const (
	attributeUseOptionalLexical   = "optional"
	attributeUseRequiredLexical   = "required"
	attributeUseProhibitedLexical = "prohibited"
)

// attributeUseValueConstraintAdmission is the compile-time projection needed to
// validate an attribute use's default against the declaration it references.
type attributeUseValueConstraintAdmission struct {
	HasDefault             bool
	ReferencedDeclHasFixed bool
}

// attributeUseFixedValueAdmission is the compile-time projection needed to
// validate fixed-value preservation against a referenced declaration.
type attributeUseFixedValueAdmission struct {
	Fixed               constraintIdentity
	ReferencedDeclFixed constraintIdentity
}

// parseAttributeUseMode parses an xs:attribute use value.
func parseAttributeUseMode(mode string) (attributeUseMode, error) {
	switch mode {
	case attributeUseOptionalLexical:
		return attributeUseOptional, nil
	case attributeUseRequiredLexical:
		return attributeUseRequired, nil
	case attributeUseProhibitedLexical:
		return attributeUseProhibited, nil
	default:
		return attributeUseOptional, errors.New("invalid attribute use " + diagnostictext.Quote(mode))
	}
}

// checkAttributeUseDefault returns the error for an xs:attribute that has a
// default and the use mode, or nil: a default requires use to be optional
// (src-attribute.2). Start-tag admission calls it.
func checkAttributeUseDefault(mode attributeUseMode) error {
	switch mode {
	case attributeUseOptional:
		return nil
	case attributeUseRequired:
		return errors.New("required attribute cannot have default")
	case attributeUseProhibited:
		return errors.New("prohibited attribute cannot have default")
	}
	return newBuildDefect("attribute use mode is invalid")
}

// validateAttributeUseValueConstraintAdmission rejects the default of an
// attribute use whose referenced declaration is fixed.
func validateAttributeUseValueConstraintAdmission(admission attributeUseValueConstraintAdmission) error {
	if admission.HasDefault && admission.ReferencedDeclHasFixed {
		return errors.New("attribute use default conflicts with fixed attribute declaration")
	}
	return nil
}

// validateAttributeUseFixedValueAdmission validates fixed-value preservation
// for an attribute use that references a fixed declaration.
func validateAttributeUseFixedValueAdmission(admission attributeUseFixedValueAdmission) error {
	if admission.ReferencedDeclFixed.Present &&
		admission.Fixed.Present &&
		!fixedValueConstraintEqual(admission.ReferencedDeclFixed, admission.Fixed) {
		return errors.New("attribute use fixed value conflicts with fixed attribute declaration")
	}
	return nil
}
