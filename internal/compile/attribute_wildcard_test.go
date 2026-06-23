package compile

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeWildcardBuilderRestriction(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	base := store.mustAdd(runtime.Wildcard{Mode: runtime.WildcardAny, Process: runtime.ProcessStrict})
	subset := store.mustAdd(runtime.Wildcard{Mode: runtime.WildcardList, Namespaces: []runtime.NamespaceID{1}, Process: runtime.ProcessStrict})

	builder := NewAttributeWildcardBuilder(base, AttributeMergeRestriction)
	if err := builder.AddAnyAttribute(store, subset); err != nil {
		t.Fatalf("AddAnyAttribute() error = %v", err)
	}
	got, err := builder.Finish(store, false)
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if got != subset {
		t.Fatalf("restriction wildcard = %d, want %d", got, subset)
	}
}

func TestAttributeWildcardBuilderRejectsRestrictionWithoutBase(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	declared := store.mustAdd(runtime.Wildcard{Mode: runtime.WildcardAny, Process: runtime.ProcessStrict})
	builder := NewAttributeWildcardBuilder(runtime.NoWildcard, AttributeMergeRestriction)
	if err := builder.AddAnyAttribute(store, declared); err != nil {
		t.Fatalf("AddAnyAttribute() error = %v", err)
	}

	_, err := builder.Finish(store, false)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
}

func TestAttributeWildcardBuilderRejectsRestrictionOutsideBase(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	base := store.mustAdd(runtime.Wildcard{Mode: runtime.WildcardList, Namespaces: []runtime.NamespaceID{1}, Process: runtime.ProcessStrict})
	declared := store.mustAdd(runtime.Wildcard{Mode: runtime.WildcardAny, Process: runtime.ProcessLax})
	builder := NewAttributeWildcardBuilder(base, AttributeMergeRestriction)
	if err := builder.AddAnyAttribute(store, declared); err != nil {
		t.Fatalf("AddAnyAttribute() error = %v", err)
	}

	_, err := builder.Finish(store, false)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
}

func TestAttributeWildcardBuilderExtensionUnionsDeclaredWithInherited(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	inherited := store.mustAdd(runtime.Wildcard{Mode: runtime.WildcardList, Namespaces: []runtime.NamespaceID{1}, Process: runtime.ProcessStrict})
	declared := store.mustAdd(runtime.Wildcard{Mode: runtime.WildcardList, Namespaces: []runtime.NamespaceID{2}, Process: runtime.ProcessSkip})
	builder := NewAttributeWildcardBuilder(inherited, AttributeMergeNormal)
	if err := builder.AddAnyAttribute(store, declared); err != nil {
		t.Fatalf("AddAnyAttribute() error = %v", err)
	}

	got, err := builder.Finish(store, true)
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	wildcard, ok := store.Wildcard(got)
	if !ok {
		t.Fatalf("union wildcard %d was not stored", got)
	}
	if wildcard.Mode != runtime.WildcardList || wildcard.Process != runtime.ProcessSkip {
		t.Fatalf("union wildcard = %#v, want list with declared process", wildcard)
	}
	if len(wildcard.Namespaces) != 2 || wildcard.Namespaces[0] != 1 || wildcard.Namespaces[1] != 2 {
		t.Fatalf("union namespaces = %v, want [1 2]", wildcard.Namespaces)
	}
}

func TestAttributeWildcardBuilderIntersectsGroupWithDeclaredProcess(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	declared := store.mustAdd(runtime.Wildcard{Mode: runtime.WildcardAny, Process: runtime.ProcessLax})
	group := store.mustAdd(runtime.Wildcard{Mode: runtime.WildcardList, Namespaces: []runtime.NamespaceID{1}, Process: runtime.ProcessStrict})
	builder := NewAttributeWildcardBuilder(runtime.NoWildcard, AttributeMergeNormal)
	if err := builder.AddAnyAttribute(store, declared); err != nil {
		t.Fatalf("AddAnyAttribute() error = %v", err)
	}
	if err := builder.AddGroup(store, group); err != nil {
		t.Fatalf("AddGroup() error = %v", err)
	}

	wildcard, ok := store.Wildcard(builder.Declared())
	if !ok {
		t.Fatalf("declared wildcard %d was not stored", builder.Declared())
	}
	if wildcard.Mode != runtime.WildcardList || wildcard.Process != runtime.ProcessLax {
		t.Fatalf("intersection wildcard = %#v, want list with declared process", wildcard)
	}
}

func TestAttributeWildcardDerivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		extension bool
		mode      AttributeMergeMode
		want      runtime.AttributeWildcardDerivation
	}{
		{name: "none", want: runtime.AttributeWildcardNone},
		{name: "extension", extension: true, want: runtime.AttributeWildcardExtension},
		{name: "restriction", extension: true, mode: AttributeMergeRestriction, want: runtime.AttributeWildcardRestriction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := AttributeWildcardDerivation(tt.extension, tt.mode); got != tt.want {
				t.Fatalf("AttributeWildcardDerivation() = %d, want %d", got, tt.want)
			}
		})
	}
}

type attributeWildcardStore struct {
	wildcards []runtime.Wildcard
}

func newAttributeWildcardStore() *attributeWildcardStore {
	return &attributeWildcardStore{
		wildcards: []runtime.Wildcard{{Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}},
	}
}

func (s *attributeWildcardStore) Wildcard(id runtime.WildcardID) (runtime.Wildcard, bool) {
	if !runtime.ValidUint32Index(uint32(id), len(s.wildcards)) {
		return runtime.Wildcard{}, false
	}
	return s.wildcards[id], true
}

func (s *attributeWildcardStore) AddWildcard(w runtime.Wildcard) (runtime.WildcardID, error) {
	id, err := checkedUint32(len(s.wildcards), "wildcard limit exceeded")
	if err != nil {
		return runtime.NoWildcard, err
	}
	s.wildcards = append(s.wildcards, w)
	return runtime.WildcardID(id), nil
}

func (s *attributeWildcardStore) mustAdd(w runtime.Wildcard) runtime.WildcardID {
	id, err := s.AddWildcard(w)
	if err != nil {
		panic(err)
	}
	return id
}
