package validatorgen

import (
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtimeids"
)

func builtinTypeNames() []model.TypeName {
	return runtimeids.BuiltinTypeNames()
}
