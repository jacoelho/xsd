package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func testSimpleType(variety SimpleVariety, base SimpleTypeID, members ...SimpleTypeID) SimpleType {
	spec := value.TypeSpec{Variety: variety, Base: base}
	switch variety {
	case SimpleVarietyList:
		if len(members) != 0 {
			spec.ListItem = members[0]
		}
	case SimpleVarietyUnion:
		spec.Union = append([]SimpleTypeID(nil), members...)
	}
	return SimpleType{ValueSpec: spec}
}

func TestCheckSimpleRestrictionBase(t *testing.T) {
	t.Parallel()

	if err := CheckSimpleRestrictionBase(1, 0); err != nil {
		t.Fatalf("CheckSimpleRestrictionBase(non-anySimpleType) error = %v", err)
	}
	err := CheckSimpleRestrictionBase(1, 1)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, "simple type cannot restrict xs:anySimpleType")
}

func TestCheckSimpleTypeFinalAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		final      DerivationMask
		derivation DerivationMask
		role       SimpleTypeFinalRole
		msg        string
	}{
		{
			name:       "restriction",
			final:      DerivationRestriction,
			derivation: DerivationRestriction,
			role:       SimpleTypeFinalBaseRestriction,
			msg:        "base simple type final blocks restriction",
		},
		{
			name:       "list",
			final:      DerivationList,
			derivation: DerivationList,
			role:       SimpleTypeFinalListItem,
			msg:        "item simple type final blocks list",
		},
		{
			name:       "union",
			final:      DerivationUnion,
			derivation: DerivationUnion,
			role:       SimpleTypeFinalUnionMember,
			msg:        "member simple type final blocks union",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := CheckSimpleTypeFinalAllows(0, tt.derivation, tt.role); err != nil {
				t.Fatalf("CheckSimpleTypeFinalAllows(allowed) error = %v", err)
			}
			err := CheckSimpleTypeFinalAllows(tt.final, tt.derivation, tt.role)
			expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, tt.msg)
		})
	}
}

func TestSimpleTypeListReachability(t *testing.T) {
	t.Parallel()

	types := []SimpleType{
		testSimpleType(SimpleVarietyAtomic, NoSimpleType),
		testSimpleType(SimpleVarietyAtomic, NoSimpleType),
		testSimpleType(SimpleVarietyList, NoSimpleType, 1),
		testSimpleType(SimpleVarietyUnion, NoSimpleType, 0, 1),
		testSimpleType(SimpleVarietyUnion, NoSimpleType, 0, 3, 2),
	}
	var reach simpleTypeListReachability
	if reach.reachesList(types, 1) {
		t.Fatal("atomic type unexpectedly reaches a list")
	}
	if reach.reachesList(types, 3) {
		t.Fatal("union without list unexpectedly reaches a list")
	}
	if !reach.reachesList(types, 4) {
		t.Fatal("union containing a list did not reach the list")
	}
	err := simpleListItemListReachError()
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "list item type cannot be a list type")
}

func TestSimpleTypeListReachabilityHandlesDeepForwardUnionChain(t *testing.T) {
	t.Parallel()

	const depth = 10_000
	types := make([]SimpleType, depth+1)
	types[0] = testSimpleType(SimpleVarietyList, NoSimpleType)
	for i := 1; i <= depth; i++ {
		types[i] = testSimpleType(SimpleVarietyUnion, NoSimpleType, 0, SimpleTypeID(i-1))
	}

	var reach simpleTypeListReachability
	if !reach.reachesList(types, depth) {
		t.Fatal("reachesList() = false, want true")
	}
	if reach.state[depth] != simpleTypeListReachChecked || !reach.reaches[depth] {
		t.Fatal("reachesList() did not memoize the root result")
	}
}

func TestSimpleTypeListReachabilityReusesShallowQueryStack(t *testing.T) {
	const count = 10_000
	types := make([]SimpleType, count+1)
	types[0] = testSimpleType(SimpleVarietyAtomic, NoSimpleType)
	for i := 1; i <= count; i++ {
		types[i] = testSimpleType(SimpleVarietyUnion, NoSimpleType, 0, 0)
	}

	allocs := testing.AllocsPerRun(1, func() {
		var reach simpleTypeListReachability
		for i := 1; i <= count; i++ {
			if reach.reachesList(types, SimpleTypeID(i)) {
				t.Fatal("reachesList() = true, want false")
			}
		}
	})
	if allocs > 10 {
		t.Fatalf("reachesList() allocations = %.0f, want at most 10", allocs)
	}
}

