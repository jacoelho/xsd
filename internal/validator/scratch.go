package validator

// Scratch holds reusable buffers for a single validation session.
type Scratch struct {
	Buf1 []byte
	Buf2 []byte
	Buf3 []byte
}

// Reset clears the scratch buffers for reuse.
func (s *Scratch) Reset() {
	if s == nil {
		return
	}
	s.Buf1 = s.Buf1[:0]
	s.Buf2 = s.Buf2[:0]
	s.Buf3 = s.Buf3[:0]
}
