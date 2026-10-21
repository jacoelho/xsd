package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func BenchmarkSessionValidateXSIHintIdentity(b *testing.B) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="key"><xs:complexType>
        <xs:attribute name="uri" type="xs:anyURI" use="required"/>
      </xs:complexType></xs:element>
      <xs:element name="ref"><xs:complexType/></xs:element>
    </xs:sequence></xs:complexType>
    <xs:key name="uri"><xs:selector xpath="key"/><xs:field xpath="@uri"/></xs:key>
    <xs:keyref name="hint" refer="uri"><xs:selector xpath="ref"/><xs:field xpath="@xsi:noNamespaceSchemaLocation"/></xs:keyref>
  </xs:element>
</xs:schema>`
	const document = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><key uri="one.xsd"/><ref xsi:noNamespaceSchemaLocation="&#x9;one.xsd&#xA;"/></root>`
	const unequal = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><key uri="one.xsd"/><ref xsi:noNamespaceSchemaLocation="other.xsd"/></root>`
	engine, err := xsd.Compile(xsd.Bytes("hint-identity.xsd", []byte(schema)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	reader := strings.NewReader(unequal)
	var identityErr *xsderrors.Error
	if err := session.Validate(reader); !errors.As(err, &identityErr) || identityErr.Code() != xsderrors.CodeValidationIdentity {
		b.Fatalf("unequal URI validation = %v; want identity failure", err)
	}
	reader.Reset(document)
	if err := session.Validate(reader); err != nil {
		b.Fatalf("equivalent URI validation = %v", err)
	}
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	for b.Loop() {
		reader.Reset(document)
		if err := session.Validate(reader); err != nil {
			b.Fatal(err)
		}
	}
}
