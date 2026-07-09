package compile_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func mutableSchemaBuild(t *testing.T, schema string) *runtime.SchemaBuild {
	t.Helper()
	return compiledCompilerRuntime(t, schema).RuntimeForTest()
}

func validateSchemaBuild(build *runtime.SchemaBuild) error {
	schema, err := runtime.PublishSchema(*build)
	if err != nil {
		return err
	}
	return runtime.ValidateSchema(schema)
}

func rootBuildContentModel(t *testing.T, build *runtime.SchemaBuild) runtime.ContentModelID {
	t.Helper()
	root := build.GlobalElements[mustQName(t, build, rootContentModelName)]
	typ, ok := build.Elements[root].Type.Complex()
	if !ok {
		t.Fatal("root type is not complex")
	}
	return build.ComplexTypes[typ].Content
}

func rootBuildAttributeUseSet(t *testing.T, build *runtime.SchemaBuild) *runtime.AttributeUseSet {
	t.Helper()
	root := build.GlobalElements[mustQName(t, build, "root")]
	typ, ok := build.Elements[root].Type.Complex()
	if !ok {
		t.Fatal("root type is not complex")
	}
	attrs := build.ComplexTypes[typ].Attrs
	if attrs == runtime.NoAttributeUseSet {
		t.Fatal("root complex type has no attribute use set")
	}
	return &build.AttributeUseSets[attrs]
}

func complexBuildTypeIDByName(t *testing.T, build *runtime.SchemaBuild, local string) runtime.ComplexTypeID {
	t.Helper()
	typ, ok := build.GlobalType(mustQName(t, build, local))
	if !ok {
		t.Fatalf("global type %q not found", local)
	}
	id, ok := typ.Complex()
	if !ok {
		t.Fatalf("global type %q is not complex", local)
	}
	return id
}

func simpleBuildTypeIDByName(t *testing.T, build *runtime.SchemaBuild, local string) runtime.SimpleTypeID {
	t.Helper()
	typ, ok := build.GlobalType(mustQName(t, build, local))
	if !ok {
		t.Fatalf("global type %q not found", local)
	}
	id, ok := typ.Simple()
	if !ok {
		t.Fatalf("global type %q is not simple", local)
	}
	return id
}

func buildValueConstraint(t *testing.T, build *runtime.SchemaBuild, id runtime.SimpleTypeID, lexical string) *runtime.ValueConstraint {
	t.Helper()
	rt, err := runtime.PublishSchema(*build)
	if err != nil {
		t.Fatalf("PublishSchema() error = %v", err)
	}
	value, err := rt.ValidateSimpleValue(id, lexical, nil, runtime.SimpleNeedCanonical|runtime.SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(%q) error = %v", lexical, err)
	}
	return &runtime.ValueConstraint{Lexical: lexical, Canonical: value.Canonical, Value: value}
}

func cloneBuildValueConstraint(in *runtime.ValueConstraint) *runtime.ValueConstraint {
	if in == nil {
		return nil
	}
	out := new(runtime.ValueConstraint)
	*out = *in
	out.ResolvedNames = append([]runtime.ResolvedValueName(nil), in.ResolvedNames...)
	return out
}

func mutateBuildBoundFacet(t *testing.T, facets *runtime.FacetSet, flag runtime.FacetMask, mutate func(*runtime.CompiledLiteral)) {
	t.Helper()
	lit, ok := runtime.BoundFacet(*facets, flag)
	if !ok {
		t.Fatalf("bound facet %d is missing", flag)
	}
	mutate(&lit)
	runtime.SetBoundFacet(facets, flag, lit, false)
}

func TestFreezeRejectsSubstitutionClosureDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" substitutionGroup="head" type="xs:string"/>
  <xs:element name="other" type="xs:string"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.SchemaBuild, head, member, other runtime.ElementID)
	}{
		{
			name: "phantom substitution member",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, head, _, other runtime.ElementID) {
				t.Helper()
				rt.Substitutions[head] = append(rt.Substitutions[head], other)
				rt.SubstitutionLookup[head][rt.Elements[other].Name] = other
			},
		},
		{
			name: "missing declared member",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, head, _, _ runtime.ElementID) {
				t.Helper()
				rt.Substitutions[head] = nil
				rt.SubstitutionLookup[head] = nil
			},
		},
		{
			name: "stale lookup",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, head, member, _ runtime.ElementID) {
				t.Helper()
				delete(rt.SubstitutionLookup[head], rt.Elements[member].Name)
			},
		},
		{
			name: "cycle",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, head, member, _ runtime.ElementID) {
				t.Helper()
				rt.Elements[head].SubstHead = member
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			head := rt.GlobalElements[mustQName(t, rt, "head")]
			member := rt.GlobalElements[mustQName(t, rt, "member")]
			other := rt.GlobalElements[mustQName(t, rt, "other")]
			tc.mutate(t, rt, head, member, other)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsSubstitutionMapsWithoutHeads(t *testing.T) {
	rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	root := rt.GlobalElements[mustQName(t, rt, "root")]
	rt.Substitutions = map[runtime.ElementID][]runtime.ElementID{root: {root}}
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
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
	wildcardByMode := func(t *testing.T, rt *runtime.SchemaBuild, mode runtime.WildcardMode) *runtime.Wildcard {
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
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "invalid process",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				wildcardByMode(t, rt, runtime.WildcardList).Process = runtime.ProcessContents(99)
			},
		},
		{
			name: "invalid mode",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				wildcardByMode(t, rt, runtime.WildcardList).Mode = runtime.WildcardMode(99)
			},
		},
		{
			name: "stale other field",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				ns, ok := rt.Names.LookupNamespace("urn:a")
				if !ok {
					t.Fatal("urn:a namespace not interned")
				}
				wildcardByMode(t, rt, runtime.WildcardList).OtherThan = ns
			},
		},
		{
			name: "unnormalized namespace list",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				w := wildcardByMode(t, rt, runtime.WildcardList)
				if len(w.Namespaces) < 2 {
					t.Fatalf("wildcard namespace list length = %d, want >= 2", len(w.Namespaces))
				}
				w.Namespaces[0], w.Namespaces[1] = w.Namespaces[1], w.Namespaces[0]
			},
		},
		{
			name: "invalid namespace id",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				wildcardByMode(t, rt, runtime.WildcardOther).OtherThan = runtime.NamespaceID(1 << 30)
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
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string" default="abc"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(rt *runtime.SchemaBuild, decl *runtime.ElementDecl)
	}{
		{
			name: "canonical value mismatch",
			mutate: func(_ *runtime.SchemaBuild, decl *runtime.ElementDecl) {
				decl.Default.Value.Canonical = "other"
			},
		},
		{
			name: "invalid value type",
			mutate: func(_ *runtime.SchemaBuild, decl *runtime.ElementDecl) {
				decl.Default.Value.Type = runtime.SimpleTypeID(1 << 30)
			},
		},
		{
			name: "stale valid value type",
			mutate: func(rt *runtime.SchemaBuild, decl *runtime.ElementDecl) {
				decl.Default.Value.Type = rt.Builtin.Boolean
			},
		},
		{
			name: "stale identity key",
			mutate: func(_ *runtime.SchemaBuild, decl *runtime.ElementDecl) {
				decl.Default.Value.Identity = "stale"
			},
		},
		{
			name: "stale idref payload",
			mutate: func(_ *runtime.SchemaBuild, decl *runtime.ElementDecl) {
				decl.Default.Value.IDRefs = decl.Default.Value.Canonical
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			rootID := rt.GlobalElements[mustQName(t, rt, "root")]
			tc.mutate(rt, &rt.Elements[rootID])
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
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
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "element declaration",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				id := rt.GlobalElements[mustQName(t, rt, "value")]
				rt.Elements[id].Fixed = cloneBuildValueConstraint(rt.Elements[id].Default)
			},
		},
		{
			name: "attribute declaration",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				id := rt.GlobalAttributes[mustQName(t, rt, "ga")]
				rt.Attributes[id].Fixed = cloneBuildValueConstraint(rt.Attributes[id].Default)
			},
		},
		{
			name: "attribute use",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
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

func TestFreezeRejectsUnionValueConstraintStoredAsOwnerType(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
  <xs:element name="root" type="U" default="1"/>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := rt.GlobalElements[mustQName(t, rt, "root")]
	unionID := simpleBuildTypeIDByName(t, rt, "U")
	rt.Elements[rootID].Default.Value.Type = unionID
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
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
	rootID := rt.GlobalElements[mustQName(t, rt, "root")]
	defaultValue := rt.Elements[rootID].Default
	defaultValue.Lexical = "B"
	defaultValue.Canonical = "B"
	defaultValue.Value.Canonical = "B"
	defaultValue.Value.Identity = runtime.SimpleIdentityKey(runtime.PrimitiveString, "B")
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
	rootID := rt.GlobalElements[mustQName(t, rt, "root")]
	defaultValue := rt.Elements[rootID].Default
	defaultValue.Lexical = "bad::item"
	defaultValue.Canonical = runtime.FormatExpandedName("urn:test", "item")
	defaultValue.Value.Canonical = defaultValue.Canonical
	defaultValue.Value.Identity = runtime.SimpleIdentityKey(runtime.PrimitiveQName, defaultValue.Canonical)
	defaultValue.ResolvedNames = []runtime.ResolvedValueName{{Lexical: defaultValue.Lexical, NS: "urn:test", Local: "item"}}
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
	rootID := rt.GlobalElements[mustQName(t, rt, "root")]
	defaultValue := rt.Elements[rootID].Default
	defaultValue.ResolvedNames = append(defaultValue.ResolvedNames, runtime.ResolvedValueName{Lexical: "t:other", NS: "urn:test", Local: "other"})
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
	rootID := rt.GlobalElements[mustQName(t, rt, "root")]
	defaultValue := rt.Elements[rootID].Default
	if len(defaultValue.ResolvedNames) != 2 {
		t.Fatalf("resolved names = %d, want 2", len(defaultValue.ResolvedNames))
	}
	canonical := runtime.FormatExpandedName("urn:a", "x") + " " + runtime.FormatExpandedName("urn:b", "x")
	defaultValue.ResolvedNames[1].NS = "urn:b"
	defaultValue.Canonical = canonical
	defaultValue.Value.Canonical = canonical
	defaultValue.Value.Identity = runtime.SimpleIdentityKey(runtime.PrimitiveString, canonical)
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
	rootID := rt.GlobalElements[mustQName(t, rt, "root")]
	rt.Elements[rootID].Default.Value.Identity = "stale"
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
	rootID := rt.GlobalElements[mustQName(t, rt, "root")]
	rt.Elements[rootID].Default.ResolvedNames = []runtime.ResolvedValueName{{Lexical: "p:x", NS: "urn:test", Local: "x"}}
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsCyclicUnionValueConstraintOwner(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
  <xs:element name="root" type="U" default="1"/>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	unionID := simpleBuildTypeIDByName(t, rt, "U")
	rt.SimpleTypes[unionID].Union = []runtime.SimpleTypeID{unionID}
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
	rt.Names = runtime.NameTable{}
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
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "global element points at other declaration",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.GlobalElements[mustQName(t, rt, "a")] = rt.GlobalElements[mustQName(t, rt, "b")]
			},
		},
		{
			name: "global attribute points at other declaration",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.GlobalAttributes[mustQName(t, rt, "ga")] = rt.GlobalAttributes[mustQName(t, rt, "gb")]
			},
		},
		{
			name: "global type points at other type",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.GlobalTypes[mustQName(t, rt, "t1")] = rt.GlobalTypes[mustQName(t, rt, "t2")]
			},
		},
		{
			name: "global identity points at other constraint",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.GlobalIdentities[mustQName(t, rt, "k1")] = rt.GlobalIdentities[mustQName(t, rt, "k2")]
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
		mutate func(ic *runtime.IdentityConstraint)
	}{
		{
			name: "dropped attribute lookup",
			mutate: func(ic *runtime.IdentityConstraint) {
				ic.AttributeFields = nil
			},
		},
		{
			name: "element lookup field index drift",
			mutate: func(ic *runtime.IdentityConstraint) {
				ic.ElementFields[0].Field = 7
			},
		},
		{
			name: "extra wildcard lookup entry",
			mutate: func(ic *runtime.IdentityConstraint) {
				ic.AttributeWildcardFields = append(ic.AttributeWildcardFields, runtime.CompiledIdentityField{Field: 0})
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := rt.GlobalIdentities[mustQName(t, rt, "k1")]
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
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "key stores refer",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]].Refer = rt.GlobalIdentities[mustQName(t, rt, "kr1")]
			},
		},
		{
			name: "keyref missing refer",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "kr1")]].Refer = runtime.NoIdentityConstraint
			},
		},
		{
			name: "keyref references keyref",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "kr1")]].Refer = rt.GlobalIdentities[mustQName(t, rt, "kr2")]
			},
		},
		{
			name: "keyref field count drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				kr := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "kr1")]]
				kr.Fields = append(kr.Fields, runtime.IdentityField{})
				kr.ElementFields, kr.AttributeFields, kr.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(kr.Fields)
			},
		},
		{
			name: "missing selector",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]].Selector = nil
			},
		},
		{
			name: "missing fields",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields = nil
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "field without paths",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields[0].Paths = nil
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "selector self stores ignored path",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Selector[0] = runtime.IdentityPath{
					Self:       true,
					Descendant: true,
					Steps: []runtime.IdentityStep{{
						Name: mustQName(t, rt, "item"),
					}},
				}
			},
		},
		{
			name: "selector wildcard stores ignored name",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Selector[0].Steps[0].Wildcard = true
				ic.Selector[0].Steps[0].Name = runtime.QName{}
			},
		},
		{
			name: "field self stores ignored attribute",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields[0].Paths[0] = runtime.IdentityFieldPath{
					Self:      true,
					Attr:      true,
					Attribute: mustQName(t, rt, "id"),
				}
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "element field stores ignored attribute",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields[0].Paths[0] = runtime.IdentityFieldPath{
					Steps: []runtime.IdentityStep{{
						Name: mustQName(t, rt, "item"),
					}},
					Attribute: runtime.QName{},
				}
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "attribute wildcard stores ignored name",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields[0].Paths[0].AttrWildcard = true
				ic.Fields[0].Paths[0].Attribute = mustQName(t, rt, "id")
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
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
		set.Index = map[runtime.QName]uint32{mustQName(t, rt, "root"): 5}
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
		mutate func(set *runtime.AttributeUseSet)
	}{
		{
			name: "missing required slot",
			mutate: func(set *runtime.AttributeUseSet) {
				set.Required = nil
			},
		},
		{
			name: "missing value constraint slot",
			mutate: func(set *runtime.AttributeUseSet) {
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
	useByName := func(t *testing.T, rt *runtime.SchemaBuild, set *runtime.AttributeUseSet, local string) *runtime.AttributeUse {
		t.Helper()
		name := mustQName(t, rt, local)
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
		mutate func(t *testing.T, rt *runtime.SchemaBuild, set *runtime.AttributeUseSet)
	}{
		{
			name: "plain prohibited",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, set *runtime.AttributeUseSet) {
				t.Helper()
				useByName(t, rt, set, "plain").Prohibited = true
			},
		},
		{
			name: "prohibited with default",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, set *runtime.AttributeUseSet) {
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
		attr := rt.GlobalAttributes[mustQName(t, rt, "a")]
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
		root := rt.GlobalElements[mustQName(t, rt, "root")]
		rt.Elements[root].Type = runtime.SimpleRef(rt.Builtin.ID)
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
	root := rt.GlobalElements[mustQName(t, rt, "root")]
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

func TestFreezeRejectsBrokenDFARowIndex(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:string" abstract="true"/>
  <xs:element name="sub" type="xs:string" substitutionGroup="head"/>
  <xs:element name="r">
    <xs:complexType>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="c1" type="xs:string"/>
        <xs:element name="c2" type="xs:string"/>
        <xs:element name="c3" type="xs:string"/>
        <xs:element name="c4" type="xs:string"/>
        <xs:element name="c5" type="xs:string"/>
        <xs:element name="c6" type="xs:string"/>
        <xs:element name="c7" type="xs:string"/>
        <xs:element ref="head"/>
        <xs:any namespace="urn:a" processContents="lax"/>
        <xs:any namespace="urn:b" processContents="lax"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	indexedRow := func(t *testing.T, rt *runtime.SchemaBuild) *runtime.CompiledModelRow {
		t.Helper()
		model := rt.CompiledModels[rootBuildContentModel(t, rt)]
		for i := range model.Rows {
			if model.Rows[i].Index.IsEnabled() {
				return &model.Rows[i]
			}
		}
		t.Fatal("no indexed row in root content model")
		return nil
	}
	anyKey := func(t *testing.T, idx runtime.DFARowIndex) runtime.QName {
		t.Helper()
		for k := range idx.NameToEdge {
			return k
		}
		t.Fatal("name index is empty")
		return runtime.QName{}
	}
	mutations := []struct {
		name   string
		mutate func(t *testing.T, row *runtime.CompiledModelRow)
	}{
		{
			name: "name index position out of range",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				row.Index.NameToEdge[anyKey(t, row.Index)] = ^uint32(0)
			},
		},
		{
			name: "name index points at wildcard edge",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				row.Index.NameToEdge[anyKey(t, row.Index)] = row.Index.WildcardEdges[0]
			},
		},
		{
			name: "name index key does not match edge element",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				idx := row.Index
				a := anyKey(t, idx)
				own := idx.NameToEdge[a]
				for _, pos := range idx.NameToEdge {
					if pos != own {
						idx.NameToEdge[a] = pos
						return
					}
				}
				t.Fatal("name index has no second edge position")
			},
		},
		{
			name: "element edge missing from name index",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				delete(row.Index.NameToEdge, anyKey(t, row.Index))
			},
		},
		{
			name: "wildcard edge positions out of order",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				w := row.Index.WildcardEdges
				if len(w) < 2 {
					t.Fatalf("len(WildcardEdges) = %d, want >= 2", len(w))
				}
				w[0], w[1] = w[1], w[0]
			},
		},
		{
			name: "wildcard list contains element edge",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				row.Index.WildcardEdges[0] = row.Index.NameToEdge[anyKey(t, row.Index)]
			},
		},
		{
			name: "wildcard edge missing from wildcard list",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				row.Index.WildcardEdges = row.Index.WildcardEdges[:len(row.Index.WildcardEdges)-1]
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, indexedRow(t, rt))
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsAmbiguousDFARow(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	model := &rt.CompiledModels[rootBuildContentModel(t, rt)]
	for i := range model.Rows {
		row := &model.Rows[i]
		if row.Index.IsEnabled() || len(row.Edges) < 2 {
			continue
		}
		row.Edges[1].Particle = row.Edges[0].Particle
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		return
	}
	t.Fatal("no unindexed row with two edges")
}

