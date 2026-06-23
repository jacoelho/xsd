package compile

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCheckSimpleRestrictionBase(t *testing.T) {
	t.Parallel()

	if err := CheckSimpleRestrictionBase(1, 0); err != nil {
		t.Fatalf("CheckSimpleRestrictionBase(non-anySimpleType) error = %v", err)
	}
	err := CheckSimpleRestrictionBase(1, 1)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, "simple type cannot restrict xs:anySimpleType")
}

func TestCheckSimpleTypeFinalAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		final      runtime.DerivationMask
		derivation runtime.DerivationMask
		role       SimpleTypeFinalRole
		msg        string
	}{
		{
			name:       "restriction",
			final:      runtime.DerivationRestriction,
			derivation: runtime.DerivationRestriction,
			role:       SimpleTypeFinalBaseRestriction,
			msg:        "base simple type final blocks restriction",
		},
		{
			name:       "list",
			final:      runtime.DerivationList,
			derivation: runtime.DerivationList,
			role:       SimpleTypeFinalListItem,
			msg:        "item simple type final blocks list",
		},
		{
			name:       "union",
			final:      runtime.DerivationUnion,
			derivation: runtime.DerivationUnion,
			role:       SimpleTypeFinalUnionMember,
			msg:        "member simple type final blocks union",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := CheckSimpleTypeFinalAllows(0, tt.derivation, tt.role); err != nil {
				t.Fatalf("CheckSimpleTypeFinalAllows(allowed) error = %v", err)
			}
			err := CheckSimpleTypeFinalAllows(tt.final, tt.derivation, tt.role)
			expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, tt.msg)
		})
	}
}

func TestCheckSimpleListItemType(t *testing.T) {
	t.Parallel()

	types := []runtime.SimpleType{
		{Variety: runtime.SimpleVarietyAtomic, Base: runtime.NoSimpleType},
		{Variety: runtime.SimpleVarietyAtomic, Base: runtime.NoSimpleType},
		{Variety: runtime.SimpleVarietyList, Base: runtime.NoSimpleType, ListItem: 1},
	}
	if err := CheckSimpleListItemType(types, 1); err != nil {
		t.Fatalf("CheckSimpleListItemType(atomic) error = %v", err)
	}
	err := CheckSimpleListItemType(types, 2)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "list item type cannot be a list type")
}

func expectCompileDiagnostic(t *testing.T, err error, code xsderrors.Code, message string) {
	t.Helper()

	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != code {
		t.Fatalf("diagnostic = %s/%s, want schema compile/%s", xerr.Category, xerr.Code, code)
	}
	if xerr.Message != message {
		t.Fatalf("message = %q, want %q", xerr.Message, message)
	}
}
