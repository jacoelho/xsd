package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateSubstitutionMembershipMapsRuntimeErrors(t *testing.T) {
	t.Parallel()

	labels := SubstitutionMembershipLabels{
		MemberName: "m",
		MemberType: "memberType",
		HeadName:   "h",
		HeadType:   "headType",
	}
	rt := substitutionMembershipRuntime{
		complex: map[ComplexTypeID]ComplexTypeDerivation{
			0: {},
			1: {Base: ComplexRef(0), Kind: DerivationKindExtension},
			2: {},
		},
	}
	head := ElementDecl{Type: ComplexRef(0)}

	tests := []struct {
		name    string
		head    ElementDecl
		member  ElementDecl
		message string
	}{
		{
			name:    "not derived",
			head:    head,
			member:  ElementDecl{Type: ComplexRef(2)},
			message: "substitution group member m type memberType is not derived from head h type headType",
		},
		{
			name: "excluded derivation",
			head: func() ElementDecl {
				h := head
				h.Final = DerivationExtension
				return h
			}(),
			member:  ElementDecl{Type: ComplexRef(1)},
			message: "substitution group member type uses excluded derivation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSchemaSubstitutionMembership(rt, tt.head, tt.member, labels, unboundedTypeDerivationWork)
			xerr, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("ValidateSubstitutionMembership() error = %T %v, want *xsderrors.Error", err, err)
			}
			if xerr.Category() != xsderrors.CategorySchemaCompile || xerr.Code() != xsderrors.CodeSchemaReference {
				t.Fatalf("diagnostic = %s/%s, want schema compile reference", xerr.Category(), xerr.Code())
			}
			if !strings.Contains(xerr.Message(), tt.message) {
				t.Fatalf("message = %q, want %q", xerr.Message(), tt.message)
			}
		})
	}
}

type substitutionMembershipRuntime struct {
	simple  map[SimpleTypeID]SimpleTypeDerivation
	complex map[ComplexTypeID]ComplexTypeDerivation
}

//nolint:revive // The receiver is required to satisfy TypeDerivationRuntime.
func (s substitutionMembershipRuntime) AnyTypeID() ComplexTypeID {
	return 0
}

func (s substitutionMembershipRuntime) ComplexTypeCount() int {
	return len(s.complex)
}

func (s substitutionMembershipRuntime) SimpleTypeCount() int {
	return len(s.simple)
}

func (s substitutionMembershipRuntime) SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	derivation, ok := s.simple[id]
	return derivation, ok
}

func (s substitutionMembershipRuntime) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	derivation, ok := s.complex[id]
	return derivation, ok
}
