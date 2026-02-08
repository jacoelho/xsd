package xsd

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestCompilationPipelineLoadAndParse(t *testing.T) {
	fsys := fstest.MapFS{
		"main.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}

	p, _, err := newCompilationPipeline(fsys, "main.xsd", LoadOptions{})
	if err != nil {
		t.Fatalf("newCompilationPipeline() error = %v", err)
	}
	loader, err := p.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loader == nil {
		t.Fatal("Load() returned nil loader")
	}

	parsed, err := p.Parse(loader)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed == nil || parsed.schema == nil {
		t.Fatal("Parse() returned nil schema")
	}
	foundRoot := false
	for qname := range parsed.schema.ElementDecls {
		if qname.Local == "root" {
			foundRoot = true
			break
		}
	}
	if !foundRoot {
		t.Fatalf("Parse() result missing root element, got %v", parsed.schema.ElementDecls)
	}
}

func TestCompilationPipelineParseRejectsNilLoader(t *testing.T) {
	p, _, err := newCompilationPipeline(fstest.MapFS{}, "main.xsd", LoadOptions{})
	if err != nil {
		t.Fatalf("newCompilationPipeline() error = %v", err)
	}
	_, err = p.Parse(nil)
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "nil schema loader") {
		t.Fatalf("Parse() error = %v, want nil schema loader", err)
	}
}

func TestCompilationPipelineLoadRejectsNilFS(t *testing.T) {
	p, _, err := newCompilationPipeline(nil, "main.xsd", LoadOptions{})
	if err != nil {
		t.Fatalf("newCompilationPipeline() error = %v", err)
	}
	_, err = p.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "nil fs") {
		t.Fatalf("Load() error = %v, want nil fs", err)
	}
}

func TestCompilationPipelinePrepareRejectsNilParsed(t *testing.T) {
	p, _, err := newCompilationPipeline(fstest.MapFS{}, "main.xsd", LoadOptions{})
	if err != nil {
		t.Fatalf("newCompilationPipeline() error = %v", err)
	}
	_, err = p.Prepare(nil)
	if err == nil {
		t.Fatal("Prepare() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "nil parsed schema") {
		t.Fatalf("Prepare() error = %v, want nil parsed schema", err)
	}
}

func TestCompilationPipelineCompileRejectsNilPrepared(t *testing.T) {
	p, _, err := newCompilationPipeline(fstest.MapFS{}, "main.xsd", LoadOptions{})
	if err != nil {
		t.Fatalf("newCompilationPipeline() error = %v", err)
	}
	_, err = p.Compile(nil)
	if err == nil {
		t.Fatal("Compile() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "nil prepared schema") {
		t.Fatalf("Compile() error = %v, want nil prepared schema", err)
	}
}

func TestCompilationPipelineRunBuildsRuntime(t *testing.T) {
	fsys := fstest.MapFS{
		"main.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}

	p, _, err := newCompilationPipeline(fsys, "main.xsd", LoadOptions{})
	if err != nil {
		t.Fatalf("newCompilationPipeline() error = %v", err)
	}
	rt, err := p.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if rt == nil {
		t.Fatal("Run() returned nil runtime schema")
	}
}

func TestCompilationPipelinePrepareRejectsInvalidParsedSchema(t *testing.T) {
	fsys := fstest.MapFS{
		"main.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="MissingType"/>
</xs:schema>`)},
	}

	p, _, err := newCompilationPipeline(fsys, "main.xsd", LoadOptions{})
	if err != nil {
		t.Fatalf("newCompilationPipeline() error = %v", err)
	}
	loaded, err := p.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	parsed, err := p.Parse(loaded)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if _, err := p.Prepare(parsed); err == nil {
		t.Fatal("Prepare() expected error for invalid schema")
	}
}
