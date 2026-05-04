package xsd

import "testing"

func TestDuplicateIdenticalTargetNamespaceSourceIsIgnored(t *testing.T) {
	schema := []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t">
  <xs:element name="root"/>
</xs:schema>`)
	if _, err := Compile(sourceBytes("a.xsd", schema), sourceBytes("A.xsd", schema)); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestQNameAttributeDefaultUsesSchemaNamespaces(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="kind" type="xs:QName" default="xs:anyType"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestQNameEnumerationFacetUsesSchemaNamespaces(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:p="urn:p">
  <xs:simpleType name="q">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="p:name"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="q"/>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns:q="urn:p">q:name</root>`)
	mustNotValidate(t, engine, `<root xmlns:q="urn:p">q:other</root>`, ErrValidationFacet)
}

func TestValidateNamespacesAndXSIType(t *testing.T) {
	schema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
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
          <xs:element name="b" type="xs:int"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Base"/>
</xs:schema>`
	engine := mustCompile(t, schema)
	mustValidate(t, engine, `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test" xsi:type="tns:Derived"><a>x</a><b>7</b></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test"><a>x</a><b>7</b></root>`, ErrValidationElement)
}

func TestUndeclaredRootCanBeAssessedByXSIType(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test">
  <xs:complexType name="Root">
    <xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	mustValidate(t, engine, `<x:doc xmlns:x="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test" xsi:type="tns:Root"><a>x</a></x:doc>`)
	mustNotValidate(t, engine, `<x:doc xmlns:x="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Root"><a>x</a></x:doc>`, ErrValidationType)
}

func TestXSITypeRespectsElementBlock(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Ext">
    <xs:complexContent><xs:extension base="tns:Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:simpleType name="RestrictedString"><xs:restriction base="xs:string"><xs:minLength value="1"/></xs:restriction></xs:simpleType>
  <xs:element name="blockedExt" type="tns:Base" block="extension"/>
  <xs:element name="blockedRestriction" type="xs:string" block="restriction"/>
</xs:schema>`)
	mustNotValidate(t, engine, `<blockedExt xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test" xsi:type="tns:Ext"><a>x</a><b>y</b></blockedExt>`, ErrValidationType)
	mustNotValidate(t, engine, `<blockedRestriction xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test" xsi:type="tns:RestrictedString">x</blockedRestriction>`, ErrValidationType)

	defaultBlock := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified"
           blockDefault="extension">
  <xs:complexType name="Base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="Ext"><xs:complexContent><xs:extension base="tns:Base"/></xs:complexContent></xs:complexType>
  <xs:element name="root" type="tns:Base" block=""/>
</xs:schema>`)
	mustNotValidate(t, defaultBlock, `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test" xsi:type="tns:Ext"><a/></root>`, ErrValidationType)
}

func TestXSITypeAllowsUnionMemberType(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Upper">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Positive">
    <xs:restriction base="xs:positiveInteger"/>
  </xs:simpleType>
  <xs:simpleType name="Value">
    <xs:union memberTypes="tns:Upper tns:Positive"/>
  </xs:simpleType>
  <xs:element name="value" type="tns:Value"/>
  <xs:element name="blocked" type="tns:Value" block="restriction"/>
</xs:schema>`)
	mustValidate(t, engine, `<value xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test" xsi:type="tns:Upper">ABC</value>`)
	mustValidate(t, engine, `<value xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test" xsi:type="tns:Positive">12</value>`)
	mustNotValidate(t, engine, `<blocked xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test" xsi:type="tns:Upper">ABC</blocked>`, ErrValidationType)
}

func TestXSITypeAllowsSimpleTypeForAnyType(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="any" type="xs:anyType"/>
  <xs:element name="blocked" type="xs:anyType" block="restriction"/>
</xs:schema>`)
	mustValidate(t, engine, `<any xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="xs:boolean" xmlns:xs="http://www.w3.org/2001/XMLSchema">true</any>`)
	mustNotValidate(t, engine, `<blocked xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="xs:boolean" xmlns:xs="http://www.w3.org/2001/XMLSchema">true</blocked>`, ErrValidationType)
}

func TestXSITypeRespectsDeclaredTypeBlock(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base" block="extension">
    <xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="tns:Base"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Base"/>
</xs:schema>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="urn:test" xsi:type="tns:Derived"><a>x</a></root>`, ErrValidationType)
}

