package xmltext

import (
	"bytes"
	"errors"
	"io"
)

// Value holds a raw XML value returned by ReadValue.
type Value []byte

// Clone returns a copy of the value bytes.
func (v Value) Clone() Value {
	if len(v) == 0 {
		return nil
	}
	clone := make([]byte, len(v))
	copy(clone, v)
	return clone
}

// IsValid reports whether the value is well-formed with the given options.
func (v Value) IsValid(opts ...Options) bool {
	dec := NewDecoder(bytes.NewReader(v), opts...)
	for {
		_, err := dec.ReadToken()
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return true
		}
		return false
	}
}
