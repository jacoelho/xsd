package schema_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaXMLNamespaceWellFormedness(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{
			name:   "duplicate literal attribute",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="a" name="b"/></xs:schema>`,
		},
		{
			name:   "duplicate expanded attribute",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:a="urn:e" xmlns:b="urn:e"><xs:annotation><xs:appinfo><p a:x="1" b:x="2"/></xs:appinfo></xs:annotation></xs:schema>`,
		},
		{
			name:   "unbound attribute prefix",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="a" q:flag="x"/></xs:schema>`,
		},
		{
			name:   "reserved xml binding",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xml="urn:wrong"/>`,
		},
		{
			name:   "unbound opaque element prefix",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:appinfo><q:payload/></xs:appinfo></xs:annotation></xs:schema>`,
		},
		{
			name:   "mismatched opaque lexical end name",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:appinfo><a:p xmlns:a="urn:e" xmlns:b="urn:e"></b:p></xs:appinfo></xs:annotation></xs:schema>`,
		},
		{
			name:   "colon processing instruction target in appinfo",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:appinfo><?a:b data?></xs:appinfo></xs:annotation></xs:schema>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("malformed.xsd", []byte(test.schema))})
			var schemaErr *xsderrors.Error
			if !errors.As(err, &schemaErr) {
				t.Fatalf("Compile() error = %v, want structured schema error", err)
			}
			if schemaErr.Code() != xsderrors.CodeSchemaXML || schemaErr.Path() != "malformed.xsd" {
				t.Fatalf("Compile() error = %#v, want code %q and source path", schemaErr, xsderrors.CodeSchemaXML)
			}
			if schemaErr.Line() == 0 || schemaErr.Column() == 0 {
				t.Fatalf("Compile() location = %d:%d, want nonzero", schemaErr.Line(), schemaErr.Column())
			}
		})
	}
}

func TestCompileRejectsTruncatedTrailingMarkup(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	for _, suffix := range []string{`</`, `</xs:schema `, `<next `} {
		t.Run(suffix, func(t *testing.T) {
			t.Parallel()
			_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{
				source.Bytes("malformed.xsd", []byte(schema+suffix)),
			})
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeSchemaXML {
				t.Fatalf("Compile() error = %v, want %q", err, xsderrors.CodeSchemaXML)
			}
		})
	}
}

func TestSchemaXMLUsesBufferedCharacterFailurePosition(t *testing.T) {
	t.Parallel()
	schema := "<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\nabcdefgh\x01</xs:schema>"
	_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("malformed.xsd", []byte(schema))})
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("Compile() error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Code() != xsderrors.CodeSchemaXML || xerr.Path() != "malformed.xsd" || xerr.Line() != 2 || xerr.Column() != 9 {
		t.Fatalf("Compile() diagnostic = %s %q at %d:%d, want %s malformed.xsd at 2:9", xerr.Code(), xerr.Path(), xerr.Line(), xerr.Column(), xsderrors.CodeSchemaXML)
	}
}