func TestFreezeRejectsInconsistentSimpleVariety(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="atomicT"><xs:restriction base="xs:string"><xs:minLength value="1"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="listT"><xs:list itemType="xs:int"/></xs:simpleType>
  <xs:simpleType name="unionT"><xs:union memberTypes="xs:int xs:string"/></xs:simpleType>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	mutations := []struct {
		name     string
		typeName string
		mutate   func(rt *runtime.SchemaBuild, st *runtime.SimpleType)
	}{
		{
			name:     "atomic with union members",
			typeName: "atomicT",
			mutate: func(rt *runtime.SchemaBuild, st *runtime.SimpleType) {
				st.Union = []runtime.SimpleTypeID{rt.Builtin.String}
			},
		},
		{
			name:     "atomic with list item",
			typeName: "atomicT",
			mutate: func(rt *runtime.SchemaBuild, st *runtime.SimpleType) {
				st.ListItem = rt.Builtin.String
			},
		},
		{
			name:     "list without list item",
			typeName: "listT",
			mutate: func(rt *runtime.SchemaBuild, st *runtime.SimpleType) {
				st.ListItem = runtime.NoSimpleType
			},
		},
		{
			name:     "list with union members",
			typeName: "listT",
			mutate: func(rt *runtime.SchemaBuild, st *runtime.SimpleType) {
				st.Union = []runtime.SimpleTypeID{rt.Builtin.String}
			},
		},
		{
			name:     "union without members",
			typeName: "unionT",
			mutate: func(rt *runtime.SchemaBuild, st *runtime.SimpleType) {
				st.Union = nil
			},
		},
		{
			name:     "union with list item",
			typeName: "unionT",
			mutate: func(rt *runtime.SchemaBuild, st *runtime.SimpleType) {
				st.ListItem = rt.Builtin.String
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id, ok := rt.GlobalTypes[mustQName(t, rt, tc.typeName)].Simple()
			if !ok {
				t.Fatalf("%s is not a simple type", tc.typeName)
			}
			tc.mutate(rt, &rt.SimpleTypes[id])
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsZeroTypeID(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="CT"><xs:sequence/></xs:complexType>
  <xs:element name="root" type="CT"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "element type",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rootID := rt.GlobalElements[mustQName(t, rt, "root")]
				rt.Elements[rootID].Type = runtime.TypeID{}
			},
		},
		{
			name: "complex type base",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				ctID, ok := rt.GlobalTypes[mustQName(t, rt, "CT")].Complex()
				if !ok {
					t.Fatal("CT is not a complex type")
				}
				rt.ComplexTypes[ctID].Base = runtime.TypeID{}
			},
		},
		{
			name: "global type",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.GlobalTypes[mustQName(t, rt, "CT")] = runtime.TypeID{}
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

func TestFreezeRejectsMisclassifiedSimpleIdentity(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Ref"><xs:restriction base="xs:IDREF"/></xs:simpleType>
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:element name="root" type="Plain"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "idref restriction loses identity",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				id, ok := rt.GlobalTypes[mustQName(t, rt, "Ref")].Simple()
				if !ok {
					t.Fatal("Ref is not a simple type")
				}
				rt.SimpleTypes[id].Identity = runtime.SimpleIdentityNone
			},
		},
		{
			name: "plain type gains identity",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				id, ok := rt.GlobalTypes[mustQName(t, rt, "Plain")].Simple()
				if !ok {
					t.Fatal("Plain is not a simple type")
				}
				rt.SimpleTypes[id].Identity = runtime.SimpleIdentityID
			},
		},
		{
			name: "builtin ID loses identity",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.SimpleTypes[rt.Builtin.ID].Identity = runtime.SimpleIdentityNone
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

func TestFreezeRejectsInvalidSimpleTypeEnums(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(st *runtime.SimpleType)
	}{
		{
			name: "invalid primitive",
			mutate: func(st *runtime.SimpleType) {
				st.Primitive = runtime.PrimitiveKind(255)
			},
		},
		{
			name: "invalid whitespace",
			mutate: func(st *runtime.SimpleType) {
				st.Whitespace = runtime.WhitespaceMode(255)
			},
		},
		{
			name: "invalid builtin validation",
			mutate: func(st *runtime.SimpleType) {
				st.Builtin = runtime.BuiltinValidationKind(255)
			},
		},
		{
			name: "invalid final mask",
			mutate: func(st *runtime.SimpleType) {
				st.Final = runtime.DerivationExtension
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleBuildTypeIDByName(t, rt, "Plain")
			tc.mutate(&rt.SimpleTypes[id])
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsSimpleTypeSemanticDrift(t *testing.T) {
	rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	id := simpleBuildTypeIDByName(t, rt, "Plain")
	rt.SimpleTypes[id].Primitive = runtime.PrimitiveBoolean
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsBuiltinHandleDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "simple handle points at wrong valid type",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.Builtin.String = rt.Builtin.Boolean
			},
		},
		{
			name: "simple shape drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.SimpleTypes[rt.Builtin.String].Whitespace = runtime.WhitespaceCollapse
			},
		},
		{
			name: "global type binding drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				q, ok := rt.Names.LookupQName(vocab.XSDNamespaceURI, "string")
				if !ok {
					t.Fatal("xs:string name not found")
				}
				rt.GlobalTypes[q] = runtime.SimpleRef(rt.Builtin.Boolean)
			},
		},
		{
			name: "missing builtin declaration table",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.Wildcards = nil
			},
		},
		{
			name: "builtin attribute handle drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
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
			name: "builtin attribute lexical validator drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				q, ok := rt.Names.LookupQName(vocab.XMLNamespaceURI, vocab.XMLAttrLang)
				if !ok {
					t.Fatal("xml:lang name not found")
				}
				id, ok := rt.GlobalAttributes[q]
				if !ok {
					t.Fatal("xml:lang attribute not found")
				}
				rt.SimpleTypes[rt.Attributes[id].Type].Builtin = runtime.BuiltinValidationNone
			},
		},
		{
			name: "anyType shape drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.ComplexTypes[rt.Builtin.AnyType].ContentKind = runtime.ContentElementOnly
			},
		},
		{
			name: "builtin list item drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.SimpleTypes[rt.Builtin.IDREFS].ListItem = rt.Builtin.String
			},
		},
		{
			name: "builtin facet drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				mutateBuildBoundFacet(t, &rt.SimpleTypes[rt.Builtin.Int].Facets, runtime.FacetMaxInclusive, func(lit *runtime.CompiledLiteral) {
					lit.Canonical = "1"
				})
			},
		},
		{
			name: "anyType wildcard drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				attrs := rt.ComplexTypes[rt.Builtin.AnyType].Attrs
				rt.AttributeUseSets[attrs].Wildcard = runtime.NoWildcard
			},
		},
		{
			name: "non-handle builtin drift",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				q, ok := rt.Names.LookupQName(vocab.XSDNamespaceURI, "long")
				if !ok {
					t.Fatal("xs:long name not found")
				}
				id, ok := rt.GlobalTypes[q].Simple()
				if !ok {
					t.Fatal("xs:long is not simple")
				}
				rt.SimpleTypes[id].Whitespace = runtime.WhitespacePreserve
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
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "invalid kind",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.Models[rootBuildContentModel(t, rt)].Kind = runtime.ModelKind(255)
			},
		},
		{
			name: "invalid occurrence range",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rootBuildContentModel(t, rt)]
				model.Occurs = runtime.Occurrence{Min: 2, Max: 1}
			},
		},
		{
			name: "unsorted choice limits",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rootBuildContentModel(t, rt)]
				model.ChoiceLimits = []uint32{1, 0}
			},
		},
		{
			name: "unjustified choice limits",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rootBuildContentModel(t, rt)]
				model.ChoiceLimits = []uint32{1}
			},
		},
		{
			name: "choice limit on non-sequence",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rootBuildContentModel(t, rt)]
				model.Kind = runtime.ModelChoice
				model.ChoiceLimits = []uint32{1}
			},
		},
		{
			name: "any model inactive state",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				model := &rt.Models[rt.ComplexTypes[rt.Builtin.AnyType].Content]
				model.Occurs = runtime.Occurrence{Min: 1, Max: 1}
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

func TestFreezeRejectsSimpleFacetLoosening(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		mutate func(*runtime.FacetSet)
	}{
		{
			name: "length",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:length value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.Length = 3
			},
		},
		{
			name: "minLength",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:minLength value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.MinLength = 1
			},
		},
		{
			name: "maxLength",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:maxLength value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.MaxLength = 3
			},
		},
		{
			name: "totalDigits",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:decimal"><xs:totalDigits value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.TotalDigits = 3
			},
		},
		{
			name: "fractionDigits",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:decimal"><xs:fractionDigits value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.FractionDigits = 3
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, tt.schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleBuildTypeIDByName(t, rt, "Derived")
			tt.mutate(&rt.SimpleTypes[id].Facets)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsFixedFacetMutation(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		mutate func(*runtime.SchemaBuild, *runtime.SimpleType)
	}{
		{
			name: "fixed maxLength",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:maxLength value="5" fixed="true"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"><xs:maxLength value="5"/></xs:restriction></xs:simpleType>
</xs:schema>`,
			mutate: func(_ *runtime.SchemaBuild, st *runtime.SimpleType) {
				st.Facets.MaxLength = 4
			},
		},
		{
			name: "fixed whiteSpace",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:whiteSpace value="replace" fixed="true"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"><xs:whiteSpace value="replace"/></xs:restriction></xs:simpleType>
</xs:schema>`,
			mutate: func(_ *runtime.SchemaBuild, st *runtime.SimpleType) {
				st.Whitespace = runtime.WhitespaceCollapse
			},
		},
		{
			name: "fixed ordered literal",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:decimal"><xs:minInclusive value="5" fixed="true"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Other"><xs:restriction base="xs:decimal"><xs:minInclusive value="6"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"><xs:minInclusive value="5.0"/></xs:restriction></xs:simpleType>
</xs:schema>`,
			mutate: func(rt *runtime.SchemaBuild, st *runtime.SimpleType) {
				id := simpleBuildTypeIDByName(t, rt, "Other")
				lit, ok := runtime.BoundFacet(rt.SimpleTypes[id].Facets, runtime.FacetMinInclusive)
				if !ok {
					t.Fatal("Other minInclusive facet is missing")
				}
				runtime.SetBoundFacet(&st.Facets, runtime.FacetMinInclusive, lit, false)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, tt.schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleBuildTypeIDByName(t, rt, "Derived")
			tt.mutate(rt, &rt.SimpleTypes[id])
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsInheritedFacetLoss(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		mutate func(*runtime.FacetSet)
	}{
		{
			name: "totalDigits",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:decimal"><xs:totalDigits value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.TotalDigits = 0
				f.Present &^= runtime.FacetTotalDigits
			},
		},
		{
			name: "date minInclusive",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:date"><xs:minInclusive value="2020-01-01"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				mutateBuildBoundFacet(t, f, runtime.FacetMinInclusive, func(lit *runtime.CompiledLiteral) {
					*lit = runtime.CompiledLiteral{}
				})
				f.Present &^= runtime.FacetMinInclusive
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, tt.schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleBuildTypeIDByName(t, rt, "Derived")
			tt.mutate(&rt.SimpleTypes[id].Facets)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsOrderedFacetLoosening(t *testing.T) {
	rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:date"><xs:minInclusive value="2020-01-01"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Earlier"><xs:restriction base="xs:date"><xs:minInclusive value="2019-01-01"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"><xs:minInclusive value="2021-01-01"/></xs:restriction></xs:simpleType>
</xs:schema>`)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	derivedID := simpleBuildTypeIDByName(t, rt, "Derived")
	earlierID := simpleBuildTypeIDByName(t, rt, "Earlier")
	lit, ok := runtime.BoundFacet(rt.SimpleTypes[earlierID].Facets, runtime.FacetMinInclusive)
	if !ok {
		t.Fatal("Earlier minInclusive facet is missing")
	}
	runtime.SetBoundFacet(&rt.SimpleTypes[derivedID].Facets, runtime.FacetMinInclusive, lit, false)
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsLengthFacetInconsistency(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*runtime.FacetSet)
	}{
		{
			name: "length differs from minLength",
			mutate: func(f *runtime.FacetSet) {
				f.MinLength = 1
			},
		},
		{
			name: "length differs from maxLength",
			mutate: func(f *runtime.FacetSet) {
				f.MaxLength = 3
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Sized">
    <xs:restriction base="xs:string">
      <xs:length value="2"/>
      <xs:minLength value="2"/>
      <xs:maxLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleBuildTypeIDByName(t, rt, "Sized")
			tt.mutate(&rt.SimpleTypes[id].Facets)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsSimpleTypeGraphInvalidity(t *testing.T) {
	t.Run("base cycle", func(t *testing.T) {
		rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="A"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:simpleType name="B"><xs:restriction base="A"/></xs:simpleType>
</xs:schema>`)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		a := simpleBuildTypeIDByName(t, rt, "A")
		b := simpleBuildTypeIDByName(t, rt, "B")
		rt.SimpleTypes[a].Base = b
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("list item is list variety", func(t *testing.T) {
		rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Words"><xs:list itemType="xs:string"/></xs:simpleType>
  <xs:simpleType name="Bad"><xs:list itemType="xs:string"/></xs:simpleType>
</xs:schema>`)
		if err := validateSchemaBuild(rt); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		words := simpleBuildTypeIDByName(t, rt, "Words")
		bad := simpleBuildTypeIDByName(t, rt, "Bad")
		rt.SimpleTypes[bad].ListItem = words
		err := validateSchemaBuild(rt)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
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
	limited := runtime.NoContentModel
	for id, model := range rt.Models {
		if len(model.ChoiceLimits) != 0 {
			limited = runtime.ContentModelID(id)
			break
		}
	}
	if limited == runtime.NoContentModel {
		t.Fatal("no limited content model found")
	}
	otherID, ok := rt.GlobalTypes[mustQName(t, rt, "Other")].Complex()
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
		mutate func(t *testing.T, rt *runtime.SchemaBuild, ct *runtime.ComplexType)
	}{
		{
			name: "missing content",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, ct *runtime.ComplexType) {
				t.Helper()
				ct.Content = runtime.NoContentModel
			},
		},
		{
			name: "missing attrs",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, ct *runtime.ComplexType) {
				t.Helper()
				ct.Attrs = runtime.NoAttributeUseSet
			},
		},
		{
			name: "invalid content kind",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, ct *runtime.ComplexType) {
				t.Helper()
				ct.ContentKind = runtime.ContentKind(255)
			},
		},
		{
			name: "invalid derivation",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, ct *runtime.ComplexType) {
				t.Helper()
				ct.Derivation = runtime.DerivationKind(255)
			},
		},
		{
			name: "invalid block mask",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, ct *runtime.ComplexType) {
				t.Helper()
				ct.Block = runtime.DerivationSubstitution
			},
		},
		{
			name: "invalid final mask",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild, ct *runtime.ComplexType) {
				t.Helper()
				ct.Final = runtime.DerivationSubstitution
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			root := rt.GlobalElements[mustQName(t, rt, "root")]
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
	if set.WildcardDeclared == runtime.NoWildcard {
		t.Fatal("derived attribute wildcard did not record declared wildcard")
	}
	set.WildcardBase = runtime.NoWildcard
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
		bad := runtime.WildcardID(1 << 30)
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
		model.Particles[0] = runtime.ModelParticle(runtime.ContentModelID(1<<30), runtime.Occurrence{Min: 1, Max: 1})
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
		mutate func(decl *runtime.ElementDecl)
	}{
		{
			name: "invalid block",
			mutate: func(decl *runtime.ElementDecl) {
				decl.Block = runtime.DerivationList
			},
		},
		{
			name: "invalid final",
			mutate: func(decl *runtime.ElementDecl) {
				decl.Final = runtime.DerivationSubstitution
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			root := rt.GlobalElements[mustQName(t, rt, "root")]
			tc.mutate(&rt.Elements[root])
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsMisclassifiedSimpleFastPath(t *testing.T) {
	rt := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="MyInt"><xs:restriction base="xs:int"/></xs:simpleType>
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)

	id := simpleBuildTypeIDByName(t, rt, "MyInt")
	rt.SimpleTypes[id].Fast = runtime.SimpleFastNone
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)

	rt = mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)
	id = simpleBuildTypeIDByName(t, rt, "Plain")
	rt.SimpleTypes[id].Fast = runtime.SimpleFastInt
	err = validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInvalidPatternFacetRepresentation(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Patterned">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Patterned"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(f *runtime.FacetSet)
	}{
		{
			name: "empty pattern group",
			mutate: func(f *runtime.FacetSet) {
				f.Patterns[0].Patterns = nil
			},
		},
		{
			name: "pattern without matcher",
			mutate: func(f *runtime.FacetSet) {
				f.Patterns[0].Patterns[0] = runtime.StringPattern{}
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleBuildTypeIDByName(t, rt, "Patterned")
			tc.mutate(&rt.SimpleTypes[id].Facets)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsCompiledModelSourceMismatch(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence><xs:element name="child" type="xs:string"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(model *runtime.CompiledModel)
	}{
		{
			name: "source id drift",
			mutate: func(model *runtime.CompiledModel) {
				model.Source = runtime.NoContentModel
			},
		},
		{
			name: "kind drift",
			mutate: func(model *runtime.CompiledModel) {
				model.Kind = runtime.CompiledModelEmpty
				model.Rows = nil
				model.Empty = true
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			modelID := rootBuildContentModel(t, rt)
			tc.mutate(&rt.CompiledModels[modelID])
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
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "model particle",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				for i := range rt.Models {
					for j := range rt.Models[i].Particles {
						p := &rt.Models[i].Particles[j]
						if p.Kind == runtime.ParticleElement {
							p.Wildcard = 0
							return
						}
					}
				}
				t.Fatal("no element particle found")
			},
		},
		{
			name: "compiled edge particle",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				for i := range rt.CompiledModels {
					for j := range rt.CompiledModels[i].Rows {
						row := &rt.CompiledModels[i].Rows[j]
						for k := range row.Edges {
							if row.Edges[k].Particle.Kind == runtime.ParticleElement {
								row.Edges[k].Particle.Model = 0
								return
							}
						}
					}
				}
				t.Fatal("no compiled element edge found")
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

func TestFreezeRejectsFacetPresenceMismatch(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Sized">
    <xs:restriction base="xs:string">
      <xs:maxLength value="4"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Sized"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(f *runtime.FacetSet)
	}{
		{
			name: "bit without value",
			mutate: func(f *runtime.FacetSet) {
				f.Present |= runtime.FacetLength
			},
		},
		{
			name: "value without bit",
			mutate: func(f *runtime.FacetSet) {
				f.Present &^= runtime.FacetMaxLength
			},
		},
		{
			name: "whiteSpace bit in presence mask",
			mutate: func(f *runtime.FacetSet) {
				f.Present |= runtime.FacetWhiteSpace
			},
		},
		{
			name: "fixed facet without presence",
			mutate: func(f *runtime.FacetSet) {
				f.Fixed |= runtime.FacetMinInclusive
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			rt := mutableSchemaBuild(t, schema)
			if err := validateSchemaBuild(rt); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			typ := rt.GlobalTypes[mustQName(t, rt, "Sized")]
			id, ok := typ.Simple()
			if !ok {
				t.Fatal("Sized is not a simple type")
			}
			tc.mutate(&rt.SimpleTypes[id].Facets)
			err := validateSchemaBuild(rt)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsDecimalBoundWithoutActual(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Positive">
    <xs:restriction base="xs:int">
      <xs:minInclusive value="1"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Positive"/>
</xs:schema>`
	rt := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(rt); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	typ := rt.GlobalTypes[mustQName(t, rt, "Positive")]
	id, ok := typ.Simple()
	if !ok {
		t.Fatal("Positive is not a simple type")
	}
	mutateBuildBoundFacet(t, &rt.SimpleTypes[id].Facets, runtime.FacetMinInclusive, func(lit *runtime.CompiledLiteral) {
		lit.Actual = runtime.PrimitiveActualValue{}
	})
	err := validateSchemaBuild(rt)
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
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
	complexID := func(t *testing.T, rt *runtime.SchemaBuild, local string) runtime.ComplexTypeID {
		t.Helper()
		typ := rt.GlobalTypes[mustQName(t, rt, local)]
		id, ok := typ.Complex()
		if !ok {
			t.Fatalf("%s is not a complex type", local)
		}
		return id
	}
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.SchemaBuild)
	}{
		{
			name: "text type without simple content",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.ComplexTypes[complexID(t, rt, "E")].TextType = rt.Builtin.String
			},
		},
		{
			name: "simple content with particles",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				elementOnly := rt.ComplexTypes[complexID(t, rt, "E")]
				rt.ComplexTypes[complexID(t, rt, "S")].Content = elementOnly.Content
			},
		},
		{
			name: "simple content with invalid text type",
			mutate: func(t *testing.T, rt *runtime.SchemaBuild) {
				t.Helper()
				rt.ComplexTypes[complexID(t, rt, "S")].TextType = runtime.SimpleTypeID(1 << 30)
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
