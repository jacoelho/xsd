package validator

// Arena provides stable byte storage for a single validation session.
type Arena struct {
	buf []byte
	off int

	OverflowCount int
	Max           int
}

// Alloc returns stable storage for the arena lifetime.
func (a *Arena) Alloc(n int) []byte {
	if n <= 0 {
		return nil
	}
	if a == nil {
		return make([]byte, n)
	}
	if a.Max > 0 && a.off+n > a.Max {
		a.OverflowCount++
		return make([]byte, n)
	}
	if a.off+n > cap(a.buf) {
		newCap := max(cap(a.buf)*2, a.off+n)
		buf := make([]byte, a.off+n, newCap)
		copy(buf, a.buf[:a.off])
		a.buf = buf
	} else if a.off+n > len(a.buf) {
		a.buf = a.buf[:a.off+n]
	}
	out := a.buf[a.off : a.off+n]
	a.off += n
	return out
}

// Reset clears the arena for reuse.
func (a *Arena) Reset() {
	if a == nil {
		return
	}
	a.off = 0
}
