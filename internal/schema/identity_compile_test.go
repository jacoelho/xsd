package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

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
		{name: "invalid", local: "selector", wantMessage: "invalid identity constraint \"selector\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := identityConstraintKindForLocal(tt.local)
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
	if err := checkIdentityConstraintNameAvailable(map[QName]IdentityConstraintID{}, name, "p:k"); err != nil {
		t.Fatalf("CheckIdentityConstraintNameAvailable(absent) error = %v", err)
	}

	err := checkIdentityConstraintNameAvailable(
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
	id, err := resolveIdentityConstraintRefer(identities, name, "p:k")
	if err != nil {
		t.Fatalf("ResolveIdentityConstraintRefer(existing) error = %v", err)
	}
	if id != 4 {
		t.Fatalf("ResolveIdentityConstraintRefer(existing) = %d, want 4", id)
	}

	missing := QName{Namespace: 2, Local: 5}
	id, err = resolveIdentityConstraintRefer(identities, missing, "p:missing")
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

	field := identityField{Paths: []identityFieldPath{{Attribute: noQName()}}}
	twoFields := []identityField{field, field}

	tests := []struct {
		name       string
		identities []identityConstraintSpec
		category   xsderrors.Category
		code       xsderrors.Code
	}{
		{
			name: "keyref references key",
			identities: []identityConstraintSpec{
				{Kind: IdentityKey, Fields: []identityField{field}},
				{Kind: IdentityKeyRef, Refer: 0, Fields: []identityField{field}},
			},
		},
		{
			name: "keyref references unique",
			identities: []identityConstraintSpec{
				{Kind: IdentityUnique, Fields: []identityField{field}},
				{Kind: IdentityKeyRef, Refer: 0, Fields: []identityField{field}},
			},
		},
		{
			name: "non-keyref ignores refer",
			identities: []identityConstraintSpec{
				{Kind: IdentityKey, Refer: 99, Fields: []identityField{field}},
			},
		},
		{
			name: "keyref cannot reference keyref",
			identities: []identityConstraintSpec{
				{Kind: IdentityKey, Fields: []identityField{field}},
				{Kind: IdentityKeyRef, Refer: 0, Fields: []identityField{field}},
				{Kind: IdentityKeyRef, Refer: 1, Fields: []identityField{field}},
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "keyref field count must match referenced key",
			identities: []identityConstraintSpec{
				{Kind: IdentityKey, Fields: []identityField{field}},
				{Kind: IdentityKeyRef, Refer: 0, Fields: twoFields},
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid refer is invariant failure",
			identities: []identityConstraintSpec{
				{Kind: IdentityKeyRef, Refer: 9, Fields: []identityField{field}},
			},
			category: xsderrors.CategoryInternal,
			code:     xsderrors.CodeInternalInvariant,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var err error
			for _, ic := range tt.identities {
				if err = validateKeyrefReference(tt.identities, ic); err != nil {
					break
				}
			}
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

func TestDeclaringAnIdentityConstraintTwiceInOneContextIsAnInternalInvariant(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	input := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:key name="k"><xs:selector xpath="a"/><xs:field xpath="@id"/></xs:key></xs:element></xs:schema>`
	if err = c.loadOwned([]source.Source{source.Bytes("identity.xsd", []byte(input))}); err != nil {
		t.Fatal(err)
	}
	if err = c.declareAllIdentityConstraints(); err != nil {
		t.Fatalf("first declaration: %v", err)
	}
	expectDiagnostic(t, c.declareAllIdentityConstraints(), xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}
