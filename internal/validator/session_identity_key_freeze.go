package validator

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

func freezeIdentityKey(arena *Arena, kind runtime.ValueKind, key []byte) runtime.ValueKey {
	if len(key) == 0 {
		return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, nil)}
	}
	if arena == nil {
		copied := slices.Clone(key)
		return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, copied), Bytes: copied}
	}
	buf := arena.Alloc(len(key))
	copy(buf, key)
	return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, buf), Bytes: buf}
}
