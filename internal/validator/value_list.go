package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuecodec"
)

func (s *Session) listItemValidator(meta runtime.ValidatorMeta) (runtime.ValidatorID, bool) {
	if int(meta.Index) >= len(s.rt.Validators.List) {
		return 0, false
	}
	return s.rt.Validators.List[meta.Index].Item, true
}

func (s *Session) canonicalizeList(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	itemValidator, ok := s.listItemValidator(meta)
	if !ok {
		return nil, valueErrorf(valueErrInvalid, "list validator out of range")
	}
	count := 0
	tmp := make([]byte, 0, len(normalized))
	var keyTmp []byte
	if needKey {
		keyTmp = make([]byte, 0, len(normalized))
	}
	spaceOnly := opts.applyWhitespace && meta.WhiteSpace == runtime.WSCollapse
	err := forEachListItem(normalized, spaceOnly, func(item []byte) error {
		itemOpts := opts
		itemOpts.applyWhitespace = false
		itemOpts.requireCanonical = true
		itemOpts.storeValue = false
		itemOpts.trackIDs = false
		itemOpts.needKey = needKey
		canon, itemMetrics, err := s.validateValueInternalWithMetrics(itemValidator, item, resolver, itemOpts)
		if err != nil {
			return err
		}
		if count > 0 {
			tmp = append(tmp, ' ')
		}
		tmp = append(tmp, canon...)
		if needKey {
			if !itemMetrics.keySet {
				return valueErrorf(valueErrInvalid, "list item key missing")
			}
			keyTmp = valuecodec.AppendListEntry(keyTmp, byte(itemMetrics.keyKind), itemMetrics.keyBytes)
		}
		count++
		return nil
	})
	if err != nil {
		return nil, err
	}
	if metrics != nil {
		metrics.listCount = count
		metrics.listSet = true
	}
	canon := tmp
	if needKey {
		listKey := valuecodec.AppendUvarint(s.keyTmp[:0], uint64(count))
		listKey = append(listKey, keyTmp...)
		s.keyTmp = listKey
		s.setKey(metrics, runtime.VKList, listKey, false)
	}
	return canon, nil
}

func (s *Session) validateListNoCanonical(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions) error {
	itemValidator, ok := s.listItemValidator(meta)
	if !ok {
		return valueErrorf(valueErrInvalid, "list validator out of range")
	}
	itemOpts := opts
	itemOpts.applyWhitespace = false
	itemOpts.requireCanonical = false
	itemOpts.storeValue = false
	itemOpts.trackIDs = false
	spaceOnly := opts.applyWhitespace && meta.WhiteSpace == runtime.WSCollapse
	if spaceOnly && s.canValidateCollapsedFloatListFast(itemValidator) {
		return validateCollapsedFloatList(normalized, s.rt.Validators.Meta[itemValidator].Kind)
	}
	if spaceOnly {
		return s.validateSpaceSeparatedListItemsNoCanonical(itemValidator, normalized, resolver, itemOpts)
	}
	return s.validateWhitespaceListItemsNoCanonical(itemValidator, normalized, resolver, itemOpts)
}

