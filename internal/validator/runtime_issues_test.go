package validator

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func TestValidateRootSeenOnError(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	err = sess.Validate(strings.NewReader("<root/>"))
	if err == nil {
		t.Fatalf("expected validation error")
	}
	list := mustValidationList(t, err)
	if hasValidationCode(list, xsderrors.ErrNoRoot) {
		t.Fatalf("unexpected ErrNoRoot when root element was present")
	}
	if !hasValidationCode(list, xsderrors.ErrValidateRootNotDeclared) {
		t.Fatalf("expected ErrValidateRootNotDeclared, got %+v", list)
	}
}

func TestValidateReaderSetupErrorWrapped(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)

	sentinel := errors.New("reader setup failed")
	orig := newXMLReader
	newXMLReader = func(_ io.Reader, _ ...xmlstream.Option) (*xmlstream.Reader, error) {
		return nil, sentinel
	}
	t.Cleanup(func() {
		newXMLReader = orig
	})

	err = sess.Validate(strings.NewReader("<root/>"))
	if err == nil {
		t.Fatalf("expected validation error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrXMLParse) {
		t.Fatalf("expected ErrXMLParse, got %+v", list)
	}
	if !strings.Contains(list[0].Message, sentinel.Error()) {
		t.Fatalf("expected error message to contain %q, got %q", sentinel.Error(), list[0].Message)
	}
}

func TestValidateNilReaderWrapped(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)

	err = sess.Validate(nil)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrXMLParse) {
		t.Fatalf("expected ErrXMLParse, got %+v", list)
	}
	if !strings.Contains(list[0].Message, "nil reader") {
		t.Fatalf("expected error message to contain %q, got %q", "nil reader", list[0].Message)
	}
}

func TestAttZ015ProhibitedAttributeGroup(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema">

<xsd:complexType name="base">
	<xsd:attribute name="a" />
</xsd:complexType>

<xsd:complexType name="derived">
  <xsd:complexContent>

    <xsd:restriction base="base">
      <xsd:attributeGroup ref="attG"/>
    </xsd:restriction>

  </xsd:complexContent>
</xsd:complexType>

<xsd:attributeGroup name="attG">
 <xsd:attribute name="a" use="prohibited"/> 
</xsd:attributeGroup>

<xsd:element name="doc" type="derived" />

</xsd:schema>`

	docXML := `<doc a="a"/>`
	if err := validateRuntimeDoc(t, schemaXML, docXML); err != nil {
		t.Fatalf("expected validation to succeed: %v", err)
	}
}

func TestRootAnyAllowsUndeclaredRoot(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	schema.RootPolicy = runtime.RootAny
	sess := NewSession(schema)
	err = sess.Validate(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("expected validation to succeed: %v", err)
	}
}

func TestUnionWhitespaceCollapseRuntime(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="E">
    <xs:restriction base="tns:U">
      <xs:pattern value="\S+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:E"/>
</xs:schema>`

	docXML := `<root xmlns="urn:test">  a  </root>`
	if err := validateRuntimeDoc(t, schemaXML, docXML); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestInvalidIDDoesNotSatisfyIDREF(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="ID10">
    <xs:restriction base="xs:ID">
      <xs:pattern value="\d{10}"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="id" type="tns:ID10"/>
      <xs:attribute name="ref" type="xs:IDREF"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" id="abc" ref="abc"/>`
	err := validateRuntimeDoc(t, schemaXML, docXML)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrIDRefNotFound) {
		t.Fatalf("expected ErrIDRefNotFound, got %+v", list)
	}
}

func TestProhibitedAttributeFixedRejectedAtRuntime(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string" use="prohibited" fixed="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" a="x"/>`
	err := validateRuntimeDoc(t, schemaXML, docXML)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrAttributeProhibited) {
		t.Fatalf("expected ErrAttributeProhibited, got %+v", list)
	}
}

