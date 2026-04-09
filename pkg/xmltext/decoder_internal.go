package xmltext

import (
	"errors"
	"io"
)

func (d *Decoder) scanTokenInto(dst *rawToken, allowCompact bool) (bool, error) {
	if allowCompact {
		d.compactIfNeeded()
	}
	d.scratch.data = d.scratch.data[:0]

	if err := d.ensureIndex(d.pos, allowCompact); err != nil {
		return false, err
	}
	if d.pos >= len(d.buf.data) {
		return false, io.EOF
	}
	if d.buf.data[d.pos] != '<' {
		return d.scanCharDataInto(dst, allowCompact)
	}

	if err := d.ensureIndex(d.pos+1, allowCompact); err != nil {
		if errors.Is(err, io.EOF) {
			return false, errUnexpectedEOF
		}
		return false, err
	}

	switch d.buf.data[d.pos+1] {
	case '/':
		return d.scanEndTagInto(dst, allowCompact)
	case '?':
		return d.scanPIInto(dst, allowCompact)
	case '!':
		return d.scanBangInto(dst, allowCompact)
	default:
		return d.scanStartTagInto(dst, allowCompact)
	}
}

func setCharDataToken(dst *rawToken, text span, needs, rawNeeds bool, line, column int, raw span) {
	dst.kind = KindCharData
	dst.name = qnameSpan{}
	dst.attrs = nil
	dst.attrNeeds = nil
	dst.attrRaw = nil
	dst.attrRawNeeds = nil
	dst.text = text
	dst.textNeeds = needs
	dst.textRawNeeds = rawNeeds
	dst.line = line
	dst.column = column
	dst.raw = raw
	dst.isXMLDecl = false
}

func copyToken(dst, src *rawToken) {
	if dst == nil || src == nil {
		return
	}
	dst.kind = src.kind
	dst.name = src.name
	dst.attrs = src.attrs
	dst.attrNeeds = src.attrNeeds
	dst.attrRaw = src.attrRaw
	dst.attrRawNeeds = src.attrRawNeeds
	dst.text = src.text
	dst.textNeeds = src.textNeeds
	dst.textRawNeeds = src.textRawNeeds
	dst.line = src.line
	dst.column = src.column
	dst.raw = src.raw
	dst.isXMLDecl = src.isXMLDecl
}

func setTextToken(dst *rawToken, kind Kind, text span, line, column int, raw span, isXMLDecl bool) {
	dst.kind = kind
	dst.name = qnameSpan{}
	dst.attrs = nil
	dst.attrNeeds = nil
	dst.attrRaw = nil
	dst.attrRawNeeds = nil
	dst.text = text
	dst.textNeeds = false
	dst.textRawNeeds = false
	dst.line = line
	dst.column = column
	dst.raw = raw
	dst.isXMLDecl = isXMLDecl
}
