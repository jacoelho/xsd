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
			name: "key field ignores qualified attribute defaults",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:attribute name="id" type="xs:string" default="DEF"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="tns:id"/>
    </xs:complexType>
    <xs:key name="rootKey">
      <xs:selector xpath="."/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"/>`,
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
		{
			name: "unique uses element default",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" default="A" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueItem">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><item/><item/></root>`,
			wantCode: errors.ErrIdentityDuplicate,
		},
		{
			name: "unique uses element fixed",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" fixed="A" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueItem">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><item/><item/></root>`,
			wantCode: errors.ErrIdentityDuplicate,
		},
		{
			name: "unique compares boolean value space",
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
            <xs:attribute name="flag" type="xs:boolean"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueFlag">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@flag"/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><item flag="true"/><item flag="1"/></root>`,
			wantCode: errors.ErrIdentityDuplicate,
		},
		{
			name: "unique compares dateTime value space",
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
            <xs:attribute name="ts" type="xs:dateTime"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueTimestamp">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@ts"/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><item ts="2001-10-26T21:32:52+02:00"/><item ts="2001-10-26T19:32:52Z"/></root>`,
			wantCode: errors.ErrIdentityDuplicate,
		},
		{
			name: "unique compares list value space",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="stringList">
    <xs:list itemType="xs:string"/>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="values" type="tns:stringList"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueList">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@values"/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><item values="a b"/><item values="a   b"/></root>`,
			wantCode: errors.ErrIdentityDuplicate,
		},
		{
			name: "unique compares base64Binary value space",
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
            <xs:attribute name="bin" type="xs:base64Binary"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueBase64">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@bin"/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><item bin="AQID"/><item bin="A Q I D"/></root>`,
			wantCode: errors.ErrIdentityDuplicate,
		},
		{
			name: "unique compares hexBinary value space",
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
            <xs:attribute name="hex" type="xs:hexBinary"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueHex">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@hex"/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><item hex="0A0B"/><item hex="0a0b"/></root>`,
			wantCode: errors.ErrIdentityDuplicate,
		},
		{
			name: "key rejects mixed content field",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType mixed="true">
            <xs:sequence>
              <xs:element name="child" type="xs:string" minOccurs="0"/>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			document: `<root xmlns="urn:test"><item>text<child>v</child></item></root>`,
			wantCode: errors.ErrIdentityAbsent,
		},
		{
			name: "key ignores qualified attribute for unprefixed field",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string" use="required"/>
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
			document: `<root xmlns="urn:test" xmlns:tns="urn:test"><item tns:id="a"/></root>`,
			wantCode: errors.ErrIdentityAbsent,
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

func TestStreamIdentityConstraintKeyrefValueSpace(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="flag" type="xs:boolean"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="ref" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="flag" type="xs:boolean"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@flag"/>
    </xs:key>
    <xs:keyref name="itemRef" refer="tns:itemKey">
      <xs:selector xpath="tns:ref"/>
      <xs:field xpath="@flag"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test"><item flag="true"/><ref flag="1"/></root>`

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

func TestStreamIdentityConstraintZeroValueSpace(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:double" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueItem">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test"><item>-0</item><item>0</item></root>`

	parsed, err := parser.Parse(strings.NewReader(schema))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}
	v := New(mustCompile(t, parsed))
	violations, err := v.ValidateStream(strings.NewReader(document))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrIdentityDuplicate) {
		t.Fatalf("expected code %s, got %v", errors.ErrIdentityDuplicate, violations)
	}
}

func TestStreamIdentityIgnoresXMLNSAttributes(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType/>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueAnyAttr">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@*"/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test"><item xmlns:p="urn:test"/><item xmlns:q="urn:test"/></root>`

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

func TestStreamIdentityInvalidNumericValue(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:integer" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueItem">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test"><item>nope</item><item>nope</item></root>`

	parsed, err := parser.Parse(strings.NewReader(schema))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}
	v := New(mustCompile(t, parsed))
	violations, err := v.ValidateStream(strings.NewReader(document))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected code %s, got %v", errors.ErrDatatypeInvalid, violations)
	}
	if hasViolationCode(violations, errors.ErrIdentityDuplicate) {
		t.Fatalf("unexpected code %s, got %v", errors.ErrIdentityDuplicate, violations)
	}
}

func TestStreamIdentityKeyInvalidValue(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:integer" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:key>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test"><item>nope</item><item>nope</item></root>`

	parsed, err := parser.Parse(strings.NewReader(schema))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}
	v := New(mustCompile(t, parsed))
	violations, err := v.ValidateStream(strings.NewReader(document))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected code %s, got %v", errors.ErrDatatypeInvalid, violations)
	}
	if !hasViolationCode(violations, errors.ErrIdentityAbsent) {
		t.Fatalf("expected code %s, got %v", errors.ErrIdentityAbsent, violations)
	}
}

func TestStreamIdentityUniqueIgnoresNilledElements(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" nillable="true" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="uniqueItem">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <item xsi:nil="true"/>
  <item xsi:nil="true"/>
</root>`

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

func TestStreamIdentityKeyrefIgnoresNilledField(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string" use="required"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="ref" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="target" type="xs:string" nillable="true"/>
            </xs:sequence>
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
      <xs:field xpath="tns:target"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <item id="a"/>
  <ref><target xsi:nil="true"/></ref>
</root>`

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

func TestIdentityTimezoneDistinctValues(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="dt" type="xs:dateTime" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="itemTime">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@dt"/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test">
  <item dt="2000-01-01T00:00:00"/>
  <item dt="2000-01-01T00:00:00Z"/>
</root>`

	parsed, err := parser.Parse(strings.NewReader(schema))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}
	v := New(mustCompile(t, parsed))
	violations, err := v.ValidateStream(strings.NewReader(document))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if hasViolationCode(violations, errors.ErrIdentityDuplicate) {
		t.Fatalf("unexpected code %s, got %v", errors.ErrIdentityDuplicate, violations)
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