func TestSchemaXMLDeclarationClassificationDoesNotDependOnPreviewLength(t *testing.T) {
	tests := []struct {
		name    string
		content string
		code    xsderrors.Code
	}{
		{name: "version", content: `version="1.1"`, code: xsderrors.CodeUnsupportedXMLVersion},
		{name: "encoding", content: `version="1.0" encoding="ISO-8859-1"`, code: xsderrors.CodeUnsupportedNonUTF8},
		{name: "malformed version", content: `version="1,0"`, code: xsderrors.CodeSchemaXML},
		{name: "malformed encoding", content: `version="1.0" encoding="UTF-8 "`, code: xsderrors.CodeSchemaXML},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema := `<?xml` + strings.Repeat(" ", 70<<10) + test.content + `?><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
			_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
			var schemaErr *xsderrors.Error
			if !errors.As(err, &schemaErr) || schemaErr.Code() != test.code {
				t.Fatalf("Compile() error = %v, want %q", err, test.code)
			}
			if test.code == xsderrors.CodeUnsupportedXMLVersion && schemaErr.Cause() != nil {
				t.Fatalf("Compile() unsupported version cause = %T, want nil", schemaErr.Cause())
			}
		})
	}
}

func TestSchemaAttributeDatatypeWhitespaceIsCollapsed(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:complexType name="T"><xs:sequence><xs:any processContents=" lax "/><xs:element name="child" form=" unqualified "/></xs:sequence><xs:attribute name="a" use=" required "/></xs:complexType>
	  <xs:element name="root" type="T"/>
	</xs:schema>`
	if _, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace=" "/>`))})
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code() != xsderrors.CodeSchemaInvalidAttribute {
		t.Fatalf("Compile(whitespace targetNamespace) error = %v, want %q", err, xsderrors.CodeSchemaInvalidAttribute)
	}
}

func TestSchemaFacetControlValuesApplyDatatypeWhitespace(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Sized"><xs:restriction base="xs:string"><xs:length value=" 2 "/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Decimal"><xs:restriction base="xs:decimal"><xs:fractionDigits value="&#x9;2&#xA;"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Collapsed"><xs:restriction base="xs:string"><xs:whiteSpace value=" collapse "/></xs:restriction></xs:simpleType>
</xs:schema>`
	if _, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestSchemaLexicalAttributesPreserveCharacterReferenceWhitespace(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="EnumTab"><xs:restriction base="xs:string"><xs:enumeration value="a&#x9;b"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="LiteralTab"><xs:restriction base="xs:string"><xs:enumeration value="a	b"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="PatternTab"><xs:restriction base="xs:string"><xs:pattern value="a&#x9;b"/></xs:restriction></xs:simpleType>
  <xs:element name="enum" type="EnumTab"/>
  <xs:element name="literal" type="LiteralTab"/>
  <xs:element name="default" type="PatternTab" default="a&#x9;b"/>
  <xs:element name="fixed" type="PatternTab" fixed="a&#x9;b"/>
</xs:schema>`
	engine, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, "<enum>a\tb</enum>")
	mustNotValidate(t, engine, "<enum>a b</enum>", xsderrors.CodeValidationFacet)
	mustValidate(t, engine, "<literal>a b</literal>")
	mustNotValidate(t, engine, "<literal>a\tb</literal>", xsderrors.CodeValidationFacet)
	mustValidate(t, engine, `<default/>`)
	mustValidate(t, engine, "<fixed>a\tb</fixed>")
	mustNotValidate(t, engine, "<fixed>a b</fixed>", xsderrors.CodeValidationFacet)
}

func TestSchemaDocumentationLanguageAppliesDatatypeWhitespace(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation><xs:documentation xml:lang=" en-US ">text</xs:documentation></xs:annotation>
</xs:schema>`
	if _, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestSchemaGraphAnyURIsAreCollapsed(t *testing.T) {
	docs := map[string]string{
		"imported.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b&#x9;"><xs:element name="e"/></xs:schema>`,
	}
	var resolver source.Resolver
	resolver = func(_, location string) (source.Source, error) {
		doc, ok := docs[location]
		if !ok {
			return source.Source{}, fmt.Errorf("missing %s", location)
		}
		return source.Bytes(location, []byte(doc)).WithResolver(resolver), nil
	}
	root := source.Bytes("root.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace=" urn:b " schemaLocation=" imported.xsd "/></xs:schema>`)).WithResolver(resolver)
	if _, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{root}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestSchemaGraphAnyURIsFailBeforeResolution(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		code   xsderrors.Code
	}{
		{
			name:   "include schemaLocation",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd?%zz"/></xs:schema>`,
			code:   xsderrors.CodeSchemaReference,
		},
		{
			name:   "built-in XML namespace import",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="http://www.w3.org/XML/1998/namespace" schemaLocation="%zz"/></xs:schema>`,
			code:   xsderrors.CodeSchemaReference,
		},
		{
			name:   "unused element xml base",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="e" xml:base="http://[bad]/"/></xs:schema>`,
			code:   xsderrors.CodeSchemaReference,
		},
		{
			name:   "target namespace",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="%zz"/>`,
			code:   xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name:   "import namespace",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="%zz" schemaLocation="child.xsd"/></xs:schema>`,
			code:   xsderrors.CodeSchemaInvalidAttribute,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			resolver := source.Resolver(func(_, _ string) (source.Source, error) {
				called = true
				return source.Bytes("child.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)), nil
			})
			root := source.Bytes("root.xsd", []byte(test.schema)).WithResolver(resolver)
			_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{root})
			if called {
				t.Fatal("resolver called before URI lexical validation")
			}
			var schemaErr *xsderrors.Error
			if !errors.As(err, &schemaErr) || schemaErr.Code() != test.code || schemaErr.Path() != "root.xsd" {
				t.Fatalf("Compile() error = %v, want %q at root.xsd", err, test.code)
			}
			if schemaErr.Line() == 0 || schemaErr.Column() == 0 {
				t.Fatalf("Compile() location = %d:%d, want nonzero", schemaErr.Line(), schemaErr.Column())
			}
		})
	}
}

func TestResolvedSchemaTargetNamespaceAnyURIIsValidated(t *testing.T) {
	called := 0
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		called++
		return source.Bytes(location, []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="%zz"/>`)), nil
	})
	root := source.Bytes("root.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="urn:child" schemaLocation="child.xsd"/></xs:schema>`)).WithResolver(resolver)
	_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{root})
	if called != 1 {
		t.Fatalf("resolver calls = %d, want 1", called)
	}
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code() != xsderrors.CodeSchemaInvalidAttribute || schemaErr.Path() != "child.xsd" {
		t.Fatalf("Compile() error = %v, want %q at child.xsd", err, xsderrors.CodeSchemaInvalidAttribute)
	}
}