func TestSubstitutionGroupMemberMatchesHeadParticle(t *testing.T) {
	schema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="name" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="item" type="tns:Base" abstract="true"/>
  <xs:element name="book" type="tns:Base" substitutionGroup="tns:item"/>
  <xs:element name="paperback" type="tns:Base" substitutionGroup="tns:book"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="tns:item"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine := mustCompile(t, schema)
	mustValidate(t, engine, `<root xmlns="urn:test"><book><name>x</name></book></root>`)
	mustValidate(t, engine, `<root xmlns="urn:test"><paperback><name>x</name></paperback></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test"><item><name>x</name></item></root>`, ErrValidationElement)
}

func TestSubstitutionGroupRequiresDerivedType(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test">
  <xs:complexType name="A"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
  <xs:complexType name="B"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:complexType>
  <xs:element name="head" type="tns:A"/>
  <xs:element name="member" type="tns:B" substitutionGroup="tns:head"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestSubstitutionGroupSimpleContentComplexTypeDerivesFromSimpleHead(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:anySimpleType"/>
  <xs:complexType name="base">
    <xs:simpleContent><xs:extension base="xs:anySimpleType"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:simpleContent><xs:restriction base="base"/></xs:simpleContent>
  </xs:complexType>
  <xs:element name="member" type="derived" substitutionGroup="head"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="head"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root><member>value</member></root>`)
}

func TestSubstitutionGroupSimpleContentComplexTypeDerivesFromAnyTypeHead(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:anyType"/>
  <xs:element name="member" substitutionGroup="head">
    <xs:complexType>
      <xs:simpleContent>
        <xs:extension base="xs:boolean">
          <xs:attribute name="nilReason" type="xs:string"/>
        </xs:extension>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="head"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root><member>true</member></root>`)
	mustNotValidate(t, engine, `<root><member>maybe</member></root>`, ErrValidationFacet)

	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:anyType" final="restriction"/>
  <xs:element name="member" substitutionGroup="head">
    <xs:complexType>
      <xs:simpleContent><xs:extension base="xs:boolean"/></xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestSubstitutionGroupMemberDefaultsToHeadType(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string"><xs:enumeration value="A"/></xs:restriction>
  </xs:simpleType>
  <xs:element name="head" type="tns:Code"/>
  <xs:element name="member" substitutionGroup="tns:head"/>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="tns:head"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns="urn:test"><member>A</member></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test"><member>B</member></root>`, ErrValidationFacet)
}

func TestSubstitutionGroupUPAIsCheckedAfterSubstitutionsCompile(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="head"/>
  <xs:element name="member" substitutionGroup="tns:head"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element ref="tns:head"/>
        <xs:element ref="tns:member"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestQNameReferencesRequireDirectImport(t *testing.T) {
	_, err := Compile(
		sourceBytes("main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:a="urn:a"
           xmlns:b="urn:b">
  <xs:import namespace="urn:a" schemaLocation="a.xsd"/>
  <xs:element name="root" type="b:B"/>
</xs:schema>`)),
		sourceBytes("a.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a">
  <xs:import namespace="urn:b" schemaLocation="b.xsd"/>
</xs:schema>`)),
		sourceBytes("b.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b">
  <xs:complexType name="B"/>
</xs:schema>`)),
	)
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(
		sourceBytes("main.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:b="urn:b">
  <xs:import namespace="urn:b" schemaLocation="b.xsd"/>
  <xs:element name="root" type="b:B"/>
</xs:schema>`)),
		sourceBytes("b.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b">
  <xs:complexType name="B"/>
</xs:schema>`)),
	)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestSubstitutionGroupHeadBlockDerivation(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
  <xs:complexType name="Ext"><xs:complexContent><xs:extension base="tns:Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
  <xs:element name="head" type="tns:Base" block="extension"/>
  <xs:element name="member" type="tns:Ext" substitutionGroup="tns:head"/>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="tns:head"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test"><member><a>x</a><b>y</b></member></root>`, ErrValidationElement)

	typeBlock := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base" block="restriction"/>
  <xs:complexType name="Restricted"><xs:complexContent><xs:restriction base="tns:Base"/></xs:complexContent></xs:complexType>
  <xs:element name="head" type="tns:Base"/>
  <xs:element name="member" type="tns:Restricted" substitutionGroup="tns:head"/>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="tns:head"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	mustNotValidate(t, typeBlock, `<root xmlns="urn:test"><member/></root>`, ErrValidationElement)

	blockSubstitution := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="head" block="substitution"/>
  <xs:element name="member" substitutionGroup="tns:head"/>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="tns:head"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	mustNotValidate(t, blockSubstitution, `<root xmlns="urn:test"><member/></root>`, ErrValidationElement)
}

func TestSubstitutionGroupTransitiveIntermediateBlock(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test" elementFormDefault="qualified">
  <xs:complexType name="Base"/>
  <xs:complexType name="Mid" block="restriction"><xs:complexContent><xs:extension base="t:Base"/></xs:complexContent></xs:complexType>
  <xs:complexType name="Leaf"><xs:complexContent><xs:restriction base="t:Mid"/></xs:complexContent></xs:complexType>
  <xs:element name="head" type="t:Base"/>
  <xs:element name="mid" type="t:Mid" substitutionGroup="t:head"/>
  <xs:element name="leaf" type="t:Leaf" substitutionGroup="t:mid"/>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="t:head"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<root xmlns="urn:test"><mid/></root>`)
	mustNotValidate(t, engine, `<root xmlns="urn:test"><leaf/></root>`, ErrValidationElement)
}

func TestSchemaNamespaceDeclarationsAreValidated(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"><xs:import namespace="urn:test"/></xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace=""/></xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace=""/>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}

func TestElementValueConstraintUsesSchemaNamespaces(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
  <xs:element name="root" fixed="p:name">
    <xs:simpleType>
      <xs:union memberTypes="xs:int xs:QName"/>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)
}

func TestXSISchemaLocationHintIsUnsupportedWhenRequired(t *testing.T) {
	elementEngine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="urn:f" processContents="strict"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, elementEngine, `<root xmlns:f="urn:f"><f:item/></root>`, ErrValidationElement)
	mustNotValidate(t, elementEngine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:f="urn:f" xsi:schemaLocation="urn:f f.xsd"><f:item/></root>`, ErrUnsupportedSchemaHint)

	attributeEngine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="urn:f" processContents="strict"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustNotValidate(t, attributeEngine, `<root xmlns:f="urn:f" f:item="x"/>`, ErrValidationAttribute)
	mustNotValidate(t, attributeEngine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:f="urn:f" xsi:schemaLocation="urn:f f.xsd" f:item="x"/>`, ErrUnsupportedSchemaHint)
}

func TestWildcardNamespaceRejectsInvalidSpecialTokens(t *testing.T) {
	for _, namespace := range []string{"##target", "##all", "##any ##local", "##any ##other"} {
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:any namespace="` + namespace + `"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
		_, err := Compile(sourceBytes("schema.xsd", []byte(schema)))
		expectCode(t, err, ErrSchemaInvalidAttribute)
	}

	mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:any namespace=""/></xs:sequence><xs:anyAttribute namespace=""/></xs:complexType></xs:element></xs:schema>`)
}

