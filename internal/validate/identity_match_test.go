package validate

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

type identityMatchNames map[runtime.NamespaceID]string

func (n identityMatchNames) Namespace(id runtime.NamespaceID) string {
	return n[id]
}

type identityRuntimeStub struct {
	names       identityMatchNames
	elements    map[runtime.ElementID]runtime.IdentityConstraintIDs
	constraints []runtime.IdentityConstraintRead
}

func newIdentityRuntimeStub(names identityMatchNames, id runtime.IdentityConstraintID, constraint runtime.IdentityConstraint) identityRuntimeStub {
	constraints := make([]runtime.IdentityConstraint, int(id)+1)
	constraints[id] = constraint
	return identityRuntimeStub{
		names:       names,
		constraints: runtime.NewIdentityConstraintReads(constraints),
	}
}

func (s identityRuntimeStub) Namespace(id runtime.NamespaceID) string {
	return s.names.Namespace(id)
}

func (s identityRuntimeStub) ElementIdentityConstraints(id runtime.ElementID) (runtime.IdentityConstraintIDs, bool) {
	constraints, ok := s.elements[id]
	return constraints, ok
}

func (s identityRuntimeStub) IdentitySelectorPaths(id runtime.IdentityConstraintID) (runtime.IdentityPathReads, bool) {
	return runtime.IdentitySelectorPathReads(s.constraints, id)
}

func (s identityRuntimeStub) IdentityFieldCount(id runtime.IdentityConstraintID) (int, bool) {
	return runtime.IdentityFieldCount(s.constraints, id)
}

func (s identityRuntimeStub) IdentityElementFields(id runtime.IdentityConstraintID) (runtime.CompiledIdentityFieldReads, bool) {
	return runtime.IdentityElementFieldReads(s.constraints, id)
}

func (s identityRuntimeStub) IdentityAttributeFields(id runtime.IdentityConstraintID, name runtime.QName) (runtime.CompiledIdentityFieldReads, bool) {
	return runtime.IdentityAttributeFieldReads(s.constraints, id, name)
}

func (s identityRuntimeStub) IdentityAttributeWildcardFields(id runtime.IdentityConstraintID) (runtime.CompiledIdentityFieldReads, bool) {
	return runtime.IdentityAttributeWildcardFieldReads(s.constraints, id)
}

func (s identityRuntimeStub) IdentityConstraintInfo(id runtime.IdentityConstraintID) (runtime.IdentityConstraintInfo, bool) {
	return runtime.IdentityConstraintInfoByID(s.constraints, id)
}

