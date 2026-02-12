package validatorgen

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *compiler) makeValueKey(kind runtime.ValueKind, key []byte) runtime.ValueKey {
	hash := runtime.HashKey(kind, key)
	bytes := slices.Clone(key)
	return runtime.ValueKey{
		Kind:  kind,
		Hash:  hash,
		Bytes: bytes,
	}
}
