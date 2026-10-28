package xmlstream

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/lex"
)

// Name byte classes let one pass find a name's terminator and judge its ASCII
// QName syntax. Bytes at or above utf8.RuneSelf have no class.
const (
	nameByteTerminator uint8 = 1 << iota
	nameByteColon
	nameByteChar
	nameByteStart
)

// nameByteClasses is derived from the lex name predicates, so the name rules
// keep one owner.
var nameByteClasses = func() (classes [256]uint8) {
	for i := range classes {
		b := byte(i)
		switch {
		case lex.IsNameTerminator(b):
			classes[i] = nameByteTerminator
		case b == ':':
			classes[i] = nameByteColon
		case lex.IsASCIINCNameStart(b):
			classes[i] = nameByteStart | nameByteChar
		case lex.IsASCIINCNameChar(b):
			classes[i] = nameByteChar
		}
	}
	return classes
}()

// qnameScan carries the ASCII QName verdict of a name read in chunks. The
// verdict is consulted only at the terminator, so token-limit and read errors
// before it keep precedence over invalid syntax.
type qnameScan struct {
	// colon is the name index of the last colon; zero means none, because a
	// leading colon already fails the start-byte rule. A second colon sets
	// bad, so a valid name's colon is its only one.
	colon int
	// need is the class the next byte must carry: start after a colon, char
	// otherwise.
	need uint8
	// bad is nonzero once any byte lacked its required class or a second colon
	// appeared.
	bad uint8
	// high is the OR of every name byte; its top bit marks non-ASCII input,
	// which the Unicode split judges instead.
	high byte
}

// startQNameScan classifies the first name byte, which the caller has already
// consumed and which counts as a name byte even when it is a terminator.
func startQNameScan(first byte) qnameScan {
	return qnameScan{
		need: nameByteChar,
		bad:  nameByteStart &^ nameByteClasses[first],
		high: first,
	}
}

// scan classifies chunk up to its first name terminator and returns the number
// of name bytes. base is the name index of chunk[0].
func (s *qnameScan) scan(chunk []byte, base int) int {
	need, bad, high := s.need, s.bad, s.high
	n := len(chunk)
	for i, b := range chunk {
		class := nameByteClasses[b]
		if class&(nameByteTerminator|nameByteColon) != 0 {
			if class&nameByteTerminator != 0 {
				n = i
				break
			}
			if s.colon != 0 {
				bad |= nameByteColon
			}
			s.colon = base + i
			need = nameByteStart
			continue
		}
		bad |= need &^ class
		need = nameByteChar
		high |= b
	}
	s.need, s.bad, s.high = need, bad, high
	return n
}

// split returns the QName parts of name, the bytes this scan classified.
func (s *qnameScan) split(name []byte) (prefix, local []byte, valid bool) {
	if s.high >= utf8.RuneSelf {
		return splitUnicodeQName(name)
	}
	if s.bad != 0 || s.need == nameByteStart {
		return nil, nil, false
	}
	if s.colon == 0 {
		return nil, name, true
	}
	return name[:s.colon], name[s.colon+1:], true
}

func (p *parser) readName(first byte) (xml.Name, error) {
	p.nameBuf = p.nameBuf[:0]
	if err := p.appendTokenByte(&p.nameBuf, first); err != nil {
		return xml.Name{}, err
	}
	scan := startQNameScan(first)
	chunk, err := p.br.buffered()
	for err == nil {
		n := scan.scan(chunk, len(p.nameBuf))
		if n > 0 {
			if appendErr := p.appendTokenBytes(&p.nameBuf, chunk[:n]); appendErr != nil {
				return xml.Name{}, appendErr
			}
			p.br.consumeBuffered(n)
		}
		if n < len(chunk) {
			return p.internQName(p.nameBuf, scan)
		}
		chunk, err = p.br.buffered()
	}
	return xml.Name{}, xmlNameReadError(err)
}

func xmlNameReadError(err error) error {
	if IsOnlyEOF(err) {
		return fmt.Errorf("unexpected EOF in XML name")
	}
	return err
}

func (p *parser) internQName(name []byte, scan qnameScan) (xml.Name, error) {
	prefix, local, ok := scan.split(name)
	if !ok {
		return xml.Name{}, fmt.Errorf("invalid XML qualified name")
	}
	return p.internQNameParts(prefix, local), nil
}

func splitUnicodeQName(name []byte) (prefix, local []byte, valid bool) {
	colon := bytes.IndexByte(name, ':')
	if colon < 0 {
		return nil, name, lex.IsNCNameBytes(name)
	}
	if colon == 0 || colon == len(name)-1 || bytes.IndexByte(name[colon+1:], ':') >= 0 {
		return nil, nil, false
	}
	prefix, local = name[:colon], name[colon+1:]
	return prefix, local, lex.IsNCNameBytes(prefix) && lex.IsNCNameBytes(local)
}

func (p *parser) internQNameParts(prefix, local []byte) xml.Name {
	if prefix == nil {
		return xml.Name{Local: p.names.Intern(local)}
	}
	return xml.Name{
		Space: p.names.Intern(prefix),
		Local: p.names.Intern(local),
	}
}
