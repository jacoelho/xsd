package validator

import (
	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateValueInternalOptions(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions) ([]byte, error) {
	return s.validateValueCore(id, lexical, resolver, opts, nil)
}

func (s *Session) validateValueInternalWithMetrics(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions) ([]byte, valueMetrics, error) {
	var metrics valueMetrics
	canon, err := s.validateValueCore(id, lexical, resolver, opts, &metrics)
	return canon, metrics, err
}

func (s *Session) validateValueCore(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions, metrics *valueMetrics) ([]byte, error) {
	if s == nil || s.rt == nil {
		return nil, valueErrorf(valueErrInvalid, "runtime schema missing")
	}
	if id == 0 {
		return nil, valueErrorf(valueErrInvalid, "validator missing")
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return nil, valueErrorf(valueErrInvalid, "validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	var localMetrics valueMetrics
	metricsInternal := false
	ensureMetrics := func() {
		if metrics == nil {
			metrics = &localMetrics
			metricsInternal = true
		}
	}
	if (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary) && s.hasLengthFacet(meta) {
		ensureMetrics()
	}
	if opts.trackIDs && meta.Kind == runtime.VUnion {
		ensureMetrics()
	}
	normalized := lexical
	popNorm := false
	if opts.applyWhitespace {
		if meta.Kind == runtime.VList || meta.Kind == runtime.VUnion {
			buf := s.pushNormBuf(len(lexical))
			popNorm = true
			normalized = value.NormalizeWhitespace(valueWhitespaceMode(meta.WhiteSpace), lexical, buf)
		} else {
			normalized = value.NormalizeWhitespace(valueWhitespaceMode(meta.WhiteSpace), lexical, s.normBuf)
		}
	}
	if popNorm {
		defer s.popNormBuf()
	}
	needsCanonical := opts.requireCanonical || meta.Facets.Len != 0 || meta.Kind == runtime.VUnion || meta.Kind == runtime.VQName || meta.Kind == runtime.VNotation
	if opts.storeValue || opts.needKey {
		needsCanonical = true
	}
	needEnumKey := meta.Flags&runtime.ValidatorHasEnum != 0
	if needEnumKey {
		ensureMetrics()
	}
	needKey := opts.needKey || opts.storeValue || needEnumKey
	if !needsCanonical {
		canon, err := s.validateValueNoCanonical(meta, normalized, resolver, opts)
		if err != nil {
			return nil, err
		}
		if opts.trackIDs {
			if err := s.trackValidatedIDs(id, canon, resolver, metrics); err != nil {
				return nil, err
			}
		}
		return canon, nil
	}
	canon, err := s.canonicalizeValueCore(meta, normalized, lexical, resolver, opts, needKey, metrics)
	if err != nil {
		return nil, err
	}
	if err := s.applyFacets(meta, normalized, canon, metrics); err != nil {
		return nil, err
	}
	if !opts.storeValue && (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary) {
		canon = append([]byte(nil), canon...)
	}
	canon = s.finalizeValue(canon, opts, metrics, metricsInternal)
	if opts.trackIDs {
		if err := s.trackValidatedIDs(id, canon, resolver, metrics); err != nil {
			return nil, err
		}
	}
	return canon, nil
}

func (s *Session) hasLengthFacet(meta runtime.ValidatorMeta) bool {
	if s == nil || s.rt == nil || meta.Facets.Len == 0 {
		return false
	}
	ok, err := facetengine.RuntimeProgramHasOp(meta, s.rt.Facets, runtime.FLength, runtime.FMinLength, runtime.FMaxLength)
	return err == nil && ok
}

func (s *Session) canonicalizeValueCore(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *valueMetrics) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString, runtime.VBoolean, runtime.VDecimal, runtime.VInteger, runtime.VFloat, runtime.VDouble, runtime.VDuration:
		return s.canonicalizeAtomic(meta, normalized, needKey, metrics)
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		return s.canonicalizeTemporal(meta.Kind, normalized, needKey, metrics)
	case runtime.VAnyURI:
		return s.canonicalizeAnyURI(normalized, needKey, metrics)
	case runtime.VQName, runtime.VNotation:
		return s.canonicalizeQName(meta, normalized, resolver, needKey, metrics)
	case runtime.VHexBinary:
		return s.canonicalizeHexBinary(normalized, needKey, metrics)
	case runtime.VBase64Binary:
		return s.canonicalizeBase64Binary(normalized, needKey, metrics)
	case runtime.VList:
		return s.canonicalizeList(meta, normalized, resolver, opts, needKey, metrics)
	case runtime.VUnion:
		return s.canonicalizeUnion(meta, normalized, lexical, resolver, opts, needKey, metrics)
	default:
		return nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", meta.Kind)
	}
}

func (s *Session) validateValueNoCanonical(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString, runtime.VBoolean, runtime.VDecimal, runtime.VInteger, runtime.VFloat, runtime.VDouble, runtime.VDuration:
		if err := s.validateAtomicNoCanonical(meta, normalized); err != nil {
			return nil, err
		}
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		if err := validateTemporalNoCanonical(meta.Kind, normalized); err != nil {
			return nil, err
		}
	case runtime.VAnyURI:
		if err := validateAnyURINoCanonical(normalized); err != nil {
			return nil, err
		}
	case runtime.VHexBinary:
		if err := validateHexBinaryNoCanonical(normalized); err != nil {
			return nil, err
		}
	case runtime.VBase64Binary:
		if err := validateBase64BinaryNoCanonical(normalized); err != nil {
			return nil, err
		}
	case runtime.VList:
		if err := s.validateListNoCanonical(meta, normalized, resolver, opts); err != nil {
			return nil, err
		}
	default:
		return nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", meta.Kind)
	}
	return s.maybeStore(normalized, opts.storeValue), nil
}
