package validator

import (
	"errors"
	"strings"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestRuntimeAnyTypeAllowsAnyContent(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:foo="urn:foo" foo:attr="1">
  <foo:child/>
  <foo:child2>text</foo:child2>
</tns:root>`

	rt := mustBuildRuntimeSchema(t, schema)
	anyType := rt.Types[rt.Builtin.AnyType]
	if anyType.Kind != runtime.TypeComplex {
		t.Fatalf("anyType kind = %d, want complex", anyType.Kind)
	}
	if anyType.Complex.ID == 0 || int(anyType.Complex.ID) >= len(rt.ComplexTypes) {
		t.Fatalf("anyType complex ref missing")
	}
	ct := rt.ComplexTypes[anyType.Complex.ID]
	if ct.AnyAttr == 0 {
		t.Fatalf("anyType missing anyAttribute wildcard")
	}
	if int(ct.AnyAttr) >= len(rt.Wildcards) {
		t.Fatalf("anyType wildcard out of range")
	}
	if rt.Wildcards[ct.AnyAttr].NS.Kind != runtime.NSAny {
		t.Fatalf("anyType wildcard kind = %d, want NSAny", rt.Wildcards[ct.AnyAttr].NS.Kind)
	}

	sess := NewSession(rt)
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeAllowsXMLNamespaceAttributes(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := `<root xml:lang="en"/>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("expected xml:lang to be accepted: %v", err)
	}
}

