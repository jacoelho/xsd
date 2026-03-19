package validator

import "github.com/jacoelho/xsd/internal/validator/valruntime"

// SessionBuffers owns reusable scratch buffers for value and text processing.
type SessionBuffers struct {
	normBuf      []byte
	valueBuf     []byte
	valueScratch []byte
	normStack    [][]byte
	metricsPool  []*valruntime.State
	errBuf       []byte
	textBuf      []byte
	keyBuf       []byte
	keyTmp       []byte
	metricsDepth int
}
