package schema

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeWildcardBuilderRestriction(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	base := store.mustAdd(wildcardRuntime{Mode: WildcardAny, Process: processStrict})
	subset := store.mustAdd(wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{1}, Process: processStrict})

	builder := newAttributeWildcardBuilder(base, attributeMergeRestriction)
	if err := builder.addAnyAttribute(store, subset); err != nil {
		t.Fatalf("addAnyAttribute() error = %v", err)
	}
	got, err := builder.finish(store)
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
	declared := store.mustAdd(wildcardRuntime{Mode: WildcardAny, Process: processStrict})
	builder := newAttributeWildcardBuilder(NoWildcard, attributeMergeRestriction)
	if err := builder.addAnyAttribute(store, declared); err != nil {
		t.Fatalf("addAnyAttribute() error = %v", err)
	}

	_, err := builder.finish(store)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
}

func TestAttributeWildcardBuilderRejectsRestrictionOutsideBase(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	base := store.mustAdd(wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{1}, Process: processStrict})
	declared := store.mustAdd(wildcardRuntime{Mode: WildcardAny, Process: ProcessLax})
	builder := newAttributeWildcardBuilder(base, attributeMergeRestriction)
	if err := builder.addAnyAttribute(store, declared); err != nil {
		t.Fatalf("addAnyAttribute() error = %v", err)
	}

	_, err := builder.finish(store)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
}

func TestAttributeWildcardBuilderExtensionUnionsDeclaredWithInherited(t *testing.T) {
	t.Parallel()

	store := newAttributeWildcardStore()
	inherited := store.mustAdd(wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{1}, Process: processStrict})
	declared := store.mustAdd(wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{2}, Process: ProcessSkip})
	builder := newAttributeWildcardBuilder(inherited, attributeMergeExtension)
	if err := builder.addAnyAttribute(store, declared); err != nil {
		t.Fatalf("addAnyAttribute() error = %v", err)
	}

	got, err := builder.finish(store)
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
	declared := store.mustAdd(wildcardRuntime{Mode: WildcardAny, Process: ProcessLax})
	group := store.mustAdd(wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{1}, Process: processStrict})
	builder := newAttributeWildcardBuilder(NoWildcard, attributeMergeDirect)
	if err := builder.addAnyAttribute(store, declared); err != nil {
		t.Fatalf("addAnyAttribute() error = %v", err)
	}
	if err := builder.addGroup(store, group); err != nil {
		t.Fatalf("addGroup() error = %v", err)
	}

	wildcard, ok := store.Wildcard(builder.declared())
	if !ok {
		t.Fatalf("declared wildcard %d was not stored", builder.declared())
	}
	if wildcard.Mode != WildcardList || wildcard.Process != ProcessLax {
		t.Fatalf("intersection wildcard = %#v, want list with declared process", wildcard)
	}
}

func TestAttributeWildcardDerivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    attributeMergeMode
		want    attributeWildcardDerivation
		wantErr bool
	}{
		{name: "direct", mode: attributeMergeDirect, want: attributeWildcardNone},
		{name: "extension", mode: attributeMergeExtension, want: attributeWildcardExtension},
		{name: "restriction", mode: attributeMergeRestriction, want: attributeWildcardRestriction},
		{name: "invalid", mode: attributeMergeInvalid, wantErr: true},
		{name: "unknown", mode: attributeMergeMode(255), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := schemaAttributeWildcardDerivation(tt.mode)
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
	wildcards []wildcardRuntime
	failure   error
}

func newAttributeWildcardStore() *attributeWildcardStore {
	return &attributeWildcardStore{
		wildcards: []wildcardRuntime{{Mode: WildcardAny, Process: processStrict}},
	}
}

func (s *attributeWildcardStore) Wildcard(id WildcardID) (wildcardRuntime, bool) {
	if !validUint32Index(uint32(id), len(s.wildcards)) {
		return wildcardRuntime{}, false
	}
	return s.wildcards[id], true
}

func (s *attributeWildcardStore) AddWildcard(w wildcardRuntime) (WildcardID, error) {
	if s.failure != nil {
		return NoWildcard, s.failure
	}
	id, err := checkedSchemaUint32(len(s.wildcards), "wildcard limit exceeded")
	if err != nil {
		return NoWildcard, err
	}
	s.wildcards = append(s.wildcards, w)
	return WildcardID(id), nil
}

func (s *attributeWildcardStore) mustAdd(w wildcardRuntime) WildcardID {
	id, err := s.AddWildcard(w)
	if err != nil {
		panic(err)
	}
	return id
}

func TestAttributeWildcardIntersectionStorageFailureAllowsRetry(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		group   bool
		process ProcessContents
	}{
		{name: "group", group: true, process: processStrict},
		{name: "local", process: ProcessLax},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &attributeWildcardStore{wildcards: []wildcardRuntime{
				{Mode: WildcardList, Namespaces: []namespaceID{1, 2}, Process: processStrict},
				{Mode: WildcardList, Namespaces: []namespaceID{2, 3}, Process: ProcessLax},
			}}
			builder := newAttributeWildcardBuilder(NoWildcard, attributeMergeDirect)
			if err := builder.addAnyAttribute(store, 0); err != nil {
				t.Fatal(err)
			}
			add := func() error {
				if tc.group {
					return builder.addGroup(store, 1)
				}
				return builder.addAnyAttribute(store, 1)
			}
			failure := errors.New("wildcard storage unavailable")
			store.failure = failure
			if err := add(); !errors.Is(err, failure) {
				t.Fatalf("storage error = %v, want %v", err, failure)
			}
			if builder.declared() != 0 {
				t.Fatal("failed intersection replaced the declared wildcard")
			}
			store.failure = nil
			if err := add(); err != nil {
				t.Fatalf("intersection retry: %v", err)
			}
			wildcard, ok := store.Wildcard(builder.declared())
			if !ok || wildcard.Mode != WildcardList || wildcard.Process != tc.process || !slices.Equal(wildcard.Namespaces, []namespaceID{2}) {
				t.Fatalf("retried intersection = %#v, want namespace 2 and process %v", wildcard, tc.process)
			}
		})
	}
}

func TestAttributeWildcardExtensionStorageFailureAllowsRetry(t *testing.T) {
	t.Parallel()

	store := &attributeWildcardStore{wildcards: []wildcardRuntime{
		{Mode: WildcardList, Namespaces: []namespaceID{1}, Process: processStrict},
		{Mode: WildcardList, Namespaces: []namespaceID{2}, Process: ProcessLax},
	}}
	builder := newAttributeWildcardBuilder(0, attributeMergeExtension)
	if err := builder.addAnyAttribute(store, 1); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("wildcard storage unavailable")
	store.failure = failure
	if id, err := builder.finish(store); id != NoWildcard || !errors.Is(err, failure) {
		t.Fatalf("failed extension = %v, %v; want no wildcard and %v", id, err, failure)
	}
	if builder.declared() != 1 {
		t.Fatal("failed extension replaced the local declaration")
	}
	store.failure = nil
	id, err := builder.finish(store)
	if err != nil {
		t.Fatalf("extension retry: %v", err)
	}
	wildcard, ok := store.Wildcard(id)
	if !ok || wildcard.Mode != WildcardList || wildcard.Process != ProcessLax || !slices.Equal(wildcard.Namespaces, []namespaceID{1, 2}) {
		t.Fatalf("retried extension = %#v, want namespaces 1,2 with lax processing", wildcard)
	}
	if builder.declared() != 1 {
		t.Fatal("extension result replaced the local declaration")
	}
}
