package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateIdentityConstraintNameSource(t *testing.T) {
	t.Parallel()

	if err := ValidateIdentityConstraintNameSource(LexicalAttribute{Value: "name", Present: true}); err != nil {
		t.Fatalf("ValidateIdentityConstraintNameSource(present) error = %v", err)
	}
	err := ValidateIdentityConstraintNameSource(LexicalAttribute{})
	expectSchemaIdentityMessage(t, err, "identity constraint missing name")
}

func TestValidateIdentityConstraintReferSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		local       string
		hasRefer    bool
		wantMessage string
	}{
		{name: "keyref with refer", local: vocab.XSDElemKeyref, hasRefer: true},
		{name: "key ignores refer", local: vocab.XSDElemKey},
		{name: "unique ignores refer", local: vocab.XSDElemUnique},
		{name: "keyref missing refer", local: vocab.XSDElemKeyref, wantMessage: "keyref missing refer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateIdentityConstraintReferSource(IdentityConstraintReferSource{
				Local: tt.local,
				Refer: LexicalAttribute{Present: tt.hasRefer},
			})
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("ValidateIdentityConstraintReferSource() error = %v", err)
				}
				return
			}
			expectSchemaIdentityMessage(t, err, tt.wantMessage)
		})
	}
}

func TestIdentityConstraintKindForLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		local       string
		want        IdentityKind
		wantMessage string
	}{
		{name: "key", local: vocab.XSDElemKey, want: IdentityKey},
		{name: "unique", local: vocab.XSDElemUnique, want: IdentityUnique},
		{name: "keyref", local: vocab.XSDElemKeyref, want: IdentityKeyRef},
		{name: "invalid", local: "selector", wantMessage: "invalid identity constraint selector"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := IdentityConstraintKindForLocal(tt.local)
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("IdentityConstraintKindForLocal() error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("IdentityConstraintKindForLocal() = %v, want %v", got, tt.want)
				}
				return
			}
			expectSchemaIdentityMessage(t, err, tt.wantMessage)
		})
	}
}

func TestCheckIdentityConstraintNameAvailable(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: 2, Local: 3}
	if err := CheckIdentityConstraintNameAvailable(map[QName]IdentityConstraintID{}, name, "p:k"); err != nil {
		t.Fatalf("CheckIdentityConstraintNameAvailable(absent) error = %v", err)
	}

	err := CheckIdentityConstraintNameAvailable(
		map[QName]IdentityConstraintID{name: 4},
		name,
		"p:k",
	)
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("CheckIdentityConstraintNameAvailable(duplicate) error = %T %v, want *xsderrors.Error", err, err)
	}
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaDuplicate {
		t.Fatalf("diagnostic = %s/%s, want schema compile duplicate", diag.Category(), diag.Code())
	}
	if diag.Message() != "duplicate identity constraint p:k" {
		t.Fatalf("message = %q, want duplicate identity constraint label", diag.Message())
	}
}

func TestResolveIdentityConstraintRefer(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: 2, Local: 3}
	identities := map[QName]IdentityConstraintID{name: 4}
	id, err := ResolveIdentityConstraintRefer(identities, name, "p:k")
	if err != nil {
		t.Fatalf("ResolveIdentityConstraintRefer(existing) error = %v", err)
	}
	if id != 4 {
		t.Fatalf("ResolveIdentityConstraintRefer(existing) = %d, want 4", id)
	}

	missing := QName{Namespace: 2, Local: 5}
	id, err = ResolveIdentityConstraintRefer(identities, missing, "p:missing")
	if id != NoIdentityConstraint {
		t.Fatalf("ResolveIdentityConstraintRefer(missing) id = %d, want NoIdentityConstraint", id)
	}
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("ResolveIdentityConstraintRefer(missing) error = %T %v, want *xsderrors.Error", err, err)
	}
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaReference {
		t.Fatalf("diagnostic = %s/%s, want schema compile reference", diag.Category(), diag.Code())
	}
	if diag.Message() != "unknown keyref refer p:missing" {
		t.Fatalf("message = %q, want unknown keyref refer label", diag.Message())
	}
}

func TestValidateIdentityReferences(t *testing.T) {
	t.Parallel()

	field := IdentityField{Paths: []IdentityFieldPath{{Self: true, Attribute: NoQName()}}}
	twoFields := []IdentityField{field, field}

	tests := []struct {
		name       string
		identities []IdentityConstraint
		category   xsderrors.Category
		code       xsderrors.Code
	}{
		{
			name: "keyref references key",
			identities: []IdentityConstraint{
				{Kind: IdentityKey, Fields: []IdentityField{field}},
				{Kind: IdentityKeyRef, Refer: 0, Fields: []IdentityField{field}},
			},
		},
		{
			name: "keyref references unique",
			identities: []IdentityConstraint{
				{Kind: IdentityUnique, Fields: []IdentityField{field}},
				{Kind: IdentityKeyRef, Refer: 0, Fields: []IdentityField{field}},
			},
		},
		{
			name: "non-keyref ignores refer",
			identities: []IdentityConstraint{
				{Kind: IdentityKey, Refer: 99, Fields: []IdentityField{field}},
			},
		},
		{
			name: "keyref cannot reference keyref",
			identities: []IdentityConstraint{
				{Kind: IdentityKey, Fields: []IdentityField{field}},
				{Kind: IdentityKeyRef, Refer: 0, Fields: []IdentityField{field}},
				{Kind: IdentityKeyRef, Refer: 1, Fields: []IdentityField{field}},
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "keyref field count must match referenced key",
			identities: []IdentityConstraint{
				{Kind: IdentityKey, Fields: []IdentityField{field}},
				{Kind: IdentityKeyRef, Refer: 0, Fields: twoFields},
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid refer is invariant failure",
			identities: []IdentityConstraint{
				{Kind: IdentityKeyRef, Refer: 9, Fields: []IdentityField{field}},
			},
			category: xsderrors.CategoryInternal,
			code:     xsderrors.CodeInternalInvariant,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateIdentityReferences(tt.identities)
			if tt.code == "" {
				if err != nil {
					t.Fatalf("ValidateIdentityReferences() error = %v", err)
				}
				return
			}
			expectDiagnostic(t, err, tt.category, tt.code)
		})
	}
}

func expectSchemaIdentityMessage(t *testing.T, err error, message string) {
	t.Helper()
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want xsderrors.Error", err)
	}
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaIdentity || diag.Message() != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)",
			diag.Category(), diag.Code(), diag.Message(),
			xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaIdentity, message)
	}
}
