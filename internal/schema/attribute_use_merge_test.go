package schema

import (
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
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

		compilation := newAttributeUseCompilation(nil, &compiler{rt: *rt.build()}, nil, NoWildcard, attributeMergeDirect)
		if err := compilation.mergeUse(attributeUse{Name: first, Type: fixtureRoot}); err != nil {
			t.Fatal(err)
		}
		err := compilation.mergeUse(attributeUse{Name: second, Type: fixtureRestriction})
		if err != nil {
			t.Fatalf("mergeUse() error = %v", err)
		}
		want := []attributeUse{{Name: first, Type: fixtureRoot}, {Name: second, Type: fixtureRestriction}}
		if !slices.Equal(compilation.uses, want) {
			t.Fatalf("uses = %#v, want %#v", compilation.uses, want)
		}
	})

	t.Run("rejects duplicate use", func(t *testing.T) {
		t.Parallel()

		compilation := newAttributeUseCompilation(nil, &compiler{rt: *rt.build()}, nil, NoWildcard, attributeMergeDirect)
		if err := compilation.mergeUse(attributeUse{Name: first, Type: fixtureRoot}); err != nil {
			t.Fatal(err)
		}
		err := compilation.mergeUse(attributeUse{Name: first, Type: fixtureRestriction})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaDuplicate)
		want := []attributeUse{{Name: first, Type: fixtureRoot}}
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

		uses := []attributeUse{
			{Name: first, Type: fixtureRoot, Required: true},
		}
		compilation := newAttributeUseCompilation(nil, &compiler{rt: *rt.build()}, uses, NoWildcard, attributeMergeRestriction)

		err := compilation.mergeUse(attributeUse{Name: first, Type: fixtureRestriction, Required: true})
		if err != nil {
			t.Fatalf("mergeUse() error = %v", err)
		}
		want := []attributeUse{{Name: first, Type: fixtureRestriction, Required: true}}
		if !slices.Equal(compilation.finalUses(), want) {
			t.Fatalf("uses = %#v, want %#v", compilation.finalUses(), want)
		}
		if uses[0].Type != fixtureRoot || !uses[0].Required {
			t.Fatalf("restriction changed inherited use: %#v", uses[0])
		}
	})

	t.Run("rejects invalid restricted use", func(t *testing.T) {
		t.Parallel()

		uses := []attributeUse{
			{Name: first, Type: fixtureRoot, Required: true},
		}
		compilation := newAttributeUseCompilation(nil, &compiler{rt: *rt.build()}, uses, NoWildcard, attributeMergeRestriction)

		err := compilation.mergeUse(attributeUse{Name: first, Type: fixtureUnrelated})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
		want := []attributeUse{{Name: first, Type: fixtureRoot, Required: true}}
		if !slices.Equal(compilation.finalUses(), want) {
			t.Fatalf("rejected restriction changed uses: %#v, want %#v", compilation.finalUses(), want)
		}
	})

	t.Run("admits new use through inherited wildcard", func(t *testing.T) {
		t.Parallel()

		compilation := newAttributeUseCompilation(nil, &compiler{rt: *rt.build()}, nil, 1, attributeMergeRestriction)
		err := compilation.mergeUse(attributeUse{Name: second, Type: fixtureRestriction})
		if err != nil {
			t.Fatalf("mergeUse() error = %v", err)
		}
		want := []attributeUse{{Name: second, Type: fixtureRestriction}}
		if !slices.Equal(compilation.uses, want) {
			t.Fatalf("uses = %#v, want %#v", compilation.uses, want)
		}
	})

	t.Run("rejects new use without inherited wildcard", func(t *testing.T) {
		t.Parallel()

		compilation := newAttributeUseCompilation(nil, &compiler{rt: *rt.build()}, nil, NoWildcard, attributeMergeRestriction)
		err := compilation.mergeUse(attributeUse{Name: second, Type: fixtureRestriction})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
		if compilation.uses != nil {
			t.Fatalf("rejected restriction changed uses: %#v, want nil", compilation.uses)
		}
	})

	t.Run("rejects new use outside inherited wildcard", func(t *testing.T) {
		t.Parallel()

		compilation := newAttributeUseCompilation(nil, &compiler{rt: *rt.build()}, nil, 1, attributeMergeRestriction)
		err := compilation.mergeUse(attributeUse{Name: foreign, Type: fixtureRestriction})
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
	actual := attributeUse{Name: name, Type: fixtureRestriction}
	for _, mode := range []attributeMergeMode{attributeMergeDirect, attributeMergeExtension, attributeMergeRestriction} {
		for _, prohibitFirst := range []bool{true, false} {
			var inherited []attributeUse
			if mode == attributeMergeRestriction {
				inherited = []attributeUse{{Name: name, Type: fixtureRoot}}
			}
			compilation := newAttributeUseCompilation(nil, &compiler{rt: *rt.build()}, inherited, NoWildcard, mode)
			if prohibitFirst {
				compilation.addProhibition(name)
			}
			if err := compilation.mergeUse(actual); err != nil {
				t.Fatal(err)
			}
			if !prohibitFirst {
				compilation.addProhibition(name)
			}
			if got := compilation.finalUses(); !slices.Equal(got, []attributeUse{actual}) {
				t.Fatalf("mode %d, prohibition first %t: uses = %#v, want explicit use", mode, prohibitFirst, got)
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
	compilation := newAttributeUseCompilation(nil, c, []attributeUse{{Name: name, Type: fixtureRoot, Required: true}}, NoWildcard, attributeMergeRestriction)

	err := compilation.mergeUse(attributeUse{Name: name, Type: fixtureRestriction, Required: true})
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	want := []attributeUse{{Name: name, Type: fixtureRoot, Required: true}}
	if !slices.Equal(compilation.finalUses(), want) {
		t.Fatalf("work failure changed uses: %#v, want %#v", compilation.finalUses(), want)
	}

	sufficient := newWorkBudget(dependencyWorkBudget, 64)
	c.rt.typeWork = &sufficient
	if err := compilation.mergeUse(attributeUse{Name: name, Type: fixtureRestriction, Required: true}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	want = []attributeUse{{Name: name, Type: fixtureRestriction, Required: true}}
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
	_, err := compileSources(Options{}, []source.Source{source.Bytes("schema.xsd", []byte(document))})
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
}

func attributeUseMergeRuntime() compiledModelFixture {
	return compiledModelFixture{
		simpleDerivations: restrictionFixtureTypes(),
		complexDerivations: map[ComplexTypeID]complexTypeDerivation{
			0: {Kind: derivationKindNone},
		},
		wildcards: map[WildcardID]wildcardRuntime{
			1: {Mode: WildcardList, Namespaces: []namespaceID{1}, Process: processStrict},
		},
	}
}
