package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	facetengine "github.com/jacoelho/xsd/internal/schemafacet"
	"github.com/jacoelho/xsd/internal/value"
	wsmode "github.com/jacoelho/xsd/internal/whitespace"
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
			normalized = value.NormalizeWhitespace(wsmode.ToValue(meta.WhiteSpace), lexical, buf)
		} else {
			normalized = value.NormalizeWhitespace(wsmode.ToValue(meta.WhiteSpace), lexical, s.normBuf)
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
	if err := facetengine.ValidateRuntimeProgram(
		facetengine.RuntimeProgram{
			Meta:       meta,
			Facets:     s.rt.Facets,
			Patterns:   s.rt.Patterns,
			Enums:      s.rt.Enums,
			Values:     s.rt.Values,
			Normalized: normalized,
			Canonical:  canon,
		},
		facetengine.RuntimeCallbacks{
			SkipPattern: func() bool {
				return metrics != nil && metrics.patternChecked
			},
			SkipEnum: func() bool {
				return metrics != nil && metrics.enumChecked
			},
			CachedEnumKey: func() (runtime.ValueKind, []byte, bool) {
				if metrics == nil || !metrics.keySet {
					return runtime.VKInvalid, nil, false
				}
				return metrics.keyKind, metrics.keyBytes, true
			},
			DeriveEnumKey: func(canonical []byte) (runtime.ValueKind, []byte, error) {
				return s.deriveKeyFromCanonical(meta.Kind, canonical)
			},
			StoreEnumKey: func(kind runtime.ValueKind, key []byte) {
				if metrics == nil {
					return
				}
				metrics.keyKind = kind
				metrics.keyBytes = key
				metrics.keySet = true
			},
			CheckRange: func(op runtime.FacetOp, kind runtime.ValidatorKind, canonical, bound []byte) error {
				switch kind {
				case runtime.VFloat, runtime.VDouble:
					return s.checkFloatRange(kind, op, canonical, bound, metrics)
				default:
					cmp, err := s.compareValue(kind, canonical, bound, metrics)
					if err != nil {
						return err
					}
					return compareRange(op, cmp)
				}
			},
			ValueLength: func(kind runtime.ValidatorKind, normalized []byte) (int, error) {
				return s.valueLength(runtime.ValidatorMeta{Kind: kind}, normalized, metrics)
			},
			ShouldSkipLength: func(kind runtime.ValidatorKind) bool {
				return kind == runtime.VQName || kind == runtime.VNotation
			},
			DigitCounts: func(kind runtime.ValidatorKind, canonical []byte) (int, int, error) {
				return digitCounts(kind, canonical, metrics)
			},
			Invalidf: func(format string, args ...any) error {
				return valueErrorf(valueErrInvalid, format, args...)
			},
			FacetViolation: func(name string) error {
				return valueErrorf(valueErrFacet, "%s violation", name)
			},
		},
	); err != nil {
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
