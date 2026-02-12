package set_test

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/set"
)

func BenchmarkPrepareAndBuildRuntime(b *testing.B) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:complexType name="ItemType">
    <xs:sequence>
      <xs:element name="name" type="xs:string"/>
      <xs:element name="quantity" type="xs:int"/>
      <xs:element name="price" type="xs:decimal"/>
    </xs:sequence>
    <xs:attribute name="id" type="xs:ID" use="required"/>
  </xs:complexType>
  <xs:element name="order">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="tns:ItemType" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)},
	}

	b.ReportAllocs()
	for b.Loop() {
		prepared, err := set.Prepare(set.PrepareConfig{
			FS:       fsys,
			Location: "schema.xsd",
		})
		if err != nil {
			b.Fatalf("prepare: %v", err)
		}
		if _, err := prepared.BuildRuntime(set.CompileConfig{}); err != nil {
			b.Fatalf("build runtime: %v", err)
		}
	}
}

func BenchmarkBuildRuntimeFromPrepared(b *testing.B) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:bench"
           xmlns:tns="urn:bench"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}
	prepared, err := set.Prepare(set.PrepareConfig{
		FS:       fsys,
		Location: "schema.xsd",
	})
	if err != nil {
		b.Fatalf("prepare: %v", err)
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := prepared.BuildRuntime(set.CompileConfig{}); err != nil {
			b.Fatalf("build runtime: %v", err)
		}
	}
}
