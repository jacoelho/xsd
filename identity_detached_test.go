package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestDetachedIdentityConstraintIsNeverEvaluatedButStaysReferable(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">
<xs:element name="root"><xs:complexType><xs:sequence>
  <xs:element name="gone" minOccurs="0" maxOccurs="0">
    <xs:complexType><xs:sequence><xs:element name="k" type="xs:string"/></xs:sequence></xs:complexType>
    <xs:key name="detached"><xs:selector xpath="p:k"/><xs:field xpath="."/></xs:key>
  </xs:element>
  <xs:element name="box"><xs:complexType><xs:sequence>
    <xs:element name="k" type="xs:string" minOccurs="0" maxOccurs="unbounded"/>
    <xs:element name="id" type="xs:string" minOccurs="0" maxOccurs="unbounded"/>
    <xs:element name="ref" type="xs:string" minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType>
  <xs:keyref name="toDetached" refer="p:detached"><xs:selector xpath="p:ref"/><xs:field xpath="."/></xs:keyref>
  </xs:element>
</xs:sequence></xs:complexType>
<xs:key name="owned"><xs:selector xpath="p:box/p:id"/><xs:field xpath="."/></xs:key>
</xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("detached.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		instance string
		message  string
	}{
		{
			name:     "instance elements the detached selector names are not keyed",
			instance: `<root xmlns="urn:t"><box><k>a</k><k>a</k><id>1</id></box></root>`,
		},
		{
			name:     "owned key in the same schema still rejects duplicates",
			instance: `<root xmlns="urn:t"><box><k>a</k><id>1</id><id>1</id></box></root>`,
			message:  `duplicate value for identity constraint "{urn:t}owned"`,
		},
		{
			name:     "keyref referring to the detached key finds no value",
			instance: `<root xmlns="urn:t"><box><k>a</k><id>1</id><ref>a</ref></box></root>`,
			message:  `keyref value does not match any value of referenced identity constraint "{urn:t}detached"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := engine.Validate(strings.NewReader(test.instance))
			if test.message == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			failures := xsderrors.Flatten(err)
			if len(failures) != 1 {
				t.Fatalf("Validate() error = %v, want one identity diagnostic", err)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](failures[0])
			if !ok || diagnostic.Code() != xsderrors.CodeValidationIdentity || !strings.Contains(diagnostic.Message(), test.message) {
				t.Fatalf("Validate() diagnostic = %v, want identity diagnostic containing %q", err, test.message)
			}
		})
	}
}
