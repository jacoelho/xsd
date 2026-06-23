package runtime

import (
	"slices"
	"strings"
	"testing"
)

func TestWildcardByID(t *testing.T) {
	t.Parallel()

	wildcards := []Wildcard{
		{Mode: WildcardAny, Process: ProcessStrict},
		{Mode: WildcardList, Process: ProcessLax, Namespaces: []NamespaceID{1, 2}},
	}
	got, ok := WildcardByID(wildcards, 1)
	if !ok || got.Mode != WildcardList || !slices.Equal(got.Namespaces, []NamespaceID{1, 2}) {
		t.Fatalf("WildcardByID(valid) = %+v, %v; want cloned wildcard 1, true", got, ok)
	}
	got.Namespaces[0] = 9
	if wildcards[1].Namespaces[0] != 1 {
		t.Fatalf("WildcardByID aliased namespace slice: %+v", wildcards[1])
	}
	for _, id := range []WildcardID{NoWildcard, 2} {
		got, ok := WildcardByID(wildcards, id)
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
		in      Wildcard
	}{
		{
			name: "any",
			in:   Wildcard{Mode: WildcardAny, Process: ProcessStrict},
		},
		{
			name: "local",
			in:   Wildcard{Mode: WildcardLocal, Process: ProcessLax},
		},
		{
			name: "other",
			in:   Wildcard{Mode: WildcardOther, OtherThan: names["urn:a"], Process: ProcessSkip},
		},
		{
			name: "target namespace",
			in: Wildcard{
				Mode:       WildcardTargetNamespace,
				Namespaces: []NamespaceID{names["urn:a"]},
				Process:    ProcessStrict,
			},
		},
		{
			name: "list",
			in: Wildcard{
				Mode:       WildcardList,
				Namespaces: []NamespaceID{EmptyNamespaceID, names["urn:a"], names["urn:b"]},
				Process:    ProcessLax,
			},
		},
		{
			name:    "invalid process",
			in:      Wildcard{Mode: WildcardAny, Process: ProcessContents(99)},
			wantErr: "wildcard has invalid process contents",
		},
		{
			name:    "invalid mode",
			in:      Wildcard{Mode: WildcardMode(99), Process: ProcessStrict},
			wantErr: "wildcard has invalid mode",
		},
		{
			name: "any stores inactive namespace",
			in: Wildcard{
				Mode:       WildcardAny,
				Namespaces: []NamespaceID{names["urn:a"]},
				Process:    ProcessStrict,
			},
			wantErr: "wildcard stores inactive namespace fields",
		},
		{
			name:    "other invalid namespace",
			in:      Wildcard{Mode: WildcardOther, OtherThan: NamespaceID(99), Process: ProcessStrict},
			wantErr: "wildcard other namespace is invalid",
		},
		{
			name: "target namespace stores inactive other",
			in: Wildcard{
				Mode:       WildcardTargetNamespace,
				Namespaces: []NamespaceID{names["urn:a"]},
				OtherThan:  names["urn:b"],
				Process:    ProcessStrict,
			},
			wantErr: "targetNamespace wildcard has invalid namespace",
		},
		{
			name: "list stores inactive other",
			in: Wildcard{
				Mode:       WildcardList,
				Namespaces: []NamespaceID{names["urn:a"]},
				OtherThan:  names["urn:b"],
				Process:    ProcessStrict,
			},
			wantErr: "namespace list wildcard stores inactive other namespace",
		},
		{
			name: "list invalid namespace",
			in: Wildcard{
				Mode:       WildcardList,
				Namespaces: []NamespaceID{NamespaceID(99)},
				Process:    ProcessStrict,
			},
			wantErr: "namespace list wildcard is invalid",
		},
		{
			name: "list is not sorted",
			in: Wildcard{
				Mode:       WildcardList,
				Namespaces: []NamespaceID{names["urn:b"], names["urn:a"]},
				Process:    ProcessStrict,
			},
			wantErr: "namespace list wildcard is invalid",
		},
		{
			name: "list has duplicate namespace",
			in: Wildcard{
				Mode:       WildcardList,
				Namespaces: []NamespaceID{names["urn:a"], names["urn:a"]},
				Process:    ProcessStrict,
			},
			wantErr: "namespace list wildcard is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateWildcard(&table, tt.in)
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

	err := ValidateWildcard(nil, Wildcard{Mode: WildcardAny, Process: ProcessStrict})
	if err == nil || !strings.Contains(err.Error(), "wildcard requires name table") {
		t.Fatalf("ValidateWildcard(nil, wildcard) error = %v", err)
	}
}

func TestEqualWildcardViewProjection(t *testing.T) {
	t.Parallel()

	table, names := wildcardValidationFixture(t)
	wildcard := Wildcard{
		Mode:       WildcardList,
		Namespaces: []NamespaceID{names["urn:a"], names["urn:b"]},
		Process:    ProcessLax,
	}
	view := NewWildcardView(&table, &wildcard)
	if !EqualWildcardViewProjection(view, &table, &wildcard) {
		t.Fatal("EqualWildcardViewProjection() = false, want true")
	}

	changed := wildcard
	changed.Process = ProcessSkip
	if EqualWildcardViewProjection(view, &table, &changed) {
		t.Fatal("EqualWildcardViewProjection() = true for different process")
	}
}

func TestWildcardViewProjectionTable(t *testing.T) {
	t.Parallel()

	table, names := wildcardValidationFixture(t)
	wildcards := []Wildcard{
		{Mode: WildcardAny, Process: ProcessStrict},
		{Mode: WildcardList, Namespaces: []NamespaceID{names["urn:a"], names["urn:b"]}, Process: ProcessLax},
	}
	views := NewWildcardViews(&table, wildcards)
	if !EqualWildcardViewProjectionTable(views, &table, wildcards) {
		t.Fatalf("NewWildcardViews() = %#v, want projection for %#v", views, wildcards)
	}
	if got, ok := WildcardViewByID(views, 1); !ok || !EqualWildcardViews(got, views[1]) {
		t.Fatalf("WildcardViewByID() = %#v, %v; want view 1, true", got, ok)
	}
	if got, ok := WildcardViewByID(views, WildcardID(99)); ok || !EqualWildcardViews(got, WildcardView{}) {
		t.Fatalf("WildcardViewByID(invalid) = %#v, %v; want zero, false", got, ok)
	}
	if EqualWildcardViewProjectionTable(views[:1], &table, wildcards) {
		t.Fatal("EqualWildcardViewProjectionTable() accepted mismatched table length")
	}

	changed := slices.Clone(wildcards)
	changed[1].Process = ProcessSkip
	if EqualWildcardViewProjectionTable(views, &table, changed) {
		t.Fatal("EqualWildcardViewProjectionTable() accepted mismatched wildcard")
	}
	if err := ValidateWildcardViewProjectionTable(NewWildcardViews(&table, wildcards), &table, wildcards); err != nil {
		t.Fatalf("ValidateWildcardViewProjectionTable() error = %v", err)
	}
	if err := ValidateWildcardViewProjectionTable(views[:1], &table, wildcards); err == nil || err.Error() != "wildcard read projection count does not match wildcards" {
		t.Fatalf("ValidateWildcardViewProjectionTable(short) error = %v, want count invariant", err)
	}
	if err := ValidateWildcardViewProjectionTable(views, &table, changed); err == nil || err.Error() != "wildcard read projection does not match wildcard" {
		t.Fatalf("ValidateWildcardViewProjectionTable(changed) error = %v, want mismatch invariant", err)
	}
}

func TestWildcardSetPredicates(t *testing.T) {
	t.Parallel()

	table, names := wildcardValidationFixture(t)
	anyWildcard := Wildcard{Mode: WildcardAny, Process: ProcessStrict}
	otherA := Wildcard{Mode: WildcardOther, OtherThan: names["urn:a"], Process: ProcessStrict}
	local := Wildcard{Mode: WildcardLocal, Process: ProcessStrict}
	listAB := Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{names["urn:a"], names["urn:b"]}, Process: ProcessStrict}
	if !WildcardAllowsNamespace(anyWildcard, names["urn:a"]) {
		t.Fatal("##any did not allow interned namespace")
	}
	if WildcardAllowsNamespace(otherA, names["urn:a"]) || !WildcardAllowsNamespace(otherA, names["urn:b"]) {
		t.Fatal("##other namespace admission is wrong")
	}
	if !WildcardAllowsNamespace(local, EmptyNamespaceID) || WildcardAllowsNamespace(local, names["urn:a"]) {
		t.Fatal("##local namespace admission is wrong")
	}
	if !WildcardAllowsURI(&table, otherA, "urn:not-in-schema") {
		t.Fatal("##other did not allow uninterned non-empty namespace")
	}
	if WildcardAllowsURI(&table, listAB, "urn:not-in-schema") {
		t.Fatal("finite wildcard allowed uninterned namespace")
	}
	if !WildcardSubset(listAB, anyWildcard) || WildcardSubset(anyWildcard, listAB) {
		t.Fatal("wildcard subset relation is wrong")
	}
	if !WildcardsOverlap(otherA, listAB) || WildcardsOverlap(local, listAB) {
		t.Fatal("wildcard overlap relation is wrong")
	}
	if !WildcardNamespaceEqual(listAB, Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{names["urn:a"], names["urn:b"]}}) {
		t.Fatal("equal wildcard namespace sets were not recognized")
	}
}

