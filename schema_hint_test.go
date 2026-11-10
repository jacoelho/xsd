package xsd_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompiledSchemaLocationHintsDoNotSupplyComponents(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="xs:decimal"/></xs:simpleType>
  <xs:element name="number" type="T"/>
  <xs:element name="strict"><xs:complexType><xs:sequence>
    <xs:any minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence><xs:anyAttribute/></xs:complexType></xs:element>
  <xs:element name="lax"><xs:complexType><xs:sequence>
    <xs:any minOccurs="0" maxOccurs="unbounded" processContents="lax"/>
  </xs:sequence><xs:anyAttribute processContents="lax"/></xs:complexType></xs:element>
  <xs:element name="skip"><xs:complexType><xs:sequence>
    <xs:any minOccurs="0" maxOccurs="unbounded" processContents="skip"/>
  </xs:sequence><xs:anyAttribute processContents="skip"/></xs:complexType></xs:element>
  <xs:element name="errors"><xs:complexType><xs:sequence>
    <xs:element name="n" type="xs:decimal"/><xs:any/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("compiled.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	// An available hinted document must not add components to this engine.
	hinted := filepath.Join(t.TempDir(), "additional.xsd")
	writeTestFile(t, hinted, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="missing" type="xs:string"/></xs:schema>`)
	const namespaces = ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:missing" `
	hints := fmt.Sprintf(`xsi:noNamespaceSchemaLocation="%s" xsi:schemaLocation="urn:missing %s"`, hinted, hinted)
	for _, test := range []struct {
		name, doc string
		codes     []xsderrors.Code
	}{
		{name: "missing root", doc: `<missing %s/>`, codes: []xsderrors.Code{xsderrors.CodeValidationRoot}},
		{name: "missing namespaced root", doc: `<p:missing %s/>`, codes: []xsderrors.Code{xsderrors.CodeValidationRoot}},
		{name: "missing type", doc: `<number %s xsi:type="p:Missing">1</number>`, codes: []xsderrors.Code{xsderrors.CodeValidationType}},
		{name: "strict missing child", doc: `<strict %s><missing/></strict>`, codes: []xsderrors.Code{xsderrors.CodeValidationElement}},
		{name: "strict missing attribute", doc: `<strict %s extra="1"/>`, codes: []xsderrors.Code{xsderrors.CodeValidationAttribute}},
		{name: "known valid value", doc: `<number %s>1</number>`},
		{name: "known invalid value", doc: `<number %s>bad</number>`, codes: []xsderrors.Code{xsderrors.CodeValidationFacet}},
		{name: "provided root type", doc: `<missing %s xsi:type="T">1</missing>`},
		{name: "provided strict child type", doc: `<strict %s><missing xsi:type="T">1</missing></strict>`},
		{name: "provided strict child invalid value", doc: `<strict %s><missing xsi:type="T">bad</missing></strict>`, codes: []xsderrors.Code{xsderrors.CodeValidationFacet}},
		{name: "missing strict child type", doc: `<strict %s><missing xsi:type="p:Missing"/></strict>`, codes: []xsderrors.Code{xsderrors.CodeValidationType}},
		{name: "lax missing components", doc: `<lax %s extra="1"><missing/></lax>`},
		{name: "skip missing components", doc: `<skip %s extra="1"><missing/></skip>`},
		{name: "successive semantic errors", doc: `<errors %s><n>bad</n><missing/></errors>`, codes: []xsderrors.Code{xsderrors.CodeValidationFacet, xsderrors.CodeValidationElement}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, maxErrors := range []int{0, 1, 2} {
				session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: maxErrors})
				if err != nil {
					t.Fatal(err)
				}
				want := test.codes
				if maxErrors > 0 && len(want) > maxErrors {
					want = want[:maxErrors]
				}
				for _, attributes := range []string{"", hints} {
					doc := fmt.Sprintf(test.doc, namespaces+attributes)
					err := session.Validate(strings.NewReader(doc))
					if len(want) == 0 {
						if err != nil {
							t.Fatalf("Validate(maxErrors=%d, hints=%t): %v", maxErrors, attributes != "", err)
						}
					} else {
						if xsderrors.IsUnsupported(err) || !xsderrors.IsConclusiveValidation(err) {
							t.Fatalf("Validate(maxErrors=%d, hints=%t) = %v, want conclusive validation errors", maxErrors, attributes != "", err)
						}
						var codes []xsderrors.Code
						for _, leaf := range xsderrors.Flatten(err) {
							diagnostic, ok := errors.AsType[*xsderrors.Error](leaf)
							if !ok || diagnostic.Category() != xsderrors.CategoryValidation {
								t.Fatalf("unexpected diagnostic %T: %v", leaf, leaf)
							}
							codes = append(codes, diagnostic.Code())
						}
						if !slices.Equal(codes, want) {
							t.Fatalf("Validate(maxErrors=%d, hints=%t) codes=%v, want %v", maxErrors, attributes != "", codes, want)
						}
					}
					if err := session.Validate(strings.NewReader(`<number>2</number>`)); err != nil {
						t.Fatalf("Validate after prior assessment: %v", err)
					}
				}
			}
		})
	}
}

