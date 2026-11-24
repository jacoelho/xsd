package schema_test

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityXPathAttributeWinsOverLaterErrors(t *testing.T) {
	t.Parallel()
	identity := func(name, body, msg string, line, col int) placementAttributeCase {
		return placementAttributeCase{name: name, body: body, wantCode: xsderrors.CodeSchemaIdentity, wantMsg: msg, wantLine: line, wantCol: col}
	}
	cases := []placementAttributeCase{
		identity("selector missing xpath before later field attribute", inIdentity(`<xs:selector/><xs:field xpath="@a" foo="1"/>`), `"selector" missing xpath`, 4, 7),
		identity("field missing xpath before later unbound prefix", inIdentity(`<xs:selector xpath="."/><xs:field/></xs:key><xs:keyref name="r" refer="p:k"><xs:selector xpath="."/><xs:field xpath="."/></xs:keyref>`), `"field" missing xpath`, 4, 7+len(`<xs:selector xpath="."/>`)),
		identity("selector empty xpath before later attribute", inIdentity(`<xs:selector xpath="" foo="1"/><xs:field xpath="@a"/>`), `"selector" xpath is empty`, 4, 7),
		identity("field whitespace xpath before later sibling", inIdentity(`<xs:selector xpath="."/><xs:field xpath=" &#9; "/><xs:field foo="1"/>`), `"field" xpath is empty`, 4, 7+len(`<xs:selector xpath="."/>`)),
		{name: "earlier unknown attribute before empty xpath", body: inIdentity(`<xs:selector foo="1" xpath=""/><xs:field xpath="@a"/>`), wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: `"selector" cannot have attribute "foo"`, wantLine: 4, wantCol: 7},
		{name: "earlier unknown attribute before missing xpath", body: inIdentity(`<xs:selector foo="1"/><xs:field xpath="@a"/>`), wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: `"selector" cannot have attribute "foo"`, wantLine: 4, wantCol: 7},
		{name: "earlier sibling error before missing xpath", body: inIdentity(`<xs:selector xpath="."/><xs:field xpath="@a"/><xs:selector/>`), wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "identity constraint can contain at most one selector", wantLine: 4, wantCol: 7 + len(`<xs:selector xpath="."/><xs:field xpath="@a"/>`)},
		{name: "valid xpaths", body: inIdentity(`<xs:selector xpath="."/><xs:field xpath="@a"/>`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compilePlacementAttributeCase(t, tc)
		})
	}
}

