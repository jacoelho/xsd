package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeValueCore(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valruntime.Options, needKey bool, metrics *valruntime.State) ([]byte, error) {
	return valruntime.DispatchCanonical(meta.Kind, valruntime.CanonicalCallbacks[[]byte]{
		Atomic: func() ([]byte, error) {
			return s.canonicalizeAtomic(meta, normalized, needKey, metrics)
		},
		Temporal: func() ([]byte, error) {
			return s.canonicalizeTemporal(meta.Kind, normalized, needKey, metrics)
		},
		AnyURI: func() ([]byte, error) {
			return s.canonicalizeAnyURI(normalized, needKey, metrics)
		},
		QName: func() ([]byte, error) {
			return s.canonicalizeQName(meta, normalized, resolver, needKey, metrics)
		},
		HexBinary: func() ([]byte, error) {
			return s.canonicalizeHexBinary(normalized, needKey, metrics)
		},
		Base64Binary: func() ([]byte, error) {
			return s.canonicalizeBase64Binary(normalized, needKey, metrics)
		},
		List: func() ([]byte, error) {
			return s.canonicalizeList(meta, normalized, resolver, opts, needKey, metrics)
		},
		Union: func() ([]byte, error) {
			return s.canonicalizeUnion(meta, normalized, lexical, resolver, opts, needKey, metrics)
		},
		Invalid: func(kind runtime.ValidatorKind) error {
			return diag.Invalidf("unsupported validator kind %d", kind)
		},
	})
}

func (s *Session) validateValueNoCanonical(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valruntime.Options) ([]byte, error) {
	return valruntime.DispatchNoCanonical(meta.Kind, valruntime.NoCanonicalCallbacks[[]byte]{
		Atomic: func() error {
			return s.validateAtomicNoCanonical(meta, normalized)
		},
		Temporal: func() error {
			if err := valruntime.ValidateTemporal(meta.Kind, normalized); err != nil {
				return diag.Invalid(err.Error())
			}
			return nil
		},
		AnyURI: func() error {
			if err := valruntime.ValidateAnyURI(normalized); err != nil {
				return diag.Invalid(err.Error())
			}
			return nil
		},
		HexBinary: func() error {
			if err := valruntime.ValidateHexBinary(normalized); err != nil {
				return diag.Invalid(err.Error())
			}
			return nil
		},
		Base64Binary: func() error {
			if err := valruntime.ValidateBase64Binary(normalized); err != nil {
				return diag.Invalid(err.Error())
			}
			return nil
		},
		List: func() error {
			return s.validateListNoCanonical(meta, normalized, resolver, opts)
		},
		Result: func() []byte {
			return s.maybeStore(normalized, opts.StoreValue)
		},
		Invalid: func(kind runtime.ValidatorKind) error {
			return diag.Invalidf("unsupported validator kind %d", kind)
		},
	})
}
