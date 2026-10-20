package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestFixedAttributeUseComparesNormalizedSpellingAndRecovers(t *testing.T) {
	t.Parallel()
	engine, err := xsd.Compile(xsd.Bytes("fixed.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="v" type="xs:integer" fixed="1"/>
  <xs:attribute name="gfloat" type="xs:float" fixed="-0"/>
  <xs:attribute name="gdouble" type="xs:double" fixed="0"/>
  <xs:element name="use"><xs:complexType><xs:attribute name="v" type="xs:integer" fixed="1"/></xs:complexType>
    <xs:unique name="U"><xs:selector xpath="."/><xs:field xpath="@v"/></xs:unique>
  </xs:element>
  <xs:element name="decl"><xs:complexType><xs:attribute ref="v"/></xs:complexType></xs:element>
  <xs:element name="wild"><xs:complexType><xs:anyAttribute processContents="strict"/></xs:complexType></xs:element>
  <xs:element name="string"><xs:complexType><xs:attribute name="v" type="xs:string" fixed="x"/></xs:complexType></xs:element>
  <xs:element name="zero"><xs:complexType><xs:sequence>
    <xs:element name="float" type="xs:float" fixed="-0"/>
    <xs:element name="double" type="xs:double" fixed="0"/>
  </xs:sequence><xs:attribute name="float" type="xs:float" fixed="-0"/>
    <xs:attribute name="double" type="xs:double" fixed="0"/>
  </xs:complexType></xs:element>
  <xs:element name="zero-decl"><xs:complexType><xs:attribute ref="gfloat"/><xs:attribute ref="gdouble"/></xs:complexType></xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		doc     string
		wantErr bool
	}{
		{"canonical after collapse", `<use v=" 1 "/>`, false},
		{"canonical character reference", `<use v="&#x31;"/>`, false},
		{"canonical character-reference whitespace", `<use v="&#x9;1&#xA;"/>`, false},
		{"alias after collapse", `<use v=" +001 "/>`, true},
		{"alias through character reference", `<use v="&#x2B;001"/>`, true},
		{"declaration typed alias", `<decl v=" +001 "/>`, false},
		{"wildcard declaration typed alias", `<wild v=" +001 "/>`, false},
		{"string preserves whitespace", `<string v=" x "/>`, true},
		{"canonical zero uses and typed element aliases", `<zero float="0.0E0" double="0.0E0"><float>0</float><double>-0</double></zero>`, false},
		{"float zero alias", `<zero float="0" double="0.0E0"><float>0</float><double>-0</double></zero>`, true},
		{"float negative zero alias", `<zero float="-0" double="0.0E0"><float>0</float><double>-0</double></zero>`, true},
		{"double zero alias", `<zero float="0.0E0" double="0"><float>0</float><double>-0</double></zero>`, true},
		{"double negative zero alias", `<zero float="0.0E0" double="-0"><float>0</float><double>-0</double></zero>`, true},
		{"isolated nonzero float", `<zero float="1.0E0" double="0.0E0"><float>0</float><double>-0</double></zero>`, true},
		{"declaration zero aliases", `<zero-decl gfloat="0" gdouble="-0"/>`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			err = session.Validate(strings.NewReader(tt.doc))
			if tt.wantErr {
				expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationAttribute)
			} else if err != nil {
				t.Fatalf("Validate(%s): %v", tt.doc, err)
			}
			if err := session.Validate(strings.NewReader(`<use v="1"/>`)); err != nil {
				t.Fatalf("session reuse after %s: %v", tt.name, err)
			}
		})
	}
}