func TestSchemaAnyURIAttributesAreValidated(t *testing.T) {
	tests := []string{
		`<xs:annotation><xs:appinfo source="%zz"/></xs:annotation>`,
		`<xs:annotation><xs:documentation source="http://[bad]/"/></xs:annotation>`,
		`<xs:notation name="n" public="id" system="%zz"/>`,
		`<xs:complexType name="t"><xs:anyAttribute namespace="urn:a %zz"/></xs:complexType>`,
	}
	for _, body := range tests {
		_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		var schemaErr *xsderrors.Error
		if !errors.As(err, &schemaErr) || schemaErr.Code() != xsderrors.CodeSchemaInvalidAttribute {
			t.Fatalf("Compile(%s) error = %v, want %q", body, err, xsderrors.CodeSchemaInvalidAttribute)
		}
	}
}

func TestSchemaExtendedAnyURIAttributesAreAccepted(t *testing.T) {
	t.Parallel()
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a^b">
		<xs:annotation><xs:documentation source="a\b"/></xs:annotation>
		<xs:complexType name="t"><xs:anyAttribute namespace="urn:a^b"/></xs:complexType>
		<xs:element name="e" xml:base="a^b"/>
	</xs:schema>`
	if _, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestResolverReceivesExtendedSchemaReferences(t *testing.T) {
	t.Parallel()
	const child = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="child"/></xs:schema>`
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="sub^/&#x7f;/"><xs:include schemaLocation="child\name.xsd"/></xs:schema>`
	resolver := source.Resolver(func(base, location string) (source.Source, error) {
		if base != "sub^/\x7f/" || location != `child\name.xsd` {
			return source.Source{}, fmt.Errorf("resolver inputs = %q, %q", base, location)
		}
		return source.Bytes("child.xsd", []byte(child)), nil
	})
	engine, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{
		source.Bytes("root.xsd", []byte(root)).WithResolver(resolver),
	})

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidate(t, engine, `<child/>`)
}

func TestSchemaGraphXMLBaseAppliesAnyURIWhitespace(t *testing.T) {
	tests := []struct {
		name   string
		root   string
		parent string
	}{
		{
			name:   "schema root",
			root:   `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="  sub/&#x9;"><xs:include schemaLocation="child.xsd"/></xs:schema>`,
			parent: "schemas/root.xsd",
		},
		{
			name:   "include",
			root:   `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include xml:base="&#xA; sub/  " schemaLocation="child.xsd"/></xs:schema>`,
			parent: "schemas/root.xsd",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wantBase := filepath.Join("schemas", "sub") + string(filepath.Separator)
			var gotBase string
			resolver := source.Resolver(func(base, location string) (source.Source, error) {
				gotBase = base
				if location != "child.xsd" {
					return source.Source{}, fmt.Errorf("location = %q", location)
				}
				return source.Bytes("child.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)), nil
			})
			root := source.Bytes(test.parent, []byte(test.root)).WithResolver(resolver)
			if _, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{root}); err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if gotBase != wantBase {
				t.Fatalf("resolver base = %q, want collapsed %q", gotBase, wantBase)
			}
		})
	}
}

func TestSchemaGraphAllowsUnavailableXMLBaseWithoutReferences(t *testing.T) {
	root := source.Bytes("urn:opaque:root", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="relative/"/>`))
	if _, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{root}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

