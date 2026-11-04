package schema

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseIdentityPaths(t *testing.T) {
	t.Parallel()

	resolver := identityNameResolverStub{
		namespaces: map[string]NamespaceID{"p": 2},
		qnames: map[string]QName{
			":row":   {Local: 1},
			":code":  {Local: 2},
			"p:item": {Namespace: 2, Local: 3},
		},
	}

	paths, err := ParseIdentityPaths(". | .// child::row / code | p:*", resolver)
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
		namespaces: map[string]NamespaceID{"p": 2},
		qnames: map[string]QName{
			":row":  {Local: 1},
			":id":   {Local: 2},
			"p:ref": {Namespace: 2, Local: 3},
		},
	}

	paths, err := ParseIdentityFieldPaths(". | row/@p:ref | attribute::p:*", resolver)
	if err != nil {
		t.Fatalf("ParseIdentityFieldPaths() error = %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("path count = %d, want 3", len(paths))
	}
	if !identityFieldPathProgramRead(paths[0]).Self() || paths[0].Attribute != NoQName() {
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
		paths, err := ParseIdentityPaths(tt.xpath, resolver)
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
		paths, err := ParseIdentityFieldPaths(tt.xpath, resolver)
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
		wantAttribute := NoQName()
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
				_, err := ParseIdentityPaths("| a", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "empty field branch",
			parse: func() error {
				_, err := ParseIdentityFieldPaths("| @a", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "trailing slash after self",
			parse: func() error {
				_, err := ParseIdentityPaths("./", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "empty step between names",
			parse: func() error {
				_, err := ParseIdentityPaths("a//a", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "descendant prefix without step",
			parse: func() error {
				_, err := ParseIdentityPaths(".//", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "empty step between self steps",
			parse: func() error {
				_, err := ParseIdentityPaths("./ /.", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "field trailing slash after self",
			parse: func() error {
				_, err := ParseIdentityFieldPaths("./", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid child axis",
			parse: func() error {
				_, err := ParseIdentityPaths("child::", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "child axis requires node test",
			parse: func() error {
				_, err := ParseIdentityPaths("child :: .", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "field child axis requires node test",
			parse: func() error {
				_, err := ParseIdentityFieldPaths("child::.", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid attribute axis",
			parse: func() error {
				_, err := ParseIdentityFieldPaths("attribute::", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "attribute in selector",
			parse: func() error {
				_, err := ParseIdentityPaths("@*", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaIdentity,
		},
		{
			name: "invalid QName",
			parse: func() error {
				_, err := ParseIdentityFieldPaths("t: *", resolver)
				return err
			},
			category: xsderrors.CategorySchemaCompile,
			code:     xsderrors.CodeSchemaReference,
		},
		{
			name: "unbound wildcard prefix",
			parse: func() error {
				_, err := ParseIdentityFieldPaths("@p:*", resolver)
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
	namespaces map[string]NamespaceID
	qnames     map[string]QName
}

func (s identityNameResolverStub) ResolveIdentityQName(parts QNameParts) (QName, error) {
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

func (s identityNameResolverStub) ResolveIdentityWildcardNamespace(prefix string) (NamespaceID, error) {
	ns, ok := s.namespaces[prefix]
	if !ok {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return ns, nil
}
