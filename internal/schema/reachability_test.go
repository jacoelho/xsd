package schema

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
)

const reachabilityFixture = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G"><xs:sequence>
    <xs:element name="list"><xs:complexType><xs:sequence><xs:element name="k" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
      <xs:key name="shared"><xs:selector xpath="k"/><xs:field xpath="."/></xs:key>
    </xs:element>
  </xs:sequence></xs:group>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="gone" minOccurs="0" maxOccurs="0"><xs:complexType><xs:sequence>
      <xs:element name="mid"><xs:complexType><xs:group ref="G"/><xs:attribute name="a"/></xs:complexType>
        <xs:key name="hidden"><xs:selector xpath="list"/><xs:field xpath="."/></xs:key>
      </xs:element>
    </xs:sequence></xs:complexType></xs:element>
    <xs:sequence minOccurs="0" maxOccurs="0"><xs:element name="absent"><xs:complexType><xs:group ref="G"/></xs:complexType></xs:element></xs:sequence>
    <xs:element name="live"><xs:complexType><xs:group ref="G"/></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`

func TestDetachingUnreachableComponentsSpendsNoBudget(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCompilerForTest(limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadForTest([]source.Source{source.Bytes("schema.xsd", []byte(reachabilityFixture))}); err != nil {
		t.Fatal(err)
	}
	if err := c.IndexForTest(); err != nil {
		t.Fatal(err)
	}
	if err := c.CompileGlobalsForTest(); err != nil {
		t.Fatal(err)
	}
	dependency, content := c.dependencyWork.remaining, c.contentWork.remaining
	if err := c.detachUnreachableComponents(); err != nil {
		t.Fatalf("detachUnreachableComponents() error = %v", err)
	}
	if c.dependencyWork.remaining != dependency || c.contentWork.remaining != content {
		t.Fatalf("detachUnreachableComponents() spent %d dependency and %d content-model steps, want none",
			dependency-c.dependencyWork.remaining, content-c.contentWork.remaining)
	}
}

func TestPublicationRejectsUnreachableElementOwningIdentityConstraint(t *testing.T) {
	t.Parallel()

	build := compiledTestBuild(t, reachabilityFixture)
	owner := -1
	for i := range build.Elements {
		if !build.ReachableElements[i] && build.Elements[i].Scope == declarationScopeNonGlobal {
			owner = i
			break
		}
	}
	detached := -1
	for i := range build.Identities {
		if build.Identities[i].Detached {
			detached = i
			break
		}
	}
	if owner < 0 || detached < 0 {
		t.Fatalf("fixture has unreachable element %d and detached constraint %d, want both", owner, detached)
	}
	build.Elements[owner].Identity = []IdentityConstraintID{IdentityConstraintID(detached)}
	build.Identities[detached].Detached = false
	_, err := publishSchema(build, unlimitedContentModelWork)
	if err == nil || !strings.Contains(err.Error(), "unreachable element declaration owns an identity constraint") {
		t.Fatalf("publishSchema() error = %v, want an unreachable owner rejected", err)
	}
}
