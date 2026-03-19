package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
)

// Callbacks supplies the caller-owned runtime lookups and side effects.
type Callbacks struct {
	Meta              func(runtime.ValidatorID) (runtime.ValidatorMeta, bool, error)
	StringKind        func(runtime.ValidatorMeta) (runtime.StringKind, bool)
	TrackString       func(runtime.StringKind, []byte) error
	LookupUnionMember func(runtime.ValidatorID, []byte) (runtime.ValidatorID, error)
}

// trackValidated performs ID and IDREF tracking for one validated value.
func trackValidated(
	id runtime.ValidatorID,
	validators runtime.ValidatorsBundle,
	canonical []byte,
	state *valruntime.State,
	callbacks Callbacks,
) error {
	return track(id, validators, canonical, actualState(state), 0, callbacks)
}

// trackDefault performs ID and IDREF tracking for one defaulted or fixed value.
func trackDefault(
	id runtime.ValidatorID,
	validators runtime.ValidatorsBundle,
	canonical []byte,
	member runtime.ValidatorID,
	callbacks Callbacks,
) error {
	return track(id, validators, canonical, nil, member, callbacks)
}

func actualState(state *valruntime.State) *valruntime.Result {
	if state == nil {
		return nil
	}
	return state.ResultState()
}

func track(
	id runtime.ValidatorID,
	validators runtime.ValidatorsBundle,
	canonical []byte,
	actual *valruntime.Result,
	member runtime.ValidatorID,
	callbacks Callbacks,
) error {
	meta, ok, err := callbacks.Meta(id)
	if err != nil {
		return err
	}
	if !ok || meta.Flags&runtime.ValidatorMayTrackIDs == 0 {
		return nil
	}

	switch meta.Kind {
	case runtime.VString:
		kind, ok := callbacks.StringKind(meta)
		if !ok {
			return diag.Invalid("string validator out of range")
		}
		return callbacks.TrackString(kind, canonical)
	case runtime.VList:
		return valruntime.TrackCanonicalList(meta, validators, canonical, func(itemValidator runtime.ValidatorID, itemValue []byte) error {
			return track(itemValidator, validators, itemValue, nil, 0, callbacks)
		})
	case runtime.VUnion:
		if member != 0 {
			return track(member, validators, canonical, nil, 0, callbacks)
		}
		actualValidator := valruntime.ResolveActualUnionValidator(actual, func() (runtime.ValidatorID, error) {
			if callbacks.LookupUnionMember == nil {
				return 0, nil
			}
			return callbacks.LookupUnionMember(id, canonical)
		})
		if actualValidator != 0 {
			return track(actualValidator, validators, canonical, nil, 0, callbacks)
		}
	}

	return nil
}
