package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuekey"
)

func (s *Session) keyForCanonicalValue(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
	if s == nil || s.rt == nil || id == 0 {
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "validator missing")
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	switch meta.Kind {
	case runtime.VList:
		item, ok := s.listItemValidator(meta)
		if !ok {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "list validator out of range")
		}
		var keyBytes []byte
		count := 0
		if err := forEachListItem(canonical, func(itemValue []byte) error {
			kind, key, err := s.keyForCanonicalValue(item, itemValue, resolver, 0)
			if err != nil {
				return err
			}
			keyBytes = valuekey.AppendListEntry(keyBytes, byte(kind), key)
			count++
			return nil
		}); err != nil {
			return runtime.VKInvalid, nil, err
		}
		listKey := valuekey.AppendUvarint(s.keyTmp[:0], uint64(count))
		listKey = append(listKey, keyBytes...)
		s.keyTmp = listKey
		return runtime.VKList, listKey, nil
	case runtime.VUnion:
		if member != 0 {
			return s.keyForCanonicalValue(member, canonical, resolver, 0)
		}
		memberOpts := valueOptions{
			applyWhitespace:  true,
			trackIDs:         false,
			requireCanonical: true,
			storeValue:       false,
		}
		if _, memberMetrics, err := s.validateValueInternalWithMetrics(id, canonical, resolver, memberOpts); err == nil {
			if memberMetrics.actualValidator != 0 {
				return s.keyForCanonicalValue(memberMetrics.actualValidator, canonical, resolver, 0)
			}
		}
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "union value does not match any member type")
	default:
		return s.deriveKeyFromCanonical(meta.Kind, canonical)
	}
}
