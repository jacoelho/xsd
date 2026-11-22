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
			if got, err := wildcardsOverlap(test.a, test.b, unlimitedWildcardWork); err != nil || got != test.want {
				t.Fatalf("WildcardsOverlap(%+v, %+v) = %v, want %v", test.a, test.b, got, test.want)
			}
			if got, err := wildcardsOverlap(test.b, test.a, unlimitedWildcardWork); err != nil || got != test.want {
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
	if !ok || got.Mode != WildcardList || got.Process != ProcessLax || !slices.Equal(got.Namespaces, []namespaceID{1, 2}) {
		t.Fatalf("WildcardByID(valid) = %+v, %v; want wildcard 1, true", got, ok)
	}
	if &got.Namespaces[0] != &wildcards[1].Namespaces[0] {
		t.Fatal("WildcardByID copied the admitted namespace list")
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
	relation := wildcardRelationOf(t)
	if !wildcardAllowsNamespace(anyWildcard, names["urn:a"]) {
		t.Fatal("##any did not allow interned namespace")
	}
	if wildcardAllowsNamespace(otherA, names["urn:a"]) || !wildcardAllowsNamespace(otherA, names["urn:b"]) {
		t.Fatal("##other namespace admission is wrong")
	}
	if !wildcardAllowsNamespace(local, emptyNamespaceID) || wildcardAllowsNamespace(local, names["urn:a"]) {
		t.Fatal("##local namespace admission is wrong")
	}
	if !relation(wildcardSubset(listAB, anyWildcard, unlimitedWildcardWork)) ||
		relation(wildcardSubset(anyWildcard, listAB, unlimitedWildcardWork)) {
		t.Fatal("wildcard subset relation is wrong")
	}
	if !relation(wildcardsOverlap(otherA, listAB, unlimitedWildcardWork)) ||
		relation(wildcardsOverlap(local, listAB, unlimitedWildcardWork)) {
		t.Fatal("wildcard overlap relation is wrong")
	}
	if !relation(wildcardNamespaceEqual(listAB, wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{names["urn:a"], names["urn:b"]}}, unlimitedWildcardWork)) {
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
	union, err := unionWildcard(listA, listB, ProcessSkip, unlimitedWildcardWork)
	if err != nil {
		t.Fatalf("UnionWildcard() error = %v", err)
	}
	if union.Mode != WildcardList || union.Process != ProcessSkip || !slices.Equal(union.Namespaces, []namespaceID{names["urn:a"], names["urn:b"]}) {
		t.Fatalf("UnionWildcard() = %+v", union)
	}
	sameList := wildcardRuntime{Mode: WildcardList, Namespaces: []namespaceID{names["urn:a"]}, Process: processStrict}
	sameUnion, err := unionWildcard(sameList, sameList, ProcessLax, unlimitedWildcardWork)
	if err != nil {
		t.Fatalf("UnionWildcard(equal list) error = %v", err)
	}
	if sameUnion.Process != ProcessLax || !slices.Equal(sameUnion.Namespaces, []namespaceID{names["urn:a"]}) {
		t.Fatalf("UnionWildcard(equal list) = %+v", sameUnion)
	}

	otherA := wildcardRuntime{Mode: WildcardOther, OtherThan: names["urn:a"], Process: processStrict}
	union, err = unionWildcard(otherA, listA, processStrict, unlimitedWildcardWork)
	if err != nil {
		t.Fatalf("UnionWildcard(other, negated namespace) error = %v", err)
	}
	if union.Mode != WildcardOther || union.OtherThan != emptyNamespaceID {
		t.Fatalf("UnionWildcard(other, negated namespace) = %+v", union)
	}
	if _, unionErr := unionWildcard(otherA, wildcardRuntime{Mode: WildcardLocal, Process: processStrict}, processStrict, unlimitedWildcardWork); unionErr == nil {
		t.Fatal("unexpressible wildcard union succeeded")
	}

	intersection, err := intersectWildcard(wildcardRuntime{Mode: WildcardAny, Process: processStrict}, listB, ProcessSkip, unlimitedWildcardWork)
	if err != nil {
		t.Fatalf("IntersectWildcard(any, list) error = %v", err)
	}
	if intersection.Mode != WildcardList || intersection.Process != ProcessSkip || !slices.Equal(intersection.Namespaces, []namespaceID{names["urn:b"]}) {
		t.Fatalf("IntersectWildcard(any, list) = %+v", intersection)
	}
	_, err = intersectWildcard(
		wildcardRuntime{Mode: WildcardOther, OtherThan: names["urn:a"], Process: processStrict},
		wildcardRuntime{Mode: WildcardOther, OtherThan: names["urn:b"], Process: processStrict},
		processStrict,
		unlimitedWildcardWork,
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

func unlimitedWildcardWork(int) error {
	return nil
}

func wildcardRelationOf(t *testing.T) func(bool, error) bool {
	t.Helper()
	return func(holds bool, err error) bool {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return holds
	}
}

// modelWildcardAdmits is kept apart from wildcardAllowsNamespace so that the
// set-operation expectations do not reuse the code under test.
func modelWildcardAdmits(w wildcardRuntime, ns namespaceID) bool {
	switch w.Mode {
	case WildcardAny:
		return true
	case WildcardOther:
		return ns != emptyNamespaceID && ns != w.OtherThan
	case WildcardLocal:
		return ns == emptyNamespaceID
	default:
		for _, member := range w.Namespaces {
			if member == ns {
				return true
			}
		}
		return false
	}
}

func TestWildcardSetOperationsMatchNamespaceMembership(t *testing.T) {
	t.Parallel()
	const listed, universe = 5, 7
	wildcards := []wildcardRuntime{{Mode: WildcardAny}, {Mode: WildcardLocal}}
	for ns := range namespaceID(listed) {
		wildcards = append(wildcards,
			wildcardRuntime{Mode: WildcardTargetNamespace, Namespaces: []namespaceID{ns}},
			wildcardRuntime{Mode: WildcardOther, OtherThan: ns})
	}
	for members := range uint(1 << listed) {
		var namespaces []namespaceID
		for ns := range namespaceID(listed) {
			if members&(1<<ns) != 0 {
				namespaces = append(namespaces, ns)
			}
		}
		wildcards = append(wildcards, wildcardRuntime{Mode: WildcardList, Namespaces: namespaces})
	}
	admits := func(w wildcardRuntime) [universe]bool {
		var out [universe]bool
		for ns := range namespaceID(universe) {
			out[ns] = modelWildcardAdmits(w, ns)
		}
		return out
	}
	relation := wildcardRelationOf(t)
	for _, a := range wildcards {
		for _, b := range wildcards {
			inA, inB := admits(a), admits(b)
			var wantOverlap bool
			wantSubset := true
			var wantUnion, wantIntersection [universe]bool
			for ns := range universe {
				wantOverlap = wantOverlap || inA[ns] && inB[ns]
				wantSubset = wantSubset && (!inA[ns] || inB[ns])
				wantUnion[ns] = inA[ns] || inB[ns]
				wantIntersection[ns] = inA[ns] && inB[ns]
			}
			bothOther := a.Mode == WildcardOther && b.Mode == WildcardOther
			if bothOther && a.OtherThan != b.OtherThan {
				// Structures §3.10.6 Wildcard Subset clause 2.
				wantSubset = false
			}
			wantUnionErr := a.Mode == WildcardOther && unionOtherFiniteInexpressible(a, b) ||
				b.Mode == WildcardOther && unionOtherFiniteInexpressible(b, a)
			wantIntersectionErr := bothOther && a.OtherThan != b.OtherThan &&
				a.OtherThan != emptyNamespaceID && b.OtherThan != emptyNamespaceID

			if got := relation(wildcardsOverlap(a, b, unlimitedWildcardWork)); got != wantOverlap {
				t.Fatalf("wildcardsOverlap(%+v, %+v) = %v, want %v", a, b, got, wantOverlap)
			}
			if got := relation(wildcardNamespaceSubset(a, b, unlimitedWildcardWork)); got != wantSubset {
				t.Fatalf("wildcardNamespaceSubset(%+v, %+v) = %v, want %v", a, b, got, wantSubset)
			}
			union, err := unionWildcard(a, b, ProcessLax, unlimitedWildcardWork)
			if (err != nil) != wantUnionErr {
				t.Fatalf("unionWildcard(%+v, %+v) error = %v, want error %v", a, b, err, wantUnionErr)
			}
			if err == nil && admits(union) != wantUnion {
				t.Fatalf("unionWildcard(%+v, %+v) = %+v, want membership %v", a, b, union, wantUnion)
			}
			intersection, err := intersectWildcard(a, b, ProcessLax, unlimitedWildcardWork)
			if (err != nil) != wantIntersectionErr {
				t.Fatalf("intersectWildcard(%+v, %+v) error = %v, want error %v", a, b, err, wantIntersectionErr)
			}
			if err == nil && admits(intersection) != wantIntersection {
				t.Fatalf("intersectWildcard(%+v, %+v) = %+v, want membership %v", a, b, intersection, wantIntersection)
			}
		}
	}
}

// Structures §3.10.6 Attribute Wildcard Union clause 5.4.
func unionOtherFiniteInexpressible(other, finite wildcardRuntime) bool {
	if finite.Mode == WildcardAny || finite.Mode == WildcardOther || other.OtherThan == emptyNamespaceID {
		return false
	}
	return modelWildcardAdmits(finite, emptyNamespaceID) && !modelWildcardAdmits(finite, other.OtherThan)
}

func TestWildcardSetOperationsChargeEveryNamespaceVisited(t *testing.T) {
	t.Parallel()
	interleaved := func(first namespaceID, count int) wildcardRuntime {
		namespaces := make([]namespaceID, count)
		for i := range namespaces {
			namespaces[i] = first + namespaceID(2*i)
		}
		return wildcardRuntime{Mode: WildcardList, Namespaces: namespaces}
	}
	for _, count := range []int{16, 1024} {
		even, odd, sameLength := interleaved(2, count), interleaved(3, 2*count), interleaved(3, count)
		subset := wildcardRuntime{Mode: WildcardList, Namespaces: even.Namespaces[:count/2]}
		other := wildcardRuntime{Mode: WildcardOther, OtherThan: 1}
		for _, test := range []struct {
			name      string
			want      int
			operation func(spend func(int) error) error
		}{
			{"overlap of the in-range runs", count - 1, func(spend func(int) error) error {
				_, err := wildcardsOverlap(even, odd, spend)
				return err
			}},
			{"subset", count / 2, func(spend func(int) error) error {
				_, err := wildcardNamespaceSubset(subset, even, spend)
				return err
			}},
			{"equality", count, func(spend func(int) error) error {
				_, err := wildcardNamespaceEqual(even, sameLength, spend)
				return err
			}},
			{"union", 3 * count, func(spend func(int) error) error {
				_, err := unionWildcard(even, odd, ProcessLax, spend)
				return err
			}},
			{"intersection of the in-range runs", count - 1, func(spend func(int) error) error {
				_, err := intersectWildcard(even, odd, ProcessLax, spend)
				return err
			}},
			{"intersection with other", count, func(spend func(int) error) error {
				_, err := intersectWildcard(other, even, ProcessLax, spend)
				return err
			}},
		} {
			charged := 0
			if err := test.operation(func(steps int) error {
				charged += steps
				return nil
			}); err != nil {
				t.Fatalf("%s of %d namespaces: %v", test.name, count, err)
			}
			if charged < test.want {
				t.Fatalf("%s of %d namespaces charged %d steps, want at least %d", test.name, count, charged, test.want)
			}
		}
	}
}
