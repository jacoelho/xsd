package num

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func trimLeadingZeros(b []byte) []byte {
	i := 0
	for i < len(b) && b[i] == '0' {
		i++
	}
	return b[i:]
}

func allZeros(b []byte) bool {
	for _, c := range b {
		if c != '0' {
			return false
		}
	}
	return true
}
