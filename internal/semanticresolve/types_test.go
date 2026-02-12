package semanticresolve

import (
	"testing"

	parser "github.com/jacoelho/xsd/internal/parser"
	model "github.com/jacoelho/xsd/internal/types"
)

func TestValidateTypeReferenceFromTypeNil(t *testing.T) {
	schema := &parser.Schema{}
	if err := validateTypeReferenceFromTypeAtLocation(schema, nil, model.NamespaceURI(""), noOriginLocation); err != nil {
		t.Fatalf("expected nil error for nil type, got %v", err)
	}
}

func TestValidateSimpleTypeFinals(t *testing.T) {
	schema := parser.NewSchema()
	qname := model.QName{Namespace: "urn:test", Local: "base"}
	st := &model.SimpleType{QName: qname}
	schema.TypeDefs[qname] = st

	cases := []struct {
		name    string
		final   model.DerivationMethod
		method  model.DerivationMethod
		errFmt  string
		wantErr bool
	}{
		{
			name:    "restriction",
			final:   model.DerivationRestriction,
			method:  model.DerivationRestriction,
			errFmt:  "cannot derive by restriction from type '%s' which is final for restriction",
			wantErr: true,
		},
		{
			name:    "list",
			final:   model.DerivationList,
			method:  model.DerivationList,
			errFmt:  "cannot use type '%s' as list item type because it is final for list",
			wantErr: true,
		},
		{
			name:    "union",
			final:   model.DerivationUnion,
			method:  model.DerivationUnion,
			errFmt:  "cannot use type '%s' as union member type because it is final for union",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st.Final = model.DerivationSet(tc.final)
			err := validateSimpleTypeFinal(schema, qname, tc.method, tc.errFmt)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	st.Final = 0
	if err := validateSimpleTypeFinal(schema, qname, model.DerivationRestriction, "cannot derive by restriction from type '%s' which is final for restriction"); err != nil {
		t.Fatalf("unexpected error when final is empty: %v", err)
	}
}
