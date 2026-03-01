package validator

func checkedSpan(off, ln uint32, size int) (start, end int, ok bool) {
	if size < 0 {
		return 0, 0, false
	}
	size64 := uint64(size)
	off64 := uint64(off)
	end64 := off64 + uint64(ln)
	if off64 > size64 || end64 > size64 {
		return 0, 0, false
	}
	return int(off64), int(end64), true
}
