package validator

type SessionBuffers struct {
	normBuf      []byte
	valueBuf     []byte
	valueScratch []byte
	normStack    [][]byte
	metricsPool  []*valueMetrics
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
