package validator

import "github.com/jacoelho/xsd/internal/runtime"

func isSimpleContent(rt *runtime.Schema, typeID runtime.TypeID) bool {
	if typeID == 0 || int(typeID) >= len(rt.Types) {
		return false
	}
	typ := rt.Types[typeID]
	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		return true
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(rt.ComplexTypes) {
			return false
		}
		ct := rt.ComplexTypes[typ.Complex.ID]
		return ct.Content == runtime.ContentSimple
	default:
		return false
	}
}
