package xsd_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestXSIStartDiagnosticsIdentifyElement(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child" nillable="true" minOccurs="0"/><xs:element name="required" minOccurs="0"/><xs:any namespace="##other" processContents="lax" minOccurs="0" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("start.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, child, path string
		code              xsderrors.Code
	}{
		{"invalid nil", `<child xsi:nil="bad"/>`, "/root/child", xsderrors.CodeValidationNil},
		{"empty nil", `<child xsi:nil=""/>`, "/root/child", xsderrors.CodeValidationNil},
		{"empty type", `<child xsi:type=""/>`, "/root/child", xsderrors.CodeValidationType},
		{"not nillable", `<required xsi:nil="true"/>`, "/root/required", xsderrors.CodeValidationNil},
		{"wildcard type", `<o:unknown xsi:type="xs:missing"/>`, "/root/{urn:o}unknown", xsderrors.CodeValidationType},
		{"wildcard nil", `<o:unknown xsi:nil="bad"/>`, "/root/{urn:o}unknown", xsderrors.CodeValidationNil},
		{"child prefix shadow", `<o:unknown xmlns:o="urn:child" xsi:type="xs:missing"/>`, "/root/{urn:child}unknown", xsderrors.CodeValidationType},
		{"child default namespace", `<unknown xmlns="urn:child" xsi:nil="bad"/>`, "/root/{urn:child}unknown", xsderrors.CodeValidationNil},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:o="urn:o">` + "\n  " + test.child + "\n</root>"
			assertStartDiagnostics(t, engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxErrors: 1}), []startDiagnostic{{test.code, test.path, 2, 3}})
		})
	}
}

func TestXSIStartDiagnosticsPreserveNilBeforeType(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child" type="xs:string" nillable="true"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("start.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, attrs := range []string{
		`xsi:nil="bad" xsi:type="p:Missing"`,
		`xsi:type="p:Missing" xsi:nil="bad"`,
	} {
		t.Run(attrs, func(t *testing.T) {
			doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:missing">` + "\n  <child " + attrs + "/>\n</root>"
			want := []startDiagnostic{
				{xsderrors.CodeValidationNil, "/root/child", 2, 3},
				{xsderrors.CodeValidationType, "/root/child", 2, 3},
			}
			for _, maxErrors := range []int{1, 2} {
				assertStartDiagnostics(t, engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxErrors: maxErrors}), want[:maxErrors])
			}
		})
	}
}

func TestDTDDiagnosticReportedOnce(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("test.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, compileErr := xsd.Compile(xsd.Bytes("dtd.xsd", []byte("\n<!DOCTYPE xs:schema>"+schema)))
	for _, test := range []struct {
		name         string
		err          error
		path         string
		line, column int
	}{
		{"schema", compileErr, "dtd.xsd", 0, 0},
		{"instance", session.Validate(strings.NewReader("\n<!DOCTYPE root><root/>")), "/", 2, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			leaves := xsderrors.Flatten(test.err)
			if len(leaves) != 1 {
				t.Fatalf("diagnostics = %v, want one", leaves)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](leaves[0])
			if !ok {
				t.Fatalf("diagnostic = %T, want structured error", leaves[0])
			}
			if diagnostic.Category() != xsderrors.CategoryUnsupported || diagnostic.Code() != xsderrors.CodeUnsupportedDTD {
				t.Fatalf("unexpected diagnostic: %v", diagnostic)
			}
			if diagnostic.Path() != test.path || diagnostic.Line() != test.line || diagnostic.Column() != test.column {
				t.Errorf("location = %s %d:%d, want %s %d:%d", diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), test.path, test.line, test.column)
			}
			if count := strings.Count(test.err.Error(), "DTD declarations are not supported"); count != 1 {
				t.Errorf("DTD message appears %d times: %v", count, test.err)
			}
		})
	}
	if err := session.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("session reuse: %v", err)
	}
}

type diagnosticLocation struct {
	code         xsderrors.Code
	path         string
	line, column int
}

func diagnosticLocations(t *testing.T, err error) []diagnosticLocation {
	t.Helper()
	var got []diagnosticLocation
	for _, leaf := range xsderrors.Flatten(err) {
		diagnostic, ok := errors.AsType[*xsderrors.Error](leaf)
		if !ok {
			t.Fatalf("diagnostic %T %v is not structured", leaf, leaf)
		}
		got = append(got, diagnosticLocation{diagnostic.Code(), diagnostic.Path(), diagnostic.Line(), diagnostic.Column()})
	}
	return got
}

func requireLocations(t *testing.T, err error, want ...diagnosticLocation) {
	t.Helper()
	got := diagnosticLocations(t, err)
	if len(got) != len(want) {
		t.Fatalf("diagnostics = %+v, want %+v (error %v)", got, want, err)
	}
	for i := range want {
		// Columns are asserted only where the case names one.
		if want[i].column == 0 {
			got[i].column = 0
		}
		if got[i] != want[i] {
			t.Fatalf("diagnostic %d = %+v, want %+v (error %v)", i, got[i], want[i], err)
		}
	}
}

