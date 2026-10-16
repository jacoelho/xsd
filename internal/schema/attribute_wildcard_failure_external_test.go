package schema_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/schema"
)

func TestAttributeWildcardIntersectionStorageFailureAllowsRetry(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		group   bool
		process schema.ProcessContents
	}{
		{name: "group", group: true, process: schema.ProcessStrict},
		{name: "local", process: schema.ProcessLax},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &failingAttributeWildcardStore{wildcards: []schema.Wildcard{
				{Mode: schema.WildcardList, Namespaces: []schema.NamespaceID{1, 2}, Process: schema.ProcessStrict},
				{Mode: schema.WildcardList, Namespaces: []schema.NamespaceID{2, 3}, Process: schema.ProcessLax},
			}}
			builder := schema.NewAttributeWildcardBuilder(schema.NoWildcard, schema.AttributeMergeDirect)
			if err := builder.AddAnyAttribute(store, 0); err != nil {
				t.Fatal(err)
			}
			add := func() error {
				if tc.group {
					return builder.AddGroup(store, 1)
				}
				return builder.AddAnyAttribute(store, 1)
			}
			failure := errors.New("wildcard storage unavailable")
			store.failure = failure
			if err := add(); !errors.Is(err, failure) {
				t.Fatalf("storage error = %v, want %v", err, failure)
			}
			if builder.Declared() != 0 {
				t.Fatal("failed intersection replaced the declared wildcard")
			}
			store.failure = nil
			if err := add(); err != nil {
				t.Fatalf("intersection retry: %v", err)
			}
			wildcard, ok := store.Wildcard(builder.Declared())
			if !ok || wildcard.Mode != schema.WildcardList || wildcard.Process != tc.process || !slices.Equal(wildcard.Namespaces, []schema.NamespaceID{2}) {
				t.Fatalf("retried intersection = %#v, want namespace 2 and process %v", wildcard, tc.process)
			}
		})
	}
}

func TestAttributeWildcardExtensionStorageFailureAllowsRetry(t *testing.T) {
	t.Parallel()

	store := &failingAttributeWildcardStore{wildcards: []schema.Wildcard{
		{Mode: schema.WildcardList, Namespaces: []schema.NamespaceID{1}, Process: schema.ProcessStrict},
		{Mode: schema.WildcardList, Namespaces: []schema.NamespaceID{2}, Process: schema.ProcessLax},
	}}
	builder := schema.NewAttributeWildcardBuilder(0, schema.AttributeMergeExtension)
	if err := builder.AddAnyAttribute(store, 1); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("wildcard storage unavailable")
	store.failure = failure
	if id, err := builder.Finish(store); id != schema.NoWildcard || !errors.Is(err, failure) {
		t.Fatalf("failed extension = %v, %v; want no wildcard and %v", id, err, failure)
	}
	if builder.Declared() != 1 {
		t.Fatal("failed extension replaced the local declaration")
	}
	store.failure = nil
	id, err := builder.Finish(store)
	if err != nil {
		t.Fatalf("extension retry: %v", err)
	}
	wildcard, ok := store.Wildcard(id)
	if !ok || wildcard.Mode != schema.WildcardList || wildcard.Process != schema.ProcessLax || !slices.Equal(wildcard.Namespaces, []schema.NamespaceID{1, 2}) {
		t.Fatalf("retried extension = %#v, want namespaces 1,2 with lax processing", wildcard)
	}
	if builder.Declared() != 1 {
		t.Fatal("extension result replaced the local declaration")
	}
}

type failingAttributeWildcardStore struct {
	wildcards []schema.Wildcard
	failure   error
}

func (s *failingAttributeWildcardStore) Wildcard(id schema.WildcardID) (schema.Wildcard, bool) {
	if uint64(id) >= uint64(len(s.wildcards)) {
		return schema.Wildcard{}, false
	}
	return s.wildcards[id], true
}

func (s *failingAttributeWildcardStore) AddWildcard(w schema.Wildcard) (schema.WildcardID, error) {
	if s.failure != nil {
		return schema.NoWildcard, s.failure
	}
	index, err := schema.CheckedUint32Index(len(s.wildcards), "test wildcard storage exhausted")
	if err != nil {
		return schema.NoWildcard, err
	}
	id := schema.WildcardID(index)
	s.wildcards = append(s.wildcards, w)
	return id, nil
}
