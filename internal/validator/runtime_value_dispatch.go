package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

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
