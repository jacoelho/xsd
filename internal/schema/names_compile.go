package schema

import (
	"cmp"
	"maps"
	"slices"
)

// sortedQNames returns map keys ordered by expanded name text.
func sortedQNames[T any](m map[QName]T, names nameTable) []QName {
	return slices.SortedFunc(maps.Keys(m), func(a, b QName) int {
		return compareQNameText(&names, a, b)
	})
}

// compareQNameText orders a and b by namespace URI, then local name. It is
// the order of every diagnostic that lists or picks names, which must not
// depend on the order in which names were interned.
func compareQNameText(names *nameTable, a, b QName) int {
	if n := cmp.Compare(names.Namespace(a.Namespace), names.Namespace(b.Namespace)); n != 0 {
		return n
	}
	return cmp.Compare(names.Local(a.Local), names.Local(b.Local))
}
