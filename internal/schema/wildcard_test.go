package schema

import (
	"slices"
	"strings"
	"testing"
)

func TestWildcardsOverlapBoundarySets(t *testing.T) {
	t.Parallel()
	empty := wildcardRuntime{Mode: WildcardList}
	other := wildcardRuntime{Mode: WildcardOther, OtherThan: 1}
	for _, test := range []struct {
		name string
		a, b wildcardRuntime
		want bool
	}{
		{name: "empty and any", a: empty, b: wildcardRuntime{Mode: WildcardAny}},
		{name: "empty and other", a: empty, b: other},
		{name: "empty and local", a: empty, b: wildcardRuntime{Mode: WildcardLocal}},
		{name: "empty and finite", a: empty, b: wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{1, 2}}},
		{name: "empty and empty", a: empty, b: empty},
		{name: "other excludes every finite member", a: other, b: wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{emptyNamespaceID, 1}}},
		{name: "other admits a finite member", a: other, b: wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{emptyNamespaceID, 1, 2}}, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := wildcardsOverlap(test.a, test.b); got != test.want {
				t.Fatalf("WildcardsOverlap(%+v, %+v) = %v, want %v", test.a, test.b, got, test.want)
			}
			if got := wildcardsOverlap(test.b, test.a); got != test.want {
				t.Fatalf("WildcardsOverlap(%+v, %+v) = %v, want %v", test.b, test.a, got, test.want)
			}
		})
	}
}

func TestWildcardByID(t *testing.T) {
	t.Parallel()

	wildcards := []wildcardRuntime{
		{Mode: WildcardAny, Process: processStrict},
		{Mode: WildcardList, Process: ProcessLax, Namespaces: []namespaceID{1, 2}},
	}
	got, ok := wildcardByID(wildcards, 1)
	if !ok || got.Mode != WildcardList || !slices.Equal(got.Namespaces, []namespaceID{1, 2}) {
		t.Fatalf("WildcardByID(valid) = %+v, %v; want cloned wildcard 1, true", got, ok)
	}
	got.Namespaces[0] = 9
	if wildcards[1].Namespaces[0] != 1 {
		t.Fatalf("WildcardByID aliased namespace slice: %+v", wildcards[1])
	}
	for _, id := range []WildcardID{NoWildcard, 2} {
		got, ok := wildcardByID(wildcards, id)
		if ok || got.Mode != 0 || got.Process != 0 || got.OtherThan != 0 || len(got.Namespaces) != 0 {
			t.Fatalf("WildcardByID(%d) = %+v, %v; want zero, false", id, got, ok)
		}
	}
}

