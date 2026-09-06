package schema

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeUseMergerNormalMode(t *testing.T) {
	t.Parallel()

	first := QName{Namespace: 1, Local: 1}
	second := QName{Namespace: 1, Local: 2}
	rt := attributeUseMergeRuntime()

	t.Run("appends new use", func(t *testing.T) {
		t.Parallel()

		uses := []AttributeUse{
			{Name: first, Type: 0},
		}
		merger := NewAttributeUseMerger(uses, NoWildcard, AttributeMergeDirect)

		result, err := merger.Add(rt, uses, AttributeUse{Name: second, Type: 1}, unboundedTypeDerivationWork)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if !result.Appended || result.Index != 1 {
			t.Fatalf("merge result = %#v, want append at 1", result)
		}
	})

	t.Run("rejects duplicate non-prohibited use", func(t *testing.T) {
		t.Parallel()

		uses := []AttributeUse{
			{Name: first, Type: 0},
		}
		merger := NewAttributeUseMerger(uses, NoWildcard, AttributeMergeDirect)

		_, err := merger.Add(rt, uses, AttributeUse{Name: first, Type: 1}, unboundedTypeDerivationWork)
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaDuplicate)
	})

	t.Run("replaces prohibited duplicate", func(t *testing.T) {
		t.Parallel()

		uses := []AttributeUse{
			{Name: first, Type: 0, Prohibited: true},
		}
		merger := NewAttributeUseMerger(uses, NoWildcard, AttributeMergeDirect)

		result, err := merger.Add(rt, uses, AttributeUse{Name: first, Type: 1}, unboundedTypeDerivationWork)
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

	first := QName{Namespace: 1, Local: 1}
	second := QName{Namespace: 1, Local: 2}
	foreign := QName{Namespace: 2, Local: 3}
	rt := attributeUseMergeRuntime()

	t.Run("replaces valid restricted use", func(t *testing.T) {
		t.Parallel()

		uses := []AttributeUse{
			{Name: first, Type: 0, Required: true},
		}
		merger := NewAttributeUseMerger(uses, NoWildcard, AttributeMergeRestriction)

		result, err := merger.Add(rt, uses, AttributeUse{Name: first, Type: 1, Required: true}, unboundedTypeDerivationWork)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if result.Appended || result.Index != 0 {
			t.Fatalf("merge result = %#v, want replace at 0", result)
		}
	})

	t.Run("rejects invalid restricted use", func(t *testing.T) {
		t.Parallel()

		uses := []AttributeUse{
			{Name: first, Type: 0, Required: true},
		}
		merger := NewAttributeUseMerger(uses, NoWildcard, AttributeMergeRestriction)

		_, err := merger.Add(rt, uses, AttributeUse{Name: first, Type: 2}, unboundedTypeDerivationWork)
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
	})

	t.Run("admits new use through inherited wildcard", func(t *testing.T) {
		t.Parallel()

		merger := NewAttributeUseMerger(nil, 1, AttributeMergeRestriction)
		result, err := merger.Add(rt, nil, AttributeUse{Name: second, Type: 1}, unboundedTypeDerivationWork)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if !result.Appended || result.Index != 0 {
			t.Fatalf("merge result = %#v, want append at 0", result)
		}
	})

	t.Run("rejects new use without inherited wildcard", func(t *testing.T) {
		t.Parallel()

		merger := NewAttributeUseMerger(nil, NoWildcard, AttributeMergeRestriction)
		_, err := merger.Add(rt, nil, AttributeUse{Name: second, Type: 1}, unboundedTypeDerivationWork)
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
	})

	t.Run("rejects new use outside inherited wildcard", func(t *testing.T) {
		t.Parallel()

		merger := NewAttributeUseMerger(nil, 1, AttributeMergeRestriction)
		_, err := merger.Add(rt, nil, AttributeUse{Name: foreign, Type: 1}, unboundedTypeDerivationWork)
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

	first := AttributeUse{Name: QName{Namespace: 1, Local: 1}}
	prohibited := AttributeUse{Name: QName{Namespace: 1, Local: 2}, Prohibited: true}
	last := AttributeUse{Name: QName{Namespace: 1, Local: 3}, Required: true}

	got := RemoveProhibitedAttributeUses([]AttributeUse{first, prohibited, last})
	want := []AttributeUse{first, last}
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
		simpleDerivations: map[SimpleTypeID]SimpleTypeDerivation{
			0: {Base: NoSimpleType, Variety: SimpleVarietyAtomic},
			1: {Base: 0, Variety: SimpleVarietyAtomic},
			2: {Base: NoSimpleType, Variety: SimpleVarietyAtomic},
		},
		complexDerivations: map[ComplexTypeID]ComplexTypeDerivation{
			0: {Kind: DerivationKindNone},
		},
		wildcards: map[WildcardID]Wildcard{
			1: {Mode: WildcardList, Namespaces: []NamespaceID{1}, Process: ProcessStrict},
		},
	}
}
