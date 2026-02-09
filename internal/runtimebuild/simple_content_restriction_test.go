package runtimebuild

import "testing"

func TestBuildSchemaSimpleContentRestrictionFromComplexType(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:sc"
           xmlns:tns="urn:sc"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:string"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:simpleContent>
      <xs:restriction base="tns:Base">
        <xs:maxLength value="4"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	_ = mustBuildRuntimeSchema(t, schemaXML)
}

func TestBuildSchemaSimpleContentRestrictionListBase(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:list"
           xmlns:tns="urn:list"
           elementFormDefault="qualified">
  <xs:simpleType name="ListType">
    <xs:list itemType="xs:decimal"/>
  </xs:simpleType>
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="tns:ListType"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:simpleContent>
      <xs:restriction base="tns:Base">
        <xs:length value="2"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	_ = mustBuildRuntimeSchema(t, schemaXML)
}
