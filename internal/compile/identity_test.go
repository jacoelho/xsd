package compile

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateIdentityConstraintNameSource(t *testing.T) {
	t.Parallel()

	if err := ValidateIdentityConstraintNameSource(true); err != nil {
		t.Fatalf("ValidateIdentityConstraintNameSource(true) error = %v", err)
	}
	err := ValidateIdentityConstraintNameSource(false)
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

			err := ValidateIdentityConstraintReferSource(tt.local, tt.hasRefer)
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
		want        runtime.IdentityKind
		wantMessage string
	}{
		{name: "key", local: vocab.XSDElemKey, want: runtime.IdentityKey},
		{name: "unique", local: vocab.XSDElemUnique, want: runtime.IdentityUnique},
		{name: "keyref", local: vocab.XSDElemKeyref, want: runtime.IdentityKeyRef},
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

	name := runtime.QName{Namespace: 2, Local: 3}
	if err := CheckIdentityConstraintNameAvailable(map[runtime.QName]runtime.IdentityConstraintID{}, name, "p:k"); err != nil {
		t.Fatalf("CheckIdentityConstraintNameAvailable(absent) error = %v", err)
	}

	err := CheckIdentityConstraintNameAvailable(
		map[runtime.QName]runtime.IdentityConstraintID{name: 4},
		name,
		"p:k",
	)
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("CheckIdentityConstraintNameAvailable(duplicate) error = %T %v, want *xsderrors.Error", err, err)
	}
	if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != xsderrors.CodeSchemaDuplicate {
		t.Fatalf("diagnostic = %s/%s, want schema compile duplicate", diag.Category, diag.Code)
	}
	if diag.Message != "duplicate identity constraint p:k" {
		t.Fatalf("message = %q, want duplicate identity constraint label", diag.Message)
	}
}

func TestResolveIdentityConstraintRefer(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: 2, Local: 3}
	identities := map[runtime.QName]runtime.IdentityConstraintID{name: 4}
	id, err := ResolveIdentityConstraintRefer(identities, name, "p:k")
	if err != nil {
		t.Fatalf("ResolveIdentityConstraintRefer(existing) error = %v", err)
	}
	if id != 4 {
		t.Fatalf("ResolveIdentityConstraintRefer(existing) = %d, want 4", id)
	}

	missing := runtime.QName{Namespace: 2, Local: 5}
	id, err = ResolveIdentityConstraintRefer(identities, missing, "p:missing")
	if id != runtime.NoIdentityConstraint {
		t.Fatalf("ResolveIdentityConstraintRefer(missing) id = %d, want NoIdentityConstraint", id)
	}
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("ResolveIdentityConstraintRefer(missing) error = %T %v, want *xsderrors.Error", err, err)
	}
	if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != xsderrors.CodeSchemaReference {
		t.Fatalf("diagnostic = %s/%s, want schema compile reference", diag.Category, diag.Code)
	}
	if diag.Message != "unknown keyref refer p:missing" {
		t.Fatalf("message = %q, want unknown keyref refer label", diag.Message)
	}
}

func TestValidateIdentityReferences(t *testing.T) {
	t.Parallel()

	field := runtime.IdentityField{Paths: []runtime.IdentityFieldPath{{Self: true, Attribute: runtime.NoQName()}}}
	twoFields := []runtime.IdentityField{field, field}

	tests := []struct {
		name       string
		identities []runtime.IdentityConstraint
		category   xsderrors.Category
		code       xsderrors.Code
	}{
		{
			name: "keyref references key",
			identities: []runtime.IdentityConstraint{
				{Kind: runtime.IdentityKey, Fields: []runtime.IdentityField{field}},
				{Kind: runtime.IdentityKeyRef, Refer: 0, Fields: []runtime.IdentityField{field}},
			},
		},
		{
			name: "keyref references unique",
			identities: []runtime.IdentityConstraint{
				{Kind: runtime.IdentityUnique, Fields: []runtime.IdentityField{field}},
				{Kind: runtime.IdentityKeyRef, Refer: 0, Fields: []runtime.IdentityField{field}},
			},
		},
		{
			name: "non-keyref ignores refer",
			identities: []runtime.IdentityConstraint{
				{Kind: runtime.IdentityKey, Refer: 99, Fields: []runtime.IdentityField{field}},
			},
		},
		{
			name: "keyref cannot reference keyref",
			identities: []runtime.IdentityConstraint{
				{Kind: runtime.IdentityKey, Fields: []runtime.IdentityField{field}},
				{Kind: runtime.IdentityKeyRef, Refer: 0, Fields: []runtime.IdentityField{field}},
				{Kind: runtime.IdentityKeyRef, Refer: 1, Fields: []runtime.IdentityField{field}},
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "keyref field count must match referenced key",
			identities: []runtime.IdentityConstraint{
				{Kind: runtime.IdentityKey, Fields: []runtime.IdentityField{field}},
				{Kind: runtime.IdentityKeyRef, Refer: 0, Fields: twoFields},
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid refer is invariant failure",
			identities: []runtime.IdentityConstraint{
				{Kind: runtime.IdentityKeyRef, Refer: 9, Fields: []runtime.IdentityField{field}},
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
	if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != xsderrors.CodeSchemaIdentity || diag.Message != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)",
			diag.Category, diag.Code, diag.Message,
			xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaIdentity, message)
	}
}
