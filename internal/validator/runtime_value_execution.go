package validator

import (
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// valueExecutionPlan captures the execution work needed for one value request.
type valueExecutionPlan struct {
	NeedCanonical           bool
	NeedKey                 bool
	NeedLocalMetrics        bool
	UseScratchNormalization bool
	CloneCanonical          bool
}

func (s *Session) validateValueCore(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions, metricState *ValueMetrics) ([]byte, error) {
	meta, err := s.validatorMeta(id)
	if err != nil {
		return nil, err
	}

	plan := buildValueExecutionPlan(meta, opts, hasLengthFacet(meta, s.rt.Facets))
	metrics, metricsInternal := s.prepareValueMetrics(plan, metricState)
	normalized, finishNormalize := s.normalizeValueInput(meta, lexical, opts, plan)
	defer finishNormalize()

	if !plan.NeedCanonical {
		return s.validateValueWithoutCanonical(id, meta, normalized, resolver, opts, metrics)
	}
	return s.validateValueWithCanonical(id, meta, lexical, normalized, resolver, opts, plan, metrics, metricsInternal)
}

func hasLengthFacet(meta runtime.ValidatorMeta, facetCode []runtime.FacetInstr) bool {
	if meta.Facets.Len == 0 {
		return false
	}
	ok, err := RuntimeProgramHasOp(meta, facetCode, runtime.FLength, runtime.FMinLength, runtime.FMaxLength)
	return err == nil && ok
}

func buildValueExecutionPlan(meta runtime.ValidatorMeta, opts valueOptions, hasLengthFacet bool) valueExecutionPlan {
	needEnumKey := meta.Flags&runtime.ValidatorHasEnum != 0
	needLocalMetrics := needEnumKey
	if !needLocalMetrics && (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary) {
		needLocalMetrics = hasLengthFacet
	}
	if !needLocalMetrics && opts.TrackIDs && meta.Kind == runtime.VUnion && meta.Flags&runtime.ValidatorMayTrackIDs != 0 {
		needLocalMetrics = true
	}

	needCanonical := opts.RequireCanonical || meta.Facets.Len != 0 || meta.Kind == runtime.VUnion || meta.Kind == runtime.VQName || meta.Kind == runtime.VNotation
	if opts.StoreValue || opts.NeedKey {
		needCanonical = true
	}

	return valueExecutionPlan{
		NeedCanonical:           needCanonical,
		NeedKey:                 opts.NeedKey || opts.StoreValue || needEnumKey,
		NeedLocalMetrics:        needLocalMetrics,
		UseScratchNormalization: opts.ApplyWhitespace && (meta.Kind == runtime.VList || meta.Kind == runtime.VUnion),
		CloneCanonical:          !opts.StoreValue && (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary),
	}
}

func (s *Session) prepareValueMetrics(plan valueExecutionPlan, metricState *ValueMetrics) (*ValueMetrics, bool) {
	if metricState != nil || !plan.NeedLocalMetrics {
		return metricState, false
	}
	return &ValueMetrics{}, true
}

func (s *Session) normalizeValueInput(meta runtime.ValidatorMeta, lexical []byte, opts valueOptions, plan valueExecutionPlan) ([]byte, func()) {
	if !opts.ApplyWhitespace {
		return lexical, func() {}
	}
	mode := valueWhitespaceMode(meta.WhiteSpace)
	if !plan.UseScratchNormalization {
		normalized := value.NormalizeWhitespace(mode, lexical, s.normBuf)
		return normalized, func() {}
	}
	if !value.NeedsWhitespaceNormalization(mode, lexical) {
		return lexical, func() {}
	}
	buf := s.pushNormBuf(len(lexical))
	normalized := value.NormalizeWhitespace(mode, lexical, buf)
	return normalized, s.popNormBuf
}

func (s *Session) validateValueWithoutCanonical(id runtime.ValidatorID, meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions, metrics *ValueMetrics) ([]byte, error) {
	canon, err := s.validateValueNoCanonical(meta, normalized, resolver, opts)
	if err != nil {
		return nil, err
	}
	if err := s.trackValueIDs(id, meta, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (s *Session) validateValueWithCanonical(
	id runtime.ValidatorID,
	meta runtime.ValidatorMeta,
	lexical, normalized []byte,
	resolver value.NSResolver,
	opts valueOptions,
	plan valueExecutionPlan,
	metrics *ValueMetrics,
	metricsInternal bool,
) ([]byte, error) {
	canon, err := s.canonicalizeValueCore(meta, normalized, lexical, resolver, opts, plan.NeedKey, metrics)
	if err != nil {
		return nil, err
	}
	keyBuf, err := validateRuntimeFacets(
		meta,
		s.rt.Facets,
		s.rt.Patterns,
		s.rt.Enums,
		s.rt.Values,
		normalized,
		canon,
		metrics,
		s.keyTmp[:0],
	)
	if err != nil {
		return nil, err
	}
	s.keyTmp = keyBuf
	canon = s.finishCanonicalValue(canon, opts, plan, metrics, metricsInternal)
	if err := s.trackValueIDs(id, meta, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (s *Session) canonicalizeValueCore(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString, runtime.VBoolean, runtime.VDecimal, runtime.VInteger, runtime.VFloat, runtime.VDouble, runtime.VDuration:
		return s.canonicalizeAtomic(meta, normalized, needKey, metrics)
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		return s.canonicalizeTemporal(meta.Kind, normalized, needKey, metrics)
	case runtime.VAnyURI:
		if err := validateAnyURINoCanonical(normalized); err != nil {
			return nil, xsderrors.Invalid(err.Error())
		}
		if needKey && s != nil {
			key := runtime.StringKeyBytes(s.keyTmp[:0], 1, normalized)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKString, key, false)
		}
		return normalized, nil
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
		return nil, xsderrors.Invalidf("unsupported validator kind %d", meta.Kind)
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
			return nil, xsderrors.Invalid(err.Error())
		}
	case runtime.VAnyURI:
		if err := validateAnyURINoCanonical(normalized); err != nil {
			return nil, xsderrors.Invalid(err.Error())
		}
	case runtime.VHexBinary:
		if err := validateHexBinaryNoCanonical(normalized); err != nil {
			return nil, xsderrors.Invalid(err.Error())
		}
	case runtime.VBase64Binary:
		if err := validateBase64BinaryNoCanonical(normalized); err != nil {
			return nil, xsderrors.Invalid(err.Error())
		}
	case runtime.VList:
		if err := s.validateListNoCanonical(meta, normalized, resolver, opts); err != nil {
			return nil, err
		}
	default:
		return nil, xsderrors.Invalidf("unsupported validator kind %d", meta.Kind)
	}
	return s.maybeStore(normalized, opts.StoreValue), nil
}

func (s *Session) finishCanonicalValue(canonical []byte, opts valueOptions, plan valueExecutionPlan, metrics *ValueMetrics, metricsInternal bool) []byte {
	if plan.CloneCanonical {
		canonical = slices.Clone(canonical)
	}
	return s.finalizeValue(canonical, opts, metrics, metricsInternal)
}

func (s *Session) trackValueIDs(id runtime.ValidatorID, meta runtime.ValidatorMeta, canonical []byte, resolver value.NSResolver, opts valueOptions, metrics *ValueMetrics) error {
	if !opts.TrackIDs {
		return nil
	}
	if meta.Flags&runtime.ValidatorMayTrackIDs == 0 {
		return nil
	}
	return s.trackValidatedIDs(id, canonical, resolver, metrics)
}

func validateTemporalNoCanonical(kind runtime.ValidatorKind, normalized []byte) error {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return xsderrors.Invalidf("unsupported temporal kind %d", kind)
	}
	_, err := value.Parse(spec.Kind, normalized)
	return err
}

func validateAnyURINoCanonical(normalized []byte) error {
	return value.ValidateAnyURI(normalized)
}

func validateHexBinaryNoCanonical(normalized []byte) error {
	_, err := value.ParseHexBinary(normalized)
	return err
}

func validateBase64BinaryNoCanonical(normalized []byte) error {
	_, err := value.ParseBase64Binary(normalized)
	return err
}

func valueWhitespaceMode(mode runtime.WhitespaceMode) value.WhitespaceMode {
	switch mode {
	case runtime.WSReplace:
		return value.WhitespaceReplace
	case runtime.WSCollapse:
		return value.WhitespaceCollapse
	default:
		return value.WhitespacePreserve
	}
}
