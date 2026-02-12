package validator

// SessionBuffers defines an exported type.
type SessionBuffers struct {
	normBuf      []byte
	valueBuf     []byte
	valueScratch []byte
	normStack    [][]byte
	metricsPool  []*ValueMetrics
	prefixCache  []prefixEntry
	nameLocal    []byte
	errBuf       []byte
	nameNS       []byte
	textBuf      []byte
	keyBuf       []byte
	keyTmp       []byte
	nsDecls      []nsDecl
	metricsDepth int
}
