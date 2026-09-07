package schema_test

import (
	"errors"
	"reflect"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestPublishSchemaConsumesBuildOnSuccess(t *testing.T) {
	t.Parallel()

	build := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`)
	published, err := publishSchema(build)
	if err != nil {
		t.Fatalf("PublishSchema() error = %v", err)
	}
	if published == nil {
		t.Fatal("PublishSchema() returned nil schema")
	}
	if !reflect.DeepEqual(*build, xsdSchema.SchemaBuild{}) {
		t.Fatalf("PublishSchema() retained consumed build state: %#v", *build)
	}
}

func TestPublishSchemaChargesContentModelAuditWork(t *testing.T) {
	t.Parallel()

	build := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)
	want := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)
	budgetExceeded := xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content work exceeded")
	remaining := 1
	published, err := xsdSchema.PublishSchema(build, func(steps int) error {
		remaining -= steps
		if remaining < 0 {
			return budgetExceeded
		}
		return nil
	})
	if !errors.Is(err, budgetExceeded) {
		t.Fatalf("PublishSchema() error = %v, want budget error", err)
	}
	if published != nil {
		t.Fatal("PublishSchema() returned a schema after budget failure")
	}
	if !reflect.DeepEqual(*build, *want) {
		t.Fatal("PublishSchema() changed build after budget failure")
	}
}

func TestPublishSchemaRejectsMissingGlobalAttributeBindingAndAllowsRetry(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:attribute name="ga" type="xs:string"/></xs:schema>`
	build := mutableSchemaBuild(t, schema)
	q := mustQName(t, &build.Names, "ga")
	id := build.GlobalAttributes[q]
	delete(build.GlobalAttributes, q)
	expected := mutableSchemaBuild(t, schema)
	delete(expected.GlobalAttributes, mustQName(t, &expected.Names, "ga"))

	published, err := publishSchema(build)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if published != nil {
		t.Fatal("PublishSchema() returned a schema for an incomplete global attribute registry")
	}
	if !reflect.DeepEqual(*build, *expected) {
		t.Fatal("PublishSchema() changed build after global attribute ownership audit failure")
	}
	build.GlobalAttributes[q] = id
	if _, err := publishSchema(build); err != nil {
		t.Fatalf("PublishSchema() retry error = %v", err)
	}
}

func TestPublishSchemaRejectsMissingGlobalElementAndTypeBindingsAndAllowsRetry(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		remove func(t *testing.T, build *xsdSchema.SchemaBuild) func()
	}{
		{
			name:   "element",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`,
			remove: func(t *testing.T, build *xsdSchema.SchemaBuild) func() {
				t.Helper()
				q := mustQName(t, &build.Names, "root")
				id := build.GlobalElements[q]
				delete(build.GlobalElements, q)
				return func() { build.GlobalElements[q] = id }
			},
		},
		{
			name:   "simple type",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="T"><xs:restriction base="xs:string"/></xs:simpleType></xs:schema>`,
			remove: func(t *testing.T, build *xsdSchema.SchemaBuild) func() {
				t.Helper()
				q := mustQName(t, &build.Names, "T")
				id := build.GlobalTypes[q]
				delete(build.GlobalTypes, q)
				return func() { build.GlobalTypes[q] = id }
			},
		},
		{
			name:   "complex type",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="T"/></xs:schema>`,
			remove: func(t *testing.T, build *xsdSchema.SchemaBuild) func() {
				t.Helper()
				q := mustQName(t, &build.Names, "T")
				id := build.GlobalTypes[q]
				delete(build.GlobalTypes, q)
				return func() { build.GlobalTypes[q] = id }
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			build := mutableSchemaBuild(t, test.schema)
			restore := test.remove(t, build)
			expected := mutableSchemaBuild(t, test.schema)
			test.remove(t, expected)

			published, err := publishSchema(build)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil {
				t.Fatal("PublishSchema() returned a schema for an incomplete global registry")
			}
			if !reflect.DeepEqual(*build, *expected) {
				t.Fatal("PublishSchema() changed build after global ownership audit failure")
			}
			restore()
			if _, err := publishSchema(build); err != nil {
				t.Fatalf("PublishSchema() retry error = %v", err)
			}
		})
	}
}

func TestPublishSchemaRejectsMissingGlobalIdentityBindingAndAllowsRetry(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:key name="k"><xs:selector xpath="."/><xs:field xpath="."/></xs:key>
  </xs:element>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	q := build.Identities[0].Name
	id, ok := build.GlobalIdentities[q]
	if !ok || id != xsdSchema.IdentityConstraintID(0) {
		t.Fatalf("global identity binding = %d, %t; want 0, true", id, ok)
	}
	delete(build.GlobalIdentities, q)
	expected := mutableSchemaBuild(t, schema)
	delete(expected.GlobalIdentities, expected.Identities[0].Name)

	published, err := publishSchema(build)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if published != nil {
		t.Fatal("PublishSchema() returned a schema for an incomplete global identity registry")
	}
	if !reflect.DeepEqual(*build, *expected) {
		t.Fatal("PublishSchema() changed build after global identity ownership audit failure")
	}
	build.GlobalIdentities[q] = id
	if _, err := publishSchema(build); err != nil {
		t.Fatalf("PublishSchema() retry error = %v", err)
	}
}

func TestPublishSchemaRejectsInvalidDeclarationScopeAndAllowsRetry(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	build := mutableSchemaBuild(t, schema)
	id := build.GlobalElements[mustQName(t, &build.Names, "root")]
	build.Elements[id].Scope = xsdSchema.DeclarationScopeInvalid
	expected := mutableSchemaBuild(t, schema)
	expectedID := expected.GlobalElements[mustQName(t, &expected.Names, "root")]
	expected.Elements[expectedID].Scope = xsdSchema.DeclarationScopeInvalid

	published, err := publishSchema(build)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if published != nil || !reflect.DeepEqual(*build, *expected) {
		t.Fatal("PublishSchema() consumed build after invalid declaration scope")
	}
	build.Elements[id].Scope = xsdSchema.DeclarationScopeGlobal
	if _, err := publishSchema(build); err != nil {
		t.Fatalf("PublishSchema() retry error = %v", err)
	}
}

func TestPublishSchemaAcceptsNonGlobalMissingSimpleTypeSentinel(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:m="urn:missing"><xs:import namespace="urn:missing"/><xs:element name="root" type="m:T"/></xs:schema>`
	build := mutableSchemaBuild(t, schema)
	found := false
	for _, typ := range build.SimpleTypes {
		if typ.Missing {
			found = true
			if typ.Scope != xsdSchema.DeclarationScopeNonGlobal {
				t.Fatalf("missing simple type scope = %v, want non-global", typ.Scope)
			}
		}
	}
	if !found {
		t.Fatal("compiler did not create missing simple type sentinel")
	}
	if _, err := publishSchema(build); err != nil {
		t.Fatalf("PublishSchema() error = %v", err)
	}
}

func TestPublishSchemaRejectsInvalidIdentityOwnershipAndAllowsRetry(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="other"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="row" maxOccurs="unbounded"><xs:complexType><xs:attribute name="id" use="required"/></xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:key name="k"><xs:selector xpath="row"/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, build *xsdSchema.SchemaBuild)
	}{
		{name: "orphan", mutate: func(t *testing.T, build *xsdSchema.SchemaBuild) {
			t.Helper()
			root := build.GlobalElements[mustQName(t, &build.Names, "root")]
			build.Elements[root].Identity = nil
		}},
		{name: "duplicate on owner", mutate: func(t *testing.T, build *xsdSchema.SchemaBuild) {
			t.Helper()
			root := build.GlobalElements[mustQName(t, &build.Names, "root")]
			build.Elements[root].Identity = append(build.Elements[root].Identity, build.Elements[root].Identity[0])
		}},
		{name: "shared by elements", mutate: func(t *testing.T, build *xsdSchema.SchemaBuild) {
			t.Helper()
			root := build.GlobalElements[mustQName(t, &build.Names, "root")]
			other := build.GlobalElements[mustQName(t, &build.Names, "other")]
			build.Elements[other].Identity = append(build.Elements[other].Identity, build.Elements[root].Identity[0])
		}},
		{name: "invalid id", mutate: func(t *testing.T, build *xsdSchema.SchemaBuild) {
			t.Helper()
			root := build.GlobalElements[mustQName(t, &build.Names, "root")]
			build.Elements[root].Identity[0] = xsdSchema.NoIdentityConstraint
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			build := mutableSchemaBuild(t, schema)
			test.mutate(t, build)
			expected := mutableSchemaBuild(t, schema)
			test.mutate(t, expected)

			published, err := publishSchema(build)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil {
				t.Fatal("PublishSchema() returned a schema for invalid identity ownership")
			}
			if !reflect.DeepEqual(*build, *expected) {
				t.Fatal("PublishSchema() changed build after identity ownership audit failure")
			}
			*build = *mutableSchemaBuild(t, schema)
			if _, err := publishSchema(build); err != nil {
				t.Fatalf("PublishSchema() retry error = %v", err)
			}
		})
	}
}

