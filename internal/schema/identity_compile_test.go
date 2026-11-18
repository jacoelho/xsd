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
	var node *schemaNode
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
		node = c.identitySources[id]
	}
	selector := semanticSource[*schemaIdentitySelectorSource](&node.firstXS(vocab.XSDElemSelector).semantic)
	field := semanticSource[*schemaIdentityFieldSource](&node.firstXS(vocab.XSDElemField).semantic)
	for _, ic := range compiled {
		if &ic.Selector[0] != &selector.Paths[0] || &ic.Fields[0].Paths[0] != &field.Paths[0] {
			t.Fatal("a compiled constraint does not share the paths stored at its selector and field")
		}
	}

	ids := []IdentityConstraintID{c.rt.GlobalIdentities[qname("urn:a", "k")], c.rt.GlobalIdentities[qname("urn:b", "k")]}
	idName, pIDName := qname("", "id"), qname("urn:p", "id")
	published, err := publishSchema(&c.rt, unlimitedContentModelWork)
	if err != nil {
		t.Fatalf("publishSchema() error = %v", err)
	}
	if !slices.EqualFunc(selector.Paths, wantSelector, sameSelector) || !slices.EqualFunc(field.Paths, wantFields, sameField) {
		t.Fatalf("stored paths after publication = %+v and %+v, want %+v and %+v", selector.Paths, field.Paths, wantSelector, wantFields)
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
