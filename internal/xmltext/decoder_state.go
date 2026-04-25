package xmltext

import (
	"bytes"
	"errors"
	"io"
)

type stackEntry struct {
	name       qnameSpan
	index      int64
	childCount int64
}

type attrBucket struct {
	spans []span
	gen   uint32
}

type attrSeenEntry struct {
	span span
	hash uint64
}

func (d *Decoder) bumpGen() {
	if !d.opts.debugPoisonSpans {
		return
	}
	d.buf.gen++
	d.scratch.gen++
	d.coalesce.gen++
	d.attrValueBuf.gen++
}

func (d *Decoder) fail(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		d.err = io.EOF
		return io.EOF
	}
	var syntaxErr *SyntaxError
	if errors.As(err, &syntaxErr) {
		d.err = err
		return err
	}
	syntax := d.syntaxError(err)
	d.err = syntax
	return syntax
}

func (d *Decoder) syntaxError(err error) error {
	if d == nil {
		return err
	}
	offset := d.baseOffset + int64(d.pos)
	line := 0
	column := 0
	if d.opts.trackLineColumn {
		line = d.line
		column = d.column
	}
	return &SyntaxError{
		Offset:  offset,
		Line:    line,
		Column:  column,
		Path:    d.StackPointer(),
		Snippet: d.snippet(d.pos),
		Err:     err,
	}
}

func (d *Decoder) snippet(pos int) []byte {
	const window = 32
	if pos < 0 || pos > len(d.buf.data) {
		return nil
	}
	start := max(pos-window, 0)
	end := min(pos+window, len(d.buf.data))
	if start >= end {
		return nil
	}
	out := make([]byte, end-start)
	copy(out, d.buf.data[start:end])
	return out
}

func (d *Decoder) refreshToken(tok *rawToken) {
	if tok == nil {
		return
	}
	refreshSpan(&tok.name.Full)
	refreshSpan(&tok.name.Prefix)
	refreshSpan(&tok.name.Local)
	refreshSpan(&tok.text)
	refreshSpan(&tok.raw)
	for i := range tok.attrRaw {
		refreshSpan(&tok.attrRaw[i])
	}
	for i := range tok.attrs {
		attr := &tok.attrs[i]
		refreshSpan(&attr.Name.Full)
		refreshSpan(&attr.Name.Prefix)
		refreshSpan(&attr.Name.Local)
		refreshSpan(&attr.ValueSpan)
	}
}

func refreshSpan(span *span) {
	if span == nil || span.buf == nil {
		return
	}
	span.gen = span.buf.gen
}

func (d *Decoder) nextToken(allowCompact bool) (rawToken, error) {
	var tok rawToken
	if _, err := d.nextTokenInto(&tok, allowCompact); err != nil {
		return rawToken{}, err
	}
	return tok, nil
}

func (d *Decoder) nextTokenInto(dst *rawToken, allowCompact bool) (bool, error) {
	if d.pendingTokenValid {
		copyToken(dst, &d.pendingToken)
		selfClosing := d.pendingSelfClosing
		d.pendingTokenValid = false
		d.pendingSelfClosing = false
		return selfClosing, nil
	}
	if d.pendingEnd {
		copyToken(dst, &d.pendingEndToken)
		d.pendingEnd = false
		if err := d.popStackInterned(&dst.name); err != nil {
			return false, err
		}
		return false, nil
	}
	for {
		selfClosing, err := d.scanTokenInto(dst, allowCompact)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(d.stack) > 0 {
					return false, errUnexpectedEOF
				}
				if !d.rootSeen {
					return false, errMissingRoot
				}
				return false, io.EOF
			}
			return false, err
		}
		if err := d.applyToken(dst, selfClosing); err != nil {
			return false, err
		}
		switch dst.kind {
		case KindComment:
			if !d.opts.emitComments {
				continue
			}
		case KindPI:
			if !d.opts.emitPI {
				continue
			}
		case KindDirective:
			if !d.opts.emitDirectives {
				continue
			}
		}
		return selfClosing, nil
	}
}

