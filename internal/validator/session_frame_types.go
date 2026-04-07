package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

type elemFrame struct {
	local              []byte
	ns                 []byte
	modelState         StartModelState
	text               TextState
	model              runtime.ModelRef
	name               NameID
	elem               runtime.ElemID
	typ                runtime.TypeID
	content            runtime.ContentKind
	mixed              bool
	nilled             bool
	hasChildElements   bool
	childErrorReported bool
}
