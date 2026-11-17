package schema_test

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func inSimpleType(derivation string) string {
	return `  <xs:simpleType name="S">
    ` + derivation + `
  </xs:simpleType>`
}

func inTopLevel(element, children string) string {
	return `  <xs:` + element + ` name="N">
    ` + children + `
  </xs:` + element + `>`
}

func inIdentity(children string) string {
	return `  <xs:element name="r">
    <xs:key name="k">
      ` + children + `
    </xs:key>
  </xs:element>`
}

func TestChildAdmissionWinsOverLaterErrors(t *testing.T) {
	t.Parallel()
	misplaced := func(name, body, msg string, line, col int) placementAttributeCase {
		return placementAttributeCase{name: name, body: body, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: msg, wantLine: line, wantCol: col}
	}
	const laterBadAttribute = `<xs:element name="z" foo="1"/>`
	cases := []placementAttributeCase{
		misplaced("schema unknown child before later attribute", "  <xs:future/>\n  "+laterBadAttribute, `invalid top-level schema child "future"`, 2, 3),
		misplaced("schema unknown child before its own attribute", `  <xs:future foo="1"/>`, `invalid top-level schema child "future"`, 2, 3),
		misplaced("schema misplaced child before its own attribute", `  <xs:sequence foo="1"/>`, `invalid top-level schema child "sequence"`, 2, 3),
		misplaced("schema misplaced child before unbound prefix", `  <xs:sequence><xs:element ref="p:E"/></xs:sequence>`, `invalid top-level schema child "sequence"`, 2, 3),
		misplaced("schema identity constraint", `  <xs:key name="k"><xs:selector xpath="."/><xs:field xpath="."/></xs:key>`, "identity constraint must be inside element", 2, 3),
		misplaced("schema include after declaration", "  <xs:element name=\"e\"/>\n  <xs:include schemaLocation=\"other.xsd\"/>", `xs:"include" must precede global declarations`, 3, 3),
		misplaced("schema import after declaration", "  <xs:element name=\"e\"/>\n  <xs:import namespace=\"urn:other\"/>", `xs:"import" must precede global declarations`, 3, 3),
		misplaced("schema include after declaration before its own attribute", "  <xs:element name=\"e\"/>\n  <xs:include schemaLocation=\"other.xsd\" foo=\"1\"/>", `xs:"include" must precede global declarations`, 3, 3),
		misplaced("schema notation inside element", inElement(`<xs:notation name="N" public="p"/>`), `invalid element child "notation"`, 3, 5),

		misplaced("complexType unknown child before later attribute", inComplexType(`<xs:future/><xs:attribute name="a" foo="1"/>`), `invalid complexType child "future"`, 3, 5),
		misplaced("complexType element child", inComplexType(`<xs:element name="e" type="p:T"/>`), `invalid complexType child "element"`, 3, 5),
		misplaced("complexType facet child", inComplexType(`<xs:minLength value="1"/>`), `invalid complexType child "minLength"`, 3, 5),
		misplaced("complexType model group after attribute", inComplexType(`<xs:attribute name="a"/><xs:sequence foo="1"/>`), "complexType model group is out of order", 3, 5+len(`<xs:attribute name="a"/>`)),
		misplaced("complexType second model group", inComplexType(`<xs:sequence/><xs:choice foo="1"/>`), "complexType model group is out of order", 3, 5+len(`<xs:sequence/>`)),
		misplaced("complexType attribute after anyAttribute", inComplexType(`<xs:anyAttribute/><xs:attribute name="a" foo="1"/>`), "complexType attribute is out of order", 3, 5+len(`<xs:anyAttribute/>`)),
		misplaced("complexType second anyAttribute", inComplexType(`<xs:anyAttribute/><xs:anyAttribute foo="1"/>`), "complexType can contain at most one anyAttribute", 3, 5+len(`<xs:anyAttribute/>`)),
		misplaced("complexType child after content", inComplexType(`<xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent><xs:attribute name="a" foo="1"/>`), `invalid complexType child "attribute"`, 3, 5+len(`<xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>`)),

		misplaced("simpleContent unknown child", inContent("simpleContent", `<xs:future/><xs:extension base="p:T"/>`), `invalid simpleContent child "future"`, 4, 7),
		misplaced("complexContent model group child", inContent("complexContent", `<xs:sequence/>`), `invalid complexContent child "sequence"`, 4, 7),
		misplaced("complexContent second derivation", inContent("complexContent", `<xs:extension base="xs:anyType"/><xs:restriction base="p:T"/>`), "complexContent can contain one derivation", 4, 7+len(`<xs:extension base="xs:anyType"/>`)),
		misplaced("simpleContent restriction unknown child", inContent("simpleContent", `<xs:restriction base="B"><xs:future/></xs:restriction>`), `invalid simpleContent restriction child "future"`, 4, 7+len(`<xs:restriction base="B">`)),
		misplaced("simpleContent restriction facet after attribute", inContent("simpleContent", `<xs:restriction base="xs:string"><xs:attribute name="a"/><xs:minLength value="1"/></xs:restriction>`), "simpleContent facet is out of order", 4, 7+len(`<xs:restriction base="xs:string"><xs:attribute name="a"/>`)),
		misplaced("simpleContent extension simpleType", inContent("simpleContent", `<xs:extension base="B"><xs:simpleType/></xs:extension>`), "simpleContent extension cannot contain simpleType", 4, 7+len(`<xs:extension base="B">`)),
		misplaced("simpleContent extension facet", inContent("simpleContent", `<xs:extension base="xs:string"><xs:minLength value="1"/></xs:extension>`), `invalid simpleContent extension child "minLength"`, 4, 7+len(`<xs:extension base="xs:string">`)),
		misplaced("complexContent extension model group after attribute", inContent("complexContent", `<xs:extension base="B"><xs:attribute name="a"/><xs:sequence/></xs:extension>`), "extension model group is out of order", 4, 7+len(`<xs:extension base="B"><xs:attribute name="a"/>`)),
		misplaced("simpleContent restriction simpleType after facet", inContent("simpleContent", `<xs:restriction base="B"><xs:minLength value="1"/><xs:simpleType/></xs:restriction>`), "simpleContent simpleType is out of order", 4, 7+len(`<xs:restriction base="B"><xs:minLength value="1"/>`)),
		misplaced("simpleContent restriction second simpleType", inContent("simpleContent", `<xs:restriction base="B"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType/></xs:restriction>`), "simpleContent simpleType is out of order", 4, 7+len(`<xs:restriction base="B"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`)),
		misplaced("simpleContent extension second anyAttribute", inContent("simpleContent", `<xs:extension base="B"><xs:anyAttribute/><xs:anyAttribute/></xs:extension>`), "simpleContent can contain at most one anyAttribute", 4, 7+len(`<xs:extension base="B"><xs:anyAttribute/>`)),
		misplaced("complexContent restriction model group after attribute", inContent("complexContent", `<xs:restriction base="B"><xs:attribute name="a"/><xs:choice/></xs:restriction>`), "restriction model group is out of order", 4, 7+len(`<xs:restriction base="B"><xs:attribute name="a"/>`)),
		misplaced("complexContent extension second anyAttribute", inContent("complexContent", `<xs:extension base="B"><xs:anyAttribute/><xs:anyAttribute/></xs:extension>`), "extension can contain at most one anyAttribute", 4, 7+len(`<xs:extension base="B"><xs:anyAttribute/>`)),
		misplaced("complexContent restriction simpleType child", inContent("complexContent", `<xs:restriction base="B"><xs:simpleType/></xs:restriction>`), `invalid complexContent child "simpleType"`, 4, 7+len(`<xs:restriction base="B">`)),
		misplaced("complexContent restriction unknown child", inContent("complexContent", `<xs:restriction base="B"><xs:future/></xs:restriction>`), `invalid complexContent child "future"`, 4, 7+len(`<xs:restriction base="B">`)),

		misplaced("sequence unknown child before later attribute", inSequence(`<xs:future/>`+laterBadAttribute), `invalid model group child "future"`, 4, 7),
		misplaced("sequence complexType child", inSequence(`<xs:complexType/>`+laterBadAttribute), `invalid model group child "complexType"`, 4, 7),
		misplaced("sequence attribute child before unbound prefix", inSequence(`<xs:attribute name="a" type="p:T"/>`), `invalid model group child "attribute"`, 4, 7),
		misplaced("sequence nested all", inSequence(`<xs:all foo="1"/>`), "xs:all cannot be nested in model groups", 4, 7),
		misplaced("choice nested all", `  <xs:complexType name="C">
    <xs:choice>
      <xs:all/>
    </xs:choice>
  </xs:complexType>`, "xs:all cannot be nested in model groups", 4, 7),
		misplaced("all nested sequence before unknown child", `  <xs:complexType name="C">
    <xs:all>
      <xs:sequence/><xs:future/>
    </xs:all>
  </xs:complexType>`, "xs:all can contain only element particles", 4, 7),
		misplaced("all wildcard child", `  <xs:complexType name="C">
    <xs:all>
      <xs:any/>
    </xs:all>
  </xs:complexType>`, "xs:all can contain only element particles", 4, 7),
		misplaced("sequence reference to all group before the group's own defect", `  <xs:complexType name="C">
    <xs:sequence>
      <xs:group ref="G"/>
    </xs:sequence>
  </xs:complexType>
  <xs:group name="G"><xs:all><xs:element name="a" maxOccurs="2"/></xs:all></xs:group>`, "xs:all cannot be nested in model groups", 4, 7),
		misplaced("named group model unknown child", inTopLevel("group", `<xs:sequence><xs:future/></xs:sequence>`), `invalid model group child "future"`, 3, 5+len(`<xs:sequence>`)),

		misplaced("simpleType unknown child before unbound base", inSimpleType(`<xs:future/><xs:restriction base="p:T"/>`), `invalid simpleType child "future"`, 3, 5),
		misplaced("simpleType content child", inSimpleType(`<xs:complexContent/>`), `invalid simpleType child "complexContent"`, 3, 5),
		misplaced("simpleType second derivation", inSimpleType(`<xs:restriction base="xs:string"/><xs:list itemType="p:T"/>`), "simpleType can contain one restriction, list, or union", 3, 5+len(`<xs:restriction base="xs:string"/>`)),
		misplaced("restriction attribute child before its unbound prefix", inSimpleType(`<xs:restriction base="B"><xs:attribute name="a" type="p:T"/></xs:restriction>`), `invalid restriction child "attribute"`, 3, 5+len(`<xs:restriction base="B">`)),
		misplaced("restriction simpleType after facet", inSimpleType(`<xs:restriction><xs:minLength value="1"/><xs:simpleType/></xs:restriction>`), "restriction simpleType must precede facets", 3, 5+len(`<xs:restriction><xs:minLength value="1"/>`)),
		misplaced("restriction second simpleType", inSimpleType(`<xs:restriction><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType/></xs:restriction>`), "restriction can contain one simpleType", 3, 5+len(`<xs:restriction><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`)),
		misplaced("list facet child", inSimpleType(`<xs:list itemType="xs:int"><xs:minLength value="1"/></xs:list>`), `invalid list child "minLength"`, 3, 5+len(`<xs:list itemType="xs:int">`)),
		misplaced("list second simpleType", inSimpleType(`<xs:list><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType/></xs:list>`), "list can contain one simpleType", 3, 5+len(`<xs:list><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`)),
		misplaced("union element child before its unbound prefix", inSimpleType(`<xs:union memberTypes="xs:int"><xs:element name="e" type="p:T"/></xs:union>`), `invalid union child "element"`, 3, 5+len(`<xs:union memberTypes="xs:int">`)),

		misplaced("element attribute child before unbound type", inElement(`<xs:attribute name="a" type="p:T"/>`), `invalid element child "attribute"`, 3, 5),
		misplaced("element second anonymous type", inElement(`<xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType><xs:complexType foo="1"/>`), "element can contain at most one anonymous type", 3, 5+len(`<xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`)),
		misplaced("element anonymous type after identity constraint", inElement(`<xs:unique name="u"><xs:selector xpath="."/><xs:field xpath="."/></xs:unique><xs:complexType/>`), "element anonymous type must precede identity constraints", 3, 5+len(`<xs:unique name="u"><xs:selector xpath="."/><xs:field xpath="."/></xs:unique>`)),
		misplaced("local element unknown child", inSequence(`<xs:element name="e"><xs:future/></xs:element>`), `invalid element child "future"`, 4, 7+len(`<xs:element name="e">`)),

		misplaced("attribute complexType child before unbound type", inTopLevel("attribute", `<xs:complexType/><xs:simpleType><xs:restriction base="p:T"/></xs:simpleType>`), `invalid attribute child "complexType"`, 3, 5),
		misplaced("attribute second simpleType", inTopLevel("attribute", `<xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType foo="1"/>`), "attribute can contain at most one simpleType", 3, 5+len(`<xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`)),
		misplaced("local attribute unknown child", inComplexType(`<xs:attribute name="a"><xs:future/></xs:attribute>`), `invalid attribute child "future"`, 3, 5+len(`<xs:attribute name="a">`)),

		misplaced("attributeGroup model group child", inTopLevel("attributeGroup", `<xs:sequence/>`), `invalid attribute use child "sequence"`, 3, 5),
		misplaced("attributeGroup attribute after anyAttribute", inTopLevel("attributeGroup", `<xs:anyAttribute/><xs:attribute name="a" type="p:T"/>`), "attributeGroup attribute is out of order", 3, 5+len(`<xs:anyAttribute/>`)),
		misplaced("attributeGroup second anyAttribute", inTopLevel("attributeGroup", `<xs:anyAttribute/><xs:anyAttribute foo="1"/>`), "attributeGroup can contain at most one anyAttribute", 3, 5+len(`<xs:anyAttribute/>`)),

		misplaced("group element child", inTopLevel("group", `<xs:element name="e" foo="1"/>`), `invalid top-level group child "element"`, 3, 5),
		misplaced("group second model group", inTopLevel("group", `<xs:sequence/><xs:choice foo="1"/>`), "top-level group must contain exactly one model group", 3, 5+len(`<xs:sequence/>`)),
		misplaced("top-level group reference child", inTopLevel("group", `<xs:group ref="p:G"/>`), "top-level group cannot contain group ref", 3, 5),

		misplaced("identity field before selector", inIdentity(`<xs:field xpath="."/><xs:selector xpath="." foo="1"/>`), "identity constraint field requires selector", 4, 7),
		misplaced("identity second selector", inIdentity(`<xs:selector xpath="."/><xs:selector foo="1"/>`), "identity constraint can contain at most one selector", 4, 7+len(`<xs:selector xpath="."/>`)),
		misplaced("identity selector after field", inIdentity(`<xs:selector xpath="."/><xs:field xpath="."/><xs:selector xpath="."/>`), "identity constraint can contain at most one selector", 4, 7+len(`<xs:selector xpath="."/><xs:field xpath="."/>`)),
		misplaced("identity element child", inIdentity(`<xs:selector xpath="."/><xs:element name="e" type="p:T"/>`), `invalid identity constraint child "element"`, 4, 7+len(`<xs:selector xpath="."/>`)),

		misplaced("earlier text wins over misplaced child", inSequence(`text<xs:complexType/>`), `xs:"sequence" cannot contain text`, 3, 5),
		{name: "XSD 1.1 vocabulary wins over misplaced child", body: inSequence(`<xs:assert test="true()"/>`), wantCode: xsderrors.CodeUnsupportedXSD11, wantMsg: "XSD 1.1 feature assert is not supported", wantLine: 4, wantCol: 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compilePlacementAttributeCase(t, tc)
		})
	}
}

