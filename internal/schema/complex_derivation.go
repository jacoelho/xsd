package schema

import (
	"errors"

	"github.com/jacoelho/xsd/xsderrors"
)

// complexTypeFinalRole identifies the schema component role that is applying a
// complex-type final derivation rule.
type complexTypeFinalRole uint8

const (
	// complexTypeFinalBaseExtension checks a complexContent/simpleContent
	// extension base complex type.
	complexTypeFinalBaseExtension complexTypeFinalRole = iota
	// complexTypeFinalBaseRestriction checks a complexContent/simpleContent
	// restriction base complex type.
	complexTypeFinalBaseRestriction
)

// checkComplexTypeFinalAllows maps runtime complex-type final-mask rejection
// into the compile diagnostic for the schema role being derived.
func checkComplexTypeFinalAllows(final, derivation DerivationMask, role complexTypeFinalRole) error {
	if err := validateComplexTypeFinalAllows(final, derivation); err != nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, complexTypeFinalRoleMessage(role))
	}
	return nil
}

// checkSimpleBaseComplexExtensionFinalAllows maps runtime final-mask rejection
// for complex types extending a simple-type base into a schema diagnostic.
func checkSimpleBaseComplexExtensionFinalAllows(final DerivationMask) error {
	if err := validateSimpleBaseComplexExtensionFinalAllows(final); err != nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "base simple type final blocks extension")
	}
	return nil
}

// checkComplexContentRestrictionBase rejects complexContent restriction from a
// simple-content complex base.
func checkComplexContentRestrictionBase(base complexTypeMetadata) error {
	if base.SimpleContent() {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent restriction base cannot have simple content")
	}
	return nil
}

// checkSimpleContentSimpleBase rejects xs:simpleContent restriction from an
// xs:simpleType base.
func checkSimpleContentSimpleBase(kind contentDerivationKind) error {
	if kind == contentDerivationRestriction {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "simpleContent restriction base must be complex type")
	}
	return nil
}

// simpleContentComplexBaseMissingError reports a missing complex base during
// xs:simpleContent base resolution.
func simpleContentComplexBaseMissingError() error {
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "simpleContent base must be simple or simple-content complex type")
}

// checkSimpleContentDerivationBase maps runtime base admissibility into the
// schema diagnostic for xs:simpleContent derivation.
func checkSimpleContentDerivationBase(
	analysis *contentModelAnalysis,
	base complexTypeMetadata,
	derivation contentDerivationKind,
) error {
	runtimeDerivation := derivationKindExtension
	if derivation == contentDerivationRestriction {
		runtimeDerivation = derivationKindRestriction
	}
	allowed, err := simpleContentDerivationBaseAllowed(analysis, base, runtimeDerivation)
	if err != nil {
		return contentRestrictionCompileError(err)
	}
	if !allowed {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "simpleContent base must have simple content")
	}
	return nil
}

// checkSimpleContentRestrictionTextTypePresent rejects simpleContent
// restriction of mixed content when no explicit simpleType is supplied.
func checkSimpleContentRestrictionTextTypePresent(textType SimpleTypeID) error {
	if textType == NoSimpleType {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "simpleContent restriction of mixed content requires simpleType")
	}
	return nil
}

// checkSimpleContentRestrictionTextType maps runtime text-type derivation
// validation into a schema compile diagnostic.
func checkSimpleContentRestrictionTextType(
	rt *schemaBuild,
	derived, base SimpleTypeID,
	work func(int) error,
) error {
	if err := validateSimpleContentRestrictionTextType(rt, derived, base, work); err != nil {
		if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok && diagnostic != nil {
			return err
		}
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, err.Error())
	}
	return nil
}

// checkComplexContentMixedDerivationBase maps runtime mixed-base admission into
// the schema diagnostic for xs:complexContent derivation.
func checkComplexContentMixedDerivationBase(base complexTypeMetadata, derivation contentDerivationKind, content complexContentVariety) error {
	runtimeDerivation := derivationKindRestriction
	if derivation == contentDerivationExtension {
		runtimeDerivation = derivationKindExtension
	}
	if err := validateComplexContentMixedDerivationBase(base, runtimeDerivation, content); err != nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, err.Error())
	}
	return nil
}

func complexTypeFinalRoleMessage(role complexTypeFinalRole) string {
	switch role {
	case complexTypeFinalBaseExtension:
		return "base complex type final blocks extension"
	case complexTypeFinalBaseRestriction:
		return "base complex type final blocks restriction"
	default:
		return "base complex type final blocks derivation"
	}
}