func TestRuntimeComplexContentExtensionIncludesBaseParticle(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="b" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test">
  <tns:a>one</tns:a>
  <tns:b>two</tns:b>
</tns:root>`

	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeAttributeWildcardInheritedOnExtension(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="child" type="xs:string"/>
    </xs:sequence>
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:foo="urn:foo" foo:attr="1">
  <tns:child>ok</tns:child>
</tns:root>`

	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeMultipleIDAttributesInvalid(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="T">
    <xs:attribute name="a" type="xs:ID"/>
    <xs:attribute name="b" type="xs:ID"/>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`

	if _, err := buildRuntimeSchema(schema); err == nil {
		t.Fatalf("expected schema validation error for multiple ID attributes")
	}
}

func TestRuntimeDefaultIDREFSInvalid(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="drefs" type="xs:IDREFS" default="abc"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := `<root/>`
	err := validateRuntimeDoc(t, schema, doc)
	if err == nil {
		t.Fatalf("expected IDREFS default error, got nil")
	}
	var violations xsderrors.ValidationList
	if !errors.As(err, &violations) {
		t.Fatalf("expected ValidationList error, got %T", err)
	}
	if !hasViolationCode([]xsderrors.Validation(violations), xsderrors.ErrIDRefNotFound) {
		t.Fatalf("expected code %s, got %v", xsderrors.ErrIDRefNotFound, violations)
	}
}

func TestRuntimeAttributeGroupProhibitedIgnored(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:attribute name="a" type="xs:string"/>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:attributeGroup ref="attG"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:attributeGroup name="attG">
    <xs:attribute name="a" type="xs:string" use="prohibited"/>
  </xs:attributeGroup>
  <xs:element name="doc" type="derived"/>
</xs:schema>`

	doc := `<doc a="ok"/>`
	err := validateRuntimeDoc(t, schema, doc)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestRuntimeEmptyChoiceRejectsEmpty(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="z">
    <xs:complexType>
      <xs:choice/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := `<z/>`
	err := validateRuntimeDoc(t, schema, doc)
	if err == nil {
		t.Fatalf("expected empty choice to reject empty content")
	}
	var violations xsderrors.ValidationList
	if !errors.As(err, &violations) {
		t.Fatalf("expected ValidationList error, got %T", err)
	}
	if !hasViolationCode([]xsderrors.Validation(violations), xsderrors.ErrContentModelInvalid) {
		t.Fatalf("expected code %s, got %v", xsderrors.ErrContentModelInvalid, violations)
	}
}

func TestRuntimeXsiTypeDerivedFromBuiltin(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Derived">
    <xs:restriction base="xs:string">
      <xs:pattern value="[a-z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="tns:Derived">abc</tns:root>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeQNameNamespaceOrderingXsiType(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="T">
    <xs:restriction base="xs:string">
      <xs:pattern value="[a-z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	doc := `<root xmlns="urn:test" xmlns:p="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="p:T">ok</root>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeAnyAttributeSkipSkipsDeclaredAttributes(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:attribute name="a" type="xs:integer"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" tns:a="bra"/>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeSubstitutionGroupAnySimpleTypeEnumeration(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="item" type="xs:anySimpleType"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="item" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:element name="a" type="A" substitutionGroup="item"/>
  <xs:simpleType name="A">
    <xs:restriction base="xs:int">
      <xs:enumeration value="1"/>
      <xs:enumeration value="2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`
	doc := `<root><a>1</a></root>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeSubstitutionGroupXsiTypeDerivedFromMember(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="tns:head"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:element name="head" type="tns:HeadType" abstract="true"/>
  <xs:element name="member" type="tns:MemberType" substitutionGroup="tns:head"/>
  <xs:complexType name="HeadType">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="MemberType">
    <xs:complexContent>
      <xs:extension base="tns:HeadType">
        <xs:sequence>
          <xs:element name="b" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="DerivedFromMember">
    <xs:complexContent>
      <xs:extension base="tns:MemberType">
        <xs:sequence>
          <xs:element name="c" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <tns:member xsi:type="tns:DerivedFromMember">
    <tns:a>ok</tns:a>
    <tns:b>ok</tns:b>
    <tns:c>ok</tns:c>
  </tns:member>
</tns:root>`

	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeSubstitutionGroupXsiTypeDerivedFromHeadInvalid(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="tns:head"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:element name="head" type="tns:HeadType" abstract="true"/>
  <xs:element name="member" type="tns:MemberType" substitutionGroup="tns:head"/>
  <xs:complexType name="HeadType">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="MemberType">
    <xs:complexContent>
      <xs:extension base="tns:HeadType">
        <xs:sequence>
          <xs:element name="b" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="DerivedFromHead">
    <xs:complexContent>
      <xs:extension base="tns:HeadType">
        <xs:sequence>
          <xs:element name="d" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <tns:member xsi:type="tns:DerivedFromHead">
    <tns:a>ok</tns:a>
    <tns:d>ok</tns:d>
  </tns:member>
</tns:root>`

	err := validateRuntimeDoc(t, schema, doc)
	if err == nil {
		t.Fatalf("expected xsi:type derivation error")
	}
	var violations xsderrors.ValidationList
	if !errors.As(err, &violations) {
		t.Fatalf("expected ValidationList error, got %T", err)
	}
	if len(violations) == 0 || violations[0].Code != string(xsderrors.ErrValidateXsiTypeDerivationBlocked) {
		t.Fatalf("expected code %s, got %v", xsderrors.ErrValidateXsiTypeDerivationBlocked, violations)
	}
}

func TestRuntimeErrorOrderDocumentOrder(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:all>
        <xs:element name="a" type="xs:int"/>
        <xs:element name="b" type="xs:int"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	cases := []struct {
		name     string
		doc      string
		wantPath string
	}{
		{
			name:     "a first",
			doc:      `<root><a>bad</a><b>bad</b></root>`,
			wantPath: "/root/a",
		},
		{
			name:     "b first",
			doc:      `<root><b>bad</b><a>bad</a></root>`,
			wantPath: "/root/b",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRuntimeDoc(t, schema, tc.doc)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			var violations xsderrors.ValidationList
			if !errors.As(err, &violations) {
				t.Fatalf("expected ValidationList error, got %T", err)
			}
			if len(violations) == 0 {
				t.Fatalf("expected validation errors")
			}
			if violations[0].Path != tc.wantPath {
				t.Fatalf("first error path = %q, want %q", violations[0].Path, tc.wantPath)
			}
		})
	}
}

func TestRuntimeTotalDigitsLeadingDotDecimal(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="t1" type="t1"/>
  <xs:complexType name="t1">
    <xs:attribute name="att9" use="optional">
      <xs:simpleType>
        <xs:restriction base="xs:decimal">
          <xs:totalDigits value="5"/>
          <xs:fractionDigits value="5"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:attribute>
  </xs:complexType>
</xs:schema>`

	doc := `<t1 att9=".12345"/>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("expected leading dot decimal to pass: %v", err)
	}
}

func TestRuntimeXsiTypeBlockedByBaseTypeBlock(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base" block="extension">
    <xs:sequence>
      <xs:element name="foo" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Base"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="tns:Derived"><tns:foo>ok</tns:foo></tns:root>`
	if err := validateRuntimeDoc(t, schema, doc); err == nil {
		t.Fatalf("expected xsi:type block violation")
	}
}

func TestRuntimeAnyURIAllowsSpaces(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyURI"/>
</xs:schema>`

	bad := `<root>http://exa mple.com</root>`
	if err := validateRuntimeDoc(t, schema, bad); err != nil {
		t.Fatalf("expected anyURI whitespace to pass: %v", err)
	}

	good := `<root>http://example.com/%20</root>`
	if err := validateRuntimeDoc(t, schema, good); err != nil {
		t.Fatalf("expected anyURI percent-encoding to pass: %v", err)
	}
}

func TestRuntimeIdentityFieldUnionSingleField(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tn="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice maxOccurs="unbounded">
        <xs:element name="key">
          <xs:complexType>
            <xs:attribute name="id" type="xs:decimal"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="ref" type="xs:decimal"/>
      </xs:choice>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath=".//tn:key"/>
      <xs:field xpath="@id|@id"/>
    </xs:key>
    <xs:keyref name="r" refer="tn:k">
      <xs:selector xpath=".//tn:ref"/>
      <xs:field xpath="."/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	doc := `<tn:root xmlns:tn="urn:test"><tn:key id="12"/><tn:ref> 12 </tn:ref></tn:root>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeIdentitySelectorDescendantMidPath(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="wrap">
                <xs:complexType>
                  <xs:sequence>
                    <xs:element name="b" maxOccurs="unbounded">
                      <xs:complexType>
                        <xs:attribute name="id" type="xs:ID" use="required"/>
                      </xs:complexType>
                    </xs:element>
                  </xs:sequence>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="a//b"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`

	doc := `<root><a><wrap><b id="a1"/></wrap></a></root>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeKeyrefMissingFieldExcluded(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID" use="required"/>
            <xs:attribute name="ref" type="xs:IDREF"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="r" refer="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@ref"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	doc := `<root><item id="a"/><item id="b" ref="a"/></root>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("expected missing keyref fields to be excluded, got %v", err)
	}
}

func TestRuntimeKeyrefMissingFieldMismatchFails(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID" use="required"/>
            <xs:attribute name="ref" type="xs:IDREF"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="r" refer="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@ref"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	doc := `<root><item id="a"/><item id="b" ref="c"/></root>`
	err := validateRuntimeDoc(t, schema, doc)
	if err == nil {
		t.Fatalf("expected keyref mismatch violation")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrIdentityKeyRefFailed) {
		t.Fatalf("expected ErrIdentityKeyRefFailed, got %+v", list)
	}
}

func TestEnumCanonicalizationDecimalEquivalence(t *testing.T) {
	// Per refactor.md §12.1 item 3: decimal `1.0` == `1.00` in enum
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Dec">
    <xs:restriction base="xs:decimal">
      <xs:enumeration value="1.0"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Dec"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)

	// "1.00" should match the enum "1.0" because they are equal in decimal value space
	doc := `<root>1.00</root>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("expected 1.00 to match enum 1.0: %v", err)
	}

	// "01" should also match "1.0" (same value: 1)
	sess.Reset()
	doc2 := `<root>01</root>`
	if err := sess.Validate(strings.NewReader(doc2)); err != nil {
		t.Fatalf("expected 01 to match enum 1.0: %v", err)
	}

	// "2.0" should NOT match
	sess.Reset()
	doc3 := `<root>2.0</root>`
	if err := sess.Validate(strings.NewReader(doc3)); err == nil {
		t.Fatalf("expected 2.0 to fail enum constraint")
	}
}

func TestEnumCanonicalizationDateTimezone(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="DateEnum">
    <xs:restriction base="xs:date">
      <xs:enumeration value="2023-12-31Z"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="DateEnum"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)
	doc := `<root>2024-01-01+02:00</root>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("expected date enum to match across timezones: %v", err)
	}
}

func TestEnumCanonicalizationDurationNegativeZero(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="DurEnum">
    <xs:restriction base="xs:duration">
      <xs:enumeration value="PT0S"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="DurEnum"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)
	doc := `<root>-PT0S</root>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("expected -PT0S to match PT0S enum: %v", err)
	}
}

func TestEnumCanonicalizationFloatZero(t *testing.T) {
	// Per refactor.md §12.1 item 3: float `-0` == `0` in enum
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="F">
    <xs:restriction base="xs:float">
      <xs:enumeration value="0"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="F"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)

	// "-0" should match "0" because there's only one zero in float value space
	doc := `<root>-0</root>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("expected -0 to match enum 0: %v", err)
	}

	// "0.0" should also match
	sess.Reset()
	doc2 := `<root>0.0</root>`
	if err := sess.Validate(strings.NewReader(doc2)); err != nil {
		t.Fatalf("expected 0.0 to match enum 0: %v", err)
	}
}

