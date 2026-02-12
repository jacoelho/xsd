package xiter

import (
	"cmp"
	"iter"
	"slices"
)

// Slice exposes a slice as an iterator sequence.
func Slice[T any](items []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, item := range items {
			if !yield(item) {
				return
			}
		}
	}
}

// Collect gathers all values from a sequence.
func Collect[T any](seq iter.Seq[T]) []T {
	return slices.Collect(seq)
}

// Count returns how many values are yielded by a sequence.
func Count[T any](seq iter.Seq[T]) int {
	n := 0
	for range seq {
		n++
	}
	return n
}

// SortedKeys yields map keys in deterministic sorted order.
func SortedKeys[K cmp.Ordered, V any](m map[K]V) iter.Seq[K] {
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return Slice(keys)
}

// ValuesBySortedKeys yields map values following sorted key order.
func ValuesBySortedKeys[K cmp.Ordered, V any](m map[K]V) iter.Seq[V] {
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return func(yield func(V) bool) {
		for _, key := range keys {
			if !yield(m[key]) {
				return
			}
		}
	}
}
