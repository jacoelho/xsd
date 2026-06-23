package compile

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeUseMergerNormalMode(t *testing.T) {
	t.Parallel()

	first := runtime.QName{Namespace: 1, Local: 1}
	second := runtime.QName{Namespace: 1, Local: 2}
	rt := attributeUseMergeRuntime()

	t.Run("appends new use", func(t *testing.T) {
		t.Parallel()

		uses := []runtime.AttributeUse{
			{Name: first, Type: 0},
		}
		merger := NewAttributeUseMerger(uses, runtime.NoWildcard, AttributeMergeNormal)

		result, err := merger.Add(rt, uses, runtime.AttributeUse{Name: second, Type: 1})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if !result.Appended || result.Index != 1 {
			t.Fatalf("merge result = %#v, want append at 1", result)
		}
	})

	t.Run("rejects duplicate non-prohibited use", func(t *testing.T) {
		t.Parallel()

		uses := []runtime.AttributeUse{
			{Name: first, Type: 0},
		}
		merger := NewAttributeUseMerger(uses, runtime.NoWildcard, AttributeMergeNormal)

		_, err := merger.Add(rt, uses, runtime.AttributeUse{Name: first, Type: 1})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaDuplicate)
	})

	t.Run("replaces prohibited duplicate", func(t *testing.T) {
		t.Parallel()

		uses := []runtime.AttributeUse{
			{Name: first, Type: 0, Prohibited: true},
		}
		merger := NewAttributeUseMerger(uses, runtime.NoWildcard, AttributeMergeNormal)

		result, err := merger.Add(rt, uses, runtime.AttributeUse{Name: first, Type: 1})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if result.Appended || result.Index != 0 {
			t.Fatalf("merge result = %#v, want replace at 0", result)
		}
	})
}

func TestAttributeUseMergerRestrictionMode(t *testing.T) {
	t.Parallel()

	first := runtime.QName{Namespace: 1, Local: 1}
	second := runtime.QName{Namespace: 1, Local: 2}
	foreign := runtime.QName{Namespace: 2, Local: 3}
	rt := attributeUseMergeRuntime()

	t.Run("replaces valid restricted use", func(t *testing.T) {
		t.Parallel()

		uses := []runtime.AttributeUse{
			{Name: first, Type: 0, Required: true},
		}
		merger := NewAttributeUseMerger(uses, runtime.NoWildcard, AttributeMergeRestriction)

		result, err := merger.Add(rt, uses, runtime.AttributeUse{Name: first, Type: 1, Required: true})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if result.Appended || result.Index != 0 {
			t.Fatalf("merge result = %#v, want replace at 0", result)
		}
	})

	t.Run("rejects invalid restricted use", func(t *testing.T) {
		t.Parallel()

		uses := []runtime.AttributeUse{
			{Name: first, Type: 0, Required: true},
		}
		merger := NewAttributeUseMerger(uses, runtime.NoWildcard, AttributeMergeRestriction)

		_, err := merger.Add(rt, uses, runtime.AttributeUse{Name: first, Type: 2})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
	})

	t.Run("admits new use through inherited wildcard", func(t *testing.T) {
		t.Parallel()

		merger := NewAttributeUseMerger(nil, 1, AttributeMergeRestriction)
		result, err := merger.Add(rt, nil, runtime.AttributeUse{Name: second, Type: 1})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if !result.Appended || result.Index != 0 {
			t.Fatalf("merge result = %#v, want append at 0", result)
		}
	})

	t.Run("rejects new use without inherited wildcard", func(t *testing.T) {
		t.Parallel()

		merger := NewAttributeUseMerger(nil, runtime.NoWildcard, AttributeMergeRestriction)
		_, err := merger.Add(rt, nil, runtime.AttributeUse{Name: second, Type: 1})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
	})

	t.Run("rejects new use outside inherited wildcard", func(t *testing.T) {
		t.Parallel()

		merger := NewAttributeUseMerger(nil, 1, AttributeMergeRestriction)
		_, err := merger.Add(rt, nil, runtime.AttributeUse{Name: foreign, Type: 1})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
	})
}

func TestClassifyAttributeUseChild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		local string
		want  AttributeUseChildKind
	}{
		{local: attributeChild, want: AttributeUseChildAttribute},
		{local: attributeGroup, want: AttributeUseChildGroup},
		{local: anyAttribute, want: AttributeUseChildWildcard},
		{local: elementChild, want: AttributeUseChildIgnored},
	}
	for _, tt := range tests {
		if got := ClassifyAttributeUseChild(tt.local); got != tt.want {
			t.Fatalf("ClassifyAttributeUseChild(%q) = %v, want %v", tt.local, got, tt.want)
		}
	}
}

func TestRemoveProhibitedAttributeUses(t *testing.T) {
	t.Parallel()

	first := runtime.AttributeUse{Name: runtime.QName{Namespace: 1, Local: 1}}
	prohibited := runtime.AttributeUse{Name: runtime.QName{Namespace: 1, Local: 2}, Prohibited: true}
	last := runtime.AttributeUse{Name: runtime.QName{Namespace: 1, Local: 3}, Required: true}

	got := RemoveProhibitedAttributeUses([]runtime.AttributeUse{first, prohibited, last})
	want := []runtime.AttributeUse{first, last}
	if len(got) != len(want) {
		t.Fatalf("len(RemoveProhibitedAttributeUses()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RemoveProhibitedAttributeUses()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}

	if got := RemoveProhibitedAttributeUses(nil); got != nil {
		t.Fatalf("RemoveProhibitedAttributeUses(nil) = %#v, want nil", got)
	}
}

func attributeUseMergeRuntime() compiledModelRuntimeStub {
	return compiledModelRuntimeStub{
		simpleDerivations: map[runtime.SimpleTypeID]runtime.SimpleTypeDerivation{
			0: {Base: runtime.NoSimpleType, Variety: runtime.SimpleVarietyAtomic},
			1: {Base: 0, Variety: runtime.SimpleVarietyAtomic},
			2: {Base: runtime.NoSimpleType, Variety: runtime.SimpleVarietyAtomic},
		},
		complexDerivations: map[runtime.ComplexTypeID]runtime.ComplexTypeDerivation{
			0: {Kind: runtime.DerivationKindNone},
		},
		wildcards: map[runtime.WildcardID]runtime.Wildcard{
			1: {Mode: runtime.WildcardList, Namespaces: []runtime.NamespaceID{1}, Process: runtime.ProcessStrict},
		},
	}
}
