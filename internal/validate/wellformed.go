package validate

import (
	"io"

	"github.com/jacoelho/xsd/internal/xmlstream"
)

// CheckXMLWellFormed checks XML instance syntax without compiling or using a schema runtime.
func CheckXMLWellFormed(r io.Reader, opts Options) error {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return err
	}
	// Separate field initialization avoids clearing the embedded input buffer
	// again after the compiler allocates the zeroed checker.
	var c xmlWellFormedChecker
	c.limits = limits
	return c.check(r)
}

type xmlWellFormedChecker struct {
	doc    xmlDocument[struct{}]
	reader xmlstream.Reader
	limits Limits
}

func (c *xmlWellFormedChecker) check(r io.Reader) error {
	if err := resetInstanceReader(&c.reader, r, c.limits); err != nil {
		return err
	}
	defer c.reader.Detach()
	return c.checkTokens()
}

func (c *xmlWellFormedChecker) checkTokens() error {
	for {
		tok, err := c.reader.Next()
		if err != nil {
			return c.finishTokenStream(err)
		}
		if err := c.checkToken(tok); err != nil {
			return err
		}
	}
}

func (c *xmlWellFormedChecker) finishTokenStream(err error) error {
	if xmlstream.IsEOF(err) {
		return c.doc.Complete(&c.reader)
	}
	return c.streamError(err)
}

func (c *xmlWellFormedChecker) checkToken(tok *xmlstream.Token) error {
	switch tok.Kind { //nolint:exhaustive // Reader.Next rejects directives before consumers see them.
	case xmlstream.KindStart:
		return c.start(tok.Line, tok.Column)
	case xmlstream.KindEnd:
		return c.end(tok.Line, tok.Column)
	case xmlstream.KindCharData, xmlstream.KindComment, xmlstream.KindPI:
		return nil
	default:
	}
	return nil
}

func (c *xmlWellFormedChecker) start(line, col int) error {
	translated, err := c.doc.PrepareStart(&c.reader, c.limits, line, col)
	if err != nil {
		return err
	}
	c.doc.CommitStart(translated, struct{}{})
	return nil
}

func (c *xmlWellFormedChecker) end(line, col int) error {
	if err := c.doc.ValidateEnd(&c.reader, line, col); err != nil {
		return err
	}
	return c.doc.CommitEnd(&c.reader)
}

func (c *xmlWellFormedChecker) streamError(err error) error {
	line, col := streamErrorPosition(&c.reader, err)
	return streamError(err, c.doc.context(line, col), c.limits)
}
