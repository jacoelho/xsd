package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestMissingSimpleType(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: 1, Local: 2}
	const base SimpleTypeID = 3
	got := MissingSimpleType(name, base)
	if got.Name != name || !got.Missing ||
		got.ValueSpec.Variety != value.Atomic ||
		got.ValueSpec.Primitive != value.PrimitiveString ||
		got.ValueSpec.Whitespace != value.WhitespaceCollapse ||
		!got.ValueSpec.WhitespacePresent ||
		got.ValueSpec.Base != base || got.ValueSpec.ListItem != value.NoType {
		t.Fatalf("MissingSimpleType() = %+v", got)
	}
	if MissingSimpleTypeLocalName() != "missing" {
		t.Fatalf("MissingSimpleTypeLocalName() = %q", MissingSimpleTypeLocalName())
	}
}

func TestSimpleTypeByID(t *testing.T) {
	t.Parallel()

	types := []SimpleType{
		{Name: QName{Local: 1}},
		{Name: QName{Local: 2}},
	}
	got, ok := SimpleTypeByID(types, 1)
	if !ok || got.Name != types[1].Name {
		t.Fatalf("SimpleTypeByID(valid) = %+v, %v; want type 1, true", got, ok)
	}
	for _, id := range []SimpleTypeID{NoSimpleType, 2} {
		got, ok := SimpleTypeByID(types, id)
		if ok || got != nil {
			t.Fatalf("SimpleTypeByID(%d) = %+v, %v; want nil, false", id, got, ok)
		}
	}
}

func TestUsableSimpleTypeRejectsMissingSentinel(t *testing.T) {
	t.Parallel()

	types := []SimpleType{
		{Name: QName{Local: 1}},
		MissingSimpleType(QName{Local: 2}, 0),
	}
	if got, ok := UsableSimpleType(types, 0); !ok || got.Name != types[0].Name {
		t.Fatalf("UsableSimpleType(valid) = %+v, %v; want type 0, true", got, ok)
	}
	if got, ok := UsableSimpleType(types, 1); ok || got != nil {
		t.Fatalf("UsableSimpleType(missing) = %+v, %v; want nil, false", got, ok)
	}
	if got, ok := UsableSimpleType(types, NoSimpleType); ok || got != nil {
		t.Fatalf("UsableSimpleType(invalid) = %+v, %v; want nil, false", got, ok)
	}
}

func TestValidateSimpleTypeFinalAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantErr    string
		final      DerivationMask
		derivation DerivationMask
	}{
		{
			name:       "restriction allowed",
			derivation: DerivationRestriction,
		},
		{
			name:       "list allowed",
			derivation: DerivationList,
		},
		{
			name:       "union allowed",
			derivation: DerivationUnion,
		},
		{
			name:       "restriction blocked",
			final:      DerivationRestriction,
			derivation: DerivationRestriction,
			wantErr:    "simple type final blocks restriction",
		},
		{
			name:       "list blocked",
			final:      DerivationList | DerivationUnion,
			derivation: DerivationList,
			wantErr:    "simple type final blocks list",
		},
		{
			name:       "union blocked",
			final:      DerivationUnion,
			derivation: DerivationUnion,
			wantErr:    "simple type final blocks union",
		},
		{
			name:       "invalid final mask",
			final:      DerivationExtension,
			derivation: DerivationRestriction,
			wantErr:    "simple type final mask is invalid",
		},
		{
			name:       "invalid derivation",
			derivation: DerivationExtension,
			wantErr:    "simple type final derivation is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleTypeFinalAllows(tt.final, tt.derivation)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleTypeFinalAllows() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleTypeFinalAllows() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
