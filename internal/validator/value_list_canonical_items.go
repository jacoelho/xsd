package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func listItemValidator(meta runtime.ValidatorMeta, validators runtime.ValidatorsBundle) (runtime.ValidatorID, bool) {
	if int(meta.Index) >= len(validators.List) {
		return 0, false
	}
	return validators.List[meta.Index].Item, true
}

func forEachCanonicalListItem(canonical []byte, fn func([]byte) error) error {
	i := 0
	for i < len(canonical) {
		for i < len(canonical) && canonical[i] == ' ' {
			i++
		}
		if i >= len(canonical) {
			return nil
		}
		start := i
		for i < len(canonical) && canonical[i] != ' ' {
			i++
		}
		if fn != nil {
			if err := fn(canonical[start:i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func trackCanonicalList(meta runtime.ValidatorMeta, validators runtime.ValidatorsBundle, canonical []byte, track func(itemValidator runtime.ValidatorID, canonical []byte) error) error {
	itemValidator, ok := listItemValidator(meta, validators)
	if !ok {
		return xsderrors.Invalid("list validator out of range")
	}
	return forEachCanonicalListItem(canonical, func(item []byte) error {
		return track(itemValidator, item)
	})
}

func deriveCanonicalListKey(meta runtime.ValidatorMeta, validators runtime.ValidatorsBundle, canonical, dst []byte, derive func(itemValidator runtime.ValidatorID, canonical []byte) (runtime.ValueKind, []byte, error)) (runtime.ValueKind, []byte, error) {
	itemValidator, ok := listItemValidator(meta, validators)
	if !ok {
		return runtime.VKInvalid, nil, xsderrors.Invalid("list validator out of range")
	}

	entryKey := make([]byte, 0, len(canonical))
	count := 0
	if err := forEachCanonicalListItem(canonical, func(item []byte) error {
		kind, key, err := derive(itemValidator, item)
		if err != nil {
			return err
		}
		entryKey = runtime.AppendListEntry(entryKey, byte(kind), key)
		count++
		return nil
	}); err != nil {
		return runtime.VKInvalid, nil, err
	}

	listKey := runtime.AppendUvarint(dst[:0], uint64(count))
	listKey = append(listKey, entryKey...)
	return runtime.VKList, listKey, nil
}
