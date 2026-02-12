package validator

import "github.com/jacoelho/xsd/internal/runtime"

func elementByID(rt *runtime.Schema, id runtime.ElemID) (*runtime.Element, bool) {
	if rt == nil || id == 0 || int(id) >= len(rt.Elements) {
		return nil, false
	}
	return &rt.Elements[id], true
}
