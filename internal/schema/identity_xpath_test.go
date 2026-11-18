package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseIdentityPaths(t *testing.T) {
	t.Parallel()

	resolver := identityNameResolverStub{
		namespaces: map[string]namespaceID{"p": 2},
		qnames: map[string]QName{
			":row":   {Local: 1},
			":code":  {Local: 2},
			"p:item": {Namespace: 2, Local: 3},
		},
	}

	paths, err := parseIdentityPaths(". | .// child::row / code | p:*", resolver)
	if err != nil {
		t.Fatalf("ParseIdentityPaths() error = %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("path count = %d, want 3", len(paths))
	}
	if !identitySelectorPathProgramRead(paths[0]).Self() {
		t.Fatalf("first path = %#v, want self", paths[0])
	}
	if !paths[1].Descendant || len(paths[1].Steps) != 2 || paths[1].Steps[0].Name != resolver.qnames[":row"] || paths[1].Steps[1].Name != resolver.qnames[":code"] {
		t.Fatalf("second path = %#v, want descendant row/code", paths[1])
	}
	if !paths[2].Steps[0].Wildcard || !paths[2].Steps[0].NamespaceSet || paths[2].Steps[0].Namespace != 2 {
		t.Fatalf("third path = %#v, want p:*", paths[2])
	}
}

func TestParseIdentityFieldPaths(t *testing.T) {
	t.Parallel()

	resolver := identityNameResolverStub{
		namespaces: map[string]namespaceID{"p": 2},
		qnames: map[string]QName{
			":row":  {Local: 1},
			":id":   {Local: 2},
			"p:ref": {Namespace: 2, Local: 3},
		},
	}

	paths, err := parseIdentityFieldPaths(". | row/@p:ref | attribute::p:*", resolver)
	if err != nil {
		t.Fatalf("ParseIdentityFieldPaths() error = %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("path count = %d, want 3", len(paths))
	}
	if !identityFieldPathProgramRead(paths[0]).Self() || paths[0].Attribute != noQName() {
		t.Fatalf("first field path = %#v, want self", paths[0])
	}
	if !paths[1].Attr || paths[1].Attribute != resolver.qnames["p:ref"] || len(paths[1].Steps) != 1 || paths[1].Steps[0].Name != resolver.qnames[":row"] {
		t.Fatalf("second field path = %#v, want row/@p:ref", paths[1])
	}
	if !paths[2].Attr || !paths[2].AttrWildcard || !paths[2].AttrNamespaceSet || paths[2].AttrNamespace != 2 {
		t.Fatalf("third field path = %#v, want attribute::p:*", paths[2])
	}
}

// TestParseIdentityPathShapes pins the canonical path shape: every spelling of
// the context node ("." steps in any number) is one non-descendant zero-step
// path, which the read view reports as self. Attribute fields with zero element
// steps are not self.
func TestParseIdentityPathShapes(t *testing.T) {
	t.Parallel()

	resolver := identityNameResolverStub{qnames: map[string]QName{
		":e":  {Local: 1},
		":f":  {Local: 2},
		":id": {Local: 3},
	}}
	type shape struct {
		descendant bool
		steps      int
		self       bool
	}
	selectors := []struct {
		xpath string
		want  []shape
	}{
		{xpath: ".", want: []shape{{self: true}}},
		{xpath: "./.", want: []shape{{self: true}}},
		{xpath: "././.", want: []shape{{self: true}}},
		{xpath: ". /. ", want: []shape{{self: true}}},
		{xpath: ". | ./.", want: []shape{{self: true}, {self: true}}},
		{xpath: ".//.", want: []shape{{descendant: true}}},
		{xpath: "./e/.", want: []shape{{steps: 1}}},
		{xpath: "e/./f", want: []shape{{steps: 2}}},
		{xpath: "./*", want: []shape{{steps: 1}}},
	}
	for _, tt := range selectors {
		paths, err := parseIdentityPaths(tt.xpath, resolver)
		if err != nil {
			t.Fatalf("ParseIdentityPaths(%q) error = %v", tt.xpath, err)
		}
		if len(paths) != len(tt.want) {
			t.Fatalf("ParseIdentityPaths(%q) count = %d, want %d", tt.xpath, len(paths), len(tt.want))
		}
		for i, path := range paths {
			got := shape{descendant: path.Descendant, steps: len(path.Steps), self: identitySelectorPathProgramRead(path).Self()}
			if got != tt.want[i] {
				t.Fatalf("ParseIdentityPaths(%q)[%d] = %+v, want %+v", tt.xpath, i, got, tt.want[i])
			}
		}
	}

	fields := []struct {
		xpath string
		want  shape
		attr  bool
	}{
		{xpath: ".", want: shape{self: true}},
		{xpath: "./.", want: shape{self: true}},
		{xpath: ".//.", want: shape{descendant: true}},
		{xpath: "e/.", want: shape{steps: 1}},
		{xpath: "@id", want: shape{}, attr: true},
		{xpath: "./@id", want: shape{}, attr: true},
		{xpath: ".//@id", want: shape{descendant: true}, attr: true},
	}
	for _, tt := range fields {
		paths, err := parseIdentityFieldPaths(tt.xpath, resolver)
		if err != nil {
			t.Fatalf("ParseIdentityFieldPaths(%q) error = %v", tt.xpath, err)
		}
		if len(paths) != 1 {
			t.Fatalf("ParseIdentityFieldPaths(%q) count = %d, want 1", tt.xpath, len(paths))
		}
		path := paths[0]
		got := shape{descendant: path.Descendant, steps: len(path.Steps), self: identityFieldPathProgramRead(path).Self()}
		if got != tt.want || path.Attr != tt.attr {
			t.Fatalf("ParseIdentityFieldPaths(%q) = %+v attr %v, want %+v attr %v", tt.xpath, got, path.Attr, tt.want, tt.attr)
		}
		wantAttribute := noQName()
		if tt.attr {
			wantAttribute = resolver.qnames[":id"]
		}
		if path.Attribute != wantAttribute {
			t.Fatalf("ParseIdentityFieldPaths(%q) attribute = %v, want %v", tt.xpath, path.Attribute, wantAttribute)
		}
	}
}

func TestParseIdentityXPathErrors(t *testing.T) {
	t.Parallel()

	resolver := identityNameResolverStub{qnames: map[string]QName{":a": {Local: 1}}}
	tests := []struct {
		name     string
		parse    func() error
		category xsderrors.Category
		code     xsderrors.Code
	}{
		{
			name: "empty selector branch",
			parse: func() error {
				_, err := parseIdentityPaths("| a", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "empty field branch",
			parse: func() error {
				_, err := parseIdentityFieldPaths("| @a", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "trailing slash after self",
			parse: func() error {
				_, err := parseIdentityPaths("./", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "empty step between names",
			parse: func() error {
				_, err := parseIdentityPaths("a//a", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "descendant prefix without step",
			parse: func() error {
				_, err := parseIdentityPaths(".//", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "empty step between self steps",
			parse: func() error {
				_, err := parseIdentityPaths("./ /.", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "field trailing slash after self",
			parse: func() error {
				_, err := parseIdentityFieldPaths("./", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid child axis",
			parse: func() error {
				_, err := parseIdentityPaths("child::", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "child axis requires node test",
			parse: func() error {
				_, err := parseIdentityPaths("child :: .", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "field child axis requires node test",
			parse: func() error {
				_, err := parseIdentityFieldPaths("child::.", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid attribute axis",
			parse: func() error {
				_, err := parseIdentityFieldPaths("attribute::", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "attribute in selector",
			parse: func() error {
				_, err := parseIdentityPaths("@*", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid QName",
			parse: func() error {
				_, err := parseIdentityFieldPaths("t: *", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
		},
		{
			name: "unbound wildcard prefix",
			parse: func() error {
				_, err := parseIdentityFieldPaths("@p:*", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expectDiagnostic(t, tt.parse(), tt.category, tt.code)
		})
	}
}

type identityNameResolverStub struct {
	namespaces map[string]namespaceID
	qnames     map[string]QName
}

func (s identityNameResolverStub) ResolveIdentityQName(parts qNameParts) (QName, error) {
	key := ":" + parts.Local
	if parts.Prefixed {
		key = parts.Prefix + ":" + parts.Local
	}
	q, ok := s.qnames[key]
	if !ok {
		return QName{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unbound QName prefix "+parts.Prefix)
	}
	return q, nil
}

func (s identityNameResolverStub) ResolveIdentityWildcardNamespace(prefix string) (namespaceID, error) {
	ns, ok := s.namespaces[prefix]
	if !ok {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return ns, nil
}

func TestSelectorAndFieldStoreThePathsBuiltAtTheirStartTag(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:default" xmlns:p="urn:outer" targetNamespace="urn:target">
  <xs:element name="root">
    <xs:key name="k">
      <xs:selector xmlns:p="urn:selector" xpath="p:row | .//cell"/>
      <xs:field xpath="p:code/@p:id"/>
      <xs:field xpath="attribute::p:*"/>
    </xs:key>
  </xs:element>
</xs:schema>`
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	admission := testSchemaAdmission(limits)
	doc, err := parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader), admission)
	if err != nil {
		t.Fatalf("parseSchemaDocument() error = %v", err)
	}
	key := doc.node(doc.node(doc.root.children[0]).children[0])
	name := func(ns, local string) QName {
		q, ok := admission.names.LookupQName(ns, local)
		if !ok {
			t.Fatalf("name table has no {%s}%s", ns, local)
		}
		return q
	}
	outer, ok := admission.names.LookupNamespace("urn:outer")
	if !ok {
		t.Fatal("name table has no urn:outer")
	}

	selector := semanticSource[*schemaIdentitySelectorSource](&doc.node(key.children[0]).semantic)
	if selector == nil || len(selector.Paths) != 2 {
		t.Fatalf("selector source = %#v, want two paths", selector)
	}
	if got := selector.Paths[0]; got.Descendant || len(got.Steps) != 1 || got.Steps[0].Name != name("urn:selector", "row") {
		t.Fatalf("selector path 0 = %#v, want {urn:selector}row from the selector's own binding", got)
	}
	if got := selector.Paths[1]; !got.Descendant || len(got.Steps) != 1 || got.Steps[0].Name != name("", "cell") {
		t.Fatalf("selector path 1 = %#v, want .//cell in no namespace despite the default and target namespaces", got)
	}

	code := semanticSource[*schemaIdentityFieldSource](&doc.node(key.children[1]).semantic)
	if code == nil || len(code.Paths) != 1 {
		t.Fatalf("first field source = %#v, want one path", code)
	}
	if got := code.Paths[0]; !got.Attr || got.Attribute != name("urn:outer", "id") || len(got.Steps) != 1 || got.Steps[0].Name != name("urn:outer", "code") {
		t.Fatalf("first field path = %#v, want {urn:outer}code/@{urn:outer}id", got)
	}
	wildcard := semanticSource[*schemaIdentityFieldSource](&doc.node(key.children[2]).semantic)
	if wildcard == nil || len(wildcard.Paths) != 1 {
		t.Fatalf("second field source = %#v, want one path", wildcard)
	}
	if got := wildcard.Paths[0]; !got.Attr || !got.AttrWildcard || !got.AttrNamespaceSet || got.AttrNamespace != outer {
		t.Fatalf("second field path = %#v, want attribute::{urn:outer}*", got)
	}
}

func TestUnboundXPathPrefixIsReportedAtItsStartTag(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:key name="k">
      <xs:selector xpath="."/>
      <xs:field xmlns:p="urn:bound" xpath="@p:a | @q:a"/>
    </xs:key>
  </xs:element>
</xs:schema>`
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader), testSchemaAdmission(limits))
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Code() != xsderrors.CodeSchemaReference || !strings.Contains(diagnostic.Message(), `unbound QName prefix "q"`) {
		t.Fatalf("parseSchemaDocument() error = %v, want schema.reference for unbound prefix q", err)
	}
	if diagnostic.Line() != 5 || diagnostic.Column() != 7 {
		t.Fatalf("parseSchemaDocument() location = %d:%d, want the xs:field start tag at 5:7", diagnostic.Line(), diagnostic.Column())
	}
}
