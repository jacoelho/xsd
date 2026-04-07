package preprocessor

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

func mergeNamed[V any](
	source map[model.QName]V,
	target map[model.QName]V,
	targetOrigins map[model.QName]string,
	remap func(model.QName) model.QName,
	originFor func(model.QName) string,
	insert func(V) V,
	candidate func(V) V,
	equivalent func(existing V, candidate V) bool,
	kindName string,
) error {
	if insert == nil {
		insert = func(value V) V { return value }
	}
	for _, name := range model.SortedMapKeys(source) {
		value := source[name]
		targetQName := remap(name)
		origin := originFor(name)
		if existing, exists := target[targetQName]; exists {
			if targetOrigins[targetQName] == origin {
				continue
			}
			if equivalent != nil {
				cand := value
				if candidate != nil {
					cand = candidate(value)
				}
				if equivalent(existing, cand) {
					continue
				}
			}
			return fmt.Errorf("duplicate %s %s", kindName, targetQName)
		}
		target[targetQName] = insert(value)
		targetOrigins[targetQName] = origin
	}
	return nil
}
