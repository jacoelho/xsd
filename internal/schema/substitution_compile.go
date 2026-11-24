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
			"substitution group member "+labels.MemberName+" "+labels.MemberType+
				" is not derived from head "+labels.HeadName+" "+labels.HeadType,
			nil,
		)
	case errors.Is(err, errSubstitutionMemberTypeExcludedDerivation):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "substitution group member type uses excluded derivation", nil)
	default:
		return compileRuleError(xsderrors.CodeSchemaReference, "", err)
	}
}
