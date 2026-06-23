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
	elements    map[runtime.ElementID][]runtime.IdentityConstraintID
	constraints map[runtime.IdentityConstraintID]runtime.IdentityConstraint
}

func (s identityRuntimeStub) Namespace(id runtime.NamespaceID) string {
	return s.names.Namespace(id)
}

func (s identityRuntimeStub) ForEachElementIdentityConstraint(id runtime.ElementID, fn func(runtime.IdentityConstraintID) bool) {
	for _, constraint := range s.elements[id] {
		if !fn(constraint) {
			return
		}
	}
}

func (s identityRuntimeStub) ForEachIdentitySelector(id runtime.IdentityConstraintID, fn func(runtime.IdentityPath) bool) bool {
	ic, ok := s.constraints[id]
	if !ok {
		return false
	}
	for _, path := range ic.Selector {
		if !fn(path) {
			return true
		}
	}
	return true
}

func (s identityRuntimeStub) IdentityFieldCount(id runtime.IdentityConstraintID) (int, bool) {
	ic, ok := s.constraints[id]
	if !ok {
		return 0, false
	}
	return len(ic.Fields), true
}

func (s identityRuntimeStub) ForEachIdentityElementField(id runtime.IdentityConstraintID, fn func(runtime.CompiledIdentityField) bool) bool {
	ic, ok := s.constraints[id]
	if !ok {
		return false
	}
	for _, field := range ic.ElementFields {
		if !fn(field) {
			return true
		}
	}
	return true
}

func (s identityRuntimeStub) ForEachIdentityAttributeField(id runtime.IdentityConstraintID, name runtime.QName, fn func(runtime.CompiledIdentityField) bool) bool {
	ic, ok := s.constraints[id]
	if !ok {
		return false
	}
	for _, field := range ic.AttributeFields[name] {
		if !fn(field) {
			return true
		}
	}
	return true
}

func (s identityRuntimeStub) ForEachIdentityAttributeWildcardField(id runtime.IdentityConstraintID, fn func(runtime.CompiledIdentityField) bool) bool {
	ic, ok := s.constraints[id]
	if !ok {
		return false
	}
	for _, field := range ic.AttributeWildcardFields {
		if !fn(field) {
			return true
		}
	}
	return true
}

func (s identityRuntimeStub) IdentityConstraintInfo(id runtime.IdentityConstraintID) (runtime.IdentityConstraintInfo, bool) {
	ic, ok := s.constraints[id]
	if !ok {
		return runtime.IdentityConstraintInfo{}, false
	}
	return runtime.IdentityConstraintInfo{Refer: ic.Refer, Kind: ic.Kind}, true
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

	if !IdentitySelectorMatches(names, namePath, 1, 1, []runtime.IdentityPath{{Self: true}}) {
		t.Fatal("self selector did not match selected depth")
	}
	if IdentitySelectorMatches(names, namePath, 1, 2, []runtime.IdentityPath{{Self: true}}) {
		t.Fatal("self selector matched child depth")
	}
	if !IdentitySelectorMatches(names, namePath, 1, 3, []runtime.IdentityPath{{
		Steps: []runtime.IdentityStep{{Name: child}, {Name: leaf}},
	}}) {
		t.Fatal("exact selector did not match relative path")
	}
	if !IdentitySelectorMatches(names, namePath, 0, 3, []runtime.IdentityPath{{
		Descendant: true,
		Steps:      []runtime.IdentityStep{{Name: leaf}},
	}}) {
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
	path := []runtime.IdentityPath{{
		Steps: []runtime.IdentityStep{{Wildcard: true, NamespaceSet: true, Namespace: 1}},
	}}

	if !IdentitySelectorMatches(names, known, 0, 1, path) {
		t.Fatal("namespace wildcard did not match known runtime name")
	}
	if !IdentitySelectorMatches(names, unknown, 0, 1, path) {
		t.Fatal("namespace wildcard did not match unknown runtime name by URI")
	}
	if IdentitySelectorMatches(names, known, 0, 1, []runtime.IdentityPath{{
		Steps: []runtime.IdentityStep{{Wildcard: true, NamespaceSet: true, Namespace: 2}},
	}}) {
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
	rt := identityRuntimeStub{
		names:    identityMatchNames{1: "urn:a"},
		elements: map[runtime.ElementID][]runtime.IdentityConstraintID{elemID: {constraintID}},
		constraints: map[runtime.IdentityConstraintID]runtime.IdentityConstraint{
			constraintID: {
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
			},
		},
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
	rt := identityRuntimeStub{}

	t.Run("selector", func(t *testing.T) {
		t.Parallel()

		var state IdentityState
		if err := state.StartScope([]runtime.IdentityConstraintID{invalidConstraint}, 0, 0, StartContext{Path: "/root"}); err != nil {
			t.Fatalf("StartScope() error = %v", err)
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

func TestIdentityFieldPathsMatchElementAndAttributeBranches(t *testing.T) {
	t.Parallel()

	names := identityMatchNames{1: "urn:a", 2: "urn:b"}
	elem := runtime.QName{Namespace: 1, Local: 1}
	attr := runtime.QName{Namespace: 1, Local: 2}
	otherAttr := runtime.QName{Namespace: 2, Local: 2}
	namePath := []runtime.RuntimeName{{Known: true, Name: elem}}

	if !IdentityFieldPathsMatch(names, namePath, 0, 1, []runtime.IdentityFieldPath{{
		Steps: []runtime.IdentityStep{{Name: elem}},
	}}) {
		t.Fatal("element field path did not match")
	}
	if !IdentityAttributeFieldPathsMatch(names, namePath, 1, 1, attr, []runtime.IdentityFieldPath{{
		Attr:      true,
		Attribute: attr,
	}}) {
		t.Fatal("exact attribute field path did not match")
	}
	if !IdentityAttributeFieldPathsMatch(names, namePath, 1, 1, attr, []runtime.IdentityFieldPath{{
		Attr:             true,
		AttrWildcard:     true,
		AttrNamespaceSet: true,
		AttrNamespace:    1,
	}}) {
		t.Fatal("attribute namespace wildcard did not match")
	}
	if IdentityAttributeFieldPathsMatch(names, namePath, 1, 1, otherAttr, []runtime.IdentityFieldPath{{
		Attr:             true,
		AttrWildcard:     true,
		AttrNamespaceSet: true,
		AttrNamespace:    1,
	}}) {
		t.Fatal("attribute namespace wildcard matched wrong namespace")
	}
}
