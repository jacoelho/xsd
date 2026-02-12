package validator

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	wsmode "github.com/jacoelho/xsd/internal/whitespace"
)

func (s *Session) validateValueCore(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions, metrics *ValueMetrics) ([]byte, error) {
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
	metricsInternal := false
	needEnumKey := meta.Flags&runtime.ValidatorHasEnum != 0
	if metrics == nil {
		needsLocalMetrics := needEnumKey
		if !needsLocalMetrics && (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary) {
			needsLocalMetrics = s.hasLengthFacet(meta)
		}
		if !needsLocalMetrics && opts.trackIDs && meta.Kind == runtime.VUnion {
			needsLocalMetrics = true
		}
		if needsLocalMetrics {
			localMetrics := &ValueMetrics{}
			metrics = localMetrics
			metricsInternal = true
		}
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
	if err := s.validateRuntimeFacets(meta, normalized, canon, metrics); err != nil {
		return nil, err
	}
	if !opts.storeValue && (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary) {
		canon = slices.Clone(canon)
	}
	canon = s.finalizeValue(canon, opts, metrics, metricsInternal)
	if opts.trackIDs {
		if err := s.trackValidatedIDs(id, canon, resolver, metrics); err != nil {
			return nil, err
		}
	}
	return canon, nil
}
