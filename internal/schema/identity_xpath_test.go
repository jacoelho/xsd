package schema

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseIdentityPaths(t *testing.T) {
	t.Parallel()

	xpaths := newIdentityXPathFixture(t, map[string]string{"p": "urn:p"})
	paths, err := xpaths.selector(". | .// child::row / code | p:*")
	if err != nil {
		t.Fatalf("ParseIdentityPaths() error = %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("path count = %d, want 3", len(paths))
	}
	if !identitySelectorPathProgramRead(paths[0]).Self() {
		t.Fatalf("first path = %#v, want self", paths[0])
	}
	if !paths[1].Descendant || len(paths[1].Steps) != 2 || paths[1].Steps[0].Name != xpaths.qname(t, "", "row") || paths[1].Steps[1].Name != xpaths.qname(t, "", "code") {
		t.Fatalf("second path = %#v, want descendant row/code", paths[1])
	}
	if !paths[2].Steps[0].Wildcard || !paths[2].Steps[0].NamespaceSet || paths[2].Steps[0].Namespace != xpaths.namespace(t, "urn:p") {
		t.Fatalf("third path = %#v, want p:*", paths[2])
	}
}

func TestParseIdentityFieldPaths(t *testing.T) {
	t.Parallel()

	xpaths := newIdentityXPathFixture(t, map[string]string{"p": "urn:p"})
	paths, err := xpaths.field(". | row/@p:ref | attribute::p:*")
	if err != nil {
		t.Fatalf("ParseIdentityFieldPaths() error = %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("path count = %d, want 3", len(paths))
	}
	if !identityFieldPathProgramRead(paths[0]).Self() || paths[0].Attribute != noQName() {
		t.Fatalf("first field path = %#v, want self", paths[0])
	}
	if !paths[1].Attr || paths[1].Attribute != xpaths.qname(t, "urn:p", "ref") || len(paths[1].Steps) != 1 || paths[1].Steps[0].Name != xpaths.qname(t, "", "row") {
		t.Fatalf("second field path = %#v, want row/@p:ref", paths[1])
	}
	if !paths[2].Attr || !paths[2].AttrWildcard || !paths[2].AttrNamespaceSet || paths[2].AttrNamespace != xpaths.namespace(t, "urn:p") {
		t.Fatalf("third field path = %#v, want attribute::p:*", paths[2])
	}
}

// TestParseIdentityPathShapes pins the canonical path shape: every spelling of
// the context node ("." steps in any number) is one non-descendant zero-step
// path, which the read view reports as self. Attribute fields with zero element
// steps are not self.
func TestParseIdentityPathShapes(t *testing.T) {
	t.Parallel()

	xpaths := newIdentityXPathFixture(t, nil)
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
		paths, err := xpaths.selector(tt.xpath)
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
		paths, err := xpaths.field(tt.xpath)
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
			wantAttribute = xpaths.qname(t, "", "id")
		}
		if path.Attribute != wantAttribute {
			t.Fatalf("ParseIdentityFieldPaths(%q) attribute = %v, want %v", tt.xpath, path.Attribute, wantAttribute)
		}
	}
}

