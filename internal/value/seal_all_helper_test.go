package value

func sealAll(b *Builder) (*Program, error) {
	keep := make([]bool, userTypeCount(b))
	for i := range keep {
		keep[i] = true
	}
	return b.SealKeeping(keep)
}

// userTypeCount counts the completed user types b holds from the first user
// ID; a reserved slot ends the count, and SealKeeping reports it first.
func userTypeCount(b *Builder) int {
	n := 0
	for id := BuiltinTypeCount; ; id++ {
		if _, ok := b.TypeView(id); !ok {
			return n
		}
		n++
	}
}

func builtinTypeID(local string) (TypeID, bool) {
	for id := range BuiltinTypeCount {
		if name, _ := BuiltinTypeName(id); name == local {
			return id, true
		}
	}
	return NoType, false
}
