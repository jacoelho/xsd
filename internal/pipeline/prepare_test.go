package pipeline

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestPrepareBuildsSemanticArtifacts(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	prepared, err := Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared == nil {
		t.Fatal("Prepare() returned nil")
	}
	rt, err := prepared.BuildRuntime(CompileConfig{})
	if err != nil {
		t.Fatalf("BuildRuntime() error = %v", err)
	}
	if rt == nil {
		t.Fatal("BuildRuntime() returned nil")
	}
}

func TestPrepareDefersRuntimeValidatorCompilation(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="att" fixed="123" />
  <xs:element name="doc">
    <xs:complexType>
      <xs:attribute ref="att" fixed="123"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	prepared, err := Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared == nil {
		t.Fatal("Prepare() returned nil")
	}
}

func TestPrepareDoesNotMutateInputSchema(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="number" type="xs:integer"/>
    </xs:complexType>
    <xs:key name="rootKey">
      <xs:selector xpath="."/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := sch.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if root == nil || len(root.Constraints) == 0 || len(root.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field")
	}
	before := root.Constraints[0].Fields[0].ResolvedType
	if before != nil {
		t.Fatalf("expected unresolved field before Prepare(), got %v", before.Name())
	}

	if _, err := Prepare(sch); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	rootAfter := sch.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if rootAfter == nil || len(rootAfter.Constraints) == 0 || len(rootAfter.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field after Prepare()")
	}
	after := rootAfter.Constraints[0].Fields[0].ResolvedType
	if after != nil {
		t.Fatalf("expected input schema field to remain unresolved after Prepare(), got %v", after.Name())
	}
}

func TestPrepareReturnsArtifactsWithoutMutatingInputSchema(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="number" type="xs:integer"/>
    </xs:complexType>
    <xs:key name="rootKey">
      <xs:selector xpath="."/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := sch.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if root == nil || len(root.Constraints) == 0 || len(root.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field")
	}
	before := root.Constraints[0].Fields[0].ResolvedType
	if before != nil {
		t.Fatalf("expected unresolved field before Prepare(), got %v", before.Name())
	}

	prepared, err := Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared == nil {
		t.Fatal("Prepare() returned nil")
	}

	rootAfter := sch.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if rootAfter == nil || len(rootAfter.Constraints) == 0 || len(rootAfter.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field after Prepare()")
	}
	after := rootAfter.Constraints[0].Fields[0].ResolvedType
	if after != nil {
		t.Fatalf("expected input schema field to remain unresolved after Prepare(), got %v", after.Name())
	}
}

func TestPrepareBuildsRuntimeFromArtifacts(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	prepared, err := Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	rt, err := prepared.BuildRuntime(CompileConfig{})
	if err != nil {
		t.Fatalf("BuildRuntime() error = %v", err)
	}
	if rt == nil {
		t.Fatal("BuildRuntime() returned nil")
	}
}

func TestPrepareRejectsNilSchema(t *testing.T) {
	if _, err := Prepare(nil); err == nil {
		t.Fatal("Prepare(nil) expected error")
	}
}

func TestPrepareOwnedRejectsNilSchema(t *testing.T) {
	if _, err := PrepareOwned(nil); err == nil {
		t.Fatal("PrepareOwned(nil) expected error")
	}
}

func TestPrepareOwnedMutatesInputSchema(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="number" type="xs:integer"/>
    </xs:complexType>
    <xs:key name="rootKey">
      <xs:selector xpath="."/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := sch.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if root == nil || len(root.Constraints) == 0 || len(root.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field")
	}
	if got := root.Constraints[0].Fields[0].ResolvedType; got != nil {
		t.Fatalf("expected unresolved field before PrepareOwned(), got %v", got.Name())
	}

	if _, err := PrepareOwned(sch); err != nil {
		t.Fatalf("PrepareOwned() error = %v", err)
	}

	rootAfter := sch.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if rootAfter == nil || len(rootAfter.Constraints) == 0 || len(rootAfter.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field after PrepareOwned()")
	}
	if got := rootAfter.Constraints[0].Fields[0].ResolvedType; got == nil {
		t.Fatal("expected input schema field to be resolved after PrepareOwned()")
	}
}

func TestPreparedSchemaBuildRuntimeRejectsNilPreparedSchema(t *testing.T) {
	var prepared *PreparedSchema
	if _, err := prepared.BuildRuntime(CompileConfig{}); err == nil {
		t.Fatal("BuildRuntime() expected error for nil prepared schema")
	}
}

func TestPreparedSchemaGlobalElementOrderSeqIsDeterministic(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="xs:string"/>
  <xs:element name="b" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	prepared, err := Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	first := slices.Collect(prepared.GlobalElementOrderSeq())
	if len(first) != 2 {
		t.Fatalf("GlobalElementOrderSeq() length = %d, want 2", len(first))
	}

	second := slices.Collect(prepared.GlobalElementOrderSeq())
	if len(second) != 2 {
		t.Fatalf("GlobalElementOrderSeq() second length = %d, want 2", len(second))
	}
	if !slices.Equal(first, second) {
		t.Fatalf("GlobalElementOrderSeq() changed between iterations: first=%v second=%v", first, second)
	}
}
