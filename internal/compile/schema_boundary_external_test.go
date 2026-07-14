package compile_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("malformed.xsd", []byte(test.schema))})
			var schemaErr *xsderrors.Error
			if !errors.As(err, &schemaErr) {
				t.Fatalf("Compile() error = %v, want structured schema error", err)
			}
			if schemaErr.Code != xsderrors.CodeSchemaXML || schemaErr.Path != "malformed.xsd" {
				t.Fatalf("Compile() error = %#v, want code %q and source path", schemaErr, xsderrors.CodeSchemaXML)
			}
			if schemaErr.Line == 0 || schemaErr.Column == 0 {
				t.Fatalf("Compile() location = %d:%d, want nonzero", schemaErr.Line, schemaErr.Column)
			}
		})
	}
}

func TestSchemaUnsupportedXMLDeclarationClassificationDoesNotDependOnPreviewLength(t *testing.T) {
	tests := []struct {
		name    string
		content string
		code    xsderrors.Code
	}{
		{name: "version", content: `version="1.1"`, code: xsderrors.CodeUnsupportedXML11},
		{name: "encoding", content: `version="1.0" encoding="ISO-8859-1"`, code: xsderrors.CodeUnsupportedNonUTF8},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema := `<?xml` + strings.Repeat(" ", 70<<10) + test.content + `?><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
			_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
			var schemaErr *xsderrors.Error
			if !errors.As(err, &schemaErr) || schemaErr.Code != test.code {
				t.Fatalf("Compile() error = %v, want %q", err, test.code)
			}
		})
	}
}

func TestSchemaAttributeDatatypeWhitespaceIsCollapsed(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:complexType name="T"><xs:sequence><xs:any processContents=" lax "/><xs:element name="child" form=" unqualified "/></xs:sequence><xs:attribute name="a" use=" required "/></xs:complexType>
	  <xs:element name="root" type="T"/>
	</xs:schema>`
	if _, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace=" "/>`))})
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeSchemaInvalidAttribute {
		t.Fatalf("Compile(whitespace targetNamespace) error = %v, want %q", err, xsderrors.CodeSchemaInvalidAttribute)
	}
}

func TestSchemaFacetControlValuesApplyDatatypeWhitespace(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Sized"><xs:restriction base="xs:string"><xs:length value=" 2 "/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Decimal"><xs:restriction base="xs:decimal"><xs:fractionDigits value="&#x9;2&#xA;"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Collapsed"><xs:restriction base="xs:string"><xs:whiteSpace value=" collapse "/></xs:restriction></xs:simpleType>
</xs:schema>`
	if _, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
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
	engine, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, "<enum>a\tb</enum>")
	mustNotValidateRuntime(t, engine, "<enum>a b</enum>", xsderrors.CodeValidationFacet)
	mustValidateRuntime(t, engine, "<literal>a b</literal>")
	mustNotValidateRuntime(t, engine, "<literal>a\tb</literal>", xsderrors.CodeValidationFacet)
	mustValidateRuntime(t, engine, `<default/>`)
	mustValidateRuntime(t, engine, "<fixed>a\tb</fixed>")
	mustNotValidateRuntime(t, engine, "<fixed>a b</fixed>", xsderrors.CodeValidationFacet)
}

func TestSchemaDocumentationLanguageAppliesDatatypeWhitespace(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation><xs:documentation xml:lang=" en-US ">text</xs:documentation></xs:annotation>
</xs:schema>`
	if _, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestSchemaGraphAnyURIsAreCollapsed(t *testing.T) {
	docs := map[string]string{
		"imported.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b&#x9;"><xs:element name="e"/></xs:schema>`,
	}
	var resolver source.Resolver
	resolver = func(_ context.Context, _, location string) (source.Source, error) {
		doc, ok := docs[location]
		if !ok {
			return source.Source{}, fmt.Errorf("missing %s", location)
		}
		return source.Bytes(location, []byte(doc)).WithResolver(resolver), nil
	}
	root := source.Bytes("root.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace=" urn:b " schemaLocation=" imported.xsd "/></xs:schema>`)).WithResolver(resolver)
	if _, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{root}); err != nil {
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
			resolver := source.Resolver(func(_ context.Context, _, _ string) (source.Source, error) {
				called = true
				return source.Bytes("child.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)), nil
			})
			root := source.Bytes("root.xsd", []byte(test.schema)).WithResolver(resolver)
			_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{root})
			if called {
				t.Fatal("resolver called before URI lexical validation")
			}
			var schemaErr *xsderrors.Error
			if !errors.As(err, &schemaErr) || schemaErr.Code != test.code || schemaErr.Path != "root.xsd" {
				t.Fatalf("Compile() error = %v, want %q at root.xsd", err, test.code)
			}
			if schemaErr.Line == 0 || schemaErr.Column == 0 {
				t.Fatalf("Compile() location = %d:%d, want nonzero", schemaErr.Line, schemaErr.Column)
			}
		})
	}
}