func (d *Decoder) applyToken(tok *rawToken, selfClosing bool) error {
	if err := d.checkTokenPlacement(tok); err != nil {
		return err
	}
	switch tok.kind {
	case KindStartElement:
		interned := d.internQName(&tok.name)
		tok.name = interned
		if err := d.pushStack(&interned); err != nil {
			return err
		}
		if selfClosing {
			d.pendingEnd = true
			d.pendingEndToken = rawToken{
				kind:   KindEndElement,
				name:   interned,
				line:   d.line,
				column: d.column,
			}
		}
	case KindEndElement:
		interned, err := d.popStackRaw(&tok.name)
		if err != nil {
			return err
		}
		tok.name = interned
		if len(d.stack) == 0 {
			d.afterRoot = true
		}
	}
	return nil
}

func (d *Decoder) checkTokenPlacement(tok *rawToken) error {
	if tok == nil {
		return nil
	}
	if tok.isXMLDecl {
		if d.xmlDeclSeen {
			return errDuplicateXMLDecl
		}
		if d.seenNonXMLDecl {
			return errMisplacedXMLDecl
		}
		d.xmlDeclSeen = true
	} else {
		d.seenNonXMLDecl = true
	}
	switch tok.kind {
	case KindDirective:
		if d.rootSeen || d.afterRoot {
			return errMisplacedDirective
		}
		if d.directiveSeen {
			return errDuplicateDirective
		}
		d.directiveSeen = true
	case KindCharData:
		if len(d.stack) == 0 {
			whitespace, err := d.isWhitespaceCharData(tok)
			if err != nil {
				return err
			}
			if !whitespace {
				return errContentOutsideRoot
			}
		}
	case KindCDATA:
		if len(d.stack) == 0 {
			return errContentOutsideRoot
		}
	case KindStartElement:
		if d.afterRoot || (d.rootSeen && len(d.stack) == 0) {
			return errMultipleRoots
		}
		if len(d.stack) == 0 {
			d.rootSeen = true
		}
	}
	return nil
}

func (d *Decoder) internQName(name *qnameSpan) qnameSpan {
	if d.interner == nil {
		d.interner = newNameInterner(d.opts.maxQNameInternEntries)
	}
	return d.interner.internQName(name)
}

func (d *Decoder) internQNameHash(name *qnameSpan, hash uint64) qnameSpan {
	if d.interner == nil {
		d.interner = newNameInterner(d.opts.maxQNameInternEntries)
	}
	return d.interner.internQNameHash(name, hash)
}

func (d *Decoder) pushStack(name *qnameSpan) error {
	if name == nil {
		return errInvalidName
	}
	if d.opts.maxDepth > 0 && len(d.stack)+1 > d.opts.maxDepth {
		return errDepthLimit
	}
	var index int64
	if len(d.stack) == 0 {
		d.rootCount++
		index = d.rootCount
	} else {
		parent := &d.stack[len(d.stack)-1]
		parent.childCount++
		index = parent.childCount
	}
	d.stack = append(d.stack, stackEntry{
		name:       *name,
		index:      index,
		childCount: 0,
	})
	return nil
}

func (d *Decoder) popStackRaw(name *qnameSpan) (qnameSpan, error) {
	if name == nil {
		return qnameSpan{}, errMismatchedEndTag
	}
	if len(d.stack) == 0 {
		return qnameSpan{}, errMismatchedEndTag
	}
	top := d.stack[len(d.stack)-1]
	if !bytes.Equal(name.Full.bytesUnsafe(), top.name.Full.bytesUnsafe()) {
		return qnameSpan{}, errMismatchedEndTag
	}
	d.stack = d.stack[:len(d.stack)-1]
	return top.name, nil
}

func (d *Decoder) popStackInterned(name *qnameSpan) error {
	if name == nil {
		return errMismatchedEndTag
	}
	if len(d.stack) == 0 {
		return errMismatchedEndTag
	}
	top := d.stack[len(d.stack)-1]
	if !bytes.Equal(name.Full.bytesUnsafe(), top.name.Full.bytesUnsafe()) {
		return errMismatchedEndTag
	}
	d.stack = d.stack[:len(d.stack)-1]
	return nil
}
