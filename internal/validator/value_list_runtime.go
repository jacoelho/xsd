package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

type listBuffers struct {
	Value []byte
	Key   []byte
}

type listOutcome struct {
	Canonical []byte
	Key       []byte
	Count     int
	KeySet    bool
}

func listRuntimeItemValidator(meta runtime.ValidatorMeta, validators runtime.ValidatorsBundle) (runtime.ValidatorID, bool) {
	if int(meta.Index) >= len(validators.List) {
		return 0, false
	}
	return validators.List[meta.Index].Item, true
}

func canValidateCollapsedFloatListFast(itemValidator runtime.ValidatorID, validators runtime.ValidatorsBundle) bool {
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

func canonicalizeListRuntime(
	meta runtime.ValidatorMeta,
	validators runtime.ValidatorsBundle,
	normalized []byte,
	applyWhitespace bool,
	needKey bool,
	bufs listBuffers,
	validate func(itemValidator runtime.ValidatorID, item []byte, needKey bool) ([]byte, runtime.ValueKind, []byte, bool, error),
) (listOutcome, listBuffers, error) {
	itemValidator, ok := listRuntimeItemValidator(meta, validators)
	if !ok {
		return listOutcome{}, bufs, xsderrors.Invalid("list validator out of range")
	}

	canonical := bufs.Value[:0]
	count := 0

	var entryKey []byte
	if needKey {
		entryKey = make([]byte, 0, len(normalized))
	}

	spaceOnly := applyWhitespace && meta.WhiteSpace == runtime.WSCollapse
	err := forEachListItemRuntime(normalized, spaceOnly, func(item []byte) error {
		itemCanonical, keyKind, keyBytes, keySet, err := validate(itemValidator, item, needKey)
		if err != nil {
			return err
		}
		if count > 0 {
			canonical = append(canonical, ' ')
		}
		canonical = append(canonical, itemCanonical...)
		if needKey {
			if !keySet {
				return xsderrors.Invalid("list item key missing")
			}
			entryKey = runtime.AppendListEntry(entryKey, byte(keyKind), keyBytes)
		}
		count++
		return nil
	})
	if err != nil {
		return listOutcome{}, bufs, err
	}

	bufs.Value = canonical
	out := listOutcome{
		Canonical: canonical,
		Count:     count,
	}
	if needKey {
		listKey := runtime.AppendUvarint(bufs.Key[:0], uint64(count))
		listKey = append(listKey, entryKey...)
		bufs.Key = listKey
		out.Key = listKey
		out.KeySet = true
	}
	return out, bufs, nil
}

func validateListNoCanonicalRuntime(
	meta runtime.ValidatorMeta,
	validators runtime.ValidatorsBundle,
	normalized []byte,
	applyWhitespace bool,
	validate func(itemValidator runtime.ValidatorID, item []byte) error,
) error {
	itemValidator, ok := listRuntimeItemValidator(meta, validators)
	if !ok {
		return xsderrors.Invalid("list validator out of range")
	}

	spaceOnly := applyWhitespace && meta.WhiteSpace == runtime.WSCollapse
	if spaceOnly && canValidateCollapsedFloatListFast(itemValidator, validators) {
		if err := validateCollapsedFloatListRuntime(normalized, validators.Meta[itemValidator].Kind); err != nil {
			return xsderrors.Invalid(err.Error())
		}
		return nil
	}
	return forEachListItemRuntime(normalized, spaceOnly, func(item []byte) error {
		return validate(itemValidator, item)
	})
}

func forEachListItemRuntime(normalized []byte, spaceOnly bool, fn func([]byte) error) error {
	if len(normalized) == 0 {
		return nil
	}
	if spaceOnly {
		return forEachSpaceSeparatedItemRuntime(normalized, fn)
	}
	i := 0
	for i < len(normalized) {
		for i < len(normalized) && isXMLWhitespaceByteRuntime(normalized[i]) {
			i++
		}
		if i >= len(normalized) {
			return nil
		}
		start := i
		for i < len(normalized) && !isXMLWhitespaceByteRuntime(normalized[i]) {
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

func validateCollapsedFloatListRuntime(normalized []byte, kind runtime.ValidatorKind) error {
	for i := 0; i < len(normalized); {
		if normalized[i] == ' ' {
			i++
			continue
		}
		next, ok := parseCollapsedFloatItemRuntime(normalized, i)
		if !ok {
			return invalidCollapsedFloatRuntime(kind)
		}
		i = next
		if i < len(normalized) {
			i++
		}
	}
	return nil
}

func parseCollapsedFloatItemRuntime(normalized []byte, start int) (int, bool) {
	if next, ok, matched := parseCollapsedFloatSpecialRuntime(normalized, start); matched {
		return next, ok
	}
	next, ok := parseCollapsedFloatNumberRuntime(normalized, start)
	if !ok {
		return 0, false
	}
	if next < len(normalized) && normalized[next] != ' ' {
		return 0, false
	}
	return next, true
}

func parseCollapsedFloatSpecialRuntime(normalized []byte, start int) (int, bool, bool) {
	switch normalized[start] {
	case 'I':
		next, ok := matchCollapsedLiteralRuntime(normalized, start, "INF")
		return next, ok, true
	case 'N':
		next, ok := matchCollapsedLiteralRuntime(normalized, start, "NaN")
		return next, ok, true
	case '-':
		next, ok := matchCollapsedLiteralRuntime(normalized, start, "-INF")
		return next, ok, ok
	case '+':
		_, ok := matchCollapsedLiteralRuntime(normalized, start, "+INF")
		return 0, false, ok
	default:
		return 0, false, false
	}
}

func parseCollapsedFloatNumberRuntime(normalized []byte, start int) (int, bool) {
	i := start
	n := len(normalized)
	if hasFloatSignRuntime(normalized[i]) {
		i++
		if i >= n || normalized[i] == ' ' {
			return 0, false
		}
	}

	wholeStart := i
	for i < n && isDigitByteRuntime(normalized[i]) {
		i++
	}
	wholeDigits := i - wholeStart

	fractionDigits := 0
	if i < n && normalized[i] == '.' {
		i++
		fractionStart := i
		for i < n && isDigitByteRuntime(normalized[i]) {
			i++
		}
		fractionDigits = i - fractionStart
	}
	if wholeDigits == 0 && fractionDigits == 0 {
		return 0, false
	}
	if i >= n || (normalized[i] != 'e' && normalized[i] != 'E') {
		return i, true
	}

	i++
	if i >= n || normalized[i] == ' ' {
		return 0, false
	}
	if hasFloatSignRuntime(normalized[i]) {
		i++
		if i >= n || normalized[i] == ' ' {
			return 0, false
		}
	}

	exponentStart := i
	for i < n && isDigitByteRuntime(normalized[i]) {
		i++
	}
	if i == exponentStart {
		return 0, false
	}
	return i, true
}

func hasFloatSignRuntime(b byte) bool {
	return b == '+' || b == '-'
}

func forEachSpaceSeparatedItemRuntime(normalized []byte, fn func([]byte) error) error {
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

func invalidCollapsedFloatRuntime(kind runtime.ValidatorKind) error {
	if kind == runtime.VDouble {
		return invalidListErrorRuntime("invalid double")
	}
	return invalidListErrorRuntime("invalid float")
}

func matchCollapsedLiteralRuntime(normalized []byte, start int, literal string) (int, bool) {
	end := start + len(literal)
	if end > len(normalized) {
		return 0, false
	}
	for i := 0; i < len(literal); i++ {
		if normalized[start+i] != literal[i] {
			return 0, false
		}
	}
	return matchCollapsedLiteralEndRuntime(normalized, end)
}

func matchCollapsedLiteralEndRuntime(normalized []byte, end int) (int, bool) {
	if end < len(normalized) && normalized[end] != ' ' {
		return 0, false
	}
	return end, true
}

func isXMLWhitespaceByteRuntime(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func isDigitByteRuntime(b byte) bool {
	return b >= '0' && b <= '9'
}

type invalidListErrorRuntime string

func (e invalidListErrorRuntime) Error() string {
	return string(e)
}
