package validatorcompile

import (
	"github.com/jacoelho/xsd/internal/model"
)

// BuiltinTypeNames returns the deterministic builtin type compile order.
func BuiltinTypeNames() []model.TypeName {
	builtin := builtinTypeNames()
	out := make([]model.TypeName, len(builtin))
	copy(out, builtin)
	return out
}
