package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestSubstitutionMembershipDiagnosticMapsRejectedEdges(t *testing.T) {
	t.Parallel()

	labels := SubstitutionMembershipLabels{
		MemberName: "m",
		MemberType: "memberType",
		HeadName:   "h",
		HeadType:   "headType",
	}
	rt := substitutionMembershipFixture{
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
		cause   error
	}{
		{
			name:    "not derived",
			head:    head,
			member:  ElementDecl{Type: ComplexRef(2)},
			message: "substitution group member m type memberType is not derived from head h type headType",
			cause:   ErrSubstitutionMemberTypeNotDerived,
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
			cause:   ErrSubstitutionMemberTypeExcludedDerivation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, edgeErr := validateSubstitutionEdge(rt.build(), []ElementDecl{tt.head, tt.member}, 1, 0, unboundedTypeDerivationWork)
			membership, ok := errors.AsType[SubstitutionMembershipError](edgeErr)
			if !errors.Is(edgeErr, tt.cause) || !ok || !errors.Is(membership.Cause, tt.cause) || membership.Member != 1 || membership.Head != 0 {
				t.Fatalf("edge error = %#v, want %v for member1/head0", edgeErr, tt.cause)
			}
			err := substitutionMembershipDiagnostic(membership.Cause, labels)
			xerr, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("substitutionMembershipDiagnostic() error = %T %v, want *xsderrors.Error", err, err)
			}
			if xerr.Category() != xsderrors.CategorySchemaCompile || xerr.Code() != xsderrors.CodeSchemaReference {
				t.Fatalf("diagnostic = %s/%s, want schema compile reference", xerr.Category(), xerr.Code())
			}
			if xerr.Message() != tt.message {
				t.Fatalf("message = %q, want %q", xerr.Message(), tt.message)
			}
		})
	}
}

type substitutionMembershipFixture struct {
	simple  map[SimpleTypeID]SimpleTypeDerivation
	complex map[ComplexTypeID]ComplexTypeDerivation
}

func (f substitutionMembershipFixture) build() *schemaBuild {
	return (&derivationFixture{simple: fixtureMapSlice(f.simple), complex: fixtureMapSlice(f.complex)}).build()
}
