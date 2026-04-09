package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// idTrackCallbacks supplies the caller-owned runtime lookups and side effects.
type idTrackCallbacks struct {
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
	actual runtime.ValidatorID,
	callbacks idTrackCallbacks,
) error {
	return track(id, validators, canonical, actual, 0, callbacks)
}

// trackDefault performs ID and IDREF tracking for one defaulted or fixed value.
func trackDefault(
	id runtime.ValidatorID,
	validators runtime.ValidatorsBundle,
	canonical []byte,
	member runtime.ValidatorID,
	callbacks idTrackCallbacks,
) error {
	return track(id, validators, canonical, 0, member, callbacks)
}

func track(
	id runtime.ValidatorID,
	validators runtime.ValidatorsBundle,
	canonical []byte,
	actual runtime.ValidatorID,
	member runtime.ValidatorID,
	callbacks idTrackCallbacks,
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
			return xsderrors.Invalid("string validator out of range")
		}
		return callbacks.TrackString(kind, canonical)
	case runtime.VList:
		return trackCanonicalList(meta, validators, canonical, func(itemValidator runtime.ValidatorID, itemValue []byte) error {
			return track(itemValidator, validators, itemValue, 0, 0, callbacks)
		})
	case runtime.VUnion:
		if member != 0 {
			return track(member, validators, canonical, 0, 0, callbacks)
		}
		actualValidator := actual
		if actualValidator == 0 && callbacks.LookupUnionMember != nil {
			lookedUp, err := callbacks.LookupUnionMember(id, canonical)
			if err == nil {
				actualValidator = lookedUp
			}
		}
		if actualValidator != 0 {
			return track(actualValidator, validators, canonical, 0, 0, callbacks)
		}
	}

	return nil
}
