package xmltext

import (
	"bytes"
	"errors"
	"io"
)

var whitespaceLUT = [256]bool{
	'\t': true,
	'\n': true,
	'\r': true,
	' ':  true,
}

func (d *Decoder) ensureIndex(idx int, allowCompact bool) error {
	for idx >= len(d.buf.data) {
		if d.eof {
			return io.EOF
		}
		if err := d.readMore(allowCompact); err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) readMore(allowCompact bool) error {
	if d.eof {
		return io.EOF
	}
	if allowCompact && d.pos == 0 {
		d.compactIfNeeded()
	}
	if len(d.buf.data) == cap(d.buf.data) {
		if err := d.growBuffer(); err != nil {
			return err
		}
	}
	space := cap(d.buf.data) - len(d.buf.data)
	if space == 0 {
		return io.EOF
	}
	buf := d.buf.data
	n, err := d.r.Read(buf[len(buf) : len(buf)+space])
	if n > 0 {
		d.buf.data = buf[:len(buf)+n]
		return nil
	}
	if errors.Is(err, io.EOF) {
		d.eof = true
		return io.EOF
	}
	return err
}

func (d *Decoder) compactIfNeeded() {
	if d.pos == 0 {
		return
	}
	keepIndex := d.pos
	if d.compactFloorSet {
		floorIndex := int(d.compactFloorAbs - d.baseOffset)
		if floorIndex < 0 {
			return
		}
		if floorIndex < keepIndex {
			keepIndex = floorIndex
		}
	}
	if keepIndex >= len(d.buf.data) {
		d.baseOffset += int64(keepIndex)
		d.buf.data = d.buf.data[:0]
		d.pos -= keepIndex
		return
	}
	remaining := len(d.buf.data) - keepIndex
	if remaining >= cap(d.buf.data)/4 {
		return
	}
	if cap(d.buf.data)-len(d.buf.data) >= cap(d.buf.data)/4 {
		return
	}
	if keepIndex == d.pos {
		d.compact()
		return
	}
	copy(d.buf.data, d.buf.data[keepIndex:])
	d.buf.data = d.buf.data[:len(d.buf.data)-keepIndex]
	d.baseOffset += int64(keepIndex)
	d.pos -= keepIndex
}

func (d *Decoder) setCompactFloorAbs(offset int64) {
	d.compactFloorAbs = offset
	d.compactFloorSet = true
}

func (d *Decoder) clearCompactFloor() {
	d.compactFloorSet = false
	d.compactFloorAbs = 0
}

func (d *Decoder) growBuffer() error {
	capNow := cap(d.buf.data)
	newCap := capNow * 2
	minCap := d.opts.bufferSize
	if minCap <= 0 {
		minCap = defaultBufferSize
	}
	if newCap < minCap {
		newCap = minCap
	}
	if d.opts.maxTokenSize > 0 && newCap > d.opts.maxTokenSize {
		newCap = d.opts.maxTokenSize
	}
	if newCap <= capNow {
		return errTokenTooLarge
	}
	newBuf := make([]byte, len(d.buf.data), newCap)
	copy(newBuf, d.buf.data)
	d.buf.data = newBuf
	return nil
}

func (d *Decoder) compact() {
	if d.pos == 0 {
		return
	}
	if d.pos >= len(d.buf.data) {
		d.baseOffset += int64(d.pos)
		d.buf.data = d.buf.data[:0]
		d.pos = 0
		return
	}
	copy(d.buf.data, d.buf.data[d.pos:])
	d.buf.data = d.buf.data[:len(d.buf.data)-d.pos]
	d.baseOffset += int64(d.pos)
	d.pos = 0
}

func (d *Decoder) advance(n int) {
	if n <= 0 {
		return
	}
	if d.opts.trackLineColumn {
		data := d.buf.data[d.pos : d.pos+n]
		for i := range data {
			b := data[i]
			if b == '\n' || b == '\r' {
				d.advanceWithNewlines(data)
				return
			}
		}
		d.pendingCR = false
		d.column += n
	}
	d.pos += n
}

func (d *Decoder) advanceWithNewlines(data []byte) {
	i := 0
	if d.pendingCR {
		d.pendingCR = false
		if len(data) > 0 && data[0] == '\n' {
			i = 1
		}
	}
	for ; i < len(data); i++ {
		switch data[i] {
		case '\n':
			d.line++
			d.column = 1
		case '\r':
			d.line++
			d.column = 1
			if i+1 < len(data) && data[i+1] == '\n' {
				i++
			} else if i+1 == len(data) {
				d.pendingCR = true
			}
		default:
			d.column++
		}
	}
	d.pos += len(data)
}

func (d *Decoder) advanceName(n int) {
	if n <= 0 {
		return
	}
	if d.opts.trackLineColumn {
		d.column += n
	}
	d.pos += n
}

func (d *Decoder) advanceTo(pos int) {
	d.advance(pos - d.pos)
}

func (d *Decoder) advanceRaw(n int) {
	if n <= 0 {
		return
	}
	if d.opts.trackLineColumn {
		if d.opts.debugPoisonSpans {
			end := d.pos + n
			if end > len(d.buf.data) {
				panic("xmltext: advanceRaw beyond buffer")
			}
			if bytes.ContainsAny(d.buf.data[d.pos:end], "\n\r") {
				panic("xmltext: advanceRaw consumed newline")
			}
		}
		d.pendingCR = false
		d.column += n
	}
	d.pos += n
}

func isWhitespace(b byte) bool {
	return whitespaceLUT[b]
}