func compileLocationSchema(t *testing.T, schema string) *xsd.Engine {
	t.Helper()
	engine, err := xsd.Compile(xsd.Bytes("location.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestUnsupportedValueDiagnosticsAreLocated(t *testing.T) {
	engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="e" type="xs:ENTITY" minOccurs="0"/>
  </xs:sequence><xs:attribute name="a" type="xs:ENTITY"/></xs:complexType></xs:element>
</xs:schema>`)
	t.Run("attribute", func(t *testing.T) {
		err := engine.Validate(strings.NewReader("<!-- one -->\n<!-- two -->\n<root a=\"x\"/>"))
		requireLocations(t, err, diagnosticLocation{xsderrors.CodeUnsupportedEntity, "/root", 3, 0})
	})
	t.Run("element content", func(t *testing.T) {
		err := engine.Validate(strings.NewReader("<root>\n<e>x</e>\n</root>"))
		requireLocations(t, err, diagnosticLocation{xsderrors.CodeUnsupportedEntity, "/root/e", 2, 0})
	})
}

func TestSchemaLocationHintDiagnosticsAreLocatedOnTheElement(t *testing.T) {
	engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r"><xs:complexType><xs:sequence>
    <xs:element name="a"><xs:complexType><xs:sequence>
      <xs:element name="c" nillable="true" minOccurs="0"/>
    </xs:sequence></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	const xsi = `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`
	for _, test := range []struct {
		name, doc string
		want      diagnosticLocation
	}{
		{"invalid schemaLocation item", "<r " + xsi + ">\n<a>\n<c xsi:schemaLocation=\"urn:x %zz\"/>\n</a>\n</r>", diagnosticLocation{xsderrors.CodeValidationAttribute, "/r/a/c", 3, 0}},
		{"invalid noNamespaceSchemaLocation", "<r " + xsi + ">\n<a>\n<c xsi:noNamespaceSchemaLocation=\"%zz\"/>\n</a>\n</r>", diagnosticLocation{xsderrors.CodeValidationAttribute, "/r/a/c", 3, 0}},
		{"root", "<r " + xsi + " xsi:schemaLocation=\"urn:x %zz\">\n<a/>\n</r>", diagnosticLocation{xsderrors.CodeValidationAttribute, "/r", 1, 0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireLocations(t, engine.Validate(strings.NewReader(test.doc)), test.want)
		})
	}
	t.Run("same element as xsi:nil", func(t *testing.T) {
		// Order within one element start is not contractual: compare as a multiset.
		doc := "<r " + xsi + ">\n<a>\n<c xsi:schemaLocation=\"%zz\" xsi:nil=\"bad\"/>\n</a>\n</r>"
		got := diagnosticLocations(t, engine.Validate(strings.NewReader(doc)))
		slices.SortFunc(got, func(a, b diagnosticLocation) int { return strings.Compare(string(a.code), string(b.code)) })
		want := []diagnosticLocation{
			{xsderrors.CodeValidationAttribute, "/r/a/c", 3, 0},
			{xsderrors.CodeValidationNil, "/r/a/c", 3, 0},
		}
		if len(got) != 2 {
			t.Fatalf("diagnostics = %+v, want %+v", got, want)
		}
		for i := range want {
			got[i].column = 0
			if got[i] != want[i] {
				t.Fatalf("diagnostics = %+v, want %+v", got, want)
			}
		}
	})
}

func TestDocumentEndDiagnosticsAreLocated(t *testing.T) {
	engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:any processContents="lax" minOccurs="0"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)
	err := engine.Validate(strings.NewReader("<root>\n<a>\n"))
	requireLocations(t, err, diagnosticLocation{xsderrors.CodeValidationXML, "/root/a", 3, 0})
	if d, _ := errors.AsType[*xsderrors.Error](err); !strings.Contains(d.Message(), "unclosed element") {
		t.Fatalf("message = %q, want unclosed element", d.Message())
	}
	err = engine.Validate(strings.NewReader(""))
	got := diagnosticLocations(t, err)
	if len(got) != 1 || got[0].code != xsderrors.CodeValidationXML || got[0].path == "" || got[0].line != 1 {
		t.Fatalf("empty document diagnostics = %+v, want validation.xml with a path and line 1", got)
	}
}

func TestUnexpectedChildStaysLocatedOnTheParent(t *testing.T) {
	engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="a" minOccurs="0"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)
	err := engine.Validate(strings.NewReader("<root>\n<b/>\n</root>"))
	requireLocations(t, err, diagnosticLocation{xsderrors.CodeValidationElement, "/root", 2, 0})
}

func TestAttributePatternLimitIsLocatedOnTheStartTag(t *testing.T) {
	engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element name="e" minOccurs="0"><xs:complexType><xs:attribute name="a"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="[a-z]+"/></xs:restriction></xs:simpleType></xs:attribute></xs:complexType></xs:element></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	doc := "<root>\n<e\n a=\"" + strings.Repeat("a", 1_000) + "\"/>\n</root>"
	err := engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxInstancePatternWork: 10})
	got := diagnosticLocations(t, err)
	if len(got) != 1 || got[0].code != xsderrors.CodeValidationLimit || got[0].path != "/root/e" || got[0].line != 2 {
		t.Fatalf("diagnostics = %+v, want validation.limit on /root/e line 2", got)
	}
}

func TestIdentityDiagnosticsKeepTheirStoredLocation(t *testing.T) {
	engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element name="e" maxOccurs="unbounded"><xs:complexType><xs:attribute name="k" type="xs:string"/></xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:key name="keys"><xs:selector xpath="e"/><xs:field xpath="@k"/></xs:key>
  </xs:element>
</xs:schema>`)
	err := engine.Validate(strings.NewReader("<root>\n<e k=\"1\"/>\n<e k=\"1\"/>\n</root>"))
	requireLocations(t, err, diagnosticLocation{xsderrors.CodeValidationIdentity, "/root/e", 3, 0})
}

func TestPrologueFailureIsLocatedAtTheDocumentStart(t *testing.T) {
	engine := compileLocationSchema(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	got := diagnosticLocations(t, engine.Validate(strings.NewReader("\xff\xfe")))
	if len(got) != 1 || got[0].code != xsderrors.CodeUnsupportedNonUTF8 || got[0].path == "" || got[0].line != 1 {
		t.Fatalf("diagnostics = %+v, want unsupported.non_utf8 with a path and line 1", got)
	}
}