func matchIdentitySelectorForTest(
	t *testing.T,
	names identityMatchNames,
	namePath []runtime.RuntimeName,
	scopeDepth, currentDepth int,
	path runtime.IdentityPath,
) bool {
	t.Helper()

	const constraintID runtime.IdentityConstraintID = 0
	rt := newIdentityRuntimeStub(names, constraintID, runtime.IdentityConstraint{
		Selector: []runtime.IdentityPath{path},
	})
	matched, ok := identitySelectorMatches(rt, constraintID, namePath, scopeDepth, currentDepth)
	if !ok {
		t.Fatal("identitySelectorMatches() rejected runtime metadata")
	}
	return matched
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

	if !matchIdentitySelectorForTest(t, names, namePath, 1, 1, runtime.IdentityPath{Self: true}) {
		t.Fatal("self selector did not match selected depth")
	}
	if matchIdentitySelectorForTest(t, names, namePath, 1, 2, runtime.IdentityPath{Self: true}) {
		t.Fatal("self selector matched child depth")
	}
	if !matchIdentitySelectorForTest(t, names, namePath, 1, 3, runtime.IdentityPath{
		Steps: []runtime.IdentityStep{{Name: child}, {Name: leaf}},
	}) {
		t.Fatal("exact selector did not match relative path")
	}
	if !matchIdentitySelectorForTest(t, names, namePath, 0, 3, runtime.IdentityPath{
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

	if !matchIdentitySelectorForTest(t, names, known, 0, 1, path) {
		t.Fatal("namespace wildcard did not match known runtime name")
	}
	if !matchIdentitySelectorForTest(t, names, unknown, 0, 1, path) {
		t.Fatal("namespace wildcard did not match unknown runtime name by URI")
	}
	if matchIdentitySelectorForTest(t, names, known, 0, 1, runtime.IdentityPath{
		Steps: []runtime.IdentityStep{{Wildcard: true, NamespaceSet: true, Namespace: 2}},
	}) {
		t.Fatal("namespace wildcard matched wrong namespace")
	}
}

func TestIdentityStateUsesRuntimeMetadataForSelectorAndFieldMatching(t *testing.T) {
	t.Parallel()

	const (
		elemID       runtime.ElementID            = 1
		constraintID runtime.IdentityConstraintID = 2
	)
	elemName := runtime.QName{Namespace: 1, Local: 1}
	attrName := runtime.QName{Namespace: 1, Local: 2}
	rt := newIdentityRuntimeStub(identityMatchNames{1: "urn:a"}, constraintID, runtime.IdentityConstraint{
		Selector: []runtime.IdentityPath{{Self: true}},
		Fields:   []runtime.IdentityField{{}},
		ElementFields: []runtime.CompiledIdentityField{{
			Field: 0,
			Paths: []runtime.IdentityFieldPath{{Self: true}},
		}},
		AttributeFields: map[runtime.QName][]runtime.CompiledIdentityField{
			attrName: {{
				Field: 0,
				Paths: []runtime.IdentityFieldPath{{Self: true, Attr: true, Attribute: attrName}},
			}},
		},
		AttributeWildcardFields: []runtime.CompiledIdentityField{{
			Field: 0,
			Paths: []runtime.IdentityFieldPath{{
				Self:             true,
				Attr:             true,
				AttrWildcard:     true,
				AttrNamespaceSet: true,
				AttrNamespace:    1,
			}},
		}},
	})
	elementConstraints, ok := runtime.ElementIdentityConstraintIDs([][]runtime.IdentityConstraintID{{constraintID}}, 0)
	if !ok {
		t.Fatal("ElementIdentityConstraintIDs() rejected test fixture")
	}
	rt.elements = map[runtime.ElementID]runtime.IdentityConstraintIDs{
		elemID: elementConstraints,
	}
	namePath := []runtime.RuntimeName{{Known: true, Name: elemName}}

	var state IdentityState
	if err := state.StartElementScope(rt, elemID, len(namePath), 0, StartContext{Path: "/root"}); err != nil {
		t.Fatalf("StartElementScope() error = %v", err)
	}
	if err := state.MatchSelectors(rt, namePath, StartContext{Path: "/root", Line: 2, Column: 3}); err != nil {
		t.Fatalf("MatchSelectors() error = %v", err)
	}
	if len(state.selections) != 1 {
		t.Fatalf("selections = %d, want 1", len(state.selections))
	}

	elementMatches, err := state.ElementFieldMatches(rt, namePath)
	if err != nil {
		t.Fatalf("ElementFieldMatches() error = %v", err)
	}
	if len(elementMatches) != 1 || elementMatches[0] != (IdentityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("ElementFieldMatches() = %+v, want selection 0 field 0", elementMatches)
	}

	attrMatches, err := state.AttributeFieldMatches(rt, namePath, attrName)
	if err != nil {
		t.Fatalf("AttributeFieldMatches() error = %v", err)
	}
	if len(attrMatches) != 1 || attrMatches[0] != (IdentityFieldMatch{Selection: 0, Field: 0}) {
		t.Fatalf("AttributeFieldMatches() = %+v, want one deduplicated field match", attrMatches)
	}
}

func TestIdentityStateRejectsInvalidRuntimeIdentityMetadata(t *testing.T) {
	t.Parallel()

	const invalidConstraint runtime.IdentityConstraintID = 99
	const elem runtime.ElementID = 0
	elementConstraints, ok := runtime.ElementIdentityConstraintIDs([][]runtime.IdentityConstraintID{{invalidConstraint}}, elem)
	if !ok {
		t.Fatal("ElementIdentityConstraintIDs() rejected test fixture")
	}
	rt := identityRuntimeStub{elements: map[runtime.ElementID]runtime.IdentityConstraintIDs{elem: elementConstraints}}

	t.Run("selector", func(t *testing.T) {
		t.Parallel()

		var state IdentityState
		if err := state.StartElementScope(rt, elem, 0, 0, StartContext{Path: "/root"}); err != nil {
			t.Fatalf("StartElementScope() error = %v", err)
		}
		err := state.MatchSelectors(rt, []runtime.RuntimeName{{}}, StartContext{Path: "/root", Line: 1, Column: 2})
		expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
	})

	t.Run("element field", func(t *testing.T) {
		t.Parallel()

		var state IdentityState
		state.StartSelection(0, 1, invalidConstraint, 1, StartContext{Path: "/root"})
		_, err := state.ElementFieldMatches(rt, []runtime.RuntimeName{{}})
		expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
	})

	t.Run("attribute field", func(t *testing.T) {
		t.Parallel()

		var state IdentityState
		state.StartSelection(0, 1, invalidConstraint, 1, StartContext{Path: "/root"})
		_, err := state.AttributeFieldMatches(rt, []runtime.RuntimeName{{}}, runtime.QName{})
		expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
	})
}

func TestCompiledIdentityFieldPathsMatchElementAndAttributeBranches(t *testing.T) {
	t.Parallel()

	const constraintID runtime.IdentityConstraintID = 0
	names := identityMatchNames{1: "urn:a", 2: "urn:b"}
	elem := runtime.QName{Namespace: 1, Local: 1}
	attr := runtime.QName{Namespace: 1, Local: 2}
	otherAttr := runtime.QName{Namespace: 2, Local: 2}
	namePath := []runtime.RuntimeName{{Known: true, Name: elem}}
	rt := newIdentityRuntimeStub(names, constraintID, runtime.IdentityConstraint{
		ElementFields: []runtime.CompiledIdentityField{{
			Paths: []runtime.IdentityFieldPath{{Steps: []runtime.IdentityStep{{Name: elem}}}},
		}},
		AttributeWildcardFields: []runtime.CompiledIdentityField{
			{Paths: []runtime.IdentityFieldPath{{Attr: true, Attribute: attr}}},
			{Paths: []runtime.IdentityFieldPath{{
				Attr:             true,
				AttrWildcard:     true,
				AttrNamespaceSet: true,
				AttrNamespace:    1,
			}}},
		},
	})

	elementFields, ok := rt.IdentityElementFields(constraintID)
	if !ok {
		t.Fatal("IdentityElementFields() rejected runtime metadata")
	}
	elementField, ok := elementFields.At(0)
	if !ok {
		t.Fatal("IdentityElementFields() returned no field")
	}
	if !identityCompiledFieldPathsMatch(rt, namePath, 0, 1, elementField) {
		t.Fatal("element field path did not match")
	}

	attributeFields, ok := rt.IdentityAttributeWildcardFields(constraintID)
	if !ok {
		t.Fatal("IdentityAttributeWildcardFields() rejected runtime metadata")
	}
	exactAttributeField, ok := attributeFields.At(0)
	if !ok {
		t.Fatal("IdentityAttributeWildcardFields() returned no exact field")
	}
	if !identityCompiledAttributeFieldPathsMatch(rt, namePath, 1, 1, attr, exactAttributeField) {
		t.Fatal("exact attribute field path did not match")
	}
	wildcardAttributeField, ok := attributeFields.At(1)
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
