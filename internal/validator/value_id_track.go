package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
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

func (s *Session) trackValidatedIDs(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, metrics *ValueMetrics) error {
	actual := metricsActualValidator(metrics)
	return trackValidated(id, s.rt.Validators, canonical, actual, s.idTrackCallbacks(resolver))
}

func (s *Session) trackDefaultValue(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, member runtime.ValidatorID) error {
	return trackDefault(id, s.rt.Validators, canonical, member, s.idTrackCallbacks(resolver))
}

func (s *Session) idTrackCallbacks(resolver value.NSResolver) Callbacks {
	return Callbacks{
		Meta:        s.validatorMetaIfPresent,
		StringKind:  s.stringKind,
		TrackString: s.trackIDs,
		LookupUnionMember: func(id runtime.ValidatorID, canonical []byte) (runtime.ValidatorID, error) {
			return s.lookupActualUnionValidator(id, canonical, resolver)
		},
	}
}

func metricsActualValidator(metrics *ValueMetrics) runtime.ValidatorID {
	state := metrics.result()
	if state == nil {
		return 0
	}
	_, actual := state.Actual()
	return actual
}
