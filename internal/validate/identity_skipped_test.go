package validate

import (
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentitySkippedSelectorsContributeNoRows(t *testing.T) {
	t.Parallel()
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:element name="known"><xs:complexType><xs:attribute name="id" type="xs:ID"/></xs:complexType></xs:element>
<xs:element name="root"><xs:complexType><xs:sequence><xs:any minOccurs="0" maxOccurs="unbounded" processContents="skip"/></xs:sequence></xs:complexType>
<xs:key name="k"><xs:selector xpath=".//*"/><xs:field xpath="@id"/></xs:key>
</xs:element></xs:schema>`)
	for _, doc := range []string{
		`<root><unknown/></root>`,
		`<root><known id="same"/><known id="same"/></root>`,
		`<root><unknown><known id="same"/><known id="same"/></unknown></root>`,
	} {
		session, err := newSessionForTest(rt, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			t.Fatalf("Validate(%s) error = %v", doc, err)
		}
	}
}

func TestIdentitySkippedFieldsDoNotQualifyOrCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		kind    string
		field   string
		body    string
		message string
	}{
		{name: "only skipped elements", kind: "unique", field: "*", body: `<row><s:a/><s:b/></row>`},
		{name: "skipped then typed element", kind: "key", field: "*", body: `<row><s:a/><typed>x</typed></row>`},
		{name: "typed then skipped elements", kind: "key", field: "*", body: `<row><typed>x</typed><s:a/><s:b/></row>`},
		{name: "two typed elements", kind: "unique", field: "*", body: `<row><typed>x</typed><s:a/><typed>y</typed></row>`, message: "identity constraint \"values\", field 1: selects multiple nodes; at most one is allowed"},
		{name: "key skipped element missing", kind: "key", field: "*", body: `<row><s:a/></row>`, message: "identity constraint \"values\", field 1: a key requires a value, but the selected node is absent or nilled"},
		{name: "keyref skipped element omitted", kind: "keyref", field: "*", body: `<row><s:a/></row>`},
		{name: "only skipped attributes", kind: "unique", field: "@*", body: `<row s:a="x" s:b="y"/>`},
		{name: "skipped and typed attributes", kind: "key", field: "@*", body: `<row s:a="x" typed="y" s:b="z"/>`},
		{name: "two typed attributes", kind: "unique", field: "@*", body: `<row typed="x" s:a="y" other="z"/>`, message: "identity constraint \"values\", field 1: selects multiple nodes; at most one is allowed"},
		{name: "key skipped attribute missing", kind: "key", field: "@*", body: `<row s:a="x"/>`, message: "identity constraint \"values\", field 1: a key requires a value, but the selected node is absent or nilled"},
		{name: "keyref skipped attribute omitted", kind: "keyref", field: "@*", body: `<row s:a="x"/>`},
		{name: "skipped child xsi attribute missing", kind: "key", field: "s:child/@xsi:nil", body: `<row><s:child xsi:nil="false"/></row>`, message: "identity constraint \"values\", field 1: a key requires a value, but the selected node is absent or nilled"},
		{name: "assessed xsi attribute remains typed", kind: "key", field: "@xsi:nil", body: `<row xsi:nil="false"/>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			refer := ""
			if test.kind == "keyref" {
				refer = ` refer="targets"`
			}
			rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:s="urn:skip" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<xs:element name="root"><xs:complexType><xs:sequence>
<xs:element name="target" minOccurs="0" maxOccurs="unbounded"><xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType></xs:element>
<xs:element name="row" nillable="true" minOccurs="0" maxOccurs="unbounded"><xs:complexType>
<xs:choice minOccurs="0" maxOccurs="unbounded"><xs:element name="typed" type="xs:string"/><xs:any namespace="urn:skip" processContents="skip"/></xs:choice>
<xs:attribute name="typed" type="xs:string"/><xs:attribute name="other" type="xs:string"/><xs:anyAttribute namespace="urn:skip" processContents="skip"/>
</xs:complexType></xs:element></xs:sequence></xs:complexType>
<xs:key name="targets"><xs:selector xpath="target"/><xs:field xpath="@id"/></xs:key>
<xs:`+test.kind+` name="values"`+refer+`><xs:selector xpath="row"/><xs:field xpath="`+test.field+`"/></xs:`+test.kind+`>
</xs:element></xs:schema>`)
			session, err := newSessionForTest(rt, Options{})
			if err != nil {
				t.Fatal(err)
			}
			err = session.Validate(strings.NewReader(`<root xmlns:s="urn:skip" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` + test.body + `</root>`))
			if test.message == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
			expectXSDMessage(t, err, test.message)
		})
	}
}

func TestIdentitySkippedOwnerHasNoAttributeTargets(t *testing.T) {
	t.Parallel()
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<xs:element name="root"><xs:complexType><xs:sequence><xs:any processContents="skip"/></xs:sequence></xs:complexType>
<xs:key name="k"><xs:selector xpath="."/><xs:field xpath="child/@xsi:nil"/></xs:key>
</xs:element></xs:schema>`)
	evaluation := newIdentityEvaluation(rt, identityLimits{}, 0)
	name, _ := rt.LookupQName("", "root")
	runtimeName := xsdSchema.RuntimeName{Known: true, Name: name}
	id, _, ok := rt.RootElement(runtimeName)
	if !ok {
		t.Fatal("compiled schema is missing its root element")
	}
	if err := evaluation.startElement(identityElementStart{Name: runtimeName, Element: id, Mode: elementAssessed}); err != nil {
		t.Fatal(err)
	}
	childName, ok := rt.LookupQName("", "child")
	if !ok {
		t.Fatal("compiled field is missing its child name")
	}
	if err := evaluation.startElement(identityElementStart{Name: xsdSchema.RuntimeName{Known: true, Name: childName}, Element: xsdSchema.NoElement, Mode: elementWildcardSkipped}); err != nil {
		t.Fatal(err)
	}
	attributeName, ok := rt.LookupQName("http://www.w3.org/2001/XMLSchema-instance", "nil")
	if !ok {
		t.Fatal("compiled field is missing its XSI attribute name")
	}
	target, err := evaluation.prepareAttributeValue(xsdSchema.RuntimeName{Known: true, Name: attributeName}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if target.needsIdentity() {
		t.Fatal("skipped owner's attribute prepared an identity field target")
	}
}
