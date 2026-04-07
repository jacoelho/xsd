package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

// derive converts one canonical primitive lexical value into its runtime key.
// The caller owns dst and may reuse it across calls.
func derive(kind runtime.ValidatorKind, canonical, dst []byte) (runtime.ValueKind, []byte, error) {
	return deriveCanonicalPrimitiveKey(kind, canonical, dst)
}
