package xsd

import (
	"strings"
	"testing"
)

func TestIdentityConstraintSchemaShapeIsValidated(t *testing.T) {
	tests := []struct {
		body string
		code ErrorCode
	}{
		{`<xs:unique name="u"><xs:selector xpath="a"/><xs:field xpath="@id"/></xs:unique>`, ErrSchemaContentModel},
		{`<xs:element name="r"><xs:complexType><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType><xs:unique name="u" refer="k"><xs:selector xpath="a"/><xs:field xpath="@id"/></xs:unique></xs:element>`, ErrSchemaInvalidAttribute},
		{`<xs:element name="r"><xs:unique name="u"><xs:selector xpath="a"/><xs:annotation/><xs:field xpath="@id"/></xs:unique></xs:element>`, ErrSchemaContentModel},
		{`<xs:element name="r"><xs:unique name="u"><xs:field xpath="@id"/><xs:selector xpath="a"/></xs:unique></xs:element>`, ErrSchemaContentModel},
		{`<xs:element name="r"><xs:unique name="u"><xs:selector xpath="a"/><xs:selector xpath="b"/><xs:field xpath="@id"/></xs:unique></xs:element>`, ErrSchemaContentModel},
		{`<xs:group name="g"><xs:sequence><xs:unique name="u"><xs:selector xpath="a"/><xs:field xpath="@id"/></xs:unique></xs:sequence></xs:group>`, ErrSchemaContentModel},
		{`<xs:field xpath="@id"/>`, ErrSchemaContentModel},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="a"/><xs:field xpath=""/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="a"/><xs:field xpath="| b"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="| b"/><xs:field xpath="@id"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="a"/><xs:field xpath=".///@id"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="a"/><xs:field xpath=".//"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="/a"/><xs:field xpath="@id"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="a[1]"/><xs:field xpath="@id"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="document('')"/><xs:field xpath="@id"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="child::"/><xs:field xpath="@id"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="a"/><xs:field xpath="attribute::"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="@*"/><xs:field xpath="@id"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r" xmlns:t="urn:t"><xs:key name="k"><xs:selector xpath="*"/><xs:field xpath="t: *"/></xs:key></xs:element>`, ErrSchemaReference},
		{`<xs:element name="r"><xs:key name="k"><xs:selector xpath="*"/><xs:field xpath="@"/></xs:key></xs:element>`, ErrSchemaIdentity},
		{`<xs:element name="r"><xs:keyref name="kr" refer="k"><xs:selector xpath="a"/><xs:field xpath="@a"/><xs:field xpath="@b"/></xs:keyref><xs:key name="k"><xs:selector xpath="b"/><xs:field xpath="@a"/></xs:key></xs:element>`, ErrSchemaIdentity},
	}
	for _, test := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.body+`</xs:schema>`)))
		expectCode(t, err, test.code)
	}
}

func TestIdentityConstraintsUniqueKeyAndKeyref(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="code" type="xs:string" minOccurs="0"/>
            </xs:sequence>
            <xs:attribute name="id" type="xs:string"/>
            <xs:attribute name="ref" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u"><xs:selector xpath="item"/><xs:field xpath="code"/></xs:unique>
    <xs:key name="k"><xs:selector xpath="item"/><xs:field xpath="@id"/></xs:key>
    <xs:keyref name="r" refer="k"><xs:selector xpath="item"/><xs:field xpath="@ref"/></xs:keyref>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root><item id="a"><code>x</code></item><item id="b" ref="a"><code>y</code></item><item id="c"/></root>`)
	mustNotValidate(t, engine, `<root><item id="a"><code>x</code></item><item id="b"><code>x</code></item></root>`, ErrValidationIdentity)
	mustNotValidate(t, engine, `<root><item><code>x</code></item></root>`, ErrValidationIdentity)
	mustNotValidate(t, engine, `<root><item id="a"/><item id="b" ref="missing"/></root>`, ErrValidationIdentity)
}

func TestIdentityConstraintDecimalFieldsUseValueSpace(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:element name="root">
	    <xs:complexType>
	      <xs:sequence>
	        <xs:element name="row" maxOccurs="unbounded">
	          <xs:complexType>
	            <xs:attribute name="amount" type="xs:decimal"/>
	          </xs:complexType>
	        </xs:element>
	      </xs:sequence>
	    </xs:complexType>
	    <xs:unique name="u"><xs:selector xpath="row"/><xs:field xpath="@amount"/></xs:unique>
	  </xs:element>
	</xs:schema>`)
	mustValidate(t, engine, `<root><row amount="1"/><row amount="2.0"/></root>`)
	mustNotValidate(t, engine, `<root><row amount="1"/><row amount="1.0"/></root>`, ErrValidationIdentity)
}