func TestRestrictionWildcardNamespaceAndEmptiableTerms(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:any namespace="##any"/>
      <xs:sequence><xs:element name="a" minOccurs="0"/></xs:sequence>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence><xs:element name="a"/><xs:element name="a" minOccurs="0"/></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:any namespace="##other" minOccurs="2" maxOccurs="3"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="e"/><xs:element name="e"/><xs:any namespace="##other"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRepeatingSequenceWildcardNamespaceOverlapIsUPACompileError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:complexType name="bad">
    <xs:sequence maxOccurs="10">
      <xs:any namespace="##other" maxOccurs="2" processContents="lax"/>
      <xs:any namespace="urn:other" processContents="lax"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestCyclicSubstitutionGroupsAreSchemaErrors(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" substitutionGroup="b"/>
  <xs:element name="b" substitutionGroup="a"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestQNameValueUsesInstanceNamespacesWithoutInterning(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="q" type="xs:QName"/>
</xs:schema>`)
	localsBefore := len(engine.rt.Names.locals)
	mustValidate(t, engine, `<q xmlns:p="urn:dynamic">p:notInSchema</q>`)
	if got := len(engine.rt.Names.locals); got != localsBefore {
		t.Fatalf("QName validation interned instance local names: before=%d after=%d", localsBefore, got)
	}
	mustNotValidate(t, engine, `<q>p:notBound</q>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<q>bad:name:shape</q>`, ErrValidationFacet)
}