func TestPublishSchemaRejectsForgedMissingSimpleTypeWithoutConsumingBuild(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="User"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	id := simpleBuildTypeIDByName(t, build, "User")
	build.SimpleTypes[id].Missing = true
	expected := mutableSchemaBuild(t, schema)
	expectedID := simpleBuildTypeIDByName(t, expected, "User")
	expected.SimpleTypes[expectedID].Missing = true

	published, err := publishSchema(build)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if published != nil {
		t.Fatal("PublishSchema() returned a schema for a forged missing type")
	}
	if !reflect.DeepEqual(*build, *expected) {
		t.Fatal("PublishSchema() changed build after failed missing-sentinel audit")
	}
}

func TestPublishSchemaRejectsContentModelCyclesWithoutConsumingBuild(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	one := xsdSchema.Occurrence{Min: 1, Max: 1}
	addCycle := func(build *xsdSchema.SchemaBuild, size int) {
		start := len(build.Models)
		for range size {
			build.Models = append(build.Models, xsdSchema.ContentModel{Kind: xsdSchema.ModelSequence, Occurs: one})
		}
		for i := range size {
			child := start + (i+1)%size
			childID := xsdSchema.ContentModelID(child) //nolint:gosec // the test model table is bounded to two added entries.
			build.Models[start+i].Particles = []xsdSchema.Particle{xsdSchema.ModelParticle(childID, one)}
		}
	}
	for _, size := range []int{1, 2} {
		t.Run(map[int]string{1: "self", 2: "multi-node"}[size], func(t *testing.T) {
			build := mutableSchemaBuild(t, schema)
			addCycle(build, size)
			expected := mutableSchemaBuild(t, schema)
			addCycle(expected, size)

			published, err := publishSchema(build)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil {
				t.Fatal("PublishSchema() returned a schema for cyclic content models")
			}
			if !reflect.DeepEqual(*build, *expected) {
				t.Fatal("PublishSchema() changed build after cyclic-model audit failure")
			}
		})
	}
}

