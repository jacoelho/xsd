package schema

import (
	"slices"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeUseCompilationNormalMode(t *testing.T) {
	t.Parallel()

	first := QName{Namespace: 1, Local: 1}
	second := QName{Namespace: 1, Local: 2}
	rt := attributeUseMergeRuntime()

	t.Run("appends new use", func(t *testing.T) {
		t.Parallel()

		uses := []AttributeUse{
			{Name: first, Type: 0},
		}
		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, uses, NoWildcard, AttributeMergeDirect)

		err := compilation.mergeUse(AttributeUse{Name: second, Type: 1})
		if err != nil {
			t.Fatalf("mergeUse() error = %v", err)
		}
		want := []AttributeUse{{Name: first, Type: 0}, {Name: second, Type: 1}}
		if !slices.Equal(compilation.uses, want) {
			t.Fatalf("uses = %#v, want %#v", compilation.uses, want)
		}
	})

	t.Run("rejects duplicate non-prohibited use", func(t *testing.T) {
		t.Parallel()

		uses := []AttributeUse{
			{Name: first, Type: 0},
		}
		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, uses, NoWildcard, AttributeMergeDirect)

		err := compilation.mergeUse(AttributeUse{Name: first, Type: 1})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaDuplicate)
		want := []AttributeUse{{Name: first, Type: 0}}
		if !slices.Equal(compilation.uses, want) {
			t.Fatalf("rejected duplicate changed uses: %#v, want %#v", compilation.uses, want)
		}
	})

	t.Run("replaces prohibited duplicate", func(t *testing.T) {
		t.Parallel()

		uses := []AttributeUse{
			{Name: first, Type: 0, Prohibited: true},
		}
		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, uses, NoWildcard, AttributeMergeDirect)

		err := compilation.mergeUse(AttributeUse{Name: first, Type: 1})
		if err != nil {
			t.Fatalf("mergeUse() error = %v", err)
		}
		want := []AttributeUse{{Name: first, Type: 1}}
		if !slices.Equal(compilation.uses, want) {
			t.Fatalf("uses = %#v, want %#v", compilation.uses, want)
		}
	})
}

func TestAttributeUseCompilationRestrictionMode(t *testing.T) {
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
		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, uses, NoWildcard, AttributeMergeRestriction)

		err := compilation.mergeUse(AttributeUse{Name: first, Type: 1, Required: true})
		if err != nil {
			t.Fatalf("mergeUse() error = %v", err)
		}
		want := []AttributeUse{{Name: first, Type: 1, Required: true}}
		if !slices.Equal(compilation.uses, want) {
			t.Fatalf("uses = %#v, want %#v", compilation.uses, want)
		}
	})

	t.Run("rejects invalid restricted use", func(t *testing.T) {
		t.Parallel()

		uses := []AttributeUse{
			{Name: first, Type: 0, Required: true},
		}
		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, uses, NoWildcard, AttributeMergeRestriction)

		err := compilation.mergeUse(AttributeUse{Name: first, Type: 2})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
		want := []AttributeUse{{Name: first, Type: 0, Required: true}}
		if !slices.Equal(compilation.uses, want) {
			t.Fatalf("rejected restriction changed uses: %#v, want %#v", compilation.uses, want)
		}
	})

	t.Run("admits new use through inherited wildcard", func(t *testing.T) {
		t.Parallel()

		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, nil, 1, AttributeMergeRestriction)
		err := compilation.mergeUse(AttributeUse{Name: second, Type: 1})
		if err != nil {
			t.Fatalf("mergeUse() error = %v", err)
		}
		want := []AttributeUse{{Name: second, Type: 1}}
		if !slices.Equal(compilation.uses, want) {
			t.Fatalf("uses = %#v, want %#v", compilation.uses, want)
		}
	})

	t.Run("rejects new use without inherited wildcard", func(t *testing.T) {
		t.Parallel()

		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, nil, NoWildcard, AttributeMergeRestriction)
		err := compilation.mergeUse(AttributeUse{Name: second, Type: 1})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
		if compilation.uses != nil {
			t.Fatalf("rejected restriction changed uses: %#v, want nil", compilation.uses)
		}
	})

	t.Run("rejects new use outside inherited wildcard", func(t *testing.T) {
		t.Parallel()

		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, nil, 1, AttributeMergeRestriction)
		err := compilation.mergeUse(AttributeUse{Name: foreign, Type: 1})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
		if compilation.uses != nil {
			t.Fatalf("rejected restriction changed uses: %#v, want nil", compilation.uses)
		}
	})
}

