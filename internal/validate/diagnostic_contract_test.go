package validate

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestContentDiagnosticsDescribeCurrentState(t *testing.T) {
	tests := []struct {
		name, model, body string
		contains, absent  []string
	}{
		{name: "sequence at start", model: `<xs:sequence><xs:element name="first"/><xs:element name="second"/></xs:sequence>`, body: `<wrong/>`, contains: []string{`unexpected child element "wrong"`, `expected next child ["first"]`}, absent: []string{`"second"`}},
		{name: "sequence after first", model: `<xs:sequence><xs:element name="first"/><xs:element name="second"/></xs:sequence>`, body: `<first/><wrong/>`, contains: []string{`expected next child ["second"]`}, absent: []string{`"first"`}},
		{name: "choice", model: `<xs:choice><xs:element name="left"/><xs:element name="right"/></xs:choice>`, body: `<wrong/>`, contains: []string{`"left"`, `"right"`}},
		{name: "counted minimum", model: `<xs:sequence><xs:element name="item" minOccurs="2" maxOccurs="3"/><xs:element name="last"/></xs:sequence>`, body: `<item/><wrong/>`, contains: []string{`expected next child ["item"]`}, absent: []string{`"last"`}},
		{name: "counted maximum", model: `<xs:sequence><xs:element name="item" minOccurs="1" maxOccurs="2"/><xs:element name="last"/></xs:sequence>`, body: `<item/><item/><wrong/>`, contains: []string{`expected next child ["last"]`}, absent: []string{`"item"`}},
		{name: "all unseen", model: `<xs:all><xs:element name="first"/><xs:element name="second"/><xs:element name="optional" minOccurs="0"/></xs:all>`, body: `<first/><wrong/>`, contains: []string{`"second"`, `"optional"`}, absent: []string{`"first"`}},
		{name: "all missing required", model: `<xs:all><xs:element name="first"/><xs:element name="second"/><xs:element name="optional" minOccurs="0"/></xs:all>`, body: `<first/>`, contains: []string{`content is incomplete`, `expected next child ["second"]`}, absent: []string{`"optional"`, `"first"`}},
		{name: "wildcard namespace", model: `<xs:sequence><xs:any namespace="urn:allowed" processContents="skip"/></xs:sequence>`, body: `<wrong/>`, contains: []string{`an element in namespace ["urn:allowed"]`}},
		{name: "wildcard other", model: `<xs:sequence><xs:any namespace="##other" processContents="skip"/></xs:sequence>`, body: `<wrong/>`, contains: []string{`an element in a non-empty namespace other than ""`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`+test.model+`</xs:complexType></xs:element></xs:schema>`)
			session, err := newSessionForTest(rt, Options{MaxErrors: 1})
			if err != nil {
				t.Fatal(err)
			}
			err = session.Validate(strings.NewReader(`<root>` + test.body + `</root>`))
			if err == nil {
				t.Fatal("invalid content accepted")
			}
			message := err.Error()
			for _, fact := range test.contains {
				if !strings.Contains(message, fact) {
					t.Errorf("message %q lacks %q", message, fact)
				}
			}
			for _, fact := range test.absent {
				if strings.Contains(message, fact) {
					t.Errorf("message %q includes stale alternative %q", message, fact)
				}
			}
			if err = session.Validate(strings.NewReader(`<root>` + validContentForDiagnosticTest(test.name) + `</root>`)); err != nil {
				t.Fatalf("session reuse failed: %v", err)
			}
		})
	}
}
func validContentForDiagnosticTest(name string) string {
	switch name {
	case "choice":
		return `<left/>`
	case "counted minimum":
		return `<item/><item/><last/>`
	case "counted maximum":
		return `<item/><last/>`
	case "wildcard namespace", "wildcard other":
		return `<allowed xmlns="urn:allowed"/>`
	default:
		return `<first/><second/>`
	}
}

func TestRuntimeDiagnosticValuesAndDeclarationFacts(t *testing.T) {
	tests := []struct {
		name, schema, doc string
		code              xsderrors.Code
		facts             []string
	}{
		{"fixed attribute", `<xs:element name="root"><xs:complexType><xs:attribute name="answer" type="xs:int" fixed="42"/></xs:complexType></xs:element>`, `<root answer="0041"/>`, xsderrors.CodeValidationAttribute, []string{`attribute "answer"`, `value "0041"`, `fixed value "42"`}},
		{"fixed element", `<xs:element name="root" type="xs:int" fixed="42"/>`, `<root>41</root>`, xsderrors.CodeValidationElement, []string{`element value "41"`, `fixed value "42"`}},
		{"invalid nil", `<xs:element name="root" nillable="true"/>`, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="bogus"/>`, xsderrors.CodeValidationNil, []string{`xsi:nil`, `"bogus"`, `"true"`, `"false"`, `"1"`, `"0"`}},
		{"empty text", `<xs:element name="root"><xs:complexType/></xs:element>`, `<root> </root>`, xsderrors.CodeValidationText, []string{`content type is empty`, `including whitespace`}},
		{"element only text", `<xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>`, `<root>bad</root>`, xsderrors.CodeValidationText, []string{`non-whitespace`, `element-only`}},
		{"strict attribute", `<xs:element name="root"><xs:complexType><xs:anyAttribute processContents="strict"/></xs:complexType></xs:element>`, `<root unknown="x"/>`, xsderrors.CodeValidationAttribute, []string{`attribute "unknown"`, `strict wildcard`, `no global attribute declaration`}},
		{"unknown QName prefix", `<xs:element name="root"/>`, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="missing:Type"/>`, xsderrors.CodeValidationType, []string{`"missing:Type"`, `undeclared namespace prefix`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.schema+`</xs:schema>`)
			session, err := newSessionForTest(rt, Options{MaxErrors: 1})
			if err != nil {
				t.Fatal(err)
			}
			err = session.Validate(strings.NewReader(test.doc))
			if err == nil {
				t.Fatal("invalid document accepted")
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("diagnostic type %T", err)
			}
			if diagnostic.Code() != test.code {
				t.Fatalf("code %s, want %s", diagnostic.Code(), test.code)
			}
			for _, fact := range test.facts {
				if !strings.Contains(diagnostic.Message(), fact) {
					t.Errorf("message %q lacks %q", diagnostic.Message(), fact)
				}
			}
			path, column := "/root", 1
			switch test.name {
			case "fixed element":
				column = 9
			case "empty text", "element only text":
				column = 7
			case "invalid nil", "unknown QName prefix":
				path = "/"
			}
			if diagnostic.Path() != path || diagnostic.Line() != 1 || diagnostic.Column() != column {
				t.Errorf("lost diagnostic location: %v", diagnostic)
			}
		})
	}
}

func TestContentDiagnosticsPreserveNamespacesAndSubstitutions(t *testing.T) {
	tests := []struct {
		name, schema, document string
		facts, absent          []string
	}{
		{name: "namespace-distinct alternatives", schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:items"><xs:element name="root"><xs:complexType><xs:choice><xs:element name="item" form="qualified"/><xs:element name="item" form="unqualified"/></xs:choice></xs:complexType></xs:element></xs:schema>`, document: `<t:root xmlns:t="urn:items"><wrong/></t:root>`, facts: []string{`"{urn:items}item"`, `"item"`}},
		{name: "abstract substitution head", schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="head" abstract="true"/><xs:element name="member" substitutionGroup="head"/><xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="head"/></xs:sequence></xs:complexType></xs:element></xs:schema>`, document: `<root><wrong/></root>`, facts: []string{`expected next child ["member"]`}, absent: []string{`"head"`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := compileRuntimeForTest(t, test.schema)
			session, err := newSessionForTest(rt, Options{MaxErrors: 1})
			if err != nil {
				t.Fatal(err)
			}
			err = session.Validate(strings.NewReader(test.document))
			if err == nil {
				t.Fatal("unexpected child accepted")
			}
			for _, fact := range test.facts {
				if !strings.Contains(err.Error(), fact) {
					t.Errorf("diagnostic %q lacks %q", err, fact)
				}
			}
			for _, fact := range test.absent {
				if strings.Contains(err.Error(), fact) {
					t.Errorf("diagnostic %q includes abstract name %q", err, fact)
				}
			}
		})
	}
}

func TestRuntimeDiagnosticValuesAreEscapedBoundedAndOwned(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:attribute name="answer" fixed="good"/></xs:complexType></xs:element></xs:schema>`)
	session, err := newSessionForTest(rt, Options{MaxErrors: 1})
	if err != nil {
		t.Fatal(err)
	}
	err = session.Validate(strings.NewReader(`<root answer="&quot;&#10;` + strings.Repeat("é", 1000) + `"/>`))
	if err == nil {
		t.Fatal("fixed mismatch accepted")
	}
	message := err.Error()
	if !strings.Contains(message, `\"\n`) || !strings.Contains(message, `"...`) {
		t.Fatalf("missing escaped value or truncation marker: %q", message)
	}
	if len(message) > 700 {
		t.Fatalf("unbounded value diagnostic: %d bytes", len(message))
	}
	if next := session.Validate(strings.NewReader(`<root answer="good"/>`)); next != nil {
		t.Fatalf("session reuse failed: %v", next)
	}
	if err.Error() != message {
		t.Fatal("diagnostic changed after session reuse")
	}
}

func TestRuntimeLimitsNameConfiguredOptionAndObservedSize(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`)
	session, err := newSessionForTest(rt, Options{MaxInstanceTextBytes: 2})
	if err != nil {
		t.Fatal(err)
	}
	err = session.Validate(strings.NewReader(`<root>abc</root>`))
	if err == nil {
		t.Fatal("oversized text accepted")
	}
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	if !strings.Contains(err.Error(), `3 exceeds MaxInstanceTextBytes=2`) {
		t.Fatalf("limit diagnostic lacks observed/configured sizes: %v", err)
	}
	_, err = NormalizeOptions(Options{MaxErrors: -7})
	if err == nil || !strings.Contains(err.Error(), `MaxErrors must be non-negative; got -7`) {
		t.Fatalf("option diagnostic lacks rejected value: %v", err)
	}
}
