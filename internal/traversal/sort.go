package traversal

import (
	"github.com/jacoelho/xsd/internal/model"
	qnameorder "github.com/jacoelho/xsd/internal/qname"
)

// SortedQNames returns QNames in deterministic order (namespace, local).
func SortedQNames[V any](m map[model.QName]V) []model.QName {
	return qnameorder.SortedMapKeys(m)
}
