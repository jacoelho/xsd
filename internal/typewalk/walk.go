package typewalk

import "github.com/jacoelho/xsd/internal/model"

// NextType returns the next base type for a derivation walk.
type NextType func(model.Type) model.Type

// Walk traverses a type chain until visit returns false, next returns nil, or a cycle is reached.
func Walk(start model.Type, next NextType, visit func(model.Type) bool) {
	if start == nil || visit == nil {
		return
	}
	seen := make(map[model.Type]bool)
	for current := start; current != nil; {
		if seen[current] {
			return
		}
		seen[current] = true
		if !visit(current) {
			return
		}
		if next == nil {
			return
		}
		current = next(current)
	}
}
