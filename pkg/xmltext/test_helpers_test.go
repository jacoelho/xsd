package xmltext

type tokenReader struct {
	dec *Decoder
	tok Token
}

func newTokenReader(dec *Decoder) *tokenReader {
	return &tokenReader{dec: dec}
}

func (r *tokenReader) Next() (Token, error) {
	if r == nil || r.dec == nil {
		return Token{}, errNilReader
	}
	err := r.dec.ReadTokenInto(&r.tok)
	return r.tok, err
}