func TestEnumCanonicalizationFloatNaN(t *testing.T) {
	// Per refactor.md §12.1 item 3: float NaN equals itself in enum
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="F">
    <xs:restriction base="xs:float">
      <xs:enumeration value="NaN"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="F"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)

	// "NaN" should match itself
	doc := `<root>NaN</root>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("expected NaN to match enum NaN: %v", err)
	}

	// "1.0" should NOT match NaN
	sess.Reset()
	doc2 := `<root>1.0</root>`
	if err := sess.Validate(strings.NewReader(doc2)); err == nil {
		t.Fatalf("expected 1.0 to fail NaN-only enum")
	}
}

func TestEnumCanonicalizationQNamePrefixEquality(t *testing.T) {
	// Per refactor.md §12.1 item 3: QName with different prefixes but same URI/local are equal
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test">
  <xs:simpleType name="Q">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="tns:val"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Q"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)

	// Using a different prefix "p" that resolves to the same namespace should match
	doc := `<root xmlns:p="urn:test">p:val</root>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("expected p:val to match enum tns:val (same namespace): %v", err)
	}

	// Different namespace should NOT match
	sess.Reset()
	doc2 := `<root xmlns:other="urn:other">other:val</root>`
	if err := sess.Validate(strings.NewReader(doc2)); err == nil {
		t.Fatalf("expected other:val to fail enum (different namespace)")
	}
}