func TestMissingRequiredChildWinsOverLaterErrors(t *testing.T) {
	t.Parallel()
	const laterUnknownAttribute = "\n  <xs:element name=\"z\" foo=\"1\"/>"
	const laterUnboundPrefix = "\n  <xs:element name=\"z\" type=\"p:T\"/>"
	missing := func(name, body string, code xsderrors.Code, msg string, line, col int) placementAttributeCase {
		return placementAttributeCase{name: name, body: body, wantCode: code, wantMsg: msg, wantLine: line, wantCol: col}
	}
	const (
		contentModel = xsderrors.CodeSchemaContentModel
		identity     = xsderrors.CodeSchemaIdentity
		reference    = xsderrors.CodeSchemaReference
	)
	const noDerivation = "simpleType must contain one restriction, list, or union"
	cases := []placementAttributeCase{
		missing("empty simpleType before later unbound prefix", `  <xs:simpleType name="S"/>`+laterUnboundPrefix, contentModel, noDerivation, 2, 3),
		missing("annotated simpleType before later attribute", `  <xs:simpleType name="S"><xs:annotation/></xs:simpleType>`+laterUnknownAttribute, contentModel, noDerivation, 2, 3),
		missing("local simpleType before later attribute", inElement(`<xs:simpleType/>`)+laterUnknownAttribute, contentModel, noDerivation, 3, 5),
		missing("simpleContent without derivation before later attribute", inContent("simpleContent", `<xs:annotation/>`)+laterUnknownAttribute, contentModel, "simpleContent missing extension or restriction", 3, 5),
		missing("complexContent without derivation before later unbound prefix", inContent("complexContent", ``)+laterUnboundPrefix, contentModel, "complexContent missing extension or restriction", 3, 5),
		missing("top-level group without model group before later unbound prefix", `  <xs:group name="G"><xs:annotation/></xs:group>`+laterUnboundPrefix, contentModel, "top-level group must contain exactly one model group", 2, 3),
		missing("key without selector before later attribute", inIdentity(``)+laterUnknownAttribute, identity, "identity constraint missing selector", 3, 5),
		missing("key without field before later attribute", inIdentity(`<xs:selector xpath="."/>`)+laterUnknownAttribute, identity, "identity constraint missing fields", 3, 5),
		missing("key without selector or field reports selector", inIdentity(`<xs:annotation/>`)+laterUnknownAttribute, identity, "identity constraint missing selector", 3, 5),
		missing("restriction without base before later attribute", inSimpleType(`<xs:restriction><xs:minLength value="1"/></xs:restriction>`)+laterUnknownAttribute, reference, "simple restriction missing base", 3, 5),
		missing("list without item type before later unbound prefix", inSimpleType(`<xs:list/>`)+laterUnboundPrefix, reference, "list missing item type", 3, 5),
		missing("union without members before later unbound prefix", inSimpleType(`<xs:union/>`)+laterUnboundPrefix, reference, "union missing member types", 3, 5),
		missing("union with empty memberTypes before later attribute", inSimpleType(`<xs:union memberTypes=" "/>`)+laterUnknownAttribute, reference, "union missing member types", 3, 5),
		missing("missing derivation of a nested simpleType before its parent's", inSimpleType(`<xs:list><xs:simpleType/></xs:list>`), contentModel, noDerivation, 3, 5+len(`<xs:list>`)),

		missing("earlier sibling attribute before missing derivation", `  <xs:element name="a" foo="1"/>`+"\n"+`  <xs:simpleType name="S"/>`, xsderrors.CodeSchemaInvalidAttribute, `"element" cannot have attribute "foo"`, 2, 3),
		missing("child attribute before missing base", inSimpleType(`<xs:restriction><xs:minLength value="1" foo="1"/></xs:restriction>`), xsderrors.CodeSchemaInvalidAttribute, `"minLength" cannot have attribute "foo"`, 3, 5+len(`<xs:restriction>`)),
		missing("text before missing derivation", `  <xs:simpleType name="S">text</xs:simpleType>`, contentModel, `xs:"simpleType" cannot contain text`, 2, 27),
		missing("text before missing selector", inIdentity(`text`), contentModel, `xs:"key" cannot contain text`, 4, 7),

		{name: "restriction with anonymous base", body: inSimpleType(`<xs:restriction><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:restriction>`)},
		{name: "list with anonymous item type", body: inSimpleType(`<xs:list><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:list>`)},
		{name: "union with anonymous members only", body: inSimpleType(`<xs:union memberTypes=""><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:union>`)},
		{name: "union with named and anonymous members", body: inSimpleType(`<xs:union memberTypes="xs:int"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:union>`)},
		{name: "key with selector and fields", body: inIdentity(`<xs:selector xpath="."/><xs:field xpath="@a"/><xs:field xpath="@b"/>`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compilePlacementAttributeCase(t, tc)
		})
	}
}

