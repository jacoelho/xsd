package valruntime

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

// ListBuffers carries caller-owned scratch slices between list canonicalization calls.
type ListBuffers struct {
	Value []byte
	Key   []byte
}

// ListInput describes one list canonicalization request.
type ListInput struct {
	Validators      runtime.ValidatorsBundle
	Buffers         ListBuffers
	Normalized      []byte
	Meta            runtime.ValidatorMeta
	ApplyWhitespace bool
	NeedKey         bool
}

// ListNoCanonicalInput describes one list validation request on the no-canonical path.
type ListNoCanonicalInput struct {
	Validators      runtime.ValidatorsBundle
	Normalized      []byte
	Meta            runtime.ValidatorMeta
	ApplyWhitespace bool
}

// ListOutcome describes one canonicalized list result.
type ListOutcome struct {
	Canonical []byte
	Key       []byte
	Count     int
	KeySet    bool
}

// ListItemResult carries one canonicalized item key back to the list runtime.
type ListItemResult struct {
	KeyBytes []byte
	KeyKind  runtime.ValueKind
	KeySet   bool
}

// ValidateCanonicalListItem canonicalizes one list item using caller-owned runtime state.
type ValidateCanonicalListItem func(itemValidator runtime.ValidatorID, item []byte, needKey bool) ([]byte, ListItemResult, error)

// ValidateListItem validates one list item on the no-canonical path.
type ValidateListItem func(itemValidator runtime.ValidatorID, item []byte) error

// TrackCanonicalListItem processes one canonicalized list item.
type TrackCanonicalListItem func(itemValidator runtime.ValidatorID, canonical []byte) error

// DeriveCanonicalListItemKey derives one typed key for a canonicalized list item.
type DeriveCanonicalListItemKey func(itemValidator runtime.ValidatorID, canonical []byte) (runtime.ValueKind, []byte, error)

// ListItemValidator resolves the item validator for one runtime list validator.
func ListItemValidator(meta runtime.ValidatorMeta, validators runtime.ValidatorsBundle) (runtime.ValidatorID, bool) {
	if int(meta.Index) >= len(validators.List) {
		return 0, false
	}
	return validators.List[meta.Index].Item, true
}

// CanValidateCollapsedFloatListFast reports whether one list can use the collapsed
// float/double lexical fast path.
func CanValidateCollapsedFloatListFast(itemValidator runtime.ValidatorID, validators runtime.ValidatorsBundle) bool {
	if int(itemValidator) >= len(validators.Meta) {
		return false
	}
	meta := validators.Meta[itemValidator]
	if meta.Kind != runtime.VFloat && meta.Kind != runtime.VDouble {
		return false
	}
	if meta.Facets.Len != 0 {
		return false
	}
	return meta.Flags&(runtime.ValidatorHasEnum|runtime.ValidatorMayTrackIDs) == 0
}

// CanonicalizeList canonicalizes one list value using caller-provided item validation.
func CanonicalizeList(in ListInput, validate ValidateCanonicalListItem) (ListOutcome, ListBuffers, error) {
	itemValidator, ok := ListItemValidator(in.Meta, in.Validators)
	if !ok {
		return ListOutcome{}, in.Buffers, diag.Invalid("list validator out of range")
	}

	bufs := in.Buffers
	canonical := bufs.Value[:0]
	count := 0

	var entryKey []byte
	if in.NeedKey {
		entryKey = make([]byte, 0, len(in.Normalized))
	}

	spaceOnly := in.ApplyWhitespace && in.Meta.WhiteSpace == runtime.WSCollapse
	err := forEachListItem(in.Normalized, spaceOnly, func(item []byte) error {
		itemCanonical, result, err := validate(itemValidator, item, in.NeedKey)
		if err != nil {
			return err
		}
		if count > 0 {
			canonical = append(canonical, ' ')
		}
		canonical = append(canonical, itemCanonical...)
		if in.NeedKey {
			if !result.KeySet {
				return diag.Invalid("list item key missing")
			}
			entryKey = runtime.AppendListEntry(entryKey, byte(result.KeyKind), result.KeyBytes)
		}
		count++
		return nil
	})
	if err != nil {
		return ListOutcome{}, bufs, err
	}

	bufs.Value = canonical
	out := ListOutcome{
		Canonical: canonical,
		Count:     count,
	}
	if in.NeedKey {
		listKey := runtime.AppendUvarint(bufs.Key[:0], uint64(count))
		listKey = append(listKey, entryKey...)
		bufs.Key = listKey
		out.Key = listKey
		out.KeySet = true
	}
	return out, bufs, nil
}

// ValidateListNoCanonical validates one list value on the no-canonical path.
func ValidateListNoCanonical(in ListNoCanonicalInput, validate ValidateListItem) error {
	itemValidator, ok := ListItemValidator(in.Meta, in.Validators)
	if !ok {
		return diag.Invalid("list validator out of range")
	}

	spaceOnly := in.ApplyWhitespace && in.Meta.WhiteSpace == runtime.WSCollapse
	if spaceOnly && CanValidateCollapsedFloatListFast(itemValidator, in.Validators) {
		if err := ValidateCollapsedFloatList(in.Normalized, in.Validators.Meta[itemValidator].Kind); err != nil {
			return diag.Invalid(err.Error())
		}
		return nil
	}
	return forEachListItem(in.Normalized, spaceOnly, func(item []byte) error {
		return validate(itemValidator, item)
	})
}

// TrackCanonicalList walks the canonical items of one list value.
func TrackCanonicalList(meta runtime.ValidatorMeta, validators runtime.ValidatorsBundle, canonical []byte, track TrackCanonicalListItem) error {
	itemValidator, ok := ListItemValidator(meta, validators)
	if !ok {
		return diag.Invalid("list validator out of range")
	}
	return forEachListItem(canonical, true, func(item []byte) error {
		return track(itemValidator, item)
	})
}

// DeriveCanonicalListKey derives the typed runtime key for one canonicalized list value.
func DeriveCanonicalListKey(meta runtime.ValidatorMeta, validators runtime.ValidatorsBundle, canonical, dst []byte, derive DeriveCanonicalListItemKey) (runtime.ValueKind, []byte, error) {
	itemValidator, ok := ListItemValidator(meta, validators)
	if !ok {
		return runtime.VKInvalid, nil, diag.Invalid("list validator out of range")
	}

	entryKey := make([]byte, 0, len(canonical))
	count := 0
	if err := forEachListItem(canonical, true, func(item []byte) error {
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