func TestRuntimeEnumListWithoutMetrics(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="IntList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="IntListEnum">
    <xs:restriction base="IntList">
      <xs:enumeration value="1 2 3"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="IntListEnum"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)

	doc := `<root>1 2 3</root>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("expected list enum to validate: %v", err)
	}

	sess.Reset()
	doc2 := `<root>1 2 4</root>`
	if err := sess.Validate(strings.NewReader(doc2)); err == nil {
		t.Fatalf("expected list enum violation")
	}
}

func TestRuntimeListEmptyValues(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="List">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="ListMin">
    <xs:restriction base="List">
      <xs:minLength value="1"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="ListEnum">
    <xs:restriction base="List">
      <xs:enumeration value=""/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="List"/>
  <xs:element name="rootMin" type="ListMin"/>
  <xs:element name="rootEnum" type="ListEnum"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)

	if err := sess.Validate(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("expected empty list to validate: %v", err)
	}

	sess.Reset()
	if err := sess.Validate(strings.NewReader("<rootMin/>")); err == nil {
		t.Fatalf("expected minLength violation for empty list")
	}

	sess.Reset()
	if err := sess.Validate(strings.NewReader("<rootEnum/>")); err != nil {
		t.Fatalf("expected empty list enum to validate: %v", err)
	}
}

func TestRuntimeEnumUnionWithoutMetrics(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="UnionType">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
  <xs:simpleType name="UnionEnum">
    <xs:restriction base="UnionType">
      <xs:enumeration value="1"/>
      <xs:enumeration value="true"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="UnionEnum"/>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)

	doc := `<root>true</root>`
	if err := sess.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("expected union enum to validate: %v", err)
	}

	sess.Reset()
	doc2 := `<root>false</root>`
	if err := sess.Validate(strings.NewReader(doc2)); err == nil {
		t.Fatalf("expected union enum violation")
	}
}

func TestXmlnsAppliedBeforeXsiType(t *testing.T) {
	// Per refactor.md §7.1 and §12.1 item 5:
	// Namespace declarations on a start tag MUST be applied BEFORE
	// resolving xsi:type QName on the same tag.
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="value" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:extension base="tns:BaseType">
        <xs:sequence>
          <xs:element name="extra" type="xs:int"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:BaseType"/>
</xs:schema>`

	// Document declares xmlns:p and uses it in xsi:type on the SAME start tag.
	// This tests that xmlns declarations are processed before xsi:type resolution.
	doc := `<tns:root xmlns:tns="urn:test"
                  xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
                  xmlns:p="urn:test"
                  xsi:type="p:DerivedType">
  <tns:value>test</tns:value>
  <tns:extra>42</tns:extra>
</tns:root>`

	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("xmlns should be applied before xsi:type resolution: %v", err)
	}
}