func TestSimpleTypeListReachabilityPreservesGraphSemantics(t *testing.T) {
	t.Parallel()

	t.Run("cycle is false", func(t *testing.T) {
		t.Parallel()

		types := []SimpleType{
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 0, 1),
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 0),
		}
		var reach simpleTypeListReachability
		if reach.reachesList(types, 0) {
			t.Fatal("reachesList(cycle) = true, want false")
		}
		if reach.state[0] != simpleTypeListReachUnchecked || reach.state[1] != simpleTypeListReachUnchecked {
			t.Fatal("reachesList(cycle) memoized an unstable false result")
		}
		if reach.reachesList(types, 1) {
			t.Fatal("reachesList(cycle second root) = true, want false")
		}
	})

	t.Run("later query resolves through settled ancestor", func(t *testing.T) {
		t.Parallel()

		types := []SimpleType{
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 1, 2),
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 0),
			testSimpleType(SimpleVarietyList, NoSimpleType),
		}
		var reach simpleTypeListReachability
		if !reach.reachesList(types, 0) {
			t.Fatal("reachesList(A) = false, want true")
		}
		if reach.state[1] != simpleTypeListReachUnchecked {
			t.Fatal("reachesList(A) memoized B's ancestor-dependent false result")
		}
		if !reach.reachesList(types, 1) {
			t.Fatal("reachesList(B after A settled) = false, want true")
		}
	})

	t.Run("ancestor of cyclic child remains recomputable", func(t *testing.T) {
		t.Parallel()

		types := []SimpleType{
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 0, 1),
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 0),
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 0),
		}
		var reach simpleTypeListReachability
		if reach.reachesList(types, 2) {
			t.Fatal("reachesList(ancestor of cycle) = true, want false")
		}
		for id := range types {
			if reach.state[id] != simpleTypeListReachUnchecked {
				t.Fatalf("state[%d] = %d, want unchecked", id, reach.state[id])
			}
		}
		if reach.reachesList(types, 2) {
			t.Fatal("reachesList(ancestor of cycle second query) = true, want false")
		}
	})

	t.Run("shared false branch is memoized", func(t *testing.T) {
		t.Parallel()

		types := []SimpleType{
			testSimpleType(SimpleVarietyAtomic, NoSimpleType),
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 0),
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 1, 1),
		}
		var reach simpleTypeListReachability
		if reach.reachesList(types, 2) {
			t.Fatal("reachesList(shared atomic branch) = true, want false")
		}
		if reach.state[1] != simpleTypeListReachChecked || reach.reaches[1] {
			t.Fatal("reachesList() did not preserve the shared false result")
		}
	})

	t.Run("first true branch stops traversal", func(t *testing.T) {
		t.Parallel()

		invalid := SimpleTypeID(99)
		types := []SimpleType{
			testSimpleType(SimpleVarietyList, NoSimpleType),
			testSimpleType(SimpleVarietyUnion, NoSimpleType, invalid, 0),
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 1, 3),
			testSimpleType(SimpleVarietyUnion, NoSimpleType, 3),
		}
		var reach simpleTypeListReachability
		if !reach.reachesList(types, 2) {
			t.Fatal("reachesList(later valid branch) = false, want true")
		}
		if reach.state[3] != simpleTypeListReachUnchecked {
			t.Fatal("reachesList() evaluated a branch after the first true result")
		}
	})
}

func expectCompileDiagnostic(t *testing.T, err error, code xsderrors.Code, message string) {
	t.Helper()

	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category() != xsderrors.CategorySchemaCompile || xerr.Code() != code {
		t.Fatalf("diagnostic = %s/%s, want schema compile/%s", xerr.Category(), xerr.Code(), code)
	}
	if xerr.Message() != message {
		t.Fatalf("message = %q, want %q", xerr.Message(), message)
	}
}
