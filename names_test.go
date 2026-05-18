package xsd

import "testing"

func TestNameTableLimitStopsGrowthAfterFirstFailure(t *testing.T) {
	names, err := newNameTable(0)
	if err != nil {
		t.Fatalf("newNameTable() error = %v", err)
	}
	base := len(names.namespaces) + len(names.locals)
	names.maxNames = base + 1

	if _, err := names.InternQName("urn:new", "new"); err == nil {
		t.Fatal("InternQName() succeeded")
	}
	if got := len(names.namespaces) + len(names.locals); got != base {
		t.Fatalf("name count after first failure = %d, want %d", got, base)
	}

	if _, err := names.InternQName("urn:other", "other"); err == nil {
		t.Fatal("second InternQName() succeeded")
	}
	if got := len(names.namespaces) + len(names.locals); got != base {
		t.Fatalf("name count after second failure = %d, want %d", got, base)
	}
}