func TestWildcardSetOperators(t *testing.T) {
	t.Parallel()

	_, names := wildcardValidationFixture(t)
	normalized := NormalizeNamespaceList([]NamespaceID{names["urn:b"], names["urn:a"], names["urn:a"]})
	if !slices.Equal(normalized, []NamespaceID{names["urn:a"], names["urn:b"]}) {
		t.Fatalf("NormalizeNamespaceList() = %v", normalized)
	}

	listA := Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{names["urn:a"]}, Process: ProcessStrict}
	listB := Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{names["urn:b"]}, Process: ProcessLax}
	union, err := UnionWildcard(listA, listB, ProcessSkip)
	if err != nil {
		t.Fatalf("UnionWildcard() error = %v", err)
	}
	if union.Mode != WildcardList || union.Process != ProcessSkip || !slices.Equal(union.Namespaces, []NamespaceID{names["urn:a"], names["urn:b"]}) {
		t.Fatalf("UnionWildcard() = %+v", union)
	}
	sameList := Wildcard{Mode: WildcardList, Namespaces: []NamespaceID{names["urn:a"]}, Process: ProcessStrict}
	sameUnion, err := UnionWildcard(sameList, sameList, ProcessLax)
	if err != nil {
		t.Fatalf("UnionWildcard(equal list) error = %v", err)
	}
	sameList.Namespaces[0] = names["urn:b"]
	if !slices.Equal(sameUnion.Namespaces, []NamespaceID{names["urn:a"]}) {
		t.Fatalf("UnionWildcard(equal list) aliased input namespace slice: %+v", sameUnion)
	}

	otherA := Wildcard{Mode: WildcardOther, OtherThan: names["urn:a"], Process: ProcessStrict}
	union, err = UnionWildcard(otherA, listA, ProcessStrict)
	if err != nil {
		t.Fatalf("UnionWildcard(other, negated namespace) error = %v", err)
	}
	if union.Mode != WildcardOther || union.OtherThan != EmptyNamespaceID {
		t.Fatalf("UnionWildcard(other, negated namespace) = %+v", union)
	}
	if _, unionErr := UnionWildcard(otherA, Wildcard{Mode: WildcardLocal, Process: ProcessStrict}, ProcessStrict); unionErr == nil {
		t.Fatal("unexpressible wildcard union succeeded")
	}

	intersection, err := IntersectWildcard(Wildcard{Mode: WildcardAny, Process: ProcessStrict}, listB, ProcessSkip)
	if err != nil {
		t.Fatalf("IntersectWildcard(any, list) error = %v", err)
	}
	if !WildcardNamespaceEqual(intersection, listB) || intersection.Process != ProcessSkip {
		t.Fatalf("IntersectWildcard(any, list) = %+v", intersection)
	}
	listB.Namespaces[0] = names["urn:a"]
	if !slices.Equal(intersection.Namespaces, []NamespaceID{names["urn:b"]}) {
		t.Fatalf("IntersectWildcard(any, list) aliased input namespace slice: %+v", intersection)
	}
	_, err = IntersectWildcard(
		Wildcard{Mode: WildcardOther, OtherThan: names["urn:a"], Process: ProcessStrict},
		Wildcard{Mode: WildcardOther, OtherThan: names["urn:b"], Process: ProcessStrict},
		ProcessStrict,
	)
	if err == nil {
		t.Fatalf("IntersectWildcard(other a, other b) error = %v, want error", err)
	}
}

func wildcardValidationFixture(t *testing.T) (NameTable, map[string]NamespaceID) {
	t.Helper()

	table, err := NewNameTable(16, []string{EmptyNamespaceURI, "urn:a", "urn:b"}, nil)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	names := map[string]NamespaceID{EmptyNamespaceURI: EmptyNamespaceID}
	for _, uri := range []string{"urn:a", "urn:b"} {
		id, ok := table.LookupNamespace(uri)
		if !ok {
			t.Fatalf("LookupNamespace(%q) failed", uri)
		}
		names[uri] = id
	}
	return table, names
}
