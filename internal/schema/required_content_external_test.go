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
