package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func testSimpleType(variety value.Variety, base SimpleTypeID, members ...SimpleTypeID) simpleType {
	spec := value.TypeSpec{Variety: variety, Base: base}
	switch variety {
	case value.Atomic:
	case value.List:
		if len(members) != 0 {
			spec.ListItem = members[0]
		}
	case value.Union:
		spec.Union = append([]SimpleTypeID(nil), members...)
	}
	return simpleType{ValueSpec: spec}
}

func TestCheckSimpleRestrictionBase(t *testing.T) {
	t.Parallel()

	if err := checkSimpleRestrictionBase(1, 0); err != nil {
		t.Fatalf("CheckSimpleRestrictionBase(non-anySimpleType) error = %v", err)
	}
	err := checkSimpleRestrictionBase(1, 1)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, "simple type cannot restrict xs:anySimpleType")
}

func TestCheckSimpleTypeFinalAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		final      DerivationMask
		derivation DerivationMask
		role       simpleTypeFinalRole
		msg        string
	}{
		{
			name:       "restriction",
			final:      DerivationRestriction,
			derivation: DerivationRestriction,
			role:       simpleTypeFinalBaseRestriction,
			msg:        "base simple type final blocks restriction",
		},
		{
			name:       "list",
			final:      derivationList,
			derivation: derivationList,
			role:       simpleTypeFinalListItem,
			msg:        "item simple type final blocks list",
		},
		{
			name:       "union",
			final:      derivationUnion,
			derivation: derivationUnion,
			role:       simpleTypeFinalUnionMember,
			msg:        "member simple type final blocks union",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := checkSimpleTypeFinalAllows(0, tt.derivation, tt.role); err != nil {
				t.Fatalf("CheckSimpleTypeFinalAllows(allowed) error = %v", err)
			}
			err := checkSimpleTypeFinalAllows(tt.final, tt.derivation, tt.role)
			expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, tt.msg)
		})
	}
}

func expectCompileDiagnostic(t *testing.T, err error, code xsderrors.Code, message string) {
	t.Helper()

	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category() != xsderrors.CategorySchemaCompile || xerr.Code() != code {
		t.Fatalf("diagnostic = %s/%s, want schema compile/%s", xerr.Category(), xerr.Code(), code)
	}
	if xerr.Message() != message {
		t.Fatalf("message = %q, want %q", xerr.Message(), message)
	}
}
