package qname

import (
	"cmp"
	"maps"
	"slices"
)

// Compare orders QNames by namespace then local name.
func Compare(a, b QName) int {
	if a.Namespace != b.Namespace {
		return cmp.Compare(a.Namespace, b.Namespace)
	}
	return cmp.Compare(a.Local, b.Local)
}

// SortedMapKeys returns map keys in deterministic QName order.
func SortedMapKeys[V any](m map[QName]V) []QName {
	if len(m) == 0 {
		return nil
	}
	keys := slices.Collect(maps.Keys(m))
	SortInPlace(keys)
	return keys
}

// SortInPlace sorts QNames by namespace then local name.
func SortInPlace(names []QName) {
	slices.SortFunc(names, Compare)
}

// SortAndDedupe sorts QNames and removes duplicates in place.
func SortAndDedupe(names []QName) []QName {
	if len(names) < 2 {
		return names
	}
	SortInPlace(names)
	out := names[:0]
	var last QName
	for i, name := range names {
		if i == 0 || !name.Equal(last) {
			out = append(out, name)
			last = name
		}
	}
	return out
}
