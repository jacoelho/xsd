package xmltext

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

type charDataByteClass uint8

const (
	charDataByteOK charDataByteClass = iota
	charDataByteAmp
	charDataByteRightBracket
	charDataByteGreater
	charDataByteInvalid
	charDataByteNonASCII
)

var charDataByteClassLUT = func() [256]charDataByteClass {
	var lut [256]charDataByteClass
	for i := range len(lut) {
		b := byte(i)
		switch {
		case b >= utf8.RuneSelf:
			lut[i] = charDataByteNonASCII
		case b == '&':
			lut[i] = charDataByteAmp
		case b == ']':
			lut[i] = charDataByteRightBracket
		case b == '>':
			lut[i] = charDataByteGreater
		case b < 0x20 && b != 0x9 && b != 0xA && b != 0xD:
			lut[i] = charDataByteInvalid
		default:
			lut[i] = charDataByteOK
		}
	}
	return lut
}()

var plainCharDataASCIIByteLUT = func() [256]bool {
	var lut [256]bool
	for i := range len(lut) {
		class := charDataByteClassLUT[i]
		lut[i] = class == charDataByteOK || class == charDataByteGreater
	}
	return lut
}()

func scanPlainCharDataASCII(data []byte, start int) int {
	i := start
	for i+8 <= len(data) {
		b0 := data[i]
		b1 := data[i+1]
		b2 := data[i+2]
		b3 := data[i+3]
		b4 := data[i+4]
		b5 := data[i+5]
		b6 := data[i+6]
		b7 := data[i+7]
		if !plainCharDataASCIIByteLUT[b0] ||
			!plainCharDataASCIIByteLUT[b1] ||
			!plainCharDataASCIIByteLUT[b2] ||
			!plainCharDataASCIIByteLUT[b3] ||
			!plainCharDataASCIIByteLUT[b4] ||
			!plainCharDataASCIIByteLUT[b5] ||
			!plainCharDataASCIIByteLUT[b6] ||
			!plainCharDataASCIIByteLUT[b7] {
			break
		}
		i += 8
	}
	for i < len(data) && plainCharDataASCIIByteLUT[data[i]] {
		i++
	}
	return i
}

func validateCharDataRune(data []byte, i int) (int, error) {
	r, size := utf8.DecodeRune(data[i:])
	if r == utf8.RuneError && size == 1 {
		return 0, errInvalidChar
	}
	if !isValidXMLChar(r) {
		return 0, errInvalidChar
	}
	return i + size, nil
}

func unescapeIntoSpanBuffer(buf *spanBuffer, start int, data []byte, resolver *entityResolver, maxTokenSize int) ([]byte, error) {
	for {
		scratch := buf.data[start:cap(buf.data)]
		n, err := unescapeInto(scratch, data, resolver, maxTokenSize)
		if err == nil {
			end := start + n
			buf.data = buf.data[:end]
			return buf.data, nil
		}
		if !errors.Is(err, io.ErrShortBuffer) {
			return nil, err
		}
		newCap := cap(buf.data) * 2
		minCap := start + len(data)
		if maxTokenSize > 0 {
			limit := start + maxTokenSize
			if minCap > limit {
				minCap = limit
			}
		}
		if newCap < minCap {
			newCap = minCap
		}
		if newCap == 0 {
			newCap = minCap
		}
		next := make([]byte, start, newCap)
		copy(next, buf.data[:start])
		buf.data = next
	}
}

func (d *Decoder) isWhitespaceCharData(tok *rawToken) (bool, error) {
	if tok == nil {
		return true, nil
	}
	data := tok.text.bytesUnsafe()
	if len(data) == 0 {
		return true, nil
	}
	if !tok.textNeeds {
		return isWhitespaceBytes(data), nil
	}
	out, err := unescapeIntoSpanBuffer(&d.scratch, 0, data, &d.entities, d.opts.maxTokenSize)
	if err != nil {
		return false, err
	}
	return isWhitespaceBytes(out), nil
}

func (d *Decoder) scanCharDataInto(dst *rawToken, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	start := d.pos
	for {
		idx := bytes.IndexByte(d.buf.data[d.pos:], '<')
		if idx >= 0 {
			end := d.pos + idx
			if d.opts.maxTokenSize > 0 && end-start > d.opts.maxTokenSize {
				return false, errTokenTooLarge
			}
			d.advanceTo(end)
			rawSpan := makeSpan(&d.buf, start, end)
			textSpan, needs, rawNeeds, err := d.resolveText(rawSpan)
			if err != nil {
				return false, err
			}
			setCharDataToken(dst, textSpan, needs, rawNeeds, startLine, startColumn, rawSpan)
			return false, nil
		}
		if d.eof {
			end := len(d.buf.data)
			if end == start {
				return false, io.EOF
			}
			if d.opts.maxTokenSize > 0 && end-start > d.opts.maxTokenSize {
				return false, errTokenTooLarge
			}
			d.advanceTo(end)
			rawSpan := makeSpan(&d.buf, start, end)
			textSpan, needs, rawNeeds, err := d.resolveText(rawSpan)
			if err != nil {
				return false, err
			}
			setCharDataToken(dst, textSpan, needs, rawNeeds, startLine, startColumn, rawSpan)
			return false, nil
		}
		if err := d.readMore(allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				d.eof = true
				continue
			}
			return false, err
		}
	}
}

