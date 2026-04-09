package xmltext

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

func (d *Decoder) scanQName(allowCompact bool) (qnameSpan, error) {
	start := d.pos
	first := true
	colonIndex := -1
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return qnameSpan{}, errUnexpectedEOF
			}
			return qnameSpan{}, err
		}
		buf := d.buf.data
		b := buf[d.pos]
		if b < utf8.RuneSelf {
			if first {
				if !nameStartByteLUT[b] {
					return qnameSpan{}, errInvalidName
				}
			} else if !nameByteLUT[b] {
				break
			}
			if b == ':' {
				if colonIndex >= 0 {
					return qnameSpan{}, errInvalidName
				}
				colonIndex = d.pos
			}
			i := d.pos + 1
			for i < len(buf) {
				b = buf[i]
				if b >= utf8.RuneSelf || !nameByteLUT[b] {
					break
				}
				if b == ':' {
					if colonIndex >= 0 {
						return qnameSpan{}, errInvalidName
					}
					colonIndex = i
				}
				i++
			}
			d.advanceName(i - d.pos)
			first = false
			if i == len(buf) {
				continue
			}
			if buf[i] < utf8.RuneSelf {
				break
			}
		}
		r, size, err := d.peekRune(allowCompact)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return qnameSpan{}, errUnexpectedEOF
			}
			return qnameSpan{}, err
		}
		if first {
			if !isNameStartRune(r) {
				return qnameSpan{}, errInvalidName
			}
		} else if !isNameRune(r) {
			break
		}
		d.advanceName(size)
		first = false
	}
	end := d.pos
	if colonIndex >= 0 {
		if colonIndex == start || colonIndex == end-1 {
			return qnameSpan{}, errInvalidName
		}
	}
	return makeQNameSpan(&d.buf, start, end, colonIndex), nil
}

func (d *Decoder) scanName(allowCompact bool) (span, error) {
	if err := d.ensureIndex(d.pos, allowCompact); err != nil {
		if errors.Is(err, io.EOF) {
			return span{}, errUnexpectedEOF
		}
		return span{}, err
	}
	start := d.pos
	b := d.buf.data[d.pos]
	if b < utf8.RuneSelf {
		if !isNameStartByte(b) {
			return span{}, errInvalidName
		}
		buf := d.buf.data
		i := d.pos + 1
		for i < len(buf) {
			b = buf[i]
			if b >= utf8.RuneSelf || !isNameByte(b) {
				break
			}
			i++
		}
		d.advanceName(i - d.pos)
		if i < len(buf) && buf[i] < utf8.RuneSelf {
			return makeSpan(&d.buf, start, d.pos), nil
		}
	} else {
		r, size, err := d.peekRune(allowCompact)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return span{}, errUnexpectedEOF
			}
			return span{}, err
		}
		if !isNameStartRune(r) {
			return span{}, errInvalidName
		}
		d.advanceName(size)
	}
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return span{}, errUnexpectedEOF
			}
			return span{}, err
		}
		b = d.buf.data[d.pos]
		if b < utf8.RuneSelf {
			if !isNameByte(b) {
				break
			}
			d.advanceName(1)
			continue
		}
		r, size, err := d.peekRune(allowCompact)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return span{}, errUnexpectedEOF
			}
			return span{}, err
		}
		if !isNameRune(r) {
			break
		}
		d.advanceName(size)
	}
	return makeSpan(&d.buf, start, d.pos), nil
}

func (d *Decoder) peekRune(allowCompact bool) (rune, int, error) {
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			return 0, 0, err
		}
		b := d.buf.data[d.pos]
		if b < utf8.RuneSelf {
			return rune(b), 1, nil
		}
		data := d.buf.data[d.pos:]
		if utf8.FullRune(data) {
			r, size := utf8.DecodeRune(data)
			if r == utf8.RuneError && size == 1 {
				return 0, 0, errInvalidChar
			}
			return r, size, nil
		}
		if d.eof {
			return 0, 0, errInvalidChar
		}
		if err := d.readMore(allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				d.eof = true
				continue
			}
			return 0, 0, err
		}
	}
}

func (d *Decoder) skipWhitespace(allowCompact bool) bool {
	consumed := false
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			return consumed
		}
		data := d.buf.data[d.pos:]
		i := 0
		for i < len(data) && isWhitespace(data[i]) {
			i++
		}
		if i == 0 {
			return consumed
		}
		consumed = true
		d.advance(i)
		if i < len(data) {
			return consumed
		}
	}
}

func (d *Decoder) expectByte(value byte, allowCompact bool) error {
	if err := d.ensureIndex(d.pos, allowCompact); err != nil {
		if errors.Is(err, io.EOF) {
			return errUnexpectedEOF
		}
		return err
	}
	if d.buf.data[d.pos] != value {
		return errInvalidToken
	}
	d.advance(1)
	return nil
}

func (d *Decoder) matchLiteral(lit []byte, allowCompact bool) (bool, error) {
	end := d.pos + len(lit)
	for end > len(d.buf.data) {
		if err := d.readMore(allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return false, errUnexpectedEOF
			}
			return false, err
		}
	}
	return bytes.Equal(d.buf.data[d.pos:end], lit), nil
}
