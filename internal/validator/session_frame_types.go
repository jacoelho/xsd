package validator

import "github.com/jacoelho/xsd/internal/runtime"

type elemFrame struct {
	local              []byte
	ns                 []byte
	modelState         ModelState
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

type nsFrame struct {
	off      uint32
	len      uint32
	cacheOff uint32
}