// I-021: a targetless document's src-include 2.1 and src-import 1.1-1.2
// checks use the namespace adopted by each target-context view.
func TestChameleonReferenceChecksUseEachTargetContext(t *testing.T) {
	const (
		xs       = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"`
		includeA = xs + ` targetNamespace="urn:a"><xs:include schemaLocation="child.xsd"/></xs:schema>`
		includeB = xs + ` targetNamespace="urn:b"><xs:include schemaLocation="child.xsd"/></xs:schema>`
		namedA   = xs + ` targetNamespace="urn:a"><xs:element name="e" type="xs:int"/></xs:schema>`
	)
	child := func(body string) string { return xs + ">" + body + "</xs:schema>" }
	tests := []struct {
		name     string
		roots    []string
		names    []string // root source names; default root<i>.xsd
		docs     map[string]string
		failsAt  string
		instance string
	}{
		{
			name:     "include of adopted namespace",
			roots:    []string{includeA},
			docs:     map[string]string{"child.xsd": child(`<xs:include schemaLocation="named.xsd"/>`), "named.xsd": namedA},
			instance: `<e xmlns="urn:a">1</e>`,
		},
		{
			name:    "include of other namespace",
			roots:   []string{includeA},
			docs:    map[string]string{"child.xsd": child(`<xs:include schemaLocation="named.xsd"/>`), "named.xsd": xs + ` targetNamespace="urn:b"/>`},
			failsAt: "child.xsd",
		},
		{
			name:  "import without namespace in adopted view",
			roots: []string{includeA},
			docs:  map[string]string{"child.xsd": child(`<xs:import/>`)},
		},
		{
			name:    "import of adopted namespace",
			roots:   []string{includeA},
			docs:    map[string]string{"child.xsd": child(`<xs:import namespace="urn:a"/>`)},
			failsAt: "child.xsd",
		},
		{
			name:  "import of other namespace",
			roots: []string{includeA},
			docs:  map[string]string{"child.xsd": child(`<xs:import namespace="urn:b"/>`)},
		},
		{
			name:    "import of second view namespace",
			roots:   []string{includeA, includeB},
			docs:    map[string]string{"child.xsd": child(`<xs:import namespace="urn:b"/>`)},
			failsAt: "child.xsd",
		},
		{
			name:    "import of second view namespace reversed roots",
			roots:   []string{includeB, includeA},
			docs:    map[string]string{"child.xsd": child(`<xs:import namespace="urn:b"/>`)},
			failsAt: "child.xsd",
		},
		{
			name:    "include of first view namespace",
			roots:   []string{includeA, includeB},
			docs:    map[string]string{"child.xsd": child(`<xs:include schemaLocation="named.xsd"/>`), "named.xsd": namedA},
			failsAt: "child.xsd",
		},
		{
			name:  "import of namespace outside both views",
			roots: []string{includeA, includeB},
			docs:  map[string]string{"child.xsd": child(`<xs:import namespace="urn:c"/>`)},
		},
		{
			name:    "import without namespace in targetless view",
			roots:   []string{xs + `><xs:include schemaLocation="child.xsd"/></xs:schema>`},
			docs:    map[string]string{"child.xsd": child(`<xs:import/>`)},
			failsAt: "child.xsd",
		},
		{
			name:    "import without namespace in targetless root",
			roots:   []string{child(`<xs:import/>`)},
			failsAt: "root0.xsd",
		},
		{
			name:  "targetless root imports a namespace",
			roots: []string{child(`<xs:import namespace="urn:t"/>`)},
		},
		{
			name:    "chameleon target also named as targetless root",
			roots:   []string{includeA, child(`<xs:import/>`)},
			names:   []string{"root0.xsd", "child.xsd"},
			docs:    map[string]string{"child.xsd": child(`<xs:import/>`)},
			failsAt: "child.xsd",
		},
		{
			name:    "targetless root also included as chameleon",
			roots:   []string{child(`<xs:import/>`), includeA},
			names:   []string{"child.xsd", "root1.xsd"},
			docs:    map[string]string{"child.xsd": child(`<xs:import/>`)},
			failsAt: "child.xsd",
		},
		{
			name:  "chameleon target also named as root imports a namespace",
			roots: []string{includeA, child(`<xs:import namespace="urn:t"/>`)},
			names: []string{"root0.xsd", "child.xsd"},
			docs:  map[string]string{"child.xsd": child(`<xs:import namespace="urn:t"/>`)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var resolver source.Resolver
			resolver = func(_, location string) (source.Source, error) {
				doc, ok := test.docs[location]
				if !ok {
					return source.Source{}, fmt.Errorf("missing %s", location)
				}
				return source.Bytes(location, []byte(doc)).WithResolver(resolver), nil
			}
			roots := make([]source.Source, len(test.roots))
			for i, root := range test.roots {
				name := fmt.Sprintf("root%d.xsd", i)
				if test.names != nil {
					name = test.names[i]
				}
				roots[i] = source.Bytes(name, []byte(root)).WithResolver(resolver)
			}
			rt, err := xsdSchema.Compile(xsdSchema.Options{}, roots)
			if test.failsAt == "" {
				if err != nil {
					t.Fatalf("Compile() error = %v", err)
				}
				if test.instance != "" {
					mustValidate(t, rt, test.instance)
				}
				return
			}
			expectCode(t, err, xsderrors.CodeSchemaReference)
			if !strings.Contains(err.Error(), test.failsAt) {
				t.Fatalf("Compile() error = %v, want failure in %s", err, test.failsAt)
			}
		})
	}
}