func TestSchemaLocationHintOverTokenLimitIsALimitError(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceTokenBytes: 64})
	if err != nil {
		t.Fatal(err)
	}
	doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:a ` + strings.Repeat("a", 128) + `.xsd"/>`
	err = session.Validate(strings.NewReader(doc))
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Code() != xsderrors.CodeValidationLimit || !strings.Contains(err.Error(), "MaxInstanceTokenBytes") {
		t.Fatalf("Validate() = %v, want validation.limit naming MaxInstanceTokenBytes", err)
	}
	if err := session.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("clean document after limit: %v", err)
	}
}

const hintAssessmentSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="skipRoot"><xs:complexType><xs:sequence>
    <xs:any namespace="##other" processContents="skip" minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
  <xs:element name="laxRoot"><xs:complexType><xs:sequence>
    <xs:any namespace="##other" processContents="lax" minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
  <xs:element name="attrRoot"><xs:complexType><xs:anyAttribute processContents="lax"/></xs:complexType></xs:element>
  <xs:element name="simple" type="xs:string"/>
  <xs:element name="plain"><xs:complexType/></xs:element>
</xs:schema>`

const hintXSI = ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`

func hintEngine(t *testing.T) *xsd.Engine {
	t.Helper()
	engine, err := xsd.Compile(xsd.Bytes("hints.xsd", []byte(hintAssessmentSchema)))
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestSchemaLocationHintItemCountIsNotConstrained(t *testing.T) {
	engine := hintEngine(t)
	for _, value := range []string{"urn:t", "", "urn:a a.xsd urn:b", "urn:a\ta.xsd\nurn:b"} {
		if err := engine.Validate(strings.NewReader(`<plain` + hintXSI + ` xsi:schemaLocation="` + value + `"/>`)); err != nil {
			t.Errorf("xsi:schemaLocation=%q: %v", value, err)
		}
	}
}

func TestSchemaLocationHintsOnSkippedContentAreNotAssessed(t *testing.T) {
	engine := hintEngine(t)
	doc := `<skipRoot` + hintXSI + ` xmlns:o="urn:o"><o:x xmlns:o="urn:o" xsi:schemaLocation="%zz" xsi:noNamespaceSchemaLocation="%zz" xsi:nil="zz" xsi:type="nope"><o:y xsi:schemaLocation="%zz"/></o:x></skipRoot>`
	if err := engine.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("Validate() = %v, want nil: skip-processed content is only checked for well-formedness", err)
	}
}

