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

// ElementLabelFunc returns a formatted element label for diagnostics.
type ElementLabelFunc func(runtime.ElementID) (string, bool)

// ValidateSubstitutionMembership validates one declared substitution member and
// maps runtime rejection reasons to schema diagnostics.
func ValidateSubstitutionMembership(
	rt runtime.TypeDerivationRuntime,
	head, member runtime.ElementDecl,
	labels SubstitutionMembershipLabels,
) error {
	err := runtime.ValidateSubstitutionMembership(rt, head, member)
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

// BuildSubstitutionClosure delegates closure construction to runtime and maps
// construction failures to schema diagnostics.
func BuildSubstitutionClosure(direct map[runtime.ElementID][]runtime.ElementID, label ElementLabelFunc) (map[runtime.ElementID][]runtime.ElementID, error) {
	closure, err := runtime.BuildSubstitutionClosure(direct)
	if err == nil {
		return closure, nil
	}
	if cycle, ok := errors.AsType[runtime.SubstitutionCycleError](err); ok && label != nil {
		if name, ok := label(cycle.Element); ok {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "cyclic substitution group "+name)
		}
	}
	return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, err.Error())
}
