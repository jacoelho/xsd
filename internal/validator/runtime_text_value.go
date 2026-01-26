package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) ValidateTextValue(typeID runtime.TypeID, text []byte, resolver value.NSResolver, requireCanonical bool) ([]byte, error) {
	if s == nil || s.rt == nil {
		return nil, fmt.Errorf("session missing runtime schema")
	}
	typ, ok := s.typeByID(typeID)
	if !ok {
		return nil, fmt.Errorf("type %d not found", typeID)
	}
	opts := valueOptions{
		applyWhitespace:  true,
		trackIDs:         true,
		requireCanonical: requireCanonical,
		storeValue:       s.hasIdentityConstraints(),
	}
	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		canon, err := s.validateValueInternalOptions(typ.Validator, text, resolver, opts)
		if err != nil {
			return nil, wrapValueError(err)
		}
		return canon, nil
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
			return nil, fmt.Errorf("complex type %d missing", typeID)
		}
		ct := s.rt.ComplexTypes[typ.Complex.ID]
		if ct.Content != runtime.ContentSimple {
			return nil, fmt.Errorf("type %d does not have simple content", typeID)
		}
		canon, err := s.validateValueInternalOptions(ct.TextValidator, text, resolver, opts)
		if err != nil {
			return nil, wrapValueError(err)
		}
		return canon, nil
	default:
		return nil, fmt.Errorf("unknown type kind %d", typ.Kind)
	}
}
