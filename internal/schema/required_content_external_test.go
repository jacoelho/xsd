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
		missing("text before missing derivation", `  <xs:simpleType name="S">text</xs:simpleType>`, contentModel, `xs:"simpleType" cannot contain text`, 2, 3),
		missing("text before missing selector", inIdentity(`text`), contentModel, `xs:"key" cannot contain text`, 3, 5),

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