func TestTypeSourceConflictReportedAtTheChild(t *testing.T) {
	t.Parallel()
	const laterUnknownAttribute = `<xs:element name="z" foo="1"/>`
	const anonymousSimple = `<xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`
	conflict := func(name, body string, code xsderrors.Code, msg string, line, col int) placementAttributeCase {
		return placementAttributeCase{name: name, body: body, wantCode: code, wantMsg: msg, wantLine: line, wantCol: col}
	}
	const (
		elementConflict   = "element cannot have both type and anonymous type"
		attributeConflict = "attribute cannot have both type and simpleType"
		baseConflict      = "restriction cannot have both base and simpleType"
		itemTypeConflict  = "list cannot have both itemType and simpleType"
	)
	cases := []placementAttributeCase{
		conflict("top-level element type and anonymous simpleType", `  <xs:element name="e" type="xs:string">`+anonymousSimple+laterUnknownAttribute+`</xs:element>`, xsderrors.CodeSchemaInvalidAttribute, elementConflict, 2, 3+len(`<xs:element name="e" type="xs:string">`)),
		conflict("local element type and anonymous complexType", inSequence(`<xs:element name="e" type="xs:string"><xs:annotation/><xs:complexType foo="1"/></xs:element>`), xsderrors.CodeSchemaInvalidAttribute, elementConflict, 4, 7+len(`<xs:element name="e" type="xs:string"><xs:annotation/>`)),
		conflict("element type conflict before nested unbound prefix", `  <xs:element name="e" type="xs:string"><xs:simpleType><xs:restriction base="p:T"/></xs:simpleType></xs:element>`, xsderrors.CodeSchemaInvalidAttribute, elementConflict, 2, 3+len(`<xs:element name="e" type="xs:string">`)),
		conflict("top-level attribute type and simpleType", `  <xs:attribute name="a" type="xs:string">`+anonymousSimple+`</xs:attribute>`+"\n  "+laterUnknownAttribute, xsderrors.CodeSchemaInvalidAttribute, attributeConflict, 2, 3+len(`<xs:attribute name="a" type="xs:string">`)),
		conflict("prohibited local attribute type and simpleType", inComplexType(`<xs:attribute name="a" type="xs:string" use="prohibited"><xs:simpleType foo="1"/></xs:attribute>`), xsderrors.CodeSchemaInvalidAttribute, attributeConflict, 3, 5+len(`<xs:attribute name="a" type="xs:string" use="prohibited">`)),
		conflict("restriction base and simpleType", inSimpleType(`<xs:restriction base="xs:string"><xs:simpleType foo="1"/></xs:restriction>`), xsderrors.CodeSchemaContentModel, baseConflict, 3, 5+len(`<xs:restriction base="xs:string">`)),
		conflict("list itemType and simpleType before nested unbound prefix", inSimpleType(`<xs:list itemType="xs:int"><xs:simpleType><xs:restriction base="p:T"/></xs:simpleType></xs:list>`), xsderrors.CodeSchemaContentModel, itemTypeConflict, 3, 5+len(`<xs:list itemType="xs:int">`)),

		conflict("earlier facet order before restriction conflict", inSimpleType(`<xs:restriction base="xs:string"><xs:minLength value="1"/><xs:simpleType/></xs:restriction>`), xsderrors.CodeSchemaContentModel, "restriction simpleType must precede facets", 3, 5+len(`<xs:restriction base="xs:string"><xs:minLength value="1"/>`)),
		conflict("element anonymous type after identity constraint before conflict", inElement(`<xs:key name="k"><xs:selector xpath="."/><xs:field xpath="."/></xs:key><xs:complexType/>`), xsderrors.CodeSchemaContentModel, "element anonymous type must precede identity constraints", 3, 5+len(`<xs:key name="k"><xs:selector xpath="."/><xs:field xpath="."/></xs:key>`)),
		conflict("earlier sibling attribute before element conflict", `  `+laterUnknownAttribute+"\n"+`  <xs:element name="e" type="xs:string">`+anonymousSimple+`</xs:element>`, xsderrors.CodeSchemaInvalidAttribute, `"element" cannot have attribute "foo"`, 2, 3),
		conflict("earlier unbound type prefix before element conflict", `  <xs:element name="e" type="p:T">`+anonymousSimple+`</xs:element>`, xsderrors.CodeSchemaReference, `unbound QName prefix "p"`, 2, 3),

		{name: "union memberTypes and simpleType", body: inSimpleType(`<xs:union memberTypes="xs:int">` + anonymousSimple + `</xs:union>`)},
		{name: "simpleContent restriction base and simpleType", body: `  <xs:complexType name="B"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="C"><xs:simpleContent><xs:restriction base="B">` + anonymousSimple + `</xs:restriction></xs:simpleContent></xs:complexType>`},
		{name: "element anonymous type without type", body: `  <xs:element name="e">` + anonymousSimple + `</xs:element>`},
		{name: "attribute type without simpleType", body: `  <xs:attribute name="a" type="xs:string"><xs:annotation/></xs:attribute>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compilePlacementAttributeCase(t, tc)
		})
	}
}

func TestElementRequiredAttributeWinsOverLaterErrors(t *testing.T) {
	t.Parallel()
	const laterEmptySimpleType = "\n  <xs:simpleType name=\"S\"/>"
	const laterUnknownAttribute = "\n  <xs:element name=\"z\" foo=\"1\"/>"
	missing := func(name, body string, code xsderrors.Code, msg string, line, col int) placementAttributeCase {
		return placementAttributeCase{name: name, body: body, wantCode: code, wantMsg: msg, wantLine: line, wantCol: col}
	}
	const (
		identity  = xsderrors.CodeSchemaIdentity
		invalid   = xsderrors.CodeSchemaInvalidAttribute
		reference = xsderrors.CodeSchemaReference
	)
	cases := []placementAttributeCase{
		missing("include without schemaLocation before later missing derivation", `  <xs:include/>`+laterEmptySimpleType, reference, "include missing schemaLocation", 2, 3),
		missing("notation without name before later missing derivation", `  <xs:notation public="p"/>`+laterEmptySimpleType, invalid, "notation missing name", 2, 3),
		missing("notation without public or system before later attribute", `  <xs:notation name="N"/>`+laterUnknownAttribute, invalid, "notation requires public or system", 2, 3),
		missing("notation without name reports name before public or system", `  <xs:notation/>`, invalid, "notation missing name", 2, 3),
		missing("facet without value before later missing derivation", inSimpleType(`<xs:restriction base="xs:string"><xs:minLength/></xs:restriction>`)+laterEmptySimpleType, xsderrors.CodeSchemaFacet, "minLength missing value", 3, 5+len(`<xs:restriction base="xs:string">`)),
		missing("key without name before its missing field", "  <xs:element name=\"r\">\n    <xs:key><xs:selector xpath=\".\"/></xs:key>\n  </xs:element>", identity, "identity constraint missing name", 3, 5),
		missing("unique without name before its selector without xpath", "  <xs:element name=\"r\">\n    <xs:unique><xs:selector/><xs:field xpath=\".\"/></xs:unique>\n  </xs:element>", identity, "identity constraint missing name", 3, 5),
		missing("keyref without refer before its missing field", "  <xs:element name=\"r\">\n    <xs:keyref name=\"kr\"><xs:selector xpath=\".\"/></xs:keyref>\n  </xs:element>", identity, "keyref missing refer", 3, 5),
		missing("keyref without name reports name before refer", "  <xs:element name=\"r\">\n    <xs:keyref><xs:selector xpath=\".\"/><xs:field xpath=\".\"/></xs:keyref>\n  </xs:element>", identity, "identity constraint missing name", 3, 5),

		missing("earlier unknown attribute before missing schemaLocation", `  <xs:include foo="1"/>`, invalid, `"include" cannot have attribute "foo"`, 2, 3),
		missing("earlier sibling missing derivation before missing schemaLocation", `  <xs:simpleType name="S"/>`+"\n  <xs:notation name=\"N\"/>", xsderrors.CodeSchemaContentModel, "simpleType must contain one restriction, list, or union", 2, 3),

		{name: "notation with system only", body: `  <xs:notation name="N" system="urn:n"/>`},
		{name: "keyref with name and refer", body: "  <xs:element name=\"r\">\n    <xs:key name=\"k\"><xs:selector xpath=\".\"/><xs:field xpath=\"@a\"/></xs:key>\n    <xs:keyref name=\"kr\" refer=\"k\"><xs:selector xpath=\".\"/><xs:field xpath=\"@a\"/></xs:keyref>\n  </xs:element>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compilePlacementAttributeCase(t, tc)
		})
	}
}