func scanCharDataSpanUntilEntity(data []byte, start int) (int, error) {
	if start < 0 {
		return -1, errInvalidChar
	}
	size := len(data)
	if start >= size {
		return -1, nil
	}
	bracketRun := 0
	for i := start; i < size; {
		if bracketRun == 0 {
			next := scanPlainCharDataASCII(data, i)
			if next > i {
				i = next
				if i >= size {
					return -1, nil
				}
			}
		}
		switch charDataByteClassLUT[data[i]] {
		case charDataByteOK:
			bracketRun = 0
			i++
		case charDataByteAmp:
			return i, nil
		case charDataByteRightBracket:
			bracketRun++
			i++
		case charDataByteGreater:
			if bracketRun >= 2 {
				return -1, errInvalidToken
			}
			bracketRun = 0
			i++
		case charDataByteInvalid:
			return -1, errInvalidChar
		case charDataByteNonASCII:
			bracketRun = 0
			next, err := validateCharDataRune(data, i)
			if err != nil {
				return -1, err
			}
			i = next
		default:
			bracketRun = 0
			i++
		}
	}
	return -1, nil
}

func unescapeCharDataInto(dst, data []byte, resolver *entityResolver, maxTokenSize int) ([]byte, bool, error) {
	rawNeeds := false
	bracketRun := 0
	start := 0
	for i := 0; i < len(data); {
		if bracketRun == 0 {
			next := scanPlainCharDataASCII(data, i)
			if next > i {
				i = next
				if i >= len(data) {
					break
				}
			}
		}
		switch charDataByteClassLUT[data[i]] {
		case charDataByteOK:
			bracketRun = 0
			i++
		case charDataByteAmp:
			if !rawNeeds {
				required := len(dst) + len(data)
				if cap(dst) < required {
					next := make([]byte, len(dst), required)
					copy(next, dst)
					dst = next
				}
			}
			rawNeeds = true
			if start < i {
				dst = append(dst, data[start:i]...)
				if maxTokenSize > 0 && len(dst) > maxTokenSize {
					return nil, rawNeeds, errTokenTooLarge
				}
			}
			consumed, replacement, r, isNumeric, err := parseEntityRef(data, i, resolver)
			if err != nil {
				return nil, rawNeeds, err
			}
			if isNumeric {
				dst = utf8.AppendRune(dst, r)
			} else {
				dst = append(dst, replacement...)
			}
			if maxTokenSize > 0 && len(dst) > maxTokenSize {
				return nil, rawNeeds, errTokenTooLarge
			}
			i += consumed
			start = i
			bracketRun = 0
		case charDataByteRightBracket:
			bracketRun++
			i++
		case charDataByteGreater:
			if bracketRun >= 2 {
				return nil, rawNeeds, errInvalidToken
			}
			bracketRun = 0
			i++
		case charDataByteInvalid:
			return nil, rawNeeds, errInvalidChar
		case charDataByteNonASCII:
			bracketRun = 0
			next, err := validateCharDataRune(data, i)
			if err != nil {
				return nil, rawNeeds, err
			}
			i = next
		default:
			bracketRun = 0
			i++
		}
	}
	if !rawNeeds {
		return dst, false, nil
	}
	if start < len(data) {
		dst = append(dst, data[start:]...)
		if maxTokenSize > 0 && len(dst) > maxTokenSize {
			return nil, rawNeeds, errTokenTooLarge
		}
	}
	return dst, rawNeeds, nil
}

func scanCharDataSpanParse(data []byte, resolver *entityResolver) (bool, error) {
	rawNeeds := false
	bracketRun := 0
	for i := 0; i < len(data); {
		if bracketRun == 0 {
			next := scanPlainCharDataASCII(data, i)
			if next > i {
				i = next
				if i >= len(data) {
					return rawNeeds, nil
				}
			}
		}
		switch charDataByteClassLUT[data[i]] {
		case charDataByteOK:
			bracketRun = 0
			i++
		case charDataByteAmp:
			rawNeeds = true
			consumed, _, _, _, err := parseEntityRef(data, i, resolver)
			if err != nil {
				return rawNeeds, err
			}
			i += consumed
			bracketRun = 0
		case charDataByteRightBracket:
			bracketRun++
			i++
		case charDataByteGreater:
			if bracketRun >= 2 {
				return rawNeeds, errInvalidToken
			}
			bracketRun = 0
			i++
		case charDataByteInvalid:
			return rawNeeds, errInvalidChar
		case charDataByteNonASCII:
			bracketRun = 0
			next, err := validateCharDataRune(data, i)
			if err != nil {
				return rawNeeds, err
			}
			i = next
		default:
			bracketRun = 0
			i++
		}
	}
	return rawNeeds, nil
}

func (d *Decoder) resolveText(textSpan span) (span, bool, bool, error) {
	data := textSpan.bytesUnsafe()
	if len(data) == 0 {
		return textSpan, false, false, nil
	}
	if !d.opts.resolveEntities {
		rawNeeds, err := scanCharDataSpanParse(data, &d.entities)
		if err != nil {
			return span{}, false, false, err
		}
		if !rawNeeds {
			return textSpan, false, false, nil
		}
		return textSpan, true, true, nil
	}
	out, rawNeeds, err := unescapeCharDataInto(d.scratch.data[:0], data, &d.entities, d.opts.maxTokenSize)
	if err != nil {
		return span{}, false, false, err
	}
	if !rawNeeds {
		return textSpan, false, false, nil
	}
	d.scratch.data = out
	return makeSpan(&d.scratch, 0, len(out)), false, true, nil
}
