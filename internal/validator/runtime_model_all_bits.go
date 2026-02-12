package validator

func allHas(words []uint64, idx int) bool {
	if idx < 0 {
		return false
	}
	word := idx / 64
	bit := uint(idx % 64)
	if word >= len(words) {
		return false
	}
	return words[word]&(1<<bit) != 0
}

func allSet(words []uint64, idx int) {
	if idx < 0 {
		return
	}
	word := idx / 64
	bit := uint(idx % 64)
	if word >= len(words) {
		return
	}
	words[word] |= 1 << bit
}
