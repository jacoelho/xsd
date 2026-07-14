package compile

import (
	"errors"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// SubstitutionMembershipLabels carries formatted names used in compile
// diagnostics when a substitution member is rejected by runtime rules.
type SubstitutionMembershipLabels struct {
	MemberName string
	MemberType string
	HeadName   string
	HeadType   string
}

// ValidateSubstitutionMembership validates one declared substitution member and
// maps runtime rejection reasons to schema diagnostics.
func ValidateSubstitutionMembership(
	rt runtime.TypeDerivationRuntime,
	head, member runtime.ElementDecl,
	labels SubstitutionMembershipLabels,
) error {
	return substitutionMembershipDiagnostic(runtime.ValidateSubstitutionMembership(rt, head, member), labels)
}

func substitutionMembershipDiagnostic(err error, labels SubstitutionMembershipLabels) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, runtime.ErrSubstitutionMemberTypeNotDerived):
		return xsderrors.SchemaCompile(
			xsderrors.CodeSchemaReference,
			"substitution group member "+labels.MemberName+" type "+labels.MemberType+
				" is not derived from head "+labels.HeadName+" type "+labels.HeadType,
		)
	case errors.Is(err, runtime.ErrSubstitutionMemberTypeExcludedDerivation):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "substitution group member type uses excluded derivation")
	default:
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, err.Error())
	}
}
