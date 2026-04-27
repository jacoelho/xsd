package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func (s *Session) canonicalizeList(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	runner := newValueRunner(s)
	out, bufs, err := canonicalizeListRuntime(
		meta,
		s.rt.ValidatorBundle(),
		normalized,
		opts.ApplyWhitespace,
		needKey,
		listBuffers{
			Value: s.buffers.valueScratch,
			Key:   s.buffers.keyTmp,
		},
		func(itemValidator runtime.ValidatorID, item []byte, needItemKey bool) ([]byte, runtime.ValueKind, []byte, bool, error) {
			itemOpts := opts
			itemOpts.ApplyWhitespace = false
			itemOpts.TrackIDs = false
			itemOpts.RequireCanonical = true
			itemOpts.StoreValue = false
			itemOpts.NeedKey = needItemKey
			result, err := runner.validate(valueRequest{
				Validator: itemValidator,
				Lexical:   item,
				Resolver:  resolver,
				Options:   itemOpts,
			})
			if err != nil {
				return nil, runtime.VKInvalid, nil, false, err
			}
			return result.Canonical, result.KeyKind, result.KeyBytes, result.HasKey, nil
		},
	)
	s.buffers.valueScratch = bufs.Value
	s.buffers.keyTmp = bufs.Key
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
	runner := newValueRunner(s)
	itemOpts := opts
	itemOpts.ApplyWhitespace = false
	itemOpts.TrackIDs = false
	itemOpts.RequireCanonical = false
	itemOpts.StoreValue = false
	itemOpts.NeedKey = false
	return validateListNoCanonicalRuntime(
		meta,
		s.rt.ValidatorBundle(),
		normalized,
		opts.ApplyWhitespace,
		func(itemValidator runtime.ValidatorID, itemValue []byte) error {
			if _, err := runner.validate(valueRequest{
				Validator: itemValidator,
				Lexical:   itemValue,
				Resolver:  resolver,
				Options:   itemOpts,
			}); err != nil {
				return err
			}
			return nil
		},
	)
}

func (s *Session) canonicalizeUnion(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	if s == nil || s.rt == nil {
		return nil, xsderrors.Invalid("runtime schema missing")
	}
	runner := newValueRunner(s)
	enums := s.rt.EnumTable()
	return Union(
		s.rt.PatternTable(),
		s.rt.FacetTable(),
		normalized,
		lexical,
		&enums,
		s.rt.ValidatorBundle(),
		meta,
		opts.ApplyWhitespace,
		needKey,
		metrics.result(),
		func(member runtime.ValidatorID, memberLex []byte, applyWhitespace, needKey bool) ([]byte, runtime.ValueKind, []byte, bool, error) {
			memberOpts := opts
			memberOpts.ApplyWhitespace = applyWhitespace
			memberOpts.TrackIDs = false
			memberOpts.RequireCanonical = true
			memberOpts.StoreValue = false
			memberOpts.NeedKey = needKey
			result, err := runner.validate(valueRequest{
				Validator: member,
				Lexical:   memberLex,
				Resolver:  resolver,
				Options:   memberOpts,
			})
			if err != nil {
				return nil, runtime.VKInvalid, nil, false, err
			}
			return result.Canonical, result.KeyKind, result.KeyBytes, result.HasKey, nil
		},
	)
}
