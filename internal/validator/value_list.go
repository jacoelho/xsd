package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeList(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	out, bufs, err := canonicalizeListRuntime(
		meta,
		s.rt.Validators,
		normalized,
		opts.ApplyWhitespace,
		needKey,
		listBuffers{
			Value: s.valueScratch,
			Key:   s.keyTmp,
		},
		func(itemValidator runtime.ValidatorID, item []byte, needItemKey bool) ([]byte, runtime.ValueKind, []byte, bool, error) {
			itemOpts := opts
			itemOpts.ApplyWhitespace = false
			itemOpts.TrackIDs = false
			itemOpts.RequireCanonical = true
			itemOpts.StoreValue = false
			itemOpts.NeedKey = needItemKey
			var itemMetrics ValueMetrics
			canon, err := s.validateValueCore(itemValidator, item, resolver, itemOpts, &itemMetrics)
			if err != nil {
				return nil, runtime.VKInvalid, nil, false, err
			}
			itemKind, itemKey, ok := itemMetrics.State.Key()
			return canon, itemKind, itemKey, ok, nil
		},
	)
	s.valueScratch = bufs.Value
	s.keyTmp = bufs.Key
	if err != nil {
		return nil, err
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetListLength(out.Count)
	}
	if out.KeySet {
		s.setKey(metrics, runtime.VKList, out.Key, false)
	}
	return out.Canonical, nil
}

func (s *Session) validateListNoCanonical(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions) error {
	itemOpts := opts
	itemOpts.ApplyWhitespace = false
	itemOpts.TrackIDs = false
	itemOpts.RequireCanonical = false
	itemOpts.StoreValue = false
	itemOpts.NeedKey = false
	return validateListNoCanonicalRuntime(
		meta,
		s.rt.Validators,
		normalized,
		opts.ApplyWhitespace,
		func(itemValidator runtime.ValidatorID, itemValue []byte) error {
			if _, err := s.validateValueCore(itemValidator, itemValue, resolver, itemOpts, nil); err != nil {
				return err
			}
			return nil
		},
	)
}
