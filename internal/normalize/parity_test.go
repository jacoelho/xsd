package normalize_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complextypeplan"
	"github.com/jacoelho/xsd/internal/loadmerge"
	expnormalize "github.com/jacoelho/xsd/internal/normalize"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/prep"
	"github.com/jacoelho/xsd/internal/runtimeassemble"
)

func TestPrepareOwnedParityWithLegacyPipelineSteps(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:group name="g">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:group>
  <xs:complexType name="T">
    <xs:sequence>
      <xs:group ref="tns:g"/>
    </xs:sequence>
    <xs:attribute name="att" type="xs:string"/>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`

	parsedLegacy, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	parsedExp, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse exp schema: %v", err)
	}
	legacySchema, err := loadmerge.CloneSchemaDeep(parsedLegacy)
	if err != nil {
		t.Fatalf("clone legacy schema: %v", err)
	}
	expSchema, err := loadmerge.CloneSchemaDeep(parsedExp)
	if err != nil {
		t.Fatalf("clone exp schema: %v", err)
	}

	legacyReg, legacyRefs, legacyComplex := prepareLegacyNormalization(t, legacySchema)
	expArtifacts, err := expnormalize.PrepareOwned(expSchema)
	if err != nil {
		t.Fatalf("exp normalize: %v", err)
	}

	if got, want := registrySignatureExp(expArtifacts.Registry()), registrySignatureLegacy(legacyReg); got != want {
		t.Fatalf("registry signature mismatch:\nexp=%s\nlegacy=%s", got, want)
	}
	if got, want := refsSignatureExp(expArtifacts.References()), refsSignatureLegacy(legacyRefs); got != want {
		t.Fatalf("refs signature mismatch:\nexp=%s\nlegacy=%s", got, want)
	}

	expPrepared, err := runtimeassemble.PrepareBuildArtifactsWithComplexTypePlan(
		expArtifacts.Schema(),
		expArtifacts.Registry(),
		expArtifacts.References(),
		expArtifacts.ComplexTypes(),
	)
	if err != nil {
		t.Fatalf("prepare exp build artifacts: %v", err)
	}
	legacyPrepared, err := runtimeassemble.PrepareBuildArtifactsWithComplexTypePlan(
		legacySchema,
		legacyReg,
		legacyRefs,
		legacyComplex,
	)
	if err != nil {
		t.Fatalf("prepare legacy build artifacts: %v", err)
	}
	expRuntime, err := expPrepared.Build(runtimeassemble.BuildConfig{})
	if err != nil {
		t.Fatalf("build exp runtime: %v", err)
	}
	legacyRuntime, err := legacyPrepared.Build(runtimeassemble.BuildConfig{})
	if err != nil {
		t.Fatalf("build legacy runtime: %v", err)
	}
	if expRuntime.BuildHash != legacyRuntime.BuildHash {
		t.Fatalf("runtime build hash mismatch: exp=%x legacy=%x", expRuntime.BuildHash, legacyRuntime.BuildHash)
	}
}

func prepareLegacyNormalization(
	t *testing.T,
	sch *parser.Schema,
) (*analysis.Registry, *analysis.ResolvedReferences, *complextypeplan.Plan) {
	t.Helper()
	if err := prep.ResolveAndValidateOwned(sch); err != nil {
		t.Fatalf("legacy resolve and validate: %v", err)
	}
	reg, err := analysis.AssignIDs(sch)
	if err != nil {
		t.Fatalf("legacy assign IDs: %v", err)
	}
	if err := analysis.DetectCycles(sch); err != nil {
		t.Fatalf("legacy detect cycles: %v", err)
	}
	if err := prep.ValidateUPA(sch, reg); err != nil {
		t.Fatalf("legacy validate UPA: %v", err)
	}
	refs, err := analysis.ResolveReferences(sch, reg)
	if err != nil {
		t.Fatalf("legacy resolve refs: %v", err)
	}
	complexPlan, err := runtimeassemble.BuildComplexTypePlan(sch, reg)
	if err != nil {
		t.Fatalf("legacy complex type plan: %v", err)
	}
	return reg, refs, complexPlan
}

func registrySignatureLegacy(reg *analysis.Registry) string {
	if reg == nil {
		return "<nil>"
	}
	var b strings.Builder
	for _, entry := range reg.TypeOrder {
		_, _ = fmt.Fprintf(&b, "T:%d:%t:{%s}%s|", entry.ID, entry.Global, entry.QName.Namespace, entry.QName.Local)
	}
	for _, entry := range reg.ElementOrder {
		_, _ = fmt.Fprintf(&b, "E:%d:%t:{%s}%s|", entry.ID, entry.Global, entry.QName.Namespace, entry.QName.Local)
	}
	for _, entry := range reg.AttributeOrder {
		_, _ = fmt.Fprintf(&b, "A:%d:%t:{%s}%s|", entry.ID, entry.Global, entry.QName.Namespace, entry.QName.Local)
	}
	return b.String()
}

func registrySignatureExp(reg *analysis.Registry) string {
	if reg == nil {
		return "<nil>"
	}
	var b strings.Builder
	for _, entry := range reg.TypeOrder {
		_, _ = fmt.Fprintf(&b, "T:%d:%t:{%s}%s|", entry.ID, entry.Global, entry.QName.Namespace, entry.QName.Local)
	}
	for _, entry := range reg.ElementOrder {
		_, _ = fmt.Fprintf(&b, "E:%d:%t:{%s}%s|", entry.ID, entry.Global, entry.QName.Namespace, entry.QName.Local)
	}
	for _, entry := range reg.AttributeOrder {
		_, _ = fmt.Fprintf(&b, "A:%d:%t:{%s}%s|", entry.ID, entry.Global, entry.QName.Namespace, entry.QName.Local)
	}
	return b.String()
}

func refsSignatureLegacy(refs *analysis.ResolvedReferences) string {
	if refs == nil {
		return "<nil>"
	}
	parts := make([]string, 0, len(refs.ElementRefs)+len(refs.AttributeRefs)+len(refs.GroupRefs))
	for name, id := range refs.ElementRefs {
		parts = append(parts, fmt.Sprintf("ER:{%s}%s=%d", name.Namespace, name.Local, id))
	}
	for name, id := range refs.AttributeRefs {
		parts = append(parts, fmt.Sprintf("AR:{%s}%s=%d", name.Namespace, name.Local, id))
	}
	for name, target := range refs.GroupRefs {
		parts = append(parts, fmt.Sprintf("GR:{%s}%s=>{%s}%s", name.Namespace, name.Local, target.Namespace, target.Local))
	}
	slices.Sort(parts)
	return strings.Join(parts, "|")
}

func refsSignatureExp(refs *analysis.ResolvedReferences) string {
	if refs == nil {
		return "<nil>"
	}
	parts := make([]string, 0, len(refs.ElementRefs)+len(refs.AttributeRefs)+len(refs.GroupRefs))
	for name, id := range refs.ElementRefs {
		parts = append(parts, fmt.Sprintf("ER:{%s}%s=%d", name.Namespace, name.Local, id))
	}
	for name, id := range refs.AttributeRefs {
		parts = append(parts, fmt.Sprintf("AR:{%s}%s=%d", name.Namespace, name.Local, id))
	}
	for name, target := range refs.GroupRefs {
		parts = append(parts, fmt.Sprintf("GR:{%s}%s=>{%s}%s", name.Namespace, name.Local, target.Namespace, target.Local))
	}
	slices.Sort(parts)
	return strings.Join(parts, "|")
}