func TestChildAdmissionAcceptsSchemaForSchemasContent(t *testing.T) {
	t.Parallel()
	body := `  <xs:annotation/>
  <xs:import namespace="urn:other"/>
  <xs:annotation/>
  <xs:simpleType name="S">
    <xs:annotation/>
    <xs:restriction>
      <xs:simpleType><xs:list itemType="xs:int"/></xs:simpleType>
      <xs:minLength value="1"/><xs:maxLength value="3"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="U"><xs:union memberTypes="xs:int"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType><xs:restriction base="xs:date"/></xs:simpleType></xs:union></xs:simpleType>
  <xs:annotation/>
  <xs:attributeGroup name="AG"><xs:attribute name="a"/><xs:attributeGroup ref="AG2"/><xs:anyAttribute/></xs:attributeGroup>
  <xs:attributeGroup name="AG2"><xs:attribute name="b"/></xs:attributeGroup>
  <xs:group name="G"><xs:annotation/><xs:choice><xs:element name="c"/><xs:sequence><xs:any namespace="urn:other"/></xs:sequence><xs:group ref="G2"/></xs:choice></xs:group>
  <xs:group name="G2"><xs:sequence><xs:element name="d"/><xs:element name="e"/></xs:sequence></xs:group>
  <xs:group name="G3"><xs:all><xs:element name="d"/><xs:element name="e"/></xs:all></xs:group>
  <xs:complexType name="C">
    <xs:sequence><xs:element name="f"/></xs:sequence>
    <xs:attribute name="g"/><xs:attributeGroup ref="AG2"/><xs:attribute name="h"/>
    <xs:anyAttribute/>
  </xs:complexType>
  <xs:complexType name="D"><xs:complexContent><xs:extension base="C"><xs:sequence><xs:element name="i"/></xs:sequence><xs:attribute name="j"/><xs:anyAttribute/></xs:extension></xs:complexContent></xs:complexType>
  <xs:complexType name="E"><xs:simpleContent><xs:extension base="xs:string"><xs:attribute name="k"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:complexType name="F"><xs:simpleContent><xs:restriction base="E"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType><xs:maxLength value="4"/><xs:attribute name="k"/></xs:restriction></xs:simpleContent></xs:complexType>
  <xs:element name="r">
    <xs:complexType><xs:all><xs:element name="v" type="xs:string"/></xs:all></xs:complexType>
    <xs:unique name="u"><xs:selector xpath="."/><xs:field xpath="v"/><xs:field xpath="@w"/></xs:unique>
    <xs:key name="k"><xs:selector xpath="."/><xs:field xpath="v"/></xs:key>
    <xs:keyref name="kr" refer="k"><xs:selector xpath="."/><xs:field xpath="v"/></xs:keyref>
  </xs:element>
  <xs:attribute name="t"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:attribute>
  <xs:notation name="N" public="p"/>`
	compilePlacementAttributeCase(t, placementAttributeCase{name: "schema for schemas content", body: body})
}