func TestNilledElementChildReportsOnce(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" nillable="true">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"><child>ok</child></root>`
	err := validateRuntimeDoc(t, schemaXML, docXML)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	list := mustValidationList(t, err)
	if got := countValidationCode(list, xsderrors.ErrValidateNilledNotEmpty); got != 1 {
		t.Fatalf("ErrValidateNilledNotEmpty count = %d, want 1", got)
	}
}

func TestSimpleContentChildReportsOnce(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	docXML := `<root xmlns="urn:test"><child/></root>`
	err := validateRuntimeDoc(t, schemaXML, docXML)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	list := mustValidationList(t, err)
	if got := countValidationCode(list, xsderrors.ErrTextInElementOnly); got != 1 {
		t.Fatalf("ErrTextInElementOnly count = %d, want 1", got)
	}
}

func TestValidateMissingRootParseError(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	err = sess.Validate(strings.NewReader(""))
	if err == nil {
		t.Fatalf("expected parse error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrXMLParse) {
		t.Fatalf("expected ErrXMLParse, got %+v", list)
	}
}

func TestValidateCharDataOutsideRoot(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	cases := []struct {
		name    string
		doc     string
		wantErr bool
	}{
		{name: "text before root", doc: "x<root xmlns=\"urn:test\"/>", wantErr: true},
		{name: "text after root", doc: "<root xmlns=\"urn:test\"/>x", wantErr: true},
		{name: "whitespace before root", doc: " \n\t<root xmlns=\"urn:test\"/>", wantErr: false},
		{name: "whitespace after root", doc: "<root xmlns=\"urn:test\"/> \n\t", wantErr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRuntimeDoc(t, schemaXML, tc.doc)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				list := mustValidationList(t, err)
				if !hasValidationCode(list, xsderrors.ErrXMLParse) {
					t.Fatalf("expected ErrXMLParse, got %+v", list)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLookupNamespaceCacheDoesNotGrowBuffers(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	decls := make([]xmlstream.NamespaceDecl, 0, 40)
	for i := range 40 {
		decls = append(decls, xmlstream.NamespaceDecl{
			Prefix: fmt.Sprintf("p%d", i),
			URI:    fmt.Sprintf("urn:%d", i),
		})
	}
	sess.pushNamespaceScope(decls)
	beforeLocal := len(sess.nameLocal)
	beforeNS := len(sess.nameNS)

	ns, ok := sess.lookupNamespace([]byte("p10"))
	if !ok || string(ns) != "urn:10" {
		t.Fatalf("lookupNamespace result = %q, %v", ns, ok)
	}
	if len(sess.nameLocal) != beforeLocal || len(sess.nameNS) != beforeNS {
		t.Fatalf("name buffers grew after first lookup")
	}
	cacheLen := len(sess.prefixCache)

	ns, ok = sess.lookupNamespace([]byte("p10"))
	if !ok || string(ns) != "urn:10" {
		t.Fatalf("lookupNamespace cached result = %q, %v", ns, ok)
	}
	if len(sess.nameLocal) != beforeLocal || len(sess.nameNS) != beforeNS {
		t.Fatalf("name buffers grew after cached lookup")
	}
	if len(sess.prefixCache) != cacheLen {
		t.Fatalf("prefix cache grew after cached lookup")
	}
}

func TestPathStringFallbackUsesFrameName(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	sess.elemStack = []elemFrame{{
		name:  NameID(maxNameMapSize + 1),
		local: []byte("root"),
		ns:    []byte("urn:test"),
	}}
	if got := sess.pathString(); got != "/{urn:test}root" {
		t.Fatalf("pathString = %q, want %q", got, "/{urn:test}root")
	}
}

func TestBinaryLengthFacets(t *testing.T) {
	binarySchema := func(base, facet string, value int) string {
		return fmt.Sprintf(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Bin">
    <xs:restriction base="xs:%s">
      <xs:%s value="%d"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:Bin"/>
</xs:schema>`, base, facet, value)
	}

	cases := []struct {
		name      string
		schemaXML string
		docXML    string
		wantErr   bool
	}{
		{
			name:      "base64Binary length accepts one byte",
			schemaXML: binarySchema("base64Binary", "length", 1),
			docXML:    `<root xmlns="urn:test">AQ==</root>`,
		},
		{
			name:      "base64Binary length rejects two bytes",
			schemaXML: binarySchema("base64Binary", "length", 1),
			docXML:    `<root xmlns="urn:test">AQI=</root>`,
			wantErr:   true,
		},
		{
			name:      "base64Binary minLength rejects one byte",
			schemaXML: binarySchema("base64Binary", "minLength", 2),
			docXML:    `<root xmlns="urn:test">AQ==</root>`,
			wantErr:   true,
		},
		{
			name:      "base64Binary minLength accepts two bytes",
			schemaXML: binarySchema("base64Binary", "minLength", 2),
			docXML:    `<root xmlns="urn:test">AQI=</root>`,
		},
		{
			name:      "base64Binary maxLength accepts one byte",
			schemaXML: binarySchema("base64Binary", "maxLength", 1),
			docXML:    `<root xmlns="urn:test">AQ==</root>`,
		},
		{
			name:      "base64Binary maxLength rejects two bytes",
			schemaXML: binarySchema("base64Binary", "maxLength", 1),
			docXML:    `<root xmlns="urn:test">AQI=</root>`,
			wantErr:   true,
		},
		{
			name:      "hexBinary length accepts one byte",
			schemaXML: binarySchema("hexBinary", "length", 1),
			docXML:    `<root xmlns="urn:test">0F</root>`,
		},
		{
			name:      "hexBinary length rejects two bytes",
			schemaXML: binarySchema("hexBinary", "length", 1),
			docXML:    `<root xmlns="urn:test">0F10</root>`,
			wantErr:   true,
		},
		{
			name:      "hexBinary minLength rejects one byte",
			schemaXML: binarySchema("hexBinary", "minLength", 2),
			docXML:    `<root xmlns="urn:test">0F</root>`,
			wantErr:   true,
		},
		{
			name:      "hexBinary minLength accepts two bytes",
			schemaXML: binarySchema("hexBinary", "minLength", 2),
			docXML:    `<root xmlns="urn:test">0F10</root>`,
		},
		{
			name:      "hexBinary maxLength accepts one byte",
			schemaXML: binarySchema("hexBinary", "maxLength", 1),
			docXML:    `<root xmlns="urn:test">0F</root>`,
		},
		{
			name:      "hexBinary maxLength rejects two bytes",
			schemaXML: binarySchema("hexBinary", "maxLength", 1),
			docXML:    `<root xmlns="urn:test">0F10</root>`,
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRuntimeDoc(t, tc.schemaXML, tc.docXML)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestAllGroupSubstitutionMembers(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" substitutionGroup="tns:head" type="xs:string"/>
  <xs:complexType name="RootType">
    <xs:all>
      <xs:element ref="tns:head"/>
    </xs:all>
  </xs:complexType>
  <xs:element name="root" type="tns:RootType"/>
</xs:schema>`

	docXML := `<root xmlns="urn:test"><member>ok</member></root>`
	if err := validateRuntimeDoc(t, schemaXML, docXML); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func mustValidationList(t *testing.T, err error) xsderrors.ValidationList {
	t.Helper()
	var list xsderrors.ValidationList
	ok := errors.As(err, &list)
	if !ok {
		t.Fatalf("expected ValidationList, got %T", err)
	}
	return list
}

func hasValidationCode(list xsderrors.ValidationList, code xsderrors.ErrorCode) bool {
	for _, v := range list {
		if v.Code == string(code) {
			return true
		}
	}
	return false
}

func countValidationCode(list xsderrors.ValidationList, code xsderrors.ErrorCode) int {
	count := 0
	for _, v := range list {
		if v.Code == string(code) {
			count++
		}
	}
	return count
}
