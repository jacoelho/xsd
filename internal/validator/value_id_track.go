package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) trackIDs(kind runtime.StringKind, canonical []byte) error {
	switch kind {
	case runtime.StringID:
		return s.recordID(canonical)
	case runtime.StringIDREF:
		s.recordIDRef(canonical)
	case runtime.StringEntity:
		// ENTITY validation handled elsewhere
	}
	return nil
}

func (s *Session) trackValidatedIDs(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, metrics *valruntime.State) error {
	return trackValidated(id, s.rt.Validators, canonical, metrics, Callbacks{
		Meta:       s.validatorMetaIfPresent,
		StringKind: s.stringKind,
		TrackString: func(kind runtime.StringKind, canonical []byte) error {
			return s.trackIDs(kind, canonical)
		},
		LookupUnionMember: func(id runtime.ValidatorID, canonical []byte) (runtime.ValidatorID, error) {
			return s.lookupActualUnionValidator(id, canonical, resolver)
		},
	})
}

func (s *Session) trackDefaultValue(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, member runtime.ValidatorID) error {
	return trackDefault(id, s.rt.Validators, canonical, member, Callbacks{
		Meta:       s.validatorMetaIfPresent,
		StringKind: s.stringKind,
		TrackString: func(kind runtime.StringKind, canonical []byte) error {
			return s.trackIDs(kind, canonical)
		},
		LookupUnionMember: func(id runtime.ValidatorID, canonical []byte) (runtime.ValidatorID, error) {
			return s.lookupActualUnionValidator(id, canonical, resolver)
		},
	})
}
