package xsd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const xsHeader = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:b="urn:b" targetNamespace="urn:a" elementFormDefault="qualified">`

func TestCompileTreatsUnrepresentableReferenceAsUnavailable(t *testing.T) {
	for _, test := range []struct{ name, importAttrs string }{
		{"invalid port", `schemaLocation="http://h:abc/x.xsd"`},
		{"escaped host", `schemaLocation="http://a%41b/x.xsd"`},
		{"invalid port in xml:base", `schemaLocation="x.xsd" xml:base="http://h:abc/"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema := xsHeader + `<xs:import namespace="urn:b" ` + test.importAttrs + `/><xs:element name="r" type="xs:string"/></xs:schema>`
			if _, err := xsd.Compile(xsd.Bytes("a.xsd", []byte(schema))); err != nil {
				t.Fatalf("Compile() error = %v, want nil", err)
			}
		})
	}

	t.Run("malformed escape still fails", func(t *testing.T) {
		schema := xsHeader + `<xs:import namespace="urn:b" schemaLocation="%zz.xsd"/><xs:element name="r" type="xs:string"/></xs:schema>`
		_, err := xsd.Compile(xsd.Bytes("a.xsd", []byte(schema)))
		diagnostic, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || diagnostic.Code() != xsderrors.CodeSchemaReference {
			t.Fatalf("Compile() error = %v, want schema.reference", err)
		}
	})
}

func TestCompileLoadsRepresentableSiblingsThroughBuiltInFallback(t *testing.T) {
	const target = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"><xs:simpleType name="t"><xs:restriction base="xs:int"><xs:maxInclusive value="5"/></xs:restriction></xs:simpleType></xs:schema>`
	for _, test := range []struct{ name, importAttrs, targetPath string }{
		{"relative sibling", `schemaLocation="b.xsd"`, "b.xsd"},
		{"xml:base directory", `schemaLocation="b.xsd" xml:base="sub/"`, "sub/b.xsd"},
		{"percent-escaped sibling", `schemaLocation="b%20c.xsd"`, "b c.xsd"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, filepath.FromSlash(test.targetPath))
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(target), 0o600); err != nil {
				t.Fatal(err)
			}
			schema := xsHeader + `<xs:import namespace="urn:b" ` + test.importAttrs + `/><xs:element name="r" type="b:t"/></xs:schema>`
			root := filepath.Join(dir, "a.xsd")
			if err := os.WriteFile(root, []byte(schema), 0o600); err != nil {
				t.Fatal(err)
			}
			engine, err := xsd.Compile(xsd.File(root))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if err := engine.Validate(strings.NewReader(`<r xmlns="urn:a">3</r>`)); err != nil {
				t.Errorf("Validate(3) error = %v", err)
			}
			if err := engine.Validate(strings.NewReader(`<r xmlns="urn:a">9</r>`)); err == nil {
				t.Error("Validate(9) = nil, want facet violation from the imported type")
			}
		})
	}
}
