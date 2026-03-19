package valruntime

import "github.com/jacoelho/xsd/internal/runtime"

// forEachListItem visits each list item in normalized list content.
func forEachListItem(normalized []byte, spaceOnly bool, fn func([]byte) error) error {
	if len(normalized) == 0 {
		return nil
	}
	if spaceOnly {
		return forEachSpaceSeparatedItem(normalized, fn)
	}
	i := 0
	for i < len(normalized) {
		for i < len(normalized) && isXMLWhitespaceByte(normalized[i]) {
			i++
		}
		if i >= len(normalized) {
			return nil
		}
		start := i
		for i < len(normalized) && !isXMLWhitespaceByte(normalized[i]) {
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

// CountListItems reports how many items appear in normalized list content.
func CountListItems(normalized []byte) int {
	count := 0
	_ = forEachListItem(normalized, false, func(_ []byte) error {
		count++
		return nil
	})
	return count
}

// ValidateCollapsedFloatList validates collapsed list content containing xs:float or xs:double values.
func ValidateCollapsedFloatList(normalized []byte, kind runtime.ValidatorKind) error {
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
			return invalidCollapsedFloat(kind)
		case 'N':
			if next, ok := matchCollapsedNaN(normalized, i); ok {
				i = next
				if i < len(normalized) {
					i++
				}
				continue
			}
			return invalidCollapsedFloat(kind)
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
				return invalidCollapsedFloat(kind)
			}
		}
		startDigits := 0
		if normalized[i] == '+' || normalized[i] == '-' {
			i++
			if i >= len(normalized) || normalized[i] == ' ' {
				return invalidCollapsedFloat(kind)
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
				return invalidCollapsedFloat(kind)
			}
		} else if startDigits == 0 {
			return invalidCollapsedFloat(kind)
		}
		if i < len(normalized) && (normalized[i] == 'e' || normalized[i] == 'E') {
			i++
			if i >= len(normalized) || normalized[i] == ' ' {
				return invalidCollapsedFloat(kind)
			}
			if normalized[i] == '+' || normalized[i] == '-' {
				i++
				if i >= len(normalized) || normalized[i] == ' ' {
					return invalidCollapsedFloat(kind)
				}
			}
			expDigits := 0
			for i < len(normalized) && isDigitByte(normalized[i]) {
				i++
				expDigits++
			}
			if expDigits == 0 {
				return invalidCollapsedFloat(kind)
			}
		}
		if i < len(normalized) && normalized[i] != ' ' {
			return invalidCollapsedFloat(kind)
		}
		if i < len(normalized) {
			i++
		}
	}
	return nil
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

func invalidCollapsedFloat(kind runtime.ValidatorKind) error {
	if kind == runtime.VDouble {
		return invalidListError("invalid double")
	}
	return invalidListError("invalid float")
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

func isXMLWhitespaceByte(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func isDigitByte(b byte) bool {
	return b >= '0' && b <= '9'
}

type invalidListError string

func (e invalidListError) Error() string {
	return string(e)
}
