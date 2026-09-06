package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func TestSessionQNameLexicalReusePreservesNamespaceResolution(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="value" maxOccurs="unbounded">
          <xs:complexType>
            <xs:simpleContent>
              <xs:extension base="xs:QName">
                <xs:attribute name="name" type="xs:QName"/>
              </xs:extension>
            </xs:simpleContent>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	doc := `<root xmlns:p="urn:test">` + strings.Repeat(`<value name="p:item">p:item</value>`, 200) + `</root>`
	reader := strings.NewReader(doc)
	validate := func() {
		reader.Reset(doc)
		if err := session.Validate(reader); err != nil {
			t.Fatal(err)
		}
	}
	if allocs := testing.AllocsPerRun(100, validate); allocs != 0 {
		t.Fatalf("warmed QName element and attribute validation allocated %g times, want zero", allocs)
	}
	for _, unbound := range []string{
		`<root><value name="p:item">p:item</value></root>`,
		`<root><value>p:item</value></root>`,
	} {
		reader.Reset(unbound)
		if err := session.Validate(reader); err == nil {
			t.Fatal("cached spelling concealed an unbound QName prefix")
		}
		validate()
	}
}
