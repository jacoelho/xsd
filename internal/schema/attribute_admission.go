package schema

import (
	"errors"

	"github.com/jacoelho/xsd/xsderrors"
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
// validate attribute-use value-constraint placement.
type attributeUseValueConstraintAdmission struct {
	Mode                   attributeUseMode
	HasDefault             bool
	HasFixed               bool
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
		return attributeUseOptional, errors.New("invalid attribute use " + mode)
	}
}

// validateAttributeUseValueConstraintAdmission validates compile-time
// default/fixed placement for one attribute use.
func validateAttributeUseValueConstraintAdmission(admission attributeUseValueConstraintAdmission) error {
	switch admission.Mode {
	case attributeUseOptional:
	case attributeUseRequired:
		if admission.HasDefault {
			return errors.New("required attribute cannot have default")
		}
	case attributeUseProhibited:
		if admission.HasDefault {
			return errors.New("prohibited attribute cannot have default")
		}
	default:
		return errors.New("attribute use mode is invalid")
	}
	if admission.HasDefault && admission.ReferencedDeclHasFixed {
		return errors.New("attribute use default conflicts with fixed attribute declaration")
	}
	if admission.HasDefault && admission.HasFixed {
		return errors.New("attribute cannot have both default and fixed")
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

// validateElementDeclValueConstraintAdmission validates compile-time
// default/fixed placement for one element declaration.
func validateElementDeclValueConstraintAdmission(constraint declarationValueConstraint) error {
	switch constraint {
	case declarationValueConstraintNone, declarationValueConstraintDefault, declarationValueConstraintFixed:
		return nil
	case declarationValueConstraintConflict:
		return errors.New("element cannot have both default and fixed")
	default:
		return xsderrors.InternalInvariant("element declaration has unknown value constraint")
	}
}

// validateAttributeDeclValueConstraintAdmission validates compile-time
// default/fixed placement for one attribute declaration.
func validateAttributeDeclValueConstraintAdmission(constraint declarationValueConstraint) error {
	switch constraint {
	case declarationValueConstraintNone, declarationValueConstraintDefault, declarationValueConstraintFixed:
		return nil
	case declarationValueConstraintConflict:
		return errors.New("attribute cannot have both default and fixed")
	default:
		return xsderrors.InternalInvariant("attribute declaration has unknown value constraint")
	}
}
