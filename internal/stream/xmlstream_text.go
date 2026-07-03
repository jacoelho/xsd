package stream

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/lex"
)

func validXMLPrefix(data []byte) (int, error) {
	for i := 0; i < len(data); {
		if data[i] < utf8.RuneSelf {
			if !lex.IsXMLChar(rune(data[i])) {
				return i, fmt.Errorf("invalid XML character")
			}
			i++
			continue
		}
		if !utf8.FullRune(data[i:]) {
			return i, nil
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return i, fmt.Errorf("invalid UTF-8")
		}
		if !lex.IsXMLChar(r) {
			return i, fmt.Errorf("invalid XML character")
		}
		i += size
	}
	return len(data), nil
}

func (p *Parser) consumeLineFeed() {
	b, err := p.br.readByte()
	if err != nil {
		return
	}
	if b != '\n' {
		p.br.unreadByte()
	}
}

func (p *Parser) readEntity(dst *[]byte) error {
	p.entityBuf = p.entityBuf[:0]
	for {
		b, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF in entity reference", err)
		}
		if b == ';' {
			break
		}
		if err := p.appendTokenByte(&p.entityBuf, b); err != nil {
			return err
		}
		if len(p.entityBuf) > maxEntityReferenceLength {
			return fmt.Errorf("invalid character entity")
		}
	}
	switch {
	case bytes.Equal(p.entityBuf, entityLT):
		return p.appendTokenByte(dst, '<')
	case bytes.Equal(p.entityBuf, entityGT):
		return p.appendTokenByte(dst, '>')
	case bytes.Equal(p.entityBuf, entityAMP):
		return p.appendTokenByte(dst, '&')
	case bytes.Equal(p.entityBuf, entityAPOS):
		return p.appendTokenByte(dst, '\'')
	case bytes.Equal(p.entityBuf, entityQUOT):
		return p.appendTokenByte(dst, '"')
	default:
		if len(p.entityBuf) == 0 || p.entityBuf[0] != '#' {
			if lex.IsXMLNameBytes(p.entityBuf) {
				return errUnsupportedEntityReference
			}
			return fmt.Errorf("invalid character entity")
		}
		r, ok := parseCharRef(p.entityBuf[1:])
		if !ok {
			return fmt.Errorf("invalid character entity")
		}
		var buf [utf8.UTFMax]byte
		n := utf8.EncodeRune(buf[:], r)
		return p.appendTokenBytes(dst, buf[:n])
	}
}

func parseCharRef(s []byte) (rune, bool) {
	if len(s) == 0 {
		return 0, false
	}
	base := 10
	if s[0] == 'x' {
		base = 16
		s = s[1:]
		if len(s) == 0 {
			return 0, false
		}
	}
	var v uint64
	for _, b := range s {
		var d byte
		switch {
		case b >= '0' && b <= '9':
			d = b - '0'
		case base == 16 && b >= 'a' && b <= 'f':
			d = b - 'a' + 10
		case base == 16 && b >= 'A' && b <= 'F':
			d = b - 'A' + 10
		default:
			return 0, false
		}
		if int(d) >= base {
			return 0, false
		}
		v = v*uint64(base) + uint64(d)
		if v > utf8.MaxRune {
			return 0, false
		}
	}
	r := rune(v)
	if !utf8.ValidRune(r) || !lex.IsXMLChar(r) {
		return 0, false
	}
	return r, true
}

func (p *Parser) readPastSpace() (byte, bool, error) {
	hadSpace := false
	for {
		b, err := p.br.readByte()
		if err != nil {
			return 0, hadSpace, err
		}
		if !lex.IsXMLWhitespaceByte(b) {
			return b, hadSpace, nil
		}
		hadSpace = true
	}
}

func (p *Parser) readUntil(term string, dst []byte) ([]byte, error) {
	if p.maxTokenBytes <= 0 {
		return p.readUntilNoLimit(term, dst)
	}
	prefix := termPrefix(term)
	matched := 0
	for {
		b, err := p.br.readByte()
		if err != nil {
			return nil, p.syntaxError("unexpected EOF", err)
		}
		dst = append(dst, b)
		matched = advanceTermMatch(term, prefix, matched, b)
		if matched == len(term) {
			dst = dst[:len(dst)-len(term)]
			if err := p.checkTokenBytes(int64(len(dst))); err != nil {
				return nil, err
			}
			return dst, nil
		}
		if err := p.checkTokenBytes(int64(len(dst) - matched)); err != nil {
			return nil, err
		}
	}
}

func (p *Parser) readUntilNoLimit(term string, dst []byte) ([]byte, error) {
	prefix := termPrefix(term)
	matched := 0
	for {
		b, err := p.br.readByte()
		if err != nil {
			return nil, p.syntaxError("unexpected EOF", err)
		}
		dst = append(dst, b)
		matched = advanceTermMatch(term, prefix, matched, b)
		if matched == len(term) {
			return dst[:len(dst)-len(term)], nil
		}
	}
}

func (p *Parser) appendTokenByte(dst *[]byte, b byte) error {
	if p.maxTokenBytes <= 0 {
		*dst = append(*dst, b)
		return nil
	}
	if err := p.checkTokenBytes(int64(len(*dst) + 1)); err != nil {
		return err
	}
	*dst = append(*dst, b)
	return nil
}

func (p *Parser) appendTokenBytes(dst *[]byte, data []byte) error {
	if p.maxTokenBytes <= 0 {
		*dst = append(*dst, data...)
		return nil
	}
	if err := p.checkTokenBytes(int64(len(*dst) + len(data))); err != nil {
		return err
	}
	*dst = append(*dst, data...)
	return nil
}

func (p *Parser) checkTokenBytes(n int64) error {
	if p.maxTokenBytes > 0 && n > p.maxTokenBytes {
		return errXMLTokenLimit
	}
	return nil
}

func termPrefix(term string) []int {
	prefix := make([]int, len(term))
	for i, j := 1, 0; i < len(term); i++ {
		for j > 0 && term[i] != term[j] {
			j = prefix[j-1]
		}
		if term[i] == term[j] {
			j++
			prefix[i] = j
		}
	}
	return prefix
}

func advanceTermMatch(term string, prefix []int, matched int, b byte) int {
	for matched > 0 && b != term[matched] {
		matched = prefix[matched-1]
	}
	if b == term[matched] {
		matched++
	}
	return matched
}

func (p *Parser) expectString(s string) error {
	for i := range len(s) {
		b, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF", err)
		}
		if b != s[i] {
			return fmt.Errorf("invalid markup declaration")
		}
	}
	return nil
}

func (p *Parser) syntaxError(msg string, err error) error {
	if errors.Is(err, io.EOF) {
		return errors.New(msg)
	}
	return err
}
