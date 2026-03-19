package validator

import "github.com/jacoelho/xsd/internal/validator/attrs"

func (s *Session) validateSimpleTypeAttrsClassified(inputAttrs []attrs.Start, classes []attrs.Class, storeAttrs bool) (AttrResult, error) {
	validated, err := attrs.ValidateSimple(
		s.rt,
		inputAttrs,
		classes,
		storeAttrs,
		s.attrState.PrepareValidated(storeAttrs, len(inputAttrs)),
		func(validated []attrs.Start, attr attrs.Start, storeAttrs bool) []attrs.Start {
			return attrs.StoreRaw(validated, attr, storeAttrs, s.ensureAttrNameStable, s.storeValue)
		},
	)
	if err != nil {
		return AttrResult{}, err
	}
	if storeAttrs {
		s.attrState.Validated = validated[:0]
	}
	return AttrResult{Attrs: validated}, nil
}