// QName canonical prefix spelling is unresolved in the retained XSD 1.0
// sources. These checks preserve the existing expanded-name product policy;
// they do not settle that separate canonical-projection question.
func TestFixedAttributeUsePreservesQualifiedValuePolicy(t *testing.T) {
	t.Parallel()
	engine, err := xsd.Compile(xsd.Bytes("qualified-fixed.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:a" xmlns:b="urn:b">
  <xs:import namespace="urn:a"/>
  <xs:import namespace="urn:b"/>
  <xs:simpleType name="N"><xs:restriction base="xs:NOTATION"><xs:enumeration value="p:n"/><xs:enumeration value="p:m"/><xs:enumeration value="b:n"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="LQ"><xs:list itemType="xs:QName"/></xs:simpleType>
  <xs:simpleType name="LN"><xs:list itemType="N"/></xs:simpleType>
  <xs:simpleType name="IQ"><xs:union memberTypes="xs:integer xs:QName"/></xs:simpleType>
  <xs:simpleType name="NestedIQ"><xs:union memberTypes="IQ"/></xs:simpleType>
  <xs:simpleType name="LIQ"><xs:list itemType="NestedIQ"/></xs:simpleType>
  <xs:simpleType name="QS"><xs:union memberTypes="xs:QName xs:string"/></xs:simpleType>
  <xs:simpleType name="SQ"><xs:union memberTypes="xs:string xs:QName"/></xs:simpleType>
  <xs:element name="q"><xs:complexType><xs:attribute name="v" type="xs:QName" fixed="p:n"/></xs:complexType></xs:element>
  <xs:element name="n"><xs:complexType><xs:attribute name="v" type="N" fixed="p:n"/></xs:complexType></xs:element>
  <xs:element name="lq"><xs:complexType><xs:attribute name="v" type="LQ" fixed="p:n p:m"/></xs:complexType></xs:element>
  <xs:element name="ln"><xs:complexType><xs:attribute name="v" type="LN" fixed="p:n p:m"/></xs:complexType></xs:element>
  <xs:element name="number"><xs:complexType><xs:attribute name="v" type="IQ" fixed="1"/></xs:complexType></xs:element>
  <xs:element name="union"><xs:complexType><xs:attribute name="v" type="NestedIQ" fixed="p:n"/></xs:complexType></xs:element>
  <xs:element name="mixed-list"><xs:complexType><xs:attribute name="v" type="LIQ" fixed="p:n 1"/></xs:complexType></xs:element>
  <xs:element name="fallback"><xs:complexType><xs:attribute name="v" type="QS" fixed="p:n"/></xs:complexType></xs:element>
  <xs:element name="string"><xs:complexType><xs:attribute name="v" type="SQ" fixed="p:n"/></xs:complexType></xs:element>
</xs:schema>`)),
		xsd.Bytes("notation-a.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:notation name="n" public="n"/><xs:notation name="m" public="m"/></xs:schema>`)),
		xsd.Bytes("notation-b.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"><xs:notation name="n" public="n"/></xs:schema>`)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		doc     string
		wantErr bool
	}{
		{"QName same spelling", `<q xmlns:p="urn:a" v="p:n"/>`, false},
		{"QName equivalent prefix", `<q xmlns:q="urn:a" v="q:n"/>`, false},
		{"QName rebound namespace", `<q xmlns:p="urn:b" v="p:n"/>`, true},
		{"QName different local", `<q xmlns:p="urn:a" v="p:m"/>`, true},
		{"QName unbound prefix admission", `<q v="p:n"/>`, true},
		{"NOTATION same spelling", `<n xmlns:p="urn:a" v="p:n"/>`, false},
		{"NOTATION equivalent prefix", `<n xmlns:q="urn:a" v="q:n"/>`, false},
		{"NOTATION other enumerated namespace", `<n xmlns:p="urn:b" v="p:n"/>`, true},
		{"NOTATION other enumerated local", `<n xmlns:p="urn:a" v="p:m"/>`, true},
		{"QName list aliases", `<lq xmlns:q="urn:a" v=" q:n  q:m "/>`, false},
		{"QName list order", `<lq xmlns:q="urn:a" v="q:m q:n"/>`, true},
		{"NOTATION list aliases", `<ln xmlns:q="urn:a" v="q:n q:m"/>`, false},
		{"NOTATION list order", `<ln xmlns:q="urn:a" v="q:m q:n"/>`, true},
		{"numeric member canonical", `<number v=" 1 "/>`, false},
		{"numeric member alias", `<number v="+001"/>`, true},
		{"selected QName member", `<union xmlns:q="urn:a" v="q:n"/>`, false},
		{"mixed list compatibility", `<mixed-list xmlns:q="urn:a" v="q:n 01"/>`, false},
		{"mixed list order", `<mixed-list xmlns:q="urn:a" v="1 q:n"/>`, true},
		{"rejected QName then string", `<fallback v="p:n"/>`, true},
		{"selected string ignores prefix binding", `<string xmlns:p="urn:b" v="p:n"/>`, false},
		{"selected string distinguishes spelling", `<string xmlns:q="urn:a" v="q:n"/>`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			err = session.Validate(strings.NewReader(tt.doc))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate(%s) = %v; want error = %t", tt.doc, err, tt.wantErr)
			}
			if err := session.Validate(strings.NewReader(`<q xmlns:p="urn:a" v="p:n"/>`)); err != nil {
				t.Fatalf("session recovery: %v", err)
			}
		})
	}
}
