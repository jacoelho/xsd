package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/model"
	"github.com/jacoelho/xsd/internal/validator/names"
)

type elemFrame struct {
	local              []byte
	ns                 []byte
	modelState         model.State
	text               TextState
	model              runtime.ModelRef
	name               names.ID
	elem               runtime.ElemID
	typ                runtime.TypeID
	content            runtime.ContentKind
	mixed              bool
	nilled             bool
	hasChildElements   bool
	childErrorReported bool
}
