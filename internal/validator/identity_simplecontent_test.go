package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestIdentitySimpleContentFieldSelection(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:sc"
           targetNamespace="urn:sc"
           elementFormDefault="qualified">
  <xs:complexType name="CodeType">
    <xs:simpleContent>
      <xs:extension base="xs:string"/>
    </xs:simpleContent>
  </xs:complexType>

  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="tns:CodeType" maxOccurs="unbounded"/>
        <xs:element name="ref" type="xs:string" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:key>
    <xs:keyref name="itemKeyRef" refer="tns:itemKey">
      <xs:selector xpath="tns:ref"/>
      <xs:field xpath="."/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	document := `<r:root xmlns:r="urn:sc">
  <r:item>alpha</r:item>
  <r:item>beta</r:item>
  <r:ref>alpha</r:ref>
  <r:ref>beta</r:ref>
</r:root>`

	v := mustNewValidator(t, schema)
	violations, err := v.ValidateStream(strings.NewReader(document))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if hasViolationCode(violations, errors.ErrIdentityAbsent) || hasViolationCode(violations, errors.ErrIdentityKeyRefFailed) {
		t.Fatalf("unexpected identity violations: %v", violations)
	}
}