func TestValidateWildcard(t *testing.T) {
	t.Parallel()

	table, names := wildcardValidationFixture(t)
	tests := []struct {
		name    string
		wantErr string
		in      wildcardRuntime
	}{
		{
			name: "any",
			in:   wildcardRuntime{Mode: WildcardAny, Process: processStrict},
		},
		{
			name: "local",
			in:   wildcardRuntime{Mode: WildcardLocal, Process: ProcessLax},
		},
		{
			name: "other",
			in:   wildcardRuntime{Mode: WildcardOther, OtherThan: names["urn:a"], Process: ProcessSkip},
		},
		{
			name: "target namespace",
			in: wildcardRuntime{
				Mode:       WildcardTargetNamespace,
				Namespaces: []namespaceID{names["urn:a"]},
				Process:    processStrict,
			},
		},
		{
			name: "list",
			in: wildcardRuntime{
				Mode:       WildcardList,
				Namespaces: []namespaceID{emptyNamespaceID, names["urn:a"], names["urn:b"]},
				Process:    ProcessLax,
			},
		},
		{
			name:    "invalid process",
			in:      wildcardRuntime{Mode: WildcardAny, Process: ProcessContents(99)},
			wantErr: "wildcard has invalid process contents",
		},
		{
			name:    "invalid mode",
			in:      wildcardRuntime{Mode: wildcardMode(99), Process: processStrict},
			wantErr: "wildcard has invalid mode",
		},
		{
			name: "any stores inactive namespace",
			in: wildcardRuntime{
				Mode:       WildcardAny,
				Namespaces: []namespaceID{names["urn:a"]},
				Process:    processStrict,
			},
			wantErr: "wildcard stores inactive namespace fields",
		},
		{
			name:    "other invalid namespace",
			in:      wildcardRuntime{Mode: WildcardOther, OtherThan: namespaceID(99), Process: processStrict},
			wantErr: "wildcard other namespace is invalid",
		},
		{
			name: "target namespace stores inactive other",
			in: wildcardRuntime{
				Mode:       WildcardTargetNamespace,
				Namespaces: []namespaceID{names["urn:a"]},
				OtherThan:  names["urn:b"],
				Process:    processStrict,
			},
			wantErr: "targetNamespace wildcard has invalid namespace",
		},
		{
			name: "list stores inactive other",
			in: wildcardRuntime{
				Mode:       WildcardList,
				Namespaces: []namespaceID{names["urn:a"]},
				OtherThan:  names["urn:b"],
				Process:    processStrict,
			},
			wantErr: "namespace list wildcard stores inactive other namespace",
		},
		{
			name: "list invalid namespace",
			in: wildcardRuntime{
				Mode:       WildcardList,
				Namespaces: []namespaceID{namespaceID(99)},
				Process:    processStrict,
			},
			wantErr: "namespace list wildcard is invalid",
		},
		{
			name: "list is not sorted",
			in: wildcardRuntime{
				Mode:       WildcardList,
				Namespaces: []namespaceID{names["urn:b"], names["urn:a"]},
				Process:    processStrict,
			},
			wantErr: "namespace list wildcard is invalid",
		},
		{
			name: "list has duplicate namespace",
			in: wildcardRuntime{
				Mode:       WildcardList,
				Namespaces: []namespaceID{names["urn:a"], names["urn:a"]},
				Process:    processStrict,
			},
			wantErr: "namespace list wildcard is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateWildcard(&table, tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateWildcard() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateWildcard() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWildcardRejectsNilNameTable(t *testing.T) {
	t.Parallel()

	err := validateWildcard(nil, wildcardRuntime{Mode: WildcardAny, Process: processStrict})
	if err == nil || !strings.Contains(err.Error(), "wildcard requires name table") {
		t.Fatalf("ValidateWildcard(nil, wildcard) error = %v", err)
	}
}

func TestWildcardSetPredicates(t *testing.T) {
	t.Parallel()

	_, names := wildcardValidationFixture(t)
	anyWildcard := wildcardRuntime{Mode: WildcardAny, Process: processStrict}
	otherA := wildcardRuntime{Mode: WildcardOther, OtherThan: names["urn:a"], Process: processStrict}
	local := wildcardRuntime{Mode: WildcardLocal, Process: processStrict}
	listAB := wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{names["urn:a"], names["urn:b"]}, Process: processStrict}
	if !wildcardAllowsNamespace(anyWildcard, names["urn:a"]) {
		t.Fatal("##any did not allow interned namespace")
	}
	if wildcardAllowsNamespace(otherA, names["urn:a"]) || !wildcardAllowsNamespace(otherA, names["urn:b"]) {
		t.Fatal("##other namespace admission is wrong")
	}
	if !wildcardAllowsNamespace(local, emptyNamespaceID) || wildcardAllowsNamespace(local, names["urn:a"]) {
		t.Fatal("##local namespace admission is wrong")
	}
	if !wildcardSubset(listAB, anyWildcard) || wildcardSubset(anyWildcard, listAB) {
		t.Fatal("wildcard subset relation is wrong")
	}
	if !wildcardsOverlap(otherA, listAB) || wildcardsOverlap(local, listAB) {
		t.Fatal("wildcard overlap relation is wrong")
	}
	if !wildcardNamespaceEqual(listAB, wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{names["urn:a"], names["urn:b"]}}) {
		t.Fatal("equal wildcard namespace sets were not recognized")
	}
}

