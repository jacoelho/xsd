package validate

import "github.com/jacoelho/xsd/internal/value"

func sealAll(b *value.Builder) (*value.Program, error) {
	keep := make([]bool, b.TypeCount())
	for i := range keep {
		keep[i] = true
	}
	return b.SealKeeping(keep)
}
