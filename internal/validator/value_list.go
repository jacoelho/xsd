package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeList(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valruntime.Options, needKey bool, metrics *valruntime.State) ([]byte, error) {
	out, bufs, err := valruntime.CanonicalizeList(
		valruntime.ListInput{
			Meta:            meta,
			Validators:      s.rt.Validators,
			Normalized:      normalized,
			ApplyWhitespace: opts.ApplyWhitespace,
			NeedKey:         needKey,
			Buffers: valruntime.ListBuffers{
				Value: s.valueScratch,
				Key:   s.keyTmp,
			},
		},
		func(itemValidator runtime.ValidatorID, item []byte, needItemKey bool) ([]byte, valruntime.ListItemResult, error) {
			itemOpts := valruntime.ListItemOptions(opts, needItemKey)
			canon, itemMetrics, err := s.validateValueInternalWithMetrics(itemValidator, item, resolver, itemOpts)
			if err != nil {
				return nil, valruntime.ListItemResult{}, err
			}
			itemKind, itemKey, ok := itemMetrics.Result.Key()
			return canon, valruntime.ListItemResult{
				KeyKind:  itemKind,
				KeyBytes: itemKey,
				KeySet:   ok,
			}, nil
		},
	)
	s.valueScratch = bufs.Value
	s.keyTmp = bufs.Key
	if err != nil {
		return nil, err
	}
	metrics.MeasureCache().SetListLength(out.Count)
	if out.KeySet {
		s.setKey(metrics, runtime.VKList, out.Key, false)
	}
	return out.Canonical, nil
}

func (s *Session) validateListNoCanonical(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valruntime.Options) error {
	itemOpts := valruntime.ListNoCanonicalItemOptions(opts)
	return valruntime.ValidateListNoCanonical(
		valruntime.ListNoCanonicalInput{
			Meta:            meta,
			Validators:      s.rt.Validators,
			Normalized:      normalized,
			ApplyWhitespace: opts.ApplyWhitespace,
		},
		func(itemValidator runtime.ValidatorID, itemValue []byte) error {
			if _, err := s.validateValueInternalOptions(itemValidator, itemValue, resolver, itemOpts); err != nil {
				return err
			}
			return nil
		},
	)
}
