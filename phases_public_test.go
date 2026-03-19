package xsd_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
	xsderrors "github.com/jacoelho/xsd/errors"
)

func TestCompileAppliesSourceOptions(t *testing.T) {
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

	if _, err := xsd.Compile(fsys, "main.xsd", xsd.NewSourceOptions(), xsd.NewBuildOptions()); err == nil {
		t.Fatal("Compile() error = nil, want missing import error")
	}

	sourceOpts := xsd.NewSourceOptions().WithAllowMissingImportLocations(true)
	schema, err := xsd.Compile(fsys, "main.xsd", sourceOpts, xsd.NewBuildOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<root xmlns="urn:main">ok</root>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestCompileRejectsInvalidSourceOptions(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}

	if _, err := xsd.Compile(
		fsys,
		"schema.xsd",
		xsd.NewSourceOptions().WithSchemaMaxDepth(-1),
		xsd.NewBuildOptions(),
	); err == nil {
		t.Fatal("Compile() error = nil, want invalid schema options error")
	}
}

func TestSourceSetPrepareBuild(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}

	set := xsd.NewSourceSet().WithSourceOptions(xsd.NewSourceOptions())
	if err := set.AddFS(fsys, "schema.xsd"); err != nil {
		t.Fatalf("AddFS() error = %v", err)
	}

	prepared, err := set.Prepare()
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	schema, err := prepared.Build(xsd.NewBuildOptions().WithMaxDFAStates(512))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	v, err := schema.NewValidator(xsd.NewValidateOptions())
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}
	if err := v.Validate(strings.NewReader(`<root xmlns="urn:test">ok</root>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestSourceSetBuildMultipleRoots(t *testing.T) {
	fsysA := fstest.MapFS{
		"a.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="rootA" type="xs:string"/>
</xs:schema>`)},
	}
	fsysB := fstest.MapFS{
		"b.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="rootB" type="xs:string"/>
</xs:schema>`)},
	}

	set := xsd.NewSourceSet()
	if err := set.AddFS(fsysA, "a.xsd"); err != nil {
		t.Fatalf("AddFS(a) error = %v", err)
	}
	if err := set.AddFS(fsysB, "b.xsd"); err != nil {
		t.Fatalf("AddFS(b) error = %v", err)
	}

	schema, err := set.Build(xsd.NewBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootA xmlns="urn:test">ok</rootA>`)); err != nil {
		t.Fatalf("Validate(rootA) error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootB xmlns="urn:test">ok</rootB>`)); err != nil {
		t.Fatalf("Validate(rootB) error = %v", err)
	}
}

func TestSourceSetBuildMultipleRootsSameLocationDistinctFS(t *testing.T) {
	fsysA := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="rootA" type="xs:string"/>
</xs:schema>`)},
	}
	fsysB := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="rootB" type="xs:string"/>
</xs:schema>`)},
	}

	set := xsd.NewSourceSet()
	if err := set.AddFS(fsysA, "schema.xsd"); err != nil {
		t.Fatalf("AddFS(a) error = %v", err)
	}
	if err := set.AddFS(fsysB, "schema.xsd"); err != nil {
		t.Fatalf("AddFS(b) error = %v", err)
	}

	schema, err := set.Build(xsd.NewBuildOptions())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootA xmlns="urn:test">ok</rootA>`)); err != nil {
		t.Fatalf("Validate(rootA) error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootB xmlns="urn:test">ok</rootB>`)); err != nil {
		t.Fatalf("Validate(rootB) error = %v", err)
	}
}

func TestSourceSetBuildMultipleRootsSameLocationConflict(t *testing.T) {
	fsysA := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="shared" type="xs:string"/>
</xs:schema>`)},
	}
	fsysB := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="shared" type="xs:int"/>
</xs:schema>`)},
	}

	set := xsd.NewSourceSet()
	if err := set.AddFS(fsysA, "schema.xsd"); err != nil {
		t.Fatalf("AddFS(a) error = %v", err)
	}
	if err := set.AddFS(fsysB, "schema.xsd"); err != nil {
		t.Fatalf("AddFS(b) error = %v", err)
	}

	_, err := set.Build(xsd.NewBuildOptions())
	if err == nil {
		t.Fatal("Build() error = nil, want duplicate declaration error")
	}
	if !strings.Contains(err.Error(), "duplicate element") {
		t.Fatalf("Build() error = %v, want duplicate element", err)
	}
}

func TestSourceSetBuildWithoutRoots(t *testing.T) {
	set := xsd.NewSourceSet()
	if _, err := set.Build(xsd.NewBuildOptions()); err == nil {
		t.Fatal("Build() error = nil, want no roots error")
	}
}

func TestSchemaNewValidatorAppliesValidateOptions(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)},
	}

	schema, err := xsd.Compile(fsys, "schema.xsd", xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	tight, err := schema.NewValidator(xsd.NewValidateOptions().WithInstanceMaxDepth(4))
	if err != nil {
		t.Fatalf("NewValidator(tight) error = %v", err)
	}
	loose, err := schema.NewValidator(xsd.NewValidateOptions().WithInstanceMaxDepth(64))
	if err != nil {
		t.Fatalf("NewValidator(loose) error = %v", err)
	}

	doc := deepAnyTypeDocument(8)
	if err := tight.Validate(strings.NewReader(doc)); err == nil {
		t.Fatal("tight validator should reject deep document")
	}
	if err := loose.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("loose validator should accept deep document: %v", err)
	}
}

