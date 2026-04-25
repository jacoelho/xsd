package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func (s *Session) trackIDs(kind runtime.StringKind, canonical []byte) error {
	switch kind {
	case runtime.StringID:
		return s.recordID(canonical)
	case runtime.StringIDREF:
		s.recordIDRef(canonical)
	case runtime.StringEntity:
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

func (s *Session) idTrackCallbacks(resolver value.NSResolver) idTrackCallbacks {
	return idTrackCallbacks{
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

func (s *Session) keyForCanonicalValue(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
	meta, err := s.validatorMeta(id)
	if err != nil {
		return runtime.VKInvalid, nil, err
	}
	switch meta.Kind {
	case runtime.VList:
		keyKind, listKey, err := deriveCanonicalListKey(meta, s.rt.Validators, canonical, s.buffers.keyTmp[:0], func(itemValidator runtime.ValidatorID, itemValue []byte) (runtime.ValueKind, []byte, error) {
			return s.keyForCanonicalValue(itemValidator, itemValue, resolver, 0)
		})
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		s.buffers.keyTmp = listKey
		return keyKind, listKey, nil
	case runtime.VUnion:
		if member != 0 {
			return s.keyForCanonicalValue(member, canonical, resolver, 0)
		}
		actualValidator, err := s.lookupActualUnionValidator(id, canonical, resolver)
		if err == nil && actualValidator != 0 {
			return s.keyForCanonicalValue(actualValidator, canonical, resolver, 0)
		}
		return runtime.VKInvalid, nil, xsderrors.Invalid("union value does not match any member type")
	default:
		keyKind, keyBytes, err := deriveCanonicalPrimitiveKey(meta.Kind, canonical, s.buffers.keyTmp[:0])
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		s.buffers.keyTmp = keyBytes
		return keyKind, keyBytes, nil
	}
}

func deriveCanonicalPrimitiveKey(kind runtime.ValidatorKind, canonical, dst []byte) (runtime.ValueKind, []byte, error) {
	switch kind {
	case runtime.VAnyURI:
		return runtime.VKString, runtime.StringKeyBytes(dst[:0], 1, canonical), nil
	case runtime.VQName, runtime.VNotation:
		tag := byte(0)
		if kind == runtime.VNotation {
			tag = 1
		}
		key := runtime.QNameKeyCanonical(dst[:0], tag, canonical)
		if len(key) == 0 {
			return runtime.VKInvalid, nil, xsderrors.Invalid("invalid QName key")
		}
		return runtime.VKQName, key, nil
	case runtime.VHexBinary:
		decoded, err := value.ParseHexBinary(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, xsderrors.Invalid(err.Error())
		}
		return runtime.VKBinary, runtime.BinaryKeyBytes(dst[:0], 0, decoded), nil
	case runtime.VBase64Binary:
		decoded, err := value.ParseBase64Binary(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, xsderrors.Invalid(err.Error())
		}
		return runtime.VKBinary, runtime.BinaryKeyBytes(dst[:0], 1, decoded), nil
	default:
		keyKind, keyBytes, err := runtime.KeyForValidatorKind(kind, canonical)
		if err != nil {
			return runtime.VKInvalid, nil, xsderrors.Invalid(err.Error())
		}
		dst = append(dst[:0], keyBytes...)
		return keyKind, dst, nil
	}
}

func (s *Session) setKey(metrics *ValueMetrics, kind runtime.ValueKind, key []byte, store bool) {
	if s == nil {
		return
	}
	state := metrics.result()
	if state == nil {
		return
	}
	if store {
		key = s.storeKey(key)
	}
	state.SetKey(kind, key)
}

func (s *Session) storeValue(data []byte) []byte {
	if s == nil {
		return nil
	}
	start := len(s.buffers.valueBuf)
	s.buffers.valueBuf = append(s.buffers.valueBuf, data...)
	return s.buffers.valueBuf[start:len(s.buffers.valueBuf)]
}

func (s *Session) maybeStore(data []byte, store bool) []byte {
	if store {
		return s.storeValue(data)
	}
	return data
}

func (s *Session) storeKey(data []byte) []byte {
	if s == nil {
		return nil
	}
	start := len(s.buffers.keyBuf)
	s.buffers.keyBuf = append(s.buffers.keyBuf, data...)
	return s.buffers.keyBuf[start:len(s.buffers.keyBuf)]
}

func (s *Session) finalizeValue(canonical []byte, opts valueOptions, metrics *ValueMetrics, metricsInternal bool) []byte {
	if !opts.StoreValue {
		return canonical
	}
	canonStored := s.storeValue(canonical)
	state := metrics.result()
	if state != nil && state.HasKey() && !metricsInternal {
		kind, key, _ := state.Key()
		s.setKey(metrics, kind, key, true)
	}
	return canonStored
}