func TestParseIdentityXPathErrors(t *testing.T) {
	t.Parallel()

	xpaths := newIdentityXPathFixture(t, nil)
	tests := []struct {
		name     string
		parse    func() error
		category xsderrors.Category
		code     xsderrors.Code
	}{
		{
			name: "empty selector branch",
			parse: func() error {
				_, err := xpaths.selector("| a")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "empty field branch",
			parse: func() error {
				_, err := xpaths.field("| @a")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "trailing slash after self",
			parse: func() error {
				_, err := xpaths.selector("./")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "empty step between names",
			parse: func() error {
				_, err := xpaths.selector("a//a")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "descendant prefix without step",
			parse: func() error {
				_, err := xpaths.selector(".//")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "empty step between self steps",
			parse: func() error {
				_, err := xpaths.selector("./ /.")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "field trailing slash after self",
			parse: func() error {
				_, err := xpaths.field("./")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid child axis",
			parse: func() error {
				_, err := xpaths.selector("child::")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "child axis requires node test",
			parse: func() error {
				_, err := xpaths.selector("child :: .")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "field child axis requires node test",
			parse: func() error {
				_, err := xpaths.field("child::.")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid attribute axis",
			parse: func() error {
				_, err := xpaths.field("attribute::")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "attribute in selector",
			parse: func() error {
				_, err := xpaths.selector("@*")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid QName",
			parse: func() error {
				_, err := xpaths.field("t: *")
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
		},
		{
			name: "unbound wildcard prefix",
			parse: func() error {
				_, err := xpaths.field("@p:*")
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

// identityXPathFixture binds the prefixes it holds, parses paths with them, and
// interns their names into its own name table.
type identityXPathFixture struct {
	prefixes map[string]string
	names    nameTable
}

func newIdentityXPathFixture(t *testing.T, prefixes map[string]string) *identityXPathFixture {
	t.Helper()
	names, err := newRuntimeNameTable(0)
	if err != nil {
		t.Fatal(err)
	}
	return &identityXPathFixture{prefixes: prefixes, names: names}
}

func (f *identityXPathFixture) ResolveIdentityPrefix(prefix string) (string, error) {
	ns, ok := f.prefixes[prefix]
	if !ok {
		return "", xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unbound QName prefix "+diagnostictext.Quote(prefix), nil)
	}
	return ns, nil
}

func (f *identityXPathFixture) selector(xpath string) ([]identityPath, error) {
	syntax, err := parseIdentityPaths(xpath, f)
	if err != nil {
		return nil, err
	}
	return syntax.selectorPaths(&f.names)
}

func (f *identityXPathFixture) field(xpath string) ([]identityFieldPath, error) {
	syntax, err := parseIdentityFieldPaths(xpath, f)
	if err != nil {
		return nil, err
	}
	return syntax.fieldPaths(&f.names)
}

func (f *identityXPathFixture) qname(t *testing.T, ns, local string) QName {
	t.Helper()
	q, ok := f.names.LookupQName(ns, local)
	if !ok {
		t.Fatalf("name table has no {%s}%s", ns, local)
	}
	return q
}

func (f *identityXPathFixture) namespace(t *testing.T, uri string) namespaceID {
	t.Helper()
	ns, ok := f.names.LookupNamespace(uri)
	if !ok {
		t.Fatalf("name table has no namespace %s", uri)
	}
	return ns
}

func TestSelectorAndFieldBindTheirNamesAtTheStartTagAndEachCompilerInternsThemIntoItsOwnTable(t *testing.T) {
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
	doc, err := parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader), testSchemaPatterns(limits))
	if err != nil {
		t.Fatalf("parseSchemaDocument() error = %v", err)
	}
	key := doc.node(doc.node(doc.root.children[0]).children[0])
	selector := testSemanticSource[*schemaIdentityXPathSource](t, &doc.node(key.children[0]).semantic)
	code := testSemanticSource[*schemaIdentityXPathSource](t, &doc.node(key.children[1]).semantic)
	wildcard := testSemanticSource[*schemaIdentityXPathSource](t, &doc.node(key.children[2]).semantic)

	for _, unrelated := range [][]string{nil, {"cell", "code", "row", "id", "urn:outer"}} {
		names, err := newRuntimeNameTable(0)
		if err != nil {
			t.Fatal(err)
		}
		for _, local := range unrelated {
			if _, err = newNameInterner(&names).InternQName("urn:unrelated", local); err != nil {
				t.Fatal(err)
			}
		}
		selectorPaths, err := selector.XPath.selectorPaths(&names)
		if err != nil {
			t.Fatalf("selectorPaths() error = %v", err)
		}
		codePaths, err := code.XPath.fieldPaths(&names)
		if err != nil {
			t.Fatalf("fieldPaths() error = %v", err)
		}
		wildcardPaths, err := wildcard.XPath.fieldPaths(&names)
		if err != nil {
			t.Fatalf("fieldPaths() error = %v", err)
		}
		name := func(ns, local string) QName {
			q, ok := names.LookupQName(ns, local)
			if !ok {
				t.Fatalf("name table has no {%s}%s", ns, local)
			}
			return q
		}
		outer, ok := names.LookupNamespace("urn:outer")
		if !ok {
			t.Fatal("name table has no urn:outer")
		}

		if len(selectorPaths) != 2 || len(codePaths) != 1 || len(wildcardPaths) != 1 {
			t.Fatalf("paths = %d, %d, %d, want two selector paths and one path per field", len(selectorPaths), len(codePaths), len(wildcardPaths))
		}
		if got := selectorPaths[0]; got.Descendant || len(got.Steps) != 1 || got.Steps[0].Name != name("urn:selector", "row") {
			t.Fatalf("selector path 0 = %#v, want {urn:selector}row from the selector's own binding", got)
		}
		if got := selectorPaths[1]; !got.Descendant || len(got.Steps) != 1 || got.Steps[0].Name != name("", "cell") {
			t.Fatalf("selector path 1 = %#v, want .//cell in no namespace despite the default and target namespaces", got)
		}
		if got := codePaths[0]; !got.Attr || got.Attribute != name("urn:outer", "id") || len(got.Steps) != 1 || got.Steps[0].Name != name("urn:outer", "code") {
			t.Fatalf("first field path = %#v, want {urn:outer}code/@{urn:outer}id", got)
		}
		if got := wildcardPaths[0]; !got.Attr || !got.AttrWildcard || !got.AttrNamespaceSet || got.AttrNamespace != outer {
			t.Fatalf("second field path = %#v, want attribute::{urn:outer}*", got)
		}
	}
}

func TestUnboundXPathPrefixIsReportedAtItsStartTag(t *testing.T) {
	t.Parallel()

	for _, xpath := range []string{"@p:a | @q:a", "@p:a | @q:*"} {
		t.Run(xpath, func(t *testing.T) {
			t.Parallel()

			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:key name="k">
      <xs:selector xpath="."/>
      <xs:field xmlns:p="urn:bound" xpath="` + xpath + `"/>
    </xs:key>
  </xs:element>
</xs:schema>`
			limits, err := NormalizeOptions(Options{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = parseSchemaDocument("schema.xsd", "schema.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader), testSchemaPatterns(limits))
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeSchemaReference || diagnostic.Message() != `unbound QName prefix "q"` {
				t.Fatalf("parseSchemaDocument() error = %v, want schema.reference: unbound QName prefix \"q\"", err)
			}
			if diagnostic.Line() != 5 || diagnostic.Column() != 7 {
				t.Fatalf("parseSchemaDocument() location = %d:%d, want the xs:field start tag at 5:7", diagnostic.Line(), diagnostic.Column())
			}
		})
	}
}

func allocatedBytes(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestMalformedIdentityPathIsRejectedWithoutAllocatingPerSeparator(t *testing.T) {
	resolver := newIdentityXPathFixture(t, nil)
	const separators = 100_000
	// The smallest heap allocation is 8 bytes, so one allocation per separator
	// exceeds this bound eightfold. TotalAlloc is process-wide; the slack
	// absorbs runtime and race-detector allocations made during the parse.
	const budget = separators
	branches := "a" + strings.Repeat("|", separators)
	steps := strings.Repeat("./", separators) + "a b"
	tests := []struct {
		name  string
		parse func() error
	}{
		{name: "selector branches", parse: func() error {
			_, err := parseIdentityPaths(branches, resolver)
			return err
		}},
		{name: "field branches", parse: func() error {
			_, err := parseIdentityFieldPaths(branches, resolver)
			return err
		}},
		{name: "selector steps", parse: func() error {
			_, err := parseIdentityPaths(steps, resolver)
			return err
		}},
	}
	for _, tt := range tests {
		var err error
		bytes := allocatedBytes(func() { err = tt.parse() })
		if err == nil {
			t.Fatalf("%s: parse accepted a malformed path", tt.name)
		}
		if bytes > budget {
			t.Fatalf("%s: rejecting a path with %d separators allocated %d bytes, want at most %d", tt.name, separators, bytes, budget)
		}
	}
}

func TestIdentityPathsRetainCapacityForAcceptedStepsNotForSeparators(t *testing.T) {
	xpaths := newIdentityXPathFixture(t, nil)
	selfSteps := strings.Repeat("./", 1_000)
	const factor = 2

	paths, err := xpaths.selector(selfSteps + "a/" + selfSteps + "b | " + selfSteps + ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || cap(paths) > factor*len(paths) {
		t.Fatalf("selector paths len %d cap %d, want 2 paths and capacity at most %d", len(paths), cap(paths), factor*2)
	}
	if steps := paths[0].Steps; len(steps) != 2 || steps[0].Name != xpaths.qname(t, "", "a") || steps[1].Name != xpaths.qname(t, "", "b") || cap(steps) > factor*len(steps) {
		t.Fatalf("first selector path steps = %+v (cap %d), want a/b with capacity at most %d", steps, cap(steps), factor*2)
	}
	if steps := paths[1].Steps; len(steps) != 0 || cap(steps) > factor {
		t.Fatalf("self selector path steps len %d cap %d, want none and capacity at most %d", len(steps), cap(steps), factor)
	}

	fields, err := xpaths.field(selfSteps + "a/" + selfSteps + "@b")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || cap(fields) > factor {
		t.Fatalf("field paths len %d cap %d, want 1 path and capacity at most %d", len(fields), cap(fields), factor)
	}
	if steps := fields[0].Steps; len(steps) != 1 || steps[0].Name != xpaths.qname(t, "", "a") || cap(steps) > factor || fields[0].Attribute != xpaths.qname(t, "", "b") {
		t.Fatalf("field path = %+v (steps cap %d), want a/@b with step capacity at most %d", fields[0], cap(steps), factor)
	}
}