func TestPublishSchemaRejectsComplexTypeCyclesWithoutConsumingBuild(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A"/>
  <xs:complexType name="B"/>
</xs:schema>`
	addCycle := func(t *testing.T, build *xsdSchema.SchemaBuild, size int) {
		t.Helper()
		a := complexBuildTypeIDByName(t, build, "A")
		build.ComplexTypes[a].ExplicitDerivation = true
		if size == 1 {
			build.ComplexTypes[a].Base = xsdSchema.ComplexRef(a)
			return
		}
		b := complexBuildTypeIDByName(t, build, "B")
		build.ComplexTypes[a].Base = xsdSchema.ComplexRef(b)
		build.ComplexTypes[b].Base = xsdSchema.ComplexRef(a)
		build.ComplexTypes[b].ExplicitDerivation = true
	}
	for _, size := range []int{1, 2} {
		t.Run(map[int]string{1: "self", 2: "multi-node"}[size], func(t *testing.T) {
			build := mutableSchemaBuild(t, schema)
			addCycle(t, build, size)
			expected := mutableSchemaBuild(t, schema)
			addCycle(t, expected, size)

			published, err := publishSchema(build)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil {
				t.Fatal("PublishSchema() returned a schema for cyclic complex types")
			}
			if !reflect.DeepEqual(*build, *expected) {
				t.Fatal("PublishSchema() changed build after complex-type cycle audit failure")
			}
		})
	}
}

func mutableSchemaBuild(t *testing.T, schema string) *xsdSchema.SchemaBuild {
	t.Helper()
	return compiledCompilerRuntime(t, schema).RuntimeForTest()
}

func validateSchemaBuild(build *xsdSchema.SchemaBuild) error {
	return xsdSchema.ValidateBuildForTest(build)
}

func rootBuildContentModel(t *testing.T, build *xsdSchema.SchemaBuild) xsdSchema.ContentModelID {
	t.Helper()
	root := build.GlobalElements[mustQName(t, &build.Names, rootContentModelName)]
	typ, ok := build.Elements[root].Type.Complex()
	if !ok {
		t.Fatal("root type is not complex")
	}
	return build.ComplexTypes[typ].Content
}

func rootBuildAttributeUseSet(t *testing.T, build *xsdSchema.SchemaBuild) *xsdSchema.AttributeUseSet {
	t.Helper()
	root := build.GlobalElements[mustQName(t, &build.Names, "root")]
	typ, ok := build.Elements[root].Type.Complex()
	if !ok {
		t.Fatal("root type is not complex")
	}
	attrs := build.ComplexTypes[typ].Attrs
	if attrs == xsdSchema.NoAttributeUseSet {
		t.Fatal("root complex type has no attribute use set")
	}
	return &build.AttributeUseSets[attrs]
}

func complexBuildTypeIDByName(t *testing.T, build *xsdSchema.SchemaBuild, local string) xsdSchema.ComplexTypeID {
	t.Helper()
	typ, ok := build.GlobalTypes[mustQName(t, &build.Names, local)]
	if !ok {
		t.Fatalf("global type %q not found", local)
	}
	id, ok := typ.Complex()
	if !ok {
		t.Fatalf("global type %q is not complex", local)
	}
	return id
}

func simpleBuildTypeIDByName(t *testing.T, build *xsdSchema.SchemaBuild, local string) xsdSchema.SimpleTypeID {
	t.Helper()
	typ, ok := build.GlobalTypes[mustQName(t, &build.Names, local)]
	if !ok {
		t.Fatalf("global type %q not found", local)
	}
	id, ok := typ.Simple()
	if !ok {
		t.Fatalf("global type %q is not simple", local)
	}
	return id
}

func buildValueConstraint(t *testing.T, build *xsdSchema.SchemaBuild, id xsdSchema.SimpleTypeID, lexical string) *xsdSchema.ValueConstraint {
	t.Helper()
	resolver := valuepkg.Resolver{
		QName: func(lexical string) (valuepkg.ExpandedName, bool) {
			return valuepkg.ExpandedName{Local: lexical}, true
		},
		Notation: func(namespace, local string) bool {
			q, ok := build.Names.LookupQName(namespace, local)
			return ok && build.Notations[q]
		},
	}
	validated, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Validate(id, lexical, resolver, valuepkg.NeedCanonical|valuepkg.NeedIdentity, nil)
	if err != nil {
		t.Fatalf("validate %q: %v", lexical, err)
	}
	return &xsdSchema.ValueConstraint{Lexical: lexical, Canonical: validated.CanonicalText(), Value: validated}
}

func cloneBuildValueConstraint(in *xsdSchema.ValueConstraint) *xsdSchema.ValueConstraint {
	if in == nil {
		return nil
	}
	out := new(xsdSchema.ValueConstraint)
	*out = *in
	out.ResolvedNames = append([]xsdSchema.ResolvedValueName(nil), in.ResolvedNames...)
	return out
}

func TestFreezeRejectsInvalidWildcards(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="urn:b urn:a" processContents="lax" minOccurs="0"/>
      </xs:sequence>
      <xs:anyAttribute namespace="##other" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	wildcardByMode := func(t *testing.T, rt *xsdSchema.SchemaBuild, mode xsdSchema.WildcardMode) *xsdSchema.Wildcard {
		t.Helper()
		for i := range rt.Wildcards {
			if rt.Wildcards[i].Mode == mode {
				return &rt.Wildcards[i]
			}
		}
		t.Fatalf("wildcard mode %d not found", mode)
		return nil
	}
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild)
	}{
		{
			name: "invalid process",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				wildcardByMode(t, rt, xsdSchema.WildcardList).Process = xsdSchema.ProcessContents(99)
			},
		},
		{
			name: "invalid mode",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				wildcardByMode(t, rt, xsdSchema.WildcardList).Mode = xsdSchema.WildcardMode(99)
			},
		},
		{
			name: "stale other field",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				ns, ok := rt.Names.LookupNamespace("urn:a")
				if !ok {
					t.Fatal("urn:a namespace not interned")
				}
				wildcardByMode(t, rt, xsdSchema.WildcardList).OtherThan = ns
			},
		},
		{
			name: "unnormalized namespace list",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				w := wildcardByMode(t, rt, xsdSchema.WildcardList)
				if len(w.Namespaces) < 2 {
					t.Fatalf("wildcard namespace list length = %d, want >= 2", len(w.Namespaces))
				}
				w.Namespaces[0], w.Namespaces[1] = w.Namespaces[1], w.Namespaces[0]
			},
		},
		{
			name: "invalid namespace id",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				wildcardByMode(t, rt, xsdSchema.WildcardOther).OtherThan = xsdSchema.NamespaceID(1 << 30)
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsInconsistentValueConstraints(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string" default="abc"/></xs:schema>`
	for _, mismatch := range []string{"canonical", "owner type"} {
		t.Run(mismatch, func(t *testing.T) {
			build := mutableSchemaBuild(t, schema)
			root := build.GlobalElements[mustQName(t, &build.Names, "root")]
			vc := build.Elements[root].Default
			if mismatch == "canonical" {
				vc.Canonical = "other"
			} else {
				validated, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Validate(valuepkg.BuiltinType(valuepkg.PrimitiveBoolean), "true", valuepkg.Resolver{}, valuepkg.NeedCanonical|valuepkg.NeedIdentity, nil)
				if err != nil {
					t.Fatal(err)
				}
				vc.Lexical, vc.Canonical, vc.Value = "true", "true", validated
			}
			expectCategoryCode(t, validateSchemaBuild(build), xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsBothDefaultAndFixedValueConstraints(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="ga" type="xs:string" default="a"/>
  <xs:element name="value" type="xs:string" default="v"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="la" type="xs:string" default="b"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild)
	}{
		{
			name: "element declaration",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				id := rt.GlobalElements[mustQName(t, &rt.Names, "value")]
				rt.Elements[id].Fixed = cloneBuildValueConstraint(rt.Elements[id].Default)
			},
		},
		{
			name: "attribute declaration",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				id := rt.GlobalAttributes[mustQName(t, &rt.Names, "ga")]
				rt.Attributes[id].Fixed = cloneBuildValueConstraint(rt.Attributes[id].Default)
			},
		},
		{
			name: "attribute use",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				set := rootBuildAttributeUseSet(t, rt)
				set.Uses[0].Fixed = cloneBuildValueConstraint(set.Uses[0].Default)
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsValueConstraintThatNoLongerSatisfiesFacets(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:enumeration value="A"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Code" default="A"/>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
	defaultValue := rt.Elements[rootID].Default
	defaultValue.Lexical = "B"
	defaultValue.Canonical = "B"
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInvalidResolvedQNameReplay(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:t="urn:test">
  <xs:element name="root" type="xs:QName" default="t:item"/>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
	defaultValue := rt.Elements[rootID].Default
	defaultValue.Lexical = "bad::item"
	defaultValue.Canonical = xsdSchema.FormatExpandedName("urn:test", "item")
	defaultValue.ResolvedNames = []xsdSchema.ResolvedValueName{{Lexical: defaultValue.Lexical, NS: "urn:test", Local: "item"}}
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsUnusedResolvedNameProof(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:t="urn:test">
  <xs:element name="root" type="xs:QName" default="t:item"/>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
	defaultValue := rt.Elements[rootID].Default
	defaultValue.ResolvedNames = append(defaultValue.ResolvedNames, xsdSchema.ResolvedValueName{Lexical: "t:other", NS: "urn:test", Local: "other"})
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsNondeterministicResolvedNameProof(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:p="urn:a">
  <xs:simpleType name="QNames">
    <xs:list itemType="xs:QName"/>
  </xs:simpleType>
  <xs:element name="root" type="QNames" default="p:x p:x"/>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
	defaultValue := rt.Elements[rootID].Default
	if len(defaultValue.ResolvedNames) != 2 {
		t.Fatalf("resolved names = %d, want 2", len(defaultValue.ResolvedNames))
	}
	canonical := xsdSchema.FormatExpandedName("urn:a", "x") + " " + xsdSchema.FormatExpandedName("urn:b", "x")
	defaultValue.ResolvedNames[1].NS = "urn:b"
	defaultValue.Canonical = canonical
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsMixedValueConstraintIdentityPayload(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" default="text">
    <xs:complexType mixed="true"/>
  </xs:element>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
	rt.Elements[rootID].Default.Value = buildValueConstraint(t, rt, rt.Builtin.String, "text").Value
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsMixedValueConstraintResolvedNameProof(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" default="text">
    <xs:complexType mixed="true"/>
  </xs:element>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
	rt.Elements[rootID].Default.ResolvedNames = []xsdSchema.ResolvedValueName{{Lexical: "p:x", NS: "urn:test", Local: "x"}}
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInconsistentNameTable(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rt.Names = xsdSchema.NameTable{}
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsGlobalNameMismatch(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="xs:string"/>
  <xs:element name="b" type="xs:string"/>
  <xs:attribute name="ga" type="xs:string"/>
  <xs:attribute name="gb" type="xs:string"/>
  <xs:simpleType name="t1">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="t2">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
            <xs:attribute name="id2" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k1">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:key name="k2">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id2"/>
    </xs:key>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild)
	}{
		{
			name: "global element points at other declaration",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.GlobalElements[mustQName(t, &rt.Names, "a")] = rt.GlobalElements[mustQName(t, &rt.Names, "b")]
			},
		},
		{
			name: "global attribute points at other declaration",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.GlobalAttributes[mustQName(t, &rt.Names, "ga")] = rt.GlobalAttributes[mustQName(t, &rt.Names, "gb")]
			},
		},
		{
			name: "global type points at other type",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.GlobalTypes[mustQName(t, &rt.Names, "t1")] = rt.GlobalTypes[mustQName(t, &rt.Names, "t2")]
			},
		},
		{
			name: "global identity points at other constraint",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.GlobalIdentities[mustQName(t, &rt.Names, "k1")] = rt.GlobalIdentities[mustQName(t, &rt.Names, "k2")]
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsIdentityFieldLookupDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="name" type="xs:string"/>
            </xs:sequence>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k1">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
      <xs:field xpath="name"/>
    </xs:key>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(ic *xsdSchema.IdentityConstraint)
	}{
		{
			name: "dropped attribute lookup",
			mutate: func(ic *xsdSchema.IdentityConstraint) {
				ic.AttributeFields = nil
			},
		},
		{
			name: "element lookup field index drift",
			mutate: func(ic *xsdSchema.IdentityConstraint) {
				ic.ElementFields[0].Field = 7
			},
		},
		{
			name: "extra wildcard lookup entry",
			mutate: func(ic *xsdSchema.IdentityConstraint) {
				ic.AttributeWildcardFields = append(ic.AttributeWildcardFields, xsdSchema.CompiledIdentityField{Field: 0})
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := rt.GlobalIdentities[mustQName(t, &rt.Names, "k1")]
			tc.mutate(&rt.Identities[id])
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsIdentityKindReferMismatch(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="kr1" refer="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:keyref>
    <xs:keyref name="kr2" refer="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild)
	}{
		{
			name: "key stores refer",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "k")]].Refer = rt.GlobalIdentities[mustQName(t, &rt.Names, "kr1")]
			},
		},
		{
			name: "keyref missing refer",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "kr1")]].Refer = xsdSchema.NoIdentityConstraint
			},
		},
		{
			name: "keyref references keyref",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "kr1")]].Refer = rt.GlobalIdentities[mustQName(t, &rt.Names, "kr2")]
			},
		},
		{
			name: "keyref field count drift",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				kr := &rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "kr1")]]
				kr.Fields = append(kr.Fields, xsdSchema.IdentityField{})
				kr.ElementFields, kr.AttributeFields, kr.AttributeWildcardFields = xsdSchema.BuildIdentityFieldLookup(kr.Fields)
			},
		},
		{
			name: "missing selector",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "k")]].Selector = nil
			},
		},
		{
			name: "missing fields",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "k")]]
				ic.Fields = nil
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = xsdSchema.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "field without paths",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "k")]]
				ic.Fields[0].Paths = nil
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = xsdSchema.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "selector self stores ignored path",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "k")]]
				ic.Selector[0] = xsdSchema.IdentityPath{
					Self:       true,
					Descendant: true,
					Steps: []xsdSchema.IdentityStep{{
						Name: mustQName(t, &rt.Names, "item"),
					}},
				}
			},
		},
		{
			name: "selector wildcard stores ignored name",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "k")]]
				ic.Selector[0].Steps[0].Wildcard = true
				ic.Selector[0].Steps[0].Name = xsdSchema.QName{}
			},
		},
		{
			name: "field self stores ignored attribute",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "k")]]
				ic.Fields[0].Paths[0] = xsdSchema.IdentityFieldPath{
					Self:      true,
					Attr:      true,
					Attribute: mustQName(t, &rt.Names, "id"),
				}
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = xsdSchema.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "element field stores ignored attribute",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "k")]]
				ic.Fields[0].Paths[0] = xsdSchema.IdentityFieldPath{
					Steps: []xsdSchema.IdentityStep{{
						Name: mustQName(t, &rt.Names, "item"),
					}},
					Attribute: xsdSchema.QName{},
				}
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = xsdSchema.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "attribute wildcard stores ignored name",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, &rt.Names, "k")]]
				ic.Fields[0].Paths[0].AttrWildcard = true
				ic.Fields[0].Paths[0].Attribute = mustQName(t, &rt.Names, "id")
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = xsdSchema.BuildIdentityFieldLookup(ic.Fields)
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsAttributeUseSetIndexDrift(t *testing.T) {
	t.Run("stale index on empty uses", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute processContents="lax"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
		rt := mutableSchemaBuild(t, schema)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		set := rootBuildAttributeUseSet(t, rt)
		if len(set.Uses) != 0 {
			t.Fatalf("expected empty attribute uses, got %d", len(set.Uses))
		}
		set.Index = map[xsdSchema.QName]uint32{mustQName(t, &rt.Names, "root"): 5}
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("missing index entry", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string"/>
      <xs:attribute name="b" type="xs:string"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
		rt := mutableSchemaBuild(t, schema)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		set := rootBuildAttributeUseSet(t, rt)
		if len(set.Uses) != 2 {
			t.Fatalf("expected two attribute uses, got %d", len(set.Uses))
		}
		delete(set.Index, set.Uses[0].Name)
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
}

func TestFreezeRejectsAttributeUseSetDerivedSlotDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="required" type="xs:string" use="required"/>
      <xs:attribute name="defaulted" type="xs:string" default="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(set *xsdSchema.AttributeUseSet)
	}{
		{
			name: "missing required slot",
			mutate: func(set *xsdSchema.AttributeUseSet) {
				set.Required = nil
			},
		},
		{
			name: "missing value constraint slot",
			mutate: func(set *xsdSchema.AttributeUseSet) {
				set.ValueConstraints = nil
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(rootBuildAttributeUseSet(t, rt))
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsPublishedProhibitedAttributeUse(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="plain" type="xs:string"/>
      <xs:attribute name="defaulted" type="xs:string" default="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	useByName := func(t *testing.T, rt *xsdSchema.SchemaBuild, set *xsdSchema.AttributeUseSet, local string) *xsdSchema.AttributeUse {
		t.Helper()
		name := mustQName(t, &rt.Names, local)
		for i := range set.Uses {
			if set.Uses[i].Name == name {
				return &set.Uses[i]
			}
		}
		t.Fatalf("attribute use %q not found", local)
		return nil
	}
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild, set *xsdSchema.AttributeUseSet)
	}{
		{
			name: "plain prohibited",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild, set *xsdSchema.AttributeUseSet) {
				t.Helper()
				useByName(t, rt, set, "plain").Prohibited = true
			},
		},
		{
			name: "prohibited with default",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild, set *xsdSchema.AttributeUseSet) {
				t.Helper()
				useByName(t, rt, set, "defaulted").Prohibited = true
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt, rootBuildAttributeUseSet(t, rt))
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsIDAttributeSchemaInvariantDrift(t *testing.T) {
	t.Run("attribute declaration value constraint", func(t *testing.T) {
		rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" type="xs:string"/>
</xs:schema>`)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		attr := rt.GlobalAttributes[mustQName(t, &rt.Names, "a")]
		rt.Attributes[attr].Type = rt.Builtin.ID
		rt.Attributes[attr].Default = buildValueConstraint(t, rt, rt.Builtin.ID, "abc")
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("element declaration value constraint", func(t *testing.T) {
		rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		root := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
		rt.Elements[root].Type = xsdSchema.SimpleRef(rt.Builtin.ID)
		rt.Elements[root].Default = buildValueConstraint(t, rt, rt.Builtin.ID, "abc")
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("attribute use value constraint", func(t *testing.T) {
		rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:attribute name="a" type="xs:string"/></xs:complexType>
  </xs:element>
</xs:schema>`)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		set := rootBuildAttributeUseSet(t, rt)
		set.Uses[0].Type = rt.Builtin.ID
		set.Uses[0].Default = buildValueConstraint(t, rt, rt.Builtin.ID, "abc")
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("multiple ID attribute uses", func(t *testing.T) {
		rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:ID"/>
      <xs:attribute name="b" type="xs:string"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		set := rootBuildAttributeUseSet(t, rt)
		set.Uses[1].Type = rt.Builtin.ID
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
}

func TestFreezeRejectsBareNotationElementValueConstraint(t *testing.T) {
	rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="gif" public="image/gif"/>
  <xs:element name="root" type="xs:NOTATION"/>
</xs:schema>`)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	root := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
	notationQName, ok := rt.Names.LookupQName(vocab.XSDNamespaceURI, "NOTATION")
	if !ok {
		t.Fatal("missing NOTATION builtin QName")
	}
	notationID, ok := rt.GlobalTypes[notationQName].Simple()
	if !ok {
		t.Fatal("NOTATION builtin is not a simple type")
	}
	rt.Elements[root].Default = buildValueConstraint(t, rt, notationID, "gif")
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsZeroTypeID(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="CT"><xs:sequence/></xs:complexType>
  <xs:element name="root" type="CT"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild)
	}{
		{
			name: "element type",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rootID := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
				rt.Elements[rootID].Type = xsdSchema.TypeID{}
			},
		},
		{
			name: "complex type base",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				ctID, ok := rt.GlobalTypes[mustQName(t, &rt.Names, "CT")].Complex()
				if !ok {
					t.Fatal("CT is not a complex type")
				}
				rt.ComplexTypes[ctID].Base = xsdSchema.TypeID{}
			},
		},
		{
			name: "global type",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.GlobalTypes[mustQName(t, &rt.Names, "CT")] = xsdSchema.TypeID{}
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsInvalidSimpleFinalMask(t *testing.T) {
	build := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)
	build.SimpleTypes[simpleBuildTypeIDByName(t, build, "Plain")].Final = xsdSchema.DerivationExtension
	_, err := publishSchema(build)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsBuiltinHandleDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild)
	}{
		{
			name: "simple handle points at wrong valid type",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.Builtin.String = rt.Builtin.Boolean
			},
		},
		{
			name: "global type binding drift",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				q, ok := rt.Names.LookupQName(vocab.XSDNamespaceURI, "string")
				if !ok {
					t.Fatal("xs:string name not found")
				}
				rt.GlobalTypes[q] = xsdSchema.SimpleRef(rt.Builtin.Boolean)
			},
		},
		{
			name: "missing builtin declaration table",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.Wildcards = nil
			},
		},
		{
			name: "builtin attribute handle drift",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				q, ok := rt.Names.LookupQName(vocab.XMLNamespaceURI, vocab.XMLAttrBase)
				if !ok {
					t.Fatal("xml:base name not found")
				}
				id, ok := rt.GlobalAttributes[q]
				if !ok {
					t.Fatal("xml:base attribute not found")
				}
				rt.Attributes[id].Type = rt.Builtin.String
			},
		},
		{
			name: "anyType shape drift",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.ComplexTypes[rt.Builtin.AnyType].ContentKind = xsdSchema.ContentElementOnly
			},
		},
		{
			name: "anyType wildcard drift",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				attrs := rt.ComplexTypes[rt.Builtin.AnyType].Attrs
				rt.AttributeUseSets[attrs].Wildcard = xsdSchema.NoWildcard
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsInvalidContentModelShape(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild)
	}{
		{
			name: "invalid kind",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.Models[rootBuildContentModel(t, rt)].Kind = xsdSchema.ModelKind(255)
			},
		},
		{
			name: "invalid occurrence range",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rootBuildContentModel(t, rt)]
				model.Occurs = xsdSchema.Occurrence{Min: 2, Max: 1}
			},
		},
		{
			name: "unsorted choice limits",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rootBuildContentModel(t, rt)]
				model.ChoiceLimits = []uint32{1, 0}
			},
		},
		{
			name: "unjustified choice limits",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rootBuildContentModel(t, rt)]
				model.ChoiceLimits = []uint32{1}
			},
		},
		{
			name: "choice limit on non-sequence",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rootBuildContentModel(t, rt)]
				model.Kind = xsdSchema.ModelChoice
				model.ChoiceLimits = []uint32{1}
			},
		},
		{
			name: "any model inactive state",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rt.ComplexTypes[rt.Builtin.AnyType].Content]
				model.Occurs = xsdSchema.Occurrence{Min: 1, Max: 1}
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsLimitedContentModelSharedByNonRestriction(t *testing.T) {
	rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:choice maxOccurs="unbounded">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:sequence><xs:element name="a" type="xs:string" maxOccurs="unbounded"/></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="Other">
    <xs:sequence><xs:element name="other" type="xs:string"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	limited := xsdSchema.NoContentModel
	for id, model := range rt.Models {
		if len(model.ChoiceLimits) != 0 {
			limited = xsdSchema.ContentModelID(id)
			break
		}
	}
	if limited == xsdSchema.NoContentModel {
		t.Fatal("no limited content model found")
	}
	otherID, ok := rt.GlobalTypes[mustQName(t, &rt.Names, "Other")].Complex()
	if !ok {
		t.Fatal("Other is not complex")
	}
	rt.ComplexTypes[otherID].Content = limited
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsComplexExtensionDroppingOptionalBaseParticle(t *testing.T) {
	rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence><xs:element name="a" type="xs:string" minOccurs="0"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:complexType name="OnlyB">
    <xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	derived := complexBuildTypeIDByName(t, rt, "Derived")
	onlyB := complexBuildTypeIDByName(t, rt, "OnlyB")
	rt.ComplexTypes[derived].Content = rt.ComplexTypes[onlyB].Content
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestPublishSchemaRejectsInvalidComplexContentRestrictionAndAllowsRetry(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:restriction base="Base"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:restriction></xs:complexContent>
  </xs:complexType>
  <xs:complexType name="Other">
    <xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence>
  </xs:complexType>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	derived := complexBuildTypeIDByName(t, build, "Derived")
	other := complexBuildTypeIDByName(t, build, "Other")
	validContent := build.ComplexTypes[derived].Content
	build.ComplexTypes[derived].Content = build.ComplexTypes[other].Content
	expected := mutableSchemaBuild(t, schema)
	expectedDerived := complexBuildTypeIDByName(t, expected, "Derived")
	expectedOther := complexBuildTypeIDByName(t, expected, "Other")
	expected.ComplexTypes[expectedDerived].Content = expected.ComplexTypes[expectedOther].Content

	published, err := publishSchema(build)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if published != nil {
		t.Fatal("PublishSchema() returned a schema for an invalid complex content restriction")
	}
	if !reflect.DeepEqual(*build, *expected) {
		t.Fatal("PublishSchema() changed build after content-restriction audit failure")
	}
	build.ComplexTypes[derived].Content = validContent
	if _, err := publishSchema(build); err != nil {
		t.Fatalf("PublishSchema() retry error = %v", err)
	}
}

func TestFreezeRejectsComplexExtensionWrapperOccurrenceDrift(t *testing.T) {
	rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent>
  </xs:complexType>
</xs:schema>`)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	derived := complexBuildTypeIDByName(t, rt, "Derived")
	rt.Models[rt.ComplexTypes[derived].Content].Occurs.Min = 0
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInvalidComplexTypeShape(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element name="child" type="xs:string"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild, ct *xsdSchema.ComplexType)
	}{
		{
			name: "missing content",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild, ct *xsdSchema.ComplexType) {
				t.Helper()
				ct.Content = xsdSchema.NoContentModel
			},
		},
		{
			name: "missing attrs",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild, ct *xsdSchema.ComplexType) {
				t.Helper()
				ct.Attrs = xsdSchema.NoAttributeUseSet
			},
		},
		{
			name: "invalid content kind",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild, ct *xsdSchema.ComplexType) {
				t.Helper()
				ct.ContentKind = xsdSchema.ContentKind(255)
			},
		},
		{
			name: "invalid derivation",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild, ct *xsdSchema.ComplexType) {
				t.Helper()
				ct.Derivation = xsdSchema.DerivationKind(255)
			},
		},
		{
			name: "invalid block mask",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild, ct *xsdSchema.ComplexType) {
				t.Helper()
				ct.Block = xsdSchema.DerivationSubstitution
			},
		},
		{
			name: "invalid final mask",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild, ct *xsdSchema.ComplexType) {
				t.Helper()
				ct.Final = xsdSchema.DerivationSubstitution
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			root := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
			ctID, ok := rt.Elements[root].Type.Complex()
			if !ok {
				t.Fatal("root type is not complex")
			}
			tc.mutate(t, rt, &rt.ComplexTypes[ctID])
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsAttributeWildcardBaseMismatch(t *testing.T) {
	rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:anyAttribute namespace="##other" processContents="lax"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:anyAttribute namespace="##local" processContents="lax"/></xs:extension></xs:complexContent>
  </xs:complexType>
</xs:schema>`)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	derived := complexBuildTypeIDByName(t, rt, "Derived")
	set := &rt.AttributeUseSets[rt.ComplexTypes[derived].Attrs]
	if set.WildcardDeclared == xsdSchema.NoWildcard {
		t.Fatal("derived attribute wildcard did not record declared wildcard")
	}
	set.WildcardBase = xsdSchema.NoWildcard
	set.Wildcard = set.WildcardDeclared
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInvalidDerivationSourceBeforeReplay(t *testing.T) {
	t.Run("invalid base wildcard", func(t *testing.T) {
		rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:anyAttribute namespace="##other" processContents="skip"/></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:restriction base="Base"/></xs:complexContent></xs:complexType>
</xs:schema>`)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		base := complexBuildTypeIDByName(t, rt, "Base")
		set := &rt.AttributeUseSets[rt.ComplexTypes[base].Attrs]
		bad := xsdSchema.WildcardID(1 << 30)
		set.Wildcard = bad
		set.WildcardDeclared = bad
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("invalid model particle", func(t *testing.T) {
		rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:restriction base="Base"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:restriction></xs:complexContent>
  </xs:complexType>
</xs:schema>`)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		base := complexBuildTypeIDByName(t, rt, "Base")
		model := &rt.Models[rt.ComplexTypes[base].Content]
		model.Particles[0] = xsdSchema.ModelParticle(xsdSchema.ContentModelID(1<<30), xsdSchema.Occurrence{Min: 1, Max: 1})
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
}

func TestFreezeRejectsInvalidElementMasks(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(decl *xsdSchema.ElementDecl)
	}{
		{
			name: "invalid block",
			mutate: func(decl *xsdSchema.ElementDecl) {
				decl.Block = xsdSchema.DerivationList
			},
		},
		{
			name: "invalid final",
			mutate: func(decl *xsdSchema.ElementDecl) {
				decl.Final = xsdSchema.DerivationSubstitution
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			root := rt.GlobalElements[mustQName(t, &rt.Names, "root")]
			tc.mutate(&rt.Elements[root])
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsParticleWithStaleInactiveFields(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element name="child" type="xs:string"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild)
	}{
		{
			name: "model particle",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				for i := range rt.Models {
					for j := range rt.Models[i].Particles {
						p := &rt.Models[i].Particles[j]
						if p.Kind == xsdSchema.ParticleElement {
							p.Wildcard = 0
							return
						}
					}
				}
				t.Fatal("no element particle found")
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsInconsistentComplexContent(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="S">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="E">
    <xs:sequence><xs:element name="child"/></xs:sequence>
  </xs:complexType>
  <xs:element name="s" type="S"/>
  <xs:element name="e" type="E"/>
</xs:schema>`
	complexID := func(t *testing.T, rt *xsdSchema.SchemaBuild, local string) xsdSchema.ComplexTypeID {
		t.Helper()
		typ := rt.GlobalTypes[mustQName(t, &rt.Names, local)]
		id, ok := typ.Complex()
		if !ok {
			t.Fatalf("%s is not a complex type", local)
		}
		return id
	}
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *xsdSchema.SchemaBuild)
	}{
		{
			name: "text type without simple content",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.ComplexTypes[complexID(t, rt, "E")].TextType = rt.Builtin.String
			},
		},
		{
			name: "simple content with particles",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				elementOnly := rt.ComplexTypes[complexID(t, rt, "E")]
				rt.ComplexTypes[complexID(t, rt, "S")].Content = elementOnly.Content
			},
		},
		{
			name: "simple content with invalid text type",
			mutate: func(t *testing.T, rt *xsdSchema.SchemaBuild) {
				t.Helper()
				rt.ComplexTypes[complexID(t, rt, "S")].TextType = xsdSchema.SimpleTypeID(1 << 30)
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, rt)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestPublishedElementValueConstraints(t *testing.T) {
	build := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="SimpleContent"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Mixed" mixed="true"/>
  <xs:element name="plain0" type="xs:string"/>
  <xs:element name="fixed" type="xs:decimal" fixed="5" abstract="true" nillable="true" block="restriction"/>
  <xs:element name="gap1" type="xs:string"/>
  <xs:element name="default" type="xs:string" default="shared"/>
  <xs:element name="gap2" type="xs:string"/>
  <xs:element name="fixedSharedA" type="xs:decimal" fixed="5"/>
  <xs:element name="gap3" type="xs:string"/>
  <xs:element name="fixedSharedB" type="xs:decimal" fixed="5"/>
  <xs:element name="simpleContent" type="SimpleContent" default="sc"/>
  <xs:element name="gap4" type="xs:string"/>
  <xs:element name="defaultShared" type="xs:string" default="shared"/>
  <xs:element name="emptyDefault" type="xs:string" default=""/>
  <xs:element name="mixed" type="Mixed" default="mixed"/>
</xs:schema>`)
	stringID, decimalID := build.Builtin.String, build.Builtin.Decimal
	simpleContent := xsdSchema.ComplexRef(complexBuildTypeIDByName(t, build, "SimpleContent"))
	mixed := xsdSchema.ComplexRef(complexBuildTypeIDByName(t, build, "Mixed"))
	tests := []struct {
		name, lexical, canonical       string
		owner                          xsdSchema.TypeID
		valueType                      xsdSchema.SimpleTypeID
		fixed, def, abstract, nillable bool
		block                          xsdSchema.DerivationMask
	}{
		{name: "plain0", owner: xsdSchema.SimpleRef(stringID)},
		{name: "fixed", owner: xsdSchema.SimpleRef(decimalID), valueType: decimalID, lexical: "5.0", canonical: "5.0", fixed: true, abstract: true, nillable: true, block: xsdSchema.DerivationRestriction},
		{name: "gap1", owner: xsdSchema.SimpleRef(stringID)},
		{name: "default", owner: xsdSchema.SimpleRef(stringID), valueType: stringID, lexical: "shared", canonical: "shared", def: true},
		{name: "gap2", owner: xsdSchema.SimpleRef(stringID)},
		{name: "fixedSharedA", owner: xsdSchema.SimpleRef(decimalID), valueType: decimalID, lexical: "5.0", canonical: "5.0", fixed: true},
		{name: "gap3", owner: xsdSchema.SimpleRef(stringID)},
		{name: "fixedSharedB", owner: xsdSchema.SimpleRef(decimalID), valueType: decimalID, lexical: "5.0", canonical: "5.0", fixed: true},
		{name: "simpleContent", owner: simpleContent, valueType: stringID, lexical: "sc", canonical: "sc", def: true},
		{name: "gap4", owner: xsdSchema.SimpleRef(stringID)},
		{name: "defaultShared", owner: xsdSchema.SimpleRef(stringID), valueType: stringID, lexical: "shared", canonical: "shared", def: true},
		{name: "emptyDefault", owner: xsdSchema.SimpleRef(stringID), valueType: stringID, def: true},
		{name: "mixed", owner: mixed, valueType: xsdSchema.NoSimpleType, lexical: "mixed", canonical: "mixed", def: true},
	}
	ids := make(map[string]xsdSchema.ElementID, len(tests))
	names := make(map[string]xsdSchema.QName, len(tests))
	for _, test := range tests {
		q := mustQName(t, &build.Names, test.name)
		id, ok := build.GlobalElements[q]
		if !ok {
			t.Fatalf("element %q is missing from compiled fixture", test.name)
		}
		ids[test.name], names[test.name] = id, q
	}
	count := xsdSchema.ElementID(len(build.Elements)) //nolint:gosec // The fixed fixture contains thirteen element declarations.
	fixedAlias := build.Elements[ids["fixed"]].Fixed
	defaultAlias := build.Elements[ids["default"]].Default
	build.Elements[ids["fixedSharedA"]].Fixed = fixedAlias
	build.Elements[ids["fixedSharedB"]].Fixed = fixedAlias
	build.Elements[ids["defaultShared"]].Default = defaultAlias
	published, err := publishSchema(build)
	if err != nil {
		t.Fatalf("PublishSchema() error = %v", err)
	}
	for phase, label := range []string{"published", "after source alias mutation"} {
		if phase == 1 {
			fixedAlias.Lexical, fixedAlias.Canonical, fixedAlias.Value = "poison", "poison", valuepkg.Value{}
			defaultAlias.Lexical, defaultAlias.Canonical, defaultAlias.Value = "poison", "poison", valuepkg.Value{}
		}
		for _, test := range tests {
			t.Run(label+"/"+test.name, func(t *testing.T) {
				id := ids[test.name]
				constraints, present, valid := published.ElementValueConstraints(id)
				if !present || !valid || constraints.OwnerType() != test.owner || constraints.HasAny() != (test.fixed || test.def) {
					t.Fatalf("ElementValueConstraints(%d) = %+v, %v, %v; want owner %v, any %v, present/valid", id, constraints, present, valid, test.owner, test.fixed || test.def)
				}
				fixed, hasFixed := constraints.FixedValue()
				def, hasDefault := constraints.DefaultValueConstraint()
				if hasFixed != test.fixed || hasDefault != test.def {
					t.Fatalf("fixed/default presence = %v/%v, want %v/%v", hasFixed, hasDefault, test.fixed, test.def)
				}
				value := def
				if test.fixed {
					value = fixed
				}
				if test.fixed || test.def {
					if value.ApplicationText() != test.lexical || value.CanonicalText() != test.canonical || value.Value().CanonicalText() != test.canonical || value.Value().Type() != test.valueType {
						t.Fatalf("constraint = lexical %q, canonical %q, value %+v; want %q, %q, type %d", value.ApplicationText(), value.CanonicalText(), value.Value(), test.lexical, test.canonical, test.valueType)
					}
				}
				if (!test.fixed && fixed != (xsdSchema.ValueConstraintRead{})) || (!test.def && def != (xsdSchema.ValueConstraintRead{})) {
					t.Fatalf("absent constraint retained value: fixed %+v, default %+v", fixed, def)
				}
				wantStart := xsdSchema.ElementStartInfo{Type: test.owner, Block: test.block, Abstract: test.abstract, Nillable: test.nillable, Fixed: test.fixed, Default: test.def}
				if start, ok := published.Element(id); !ok || start != wantStart {
					t.Fatalf("Element(%d) = %+v, %v; want %+v", id, start, ok, wantStart)
				}
				rootID, start, ok := published.RootElement(xsdSchema.RuntimeName{Name: names[test.name], Known: true})
				if !ok || rootID != id || start != wantStart {
					t.Fatalf("RootElement(%q) = %d, %+v, %v; want %d, %+v", test.name, rootID, start, ok, id, wantStart)
				}
			})
		}
	}
	for _, id := range []xsdSchema.ElementID{xsdSchema.NoElement, count, count + 1} {
		constraints, present, valid := published.ElementValueConstraints(id)
		if constraints != (xsdSchema.ElementValueConstraints{}) || present || valid != (id == xsdSchema.NoElement) {
			t.Fatalf("ElementValueConstraints(%d) = %+v, %v, %v; want zero, absent, validity %v", id, constraints, present, valid, id == xsdSchema.NoElement)
		}
	}
}
