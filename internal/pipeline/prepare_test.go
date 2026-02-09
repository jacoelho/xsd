package pipeline

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
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

	root := sch.ElementDecls[types.QName{Namespace: "urn:test", Local: "root"}]
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

	rootAfter := sch.ElementDecls[types.QName{Namespace: "urn:test", Local: "root"}]
	if rootAfter == nil || len(rootAfter.Constraints) == 0 || len(rootAfter.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field after Prepare()")
	}
	after := rootAfter.Constraints[0].Fields[0].ResolvedType
	if after != nil {
		t.Fatalf("expected input schema field to remain unresolved after Prepare(), got %v", after.Name())
	}
}

func TestValidateReturnsArtifactsWithoutMutatingInputSchema(t *testing.T) {
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

	root := sch.ElementDecls[types.QName{Namespace: "urn:test", Local: "root"}]
	if root == nil || len(root.Constraints) == 0 || len(root.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field")
	}
	before := root.Constraints[0].Fields[0].ResolvedType
	if before != nil {
		t.Fatalf("expected unresolved field before Validate(), got %v", before.Name())
	}

	validated, err := Validate(sch)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if validated == nil {
		t.Fatal("Validate() returned nil")
	}

	rootAfter := sch.ElementDecls[types.QName{Namespace: "urn:test", Local: "root"}]
	if rootAfter == nil || len(rootAfter.Constraints) == 0 || len(rootAfter.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field after Validate()")
	}
	after := rootAfter.Constraints[0].Fields[0].ResolvedType
	if after != nil {
		t.Fatalf("expected input schema field to remain unresolved after Validate(), got %v", after.Name())
	}
}

func TestValidateReturnsIndependentResolvedSchema(t *testing.T) {
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

	validated, err := Validate(sch)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if validated == nil || validated.schema == nil {
		t.Fatal("Validate() returned nil artifacts")
	}
	if validated.schema == sch {
		t.Fatal("Validate() should return an independent schema artifact")
	}

	inputRoot := sch.ElementDecls[types.QName{Namespace: "urn:test", Local: "root"}]
	resolvedRoot := validated.schema.ElementDecls[types.QName{Namespace: "urn:test", Local: "root"}]
	if inputRoot == nil || resolvedRoot == nil {
		t.Fatal("expected root element in both schemas")
	}
	if len(inputRoot.Constraints) == 0 || len(inputRoot.Constraints[0].Fields) == 0 {
		t.Fatal("expected input key field")
	}
	if len(resolvedRoot.Constraints) == 0 || len(resolvedRoot.Constraints[0].Fields) == 0 {
		t.Fatal("expected resolved key field")
	}
	if inputRoot.Constraints[0].Fields[0].ResolvedType != nil {
		t.Fatal("input schema should remain unresolved")
	}
	if resolvedRoot.Constraints[0].Fields[0].ResolvedType == nil {
		t.Fatal("validated schema should contain resolved field type")
	}
}

func TestTransformFromValidatedArtifactsBuildsRuntime(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	validated, err := Validate(sch)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	prepared, err := Transform(validated)
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	rt, err := prepared.BuildRuntime(CompileConfig{})
	if err != nil {
		t.Fatalf("BuildRuntime() error = %v", err)
	}
	if rt == nil {
		t.Fatal("BuildRuntime() returned nil")
	}
}

func TestValidateRejectsNilSchema(t *testing.T) {
	if _, err := Validate(nil); err == nil {
		t.Fatal("Validate(nil) expected error")
	}
}

func TestTransformRejectsNilValidatedSchema(t *testing.T) {
	if _, err := Transform(nil); err == nil {
		t.Fatal("Transform(nil) expected error")
	}
}

func TestValidatedSchemaSnapshotIsDefensiveCopy(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	validated, err := Validate(sch)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	first, err := validated.SchemaSnapshot()
	if err != nil {
		t.Fatalf("SchemaSnapshot() error = %v", err)
	}
	delete(first.ElementDecls, types.QName{Local: "root"})
	first.GlobalDecls = nil

	second, err := validated.SchemaSnapshot()
	if err != nil {
		t.Fatalf("SchemaSnapshot() second error = %v", err)
	}
	if _, ok := second.ElementDecls[types.QName{Local: "root"}]; !ok {
		t.Fatal("snapshot mutation leaked into validated artifact")
	}
	if len(second.GlobalDecls) == 0 {
		t.Fatal("snapshot mutation leaked into validated global declarations")
	}
}

func TestValidatedSchemaSnapshotRejectsNilValidatedSchema(t *testing.T) {
	var validated *ValidatedSchema
	if _, err := validated.SchemaSnapshot(); err == nil {
		t.Fatal("SchemaSnapshot() expected error for nil validated schema")
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
