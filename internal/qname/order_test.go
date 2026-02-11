package qname

import (
	"slices"
	"testing"
)

func TestCompare(t *testing.T) {
	left := QName{Namespace: "urn:a", Local: "b"}
	right := QName{Namespace: "urn:b", Local: "a"}
	if got := Compare(left, right); got >= 0 {
		t.Fatalf("Compare() = %d, want < 0", got)
	}

	left = QName{Namespace: "urn:a", Local: "b"}
	right = QName{Namespace: "urn:a", Local: "c"}
	if got := Compare(left, right); got >= 0 {
		t.Fatalf("Compare() = %d, want < 0", got)
	}
}

func TestSortedMapKeys(t *testing.T) {
	in := map[QName]int{
		{Namespace: "urn:b", Local: "x"}: 1,
		{Namespace: "urn:a", Local: "z"}: 1,
		{Namespace: "urn:a", Local: "a"}: 1,
	}
	got := SortedMapKeys(in)
	want := []QName{
		{Namespace: "urn:a", Local: "a"},
		{Namespace: "urn:a", Local: "z"},
		{Namespace: "urn:b", Local: "x"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SortedMapKeys() = %v, want %v", got, want)
	}
}

func TestSortAndDedupe(t *testing.T) {
	in := []QName{
		{Namespace: "urn:b", Local: "x"},
		{Namespace: "urn:a", Local: "a"},
		{Namespace: "urn:b", Local: "x"},
		{Namespace: "urn:a", Local: "z"},
	}
	got := SortAndDedupe(in)
	want := []QName{
		{Namespace: "urn:a", Local: "a"},
		{Namespace: "urn:a", Local: "z"},
		{Namespace: "urn:b", Local: "x"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SortAndDedupe() = %v, want %v", got, want)
	}
}
