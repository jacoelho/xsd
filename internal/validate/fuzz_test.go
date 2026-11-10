package validate

import (
	"errors"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func FuzzValidateNeverPanics(f *testing.F) {
	rt, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{
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
		session, err := newSessionForTest(rt, Options{})
		if err != nil {
			t.Fatal(err)
		}
		err = session.Validate(strings.NewReader(doc))
		for _, leaf := range xsderrors.Flatten(err) {
			diagnostic, ok := errors.AsType[*xsderrors.Error](leaf)
			if !ok {
				continue
			}
			if c := diagnostic.Category(); c != xsderrors.CategoryValidation && c != xsderrors.CategoryUnsupported {
				continue
			}
			if diagnostic.Path() == "" || diagnostic.Line() <= 0 {
				t.Fatalf("unlocated %s diagnostic %q for %q", diagnostic.Code(), diagnostic.Message(), doc)
			}
		}
	})
}