func TestWildcardSetOperators(t *testing.T) {
	t.Parallel()

	_, names := wildcardValidationFixture(t)
	normalized := normalizeNamespaceList([]namespaceID{names["urn:b"], names["urn:a"], names["urn:a"]})
	if !slices.Equal(normalized, []namespaceID{names["urn:a"], names["urn:b"]}) {
		t.Fatalf("NormalizeNamespaceList() = %v", normalized)
	}

	listA := wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{names["urn:a"]}, Process: processStrict}
	listB := wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{names["urn:b"]}, Process: ProcessLax}
	union, err := unionWildcard(listA, listB, ProcessSkip)
	if err != nil {
		t.Fatalf("UnionWildcard() error = %v", err)
	}
	if union.Mode != WildcardList || union.Process != ProcessSkip || !slices.Equal(union.Namespaces, []namespaceID{names["urn:a"], names["urn:b"]}) {
		t.Fatalf("UnionWildcard() = %+v", union)
	}
	sameList := wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{names["urn:a"]}, Process: processStrict}
	sameUnion, err := unionWildcard(sameList, sameList, ProcessLax)
	if err != nil {
		t.Fatalf("UnionWildcard(equal list) error = %v", err)
	}
	sameList.Namespaces[0] = names["urn:b"]
	if !slices.Equal(sameUnion.Namespaces, []namespaceID{names["urn:a"]}) {
		t.Fatalf("UnionWildcard(equal list) aliased input namespace slice: %+v", sameUnion)
	}

	otherA := wildcardRuntime{Mode: WildcardOther, OtherThan: names["urn:a"], Process: processStrict}
	union, err = unionWildcard(otherA, listA, processStrict)
	if err != nil {
		t.Fatalf("UnionWildcard(other, negated namespace) error = %v", err)
	}
	if union.Mode != WildcardOther || union.OtherThan != emptyNamespaceID {
		t.Fatalf("UnionWildcard(other, negated namespace) = %+v", union)
	}
	if _, unionErr := unionWildcard(otherA, wildcardRuntime{Mode: WildcardLocal, Process: processStrict}, processStrict); unionErr == nil {
		t.Fatal("unexpressible wildcard union succeeded")
	}

	intersection, err := intersectWildcard(wildcardRuntime{Mode: WildcardAny, Process: processStrict}, listB, ProcessSkip)
	if err != nil {
		t.Fatalf("IntersectWildcard(any, list) error = %v", err)
	}
	if !wildcardNamespaceEqual(intersection, listB) || intersection.Process != ProcessSkip {
		t.Fatalf("IntersectWildcard(any, list) = %+v", intersection)
	}
	listB.Namespaces[0] = names["urn:a"]
	if !slices.Equal(intersection.Namespaces, []namespaceID{names["urn:b"]}) {
		t.Fatalf("IntersectWildcard(any, list) aliased input namespace slice: %+v", intersection)
	}
	_, err = intersectWildcard(
		wildcardRuntime{Mode: WildcardOther, OtherThan: names["urn:a"], Process: processStrict},
		wildcardRuntime{Mode: WildcardOther, OtherThan: names["urn:b"], Process: processStrict},
		processStrict,
	)
	if err == nil {
		t.Fatalf("IntersectWildcard(other a, other b) error = %v, want error", err)
	}
}

func wildcardValidationFixture(t *testing.T) (nameTable, map[string]namespaceID) {
	t.Helper()

	table, err := newNameTable(16, []string{EmptyNamespaceURI, "urn:a", "urn:b"}, nil)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	names := map[string]namespaceID{EmptyNamespaceURI: emptyNamespaceID}
	for _, uri := range []string{"urn:a", "urn:b"} {
		id, ok := table.LookupNamespace(uri)
		if !ok {
			t.Fatalf("LookupNamespace(%q) failed", uri)
		}
		names[uri] = id
	}
	return table, names
}
