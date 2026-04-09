package validator

import (
	"bytes"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrValue(
	validated []Start,
	attr Start,
	resolver value.NSResolver,
	storeAttrs bool,
	storeValues bool,
	spec ValueSpec,
	seenID *bool,
) ([]Start, error) {
	opts := valueOptions{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: spec.Fixed.Present,
		StoreValue:       storeAttrs && storeValues,
		NeedKey:          spec.Fixed.Present || storeAttrs,
	}
	var metrics *ValueMetrics
	if storeAttrs || spec.Fixed.Present {
		metrics = s.acquireMetricsState()
		defer s.releaseMetricsState()
	}
	canonical, err := s.validateValueCore(spec.Validator, attr.Value, resolver, opts, metrics)
	if err != nil {
		return nil, err
	}

	if s.isIDValidator(spec.Validator) {
		if *seenID {
			return nil, xsderrors.New(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
		}
		*seenID = true
	}

	keyKind := runtime.VKInvalid
	var keyBytes []byte
	hasKey := false
	if metrics != nil {
		keyKind, keyBytes, hasKey = metrics.State.Key()
	}
	if storeAttrs {
		if storeValues {
			validated = StoreCanonical(validated, attr, true, s.ensureAttrNameStable, canonical, keyKind, keyBytes)
		} else {
			if len(keyBytes) > 0 {
				keyBytes = s.storeKey(keyBytes)
			}
			validated = StoreCanonicalIdentity(validated, attr, true, s.ensureAttrNameStable, keyKind, keyBytes)
		}
	}
	if !spec.Fixed.Present {
		return validated, nil
	}

	if spec.FixedKey.Ref.Present {
		actualKind := keyKind
		actualKey := keyBytes
		if !hasKey {
			actualKind, actualKey, err = s.keyForCanonicalValue(spec.Validator, canonical, resolver, spec.FixedMember)
			if err != nil {
				return nil, err
			}
		}
		if actualKind != spec.FixedKey.Kind || !bytes.Equal(actualKey, valueBytes(s.rt.Values, spec.FixedKey.Ref)) {
			return nil, xsderrors.New(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
		}
		return validated, nil
	}

	if !bytes.Equal(canonical, valueBytes(s.rt.Values, spec.Fixed)) {
		return nil, xsderrors.New(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
	}
	return validated, nil
}