func forEachListItem(normalized []byte, spaceOnly bool, fn func([]byte) error) error {
	if len(normalized) == 0 {
		return nil
	}
	if spaceOnly {
		return forEachSpaceSeparatedItem(normalized, fn)
	}
	i := 0
	for i < len(normalized) {
		for i < len(normalized) && value.IsXMLWhitespaceByte(normalized[i]) {
			i++
		}
		if i >= len(normalized) {
			return nil
		}
		start := i
		for i < len(normalized) && !value.IsXMLWhitespaceByte(normalized[i]) {
			i++
		}
		if fn != nil {
			if err := fn(normalized[start:i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func listItemCount(normalized []byte) int {
	count := 0
	_ = forEachListItem(normalized, false, func(_ []byte) error {
		count++
		return nil
	})
	return count
}

func forEachSpaceSeparatedItem(normalized []byte, fn func([]byte) error) error {
	i := 0
	for i < len(normalized) {
		for i < len(normalized) && normalized[i] == ' ' {
			i++
		}
		if i >= len(normalized) {
			return nil
		}
		start := i
		for i < len(normalized) && normalized[i] != ' ' {
			i++
		}
		if fn != nil {
			if err := fn(normalized[start:i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Session) canValidateCollapsedFloatListFast(itemValidator runtime.ValidatorID) bool {
	if int(itemValidator) >= len(s.rt.Validators.Meta) {
		return false
	}
	meta := s.rt.Validators.Meta[itemValidator]
	if meta.Kind != runtime.VFloat && meta.Kind != runtime.VDouble {
		return false
	}
	if meta.Facets.Len != 0 {
		return false
	}
	return meta.Flags&(runtime.ValidatorHasEnum|runtime.ValidatorMayTrackIDs) == 0
}

func (s *Session) validateSpaceSeparatedListItemsNoCanonical(itemValidator runtime.ValidatorID, normalized []byte, resolver value.NSResolver, opts valueOptions) error {
	i := 0
	for i < len(normalized) {
		for i < len(normalized) && normalized[i] == ' ' {
			i++
		}
		if i >= len(normalized) {
			return nil
		}
		start := i
		for i < len(normalized) && normalized[i] != ' ' {
			i++
		}
		if _, err := s.validateValueInternalOptions(itemValidator, normalized[start:i], resolver, opts); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) validateWhitespaceListItemsNoCanonical(itemValidator runtime.ValidatorID, normalized []byte, resolver value.NSResolver, opts valueOptions) error {
	i := 0
	for i < len(normalized) {
		for i < len(normalized) && value.IsXMLWhitespaceByte(normalized[i]) {
			i++
		}
		if i >= len(normalized) {
			return nil
		}
		start := i
		for i < len(normalized) && !value.IsXMLWhitespaceByte(normalized[i]) {
			i++
		}
		if _, err := s.validateValueInternalOptions(itemValidator, normalized[start:i], resolver, opts); err != nil {
			return err
		}
	}
	return nil
}

func validateCollapsedFloatList(normalized []byte, kind runtime.ValidatorKind) error {
	for i := 0; i < len(normalized); {
		if normalized[i] == ' ' {
			i++
			continue
		}
		switch normalized[i] {
		case 'I':
			if next, ok := matchCollapsedINF(normalized, i); ok {
				i = next
				if i < len(normalized) {
					i++
				}
				continue
			}
			return invalidCollapsedFloatList(kind)
		case 'N':
			if next, ok := matchCollapsedNaN(normalized, i); ok {
				i = next
				if i < len(normalized) {
					i++
				}
				continue
			}
			return invalidCollapsedFloatList(kind)
		case '-':
			if next, ok := matchCollapsedNegINF(normalized, i); ok {
				i = next
				if i < len(normalized) {
					i++
				}
				continue
			}
		case '+':
			if _, ok := matchCollapsedPosINF(normalized, i); ok {
				return invalidCollapsedFloatList(kind)
			}
		}
		startDigits := 0
		if normalized[i] == '+' || normalized[i] == '-' {
			i++
			if i >= len(normalized) || normalized[i] == ' ' {
				return invalidCollapsedFloatList(kind)
			}
		}
		for i < len(normalized) && isDigitByte(normalized[i]) {
			i++
			startDigits++
		}
		if i < len(normalized) && normalized[i] == '.' {
			i++
			fracDigits := 0
			for i < len(normalized) && isDigitByte(normalized[i]) {
				i++
				fracDigits++
			}
			if startDigits == 0 && fracDigits == 0 {
				return invalidCollapsedFloatList(kind)
			}
		} else if startDigits == 0 {
			return invalidCollapsedFloatList(kind)
		}
		if i < len(normalized) && (normalized[i] == 'e' || normalized[i] == 'E') {
			i++
			if i >= len(normalized) || normalized[i] == ' ' {
				return invalidCollapsedFloatList(kind)
			}
			if normalized[i] == '+' || normalized[i] == '-' {
				i++
				if i >= len(normalized) || normalized[i] == ' ' {
					return invalidCollapsedFloatList(kind)
				}
			}
			expDigits := 0
			for i < len(normalized) && isDigitByte(normalized[i]) {
				i++
				expDigits++
			}
			if expDigits == 0 {
				return invalidCollapsedFloatList(kind)
			}
		}
		if i < len(normalized) && normalized[i] != ' ' {
			return invalidCollapsedFloatList(kind)
		}
		if i < len(normalized) {
			i++
		}
	}
	return nil
}

func invalidCollapsedFloatList(kind runtime.ValidatorKind) error {
	if kind == runtime.VDouble {
		return valueErrorMsg(valueErrInvalid, "invalid double")
	}
	return valueErrorMsg(valueErrInvalid, "invalid float")
}

func matchCollapsedINF(normalized []byte, start int) (int, bool) {
	end := start + 3
	if end > len(normalized) {
		return 0, false
	}
	if normalized[start] != 'I' || normalized[start+1] != 'N' || normalized[start+2] != 'F' {
		return 0, false
	}
	return matchCollapsedLiteralEnd(normalized, end)
}

func matchCollapsedNaN(normalized []byte, start int) (int, bool) {
	end := start + 3
	if end > len(normalized) {
		return 0, false
	}
	if normalized[start] != 'N' || normalized[start+1] != 'a' || normalized[start+2] != 'N' {
		return 0, false
	}
	return matchCollapsedLiteralEnd(normalized, end)
}

func matchCollapsedNegINF(normalized []byte, start int) (int, bool) {
	end := start + 4
	if end > len(normalized) {
		return 0, false
	}
	if normalized[start] != '-' || normalized[start+1] != 'I' || normalized[start+2] != 'N' || normalized[start+3] != 'F' {
		return 0, false
	}
	return matchCollapsedLiteralEnd(normalized, end)
}

func matchCollapsedPosINF(normalized []byte, start int) (int, bool) {
	end := start + 4
	if end > len(normalized) {
		return 0, false
	}
	if normalized[start] != '+' || normalized[start+1] != 'I' || normalized[start+2] != 'N' || normalized[start+3] != 'F' {
		return 0, false
	}
	return matchCollapsedLiteralEnd(normalized, end)
}

func matchCollapsedLiteralEnd(normalized []byte, end int) (int, bool) {
	if end < len(normalized) && normalized[end] != ' ' {
		return 0, false
	}
	return end, true
}

func isDigitByte(b byte) bool {
	return b >= '0' && b <= '9'
}