func TestIdentityConstraintFloatDoubleZeroUseValueSpace(t *testing.T) {
	for _, typ := range []string{"xs:float", "xs:double"} {
		t.Run(typ, func(t *testing.T) {
			engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="amount" type="`+typ+`"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u"><xs:selector xpath="row"/><xs:field xpath="@amount"/></xs:unique>
  </xs:element>
</xs:schema>`)
			mustValidate(t, engine, `<root><row amount="-0"/><row amount="1"/></root>`)
			mustNotValidate(t, engine, `<root><row amount="-0"/><row amount="0"/></root>`, ErrValidationIdentity)
		})
	}
}

func TestIdentitySelectorSelfMatchesConstraintElement(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType><xs:attribute name="ref" type="xs:string"/></xs:complexType>
        </xs:element>
      </xs:sequence>
      <xs:attribute name="id" type="xs:string"/>
    </xs:complexType>
    <xs:key name="rootID"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>
    <xs:keyref name="itemRef" refer="rootID"><xs:selector xpath="item"/><xs:field xpath="@ref"/></xs:keyref>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root id="r"><item ref="r"/></root>`)
	mustNotValidate(t, engine, `<root><item ref="r"/></root>`, ErrValidationIdentity)
	mustNotValidate(t, engine, `<root id="r"><item ref="missing"/></root>`, ErrValidationIdentity)
}

func TestIdentityKeyTablesPropagateToAncestorKeyrefs(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="key" type="xs:string" minOccurs="0">
          <xs:key name="key"><xs:selector xpath="."/><xs:field xpath="."/></xs:key>
        </xs:element>
        <xs:element name="keyref">
          <xs:complexType><xs:attribute name="att" type="xs:string"/></xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:keyref name="keyref" refer="key"><xs:selector xpath="keyref"/><xs:field xpath="@att"/></xs:keyref>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root><key>xyz</key><keyref att="xyz"/></root>`)
	mustValidate(t, engine, `<root><keyref/></root>`)
	mustNotValidate(t, engine, `<root><keyref att="xyz"/></root>`, ErrValidationIdentity)
}

func TestIdentityXPathAxesAndSelfSteps(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence><xs:element name="code" type="xs:string"/></xs:sequence>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k"><xs:selector xpath="./child :: row/."/><xs:field xpath="attribute :: id"/></xs:key>
    <xs:unique name="u"><xs:selector xpath="child::row/."/><xs:field xpath="child::code/."/></xs:unique>
    <xs:unique name="desc"><xs:selector xpath=". //."/><xs:field xpath="@id"/></xs:unique>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root><row id="a"><code>x</code></row><row id="b"><code>y</code></row></root>`)
	mustNotValidate(t, engine, `<root><row id="a"><code>x</code></row><row id="a"><code>y</code></row></root>`, ErrValidationIdentity)
	mustNotValidate(t, engine, `<root><row id="a"><code>x</code></row><row id="b"><code>x</code></row></root>`, ErrValidationIdentity)
}

func TestIdentityWildcardElementNameTests(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice maxOccurs="unbounded">
        <xs:element name="row">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="code" type="xs:string" minOccurs="0"/>
              <xs:element name="alt" type="xs:string" minOccurs="0"/>
            </xs:sequence>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="other">
          <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
        </xs:element>
      </xs:choice>
    </xs:complexType>
    <xs:unique name="anyID"><xs:selector xpath="*"/><xs:field xpath="@id"/></xs:unique>
    <xs:unique name="nsID"><xs:selector xpath="t:*"/><xs:field xpath="@id"/></xs:unique>
    <xs:unique name="childText"><xs:selector xpath="t:row"/><xs:field xpath="*"/></xs:unique>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<t:root xmlns:t="urn:test"><t:row id="a"><t:code>x</t:code></t:row><t:other id="b"/></t:root>`)
	mustNotValidate(t, engine, `<t:root xmlns:t="urn:test"><t:row id="a"><t:code>x</t:code></t:row><t:other id="a"/></t:root>`, ErrValidationIdentity)
	mustNotValidate(t, engine, `<t:root xmlns:t="urn:test"><t:row id="a"><t:code>x</t:code><t:alt>y</t:alt></t:row></t:root>`, ErrValidationIdentity)
}

func TestIdentityAttributeWildcardFields(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded" nillable="true">
          <xs:complexType>
            <xs:attribute name="col" type="xs:string"/>
            <xs:attribute name="extra" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u"><xs:selector xpath="row"/><xs:field xpath="@*"/></xs:unique>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root><row col="1"/><row col="1"/></root>`, ErrValidationIdentity)
	mustNotValidate(t, engine, `<root><row col="1" extra="2"/></root>`, ErrValidationIdentity)
	mustNotValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><row col="1" xsi:nil="false"/></root>`, ErrValidationIdentity)

	engine = mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns="urn:test" xmlns:t="urn:test" elementFormDefault="qualified" attributeFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType><xs:attribute name="col" type="xs:string"/></xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u"><xs:selector xpath="t:row"/><xs:field xpath="@t:*"/></xs:unique>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test" xmlns:t="urn:test"><row t:col="1"/><row t:col="1"/></root>`, ErrValidationIdentity)
}

