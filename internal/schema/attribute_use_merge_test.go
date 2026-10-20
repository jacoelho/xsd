package schema

import (
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeUseIdentityExhaustion(t *testing.T) {
	var c compiler
	c.attributeUseCount = ^uint32(0) - 1
	if id, err := c.allocateAttributeUseID(); err != nil || id != ^uint32(0) {
		t.Fatalf("last identity = %d, error = %v", id, err)
	}
	id, err := c.allocateAttributeUseID()
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	if id != 0 || c.attributeUseCount != ^uint32(0) {
		t.Fatalf("exhaustion reused an identity: returned %d, retained %d", id, c.attributeUseCount)
	}
}

func TestAttributeUseCompilationNormalMode(t *testing.T) {
	t.Parallel()

	first := QName{Namespace: 1, Local: 1}
	second := QName{Namespace: 1, Local: 2}
	rt := attributeUseMergeRuntime()

	t.Run("appends new use", func(t *testing.T) {
		t.Parallel()

		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, nil, NoWildcard, AttributeMergeDirect)
		if err := compilation.mergeUse(AttributeUse{Name: first, Type: 0}); err != nil {
			t.Fatal(err)
		}
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

		compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, nil, NoWildcard, AttributeMergeDirect)
		if err := compilation.mergeUse(AttributeUse{Name: first, Type: 0}); err != nil {
			t.Fatal(err)
		}
		err := compilation.mergeUse(AttributeUse{Name: first, Type: 1})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaDuplicate)
		want := []AttributeUse{{Name: first, Type: 0}}
		if !slices.Equal(compilation.uses, want) {
			t.Fatalf("rejected duplicate changed uses: %#v, want %#v", compilation.uses, want)
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
		if !slices.Equal(compilation.finalUses(), want) {
			t.Fatalf("uses = %#v, want %#v", compilation.finalUses(), want)
		}
		if uses[0].Type != 0 || !uses[0].Required {
			t.Fatalf("restriction changed inherited use: %#v", uses[0])
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
		if !slices.Equal(compilation.finalUses(), want) {
			t.Fatalf("rejected restriction changed uses: %#v, want %#v", compilation.finalUses(), want)
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

func TestAttributeUseCompilationProhibitionPreservesExplicitUses(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: 1, Local: 1}
	rt := attributeUseMergeRuntime()
	actual := AttributeUse{Name: name, Type: 1}
	prohibited := AttributeUse{Name: name, Type: 0, Prohibited: true}
	for _, mode := range []AttributeMergeMode{AttributeMergeDirect, AttributeMergeExtension, AttributeMergeRestriction} {
		for _, inputs := range [][]AttributeUse{{prohibited, actual}, {actual, prohibited}} {
			var inherited []AttributeUse
			if mode == AttributeMergeRestriction {
				inherited = []AttributeUse{{Name: name, Type: 0}}
			}
			compilation := newAttributeUseCompilation(&compiler{rt: *rt.build()}, nil, inherited, NoWildcard, mode)
			for _, use := range inputs {
				if err := compilation.mergeUse(use); err != nil {
					t.Fatal(err)
				}
			}
			if got := compilation.finalUses(); !slices.Equal(got, []AttributeUse{actual}) {
				t.Fatalf("mode %d, inputs %#v: uses = %#v, want explicit use", mode, inputs, got)
			}
		}
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
	if !slices.Equal(compilation.finalUses(), want) {
		t.Fatalf("work failure changed uses: %#v, want %#v", compilation.finalUses(), want)
	}

	sufficient := newWorkBudget(dependencyWorkBudget, 64)
	c.rt.typeWork = &sufficient
	if err := compilation.mergeUse(AttributeUse{Name: name, Type: 1, Required: true}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	want = []AttributeUse{{Name: name, Type: 1, Required: true}}
	if !slices.Equal(compilation.finalUses(), want) {
		t.Fatalf("retried uses = %#v, want %#v", compilation.finalUses(), want)
	}
}

func TestAttributeUseRestrictionRejectsRequiredBaseOmission(t *testing.T) {
	t.Parallel()

	const document = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:attribute name="a" type="xs:string" use="required"/></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:restriction base="Base">
    <xs:attribute name="a" type="xs:string" use="prohibited"/>
  </xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`
	_, err := Compile(Options{}, []source.Source{source.Bytes("schema.xsd", []byte(document))})
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
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