func TestSchemaNewValidatorRejectsInvalidValidateOptions(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}

	schema, err := xsd.Compile(fsys, "schema.xsd", xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if _, err := schema.NewValidator(xsd.NewValidateOptions().WithInstanceMaxDepth(-1)); err == nil {
		t.Fatal("NewValidator() error = nil, want invalid validate options error")
	}
}

func TestCompileFileResolvesRelativeImport(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := writePhaseTempFile(t, tempDir, "schemas/main.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:main"
           xmlns:tns="urn:main"
           xmlns:dep="urn:dep"
           elementFormDefault="qualified">
  <xs:import namespace="urn:dep" schemaLocation="deps/dep.xsd"/>
  <xs:element name="root" type="dep:codeType"/>
</xs:schema>`)
	writePhaseTempFile(t, tempDir, "schemas/deps/dep.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:dep"
           xmlns:tns="urn:dep">
  <xs:simpleType name="codeType">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`)

	schema, err := xsd.CompileFile(schemaPath, xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err != nil {
		t.Fatalf("CompileFile() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<root xmlns="urn:main">ok</root>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestCompileFileAllowsSymlinkRootAndResolvesNestedImportsInRequestedTree(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := writePhaseTempFile(t, tempDir, "outside/main.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:main"
           xmlns:tns="urn:main"
           xmlns:dep="urn:dep"
           elementFormDefault="qualified">
  <xs:import namespace="urn:dep" schemaLocation="deps/dep.xsd"/>
  <xs:element name="root" type="dep:codeType"/>
</xs:schema>`)
	writePhaseTempFile(t, tempDir, "links/deps/dep.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:dep"
           xmlns:tns="urn:dep">
  <xs:simpleType name="codeType">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`)
	linkPath := writePhaseTempSymlink(t, tempDir, "links/current.xsd", schemaPath)

	schema, err := xsd.CompileFile(linkPath, xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err != nil {
		t.Fatalf("CompileFile() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<root xmlns="urn:main">ok</root>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestCompileFileRejectsSymlinkImportEscape(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := writePhaseTempFile(t, tempDir, "schemas/main.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:main"
           xmlns:tns="urn:main"
           xmlns:dep="urn:dep"
           elementFormDefault="qualified">
  <xs:import namespace="urn:dep" schemaLocation="deps/dep.xsd"/>
  <xs:element name="root" type="dep:codeType"/>
</xs:schema>`)
	outsidePath := writePhaseTempFile(t, tempDir, "outside/dep.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:dep"
           xmlns:tns="urn:dep">
  <xs:simpleType name="codeType">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`)
	writePhaseTempSymlink(t, tempDir, "schemas/deps/dep.xsd", outsidePath)

	_, err := xsd.CompileFile(schemaPath, xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err == nil {
		t.Fatal("CompileFile() error = nil, want symlink escape rejection")
	}
	if !strings.Contains(err.Error(), "compile schema main.xsd") {
		t.Fatalf("CompileFile() error = %v, want wrapped schema location", err)
	}
}

func TestCompileFileMissingFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.xsd")

	_, err := xsd.CompileFile(missingPath, xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err == nil {
		t.Fatal("CompileFile() error = nil, want missing file error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("CompileFile() error = %v, want fs.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), "compile schema missing.xsd") {
		t.Fatalf("CompileFile() error = %v, want wrapped schema location", err)
	}
}

func TestValidatorValidateFileAppliesValidateOptions(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := writePhaseTempFile(t, tempDir, "schema.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	docPath := writePhaseTempFile(t, tempDir, "document.xml", `<root>abcdefghijklmnopqrstuvwxyz</root>`)

	schema, err := xsd.CompileFile(schemaPath, xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err != nil {
		t.Fatalf("CompileFile() error = %v", err)
	}

	v, err := schema.NewValidator(xsd.NewValidateOptions().WithInstanceMaxTokenSize(8))
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}
	requireContainsViolationCode(t, v.ValidateFile(docPath), xsderrors.ErrXMLParse)
}

func writePhaseTempFile(t *testing.T, root, name, content string) string {
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

func writePhaseTempSymlink(t *testing.T, root, name, target string) string {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	linkTarget, err := filepath.Rel(filepath.Dir(path), target)
	if err != nil {
		t.Fatalf("filepath.Rel(%q, %q) error = %v", filepath.Dir(path), target, err)
	}
	if err := os.Symlink(linkTarget, path); err != nil {
		if errors.Is(err, fs.ErrPermission) || errors.Is(err, errors.ErrUnsupported) {
			t.Skipf("symlink creation unavailable in test environment: %v", err)
		}
		t.Fatalf("Symlink(%q, %q) error = %v", linkTarget, path, err)
	}
	return path
}

func deepAnyTypeDocument(depth int) string {
	if depth < 1 {
		depth = 1
	}
	var b strings.Builder
	b.WriteString(`<root>`)
	for i := 1; i < depth; i++ {
		b.WriteString(`<n>`)
	}
	b.WriteString(`v`)
	for i := 1; i < depth; i++ {
		b.WriteString(`</n>`)
	}
	b.WriteString(`</root>`)
	return b.String()
}
