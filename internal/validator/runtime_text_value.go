package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) ValidateTextValue(typeID runtime.TypeID, text []byte, resolver value.NSResolver, requireCanonical bool) ([]byte, valueMetrics, error) {
	var metrics valueMetrics
	canon, err := s.validateTextValueCore(typeID, text, resolver, requireCanonical, &metrics)
	return canon, metrics, err
}

func (s *Session) validateTextValueCore(typeID runtime.TypeID, text []byte, resolver value.NSResolver, requireCanonical bool, metrics *valueMetrics) ([]byte, error) {
	if s == nil || s.rt == nil {
		return nil, fmt.Errorf("session missing runtime schema")
	}
	typ, ok := s.typeByID(typeID)
	if !ok {
		return nil, fmt.Errorf("type %d not found", typeID)
	}
	needMetrics := s.hasIdentityConstraints()
	opts := valueOptions{
		applyWhitespace:  true,
		trackIDs:         true,
		requireCanonical: requireCanonical,
		storeValue:       needMetrics,
	}
	var validatorID runtime.ValidatorID
	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		validatorID = typ.Validator
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
			return nil, fmt.Errorf("complex type %d missing", typeID)
		}
		ct := s.rt.ComplexTypes[typ.Complex.ID]
		if ct.Content != runtime.ContentSimple {
			return nil, fmt.Errorf("type %d does not have simple content", typeID)
		}
		validatorID = ct.TextValidator
	default:
		return nil, fmt.Errorf("unknown type kind %d", typ.Kind)
	}
	// fast path: no identity constraints, skip metrics computation
	if !needMetrics {
		canon, err := s.validateValueInternalOptions(validatorID, text, resolver, opts)
		if err != nil {
			return nil, wrapValueError(err)
		}
		return canon, nil
	}
	// slow path: need metrics for identity constraints
	canon, err := s.validateValueCore(validatorID, text, resolver, opts, metrics)
	if err != nil {
		return nil, wrapValueError(err)
	}
	return canon, nil
}
