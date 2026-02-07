package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

func sortedQNames[V any](m map[types.QName]V) []types.QName {
	return traversal.SortedQNames(m)
}
