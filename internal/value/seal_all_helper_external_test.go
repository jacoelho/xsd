package value_test

import "github.com/jacoelho/xsd/internal/value"

// sealAll seals b keeping every type.
func sealAll(b *value.Builder) (*value.Program, error) {
	keep := make([]bool, b.TypeCount())
	for i := range keep {
		keep[i] = true
	}
	return b.SealKeeping(keep)
}
