package value

import (
	"errors"
	"strconv"
	"testing"
	"unsafe"
)

func decimalRestriction(base TypeID, facets FacetSpec) TypeSpec {
	return TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveDecimal,
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              base,
		ListItem:          NoType,
		Facets:            facets,
	}
}

func decimalBound(lexical string) BoundFacet {
	return BoundFacet{Present: true, Lexical: lexical, Type: NoType}
}

func completeNext(t *testing.T, b *Builder, spec TypeSpec) (TypeID, error) {
	t.Helper()
	id, err := b.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	return id, b.Complete(id, spec)
}

func retainedFacetCapacity(b *Builder) (total uint64, shape error) {
	seen := map[unsafe.Pointer]bool{}
	for i := range b.program.types {
		f := &b.program.types[i].facets
		for _, side := range []retainedSide{
			sideOf(f.lower),
			sideOf(f.upper),
			sideOf(f.enumGroups),
			sideOf(f.patterns),
		} {
			if side.length == 0 {
				continue
			}
			if side.length != side.capacity {
				shape = errors.New("retained facet side has capacity beyond its length")
			}
			if !seen[side.first] {
				seen[side.first] = true
				total += side.bytes
			}
		}
	}
	return total, shape
}

type retainedSide struct {
	first            unsafe.Pointer
	length, capacity int
	bytes            uint64
}

func sideOf[T any](s []T) retainedSide {
	var zero T
	return retainedSide{
		first:    unsafe.Pointer(unsafe.SliceData(s)), //nolint:gosec // the test compares backing-array identity to count shared storage once
		length:   len(s),
		capacity: cap(s),
		bytes:    uint64(cap(s)) * uint64(unsafe.Sizeof(zero)),
	}
}

func TestBuilderChargesRetainedFacetEntries(t *testing.T) {
	t.Parallel()

	b := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62, MaxStorageBytes: 1 << 30})
	check := func(id TypeID, err error) TypeID {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		retained, shape := retainedFacetCapacity(b)
		if shape != nil {
			t.Fatal(shape)
		}
		if b.storageUsed < retained {
			t.Fatalf("storageUsed %d is below retained facet capacity %d", b.storageUsed, retained)
		}
		return id
	}
	base := check(completeNext(t, b, decimalRestriction(NoType, FacetSpec{})))
	for i := range 200 {
		base = check(completeNext(t, b, decimalRestriction(base, FacetSpec{MinInclusive: decimalBound(strconv.Itoa(i))})))
	}
	end := base
	tips := make([]TypeID, 50)
	for i := range tips {
		tips[i] = check(completeNext(t, b, decimalRestriction(end, FacetSpec{MaxInclusive: decimalBound("1000")})))
	}
	endLower := b.program.types[end-BuiltinTypeCount].facets.lower
	for _, tip := range tips {
		lower := b.program.types[tip-BuiltinTypeCount].facets.lower
		if len(lower) != len(endLower) || &lower[0] != &endLower[0] {
			t.Fatalf("tip %d does not share its base's lower bounds", tip)
		}
	}
}

func TestBuilderRejectsInheritedFacetsOverStorageBudget(t *testing.T) {
	t.Parallel()

	b := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62, MaxStorageBytes: 1 << 30})
	base := NoType
	for i := range 40 {
		var err error
		if base, err = completeNext(t, b, decimalRestriction(base, FacetSpec{MinInclusive: decimalBound(strconv.Itoa(i))})); err != nil {
			t.Fatal(err)
		}
	}
	next := decimalRestriction(base, FacetSpec{MinInclusive: decimalBound("40")})
	id, err := b.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	own, _ := typeSpecStorage(next)
	b.maxStorage = b.storageUsed + (own - typeStorageBase) + 40*boundValueBytes - 1
	before := b.storageUsed
	err = b.Complete(id, next)
	if !errors.Is(err, ErrStorageLimit) || !errors.Is(err, ErrLimit) {
		t.Fatalf("Complete over budget = %v, want ErrStorageLimit wrapping ErrLimit", err)
	}
	if b.storageUsed != before {
		t.Fatalf("storageUsed = %d after failed Complete, want %d", b.storageUsed, before)
	}
	small := decimalRestriction(NoType, FacetSpec{})
	if err := b.Complete(id, small); err != nil {
		t.Fatalf("small type after rejection = %v", err)
	}
}
