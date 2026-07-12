package validate

import (
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
)

type identityMatchNames map[runtime.NamespaceID]string

func (n identityMatchNames) Namespace(id runtime.NamespaceID) string {
	return n[id]
}

type identityPathForTest struct {
	path runtime.IdentityPath
}

func (p identityPathForTest) StepCount() int {
	return len(p.path.Steps)
}

func (p identityPathForTest) Step(index int) (runtime.IdentityStep, bool) {
	if index < 0 || index >= len(p.path.Steps) {
		return runtime.IdentityStep{}, false
	}
	return p.path.Steps[index], true
}

func (p identityPathForTest) Descendant() bool {
	return p.path.Descendant
}

func (p identityPathForTest) Self() bool {
	return p.path.Self
}

func matchIdentityPathForTest(names identityMatchNames, namePath []runtime.RuntimeName, scopeDepth, currentDepth int, path runtime.IdentityPath) bool {
	return identityPathMatches(names, namePath, scopeDepth, currentDepth, identityPathForTest{path: path})
}

func TestIdentitySelectorMatchesSelfDescendantAndExactPaths(t *testing.T) {
	t.Parallel()

	names := identityMatchNames{1: "urn:a"}
	root := runtime.QName{Namespace: 1, Local: 1}
	child := runtime.QName{Namespace: 1, Local: 2}
	leaf := runtime.QName{Namespace: 1, Local: 3}
	namePath := []runtime.RuntimeName{
		{Known: true, Name: root},
		{Known: true, Name: child},
		{Known: true, Name: leaf},
	}

	if !matchIdentityPathForTest(names, namePath, 1, 1, runtime.IdentityPath{Self: true}) {
		t.Fatal("self selector did not match selected depth")
	}
	if matchIdentityPathForTest(names, namePath, 1, 2, runtime.IdentityPath{Self: true}) {
		t.Fatal("self selector matched child depth")
	}
	if !matchIdentityPathForTest(names, namePath, 1, 3, runtime.IdentityPath{
		Steps: []runtime.IdentityStep{{Name: child}, {Name: leaf}},
	}) {
		t.Fatal("exact selector did not match relative path")
	}
	if !matchIdentityPathForTest(names, namePath, 0, 3, runtime.IdentityPath{
		Descendant: true,
		Steps:      []runtime.IdentityStep{{Name: leaf}},
	}) {
		t.Fatal("descendant selector did not match suffix")
	}
}

func TestIdentitySelectorWildcardNamespaceMatchesKnownAndUnknownRuntimeNames(t *testing.T) {
	t.Parallel()

	names := identityMatchNames{1: "urn:a", 2: "urn:b"}
	known := []runtime.RuntimeName{
		{Known: true, Name: runtime.QName{Namespace: 1, Local: 1}},
	}
	unknown := []runtime.RuntimeName{
		{NS: "urn:a", Local: "external"},
	}
	path := runtime.IdentityPath{
		Steps: []runtime.IdentityStep{{Wildcard: true, NamespaceSet: true, Namespace: 1}},
	}

	if !matchIdentityPathForTest(names, known, 0, 1, path) {
		t.Fatal("namespace wildcard did not match known runtime name")
	}
	if !matchIdentityPathForTest(names, unknown, 0, 1, path) {
		t.Fatal("namespace wildcard did not match unknown runtime name by URI")
	}
	if matchIdentityPathForTest(names, known, 0, 1, runtime.IdentityPath{
		Steps: []runtime.IdentityStep{{Wildcard: true, NamespaceSet: true, Namespace: 2}},
	}) {
		t.Fatal("namespace wildcard matched wrong namespace")
	}
}

