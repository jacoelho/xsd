package validator

import (
	"fmt"
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

type valueEngine struct {
	session *Session
}

// TextValueOptions controls canonicalization and key-derivation behavior for text validation.
type TextValueOptions struct {
	RequireCanonical bool
	NeedKey          bool
}

// valueExecutionPlan captures the execution work needed for one value request.
type valueExecutionPlan struct {
	NeedCanonical           bool
	NeedKey                 bool
	NeedLocalMetrics        bool
	UseScratchNormalization bool
	CloneCanonical          bool
}

func newValueEngine(s *Session) valueEngine {
	return valueEngine{session: s}
}

// ValidateTextValue validates simple-content text and returns canonical bytes plus value metrics.
func (s *Session) ValidateTextValue(typeID runtime.TypeID, text []byte, resolver value.NSResolver, textOpts TextValueOptions) ([]byte, ValueMetrics, error) {
	return newValueEngine(s).validateText(typeID, text, resolver, textOpts)
}

func (s *Session) validateValueCore(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions, metricState *ValueMetrics) ([]byte, error) {
	return newValueEngine(s).validate(id, lexical, resolver, opts, metricState)
}

func (e valueEngine) validateText(typeID runtime.TypeID, text []byte, resolver value.NSResolver, textOpts TextValueOptions) ([]byte, ValueMetrics, error) {
	metrics := e.session.acquireMetricsState()
	defer e.session.releaseMetricsState()
	canon, err := e.validateTextCore(typeID, text, resolver, textOpts, metrics)
	if err != nil {
		return nil, ValueMetrics{}, err
	}
	return canon, *metrics, nil
}

func (e valueEngine) validateTextCore(typeID runtime.TypeID, text []byte, resolver value.NSResolver, textOpts TextValueOptions, metrics *ValueMetrics) ([]byte, error) {
	s := e.session
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
		return e.validate(validatorID, text, resolver, opts, nil)
	}
	return e.validate(validatorID, text, resolver, opts, metrics)
}

func (e valueEngine) validate(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions, metricState *ValueMetrics) ([]byte, error) {
	s := e.session
	meta, err := s.validatorMeta(id)
	if err != nil {
		return nil, err
	}

	plan := buildValueExecutionPlan(meta, opts, hasLengthFacet(meta, s.rt.Facets))
	metrics, metricsInternal := e.prepareMetrics(plan, metricState)
	normalized, finishNormalize := e.normalizeInput(meta, lexical, opts, plan)
	defer finishNormalize()

	if !plan.NeedCanonical {
		return e.validateWithoutCanonical(id, meta, normalized, resolver, opts, metrics)
	}
	return e.validateWithCanonical(id, meta, lexical, normalized, resolver, opts, plan, metrics, metricsInternal)
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

func (e valueEngine) prepareMetrics(plan valueExecutionPlan, metricState *ValueMetrics) (*ValueMetrics, bool) {
	if metricState != nil || !plan.NeedLocalMetrics {
		return metricState, false
	}
	return &ValueMetrics{}, true
}

func (e valueEngine) normalizeInput(meta runtime.ValidatorMeta, lexical []byte, opts valueOptions, plan valueExecutionPlan) ([]byte, func()) {
	s := e.session
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

func (e valueEngine) validateWithoutCanonical(id runtime.ValidatorID, meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions, metrics *ValueMetrics) ([]byte, error) {
	canon, err := e.validateNoCanonical(meta, normalized, resolver, opts)
	if err != nil {
		return nil, err
	}
	if err := e.trackIDs(id, meta, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (e valueEngine) validateWithCanonical(
	id runtime.ValidatorID,
	meta runtime.ValidatorMeta,
	lexical, normalized []byte,
	resolver value.NSResolver,
	opts valueOptions,
	plan valueExecutionPlan,
	metrics *ValueMetrics,
	metricsInternal bool,
) ([]byte, error) {
	s := e.session
	canon, err := e.canonicalize(meta, normalized, lexical, resolver, opts, plan.NeedKey, metrics)
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
	canon = e.finishCanonical(canon, opts, plan, metrics, metricsInternal)
	if err := e.trackIDs(id, meta, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (e valueEngine) canonicalize(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	s := e.session
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

func (e valueEngine) validateNoCanonical(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions) ([]byte, error) {
	s := e.session
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

func (e valueEngine) finishCanonical(canonical []byte, opts valueOptions, plan valueExecutionPlan, metrics *ValueMetrics, metricsInternal bool) []byte {
	s := e.session
	if plan.CloneCanonical {
		canonical = slices.Clone(canonical)
	}
	return s.finalizeValue(canonical, opts, metrics, metricsInternal)
}

func (e valueEngine) trackIDs(id runtime.ValidatorID, meta runtime.ValidatorMeta, canonical []byte, resolver value.NSResolver, opts valueOptions, metrics *ValueMetrics) error {
	s := e.session
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
