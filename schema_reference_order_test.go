package xsd_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestReferenceViolationLosesToLaterParseViolationWhateverReferrerTargetNamespace(t *testing.T) {
	const (
		xs         = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"`
		root       = xs + ` targetNamespace="urn:a"><xs:include schemaLocation="c.xsd"/><xs:include schemaLocation="d.xsd"/></xs:schema>`
		namespaced = xs + ` targetNamespace="urn:a"><xs:import namespace="urn:a"/></xs:schema>`
		chameleon  = xs + `><xs:import namespace="urn:a"/></xs:schema>`
		badModel   = xs + ` targetNamespace="urn:a"><xs:complexType name="t"><xs:sequence><xs:foo/></xs:sequence></xs:complexType></xs:schema>`
		goodDoc    = xs + ` targetNamespace="urn:a"/>`
	)
	tests := []struct {
		name  string
		files map[string]string
		roots []string
		path  string
		code  xsderrors.Code
	}{
		{"targetNamespace referrer", map[string]string{"a.xsd": root, "c.xsd": namespaced, "d.xsd": badModel}, []string{"a.xsd"}, "d.xsd", xsderrors.CodeSchemaContentModel},
		{"chameleon referrer", map[string]string{"a.xsd": root, "c.xsd": chameleon, "d.xsd": badModel}, []string{"a.xsd"}, "d.xsd", xsderrors.CodeSchemaContentModel},
		{"explicit root referrer", map[string]string{"a.xsd": root, "c.xsd": chameleon, "d.xsd": badModel}, []string{"c.xsd", "a.xsd"}, "d.xsd", xsderrors.CodeSchemaContentModel},
		{"targetNamespace referrer alone", map[string]string{"a.xsd": root, "c.xsd": namespaced, "d.xsd": goodDoc}, []string{"a.xsd"}, "c.xsd", xsderrors.CodeSchemaReference},
		{"chameleon referrer alone", map[string]string{"a.xsd": root, "c.xsd": chameleon, "d.xsd": goodDoc}, []string{"a.xsd"}, "c.xsd", xsderrors.CodeSchemaReference},
		{
			"malformed import target behind the invalid edge",
			map[string]string{
				"a.xsd": root,
				"c.xsd": xs + ` targetNamespace="urn:a"><xs:import namespace="urn:a" schemaLocation="e.xsd"/></xs:schema>`,
				"d.xsd": goodDoc,
				"e.xsd": `<xs:schema`,
			},
			[]string{"a.xsd"}, "e.xsd", xsderrors.CodeSchemaXML,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range test.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			sources := make([]xsd.SchemaSource, len(test.roots))
			for i, name := range test.roots {
				sources[i] = xsd.File(filepath.Join(dir, name))
			}
			_, err := xsd.Compile(sources...)
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("Compile() error = %v, want diagnostic", err)
			}
			if diagnostic.Code() != test.code || filepath.Base(diagnostic.Path()) != test.path {
				t.Errorf("first diagnostic = %s in %q, want %s in %s", diagnostic.Code(), diagnostic.Path(), test.code, test.path)
			}
		})
	}
}
