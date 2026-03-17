package xsd_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
	xsderrors "github.com/jacoelho/xsd/errors"
)

func TestLoadWithOptionsAllowsMissingImportLocation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:main"
           elementFormDefault="qualified">
  <xs:import namespace="urn:dep"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"main.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	if _, err := xsd.LoadWithOptions(fsys, "main.xsd", xsd.NewLoadOptions()); err == nil {
		t.Fatal("LoadWithOptions() err = nil, want missing import error")
	}

	opts := xsd.NewLoadOptions().WithAllowMissingImportLocations(true)
	if _, err := xsd.LoadWithOptions(fsys, "main.xsd", opts); err != nil {
		t.Fatalf("LoadWithOptions() error = %v", err)
	}
}

func TestSchemaSetCompileAppliesRuntimeOptions(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	runtimeOpts := xsd.NewRuntimeOptions().WithInstanceMaxDepth(-1)
	loadOpts := xsd.NewLoadOptions().WithRuntimeOptions(runtimeOpts)
	set := xsd.NewSchemaSet().WithLoadOptions(loadOpts)
	if err := set.AddFS(fsys, "schema.xsd"); err != nil {
		t.Fatalf("AddFS() error = %v", err)
	}
	if _, err := set.Compile(); err == nil {
		t.Fatal("Compile() error = nil, want invalid runtime options error")
	}
}

func TestLoadOptionsRejectsMixedRuntimeConfiguration(t *testing.T) {
	loadOptionsType := reflect.TypeOf(xsd.NewLoadOptions())
	forbiddenRuntimeSetters := []string{
		"WithMaxDFAStates",
		"WithMaxOccursLimit",
		"WithInstanceMaxDepth",
		"WithInstanceMaxAttrs",
		"WithInstanceMaxTokenSize",
		"WithInstanceMaxQNameInternEntries",
	}
	for _, method := range forbiddenRuntimeSetters {
		if _, ok := loadOptionsType.MethodByName(method); ok {
			t.Fatalf("LoadOptions should not export runtime setter %s; use WithRuntimeOptions", method)
		}
	}
	if _, ok := loadOptionsType.MethodByName("WithRuntimeOptions"); !ok {
		t.Fatal("LoadOptions should expose WithRuntimeOptions bridge")
	}

	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	opts := xsd.NewLoadOptions().WithRuntimeOptions(
		xsd.NewRuntimeOptions().WithInstanceMaxDepth(-1),
	)
	set := xsd.NewSchemaSet().WithLoadOptions(opts)
	if err := set.AddFS(fsys, "schema.xsd"); err != nil {
		t.Fatalf("AddFS() error = %v", err)
	}
	if _, err := set.Compile(); err == nil {
		t.Fatal("Compile() error = nil, want runtime options validation error")
	}
}

func TestLoadFileWithOptionsResolvesRelativeImport(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := writeTempFile(t, tempDir, "schemas/main.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:main"
           xmlns:tns="urn:main"
           xmlns:dep="urn:dep"
           elementFormDefault="qualified">
  <xs:import namespace="urn:dep" schemaLocation="deps/dep.xsd"/>
  <xs:element name="root" type="dep:codeType"/>
</xs:schema>`)
	writeTempFile(t, tempDir, "schemas/deps/dep.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:dep"
           xmlns:tns="urn:dep">
  <xs:simpleType name="codeType">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`)

	schema, err := xsd.LoadFileWithOptions(schemaPath, xsd.NewLoadOptions())
	if err != nil {
		t.Fatalf("LoadFileWithOptions() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<root xmlns="urn:main">ok</root>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadFileWithOptionsMissingFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.xsd")

	_, err := xsd.LoadFileWithOptions(missingPath, xsd.NewLoadOptions())
	if err == nil {
		t.Fatal("LoadFileWithOptions() error = nil, want missing file error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadFileWithOptions() error = %v, want fs.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), "load schema missing.xsd") {
		t.Fatalf("LoadFileWithOptions() error = %v, want wrapped schema location", err)
	}
}

func TestLoadFileWithOptionsAppliesRuntimeOptions(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := writeTempFile(t, tempDir, "schema.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)

	schema, err := xsd.LoadFileWithOptions(
		schemaPath,
		xsd.NewLoadOptions().WithRuntimeOptions(
			xsd.NewRuntimeOptions().WithInstanceMaxTokenSize(8),
		),
	)
	if err != nil {
		t.Fatalf("LoadFileWithOptions() error = %v", err)
	}

	err = schema.Validate(strings.NewReader(`<root>abcdefghijklmnopqrstuvwxyz</root>`))
	requireContainsViolationCode(t, err, xsderrors.ErrXMLParse)
}

func writeTempFile(t *testing.T, root, name, content string) string {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
