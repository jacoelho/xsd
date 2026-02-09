package runtimebuild

import "testing"

func TestDeterministicBuild(t *testing.T) {
	cases := []struct {
		name    string
		schema  string
		changed string
	}{
		{
			name: "basic",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`,
			changed: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:element name="root2" type="xs:string"/>
</xs:schema>`,
		},
		{
			name: "identity-constraints",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:hash"
           xmlns:tns="urn:hash"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			changed: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:hash"
           xmlns:tns="urn:hash"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey2">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
		},
		{
			name: "enumerations",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:int">
      <xs:enumeration value="1"/>
      <xs:enumeration value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Code"/>
</xs:schema>`,
			changed: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:int">
      <xs:enumeration value="3"/>
      <xs:enumeration value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Code"/>
</xs:schema>`,
		},
		{
			name: "patterns",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Patterned">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]{2}[0-9]{2}"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Patterned"/>
</xs:schema>`,
			changed: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Patterned">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]{3}[0-9]{2}"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Patterned"/>
</xs:schema>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			digest1 := buildDigest(t, tc.schema)
			digest2 := buildDigest(t, tc.schema)
			if digest1 != digest2 {
				t.Fatalf("digest mismatch")
			}
			digest3 := buildDigest(t, tc.changed)
			if digest1 == digest3 {
				t.Fatalf("expected digest to change when schema changes")
			}
		})
	}
}

func buildDigest(t *testing.T, schemaXML string) [32]byte {
	t.Helper()

	parsed := mustResolveSchema(t, schemaXML)
	rt, err := buildSchemaForTest(parsed, BuildConfig{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return rt.CanonicalDigest()
}
