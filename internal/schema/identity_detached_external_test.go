package schema_test

import (
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
)

func TestDetachedIdentityConstraintPublishesNoProgramOrSelectorBranch(t *testing.T) {
	t.Parallel()
	rt := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">
<xs:element name="root"><xs:complexType><xs:sequence>
  <xs:element name="gone" minOccurs="0" maxOccurs="0">
    <xs:complexType><xs:sequence><xs:element name="k" type="xs:string"/></xs:sequence></xs:complexType>
    <xs:key name="detached"><xs:selector xpath="p:k"/><xs:field xpath="."/></xs:key>
  </xs:element>
  <xs:element name="k" type="xs:string" maxOccurs="unbounded"/>
</xs:sequence></xs:complexType>
<xs:key name="owned"><xs:selector xpath="p:k"/><xs:field xpath="."/></xs:key>
</xs:element></xs:schema>`)
	dispatch := rt.IdentityDispatch()
	if dispatch.ConstraintCount() != 2 {
		t.Fatalf("ConstraintCount() = %d, want 2", dispatch.ConstraintCount())
	}
	ids := map[string]xsdSchema.IdentityConstraintID{}
	for _, id := range []xsdSchema.IdentityConstraintID{0, 1} {
		name, ok := rt.DiagnosticIdentityName(id)
		if !ok {
			t.Fatalf("DiagnosticIdentityName(%d) failed", id)
		}
		ids[name.Local] = id
	}
	detached, owned := ids["detached"], ids["owned"]
	if len(ids) != 2 {
		t.Fatalf("published identity constraints = %v, want detached and owned", ids)
	}
	if _, ok := dispatch.Program(detached); ok {
		t.Fatal("Program(detached) published a program for a constraint no scope evaluates")
	}
	if _, ok := dispatch.Program(owned); !ok {
		t.Fatal("Program(owned) has no program")
	}
	k, ok := rt.LookupQName("urn:t", "k")
	if !ok {
		t.Fatal("LookupQName(k) failed")
	}
	branches := dispatch.SelectorExact(k)
	if branches.Len() != 1 {
		t.Fatalf("SelectorExact(k) = %d branches, want only the owned constraint's", branches.Len())
	}
	if branch, _ := branches.At(0); branch.Constraint() != owned {
		t.Fatalf("SelectorExact(k) branch constraint = %d, want owned %d", branch.Constraint(), owned)
	}
}

func TestSchemaWithOnlyDetachedIdentityConstraintsHasNoneToEvaluate(t *testing.T) {
	t.Parallel()
	rt := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:element name="root"><xs:complexType><xs:sequence>
  <xs:element name="gone" minOccurs="0" maxOccurs="0">
    <xs:complexType><xs:attribute name="a"/></xs:complexType>
    <xs:key name="detached"><xs:selector xpath="."/><xs:field xpath="@a"/></xs:key>
  </xs:element>
</xs:sequence></xs:complexType></xs:element></xs:schema>`)
	if rt.HasIdentityConstraints() {
		t.Fatal("HasIdentityConstraints() = true for a schema whose only constraint is detached")
	}
	if got := rt.IdentityDispatch().ConstraintCount(); got != 1 {
		t.Fatalf("ConstraintCount() = %d, want the detached constraint counted", got)
	}
}

func TestKeyNoInstanceReachesPublishesNoProgram(t *testing.T) {
	t.Parallel()
	const keyed = `<xs:element name="inner"><xs:complexType><xs:sequence><xs:element name="k" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
  <xs:key name="hidden"><xs:selector xpath="k"/><xs:field xpath="."/></xs:key></xs:element>`
	for _, test := range []struct {
		name   string
		group  string
		holder string
	}{
		{name: "two levels inside a zero-count element", holder: `<xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:sequence>
  <xs:element name="mid"><xs:complexType><xs:sequence>` + keyed + `</xs:sequence></xs:complexType></xs:element>
</xs:sequence></xs:complexType></xs:element>`},
		{name: "under a zero-count sequence", holder: `<xs:sequence minOccurs="0" maxOccurs="0">` + keyed + `</xs:sequence>`},
		{name: "in a group referenced only under a zero-count element", group: `<xs:group name="G"><xs:sequence>` + keyed + `</xs:sequence></xs:group>`, holder: `<xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:group ref="G"/></xs:complexType></xs:element>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rt := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.group+`
<xs:element name="root"><xs:complexType><xs:sequence>`+test.holder+`<xs:element name="live" type="xs:string"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
			if rt.HasIdentityConstraints() {
				t.Error("HasIdentityConstraints() = true, want false for a key no instance element reaches")
			}
			dispatch := rt.IdentityDispatch()
			if got := dispatch.ConstraintCount(); got != 1 {
				t.Fatalf("ConstraintCount() = %d, want the one declared key", got)
			}
			if _, ok := dispatch.Program(0); ok {
				t.Error("Program(hidden) published a program for a key no instance element reaches")
			}
		})
	}
}
