package validatorcompile

import (
	"github.com/jacoelho/xsd/internal/types"
)

// BuiltinTypeNames returns the deterministic builtin type compile order.
func BuiltinTypeNames() []types.TypeName {
	builtin := builtinTypeNames()
	out := make([]types.TypeName, len(builtin))
	copy(out, builtin)
	return out
}
