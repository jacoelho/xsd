package validate

import (
	"strings"
	"testing"
)

func TestValidationDistinguishesNameMembershipNamespacesAndWildcards(t *testing.T) {
	runtime := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:a="urn:a" targetNamespace="urn:a" elementFormDefault="qualified">
  <xs:complexType name="UnusedType"/>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="item"/>
    <xs:any namespace="urn:b" processContents="skip" minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	pool := NewSessionPool(runtime)
	for _, test := range []struct {
		input string
		valid bool
	}{
		{`<a:root xmlns:a="urn:a" xmlns:b="urn:b"><a:item/><b:item/><b:unknown/></a:root>`, true},
		{`<a:root xmlns:a="urn:a" xmlns:b="urn:b"><b:item/></a:root>`, false},
		{`<a:root xmlns:a="urn:a" xmlns:c="urn:c"><a:item/><c:item/></a:root>`, false},
		{`<a:UnusedType xmlns:a="urn:a"/>`, false},
		{`<root/>`, false},
	} {
		err := pool.Validate(strings.NewReader(test.input), Options{})
		if (err == nil) != test.valid {
			t.Fatalf("%q: %v", test.input, err)
		}
	}
}
