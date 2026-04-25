package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

type valueRequest struct {
	Lexical   []byte
	Resolver  value.NSResolver
	Options   valueOptions
	Validator runtime.ValidatorID
}

type textValueRequest struct {
	Lexical  []byte
	Resolver value.NSResolver
	Options  TextValueOptions
	Type     runtime.TypeID
}

type validatedValue struct {
	Canonical       []byte
	KeyBytes        []byte
	Metrics         ValueMetrics
	ActualValidator runtime.ValidatorID
	KeyKind         runtime.ValueKind
	HasKey          bool
}

type valueRunner struct {
	session *Session
}

type valuePlan struct {
	NeedCanonical           bool
	NeedKey                 bool
	NeedLocalMetrics        bool
	UseScratchNormalization bool
	CloneCanonical          bool
}

func newValueRunner(s *Session) valueRunner {
	return valueRunner{session: s}
}

func (s *Session) validateValue(req valueRequest) (validatedValue, error) {
	return newValueRunner(s).validate(req)
}

func (r valueRunner) validate(req valueRequest) (validatedValue, error) {
	var metrics ValueMetrics
	canonical, err := r.run(req.Validator, req.Lexical, req.Resolver, req.Options, &metrics)
	if err != nil {
		return validatedValue{}, err
	}
	out := validatedValue{Metrics: metrics}
	return finalizeValidatedValue(out, canonical), nil
}

func (r valueRunner) validateSession(req valueRequest) (validatedValue, error) {
	s := r.session
	if s == nil {
		return r.validate(req)
	}
	s.metrics = ValueMetrics{}
	canonical, err := r.run(req.Validator, req.Lexical, req.Resolver, req.Options, &s.metrics)
	if err != nil {
		return validatedValue{}, err
	}
	return finishValidatedValue(validatedValue{}, canonical, &s.metrics), nil
}

func (r valueRunner) validateText(req textValueRequest) (validatedValue, error) {
	var metrics ValueMetrics
	canonical, err := r.validateTextCore(req.Type, req.Lexical, req.Resolver, req.Options, &metrics)
	if err != nil {
		return validatedValue{}, err
	}
	out := validatedValue{Metrics: metrics}
	return finalizeValidatedValue(out, canonical), nil
}

func (r valueRunner) validateTextSession(req textValueRequest) (validatedValue, error) {
	s := r.session
	if s == nil {
		return r.validateText(req)
	}
	s.metrics = ValueMetrics{}
	canonical, err := r.validateTextCore(req.Type, req.Lexical, req.Resolver, req.Options, &s.metrics)
	if err != nil {
		return validatedValue{}, err
	}
	return finishValidatedValue(validatedValue{}, canonical, &s.metrics), nil
}

func (r valueRunner) validateTextCore(typeID runtime.TypeID, text []byte, resolver value.NSResolver, textOpts TextValueOptions, metrics *ValueMetrics) ([]byte, error) {
	s := r.session
	if s == nil || s.rt == nil {
		return nil, fmt.Errorf("session missing runtime schema")
	}
	typ, ok := s.typeByID(typeID)
	if !ok {
		return nil, fmt.Errorf("type %d not found", typeID)
	}
	storeValue := s.hasIdentityConstraints()
	opts := valueOptions{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: textOpts.RequireCanonical,
		StoreValue:       storeValue,
		NeedKey:          textOpts.NeedKey,
	}
	var validatorID runtime.ValidatorID
	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		validatorID = typ.Validator
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
			return nil, fmt.Errorf("complex type %d missing", typeID)
		}
		ct := s.rt.ComplexTypes[typ.Complex.ID]
		if ct.Content != runtime.ContentSimple {
			return nil, fmt.Errorf("type %d does not have simple content", typeID)
		}
		validatorID = ct.TextValidator
	default:
		return nil, fmt.Errorf("unknown type kind %d", typ.Kind)
	}
	if !opts.StoreValue && !opts.NeedKey {
		return r.run(validatorID, text, resolver, opts, nil)
	}
	return r.run(validatorID, text, resolver, opts, metrics)
}