func TestDefaultedIDREFSAttributesAreResolved(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="refs" type="xs:IDREFS" default="missing"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root/>`, ErrValidationType)
}

func TestIdentityValuesUsePrimitiveTypeAndDefaultAttributes(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="uid" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="u"><xs:selector xpath=".//uid"/><xs:field xpath="."/></xs:unique>
  </xs:element>
  <xs:element name="uid" type="xs:anyType"/>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema"><uid xsi:type="xs:float">1</uid><uid xsi:type="xs:decimal">1</uid></root>`)
	mustNotValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema"><uid xsi:type="xs:int">1</uid><uid xsi:type="xs:decimal">1</uid></root>`, ErrValidationIdentity)

	engine = mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="uid" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="u"><xs:selector xpath=".//uid"/><xs:field xpath="@val"/></xs:unique>
  </xs:element>
  <xs:element name="uid"><xs:complexType><xs:attribute name="val" type="xs:string" default="test"/></xs:complexType></xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root><uid val="test"/><uid/></root>`, ErrValidationIdentity)

	engine = mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="uid" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="u"><xs:selector xpath=".//uid"/><xs:field xpath="."/></xs:unique>
  </xs:element>
  <xs:element name="uid" nillable="true"/>
</xs:schema>`)
	mustNotValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><uid xsi:nil="true"/><uid xsi:nil="true"/></root>`, ErrValidationIdentity)

	engine = mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="uid"/></xs:sequence></xs:complexType>
    <xs:keyref name="kr" refer="k"><xs:selector xpath=".//uid"/><xs:field xpath="pid"/></xs:keyref>
    <xs:key name="k"><xs:selector xpath=".//kid"/><xs:field xpath="@val"/></xs:key>
  </xs:element>
  <xs:element name="uid"><xs:complexType><xs:sequence><xs:element name="pid" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element>
  <xs:element name="kid"><xs:complexType><xs:attribute name="val" type="xs:string"/></xs:complexType></xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root><uid><pid>a</pid><pid>b</pid></uid><kid val="a"/></root>`, ErrValidationIdentity)
	mustNotValidate(t, engine, `<root><uid><pid/></uid><kid val="a"/></root>`, ErrValidationIdentity)
}

func TestNestedKeyrefCanReferToParentKeyDeclaredLaterAtCompile(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType><xs:attribute name="ref" type="xs:string"/></xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
          <xs:keyref name="refs" refer="keys"><xs:selector xpath="part"/><xs:field xpath="@ref"/></xs:keyref>
        </xs:element>
        <xs:element name="b">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="keys"><xs:selector xpath="b/part"/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`)
}

func TestKeyrefCanReferToKeyOnLaterCompiledElement(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test">
  <xs:element name="Global1">
    <xs:complexType><xs:sequence><xs:element name="item"><xs:complexType><xs:attribute name="ref"/></xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:keyref name="ref" refer="t:key"><xs:selector xpath="item"/><xs:field xpath="@ref"/></xs:keyref>
  </xs:element>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="item"><xs:complexType><xs:attribute name="id"/></xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:key name="key"><xs:selector xpath="item"/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`)
}

