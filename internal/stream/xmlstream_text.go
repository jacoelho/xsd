package stream

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/lex"
)

func validXMLPrefix(data []byte) (int, error) {
	for i := 0; i < len(data); {
		size, err := validXMLRunePrefix(data[i:])
		if err != nil {
			return i, err
		}
		if size == 0 {
			return i, nil
		}
		i += size
	}
	return len(data), nil
}

func validXMLRunePrefix(data []byte) (int, error) {
	if data[0] < utf8.RuneSelf {
		if !lex.IsXMLChar(rune(data[0])) {
			return 0, fmt.Errorf("invalid XML character")
		}
		return 1, nil
	}
	if !utf8.FullRune(data) {
		return 0, nil
	}
	r, size := utf8.DecodeRune(data)
	if r == utf8.RuneError && size == 1 {
		return 0, fmt.Errorf("invalid UTF-8")
	}
	if !lex.IsXMLChar(r) {
		return 0, fmt.Errorf("invalid XML character")
	}
	return size, nil
}

func (p *Parser) consumeLineFeed() error {
	b, err := p.br.readByte()
	if err != nil {
		if IsOnlyEOF(err) {
			return nil
		}
		return err
	}
	if b != '\n' {
		p.br.unreadByte()
	}
	return nil
}

func (p *Parser) readEntity(dst *[]byte) error {
	p.entityBuf = p.entityBuf[:0]
	defer func() {
		p.entityBuf = p.entityBuf[:0]
	}()
	for {
		b, err := p.br.readByte()
		if err != nil {
			return streamSyntaxError("unexpected EOF in entity reference", err)
		}
		if b == ';' {
			break
		}
		if len(p.entityBuf) == maxEntityReferenceLength {
			return fmt.Errorf("invalid character entity")
		}
		if err := p.checkRetainedPeak(len(p.entityBuf) + 1); err != nil {
			return err
		}
		p.entityBuf = append(p.entityBuf, b)
	}
	return p.appendEntityReference(dst)
}

func (p *Parser) appendEntityReference(dst *[]byte) error {
	switch {
	case bytes.Equal(p.entityBuf, entityLT):
		return p.appendEntityBytes(dst, []byte{'<'})
	case bytes.Equal(p.entityBuf, entityGT):
		return p.appendEntityBytes(dst, []byte{'>'})
	case bytes.Equal(p.entityBuf, entityAMP):
		return p.appendEntityBytes(dst, []byte{'&'})
	case bytes.Equal(p.entityBuf, entityAPOS):
		return p.appendEntityBytes(dst, []byte{'\''})
	case bytes.Equal(p.entityBuf, entityQUOT):
		return p.appendEntityBytes(dst, []byte{'"'})
	default:
		return p.appendOtherEntityReference(dst)
	}
}

func (p *Parser) appendOtherEntityReference(dst *[]byte) error {
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
	return p.appendEntityBytes(dst, buf[:n])
}

func (p *Parser) appendEntityBytes(dst *[]byte, data []byte) error {
	if err := p.checkRetainedPeak(len(p.entityBuf) + len(data)); err != nil {
		return err
	}
	if p.maxTokenBytes > 0 {
		p.retainedBytes += int64(len(data))
	}
	*dst = append(*dst, data...)
	return nil
}

func parseCharRef(s []byte) (rune, bool) {
	s, base, ok := charRefDigits(s)
	if !ok {
		return 0, false
	}
	var v uint64
	for _, b := range s {
		d, ok := charRefDigit(b, base)
		if !ok {
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

func charRefDigits(s []byte) ([]byte, byte, bool) {
	if len(s) == 0 {
		return nil, 0, false
	}
	if s[0] != 'x' {
		return s, 10, true
	}
	if len(s) == 1 {
		return nil, 0, false
	}
	return s[1:], 16, true
}

func charRefDigit(b, base byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		d := b - '0'
		return d, d < base
	case base == 16 && b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case base == 16 && b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}

func (p *Parser) readPastSpace() (byte, bool, error) {
	hadSpace := false
	for {
		b, err := p.br.readByte()
		if err != nil {
			return 0, hadSpace, streamSyntaxError("unexpected EOF in XML tag", err)
		}
		if !lex.IsXMLWhitespaceByte(b) {
			return b, hadSpace, nil
		}
		hadSpace = true
	}
}

func (p *Parser) appendTokenByte(dst *[]byte, b byte) error {
	if p.maxTokenBytes <= 0 {
		*dst = append(*dst, b)
		return nil
	}
	if err := p.reserveRetainedBytes(1); err != nil {
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
	if err := p.reserveRetainedBytes(len(data)); err != nil {
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

func (p *Parser) reserveRetainedBytes(n int) error {
	next := p.retainedBytes + int64(n)
	if next < p.retainedBytes {
		return errXMLTokenLimit
	}
	if err := p.checkTokenBytes(next); err != nil {
		return err
	}
	p.retainedBytes = next
	return nil
}

func (p *Parser) checkRetainedPeak(transient int) error {
	if p.maxTokenBytes <= 0 {
		return nil
	}
	peak := p.retainedBytes + int64(transient)
	if peak < p.retainedBytes {
		return errXMLTokenLimit
	}
	return p.checkTokenBytes(peak)
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
			return streamSyntaxError("unexpected EOF", err)
		}
		if b != s[i] {
			return fmt.Errorf("invalid markup declaration")
		}
	}
	return nil
}

func streamSyntaxError(msg string, err error) error {
	if IsOnlyEOF(err) {
		return errors.New(msg)
	}
	return err
}
