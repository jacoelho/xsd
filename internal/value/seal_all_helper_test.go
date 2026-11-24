package value

// sealAll seals b keeping every type.
func sealAll(b *Builder) (*Program, error) {
	keep := make([]bool, b.TypeCount())
	for i := range keep {
		keep[i] = true
	}
	return b.SealKeeping(keep)
}