func TestIdentityStateUsesRuntimeMetadataForSelectorAndFieldMatching(t *testing.T) {
	t.Parallel()

	rt, elemID, _, elemName, attrName := compiledIdentityRuntimeForTest(t)
	namePath := []runtime.RuntimeName{{Known: true, Name: elemName}}

	var state IdentityState
	if err := state.startElementScope(rt, elemID, len(namePath), 0, StartContext{Path: "/root"}); err != nil {
		t.Fatalf("startElementScope() error = %v", err)
	}
	if err := state.matchSelectors(rt, namePath, StartContext{Path: "/root", Line: 2, Column: 3}); err != nil {
		t.Fatalf("matchSelectors() error = %v", err)
	}
	if len(state.selections) != 1 {
		t.Fatalf("selections = %d, want 1", len(state.selections))
	}

	elementMatches, err := state.elementFieldMatches(rt, namePath)
	if err != nil {
		t.Fatalf("elementFieldMatches() error = %v", err)
	}
	if len(elementMatches) != 1 || elementMatches[0] != (IdentityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("elementFieldMatches() = %+v, want selection 0 field 0", elementMatches)
	}

	attrMatches, err := state.attributeFieldMatches(rt, namePath, attrName)
	if err != nil {
		t.Fatalf("attributeFieldMatches() error = %v", err)
	}
	if len(attrMatches) != 1 || attrMatches[0] != (IdentityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("attributeFieldMatches() = %+v, want one deduplicated field match", attrMatches)
	}
}

func TestCompiledIdentityFieldPathsMatchElementAndAttributeBranches(t *testing.T) {
	t.Parallel()

	rt, _, constraintID, elem, attr := compiledIdentityRuntimeForTest(t)
	otherAttr := runtime.QName{Namespace: 999, Local: attr.Local}
	namePath := []runtime.RuntimeName{{Known: true, Name: elem}}

	elementFields, ok := rt.IdentityElementFields(constraintID)
	if !ok {
		t.Fatal("IdentityElementFields() rejected runtime metadata")
	}
	elementField, ok := elementFields.At(0)
	if !ok {
		t.Fatal("IdentityElementFields() returned no field")
	}
	if !identityCompiledFieldPathsMatch(rt, namePath, 1, 1, elementField) {
		t.Fatal("element field path did not match")
	}

	attributeFields, ok := rt.IdentityAttributeFields(constraintID, attr)
	if !ok {
		t.Fatal("IdentityAttributeFields() rejected runtime metadata")
	}
	exactAttributeField, ok := attributeFields.At(0)
	if !ok {
		t.Fatal("IdentityAttributeFields() returned no exact field")
	}
	if !identityCompiledAttributeFieldPathsMatch(rt, namePath, 1, 1, attr, exactAttributeField) {
		t.Fatal("exact attribute field path did not match")
	}
	attributeFields, ok = rt.IdentityAttributeWildcardFields(constraintID)
	if !ok {
		t.Fatal("IdentityAttributeWildcardFields() rejected runtime metadata")
	}
	wildcardAttributeField, ok := attributeFields.At(0)
	if !ok {
		t.Fatal("IdentityAttributeWildcardFields() returned no wildcard field")
	}
	if !identityCompiledAttributeFieldPathsMatch(rt, namePath, 1, 1, attr, wildcardAttributeField) {
		t.Fatal("attribute namespace wildcard did not match")
	}
	if identityCompiledAttributeFieldPathsMatch(rt, namePath, 1, 1, otherAttr, wildcardAttributeField) {
		t.Fatal("attribute namespace wildcard matched wrong namespace")
	}
}

func compiledIdentityRuntimeForTest(t *testing.T) (*runtime.Schema, runtime.ElementID, runtime.IdentityConstraintID, runtime.QName, runtime.QName) {
	t.Helper()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:a" targetNamespace="urn:a" elementFormDefault="qualified">
	<xs:attribute name="id" type="xs:string"/>
  <xs:element name="root">
	<xs:complexType mixed="true"><xs:attribute ref="t:id"/></xs:complexType>
	<xs:key name="k">
	  <xs:selector xpath="."/>
	  <xs:field xpath=". | @t:id | @t:*"/>
	</xs:key>
  </xs:element>
</xs:schema>`
	rt, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("identity.xsd", []byte(schema))})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	elemName, ok := rt.LookupQName("urn:a", "root")
	if !ok {
		t.Fatal("LookupQName(root) failed")
	}
	attrName, ok := rt.LookupQName("urn:a", "id")
	if !ok {
		t.Fatal("LookupQName(id) failed")
	}
	elemID, _, ok := rt.RootElement(runtime.RuntimeName{Known: true, Name: elemName})
	if !ok {
		t.Fatal("RootElement(root) failed")
	}
	constraints, ok := rt.ElementIdentityConstraints(elemID)
	if !ok {
		t.Fatal("ElementIdentityConstraints(root) failed")
	}
	constraintID, ok := constraints.At(0)
	if !ok {
		t.Fatal("root has no identity constraint")
	}
	return rt, elemID, constraintID, elemName, attrName
}
