package compile

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
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
		complex: map[runtime.ComplexTypeID]runtime.ComplexTypeDerivation{
			0: {},
			1: {Base: runtime.ComplexRef(0), Kind: runtime.DerivationKindExtension},
			2: {},
		},
	}
	head := runtime.ElementDecl{Type: runtime.ComplexRef(0)}

	tests := []struct {
		name    string
		head    runtime.ElementDecl
		member  runtime.ElementDecl
		message string
	}{
		{
			name:    "not derived",
			head:    head,
			member:  runtime.ElementDecl{Type: runtime.ComplexRef(2)},
			message: "substitution group member m type memberType is not derived from head h type headType",
		},
		{
			name: "excluded derivation",
			head: func() runtime.ElementDecl {
				h := head
				h.Final = runtime.DerivationExtension
				return h
			}(),
			member:  runtime.ElementDecl{Type: runtime.ComplexRef(1)},
			message: "substitution group member type uses excluded derivation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSubstitutionMembership(rt, tt.head, tt.member, labels)
			xerr, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("ValidateSubstitutionMembership() error = %T %v, want *xsderrors.Error", err, err)
			}
			if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaReference {
				t.Fatalf("diagnostic = %s/%s, want schema compile reference", xerr.Category, xerr.Code)
			}
			if !strings.Contains(xerr.Message, tt.message) {
				t.Fatalf("message = %q, want %q", xerr.Message, tt.message)
			}
		})
	}
}

type substitutionMembershipRuntime struct {
	simple  map[runtime.SimpleTypeID]runtime.SimpleTypeDerivation
	complex map[runtime.ComplexTypeID]runtime.ComplexTypeDerivation
}

func (s substitutionMembershipRuntime) AnyTypeID() runtime.ComplexTypeID {
	return 0
}

func (s substitutionMembershipRuntime) ComplexTypeCount() int {
	return len(s.complex)
}

func (s substitutionMembershipRuntime) SimpleTypeCount() int {
	return len(s.simple)
}

func (s substitutionMembershipRuntime) SimpleTypeDerivation(id runtime.SimpleTypeID) (runtime.SimpleTypeDerivation, bool) {
	derivation, ok := s.simple[id]
	return derivation, ok
}

func (s substitutionMembershipRuntime) ComplexTypeDerivation(id runtime.ComplexTypeID) (runtime.ComplexTypeDerivation, bool) {
	derivation, ok := s.complex[id]
	return derivation, ok
}
