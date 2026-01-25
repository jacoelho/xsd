package types

func isFloatLexical(value string) bool {
	if value == "" {
		return false
	}
	i := 0
	if value[i] == '+' || value[i] == '-' {
		i++
		if i == len(value) {
			return false
		}
	}
	startDigits := 0
	for i < len(value) && isDigit(value[i]) {
		i++
		startDigits++
	}
	if i < len(value) && value[i] == '.' {
		i++
		fracDigits := 0
		for i < len(value) && isDigit(value[i]) {
			i++
			fracDigits++
		}
		if startDigits == 0 && fracDigits == 0 {
			return false
		}
	} else if startDigits == 0 {
		return false
	}
	if i < len(value) && (value[i] == 'e' || value[i] == 'E') {
		i++
		if i == len(value) {
			return false
		}
		if value[i] == '+' || value[i] == '-' {
			i++
			if i == len(value) {
				return false
			}
		}
		expDigits := 0
		for i < len(value) && isDigit(value[i]) {
			i++
			expDigits++
		}
		if expDigits == 0 {
			return false
		}
	}
	return i == len(value)
}

func isDigit(value byte) bool {
	return value >= '0' && value <= '9'
}
