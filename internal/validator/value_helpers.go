package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/valuekey"
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

func (s *Session) trackValidatedIDs(id runtime.ValidatorID, canonical []byte) error {
	if s == nil || s.rt == nil || id == 0 {
		return nil
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return fmt.Errorf("validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return fmt.Errorf("string validator out of range")
		}
		return s.trackIDs(kind, canonical)
	case runtime.VList:
		item, ok := s.listItemValidator(meta)
		if !ok {
			return fmt.Errorf("list validator out of range")
		}
		_, err := forEachListItem(canonical, meta.WhiteSpace == runtime.WS_Collapse, func(itemValue []byte) error {
			return s.trackValidatedIDs(item, itemValue)
		})
		return err
	case runtime.VUnion:
		members, _, _, ok := s.unionMemberInfo(meta)
		if !ok || len(members) == 0 {
			return fmt.Errorf("union validator out of range")
		}
		memberOpts := valueOptions{
			applyWhitespace:  true,
			trackIDs:         false,
			requireCanonical: true,
			storeValue:       false,
		}
		for _, member := range members {
			if _, err := s.validateValueInternalOptions(member, canonical, nil, memberOpts); err == nil {
				return s.trackValidatedIDs(member, canonical)
			}
		}
		return nil
	default:
		return nil
	}
}

func (s *Session) trackDefaultValue(id runtime.ValidatorID, canonical []byte) error {
	if s == nil || s.rt == nil || id == 0 {
		return nil
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return fmt.Errorf("validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return fmt.Errorf("string validator out of range")
		}
		return s.trackIDs(kind, canonical)
	case runtime.VList:
		item, ok := s.listItemValidator(meta)
		if !ok {
			return fmt.Errorf("list validator out of range")
		}
		if _, err := forEachListItem(canonical, meta.WhiteSpace == runtime.WS_Collapse, func(itemValue []byte) error {
			if err := s.trackDefaultValue(item, itemValue); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	case runtime.VUnion:
		members, _, _, ok := s.unionMemberInfo(meta)
		if !ok || len(members) == 0 {
			return fmt.Errorf("union validator out of range")
		}
		for _, member := range members {
			if _, err := s.validateValueInternalNoTrack(member, canonical, nil, true); err == nil {
				return s.trackDefaultValue(member, canonical)
			}
		}
	}
	return nil
}

func (s *Session) keyForCanonicalValue(id runtime.ValidatorID, canonical []byte) (runtime.ValueKind, []byte, error) {
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
		spaceOnly := meta.WhiteSpace == runtime.WS_Collapse
		if _, err := forEachListItem(canonical, spaceOnly, func(itemValue []byte) error {
			kind, key, err := s.keyForCanonicalValue(item, itemValue)
			if err != nil {
				return err
			}
			keyBytes = runtime.AppendListKey(keyBytes, kind, key)
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
		members, _, _, ok := s.unionMemberInfo(meta)
		if !ok || len(members) == 0 {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "union validator out of range")
		}
		for _, member := range members {
			kind, key, err := s.keyForCanonicalValue(member, canonical)
			if err == nil {
				return kind, key, nil
			}
		}
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "union value does not match any member type")
	default:
		return s.deriveKeyFromCanonical(meta.Kind, canonical)
	}
}

func (s *Session) recordID(valueBytes []byte) error {
	if s == nil {
		return nil
	}
	if s.idTable == nil {
		s.idTable = make(map[string]struct{}, 32)
	}
	key := string(valueBytes)
	if _, ok := s.idTable[key]; ok {
		return newValidationError(xsderrors.ErrDuplicateID, "duplicate ID value")
	}
	s.idTable[key] = struct{}{}
	return nil
}

func (s *Session) recordIDRef(valueBytes []byte) {
	if s == nil {
		return
	}
	s.idRefs = append(s.idRefs, string(valueBytes))
}

func (s *Session) validateIDRefs() []error {
	if s == nil {
		return nil
	}
	if len(s.idRefs) == 0 {
		return nil
	}
	var errs []error
	for _, ref := range s.idRefs {
		if _, ok := s.idTable[ref]; !ok {
			errs = append(errs, newValidationError(xsderrors.ErrIDRefNotFound, "IDREF value not found"))
		}
	}
	return errs
}

func valueBytes(values runtime.ValueBlob, ref runtime.ValueRef) []byte {
	if !ref.Present {
		return nil
	}
	if ref.Len == 0 {
		return []byte{}
	}
	start := int(ref.Off)
	end := start + int(ref.Len)
	if start < 0 || end < 0 || end > len(values.Blob) {
		return nil
	}
	return values.Blob[start:end]
}
