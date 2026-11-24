package value_test

import "github.com/jacoelho/xsd/internal/value"

func sealAll(b *value.Builder) (*value.Program, error) {
	keep := make([]bool, b.UserTypeCount())
	for i := range keep {
		keep[i] = true
	}
	return b.SealKeeping(keep)
}