func TestResolvedSchemaTargetNamespaceAnyURIIsValidated(t *testing.T) {
	called := 0
	resolver := source.Resolver(func(_ context.Context, _, location string) (source.Source, error) {
		called++
		return source.Bytes(location, []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="%zz"/>`)), nil
	})
	root := source.Bytes("root.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="urn:child" schemaLocation="child.xsd"/></xs:schema>`)).WithResolver(resolver)
	_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{root})
	if called != 1 {
		t.Fatalf("resolver calls = %d, want 1", called)
	}
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeSchemaInvalidAttribute || schemaErr.Path != "child.xsd" {
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
		_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		var schemaErr *xsderrors.Error
		if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeSchemaInvalidAttribute {
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
	if _, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestResolverReceivesExtendedSchemaReferences(t *testing.T) {
	t.Parallel()
	const child = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="child"/></xs:schema>`
	root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="sub^/&#x7f;/"><xs:include schemaLocation="child\name.xsd"/></xs:schema>`
	resolver := source.Resolver(func(_ context.Context, base, location string) (source.Source, error) {
		if base != "sub^/\x7f/" || location != `child\name.xsd` {
			return source.Source{}, fmt.Errorf("resolver inputs = %q, %q", base, location)
		}
		return source.Bytes("child.xsd", []byte(child)), nil
	})
	engine, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{
		source.Bytes("root.xsd", []byte(root)).WithResolver(resolver),
	})

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<child/>`)
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
			wantBase, err := source.ResolveReference(test.parent, "sub/")
			if err != nil {
				t.Fatal(err)
			}
			var gotBase string
			resolver := source.Resolver(func(_ context.Context, base, location string) (source.Source, error) {
				gotBase = base
				if location != "child.xsd" {
					return source.Source{}, fmt.Errorf("location = %q", location)
				}
				return source.Bytes("child.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)), nil
			})
			root := source.Bytes(test.parent, []byte(test.root)).WithResolver(resolver)
			if _, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{root}); err != nil {
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
	if _, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{root}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestChameleonStructuralChecksUseDeclaredTargetNamespace(t *testing.T) {
	tests := []struct {
		name  string
		child string
		docs  map[string]string
	}{
		{
			name:  "targetless child cannot include named schema",
			child: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="named.xsd"/></xs:schema>`,
			docs: map[string]string{
				"named.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"/>`,
			},
		},
		{
			name:  "targetless child cannot import without namespace",
			child: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import/></xs:schema>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			docs := test.docs
			if docs == nil {
				docs = make(map[string]string)
			}
			docs["child.xsd"] = test.child
			var resolver source.Resolver
			resolver = func(_ context.Context, _, location string) (source.Source, error) {
				doc, ok := docs[location]
				if !ok {
					return source.Source{}, fmt.Errorf("missing %s", location)
				}
				return source.Bytes(location, []byte(doc)).WithResolver(resolver), nil
			}
			root := source.Bytes("root.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:include schemaLocation="child.xsd"/></xs:schema>`)).WithResolver(resolver)
			_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{root})
			var schemaErr *xsderrors.Error
			if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeSchemaReference {
				t.Fatalf("Compile() error = %v, want %q", err, xsderrors.CodeSchemaReference)
			}
		})
	}
}
