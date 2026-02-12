package xiter

import (
	"slices"
	"testing"
)

func TestSliceAndCollect(t *testing.T) {
	items := []int{3, 1, 2}
	got := Collect(Slice(items))
	if !slices.Equal(got, items) {
		t.Fatalf("Collect(Slice()) = %v, want %v", got, items)
	}
}

func TestCount(t *testing.T) {
	items := []string{"a", "b", "c", "d"}
	if got, want := Count(Slice(items)), 4; got != want {
		t.Fatalf("Count() = %d, want %d", got, want)
	}
}

func TestSortedKeys(t *testing.T) {
	input := map[string]int{"c": 3, "a": 1, "b": 2}
	got := Collect(SortedKeys(input))
	want := []string{"a", "b", "c"}
	if !slices.Equal(got, want) {
		t.Fatalf("SortedKeys() = %v, want %v", got, want)
	}
}

func TestValuesBySortedKeys(t *testing.T) {
	input := map[string]int{"c": 30, "a": 10, "b": 20}
	got := Collect(ValuesBySortedKeys(input))
	want := []int{10, 20, 30}
	if !slices.Equal(got, want) {
		t.Fatalf("ValuesBySortedKeys() = %v, want %v", got, want)
	}
}

func TestRangeOverFuncEarlyStop(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	seq := Slice(input)

	sum := 0
	for item := range seq {
		sum += item
		if item == 3 {
			break
		}
	}

	if got, want := sum, 6; got != want {
		t.Fatalf("early stop sum = %d, want %d", got, want)
	}
}
