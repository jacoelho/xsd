package validator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestIdentityUnionMemberOrderDecimalString(t *testing.T) {
	tests := []struct {
		name            string
		memberTypes     string
		expectDuplicate bool
	}{
		{name: "string-first", memberTypes: "xs:string xs:decimal", expectDuplicate: false},
		{name: "decimal-first", memberTypes: "xs:decimal xs:string", expectDuplicate: true},
	}

	for _, tt := range tests {
		schema := fmt.Sprintf(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:union"
           targetNamespace="urn:union"
           elementFormDefault="qualified">
  <xs:simpleType name="U">
    <xs:union memberTypes="%s"/>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="tns:U" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:key>
  </xs:element>
</xs:schema>`, tt.memberTypes)

		document := `<r:root xmlns:r="urn:union">
  <r:item>1</r:item>
  <r:item>1.0</r:item>
</r:root>`

		v := mustNewValidator(t, schema)
		violations, err := v.ValidateStream(strings.NewReader(document))
		if err != nil {
			t.Fatalf("ValidateStream() error = %v", err)
		}
		if tt.expectDuplicate {
			if !hasViolationCode(violations, errors.ErrIdentityDuplicate) {
				t.Fatalf("%s: expected identity duplicate, got %v", tt.name, violations)
			}
		} else if hasViolationCode(violations, errors.ErrIdentityDuplicate) {
			t.Fatalf("%s: expected no identity duplicate, got %v", tt.name, violations)
		}
	}
}

func TestIdentityUnionMemberOrderDateString(t *testing.T) {
	tests := []struct {
		name            string
		memberTypes     string
		expectDuplicate bool
	}{
		{name: "string-first", memberTypes: "xs:string xs:date", expectDuplicate: false},
		{name: "date-first", memberTypes: "xs:date xs:string", expectDuplicate: true},
	}

	for _, tt := range tests {
		schema := fmt.Sprintf(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:union-date"
           targetNamespace="urn:union-date"
           elementFormDefault="qualified">
  <xs:simpleType name="U">
    <xs:union memberTypes="%s"/>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="tns:U" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`, tt.memberTypes)

		document := `<r:root xmlns:r="urn:union-date">
  <r:item>2001-10-26Z</r:item>
  <r:item>2001-10-26+00:00</r:item>
</r:root>`

		v := mustNewValidator(t, schema)
		violations, err := v.ValidateStream(strings.NewReader(document))
		if err != nil {
			t.Fatalf("ValidateStream() error = %v", err)
		}
		if tt.expectDuplicate {
			if !hasViolationCode(violations, errors.ErrIdentityDuplicate) {
				t.Fatalf("%s: expected identity duplicate, got %v", tt.name, violations)
			}
		} else if hasViolationCode(violations, errors.ErrIdentityDuplicate) {
			t.Fatalf("%s: expected no identity duplicate, got %v", tt.name, violations)
		}
	}
}

func TestIdentityUnionWhitespaceNormalization(t *testing.T) {
	tests := []struct {
		name        string
		memberTypes string
		values      []string
	}{
		{
			name:        "string-int whitespace",
			memberTypes: "xs:string xs:int",
			values:      []string{"  42  ", "42"},
		},
		{
			name:        "string-token whitespace",
			memberTypes: "xs:string xs:token",
			values:      []string{"a  b", "a b"},
		},
	}

	for _, tt := range tests {
		schema := fmt.Sprintf(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:union-ws"
           targetNamespace="urn:union-ws"
           elementFormDefault="qualified">
  <xs:simpleType name="U">
    <xs:union memberTypes="%s"/>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="tns:U" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:key>
  </xs:element>
</xs:schema>`, tt.memberTypes)

		document := fmt.Sprintf(`<r:root xmlns:r="urn:union-ws">
  <r:item>%s</r:item>
  <r:item>%s</r:item>
</r:root>`, tt.values[0], tt.values[1])

		v := mustNewValidator(t, schema)
		violations, err := v.ValidateStream(strings.NewReader(document))
		if err != nil {
			t.Fatalf("ValidateStream() error = %v", err)
		}
		if !hasViolationCode(violations, errors.ErrIdentityDuplicate) {
			t.Fatalf("%s: expected identity duplicate, got %v", tt.name, violations)
		}
	}
}
