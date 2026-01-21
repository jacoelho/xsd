package validator

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/loader"
)

func TestChameleonQNameContextRemap(t *testing.T) {
	fsys := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:main"
           xmlns:tns="urn:main"
           elementFormDefault="qualified">
  <xs:include schemaLocation="included.xsd"/>
</xs:schema>`),
		},
		"included.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:simpleType name="QNameType">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="A"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="QNameType" fixed="A"/>
</xs:schema>`),
		},
	}

	l := loader.NewLoader(loader.Config{FS: fsys})
	compiled, err := l.LoadCompiled("main.xsd")
	if err != nil {
		t.Fatalf("LoadCompiled() error = %v", err)
	}

	v := New(compiled)
	violations, err := v.ValidateStream(strings.NewReader(`<root xmlns="urn:main"/>`))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}
