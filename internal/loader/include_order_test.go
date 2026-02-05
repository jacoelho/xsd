package loader

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

func TestIncludeGlobalDeclOrder(t *testing.T) {
	const includeDoc = `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:order"
           targetNamespace="urn:order"
           elementFormDefault="qualified">
  <xs:element name="B" type="xs:string"/>
</xs:schema>`

	cases := []struct {
		name     string
		rootDoc  string
		expected []types.QName
	}{
		{
			name: "include-between-decls",
			rootDoc: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:order"
           targetNamespace="urn:order"
           elementFormDefault="qualified">
  <xs:element name="A" type="xs:string"/>
  <xs:include schemaLocation="b.xsd"/>
  <xs:element name="C" type="xs:string"/>
</xs:schema>`,
			expected: []types.QName{
				{Namespace: "urn:order", Local: "A"},
				{Namespace: "urn:order", Local: "B"},
				{Namespace: "urn:order", Local: "C"},
			},
		},
		{
			name: "include-after-decls",
			rootDoc: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:order"
           targetNamespace="urn:order"
           elementFormDefault="qualified">
  <xs:element name="A" type="xs:string"/>
  <xs:element name="C" type="xs:string"/>
  <xs:include schemaLocation="b.xsd"/>
</xs:schema>`,
			expected: []types.QName{
				{Namespace: "urn:order", Local: "A"},
				{Namespace: "urn:order", Local: "C"},
				{Namespace: "urn:order", Local: "B"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"root.xsd": &fstest.MapFile{Data: []byte(tc.rootDoc)},
				"b.xsd":    &fstest.MapFile{Data: []byte(includeDoc)},
			}

			loader := NewLoader(Config{FS: fsys})
			loaded, err := loader.Load("root.xsd")
			if err != nil {
				t.Fatalf("Load(root.xsd) error = %v", err)
			}

			gotDecls := globalDeclNames(loaded.GlobalDecls)
			if !reflect.DeepEqual(gotDecls, tc.expected) {
				t.Fatalf("GlobalDecls = %v, want %v", gotDecls, tc.expected)
			}

			registry, err := schema.AssignIDs(loaded)
			if err != nil {
				t.Fatalf("AssignIDs error = %v", err)
			}
			gotOrder := globalElementOrder(registry)
			if !reflect.DeepEqual(gotOrder, tc.expected) {
				t.Fatalf("ElementOrder = %v, want %v", gotOrder, tc.expected)
			}
		})
	}
}

func globalDeclNames(decls []parser.GlobalDecl) []types.QName {
	if len(decls) == 0 {
		return nil
	}
	names := make([]types.QName, 0, len(decls))
	for _, decl := range decls {
		names = append(names, decl.Name)
	}
	return names
}

func globalElementOrder(registry *schema.Registry) []types.QName {
	if registry == nil {
		return nil
	}
	order := make([]types.QName, 0, len(registry.ElementOrder))
	for _, entry := range registry.ElementOrder {
		if entry.Global {
			order = append(order, entry.QName)
		}
	}
	return order
}
