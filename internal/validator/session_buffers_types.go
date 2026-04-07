package validator

// SessionBuffers owns reusable scratch buffers for value and text processing.
type SessionBuffers struct {
	normBuf      []byte
	valueBuf     []byte
	valueScratch []byte
	normStack    [][]byte
	metricsPool  []*ValueMetrics
	errBuf       []byte
	textBuf      []byte
	keyBuf       []byte
	keyTmp       []byte
	metricsDepth int
}
