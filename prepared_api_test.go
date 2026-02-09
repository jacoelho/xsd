package xsd_test

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func TestPrepareAndBuild(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	prepared, err := xsd.PrepareWithOptions(fsys, "schema.xsd", xsd.NewLoadOptions())
	if err != nil {
		t.Fatalf("PrepareWithOptions() error = %v", err)
	}
	if prepared == nil {
		t.Fatal("PrepareWithOptions() returned nil")
	}

	order := slices.Collect(prepared.GlobalElementOrderSeq())
	if len(order) != 1 {
		t.Fatalf("GlobalElementOrderSeq() length = %d, want 1", len(order))
	}
	if order[0].Local != "root" {
		t.Fatalf("GlobalElementOrderSeq()[0].Local = %q, want root", order[0].Local)
	}

	schema, err := prepared.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if schema == nil {
		t.Fatal("Build() returned nil")
	}

	doc := `<root xmlns="urn:test">ok</root>`
	if err := schema.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPreparedSchemaBuildWithOptionsRejectsInvalidRuntimeLimits(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	prepared, err := xsd.PrepareWithOptions(fsys, "schema.xsd", xsd.NewLoadOptions())
	if err != nil {
		t.Fatalf("PrepareWithOptions() error = %v", err)
	}

	opts := xsd.NewRuntimeOptions().WithInstanceMaxDepth(-1)
	if _, err := prepared.BuildWithOptions(opts); err == nil {
		t.Fatal("BuildWithOptions() error = nil, want invalid runtime options error")
	}
}

func TestLoadWithOptionsRejectsInvalidSchemaLimits(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	opts := xsd.NewLoadOptions().WithSchemaMaxDepth(-1)
	if _, err := xsd.LoadWithOptions(fsys, "schema.xsd", opts); err == nil {
		t.Fatal("LoadWithOptions() error = nil, want invalid schema options error")
	}
}

func TestQNameCompatibility(t *testing.T) {
	apiQName := xsd.QName{Namespace: "urn:test", Local: "root"}
	streamQName := xmlstream.QName{Namespace: apiQName.Namespace, Local: apiQName.Local}

	if got, want := apiQName.String(), "{urn:test}root"; got != want {
		t.Fatalf("xsd.QName.String() = %q, want %q", got, want)
	}
	if got, want := streamQName.String(), "{urn:test}root"; got != want {
		t.Fatalf("xmlstream.QName.String() = %q, want %q", got, want)
	}
	if !apiQName.Is("urn:test", "root") {
		t.Fatalf("xsd.QName.Is() = false, want true")
	}
	if !apiQName.HasLocal("root") {
		t.Fatalf("xsd.QName.HasLocal() = false, want true")
	}
}

func TestValidateFSFile(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	instanceXML := `<root xmlns="urn:test">ok</root>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
		"doc.xml":    &fstest.MapFile{Data: []byte(instanceXML)},
	}

	schema, err := xsd.LoadWithOptions(fsys, "schema.xsd", xsd.NewLoadOptions())
	if err != nil {
		t.Fatalf("LoadWithOptions() error = %v", err)
	}
	if err := schema.ValidateFSFile(fsys, "doc.xml"); err != nil {
		t.Fatalf("ValidateFSFile() error = %v", err)
	}
}

func TestRuntimeOptionsAppliedThroughPrepareBuild(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	prepared, err := xsd.PrepareWithOptions(fsys, "schema.xsd", xsd.NewLoadOptions())
	if err != nil {
		t.Fatalf("PrepareWithOptions() error = %v", err)
	}

	compileOpts := xsd.NewRuntimeOptions().
		WithMaxDFAStates(512).
		WithMaxOccursLimit(4096)
	tightOpts := compileOpts.WithInstanceMaxDepth(4)
	looseOpts := compileOpts.WithInstanceMaxDepth(64)

	tightSchema, err := prepared.BuildWithOptions(tightOpts)
	if err != nil {
		t.Fatalf("BuildWithOptions(tight) error = %v", err)
	}
	looseSchema, err := prepared.BuildWithOptions(looseOpts)
	if err != nil {
		t.Fatalf("BuildWithOptions(loose) error = %v", err)
	}

	if tightSchema == nil {
		t.Fatal("tight schema should be non-nil")
	}
	if looseSchema == nil {
		t.Fatal("loose schema should be non-nil")
	}

	doc := deepAnyTypeDocument(8)
	if err := tightSchema.Validate(strings.NewReader(doc)); err == nil {
		t.Fatal("tight runtime options should reject deep document")
	}
	if err := looseSchema.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("loose runtime options should accept deep document: %v", err)
	}
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
