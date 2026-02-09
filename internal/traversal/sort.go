package traversal

import (
	"cmp"
	"slices"

	"github.com/jacoelho/xsd/internal/types"
)

// SortedQNames returns QNames in deterministic order (namespace, local).
func SortedQNames[V any](m map[types.QName]V) []types.QName {
	if len(m) == 0 {
		return nil
	}
	keys := make([]types.QName, 0, len(m))
	for qname := range m {
		keys = append(keys, qname)
	}
	slices.SortFunc(keys, func(a, b types.QName) int {
		if a.Namespace != b.Namespace {
			return cmp.Compare(a.Namespace, b.Namespace)
		}
		return cmp.Compare(a.Local, b.Local)
	})
	return keys
}