func TestRequiredChildReportedAtItsParent(t *testing.T) {
	t.Parallel()
	missing := func(name, body string, code xsderrors.Code, msg string, line, col int) placementAttributeCase {
		return placementAttributeCase{name: name, body: body, wantCode: code, wantMsg: msg, wantLine: line, wantCol: col}
	}
	cases := []placementAttributeCase{
		missing("simpleType without derivation", `  <xs:simpleType name="S"><xs:annotation/></xs:simpleType>`, xsderrors.CodeSchemaContentModel, "simpleType must contain one restriction, list, or union", 2, 3),
		missing("simpleContent without derivation", inComplexType(`<xs:simpleContent><xs:annotation/></xs:simpleContent>`), xsderrors.CodeSchemaContentModel, "simpleContent missing extension or restriction", 3, 5),
		missing("complexContent without derivation", inComplexType(`<xs:complexContent/>`), xsderrors.CodeSchemaContentModel, "complexContent missing extension or restriction", 3, 5),
		missing("top-level group without model group", `  <xs:group name="G"><xs:annotation/></xs:group>`, xsderrors.CodeSchemaContentModel, "top-level group must contain exactly one model group", 2, 3),
		missing("identity constraint without selector", inIdentity(`<xs:annotation/>`), xsderrors.CodeSchemaIdentity, "identity constraint missing selector", 3, 5),
		missing("identity constraint without field", inIdentity(`<xs:selector xpath="."/>`), xsderrors.CodeSchemaIdentity, "identity constraint missing fields", 3, 5),
		missing("element with type and anonymous type", `  <xs:element name="e" type="xs:string"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:element>`, xsderrors.CodeSchemaInvalidAttribute, "element cannot have both type and anonymous type", 2, 3),
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compilePlacementAttributeCase(t, tc)
		})
	}
}

