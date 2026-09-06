package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func BenchmarkSessionValidateUnionLateMember(b *testing.B) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="item" maxOccurs="unbounded"><xs:simpleType><xs:union memberTypes="xs:boolean xs:string"/></xs:simpleType></xs:element></xs:sequence></xs:complexType></xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("union.xsd", []byte(schema)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	document := "<root>" + strings.Repeat("<item>value</item>", 256) + "</root>"
	if err := session.Validate(strings.NewReader(document)); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(document)); err != nil {
			b.Fatal(err)
		}
	}
}
