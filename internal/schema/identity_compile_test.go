package schema

import (
	"errors"
	"slices"
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

func identityDeclarationFixture(t *testing.T) *compiler {
	t.Helper()
	names, err := newRuntimeNameTable(0)
	if err != nil {
		t.Fatal(err)
	}
	c := &compiler{}
	c.rt.Names = names
	if _, err := c.reserveIdentity(ExpandedName{Namespace: "urn:p", Local: "x"}, nil); err != nil {
		t.Fatalf("reserveIdentity({urn:p}x) error = %v", err)
	}
	if _, err := c.reserveIdentity(ExpandedName{Namespace: "urn:p", Local: "k"}, nil); err != nil {
		t.Fatalf("reserveIdentity({urn:p}k) error = %v", err)
	}
	return c
}

func TestAnIdentityConstraintNameIsAvailableOnlyUntilItIsDeclared(t *testing.T) {
	t.Parallel()

	c := identityDeclarationFixture(t)
	for _, name := range []ExpandedName{{Namespace: "urn:q", Local: "k"}, {Namespace: "", Local: "k"}, {Namespace: "urn:p", Local: "K"}} {
		if err := c.checkIdentityConstraintNameAvailable(name); err != nil {
			t.Fatalf("checkIdentityConstraintNameAvailable(%+v) error = %v, want none", name, err)
		}
	}

	err := c.checkIdentityConstraintNameAvailable(ExpandedName{Namespace: "urn:p", Local: "k"})
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("checkIdentityConstraintNameAvailable(declared) error = %T %v, want *xsderrors.Error", err, err)
	}
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaDuplicate {
		t.Fatalf("diagnostic = %s/%s, want schema compile duplicate", diag.Category(), diag.Code())
	}
	if diag.Message() != `duplicate identity constraint "{urn:p}k"` {
		t.Fatalf("message = %q, want the duplicate's expanded name", diag.Message())
	}
}