func TestAttributeUseCompilationPreservesOrderAcrossAppendReplaceAppend(t *testing.T) {
	t.Parallel()

	first := QName{Namespace: 1, Local: 1}
	second := QName{Namespace: 1, Local: 2}
	rt := attributeUseMergeRuntime()
	compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, nil, NoWildcard, AttributeMergeDirect)

	if err := compilation.mergeUse(AttributeUse{Name: first, Type: 0, Prohibited: true}); err != nil {
		t.Fatal(err)
	}
	want := []AttributeUse{{Name: first, Type: 0, Prohibited: true}}
	if !slices.Equal(compilation.uses, want) {
		t.Fatalf("after append: uses = %#v, want %#v", compilation.uses, want)
	}
	if err := compilation.mergeUse(AttributeUse{Name: first, Type: 1}); err != nil {
		t.Fatal(err)
	}
	want = []AttributeUse{{Name: first, Type: 1}}
	if !slices.Equal(compilation.uses, want) {
		t.Fatalf("after replace: uses = %#v, want %#v", compilation.uses, want)
	}
	if err := compilation.mergeUse(AttributeUse{Name: second, Type: 2}); err != nil {
		t.Fatal(err)
	}
	want = []AttributeUse{{Name: first, Type: 1}, {Name: second, Type: 2}}
	if !slices.Equal(compilation.uses, want) {
		t.Fatalf("after second append: uses = %#v, want %#v", compilation.uses, want)
	}
}

func TestAttributeUseCompilationRestrictionWorkFailureAllowsRetry(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: 1, Local: 1}
	rt := attributeUseMergeRuntime()
	c := &compiler{rt: *rt.build()}
	exhausted := newWorkBudget(dependencyWorkBudget, 0)
	c.rt.typeWork = &exhausted
	compilation := newAttributeUseCompilation(c, nil, []AttributeUse{{Name: name, Type: 0, Required: true}}, NoWildcard, AttributeMergeRestriction)

	err := compilation.mergeUse(AttributeUse{Name: name, Type: 1, Required: true})
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	want := []AttributeUse{{Name: name, Type: 0, Required: true}}
	if !slices.Equal(compilation.uses, want) {
		t.Fatalf("work failure changed uses: %#v, want %#v", compilation.uses, want)
	}

	sufficient := newWorkBudget(dependencyWorkBudget, 64)
	c.rt.typeWork = &sufficient
	if err := compilation.mergeUse(AttributeUse{Name: name, Type: 1, Required: true}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	want = []AttributeUse{{Name: name, Type: 1, Required: true}}
	if !slices.Equal(compilation.uses, want) {
		t.Fatalf("retried uses = %#v, want %#v", compilation.uses, want)
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

func attributeUseMergeRuntime() compiledModelFixture {
	return compiledModelFixture{
		simpleDerivations: map[SimpleTypeID]SimpleTypeDerivation{
			0: {Base: NoSimpleType, Variety: valuepkg.Atomic},
			1: {Base: 0, Variety: valuepkg.Atomic},
			2: {Base: NoSimpleType, Variety: valuepkg.Atomic},
		},
		complexDerivations: map[ComplexTypeID]ComplexTypeDerivation{
			0: {Kind: DerivationKindNone},
		},
		wildcards: map[WildcardID]Wildcard{
			1: {Mode: WildcardList, Namespaces: []NamespaceID{1}, Process: ProcessStrict},
		},
	}
}
