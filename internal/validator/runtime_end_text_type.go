package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) resolveEndTextType(frame elemFrame, typ runtime.Type) (runtime.ComplexType, bool, runtime.ValidatorID, error) {
	var ct runtime.ComplexType
	hasComplexText := false
	textValidator := runtime.ValidatorID(0)

	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		textValidator = typ.Validator
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
			return ct, false, 0, fmt.Errorf("complex type %d missing", frame.typ)
		}
		ct = s.rt.ComplexTypes[typ.Complex.ID]
		hasComplexText = true
		textValidator = ct.TextValidator
	}
	return ct, hasComplexText, textValidator, nil
}