func TestIdentityUnprefixedSelectorUsesNoNamespace(t *testing.T) {
	engine := mustCompile(t, `
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
    <xs:unique name="u"><xs:selector xpath="item"/><xs:field xpath="."/></xs:unique>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns="urn:test"><item>a</item><item>a</item></root>`)
}

func TestIdentityFieldSelectingMultipleElementsIsInvalid(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="code" type="xs:string" maxOccurs="2"/>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u"><xs:selector xpath="item"/><xs:field xpath="code"/></xs:unique>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root><item><code>a</code><code>b</code></item></root>`, ErrValidationIdentity)
}

func TestIDAndIDREFValidation(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="node" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID"/>
            <xs:attribute name="ref" type="xs:IDREF"/>
            <xs:attribute name="refs" type="xs:IDREFS"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root><node id="a"/><node id="b" ref="a" refs="a b"/></root>`)
	mustNotValidate(t, engine, `<root><node id="a"/><node id="a"/></root>`, ErrValidationType)
	mustNotValidate(t, engine, `<root><node ref="missing"/></root>`, ErrValidationType)
}

func TestListOfIDValuesDoNotRecordDocumentIDs(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="ids">
	    <xs:list itemType="xs:ID"/>
	  </xs:simpleType>
	  <xs:element name="root">
	    <xs:complexType>
	      <xs:sequence>
	        <xs:element name="ids" type="ids"/>
	        <xs:element name="id" type="xs:ID"/>
	      </xs:sequence>
	    </xs:complexType>
	  </xs:element>
	</xs:schema>`)
	mustValidate(t, engine, `<root><ids>a b</ids><id>a</id></root>`)
}

func TestUnionIDTrackingUsesSelectedMember(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="idOrString">
	    <xs:union memberTypes="xs:ID xs:string"/>
	  </xs:simpleType>
	  <xs:element name="root">
	    <xs:complexType>
	      <xs:sequence>
	        <xs:element name="item" type="idOrString" maxOccurs="unbounded"/>
	      </xs:sequence>
	    </xs:complexType>
	  </xs:element>
	</xs:schema>`)
	mustNotValidate(t, engine, `<root><item>a</item><item>a</item></root>`, ErrValidationType)
}

func TestUnionIDREFTrackingUsesSelectedMember(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="refOrString">
	    <xs:union memberTypes="xs:IDREF xs:string"/>
	  </xs:simpleType>
	  <xs:element name="root">
	    <xs:complexType>
	      <xs:sequence>
	        <xs:element name="ref" type="refOrString"/>
	      </xs:sequence>
	    </xs:complexType>
	  </xs:element>
	</xs:schema>`)
	mustNotValidate(t, engine, `<root><ref>missing</ref></root>`, ErrValidationType)
}

func TestNestedUnionIDTrackingUsesSelectedMember(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="idOrString">
	    <xs:union memberTypes="xs:ID xs:string"/>
	  </xs:simpleType>
	  <xs:simpleType name="nested">
	    <xs:union memberTypes="idOrString xs:int"/>
	  </xs:simpleType>
	  <xs:element name="root">
	    <xs:complexType>
	      <xs:sequence>
	        <xs:element name="item" type="nested" maxOccurs="unbounded"/>
	      </xs:sequence>
	    </xs:complexType>
	  </xs:element>
	</xs:schema>`)
	mustNotValidate(t, engine, `<root><item>a</item><item>a</item></root>`, ErrValidationType)
}

func TestListUnionIDREFTrackingUsesSelectedMember(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="refOrInt">
	    <xs:union memberTypes="xs:IDREF xs:int"/>
	  </xs:simpleType>
	  <xs:simpleType name="refs">
	    <xs:list itemType="refOrInt"/>
	  </xs:simpleType>
	  <xs:element name="root">
	    <xs:complexType>
	      <xs:sequence>
	        <xs:element name="id" type="xs:ID"/>
	        <xs:element name="refs" type="refs"/>
	      </xs:sequence>
	    </xs:complexType>
	  </xs:element>
	</xs:schema>`)
	mustNotValidate(t, engine, `<root><id>a</id><refs>a 7 missing</refs></root>`, ErrValidationType)
}

