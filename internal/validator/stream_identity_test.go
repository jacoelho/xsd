package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestStreamIdentityConstraints(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		document string
		wantCode errors.ErrorCode
	}{
		{
			name: "key duplicate with descendant selector",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="group" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="item" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="id" type="xs:string"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath=".//tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><group><item id="a"/><item id="a"/></group></root>`,
			wantCode: errors.ErrIdentityDuplicate,
		},
		{
			name: "key missing field",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
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
			document: `<root xmlns="urn:test"><item/></root>`,
			wantCode: errors.ErrIdentityAbsent,
		},
		{
			name: "keyref not found",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="ref" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="refid" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="itemRef" refer="tns:itemKey">
      <xs:selector xpath="tns:ref"/>
      <xs:field xpath="@refid"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><item id="a"/><ref refid="b"/></root>`,
			wantCode: errors.ErrIdentityKeyRefFailed,
		},
		{
			name: "unique on element text with self field",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="person" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="name" type="xs:string"/>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueName">
      <xs:selector xpath=".//tns:name"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><person><name>A</name></person><person><name>A</name></person></root>`,
			wantCode: errors.ErrIdentityDuplicate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schema))
			if err != nil {
				t.Fatalf("Parse schema: %v", err)
			}
			v := New(mustCompile(t, schema))
			violations, err := v.ValidateStream(strings.NewReader(tt.document))
			if err != nil {
				t.Fatalf("ValidateStream() error = %v", err)
			}
			if !hasViolationCode(violations, tt.wantCode) {
				t.Fatalf("expected code %s, got %v", tt.wantCode, violations)
			}
		})
	}
}

func TestStreamIdentityConstraintWhitespacePreserve(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueItem">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test"><item> A</item><item>A</item></root>`

	parsed, err := parser.Parse(strings.NewReader(schema))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}
	v := New(mustCompile(t, parsed))
	violations, err := v.ValidateStream(strings.NewReader(document))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func hasViolationCode(violations []errors.Validation, code errors.ErrorCode) bool {
	for _, v := range violations {
		if v.Code == string(code) {
			return true
		}
	}
	return false
}
