package xmlstream

func (r *Reader) attrValueBytes(value []byte, needsUnescape bool) ([]byte, error) {
	if !needsUnescape {
		return value, nil
	}
	var out []byte
	var err error
	r.valueBuf, out, err = decodeAttrValueBytes(r.dec, r.valueBuf, value)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Reader) namespaceValueString(value []byte, needsUnescape bool) (string, error) {
	if needsUnescape {
		var out string
		var err error
		r.nsBuf, out, err = decodeNamespaceValueString(r.dec, r.nsBuf, value)
		return out, err
	}
	var out string
	r.nsBuf, out = appendNamespaceValue(r.nsBuf, value)
	return out, nil
}

func (r *Reader) textBytes(text []byte, needsUnescape bool) ([]byte, error) {
	if !needsUnescape {
		return text, nil
	}
	var out []byte
	var err error
	r.valueBuf, out, err = decodeTextBytes(r.dec, r.valueBuf, text)
	if err != nil {
		return nil, err
	}
	return out, nil
}
