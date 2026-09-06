package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

func TestPublicUnionEnumerationOverListMember(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:tns="urn:test" targetNamespace="urn:test" elementFormDefault="qualified">
  <xs:simpleType name="L"><xs:list itemType="xs:decimal"/></xs:simpleType>
  <xs:simpleType name="U"><xs:union memberTypes="tns:L xs:int"/></xs:simpleType>
  <xs:simpleType name="E">
    <xs:restriction base="tns:U"><xs:enumeration value="1.0 2.0"/></xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:E"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := engine.Validate(strings.NewReader(`<root xmlns="urn:test">1 2</root>`)); err != nil {
		t.Fatalf("Validate() equivalent list value = %v", err)
	}
	if err := engine.Validate(strings.NewReader(`<root xmlns="urn:test">1 3</root>`)); err == nil {
		t.Fatal("Validate() accepted a list value outside the union enumeration")
	}
}
