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
)

func TestCompileFSAppliesSourceConfig(t *testing.T) {
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

	if _, err := xsd.CompileFS(fsys, "main.xsd", xsd.CompileConfig{}); err == nil {
		t.Fatal("CompileFS() error = nil, want missing import error")
	}

	schema, err := xsd.CompileFS(fsys, "main.xsd", xsd.CompileConfig{
		Source: xsd.SourceConfig{AllowMissingImportLocations: true},
	})
	if err != nil {
		t.Fatalf("CompileFS() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<root xmlns="urn:main">ok</root>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestCompileFSRejectsInvalidSourceConfig(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}

	_, err := xsd.CompileFS(fsys, "schema.xsd", xsd.CompileConfig{
		Source: xsd.SourceConfig{XML: xsd.XMLConfig{MaxDepth: -1}},
	})
	if err == nil {
		t.Fatal("CompileFS() error = nil, want invalid source config error")
	}
	_ = requireClassifiedError(t, err, xsd.KindCaller, xsd.ErrCaller)
}

func TestCompilePublicErrorsAreClassified(t *testing.T) {
	tests := []struct {
		name      string
		run       func() error
		wantKind  xsd.ErrorKind
		wantCode  xsd.ErrorCode
		wantCause error
	}{
		{
			name:     "nil fs",
			run:      func() error { _, err := xsd.CompileFS(nil, "schema.xsd", xsd.CompileConfig{}); return err },
			wantKind: xsd.KindCaller,
			wantCode: xsd.ErrCaller,
		},
		{
			name:     "empty location",
			run:      func() error { _, err := xsd.CompileFS(fstest.MapFS{}, "  ", xsd.CompileConfig{}); return err },
			wantKind: xsd.KindCaller,
			wantCode: xsd.ErrCaller,
		},
		{
			name: "missing file",
			run: func() error {
				_, err := xsd.CompileFile(filepath.Join(t.TempDir(), "missing.xsd"), xsd.CompileConfig{})
				return err
			},
			wantKind:  xsd.KindIO,
			wantCode:  xsd.ErrIO,
			wantCause: fs.ErrNotExist,
		},
		{
			name: "malformed schema",
			run: func() error {
				_, err := xsd.CompileFS(fstest.MapFS{
					"schema.xsd": &fstest.MapFile{Data: []byte(`<xs:schema`)},
				}, "schema.xsd", xsd.CompileConfig{})
				return err
			},
			wantKind: xsd.KindSchema,
			wantCode: xsd.ErrSchemaParse,
		},
		{
			name: "unresolved type",
			run: func() error {
				_, err := xsd.CompileFS(fstest.MapFS{
					"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:element name="root" type="tns:Missing"/>
</xs:schema>`)},
				}, "schema.xsd", xsd.CompileConfig{})
				return err
			},
			wantKind: xsd.KindSchema,
			wantCode: xsd.ErrSchemaSemantic,
		},
		{
			name: "compile sources no roots",
			run: func() error {
				_, err := xsd.NewCompiler(xsd.CompileConfig{}).CompileSources(nil)
				return err
			},
			wantKind: xsd.KindCaller,
			wantCode: xsd.ErrCaller,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			_ = requireClassifiedError(t, err, tt.wantKind, tt.wantCode)
			if tt.wantCause != nil && !errors.Is(err, tt.wantCause) {
				t.Fatalf("error = %v, want cause %v", err, tt.wantCause)
			}
		})
	}
}

func TestCompilerCompileSourcesMultipleRoots(t *testing.T) {
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

	schema, err := xsd.NewCompiler(xsd.CompileConfig{}).CompileSources([]xsd.Source{
		{FS: fsysA, Path: "a.xsd"},
		{FS: fsysB, Path: "b.xsd"},
	})
	if err != nil {
		t.Fatalf("CompileSources() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootA xmlns="urn:test">ok</rootA>`)); err != nil {
		t.Fatalf("Validate(rootA) error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootB xmlns="urn:test">ok</rootB>`)); err != nil {
		t.Fatalf("Validate(rootB) error = %v", err)
	}
}

func TestCompilerCompileSourcesDeduplicatesSharedInclude(t *testing.T) {
	fsys := fstest.MapFS{
		"a.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="rootA" type="xs:string"/>
</xs:schema>`)},
		"b.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="rootB" type="xs:string"/>
</xs:schema>`)},
		"common.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="common" type="xs:string"/>
</xs:schema>`)},
	}

	schema, err := xsd.NewCompiler(xsd.CompileConfig{}).CompileSources([]xsd.Source{
		{FS: fsys, Path: "a.xsd"},
		{FS: fsys, Path: "b.xsd"},
	})
	if err != nil {
		t.Fatalf("CompileSources() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootA xmlns="urn:test">ok</rootA>`)); err != nil {
		t.Fatalf("Validate(rootA) error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootB xmlns="urn:test">ok</rootB>`)); err != nil {
		t.Fatalf("Validate(rootB) error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<common xmlns="urn:test">ok</common>`)); err != nil {
		t.Fatalf("Validate(common) error = %v", err)
	}
}

func TestCompilerCompileSourcesSameLocationDistinctFS(t *testing.T) {
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

	schema, err := xsd.NewCompiler(xsd.CompileConfig{}).CompileSources([]xsd.Source{
		{FS: fsysA, Path: "schema.xsd"},
		{FS: fsysB, Path: "schema.xsd"},
	})
	if err != nil {
		t.Fatalf("CompileSources() error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootA xmlns="urn:test">ok</rootA>`)); err != nil {
		t.Fatalf("Validate(rootA) error = %v", err)
	}
	if err := schema.Validate(strings.NewReader(`<rootB xmlns="urn:test">ok</rootB>`)); err != nil {
		t.Fatalf("Validate(rootB) error = %v", err)
	}
}

func TestCompilerCompileSourcesSameLocationConflict(t *testing.T) {
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

	_, err := xsd.NewCompiler(xsd.CompileConfig{}).CompileSources([]xsd.Source{
		{FS: fsysA, Path: "schema.xsd"},
		{FS: fsysB, Path: "schema.xsd"},
	})
	if err == nil {
		t.Fatal("CompileSources() error = nil, want duplicate declaration error")
	}
	if !strings.Contains(err.Error(), "duplicate element") {
		t.Fatalf("CompileSources() error = %v, want duplicate element", err)
	}
}

func TestCompilerCompileSourcesWithoutRoots(t *testing.T) {
	if _, err := xsd.NewCompiler(xsd.CompileConfig{}).CompileSources(nil); err == nil {
		t.Fatal("CompileSources() error = nil, want no roots error")
	} else {
		_ = requireClassifiedError(t, err, xsd.KindCaller, xsd.ErrCaller)
	}
}

func TestSchemaNewValidatorAppliesValidateConfig(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)},
	}

	schema, err := xsd.CompileFS(fsys, "schema.xsd", xsd.CompileConfig{})
	if err != nil {
		t.Fatalf("CompileFS() error = %v", err)
	}

	tight, err := schema.NewValidator(xsd.ValidateConfig{XML: xsd.XMLConfig{MaxDepth: 4}})
	if err != nil {
		t.Fatalf("NewValidator(tight) error = %v", err)
	}
	loose, err := schema.NewValidator(xsd.ValidateConfig{XML: xsd.XMLConfig{MaxDepth: 64}})
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

func TestSchemaNewValidatorRejectsInvalidValidateConfig(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}

	schema, err := xsd.CompileFS(fsys, "schema.xsd", xsd.CompileConfig{})
	if err != nil {
		t.Fatalf("CompileFS() error = %v", err)
	}

	if _, err := schema.NewValidator(xsd.ValidateConfig{XML: xsd.XMLConfig{MaxDepth: -1}}); err == nil {
		t.Fatal("NewValidator() error = nil, want invalid validate config error")
	}
}

func TestSchemaNotLoadedErrorIsCallerError(t *testing.T) {
	err := new(xsd.Schema).Validate(strings.NewReader(`<root/>`))
	if err == nil {
		t.Fatal("Validate() error = nil, want schema-not-loaded error")
	}
	if kind, ok := xsd.KindOf(err); !ok || kind != xsd.KindCaller {
		t.Fatalf("KindOf() = %v, %v; want KindCaller", kind, ok)
	}
	if !errors.Is(err, xsd.Error{Kind: xsd.KindCaller, Code: xsd.ErrSchemaNotLoaded}) {
		t.Fatalf("errors.Is(%v, schema-not-loaded caller target) = false", err)
	}
	if _, ok := xsd.AsValidations(err); ok {
		t.Fatal("AsValidations() ok = true, want caller error outside validation list")
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

	schema, err := xsd.CompileFile(schemaPath, xsd.CompileConfig{})
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

	schema, err := xsd.CompileFile(linkPath, xsd.CompileConfig{})
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

	_, err := xsd.CompileFile(schemaPath, xsd.CompileConfig{})
	if err == nil {
		t.Fatal("CompileFile() error = nil, want symlink escape rejection")
	}
	if !strings.Contains(err.Error(), "compile schema main.xsd") {
		t.Fatalf("CompileFile() error = %v, want wrapped schema location", err)
	}
}

func TestCompileFileMissingFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.xsd")

	_, err := xsd.CompileFile(missingPath, xsd.CompileConfig{})
	if err == nil {
		t.Fatal("CompileFile() error = nil, want missing file error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("CompileFile() error = %v, want fs.ErrNotExist", err)
	}
	_ = requireClassifiedError(t, err, xsd.KindIO, xsd.ErrIO)
	if !strings.Contains(err.Error(), "compile schema missing.xsd") {
		t.Fatalf("CompileFile() error = %v, want wrapped schema location", err)
	}
}

func TestValidatorValidateFileAppliesValidateConfig(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := writePhaseTempFile(t, tempDir, "schema.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	docPath := writePhaseTempFile(t, tempDir, "document.xml", `<root>abcdefghijklmnopqrstuvwxyz</root>`)

	schema, err := xsd.CompileFile(schemaPath, xsd.CompileConfig{})
	if err != nil {
		t.Fatalf("CompileFile() error = %v", err)
	}

	v, err := schema.NewValidator(xsd.ValidateConfig{XML: xsd.XMLConfig{MaxTokenSize: 8}})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}
	requireContainsViolationCode(t, v.ValidateFile(docPath), xsd.ErrXMLParse)
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
	for range depth - 1 {
		b.WriteString(`<n>`)
	}
	b.WriteString(`v`)
	for range depth - 1 {
		b.WriteString(`</n>`)
	}
	b.WriteString(`</root>`)
	return b.String()
}
