package xmltext

import (
	"bytes"
	"io"
)

func (d *Decoder) peekKind() (Kind, error) {
	pos := d.pos
	for {
		if err := d.peekEnsureIndex(pos); err != nil {
			return KindNone, err
		}
		if d.buf.data[pos] != '<' {
			return KindCharData, nil
		}
		if err := d.peekEnsureIndex(pos + 1); err != nil {
			return KindNone, err
		}
		switch d.buf.data[pos+1] {
		case '/':
			return KindEndElement, nil
		case '?':
			if d.opts.emitPI {
				return KindPI, nil
			}
			next, err := d.peekSkipUntil(pos+2, litPIEnd)
			if err != nil {
				return KindNone, err
			}
			pos = next
			continue
		case '!':
			if ok, err := d.peekMatchLiteral(pos, litComStart); err != nil {
				return KindNone, err
			} else if ok {
				if d.opts.emitComments {
					return KindComment, nil
				}
				next, err := d.peekSkipUntil(pos+len("<!--"), litComEnd)
				if err != nil {
					return KindNone, err
				}
				pos = next
				continue
			}
			if ok, err := d.peekMatchLiteral(pos, litCDStart); err != nil {
				return KindNone, err
			} else if ok {
				return KindCDATA, nil
			}
			if d.opts.emitDirectives {
				return KindDirective, nil
			}
			next, err := d.peekSkipDirective(pos)
			if err != nil {
				return KindNone, err
			}
			pos = next
			continue
		default:
			return KindStartElement, nil
		}
	}
}

func (d *Decoder) peekEnsureIndex(idx int) error {
	for idx >= len(d.buf.data) {
		if d.eof {
			return io.EOF
		}
		if err := d.readMore(false); err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) peekMatchLiteral(pos int, lit []byte) (bool, error) {
	end := pos + len(lit)
	for end > len(d.buf.data) {
		if err := d.readMore(false); err != nil {
			if err == io.EOF {
				return false, errUnexpectedEOF
			}
			return false, err
		}
	}
	return bytes.Equal(d.buf.data[pos:end], lit), nil
}

func (d *Decoder) peekSkipUntil(pos int, seq []byte) (int, error) {
	for {
		idx := bytes.Index(d.buf.data[pos:], seq)
		if idx >= 0 {
			return pos + idx + len(seq), nil
		}
		if d.eof {
			return 0, errUnexpectedEOF
		}
		if err := d.readMore(false); err != nil {
			if err == io.EOF {
				d.eof = true
				continue
			}
			return 0, err
		}
	}
}

func (d *Decoder) peekSkipDirective(pos int) (int, error) {
	pos += 2
	depth := 0
	quote := byte(0)
	for {
		if err := d.peekEnsureIndex(pos); err != nil {
			if err == io.EOF {
				return 0, errUnexpectedEOF
			}
			return 0, err
		}
		b := d.buf.data[pos]
		if quote != 0 {
			if b == quote {
				quote = 0
			}
			pos++
			continue
		}
		switch b {
		case '\'', '"':
			quote = b
			pos++
		case '[':
			depth++
			pos++
		case ']':
			if depth > 0 {
				depth--
			}
			pos++
		case '>':
			if depth == 0 {
				return pos + 1, nil
			}
			pos++
		default:
			pos++
		}
	}
}
