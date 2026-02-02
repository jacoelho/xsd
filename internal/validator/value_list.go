package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuekey"
)

func (s *Session) listItemValidator(meta runtime.ValidatorMeta) (runtime.ValidatorID, bool) {
	if int(meta.Index) >= len(s.rt.Validators.List) {
		return 0, false
	}
	return s.rt.Validators.List[meta.Index].Item, true
}

func (s *Session) canonicalizeList(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *valueMetrics) ([]byte, error) {
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
	spaceOnly := opts.applyWhitespace && meta.WhiteSpace == runtime.WS_Collapse
	_, err := forEachListItem(normalized, spaceOnly, func(item []byte) error {
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
			keyTmp = runtime.AppendListKey(keyTmp, itemMetrics.keyKind, itemMetrics.keyBytes)
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
		listKey := s.keyTmp[:0]
		listKey = valuekey.AppendUvarint(listKey, uint64(count))
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
	spaceOnly := opts.applyWhitespace && meta.WhiteSpace == runtime.WS_Collapse
	_, err := forEachListItem(normalized, spaceOnly, func(item []byte) error {
		itemOpts := opts
		itemOpts.applyWhitespace = false
		itemOpts.requireCanonical = false
		itemOpts.storeValue = false
		if _, err := s.validateValueInternalOptions(itemValidator, item, resolver, itemOpts); err != nil {
			return err
		}
		return nil
	})
	return err
}

func forEachListItem(normalized []byte, spaceOnly bool, fn func([]byte) error) (int, error) {
	if len(normalized) == 0 {
		return 0, nil
	}
	count := 0
	i := 0
	if spaceOnly {
		for i < len(normalized) {
			for i < len(normalized) && isXMLWhitespace(normalized[i]) {
				i++
			}
			if i >= len(normalized) {
				break
			}
			start := i
			for i < len(normalized) && !isXMLWhitespace(normalized[i]) {
				i++
			}
			if fn != nil {
				if err := fn(normalized[start:i]); err != nil {
					return count, err
				}
			}
			count++
		}
		return count, nil
	}
	for i < len(normalized) {
		if normalized[i] == ' ' {
			i++
			continue
		}
		start := i
		for i < len(normalized) && normalized[i] != ' ' {
			i++
		}
		if fn != nil {
			if err := fn(normalized[start:i]); err != nil {
				return count, err
			}
		}
		count++
	}
	return count, nil
}

func listItemCount(normalized []byte, spaceOnly bool) int {
	count, _ := forEachListItem(normalized, spaceOnly, nil)
	return count
}

func isXMLWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}