func TestUnionDefaultElementIDTrackingUsesSelectedMember(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="idOrString">
	    <xs:union memberTypes="xs:ID xs:string"/>
	  </xs:simpleType>
	  <xs:element name="root">
	    <xs:complexType>
	      <xs:sequence>
	        <xs:element name="item" type="idOrString" default="a"/>
	        <xs:element name="other" type="xs:ID"/>
	      </xs:sequence>
	    </xs:complexType>
	  </xs:element>
	</xs:schema>`)
	mustNotValidate(t, engine, `<root><item/><other>a</other></root>`, ErrValidationType)
}

func TestUnionDefaultAttributeIDTrackingUsesSelectedMember(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:simpleType name="idOrString">
	    <xs:union memberTypes="xs:ID xs:string"/>
	  </xs:simpleType>
	  <xs:element name="root">
	    <xs:complexType>
	      <xs:sequence>
	        <xs:element name="item" type="xs:ID"/>
	      </xs:sequence>
	      <xs:attribute name="id" type="idOrString" default="a"/>
	    </xs:complexType>
	  </xs:element>
	</xs:schema>`)
	mustNotValidate(t, engine, `<root><item>a</item></root>`, ErrValidationType)
}

func TestWildcardAttributesRecordIDValues(t *testing.T) {
	engine := mustCompile(t, `
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="external" type="xs:ID"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="local" type="xs:ID"/>
      <xs:anyAttribute processContents="strict"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root local="one" external="two"/>`, ErrValidationType)
}

func TestValidateOptionsIdentityLimits(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u"><xs:selector xpath="row"/><xs:field xpath="@id"/></xs:unique>
  </xs:element>
</xs:schema>`)

	err := engine.ValidateWithOptions(strings.NewReader(`<root><row id="a"/><row id="b"/></root>`), ValidateOptions{MaxIdentityEntries: 1})
	expectCode(t, err, ErrValidationIdentity)

	err = engine.ValidateWithOptions(strings.NewReader(`<root><row id="abcd"/></root>`), ValidateOptions{MaxIdentityTupleBytes: 5})
	expectCode(t, err, ErrValidationIdentity)

	engine = mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="a" type="xs:string"/>
            <xs:attribute name="b" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u">
      <xs:selector xpath="row"/>
      <xs:field xpath="@a"/>
      <xs:field xpath="@b"/>
    </xs:unique>
  </xs:element>
</xs:schema>`)
	err = engine.ValidateWithOptions(strings.NewReader(`<root><row a="ab" b="cd"/></root>`), ValidateOptions{MaxIdentityTupleBytes: 9})
	if err != nil {
		t.Fatalf("ValidateWithOptions() error = %v", err)
	}
	err = engine.ValidateWithOptions(strings.NewReader(`<root><row a="ab" b="cd"/></root>`), ValidateOptions{MaxIdentityTupleBytes: 8})
	expectCode(t, err, ErrValidationIdentity)

	engine = mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child">
          <xs:complexType>
            <xs:sequence><xs:element name="grand" minOccurs="0"/></xs:sequence>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
          <xs:unique name="childUnique"><xs:selector xpath="grand"/><xs:field xpath="@id"/></xs:unique>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="rootUnique"><xs:selector xpath="child"/><xs:field xpath="@id"/></xs:unique>
  </xs:element>
</xs:schema>`)
	err = engine.ValidateWithOptions(strings.NewReader(`<root><child id="a"/></root>`), ValidateOptions{MaxIdentityScopes: 1})
	expectCode(t, err, ErrValidationIdentity)
}

func TestIdentitySelectionRecoveryCollectsMultipleFieldErrors(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="a" type="xs:string"/>
            <xs:attribute name="b" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="ua"><xs:selector xpath="row"/><xs:field xpath="@a"/></xs:unique>
    <xs:unique name="ub"><xs:selector xpath="row"/><xs:field xpath="@b"/></xs:unique>
  </xs:element>
</xs:schema>`)
	err := engine.ValidateWithOptions(
		strings.NewReader(`<root><row a="x" b="y"/><row a="x" b="y"/></root>`),
		ValidateOptions{MaxErrors: 2},
	)
	errs, ok := err.(Errors)
	if !ok {
		t.Fatalf("ValidateWithOptions() error = %T, want Errors", err)
	}
	if len(errs) != 2 {
		t.Fatalf("ValidateWithOptions() errors = %d, want 2", len(errs))
	}
	for _, err := range errs {
		expectCode(t, err, ErrValidationIdentity)
	}
}

func TestMultipleIDAttributeUsesAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:attribute name="a" type="xs:ID"/>
    <xs:attribute name="b" type="xs:ID"/>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}