func TestValidationErrorOrdering_DocumentOrder(t *testing.T) {
	// Per refactor.md §9.1 and §12.1 item 9:
	// Validation errors MUST appear in document order.
	// End-element errors come after child errors for that element.
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:int"/>
        <xs:element name="b" type="xs:int"/>
        <xs:element name="c" type="xs:int"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	// Document with errors in a, b, and c (in that order)
	doc := `<root><a>notint1</a><b>notint2</b><c>notint3</c></root>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)
	err := sess.Validate(strings.NewReader(doc))
	if err == nil {
		t.Fatalf("expected validation errors")
	}

	violations, ok := xsderrors.AsValidations(err)
	if !ok {
		t.Fatalf("expected validation errors, got %T", err)
	}

	if len(violations) < 3 {
		t.Fatalf("expected at least 3 validation errors, got %d", len(violations))
	}

	// Verify errors are in document order: a before b before c
	paths := make([]string, len(violations))
	for i, v := range violations {
		paths[i] = v.Path
	}

	// Find indices for each element's error
	aIdx, bIdx, cIdx := -1, -1, -1
	for i, p := range paths {
		if strings.Contains(p, "/a") && aIdx == -1 {
			aIdx = i
		}
		if strings.Contains(p, "/b") && bIdx == -1 {
			bIdx = i
		}
		if strings.Contains(p, "/c") && cIdx == -1 {
			cIdx = i
		}
	}

	if aIdx == -1 || bIdx == -1 || cIdx == -1 {
		t.Fatalf("expected errors for a, b, and c, got paths: %v", paths)
	}

	if aIdx >= bIdx || bIdx >= cIdx {
		t.Fatalf("expected error ordering a < b < c, got a=%d, b=%d, c=%d (paths: %v)", aIdx, bIdx, cIdx, paths)
	}
}

func TestUnionPatternAppliesAfterNormalization(t *testing.T) {
	// Union-level patterns are evaluated on the union's whitespace-normalized value.
	// This ensures pattern checks match the union's fixed whiteSpace=collapse behavior.
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:normalizedString xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="E">
    <xs:restriction base="U">
      <xs:pattern value="[a-z ]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="E"/>
</xs:schema>`

	// Newlines collapse to spaces before pattern validation.
	docWithNewline := `<root>hello
world</root>`
	if err := validateRuntimeDoc(t, schema, docWithNewline); err != nil {
		t.Fatalf("validate: %v", err)
	}

	// Without newline should pass - matches pattern and valid for string member
	docNoNewline := `<root>hello world</root>`
	if err := validateRuntimeDoc(t, schema, docNoNewline); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
