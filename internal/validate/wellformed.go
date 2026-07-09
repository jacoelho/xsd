package validate

import (
	"errors"
	"io"

	"github.com/jacoelho/xsd/internal/stream"
)

// CheckXMLWellFormed checks XML instance syntax without compiling or using a schema runtime.
func CheckXMLWellFormed(r io.Reader, opts Options) error {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return err
	}
	reader, err := PrepareInstanceReaderWithBuffer(r, nil)
	if err != nil {
		return err
	}

	c := xmlWellFormedChecker{
		maxDepth:      limits.InstanceDepth,
		maxAttributes: limits.InstanceAttributes,
		maxTokenBytes: limits.InstanceTokenBytes,
	}
	return c.check(reader)
}

type xmlWellFormedChecker struct {
	doc           xmlDocumentState
	maxDepth      int
	maxAttributes int
	maxTokenBytes int64
}

func (c *xmlWellFormedChecker) check(r io.Reader) error {
	names := stream.NewCache()
	values := stream.NewCache()
	var parser stream.Parser
	parser.ResetWithLimit(r, &names, &values, c.maxTokenBytes)
	parser.SetLazyAttrValue(true)
	parser.SetMaxAttrs(c.maxAttributes)
	for {
		tok, err := parser.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return c.streamError(&parser, tok, err)
		}
		switch tok.Kind {
		case stream.KindStart:
			if err := c.start(tok.Line, tok.Column, tok.Start, &values); err != nil {
				return err
			}
		case stream.KindEnd:
			if err := c.end(tok.Line, tok.Column, tok.End); err != nil {
				return err
			}
		case stream.KindCharData:
			if err := c.chars(tok.Line, tok.Column, tok.Data, tok.CDATA); err != nil {
				return err
			}
		case stream.KindDirective:
			return ValidateDirective(c.doc.context(tok.Line, tok.Column), tok.Directive)
		case stream.KindComment, stream.KindPI:
		}
	}
	return c.doc.Complete()
}

func (c *xmlWellFormedChecker) start(line, col int, se stream.StartElement, values *stream.Cache) error {
	translated, err := c.doc.PrepareStart(se, values, xmlDocumentLimits{
		depth:      c.maxDepth,
		attributes: c.maxAttributes,
	}, line, col)
	if err != nil {
		return err
	}
	c.doc.CommitStart(translated.Name, translated.RawName, false)
	return nil
}

func (c *xmlWellFormedChecker) end(line, col int, ee stream.EndElement) error {
	if err := c.doc.ValidateEnd(ee, line, col); err != nil {
		return err
	}
	return c.doc.CommitEnd()
}

func (c *xmlWellFormedChecker) chars(line, col int, data []byte, cdata bool) error {
	if c.doc.Depth() != 0 {
		return nil
	}
	_, err := ValidateCharacterData(CharacterDataInput{
		Data:    data,
		Context: c.doc.context(line, col),
		CDATA:   cdata,
	})
	return err
}

func (c *xmlWellFormedChecker) streamError(parser *stream.Parser, tok stream.Token, err error) error {
	line, col := tok.Line, tok.Column
	if line == 0 {
		line, col = parser.Pos()
	}
	return StreamError(line, col, c.doc.PathString(), err)
}
