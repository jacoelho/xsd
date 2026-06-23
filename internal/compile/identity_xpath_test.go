package compile

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseIdentityPaths(t *testing.T) {
	t.Parallel()

	resolver := identityNameResolverStub{
		namespaces: map[string]runtime.NamespaceID{"p": 2},
		qnames: map[string]runtime.QName{
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
	if !paths[0].Self {
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
		namespaces: map[string]runtime.NamespaceID{"p": 2},
		qnames: map[string]runtime.QName{
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
	if !paths[0].Self || paths[0].Attribute != runtime.NoQName {
		t.Fatalf("first field path = %#v, want self", paths[0])
	}
	if !paths[1].Attr || paths[1].Attribute != resolver.qnames["p:ref"] || len(paths[1].Steps) != 1 || paths[1].Steps[0].Name != resolver.qnames[":row"] {
		t.Fatalf("second field path = %#v, want row/@p:ref", paths[1])
	}
	if !paths[2].Attr || !paths[2].AttrWildcard || !paths[2].AttrNamespaceSet || paths[2].AttrNamespace != 2 {
		t.Fatalf("third field path = %#v, want attribute::p:*", paths[2])
	}
}

func TestParseIdentityXPathErrors(t *testing.T) {
	t.Parallel()

	resolver := identityNameResolverStub{qnames: map[string]runtime.QName{":a": {Local: 1}}}
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
			name: "invalid child axis",
			parse: func() error {
				_, err := ParseIdentityPaths("child::", resolver)
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
	namespaces map[string]runtime.NamespaceID
	qnames     map[string]runtime.QName
}

func (s identityNameResolverStub) ResolveIdentityQName(prefix, local string, prefixed bool) (runtime.QName, error) {
	key := ":" + local
	if prefixed {
		key = prefix + ":" + local
	}
	q, ok := s.qnames[key]
	if !ok {
		return runtime.QName{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return q, nil
}

func (s identityNameResolverStub) ResolveIdentityWildcardNamespace(prefix string) (runtime.NamespaceID, error) {
	ns, ok := s.namespaces[prefix]
	if !ok {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return ns, nil
}
