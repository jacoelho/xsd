package schema

import "github.com/jacoelho/xsd/internal/value"

func sealAll(b *value.Builder) (*value.Program, error) {
	keep := make([]bool, userTypeCount(b))
	for i := range keep {
		keep[i] = true
	}
	return b.SealKeeping(keep)
}

// userTypeCount counts the completed user types b holds from the first user
// ID; a reserved slot ends the count, and SealKeeping reports it first.
func userTypeCount(b *value.Builder) int {
	n := 0
	for id := value.BuiltinTypeCount; ; id++ {
		if _, ok := b.TypeView(id); !ok {
			return n
		}
		n++
	}
}

func builtinTypeID(local string) (value.TypeID, bool) {
	for id := range value.BuiltinTypeCount {
		if name, _ := value.BuiltinTypeName(id); name == local {
			return id, true
		}
	}
	return value.NoType, false
}