func TestSchemaLocationHintItemIsDiagnosedOnAssessedElement(t *testing.T) {
	engine := hintEngine(t)
	for _, value := range []string{"%zz urn:a", "urn:a %zz urn:b", "urn:a %zz"} {
		t.Run(value, func(t *testing.T) {
			err := engine.Validate(strings.NewReader("<plain" + hintXSI + " xsi:schemaLocation=\"" + value + "\"/>"))
			diagnostics := pathDiagnostics(t, err)
			if len(diagnostics) != 1 {
				t.Fatalf("diagnostics = %v, want exactly one", err)
			}
			d := diagnostics[0]
			if d.Code() != xsderrors.CodeValidationAttribute || !xsderrors.IsConclusiveValidation(err) ||
				d.Path() != "/plain" || d.Line() < 1 ||
				!strings.Contains(d.Message(), "%zz") || strings.Contains(d.Message(), "urn:a") {
				t.Fatalf("diagnostic = %s %s line %d %q (conclusive %t)", d.Code(), d.Path(), d.Line(), d.Message(), xsderrors.IsConclusiveValidation(err))
			}
		})
	}
	t.Run("simple typed element", func(t *testing.T) {
		err := engine.Validate(strings.NewReader("<simple" + hintXSI + " xsi:schemaLocation=\"%zz\">x</simple>"))
		diagnostics := pathDiagnostics(t, err)
		if len(diagnostics) != 1 || diagnostics[0].Code() != xsderrors.CodeValidationAttribute {
			t.Fatalf("diagnostics = %v, want one validation.attribute", err)
		}
	})
	t.Run("both hint attributes", func(t *testing.T) {
		err := engine.Validate(strings.NewReader("<plain" + hintXSI + " xsi:schemaLocation=\"%zz\" xsi:noNamespaceSchemaLocation=\"%zz\"/>"))
		diagnostics := pathDiagnostics(t, err)
		if len(diagnostics) != 2 {
			t.Fatalf("diagnostics = %v, want two", err)
		}
		for _, d := range diagnostics {
			if d.Code() != xsderrors.CodeValidationAttribute {
				t.Fatalf("code = %s, want validation.attribute", d.Code())
			}
		}
	})
}

func TestNonXSISchemaLocationAttributeIsNotAHint(t *testing.T) {
	engine := hintEngine(t)
	if err := engine.Validate(strings.NewReader(`<attrRoot schemaLocation="%zz"/>`)); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

// Diagnostic order within one element start is not contractual, so the
// same-element rows compare (code, path) as a multiset.
func TestLaxUnknownElementPathsAreExpandedBeforeAndAfterCommit(t *testing.T) {
	engine := hintEngine(t)
	t.Run("nested paths", func(t *testing.T) {
		doc := `<laxRoot` + hintXSI + `><o:x xmlns:o="urn:o" xsi:type="nope"><o:y xsi:nil="zz"/></o:x></laxRoot>`
		diagnostics := pathDiagnostics(t, engine.Validate(strings.NewReader(doc)))
		var x, y bool
		for _, d := range diagnostics {
			switch {
			case d.Path() == "/laxRoot/{urn:o}x":
				x = true
			case strings.HasPrefix(d.Path(), "/laxRoot/{urn:o}x/"):
				y = true
			}
		}
		if !x || !y {
			t.Fatalf("diagnostics = %v, want one at /laxRoot/{urn:o}x and one below it", diagnostics)
		}
	})
	t.Run("same element before and after commit", func(t *testing.T) {
		doc := `<laxRoot` + hintXSI + `><o:x xmlns:o="urn:o" xsi:type="nope" xsi:schemaLocation="%zz"/></laxRoot>`
		var got []string
		for _, d := range pathDiagnostics(t, engine.Validate(strings.NewReader(doc))) {
			got = append(got, string(d.Code())+" "+d.Path())
		}
		slices.Sort(got)
		want := []string{"validation.attribute /laxRoot/{urn:o}x", "validation.type /laxRoot/{urn:o}x"}
		if !slices.Equal(got, want) {
			t.Fatalf("diagnostics = %q, want %q", got, want)
		}
	})
}
