package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) trackIDs(kind runtime.StringKind, canonical []byte) error {
	switch kind {
	case runtime.StringID:
		return s.recordID(canonical)
	case runtime.StringIDREF:
		s.recordIDRef(canonical)
	case runtime.StringEntity:
		// ENTITY validation handled elsewhere
	}
	return nil
}

func (s *Session) trackValidatedIDs(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, metrics *ValueMetrics) error {
	if s == nil || s.rt == nil || id == 0 {
		return nil
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return valueErrorf(valueErrInvalid, "validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return valueErrorf(valueErrInvalid, "string validator out of range")
		}
		return s.trackIDs(kind, canonical)
	case runtime.VList:
		item, ok := s.listItemValidator(meta)
		if !ok {
			return valueErrorf(valueErrInvalid, "list validator out of range")
		}
		err := forEachListItem(canonical, true, func(itemValue []byte) error {
			return s.trackValidatedIDs(item, itemValue, resolver, nil)
		})
		return err
	case runtime.VUnion:
		if metrics != nil && metrics.actualValidator != 0 {
			return s.trackValidatedIDs(metrics.actualValidator, canonical, resolver, nil)
		}
		memberOpts := valueOptions{
			applyWhitespace:  true,
			trackIDs:         false,
			requireCanonical: true,
			storeValue:       false,
		}
		if _, memberMetrics, err := s.validateValueInternalWithMetrics(id, canonical, resolver, memberOpts); err == nil {
			if memberMetrics.actualValidator != 0 {
				return s.trackValidatedIDs(memberMetrics.actualValidator, canonical, resolver, nil)
			}
		}
		return nil
	default:
		return nil
	}
}

func (s *Session) trackDefaultValue(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, member runtime.ValidatorID) error {
	if s == nil || s.rt == nil || id == 0 {
		return nil
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return valueErrorf(valueErrInvalid, "validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return valueErrorf(valueErrInvalid, "string validator out of range")
		}
		return s.trackIDs(kind, canonical)
	case runtime.VList:
		item, ok := s.listItemValidator(meta)
		if !ok {
			return valueErrorf(valueErrInvalid, "list validator out of range")
		}
		if err := forEachListItem(canonical, true, func(itemValue []byte) error {
			return s.trackDefaultValue(item, itemValue, resolver, 0)
		}); err != nil {
			return err
		}
	case runtime.VUnion:
		if member != 0 {
			return s.trackDefaultValue(member, canonical, resolver, 0)
		}
		memberOpts := valueOptions{
			applyWhitespace:  true,
			trackIDs:         false,
			requireCanonical: true,
			storeValue:       false,
		}
		if _, memberMetrics, err := s.validateValueInternalWithMetrics(id, canonical, resolver, memberOpts); err == nil {
			if memberMetrics.actualValidator != 0 {
				return s.trackDefaultValue(memberMetrics.actualValidator, canonical, resolver, 0)
			}
		}
	}
	return nil
}
