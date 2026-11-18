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

// compareExpandedName is the expanded-name order: namespace URI, then local
// name, both by text. It is the only definition of that order, used by every
// diagnostic that lists or picks names and by every check that stops at its
// first failure, so none of them depends on the order in which names were
// interned.
func compareExpandedName(a, b ExpandedName) int {
	if n := compareNamespaceURI(a.Namespace, b.Namespace); n != 0 {
		return n
	}
	return cmp.Compare(a.Local, b.Local)
}

// compareNamespaceURI orders namespace URIs by text, the first key of
// compareExpandedName. sortNamespaceURIs and searchNamespaceURI apply the same
// order without an indirect call per comparison, for validation hot paths.
func compareNamespaceURI(a, b string) int {
	return cmp.Compare(a, b)
}

// sortNamespaceURIs sorts uris by compareNamespaceURI.
func sortNamespaceURIs(uris []string) {
	slices.Sort(uris)
}

// searchNamespaceURI reports whether uris, sorted by sortNamespaceURIs,
// holds uri.
func searchNamespaceURI(uris []string, uri string) bool {
	_, found := slices.BinarySearch(uris, uri)
	return found
}

// compareQNameText orders interned names a and b by compareExpandedName.
func compareQNameText(names *nameTable, a, b QName) int {
	return compareExpandedName(
		ExpandedName{Namespace: names.Namespace(a.Namespace), Local: names.Local(a.Local)},
		ExpandedName{Namespace: names.Namespace(b.Namespace), Local: names.Local(b.Local)},
	)
}