func TestQNamePrefixCheckedOnStartTag(t *testing.T) {
	t.Parallel()
	unbound := func(name, body string, line, col int) placementAttributeCase {
		return placementAttributeCase{name: name, body: body, wantCode: xsderrors.CodeSchemaReference, wantMsg: `unbound QName prefix "p"`, wantLine: line, wantCol: col}
	}
	cases := []placementAttributeCase{
		unbound("restriction base before misplaced child", inSimpleType(`<xs:restriction base="p:T"><xs:attribute name="a"/></xs:restriction>`), 3, 5),
		unbound("extension base before out-of-order child", inContent("complexContent", `<xs:extension base="p:T"><xs:attribute name="a"/><xs:sequence/></xs:extension>`), 4, 7),
		unbound("simpleContent restriction base before unknown child", inContent("simpleContent", `<xs:restriction base="p:T"><xs:future/></xs:restriction>`), 4, 7),
		unbound("list itemType before misplaced child", inSimpleType(`<xs:list itemType="p:T"><xs:minLength value="1"/></xs:list>`), 3, 5),
		unbound("second memberTypes name before misplaced child", inSimpleType(`<xs:union memberTypes="xs:int p:T"><xs:element name="e"/></xs:union>`), 3, 5),
		unbound("keyref refer before misplaced child", inElement(`<xs:keyref name="r" refer="p:K"><xs:element name="e"/></xs:keyref>`), 3, 5),
		unbound("element type before misplaced child", inSequence(`<xs:element name="e" type="p:T"><xs:attribute name="a"/></xs:element>`), 4, 7),
		unbound("element type before later unknown attribute", inSequence(`<xs:element name="e" type="p:T" foo="1"/>`), 4, 7),
		unbound("substitutionGroup before later unknown attribute", `  <xs:element name="e" substitutionGroup="p:H" foo="1"/>`, 2, 3),
		unbound("attribute type before later sibling error", inComplexType(`<xs:attribute name="a" type="p:T"/><xs:attribute name="b" foo="1"/>`), 3, 5),
		unbound("group ref before later sibling error", inSequence(`<xs:group ref="p:G"/>`+`<xs:element name="z" foo="1"/>`), 4, 7),
		unbound("attributeGroup ref before misplaced child", inComplexType(`<xs:attributeGroup ref="p:AG"><xs:attribute name="a"/></xs:attributeGroup>`), 3, 5),
		unbound("element ref before later misplaced sibling", inSequence(`<xs:element ref="p:E" minOccurs="0"/><xs:complexType/>`), 4, 7),
		{name: "unknown attribute before later unbound type", body: inSequence(`<xs:element name="e" foo="1" type="p:T"/>`), wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: `"element" cannot have attribute "foo"`, wantLine: 4, wantCol: 7},
		{name: "forbidden attribute before later unbound ref", body: inSequence(`<xs:element type="xs:string" ref="p:E"/>`), wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: `element ref cannot have attribute "type"`, wantLine: 4, wantCol: 7},
		{name: "invalid QName before misplaced child", body: inSimpleType(`<xs:restriction base="1x"><xs:attribute name="a"/></xs:restriction>`), wantCode: xsderrors.CodeSchemaReference, wantMsg: `invalid QName "1x"`, wantLine: 3, wantCol: 5},
		{name: "prefix bound on the same start tag", body: `  <xs:element name="e" xmlns:p="http://www.w3.org/2001/XMLSchema" type="p:string"/>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compilePlacementAttributeCase(t, tc)
		})
	}
}
