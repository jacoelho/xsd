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
