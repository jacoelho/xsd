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

	fixture := compiledIdentityRuntimeForTest(t)
	namePath := []runtime.RuntimeName{{Known: true, Name: fixture.elemName}}

	var state identityState
	if err := state.startElementScope(fixture.rt, fixture.elemID, len(namePath), 0, StartContext{Path: "/root"}); err != nil {
		t.Fatalf("startElementScope() error = %v", err)
	}
	if err := state.matchSelectors(fixture.rt, namePath, 0, StartContext{Path: "/root", Line: 2, Column: 3}); err != nil {
		t.Fatalf("matchSelectors() error = %v", err)
	}
	if len(state.selections) != 1 {
		t.Fatalf("selections = %d, want 1", len(state.selections))
	}

	elementMatches, err := state.elementFieldMatches(fixture.rt, namePath)
	if err != nil {
		t.Fatalf("elementFieldMatches() error = %v", err)
	}
	if len(elementMatches) != 1 || elementMatches[0] != (identityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("elementFieldMatches() = %+v, want selection 0 field 0", elementMatches)
	}

	attrMatches, err := state.attributeFieldMatches(fixture.rt, namePath, runtime.RuntimeName{Name: fixture.attrName, Known: true})
	if err != nil {
		t.Fatalf("attributeFieldMatches() error = %v", err)
	}
	if len(attrMatches) != 1 || attrMatches[0] != (identityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("attributeFieldMatches() = %+v, want one deduplicated field match", attrMatches)
	}
	unknownMatches, err := state.attributeFieldMatches(fixture.rt, namePath, runtime.RuntimeName{NS: "urn:a", Local: "unknown"})
	if err != nil {
		t.Fatalf("attributeFieldMatches(unknown) error = %v", err)
	}
	if len(unknownMatches) != 1 || unknownMatches[0] != (identityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("attributeFieldMatches(unknown) = %+v, want namespace-wildcard match", unknownMatches)
	}
	wrongNamespace, err := state.attributeFieldMatches(fixture.rt, namePath, runtime.RuntimeName{NS: "urn:other", Local: "unknown"})
	if err != nil {
		t.Fatalf("attributeFieldMatches(wrong namespace) error = %v", err)
	}
	if len(wrongNamespace) != 0 {
		t.Fatalf("attributeFieldMatches(wrong namespace) = %+v, want no match", wrongNamespace)
	}
}

func TestCompiledIdentityFieldPathsMatchElementAndAttributeBranches(t *testing.T) {
	t.Parallel()

	fixture := compiledIdentityRuntimeForTest(t)
	otherAttr := runtime.QName{Namespace: 999, Local: fixture.attrName.Local}
	namePath := []runtime.RuntimeName{{Known: true, Name: fixture.elemName}}

	constraint, ok := fixture.rt.IdentityConstraint(fixture.constraintID)
	if !ok {
		t.Fatal("IdentityConstraint() rejected runtime metadata")
	}
	elementFields := constraint.ElementFields()
	elementField, ok := elementFields.At(0)
	if !ok {
		t.Fatal("IdentityConstraint().ElementFields() returned no field")
	}
	if !identityCompiledFieldPathsMatch(fixture.rt, namePath, 1, 1, elementField) {
		t.Fatal("element field path did not match")
	}

	attributeFields := constraint.AttributeFields(fixture.attrName)
	exactAttributeField, ok := attributeFields.At(0)
	if !ok {
		t.Fatal("IdentityConstraint().AttributeFields() returned no exact field")
	}
	if !identityCompiledAttributeFieldPathsMatch(fixture.rt, namePath, 1, 1, runtime.RuntimeName{Name: fixture.attrName, Known: true}, exactAttributeField) {
		t.Fatal("exact attribute field path did not match")
	}
	attributeFields = constraint.AttributeWildcardFields()
	wildcardAttributeField, ok := attributeFields.At(0)
	if !ok {
		t.Fatal("IdentityConstraint().AttributeWildcardFields() returned no wildcard field")
	}
	if !identityCompiledAttributeFieldPathsMatch(fixture.rt, namePath, 1, 1, runtime.RuntimeName{Name: fixture.attrName, Known: true}, wildcardAttributeField) {
		t.Fatal("attribute namespace wildcard did not match")
	}
	if identityCompiledAttributeFieldPathsMatch(fixture.rt, namePath, 1, 1, runtime.RuntimeName{Name: otherAttr, Known: true}, wildcardAttributeField) {
		t.Fatal("attribute namespace wildcard matched wrong namespace")
	}
}

type compiledIdentityFixture struct {
	rt           *runtime.Schema
	elemID       runtime.ElementID
	constraintID runtime.IdentityConstraintID
	elemName     runtime.QName
	attrName     runtime.QName
}

func compiledIdentityRuntimeForTest(t *testing.T) compiledIdentityFixture {
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
	return compiledIdentityFixture{
		rt:           rt,
		elemID:       elemID,
		constraintID: constraintID,
		elemName:     elemName,
		attrName:     attrName,
	}
}
