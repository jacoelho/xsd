package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestSubstitutionMembershipDiagnosticMapsRejectedEdges(t *testing.T) {
	t.Parallel()

	labels := substitutionMembershipLabels{
		MemberName: "m",
		MemberType: "memberType",
		HeadName:   "h",
		HeadType:   "headType",
	}
	rt := substitutionMembershipFixture{
		complex: map[ComplexTypeID]complexTypeDerivation{
			0: {},
			1: {Base: ComplexRef(0), Kind: derivationKindExtension},
			2: {},
		},
	}
	head := elementDecl{Type: ComplexRef(0)}

	tests := []struct {
		name    string
		head    elementDecl
		member  elementDecl
		message string
		cause   error
	}{
		{
			name:    "not derived",
			head:    head,
			member:  elementDecl{Type: ComplexRef(2)},
			message: "substitution group member m type memberType is not derived from head h type headType",
			cause:   errSubstitutionMemberTypeNotDerived,
		},
		{
			name: "excluded derivation",
			head: func() elementDecl {
				h := head
				h.Final = DerivationExtension
				return h
			}(),
			member:  elementDecl{Type: ComplexRef(1)},
			message: "substitution group member type uses excluded derivation",
			cause:   errSubstitutionMemberTypeExcludedDerivation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, edgeErr := validateSubstitutionEdge(rt.build(), []elementDecl{tt.head, tt.member}, 1, 0, unboundedTypeDerivationWork)
			membership, ok := errors.AsType[substitutionMembershipError](edgeErr)
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
	simple  map[SimpleTypeID]simpleTypeDerivation
	complex map[ComplexTypeID]complexTypeDerivation
}

func (f substitutionMembershipFixture) build() *schemaBuild {
	return (&derivationFixture{simple: fixtureMapSlice(f.simple), complex: fixtureMapSlice(f.complex)}).build()
}