func TestAKeyrefReferResolvesToTheConstraintDeclaredWithItsExpandedName(t *testing.T) {
	t.Parallel()

	c := identityDeclarationFixture(t)
	interner := newNameInterner(&c.rt.Names)
	declared, err := interner.InternQName("urn:p", "k")
	if err != nil {
		t.Fatal(err)
	}
	id, err := c.resolveIdentityConstraintRefer(declared)
	if err != nil || id != 1 {
		t.Fatalf("resolveIdentityConstraintRefer({urn:p}k) = %d, %v, want 1", id, err)
	}

	missing, err := interner.InternQName("urn:q", "k")
	if err != nil {
		t.Fatal(err)
	}
	id, err = c.resolveIdentityConstraintRefer(missing)
	if id != NoIdentityConstraint {
		t.Fatalf("resolveIdentityConstraintRefer({urn:q}k) id = %d, want NoIdentityConstraint", id)
	}
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("resolveIdentityConstraintRefer({urn:q}k) error = %T %v, want *xsderrors.Error", err, err)
	}
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaReference {
		t.Fatalf("diagnostic = %s/%s, want schema compile reference", diag.Category(), diag.Code())
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

func TestAChameleonIdentityConstraintPublishesTheSamePathsInEveryNamespaceItCompilesIn(t *testing.T) {
	t.Parallel()

	common := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
  <xs:element name="list">
    <xs:key name="k">
      <xs:selector xpath="item | p:item"/>
      <xs:field xpath="@id | p:code/@p:id"/>
    </xs:key>
  </xs:element>
</xs:schema>`)
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		if location == "common.xsd" {
			return source.Bytes("common.xsd", common), nil
		}
		return source.Source{}, xsderrors.ErrSchemaNotFound
	})
	root := func(name, target string) source.Source {
		data := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + target + `"><xs:include schemaLocation="common.xsd"/></xs:schema>`)
		return source.Bytes(name, data).WithResolver(resolver)
	}
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.loadOwned([]source.Source{root("a.xsd", "urn:a"), root("b.xsd", "urn:b")}); err != nil {
		t.Fatalf("load error = %v", err)
	}
	if err = c.index(); err != nil {
		t.Fatalf("index() error = %v", err)
	}
	if err = c.compileGlobals(); err != nil {
		t.Fatalf("compileGlobals() error = %v", err)
	}

	names := &c.rt.Names
	qname := func(ns, local string) QName {
		q, ok := names.LookupQName(ns, local)
		if !ok {
			t.Fatalf("name {%s}%s was not interned", ns, local)
		}
		return q
	}
	wantSelector := []identityPath{
		{Steps: []IdentityStep{{Name: qname("", "item")}}},
		{Steps: []IdentityStep{{Name: qname("urn:p", "item")}}},
	}
	wantFields := []identityFieldPath{
		{Attr: true, Attribute: qname("", "id")},
		{Attr: true, Attribute: qname("urn:p", "id"), Steps: []IdentityStep{{Name: qname("urn:p", "code")}}},
	}
	sameSelector := func(a, b identityPath) bool {
		return a.Descendant == b.Descendant && slices.Equal(a.Steps, b.Steps)
	}
	sameField := func(a, b identityFieldPath) bool {
		return a.Attr == b.Attr && a.Attribute == b.Attribute && a.AttrNamespace == b.AttrNamespace &&
			a.AttrWildcard == b.AttrWildcard && a.AttrNamespaceSet == b.AttrNamespaceSet &&
			a.Descendant == b.Descendant && slices.Equal(a.Steps, b.Steps)
	}

	var compiled []identityConstraintSpec
	for _, target := range []string{"urn:a", "urn:b"} {
		id, ok := c.rt.GlobalIdentities[qname(target, "k")]
		if !ok {
			t.Fatalf("identity constraint {%s}k was not declared", target)
		}
		ic := c.rt.Identities[id]
		if !slices.EqualFunc(ic.Selector, wantSelector, sameSelector) {
			t.Fatalf("{%s}k selector = %+v, want %+v", target, ic.Selector, wantSelector)
		}
		if len(ic.Fields) != 1 || !slices.EqualFunc(ic.Fields[0].Paths, wantFields, sameField) {
			t.Fatalf("{%s}k fields = %+v, want one field with paths %+v", target, ic.Fields, wantFields)
		}
		compiled = append(compiled, ic)
	}
	if &compiled[0].Selector[0] != &compiled[1].Selector[0] || &compiled[0].Fields[0].Paths[0] != &compiled[1].Fields[0].Paths[0] {
		t.Fatal("the constraint compiled in each namespace has its own copy of the paths")
	}

	ids := []IdentityConstraintID{c.rt.GlobalIdentities[qname("urn:a", "k")], c.rt.GlobalIdentities[qname("urn:b", "k")]}
	idName, pIDName := qname("", "id"), qname("urn:p", "id")
	published, err := publishSchema(&c.rt, unlimitedContentModelWork)
	if err != nil {
		t.Fatalf("publishSchema() error = %v", err)
	}
	if selector, field := compiled[0].Selector, compiled[0].Fields[0].Paths; !slices.EqualFunc(selector, wantSelector, sameSelector) || !slices.EqualFunc(field, wantFields, sameField) {
		t.Fatalf("compiled paths after publication = %+v and %+v, want %+v and %+v", selector, field, wantSelector, wantFields)
	}
	for i, target := range []string{"urn:a", "urn:b"} {
		program, ok := published.IdentityDispatch().Program(ids[i])
		if !ok {
			t.Fatalf("{%s}k has no published identity program", target)
		}
		selectors := program.Selectors()
		if selectors.Len() != len(wantSelector) {
			t.Fatalf("{%s}k publishes %d selector paths, want %d", target, selectors.Len(), len(wantSelector))
		}
		for j, want := range wantSelector {
			path, _ := selectors.At(j)
			if step, ok := path.Step(0); !ok || path.StepCount() != 1 || step != want.Steps[0] {
				t.Fatalf("{%s}k published selector path %d = %+v, want step %+v", target, j, path, want.Steps[0])
			}
		}
		if program.AttributeFields(idName).Len() != 1 || program.AttributeFields(pIDName).Len() != 1 {
			t.Fatalf("{%s}k does not publish one field path for each of @id and p:code/@p:id", target)
		}
	}
}
