package value

func sealAll(b *Builder) (*Program, error) {
	keep := make([]bool, b.UserTypeCount())
	for i := range keep {
		keep[i] = true
	}
	return b.SealKeeping(keep)
}