func (r valueRunner) run(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions, metricState *ValueMetrics) ([]byte, error) {
	s := r.session
	meta, err := s.validatorMeta(id)
	if err != nil {
		return nil, err
	}

	plan := buildValuePlan(meta, opts, hasLengthFacet(meta, s.rt.Facets))
	metrics := metricState
	metricsInternal := false
	if metrics == nil && plan.NeedLocalMetrics {
		s.metrics = ValueMetrics{}
		metrics = &s.metrics
		metricsInternal = true
	}

	normalized, popNormalize := r.normalizeInput(meta, lexical, opts, plan)

	if !plan.NeedCanonical {
		canonical, err := r.validateWithoutCanonical(id, meta, normalized, resolver, opts, metrics)
		if popNormalize {
			s.popNormBuf()
		}
		return canonical, err
	}
	canonical, err := r.validateWithCanonical(id, meta, lexical, normalized, resolver, opts, plan, metrics, metricsInternal)
	if popNormalize {
		s.popNormBuf()
	}
	return canonical, err
}

func buildValuePlan(meta runtime.ValidatorMeta, opts valueOptions, hasLengthFacet bool) valuePlan {
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

	return valuePlan{
		NeedCanonical:           needCanonical,
		NeedKey:                 opts.NeedKey || opts.StoreValue || needEnumKey,
		NeedLocalMetrics:        needLocalMetrics,
		UseScratchNormalization: opts.ApplyWhitespace && (meta.Kind == runtime.VList || meta.Kind == runtime.VUnion),
		CloneCanonical:          !opts.StoreValue && (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary),
	}
}

func (r valueRunner) normalizeInput(meta runtime.ValidatorMeta, lexical []byte, opts valueOptions, plan valuePlan) ([]byte, bool) {
	s := r.session
	if !opts.ApplyWhitespace {
		return lexical, false
	}
	mode := valueWhitespaceMode(meta.WhiteSpace)
	if !plan.UseScratchNormalization {
		normalized := value.NormalizeWhitespace(mode, lexical, s.buffers.normBuf)
		return normalized, false
	}
	if !value.NeedsWhitespaceNormalization(mode, lexical) {
		return lexical, false
	}
	buf := s.pushNormBuf(len(lexical))
	normalized := value.NormalizeWhitespace(mode, lexical, buf)
	return normalized, true
}

func (r valueRunner) validateWithoutCanonical(id runtime.ValidatorID, meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions, metrics *ValueMetrics) ([]byte, error) {
	canon, err := r.validateNoCanonical(meta, normalized, resolver, opts)
	if err != nil {
		return nil, err
	}
	if err := r.trackIDs(id, meta, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (r valueRunner) validateWithCanonical(
	id runtime.ValidatorID,
	meta runtime.ValidatorMeta,
	lexical, normalized []byte,
	resolver value.NSResolver,
	opts valueOptions,
	plan valuePlan,
	metrics *ValueMetrics,
	metricsInternal bool,
) ([]byte, error) {
	s := r.session
	canon, err := r.canonicalize(meta, normalized, lexical, resolver, opts, plan.NeedKey, metrics)
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
		s.buffers.keyTmp[:0],
	)
	if err != nil {
		return nil, err
	}
	s.buffers.keyTmp = keyBuf
	canon = r.finishCanonical(canon, opts, plan, metrics, metricsInternal)
	if err := r.trackIDs(id, meta, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (r valueRunner) canonicalize(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	s := r.session
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
			key := runtime.StringKeyBytes(s.buffers.keyTmp[:0], 1, normalized)
			s.buffers.keyTmp = key
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

func (r valueRunner) validateNoCanonical(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions) ([]byte, error) {
	s := r.session
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

func (r valueRunner) finishCanonical(canonical []byte, opts valueOptions, plan valuePlan, metrics *ValueMetrics, metricsInternal bool) []byte {
	if plan.CloneCanonical {
		canonical = slices.Clone(canonical)
	}
	return r.session.finalizeValue(canonical, opts, metrics, metricsInternal)
}

func (r valueRunner) trackIDs(id runtime.ValidatorID, meta runtime.ValidatorMeta, canonical []byte, resolver value.NSResolver, opts valueOptions, metrics *ValueMetrics) error {
	if !opts.TrackIDs {
		return nil
	}
	if meta.Flags&runtime.ValidatorMayTrackIDs == 0 {
		return nil
	}
	return r.session.trackValidatedIDs(id, canonical, resolver, metrics)
}

func finalizeValidatedValue(out validatedValue, canonical []byte) validatedValue {
	return finishValidatedValue(out, canonical, &out.Metrics)
}

func finishValidatedValue(out validatedValue, canonical []byte, metrics *ValueMetrics) validatedValue {
	out.Canonical = canonical
	if metrics != nil {
		out.KeyKind, out.KeyBytes, out.HasKey = metrics.State.Key()
		_, out.ActualValidator = metrics.State.Actual()
	}
	return out
}
