package validate

import (
	"io"

	"github.com/jacoelho/xsd/internal/stream"
)

// CheckXMLWellFormed checks XML instance syntax without compiling or using a schema runtime.
func CheckXMLWellFormed(r io.Reader, opts Options) error {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return err
	}
	c := xmlWellFormedChecker{
		maxDepth:      limits.InstanceDepth,
		maxAttributes: limits.InstanceAttributes,
		maxTokenBytes: limits.InstanceTokenBytes,
		maxInputBytes: limits.InstanceBytes,
	}
	return c.check(r)
}

type xmlWellFormedChecker struct {
	doc           xmlDocument[struct{}]
	maxDepth      int
	maxAttributes int
	maxTokenBytes int64
	maxInputBytes int64
}

func (c *xmlWellFormedChecker) check(r io.Reader) error {
	names := stream.NewCache()
	values := stream.NewCache()
	var parser stream.Parser
	if err := parser.ResetWithConfig(r, &names, &values, stream.Config{
		Limits: stream.Limits{
			MaxInputBytes: c.maxInputBytes,
			MaxTokenBytes: c.maxTokenBytes,
			MaxAttrs:      c.maxAttributes,
		},
		LazyAttrValues: true,
	}); err != nil {
		return instanceReaderError(err)
	}
	defer parser.Detach()
	return c.checkTokens(&parser, &values)
}

func (c *xmlWellFormedChecker) checkTokens(parser *stream.Parser, values *stream.Cache) error {
	for {
		tok, err := parser.Next()
		if err != nil {
			return c.finishTokenStream(parser, err)
		}
		if err := c.checkToken(tok, values); err != nil {
			return err
		}
	}
}

func (c *xmlWellFormedChecker) finishTokenStream(parser *stream.Parser, err error) error {
	if stream.IsOnlyEOF(err) {
		return c.doc.Complete()
	}
	return c.streamError(parser, err)
}

func (c *xmlWellFormedChecker) checkToken(tok stream.Token, values *stream.Cache) error {
	switch tok.Kind {
	case stream.KindStart:
		return c.start(tok.Line, tok.Column, tok.Start, values)
	case stream.KindEnd:
		return c.end(tok.Line, tok.Column, tok.End)
	case stream.KindCharData:
		return c.chars(tok.Line, tok.Column, tok.Data, tok.CDATA)
	case stream.KindDirective:
		return ValidateDirective(c.doc.context(tok.Line, tok.Column), tok.Directive)
	default:
		return nil
	}
}

func (c *xmlWellFormedChecker) start(line, col int, se stream.StartElement, values *stream.Cache) error {
	translated, err := c.doc.PrepareStart(se, values, c.maxDepth, line, col)
	if err != nil {
		return err
	}
	c.doc.CommitStart(translated, false, struct{}{})
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
	return ValidateDocumentCharacterData(data, cdata, c.doc.context(line, col))
}

func (c *xmlWellFormedChecker) streamError(parser *stream.Parser, err error) error {
	line, col := parser.Pos()
	return StreamError(line, col, c.doc.PathString(), err)
}
