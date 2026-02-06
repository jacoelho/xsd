package semanticresolve

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestValidateTypeReferenceFromTypeNil(t *testing.T) {
	schema := &parser.Schema{}
	if err := validateTypeReferenceFromType(schema, nil, types.NamespaceURI("")); err != nil {
		t.Fatalf("expected nil error for nil type, got %v", err)
	}
}

func TestValidateSimpleTypeFinals(t *testing.T) {
	schema := parser.NewSchema()
	qname := types.QName{Namespace: "urn:test", Local: "base"}
	st := &types.SimpleType{QName: qname}
	schema.TypeDefs[qname] = st

	cases := []struct {
		name    string
		final   types.DerivationMethod
		fn      func(*parser.Schema, types.QName) error
		wantErr bool
	}{
		{
			name:    "restriction",
			final:   types.DerivationRestriction,
			fn:      validateSimpleTypeFinalRestriction,
			wantErr: true,
		},
		{
			name:    "list",
			final:   types.DerivationList,
			fn:      validateSimpleTypeFinalList,
			wantErr: true,
		},
		{
			name:    "union",
			final:   types.DerivationUnion,
			fn:      validateSimpleTypeFinalUnion,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st.Final = types.DerivationSet(tc.final)
			err := tc.fn(schema, qname)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	st.Final = 0
	if err := validateSimpleTypeFinalRestriction(schema, qname); err != nil {
		t.Fatalf("unexpected error when final is empty: %v", err)
	}
}
