package pipeline_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/pipeline"
	"github.com/jacoelho/xsd/internal/types"
)

func TestPrepareBuildRuntimeContract(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="RootType">
    <xs:sequence>
      <xs:element name="value" type="xs:string"/>
    </xs:sequence>
    <xs:attribute name="id" type="xs:ID" use="required"/>
  </xs:complexType>
  <xs:element name="root" type="tns:RootType"/>
</xs:schema>`

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	prepared, err := pipeline.Prepare(parsed)
	if err != nil {
		t.Fatalf("prepare schema: %v", err)
	}
	if _, err := prepared.BuildRuntime(pipeline.CompileConfig{}); err != nil {
		t.Fatalf("compile prepared schema: %v", err)
	}
}

func TestBuildRuntimeDoesNotMutatePreparedSchema(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:Code"/>
</xs:schema>`

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	prepared, err := pipeline.Prepare(parsed)
	if err != nil {
		t.Fatalf("prepare schema: %v", err)
	}

	orderBefore := append([]types.QName(nil), prepared.GlobalElementOrder()...)

	rt1, err := prepared.BuildRuntime(pipeline.CompileConfig{})
	if err != nil {
		t.Fatalf("first compile prepared schema: %v", err)
	}
	rt2, err := prepared.BuildRuntime(pipeline.CompileConfig{})
	if err != nil {
		t.Fatalf("second compile prepared schema: %v", err)
	}

	if !equalQNameSlices(orderBefore, prepared.GlobalElementOrder()) {
		t.Fatalf("prepared element order mutated")
	}
	if rt1.BuildHash != rt2.BuildHash {
		t.Fatalf("runtime build hash mismatch across repeated compile: %d != %d", rt1.BuildHash, rt2.BuildHash)
	}
}

func TestPrepareIsolatedFromInputMapMutation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	prepared, err := pipeline.Prepare(parsed)
	if err != nil {
		t.Fatalf("prepare schema: %v", err)
	}
	delete(parsed.ElementDecls, types.QName{Local: "root"})
	parsed.GlobalDecls = nil

	if _, err := prepared.BuildRuntime(pipeline.CompileConfig{}); err != nil {
		t.Fatalf("compile prepared schema after input mutation: %v", err)
	}
	if got := prepared.GlobalElementOrder(); len(got) != 1 || got[0].Local != "root" {
		t.Fatalf("prepared order = %v, want root", got)
	}
}

func TestPrepareIsolatedFromInputObjectMutation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:Code"/>
</xs:schema>`

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	prepared, err := pipeline.Prepare(parsed)
	if err != nil {
		t.Fatalf("prepare schema: %v", err)
	}
	rtBefore, err := prepared.BuildRuntime(pipeline.CompileConfig{})
	if err != nil {
		t.Fatalf("compile prepared schema: %v", err)
	}

	codeName := types.QName{Namespace: "urn:test", Local: "Code"}
	codeType, ok := parsed.TypeDefs[codeName].(*types.SimpleType)
	if !ok || codeType == nil || codeType.Restriction == nil {
		t.Fatalf("expected mutable parsed simple type")
	}
	codeType.Restriction.Base = types.QName{Namespace: types.XSDNamespace, Local: "int"}

	rtAfter, err := prepared.BuildRuntime(pipeline.CompileConfig{})
	if err != nil {
		t.Fatalf("compile prepared schema after input mutation: %v", err)
	}
	if rtAfter.BuildHash != rtBefore.BuildHash {
		t.Fatalf("prepared runtime hash changed after input object mutation: %d != %d", rtAfter.BuildHash, rtBefore.BuildHash)
	}
}

func TestTransformIsolatedFromInputObjectMutation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:Code"/>
</xs:schema>`

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	validated, err := pipeline.Validate(parsed)
	if err != nil {
		t.Fatalf("validate schema: %v", err)
	}
	prepared, err := pipeline.Transform(validated)
	if err != nil {
		t.Fatalf("transform schema: %v", err)
	}
	rtBefore, err := prepared.BuildRuntime(pipeline.CompileConfig{})
	if err != nil {
		t.Fatalf("compile transformed schema: %v", err)
	}

	codeName := types.QName{Namespace: "urn:test", Local: "Code"}
	codeType, ok := parsed.TypeDefs[codeName].(*types.SimpleType)
	if !ok || codeType == nil || codeType.Restriction == nil {
		t.Fatalf("expected mutable parsed simple type")
	}
	codeType.Restriction.Base = types.QName{Namespace: types.XSDNamespace, Local: "int"}

	rtAfter, err := prepared.BuildRuntime(pipeline.CompileConfig{})
	if err != nil {
		t.Fatalf("compile transformed schema after input mutation: %v", err)
	}
	if rtAfter.BuildHash != rtBefore.BuildHash {
		t.Fatalf("transformed runtime hash changed after input object mutation: %d != %d", rtAfter.BuildHash, rtBefore.BuildHash)
	}
}

func TestValidateDeterministicAcrossRepeatedCalls(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:Code"/>
</xs:schema>`

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	validatedA, err := pipeline.Validate(parsed)
	if err != nil {
		t.Fatalf("first validate schema: %v", err)
	}
	preparedA, err := pipeline.Transform(validatedA)
	if err != nil {
		t.Fatalf("first transform schema: %v", err)
	}
	rtA, err := preparedA.BuildRuntime(pipeline.CompileConfig{})
	if err != nil {
		t.Fatalf("first compile prepared schema: %v", err)
	}

	validatedB, err := pipeline.Validate(parsed)
	if err != nil {
		t.Fatalf("second validate schema: %v", err)
	}
	preparedB, err := pipeline.Transform(validatedB)
	if err != nil {
		t.Fatalf("second transform schema: %v", err)
	}
	rtB, err := preparedB.BuildRuntime(pipeline.CompileConfig{})
	if err != nil {
		t.Fatalf("second compile prepared schema: %v", err)
	}

	if rtA.BuildHash != rtB.BuildHash {
		t.Fatalf("runtime build hash mismatch across repeated validate/transform: %d != %d", rtA.BuildHash, rtB.BuildHash)
	}
}

func equalQNameSlices(a, b []types.QName) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
