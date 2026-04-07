package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// TextValueOptions controls canonicalization and key-derivation behavior for text validation.
type TextValueOptions struct {
	RequireCanonical bool
	NeedKey          bool
}

// ValidateTextValue validates simple-content text and returns canonical bytes plus value metrics.
func (s *Session) ValidateTextValue(typeID runtime.TypeID, text []byte, resolver value.NSResolver, textOpts TextValueOptions) ([]byte, ValueMetrics, error) {
	metricState := s.acquireMetricsState()
	defer s.releaseMetricsState()
	canon, err := s.validateTextValueCore(typeID, text, resolver, textOpts, metricState)
	if err != nil {
		return nil, ValueMetrics{}, err
	}
	return canon, *metricState, nil
}

func (s *Session) validateTextValueCore(typeID runtime.TypeID, text []byte, resolver value.NSResolver, textOpts TextValueOptions, metrics *ValueMetrics) ([]byte, error) {
	if s == nil || s.rt == nil {
		return nil, fmt.Errorf("session missing runtime schema")
	}
	typ, ok := s.typeByID(typeID)
	if !ok {
		return nil, fmt.Errorf("type %d not found", typeID)
	}
	storeValue := s.hasIdentityConstraints()
	opts := valueOptions{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: textOpts.RequireCanonical,
		StoreValue:       storeValue,
		NeedKey:          textOpts.NeedKey,
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
	if !opts.StoreValue && !opts.NeedKey {
		canon, err := s.validateValueCore(validatorID, text, resolver, opts, nil)
		if err != nil {
			return nil, err
		}
		return canon, nil
	}
	// slow path: need metrics for identity constraints
	canon, err := s.validateValueCore(validatorID, text, resolver, opts, metrics)
	if err != nil {
		return nil, err
	}
	return canon, nil
}
