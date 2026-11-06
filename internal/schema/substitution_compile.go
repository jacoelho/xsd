package schema

import (
	"errors"

	"github.com/jacoelho/xsd/xsderrors"
)

// substitutionMembershipLabels carries formatted names used in compile
// diagnostics when a substitution member is rejected by runtime rules.
type substitutionMembershipLabels struct {
	MemberName string
	MemberType string
	HeadName   string
	HeadType   string
}

func substitutionMembershipDiagnostic(err error, labels substitutionMembershipLabels) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errSubstitutionMemberTypeNotDerived):
		return xsderrors.SchemaCompile(
			xsderrors.CodeSchemaReference,
			"substitution group member "+labels.MemberName+" type "+labels.MemberType+
				" is not derived from head "+labels.HeadName+" type "+labels.HeadType,
		)
	case errors.Is(err, errSubstitutionMemberTypeExcludedDerivation):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "substitution group member type uses excluded derivation")
	default:
		if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok && diagnostic != nil {
			return err
		}
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, err.Error())
	}
}
