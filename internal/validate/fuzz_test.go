package validate

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/source"
)

func FuzzValidateNeverPanics(f *testing.F) {
	rt, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="id" type="xs:ID" minOccurs="0" maxOccurs="unbounded"/>
        <xs:element name="ref" type="xs:IDREF" minOccurs="0" maxOccurs="unbounded"/>
      </xs:sequence>
      <xs:attribute name="name" type="xs:string"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)),
	})
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{
		`<root/>`,
		`<root><id>a</id><ref>a</ref></root>`,
		`<root><id>a</id><id>a</id></root>`,
		`<root><ref>missing</ref></root>`,
		`<other/>`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, doc string) {
		if len(doc) > 4096 {
			t.Skip()
		}
		session, err := NewSession(rt, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			return
		}
	})
}
