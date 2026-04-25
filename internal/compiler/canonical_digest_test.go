package compiler

import (
	"encoding/hex"
	"os"
	"testing"
)

func TestDeterministicBuild(t *testing.T) {
	cases := []struct {
		name    string
		schema  string
		changed string
		want    string
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
			want: "cdb13320a1ad797c39f136d419641d8cd3787e53cba2cdcdc110b7ba43bbc691",
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
			want: "83874af1e830cbdfccc732b925f4fba565b57075bebda99c0e44d88f113d0bd1",
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
			want: "c378eb13e270368c3d5b460f5213dc08598c3a26c3e81bd88331c4cd10ed5c3c",
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
			want: "652080a4a8a0ed2ae35e566d162bfb90fac6fdbb414dd4e3b19b052b42400269",
		},
		{
			name: "list-and-union",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Codes">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="Value">
    <xs:union memberTypes="xs:string Codes"/>
  </xs:simpleType>
  <xs:element name="root" type="Value"/>
</xs:schema>`,
			changed: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Codes">
    <xs:list itemType="xs:integer"/>
  </xs:simpleType>
  <xs:simpleType name="Value">
    <xs:union memberTypes="xs:string Codes"/>
  </xs:simpleType>
  <xs:element name="root" type="Value"/>
</xs:schema>`,
			want: "fbc056cfab314da1bdec33f328a6dfd60cf8d6edb05af52f8fc1af884243d24c",
		},
		{
			name: "complex-extension",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:hash"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="code" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="name" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`,
			changed: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:hash"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="code" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="label" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`,
			want: "2371817f62b716ad2ca33c07ca3fb51f6f4031c456f1bb919b19a61e8750fe67",
		},
		{
			name: "attribute-defaults",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Item">
    <xs:attribute name="code" type="xs:string" default="A"/>
    <xs:attribute name="kind" type="xs:string" fixed="B"/>
  </xs:complexType>
  <xs:element name="root" type="Item"/>
</xs:schema>`,
			changed: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Item">
    <xs:attribute name="code" type="xs:string" default="C"/>
    <xs:attribute name="kind" type="xs:string" fixed="B"/>
  </xs:complexType>
  <xs:element name="root" type="Item"/>
</xs:schema>`,
			want: "f2a6f32266ddf4aa86034d5cdf59570a54d673af6aa52c5b4fc1f776c812deed",
		},
		{
			name: "wildcard",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:hash"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:complexType name="Open">
    <xs:sequence>
      <xs:any namespace="##other" processContents="lax" minOccurs="0" maxOccurs="unbounded"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="tns:Open"/>
</xs:schema>`,
			changed: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:hash"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:complexType name="Open">
    <xs:sequence>
      <xs:any namespace="##any" processContents="lax" minOccurs="0" maxOccurs="unbounded"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="tns:Open"/>
</xs:schema>`,
			want: "6eb81ccca0275f1261f47702c0bb5504456cb0fb20597b3f5fbe93aee217571a",
		},
		{
			name: "substitution-groups",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:hash"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:element name="head" type="xs:string" abstract="true"/>
  <xs:element name="member" type="xs:string" substitutionGroup="tns:head"/>
  <xs:complexType name="Root">
    <xs:sequence>
      <xs:element ref="tns:head"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="tns:Root"/>
</xs:schema>`,
			changed: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:hash"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:element name="head" type="xs:string" abstract="true"/>
  <xs:element name="member2" type="xs:string" substitutionGroup="tns:head"/>
  <xs:complexType name="Root">
    <xs:sequence>
      <xs:element ref="tns:head"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="tns:Root"/>
</xs:schema>`,
			want: "686b185582314781401bf87ee3b3b6517fccc51812ac72cea1ddf6ed50171f28",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			digest1 := buildDigest(t, tc.schema)
			if got := hex.EncodeToString(digest1[:]); got != tc.want {
				t.Fatalf("digest = %s, want %s", got, tc.want)
			}
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

func TestCanonicalDigestGoldenGMLRoot(t *testing.T) {
	base := "../../testdata/gml/xsd"
	if _, err := os.Stat(base); err != nil {
		t.Skipf("GML schema fixture unavailable: %v", err)
	}
	prepared, err := PrepareRoots(LoadConfig{
		Roots: []Root{{
			FS:       os.DirFS(base),
			Location: "LandCoverVector.xsd",
		}},
		AllowMissingImportLocations: true,
	})
	if err != nil {
		t.Fatalf("PrepareRoots() error = %v", err)
	}
	rt, err := prepared.Build(BuildConfig{MaxOccursLimit: 4096})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	digest := rt.CanonicalDigest()
	got := hex.EncodeToString(digest[:])
	const want = "8c4b3711aa3651484a3eac7cede5141c396eb11969aa32d3fb3e899599ac416d"
	if got != want {
		t.Fatalf("digest = %s, want %s", got, want)
	}
}
