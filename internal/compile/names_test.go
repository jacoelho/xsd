package compile

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestNameTableLimitStopsGrowthAfterFirstFailure(t *testing.T) {
	seed, err := NewNameTable(0)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	names, err := NewNameTable(seed.NameCount() + 1)
	if err != nil {
		t.Fatalf("NewNameTable() with limit error = %v", err)
	}
	base := names.NameCount()
	interner := NewNameInterner(&names)

	if _, err := interner.InternQName("urn:new", "new"); err == nil {
		t.Fatal("InternQName() succeeded")
	}
	if got := names.NameCount(); got != base {
		t.Fatalf("name count after first failure = %d, want %d", got, base)
	}

	if _, err := interner.InternQName("urn:other", "other"); err == nil {
		t.Fatal("second InternQName() succeeded")
	}
	if got := names.NameCount(); got != base {
		t.Fatalf("name count after second failure = %d, want %d", got, base)
	}
}

func TestSortedQNamesOrdersByExpandedName(t *testing.T) {
	t.Parallel()

	names, err := NewNameTable(0)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	interner := NewNameInterner(&names)
	bBeta, err := interner.InternQName("urn:b", "beta")
	if err != nil {
		t.Fatalf("InternQName(b beta) error = %v", err)
	}
	aZulu, err := interner.InternQName("urn:a", "zulu")
	if err != nil {
		t.Fatalf("InternQName(a zulu) error = %v", err)
	}
	aAlpha, err := interner.InternQName("urn:a", "alpha")
	if err != nil {
		t.Fatalf("InternQName(a alpha) error = %v", err)
	}
	got := SortedQNames(map[runtime.QName]bool{
		bBeta:  true,
		aZulu:  true,
		aAlpha: true,
	}, names)
	want := []runtime.QName{aAlpha, aZulu, bBeta}
	if len(got) != len(want) {
		t.Fatalf("len(SortedQNames()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortedQNames()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}
