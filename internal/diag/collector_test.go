package diag

import (
	"slices"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
)

func TestCollectorAddAndLen(t *testing.T) {
	collector := NewCollector(2)
	collector.Add(Diagnostic{Code: "x", Message: "one", Path: "/a"})
	collector.Addf(xsderrors.ErrXMLParse, "/b", "bad %s", "xml")

	if got, want := collector.Len(), 2; got != want {
		t.Fatalf("Len() = %d, want %d", got, want)
	}
}

func TestCollectorSeqPreservesInsertionOrder(t *testing.T) {
	collector := NewCollector(0)
	collector.Add(Diagnostic{Code: "b", Message: "second"})
	collector.Add(Diagnostic{Code: "a", Message: "first"})

	var got []string
	for item := range collector.Seq() {
		got = append(got, item.Code)
	}
	want := []string{"b", "a"}
	if !slices.Equal(got, want) {
		t.Fatalf("Seq() order = %v, want %v", got, want)
	}
}

func TestCollectorSortedDeterministic(t *testing.T) {
	collector := NewCollector(0)
	collector.Add(Diagnostic{Code: "z", Message: "later", Line: 20, Column: 1})
	collector.Add(Diagnostic{Code: "a", Message: "earlier", Line: 10, Column: 1})

	sorted := collector.Sorted()
	if got, want := sorted[0].Code, "a"; got != want {
		t.Fatalf("Sorted()[0].Code = %q, want %q", got, want)
	}
}

func TestCollectorToValidationListCopiesExpected(t *testing.T) {
	expected := []string{"a", "b"}
	collector := NewCollector(0)
	collector.Add(Diagnostic{Code: "cvc-test", Message: "x", Expected: expected})

	expected[0] = "mutated"

	list := collector.ToValidationList()
	if len(list) != 1 {
		t.Fatalf("ToValidationList() len = %d, want 1", len(list))
	}
	if got, want := list[0].Expected[0], "a"; got != want {
		t.Fatalf("Expected copy first = %q, want %q", got, want)
	}
}

func TestCollectorErrorOrNil(t *testing.T) {
	collector := NewCollector(0)
	if err := collector.ErrorOrNil(); err != nil {
		t.Fatalf("ErrorOrNil() for empty collector = %v, want nil", err)
	}
	collector.Add(Diagnostic{Code: string(xsderrors.ErrXMLParse), Message: "bad"})
	if err := collector.ErrorOrNil(); err == nil {
		t.Fatal("ErrorOrNil() = nil, want non-nil")
	}
}
