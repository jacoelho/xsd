package validator

// SessionBuffers owns reusable scratch buffers for value and text processing.
type SessionBuffers struct {
	normBuf      []byte
	valueBuf     []byte
	valueScratch []byte
	normStack    [][]byte
	errBuf       []byte
	textBuf      []byte
	keyBuf       []byte
	keyTmp       []byte
}

func (b *SessionBuffers) Reset() {
	if b == nil {
		return
	}
	b.textBuf = b.textBuf[:0]
	b.keyBuf = b.keyBuf[:0]
	b.keyTmp = b.keyTmp[:0]
	b.normBuf = b.normBuf[:0]
	b.errBuf = b.errBuf[:0]
	b.valueBuf = b.valueBuf[:0]
	b.valueScratch = b.valueScratch[:0]
}

func (b *SessionBuffers) Shrink(bufferLimit, entryLimit int) {
	if b == nil {
		return
	}
	b.textBuf = shrinkSliceCap(b.textBuf, bufferLimit)
	b.normBuf = shrinkSliceCap(b.normBuf, bufferLimit)
	b.errBuf = shrinkSliceCap(b.errBuf, bufferLimit)
	b.valueBuf = shrinkSliceCap(b.valueBuf, bufferLimit)
	b.valueScratch = shrinkSliceCap(b.valueScratch, bufferLimit)
	b.keyBuf = shrinkSliceCap(b.keyBuf, bufferLimit)
	b.keyTmp = shrinkSliceCap(b.keyTmp, bufferLimit)
	b.normStack = shrinkNormStack(b.normStack, bufferLimit, entryLimit)
}
