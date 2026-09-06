package schema

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeWildcardBuilderRestriction(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	base := store.mustAdd(Wildcard{Mode: WildcardAny, Process: ProcessStrict})
	subset := store.mustAdd(Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{1}, Process: ProcessStrict})

	builder := NewAttributeWildcardBuilder(base, AttributeMergeRestriction)
	if err := builder.AddAnyAttribute(store, subset); err != nil {
		t.Fatalf("AddAnyAttribute() error = %v", err)
	}
	got, err := builder.Finish(store)
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
	declared := store.mustAdd(Wildcard{Mode: WildcardAny, Process: ProcessStrict})
	builder := NewAttributeWildcardBuilder(NoWildcard, AttributeMergeRestriction)
	if err := builder.AddAnyAttribute(store, declared); err != nil {
		t.Fatalf("AddAnyAttribute() error = %v", err)
	}

	_, err := builder.Finish(store)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
}

func TestAttributeWildcardBuilderRejectsRestrictionOutsideBase(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	base := store.mustAdd(Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{1}, Process: ProcessStrict})
	declared := store.mustAdd(Wildcard{Mode: WildcardAny, Process: ProcessLax})
	builder := NewAttributeWildcardBuilder(base, AttributeMergeRestriction)
	if err := builder.AddAnyAttribute(store, declared); err != nil {
		t.Fatalf("AddAnyAttribute() error = %v", err)
	}

	_, err := builder.Finish(store)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
}

func TestAttributeWildcardBuilderExtensionUnionsDeclaredWithInherited(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	inherited := store.mustAdd(Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{1}, Process: ProcessStrict})
	declared := store.mustAdd(Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{2}, Process: ProcessSkip})
	builder := NewAttributeWildcardBuilder(inherited, AttributeMergeExtension)
	if err := builder.AddAnyAttribute(store, declared); err != nil {
		t.Fatalf("AddAnyAttribute() error = %v", err)
	}

	got, err := builder.Finish(store)
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	wildcard, ok := store.Wildcard(got)
	if !ok {
		t.Fatalf("union wildcard %d was not stored", got)
	}
	if wildcard.Mode != WildcardList || wildcard.Process != ProcessSkip {
		t.Fatalf("union wildcard = %#v, want list with declared process", wildcard)
	}
	if len(wildcard.Namespaces) != 2 || wildcard.Namespaces[0] != 1 || wildcard.Namespaces[1] != 2 {
		t.Fatalf("union namespaces = %v, want [1 2]", wildcard.Namespaces)
	}
}

func TestAttributeWildcardBuilderIntersectsGroupWithDeclaredProcess(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	declared := store.mustAdd(Wildcard{Mode: WildcardAny, Process: ProcessLax})
	group := store.mustAdd(Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{1}, Process: ProcessStrict})
	builder := NewAttributeWildcardBuilder(NoWildcard, AttributeMergeDirect)
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
	if wildcard.Mode != WildcardList || wildcard.Process != ProcessLax {
		t.Fatalf("intersection wildcard = %#v, want list with declared process", wildcard)
	}
}

func TestAttributeWildcardDerivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    AttributeMergeMode
		want    AttributeWildcardDerivation
		wantErr bool
	}{
		{name: "direct", mode: AttributeMergeDirect, want: AttributeWildcardNone},
		{name: "extension", mode: AttributeMergeExtension, want: AttributeWildcardExtension},
		{name: "restriction", mode: AttributeMergeRestriction, want: AttributeWildcardRestriction},
		{name: "invalid", mode: AttributeMergeInvalid, wantErr: true},
		{name: "unknown", mode: AttributeMergeMode(255), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := SchemaAttributeWildcardDerivation(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AttributeWildcardDerivation() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("AttributeWildcardDerivation() = %d, want %d", got, tt.want)
			}
		})
	}
}

type attributeWildcardStore struct {
	wildcards []Wildcard
}

func newAttributeWildcardStore() *attributeWildcardStore {
	return &attributeWildcardStore{
		wildcards: []Wildcard{{Mode: WildcardAny, Process: ProcessStrict}},
	}
}

func (s *attributeWildcardStore) Wildcard(id WildcardID) (Wildcard, bool) {
	if !ValidUint32Index(uint32(id), len(s.wildcards)) {
		return Wildcard{}, false
	}
	return s.wildcards[id], true
}

func (s *attributeWildcardStore) AddWildcard(w Wildcard) (WildcardID, error) {
	id, err := checkedSchemaUint32(len(s.wildcards), "wildcard limit exceeded")
	if err != nil {
		return NoWildcard, err
	}
	s.wildcards = append(s.wildcards, w)
	return WildcardID(id), nil
}

func (s *attributeWildcardStore) mustAdd(w Wildcard) WildcardID {
	id, err := s.AddWildcard(w)
	if err != nil {
		panic(err)
	}
	return id
}
